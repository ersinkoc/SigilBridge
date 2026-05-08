package commands

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/adapter/mock"
	adminapi "github.com/sigilbridge/sigilbridge/internal/admin"
	"github.com/sigilbridge/sigilbridge/internal/audit"
	"github.com/sigilbridge/sigilbridge/internal/config"
	"github.com/sigilbridge/sigilbridge/internal/ir"
	"github.com/sigilbridge/sigilbridge/internal/pricing"
	"github.com/sigilbridge/sigilbridge/internal/storage"
	"github.com/sigilbridge/sigilbridge/internal/storage/repos"
)

func TestCommands(t *testing.T) {
	if out := Version(VersionInfo{Version: "test"}); out == "" {
		t.Fatalf("empty version")
	}
	if plain, hash, err := KeysCreate("test"); err != nil || plain == "" || hash == "" {
		t.Fatalf("KeysCreate plain=%q hash=%q err=%v", plain, hash, err)
	}
	if raw, err := PricingShow(); err != nil || len(raw) == 0 {
		t.Fatalf("PricingShow len=%d err=%v", len(raw), err)
	}
	db, err := storage.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB() error = %v", err)
	}
	defer db.Close()
	if err := MaintenanceVacuum(context.Background(), db); err != nil {
		t.Fatalf("MaintenanceVacuum() error = %v", err)
	}
}

func TestReadPricingSourceHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`
pricing:
  fake:
    model:
      input_per_mtok_cents: 100
      output_per_mtok_cents: 200
`))
	}))
	defer server.Close()

	raw, err := readPricingSource(server.URL)
	if err != nil {
		t.Fatalf("readPricingSource() error = %v", err)
	}
	if _, err := pricing.Parse(raw); err != nil {
		t.Fatalf("Parse(downloaded pricing) error = %v", err)
	}
}

func TestReadPricingSourceRejectsLargeHTTPResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("x", maxPricingSourceBytes+1)))
	}))
	defer server.Close()

	if _, err := readPricingSource(server.URL); err == nil {
		t.Fatalf("readPricingSource() error = nil")
	}
}

func TestRouterFromConfigPoolsPublishesConfiguredModelAliases(t *testing.T) {
	registry, err := adapter.NewRegistry(mock.New())
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	_, models, err := RouterFromConfigPools([]config.Pool{{
		Name:     "openai",
		Strategy: "priority_first",
		Upstreams: []config.Upstream{{
			ID:       "mock-1",
			Provider: "mock",
			Config: map[string]any{
				"model":         "gpt-test",
				"model_aliases": []any{"gpt-test-fast"},
			},
		}},
	}}, registry)
	if err != nil {
		t.Fatalf("RouterFromConfigPools() error = %v", err)
	}
	want := []string{"openai", "gpt-test", "gpt-test-fast"}
	if strings.Join(models, ",") != strings.Join(want, ",") {
		t.Fatalf("models = %#v, want %#v", models, want)
	}
}

func TestCLIEnablePersistsProtocolArgsAndStatus(t *testing.T) {
	dir := t.TempDir()
	poolsPath := filepath.Join(dir, "pools.yaml")
	rt := &adminRuntime{
		poolsPath: poolsPath,
		pools:     &config.PoolsFile{},
		cfg:       &config.Config{},
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("Executable() error = %v", err)
	}
	service := adminCredentialService{rt: rt}
	out, err := service.CLIEnable(context.Background(), adminapi.CLIEnableRequest{Provider: "gemini_cli", Command: exe})
	if err != nil {
		t.Fatalf("CLIEnable() error = %v", err)
	}
	if out["pool"] != "gemini" || len(rt.pools.Pools) != 1 {
		t.Fatalf("CLIEnable() out=%#v pools=%#v", out, rt.pools.Pools)
	}
	upstream := rt.pools.Pools[0].Upstreams[0]
	if upstream.Config["protocol"] != "acp" {
		t.Fatalf("protocol = %#v", upstream.Config["protocol"])
	}
	if upstream.Config["framing"] != "ndjson" {
		t.Fatalf("framing = %#v", upstream.Config["framing"])
	}
	args, ok := upstream.Config["args"].([]string)
	if !ok || strings.Join(args, " ") != "--acp --skip-trust" {
		t.Fatalf("args = %#v", upstream.Config["args"])
	}
	status := service.cliStatus()
	agents := status["agents"].([]map[string]any)
	if len(agents) != 1 || agents[0]["protocol"] != "acp" || agents[0]["framing"] != "ndjson" || agents[0]["auth_status"] == "" {
		t.Fatalf("cliStatus agents = %#v", agents)
	}
}

func TestNormalizeCatalogCredentialCodingPlanProviders(t *testing.T) {
	provider, model := normalizeCatalogCredential("openai_api", "minimax-coding-plan", "https://api.minimax.io/anthropic/v1", "MiniMax-M2.5")
	if provider != "anthropic_api" || model != "MiniMax-M2.5" {
		t.Fatalf("MiniMax provider=%q model=%q", provider, model)
	}
	provider, model = normalizeCatalogCredential("openai_api", "kimi-for-coding", "https://api.kimi.com/coding/v1", "k2p6")
	if provider != "anthropic_api" || model != "kimi-for-coding" {
		t.Fatalf("Kimi provider=%q model=%q", provider, model)
	}
	provider, model = normalizeCatalogCredential("openai_api", "zai-coding-plan", "https://api.z.ai/api/coding/paas/v4", "glm-5-turbo")
	if provider != "openai_api" || model != "glm-5-turbo" {
		t.Fatalf("Z.AI provider=%q model=%q", provider, model)
	}
}

func TestInitConfigCreatesRunnableScaffold(t *testing.T) {
	dir := t.TempDir()
	result, err := InitConfig(dir, false)
	if err != nil {
		t.Fatalf("InitConfig() error = %v", err)
	}
	if result.AdminToken == "" {
		t.Fatalf("admin token is empty")
	}
	for _, path := range []string{result.ConfigPath, result.PoolsPath, result.AdminTokensPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("scaffold file missing %s: %v", path, err)
		}
	}
	if _, err := InitConfig(dir, false); err == nil {
		t.Fatalf("InitConfig() overwrite error = nil")
	}
	if _, err := InitConfig(dir, true); err != nil {
		t.Fatalf("InitConfig(force) error = %v", err)
	}
}

func TestMaintenancePruneAuditConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`
server:
  bind: 127.0.0.1:8787
admin:
  bind: 127.0.0.1:8788
storage:
  path: data/sigilbridge.db
audit:
  enabled: true
  content_mode: none
  retention_days: 30
  rotate_compress_after_days: 7
vault:
  master_key_env: SIGILBRIDGE_MASTER_KEY
logging:
  format: json
`), 0o600); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	auditDir := filepath.Join(dir, "audit")
	if err := os.MkdirAll(auditDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"2026-04-29.jsonl":    "old\n",
		"2026-02-01.jsonl.gz": "expired",
	} {
		if err := os.WriteFile(filepath.Join(auditDir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	if err := MaintenancePruneAuditConfig(context.Background(), configPath, now); err != nil {
		t.Fatalf("MaintenancePruneAuditConfig() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(auditDir, "2026-04-29.jsonl.gz")); err != nil {
		t.Fatalf("compressed audit file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(auditDir, "2026-02-01.jsonl.gz")); !os.IsNotExist(err) {
		t.Fatalf("expired audit file should be removed, err=%v", err)
	}
}

func TestAdminBudgetServiceAggregatesCurrentCounters(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB() error = %v", err)
	}
	defer db.Close()
	if err := storage.Up(db); err != nil {
		t.Fatalf("storage.Up() error = %v", err)
	}
	keys := repos.NewBridgeKeys(db)
	counters := repos.NewBudgetCounters(db)
	now := time.Now().UTC()
	key := repos.BridgeKey{
		ID:             "key1",
		Hash:           "sha256:x",
		Name:           "production",
		CreatedAt:      now,
		ScopesJSON:     "{}",
		BudgetsJSON:    `{"daily_cents":500,"monthly_cents":5000,"hard_cap":true}`,
		RateLimitsJSON: "{}",
		MetadataJSON:   "{}",
	}
	if err := keys.Put(ctx, key); err != nil {
		t.Fatalf("BridgeKeys.Put() error = %v", err)
	}
	if err := counters.Put(ctx, repos.BudgetCounter{KeyID: key.ID, Period: "daily", Bucket: now.Format("2006-01-02"), Cents: 125}); err != nil {
		t.Fatalf("BudgetCounters.Put(daily) error = %v", err)
	}
	if err := counters.Put(ctx, repos.BudgetCounter{KeyID: key.ID, Period: "monthly", Bucket: now.Format("2006-01"), Cents: 975}); err != nil {
		t.Fatalf("BudgetCounters.Put(monthly) error = %v", err)
	}
	service := adminBudgetService{keys: keys, counters: counters}
	budgets, err := service.Budgets(ctx)
	if err != nil {
		t.Fatalf("Budgets() error = %v", err)
	}
	if budgets["daily_cents"] != int64(500) || budgets["daily_used_cents"] != int64(125) || budgets["monthly_used_cents"] != int64(975) {
		t.Fatalf("Budgets() = %#v", budgets)
	}
	usage, err := service.Usage(ctx)
	if err != nil {
		t.Fatalf("Usage() error = %v", err)
	}
	items := usage["items"].([]map[string]any)
	if len(items) != 1 || items[0]["monthly_cents"] != int64(975) || items[0]["hard_cap"] != true {
		t.Fatalf("Usage() items = %#v", items)
	}
}

func TestAdminHealthServiceMergesCooldownState(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB() error = %v", err)
	}
	defer db.Close()
	if err := storage.Up(db); err != nil {
		t.Fatalf("storage.Up() error = %v", err)
	}
	now := time.Now().UTC()
	cooldowns := repos.NewCooldowns(db)
	if err := cooldowns.Put(ctx, repos.Cooldown{UpstreamID: "up1", PoolName: "default", State: "cooldown", ConsecutiveFailures: 2, LastError: "rate limited", UpdatedAt: now}); err != nil {
		t.Fatalf("Cooldowns.Put() error = %v", err)
	}
	rt := &adminRuntime{pools: &config.PoolsFile{Pools: []config.Pool{{
		Name:     "default",
		Strategy: "priority",
		Upstreams: []config.Upstream{{
			ID:       "up1",
			Provider: "mock",
			Priority: 1,
			Weight:   100,
		}},
	}}}}
	service := adminHealthService{rt: rt, cooldowns: cooldowns}
	detail, err := service.Detail(ctx)
	if err != nil {
		t.Fatalf("Detail() error = %v", err)
	}
	upstreams := detail["upstreams"].([]map[string]any)
	if len(upstreams) != 1 || upstreams[0]["state"] != "cooldown" || upstreams[0]["last_error"] != "rate limited" {
		t.Fatalf("Detail() upstreams = %#v", upstreams)
	}
}

func TestAdminPoolProbeCallsConfiguredUpstreams(t *testing.T) {
	registry, err := adapter.NewRegistry(mock.New())
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	rt := &adminRuntime{
		registry: registry,
		pools: &config.PoolsFile{Pools: []config.Pool{{
			Name:     "default",
			Strategy: "priority",
			Upstreams: []config.Upstream{{
				ID:       "mock-primary",
				Provider: "mock",
				Priority: 1,
				Weight:   100,
				Config:   map[string]any{"model": "mock-chat"},
			}},
		}}},
	}
	service := adminPoolService{rt: rt}
	out, err := service.Probe(context.Background(), "default")
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if out["ok"] != true || out["checked"] != 1 || out["passed"] != 1 {
		t.Fatalf("Probe() = %#v", out)
	}
	upstreams := out["upstreams"].([]map[string]any)
	if len(upstreams) != 1 || upstreams[0]["ok"] != true || upstreams[0]["provider"] != "mock" {
		t.Fatalf("Probe() upstreams = %#v", upstreams)
	}
}

func TestAdminResponseTextCleansMiniMaxReasoningLeak(t *testing.T) {
	resp := ir.Response{
		UpstreamModel: "MiniMax-M2.5",
		Content:       []ir.ContentBlock{{Type: ir.ContentText, Text: "The user asks: Reply with exactly: OK\n\nThus answer: OK\nOK"}},
	}
	if got := strings.TrimSpace(adminResponseText(resp)); got != "OK" {
		t.Fatalf("adminResponseText() = %q", got)
	}
}

func TestServeAccountingHelpers(t *testing.T) {
	table, err := pricing.Parse([]byte(`
pricing:
  fake:
    model:
      input_per_mtok_cents: 1000000
      output_per_mtok_cents: 2000000
`))
	if err != nil {
		t.Fatalf("Parse(pricing) error = %v", err)
	}
	resp := ir.Response{UpstreamProvider: "fake", UpstreamModel: "model", Usage: ir.Usage{InputTokens: 3, OutputTokens: 4}}
	if got := actualCostCents(table, resp); got != 11 {
		t.Fatalf("actualCostCents() = %d, want 11", got)
	}
	req := ir.Request{ID: "req", BridgeKeyID: "key", IngressFormat: ir.IngressOpenAI, ModelAlias: "model", Messages: []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "hello"}}}}}
	record := auditRecord(nilRequest(), req, resp, nil, 5*time.Millisecond, 11, audit.ContentHash)
	if record.Status != "ok" || record.CostCents != 11 || record.Content.PromptHash == "" {
		t.Fatalf("record = %#v", record)
	}
}

func TestPublicHTTPBase(t *testing.T) {
	tests := map[string]string{
		"127.0.0.1:8787": "http://127.0.0.1:8787",
		"0.0.0.0:8787":   "http://127.0.0.1:8787",
		":8787":          "http://127.0.0.1:8787",
		"localhost:8787": "http://localhost:8787",
	}
	for bind, want := range tests {
		if got := publicHTTPBase(bind); got != want {
			t.Fatalf("publicHTTPBase(%q) = %q, want %q", bind, got, want)
		}
	}
}

func TestMountAdminUIServesRootAssets(t *testing.T) {
	assetName := firstAdminAsset(t)
	mux := http.NewServeMux()
	mountAdminUI(mux, true)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/assets/"+assetName, nil))
	if resp.Code != http.StatusOK || strings.Contains(resp.Body.String(), "<!doctype html>") {
		t.Fatalf("asset status=%d body=%q", resp.Code, resp.Body.String())
	}
	assertAdminSecurityHeaders(t, resp.Result().Header)
	resp = httptest.NewRecorder()
	mux.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/assets/missing.css", nil))
	if resp.Code != http.StatusNotFound {
		t.Fatalf("missing asset status=%d body=%q", resp.Code, resp.Body.String())
	}
	resp = httptest.NewRecorder()
	mux.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/", nil))
	if resp.Code != http.StatusMovedPermanently || resp.Header().Get("Location") != "/admin/ui/" {
		t.Fatalf("root redirect status=%d location=%q", resp.Code, resp.Header().Get("Location"))
	}
	assertAdminSecurityHeaders(t, resp.Result().Header)
	resp = httptest.NewRecorder()
	mux.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/favicon.ico", nil))
	if resp.Code != http.StatusNotFound {
		t.Fatalf("favicon status=%d body=%q", resp.Code, resp.Body.String())
	}
}

func TestMountAdminAddsSecurityHeadersToAPI(t *testing.T) {
	mux := http.NewServeMux()
	mountAdmin(mux, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), false)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/admin/v1/health", nil))
	if resp.Code != http.StatusNoContent {
		t.Fatalf("admin API status=%d body=%q", resp.Code, resp.Body.String())
	}
	assertAdminSecurityHeaders(t, resp.Result().Header)
}

func assertAdminSecurityHeaders(t *testing.T, header http.Header) {
	t.Helper()
	if got := header.Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options = %q, want DENY", got)
	}
	if got := header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := header.Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("Referrer-Policy = %q, want no-referrer", got)
	}
	if got := header.Get("Content-Security-Policy"); !strings.Contains(got, "frame-ancestors 'none'") || !strings.Contains(got, "default-src 'self'") {
		t.Fatalf("Content-Security-Policy = %q", got)
	}
}

func firstAdminAsset(t *testing.T) string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join("..", "..", "..", "internal", "admin", "ui", "dist", "assets"))
	if err != nil {
		t.Fatalf("ReadDir(admin assets) error = %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			return entry.Name()
		}
	}
	t.Fatal("no embedded admin assets found")
	return ""
}

func TestServeConfigWritesAuditRecord(t *testing.T) {
	dir := t.TempDir()
	addr := freeAddr(t)
	adminAddr := freeAddr(t)
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`
server:
  bind: `+addr+`
admin:
  bind: `+adminAddr+`
  ui_enabled: true
storage:
  path: data/sigilbridge.db
audit:
  enabled: true
  content_mode: hash
  retention_days: 30
  rotate_compress_after_days: 7
vault:
  master_key_env: SIGILBRIDGE_MASTER_KEY
logging:
  format: json
pools_file: pools.yaml
`), 0o600); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pools.yaml"), []byte(`
pools:
  - name: mock-chat
    strategy: priority_first
    upstreams:
      - id: mock-1
        provider: mock
        priority: 1
        weight: 1
        config:
          input_tokens: 7
          output_tokens: 3
`), 0o600); err != nil {
		t.Fatalf("WriteFile(pools) error = %v", err)
	}
	t.Setenv("SIGILBRIDGE_MASTER_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	plain, _, _, err := KeysCreateStored(context.Background(), configPath, "test", "smoke")
	if err != nil {
		t.Fatalf("KeysCreateStored() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- ServeConfig(ctx, configPath) }()
	waitForHTTP(t, "http://"+addr+"/healthz")
	waitForHTTP(t, "http://"+adminAddr+"/")
	body := strings.NewReader(`{"model":"mock-chat","messages":[{"role":"user","content":"hello"}]}`)
	req, err := http.NewRequest(http.MethodPost, "http://"+addr+"/v1/chat/completions", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+plain)
	resp, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("chat request error = %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("chat status = %d", resp.StatusCode)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ServeConfig() error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ServeConfig did not stop")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "audit", time.Now().UTC().Format("2006-01-02")+".jsonl"))
	if err != nil {
		t.Fatalf("ReadFile(audit) error = %v", err)
	}
	var record audit.Record
	if err := json.Unmarshal(bytes.TrimSpace(raw), &record); err != nil {
		t.Fatalf("Unmarshal(audit) error = %v raw=%s", err, raw)
	}
	if record.BridgeKeyID == "" || record.ModelAlias != "mock-chat" || record.InputTokens != 7 || record.OutputTokens != 3 || record.Status != "ok" {
		t.Fatalf("record = %#v", record)
	}
	db, err := storage.OpenDB(filepath.Join(dir, "data", "sigilbridge.db"))
	if err != nil {
		t.Fatalf("OpenDB() error = %v", err)
	}
	defer db.Close()
	entries, err := repos.NewAuditIndex(db).List(context.Background(), 10)
	if err != nil {
		t.Fatalf("AuditIndex.List() error = %v", err)
	}
	if len(entries) == 0 || entries[0].RequestID == "" {
		t.Fatalf("entries = %#v", entries)
	}
}

func nilRequest() *http.Request {
	return httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
}

func freeAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("Close(listener) error = %v", err)
	}
	return addr
}

func waitForHTTP(t *testing.T, rawURL string) {
	t.Helper()
	client := &http.Client{Timeout: 200 * time.Millisecond}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(rawURL)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode < 500 {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", rawURL)
}
