package commands

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/adapter/builtins"
	cliacpadapter "github.com/sigilbridge/sigilbridge/internal/adapter/cliacp"
	sessionadapter "github.com/sigilbridge/sigilbridge/internal/adapter/session"
	adminapi "github.com/sigilbridge/sigilbridge/internal/admin"
	"github.com/sigilbridge/sigilbridge/internal/auth"
	"github.com/sigilbridge/sigilbridge/internal/config"
	"github.com/sigilbridge/sigilbridge/internal/events"
	"github.com/sigilbridge/sigilbridge/internal/httpclient"
	"github.com/sigilbridge/sigilbridge/internal/ir"
	"github.com/sigilbridge/sigilbridge/internal/oauth"
	"github.com/sigilbridge/sigilbridge/internal/router"
	"github.com/sigilbridge/sigilbridge/internal/storage/repos"
	"github.com/sigilbridge/sigilbridge/internal/vault"
	"gopkg.in/yaml.v3"
)

type adminRuntime struct {
	db           *sql.DB
	configPath   string
	cfg          *config.Config
	poolsPath    string
	pools        *config.PoolsFile
	oauthReg     *oauth.Registry
	oauthMgr     *oauth.Manager
	tokenVault   *vault.Vault
	registry     *adapter.Registry
	liveRouter   *liveRouter
	events       *events.Bus
	sessionAuth  *auth.AdminSessionManager
	tokenStore   *auth.AdminTokenStore
	pendingMu    sync.Mutex
	pendingOAuth map[string]pendingOAuthBootstrap
}

type pendingOAuthBootstrap struct {
	Provider    string
	Name        string
	RedirectURI string
	PKCE        oauth.PKCE
	VaultID     string
	CreatedAt   time.Time
}

func newAdminRuntime(db *sql.DB, configPath string, cfg *config.Config, poolsPath string, pools *config.PoolsFile, bus *events.Bus) (*adminRuntime, error) {
	masterKey, err := vault.LoadMasterKeyFromEnv(cfg.Vault.MasterKeyEnv)
	if err != nil {
		return nil, err
	}
	defer masterKey.Wipe()
	key := masterKey.Bytes()
	defer func() {
		for i := range key {
			key[i] = 0
		}
	}()
	sessionAuth, err := auth.NewAdminSessionManager(key)
	if err != nil {
		return nil, err
	}
	tokenVault, err := vault.New(db, key)
	if err != nil {
		return nil, err
	}
	tokensPath := config.ResolveRelative(configPath, cfg.Admin.TokensFile)
	tokenStore, err := auth.LoadAdminTokens(tokensPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		token, createErr := randomToken()
		if createErr != nil {
			return nil, createErr
		}
		raw := []byte("tokens:\n  - name: local-admin\n    token: " + token + "\n")
		if writeErr := os.WriteFile(tokensPath, raw, 0o600); writeErr != nil {
			return nil, writeErr
		}
		fmt.Fprintf(os.Stderr, "created %s\nadmin token: %s\n", tokensPath, token)
		tokenStore, err = auth.LoadAdminTokens(tokensPath)
		if err != nil {
			return nil, err
		}
	}
	oauthReg, err := oauth.LoadRegistry(config.ResolveRelative(configPath, cfg.OAuth.ProvidersFile))
	if err != nil {
		tokenVault.Close()
		return nil, err
	}
	oauthMgr := oauth.NewManager(oauthReg, tokenVault, nil)
	return &adminRuntime{
		db:           db,
		configPath:   configPath,
		cfg:          cfg,
		poolsPath:    poolsPath,
		pools:        pools,
		oauthReg:     oauthReg,
		oauthMgr:     oauthMgr,
		tokenVault:   tokenVault,
		events:       bus,
		sessionAuth:  sessionAuth,
		tokenStore:   tokenStore,
		pendingOAuth: map[string]pendingOAuthBootstrap{},
	}, nil
}

func (rt *adminRuntime) services() adminapi.Services {
	return adminapi.Services{
		Auth:        adminAuthService{rt: rt},
		Keys:        adminKeyService{repo: repos.NewBridgeKeys(rt.db), now: time.Now},
		Pools:       adminPoolService{rt: rt},
		Endpoints:   adminEndpointService{rt: rt},
		Chat:        adminChatService{rt: rt},
		Credentials: adminCredentialService{rt: rt},
		Audit:       adminAuditService{repo: repos.NewAuditIndex(rt.db)},
		Budgets:     adminBudgetService{keys: repos.NewBridgeKeys(rt.db), counters: repos.NewBudgetCounters(rt.db)},
		Health:      adminHealthService{rt: rt, cooldowns: repos.NewCooldowns(rt.db)},
		Reload:      adminReloadService{rt: rt},
		Events:      rt.events,
	}
}

func (rt *adminRuntime) close() {
	if rt != nil && rt.tokenVault != nil {
		rt.tokenVault.Close()
	}
}

type adminAuthService struct {
	rt *adminRuntime
}

func (s adminAuthService) Login(_ context.Context, token string) (string, error) {
	adminToken, ok := s.rt.tokenStore.Verify(token)
	if !ok {
		return "", fmt.Errorf("invalid admin token")
	}
	session, _, err := s.rt.sessionAuth.Issue(adminToken.Name)
	return session, err
}

func (s adminAuthService) Verify(_ context.Context, r *http.Request) (string, error) {
	if adminToken, ok := s.rt.tokenStore.VerifyHeader(r.Header.Get("Authorization")); ok {
		return adminToken.Name, nil
	}
	if subject, err := s.rt.sessionAuth.VerifyCookie(r); err == nil {
		return subject, nil
	}
	return "", auth.ErrInvalidSession
}

type adminKeyService struct {
	repo *repos.BridgeKeys
	now  func() time.Time
}

func (s adminKeyService) List(ctx context.Context) ([]adminapi.KeyDTO, error) {
	rows, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]adminapi.KeyDTO, 0, len(rows))
	for _, row := range rows {
		dto, err := keyDTO(row)
		if err != nil {
			return nil, err
		}
		out = append(out, dto)
	}
	return out, nil
}

func (s adminKeyService) Create(ctx context.Context, req adminapi.CreateKeyRequest) (adminapi.CreateKeyResponse, error) {
	prefix := strings.TrimSpace(req.Prefix)
	if prefix == "" {
		prefix = auth.PrefixTest
	}
	plaintext, hash, err := auth.Generate(prefix)
	if err != nil {
		return adminapi.CreateKeyResponse{}, err
	}
	id, err := randomID("key")
	if err != nil {
		return adminapi.CreateKeyResponse{}, err
	}
	metadata := req.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	scopes := req.Scopes
	if scopes == nil {
		scopes = map[string]any{}
	}
	budgets := req.Budgets
	if budgets == nil {
		budgets = map[string]any{}
	}
	rateLimits := req.RateLimits
	if rateLimits == nil {
		rateLimits = map[string]any{}
	}
	name, _ := metadata["name"].(string)
	if strings.TrimSpace(name) == "" {
		name = id
	}
	now := s.now().UTC()
	row := repos.BridgeKey{
		ID:             id,
		Hash:           hash,
		Name:           name,
		CreatedAt:      now,
		ScopesJSON:     mustJSON(scopes),
		BudgetsJSON:    mustJSON(budgets),
		RateLimitsJSON: mustJSON(rateLimits),
		MetadataJSON:   mustJSON(metadata),
	}
	if err := s.repo.Put(ctx, row); err != nil {
		return adminapi.CreateKeyResponse{}, err
	}
	dto, err := keyDTO(row)
	if err != nil {
		return adminapi.CreateKeyResponse{}, err
	}
	return adminapi.CreateKeyResponse{KeyDTO: dto, Plaintext: plaintext}, nil
}

func (s adminKeyService) Get(ctx context.Context, id string) (adminapi.KeyDTO, error) {
	row, err := s.repo.Get(ctx, id)
	if err != nil {
		return adminapi.KeyDTO{}, err
	}
	return keyDTO(row)
}

func (s adminKeyService) Patch(ctx context.Context, id string, patch map[string]any) (adminapi.KeyDTO, error) {
	row, err := s.repo.Get(ctx, id)
	if err != nil {
		return adminapi.KeyDTO{}, err
	}
	if name, ok := patch["name"].(string); ok && strings.TrimSpace(name) != "" {
		row.Name = name
	}
	if metadata, ok := patch["metadata"].(map[string]any); ok {
		row.MetadataJSON = mustJSON(metadata)
	}
	if scopes, ok := patch["scopes"].(map[string]any); ok {
		row.ScopesJSON = mustJSON(scopes)
	}
	if budgets, ok := patch["budgets"].(map[string]any); ok {
		row.BudgetsJSON = mustJSON(budgets)
	}
	if rateLimits, ok := patch["rate_limits"].(map[string]any); ok {
		row.RateLimitsJSON = mustJSON(rateLimits)
	}
	if revoked, ok := patch["revoked"].(bool); ok {
		if revoked {
			row.RevokedAt = time.Now().UTC()
		} else {
			row.RevokedAt = time.Time{}
		}
	}
	if err := s.repo.Put(ctx, row); err != nil {
		return adminapi.KeyDTO{}, err
	}
	return keyDTO(row)
}

func (s adminKeyService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

type adminPoolService struct {
	rt *adminRuntime
}

func (s adminPoolService) List(context.Context) ([]adminapi.PoolDTO, error) {
	out := make([]adminapi.PoolDTO, 0, len(s.rt.pools.Pools))
	for _, pool := range s.rt.pools.Pools {
		out = append(out, poolDTO(pool))
	}
	return out, nil
}

func (s adminPoolService) Upsert(_ context.Context, pool adminapi.PoolDTO) (adminapi.PoolDTO, error) {
	if strings.TrimSpace(pool.ID) == "" {
		return adminapi.PoolDTO{}, fmt.Errorf("pool id is required")
	}
	next := dtoPool(pool)
	replaced := false
	for i := range s.rt.pools.Pools {
		if s.rt.pools.Pools[i].Name == pool.ID {
			s.rt.pools.Pools[i] = next
			replaced = true
			break
		}
	}
	if !replaced {
		s.rt.pools.Pools = append(s.rt.pools.Pools, next)
	}
	if err := s.persist(); err != nil {
		return adminapi.PoolDTO{}, err
	}
	return poolDTO(next), nil
}

func (s adminPoolService) Delete(_ context.Context, id string) error {
	for i := range s.rt.pools.Pools {
		if s.rt.pools.Pools[i].Name == id {
			s.rt.pools.Pools = append(s.rt.pools.Pools[:i], s.rt.pools.Pools[i+1:]...)
			return s.persist()
		}
	}
	return fmt.Errorf("pool %q not found", id)
}

func (s adminPoolService) Probe(ctx context.Context, id string) (map[string]any, error) {
	for _, pool := range s.rt.pools.Pools {
		if pool.Name == id {
			results := make([]map[string]any, 0, len(pool.Upstreams))
			okCount := 0
			for _, upstream := range pool.Upstreams {
				result := s.probeUpstream(ctx, pool.Name, upstream)
				if ok, _ := result["ok"].(bool); ok {
					okCount++
				}
				results = append(results, result)
			}
			out := map[string]any{
				"ok":        okCount == len(pool.Upstreams),
				"pool":      id,
				"checked":   len(pool.Upstreams),
				"passed":    okCount,
				"upstreams": results,
			}
			if s.rt.events != nil {
				s.rt.events.Publish(events.Event{Type: "pool_probe", Data: map[string]any{"pool": id, "checked": len(pool.Upstreams), "passed": okCount}})
			}
			return out, nil
		}
	}
	return nil, fmt.Errorf("pool %q not found", id)
}

func (s adminPoolService) probeUpstream(ctx context.Context, poolName string, upstream config.Upstream) map[string]any {
	out := map[string]any{
		"id":       upstream.ID,
		"pool":     poolName,
		"provider": upstream.Provider,
		"ok":       false,
	}
	if s.rt.registry == nil {
		out["error"] = "adapter registry is not ready"
		return out
	}
	provider, err := s.rt.registry.Get(upstream.Provider)
	if err != nil {
		out["error"] = err.Error()
		return out
	}
	model := strings.TrimSpace(stringFromMap(upstream.Config, "model"))
	if model == "" {
		model = poolName
	}
	requestID, err := randomID("admin_probe")
	if err != nil {
		out["error"] = err.Error()
		return out
	}
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	start := time.Now()
	resp, err := provider.Chat(probeCtx, ir.Request{
		Version:    ir.Version,
		ID:         requestID,
		ReceivedAt: start.UTC(),
		ModelAlias: model,
		MaxTokens:  512,
		Messages:   []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "Reply with exactly: SIGILBRIDGE_PROBE_OK"}}}},
		Extras:     map[string]any{},
	}, adapter.ProviderConfig{UpstreamID: upstream.ID, Raw: upstream.Config})
	out["latency_ms"] = time.Since(start).Milliseconds()
	out["model"] = model
	if err != nil {
		out["error"] = err.Error()
		return out
	}
	content := strings.TrimSpace(adminResponseText(resp))
	if content == "" {
		out["error"] = "upstream returned an empty text response"
		out["response_id"] = resp.ID
		out["upstream_provider"] = resp.UpstreamProvider
		out["upstream_model"] = resp.UpstreamModel
		out["stop_reason"] = resp.StopReason
		out["input_tokens"] = resp.Usage.InputTokens
		out["output_tokens"] = resp.Usage.OutputTokens
		return out
	}
	out["ok"] = true
	out["response_id"] = resp.ID
	out["upstream_provider"] = resp.UpstreamProvider
	out["upstream_model"] = resp.UpstreamModel
	out["stop_reason"] = resp.StopReason
	out["content"] = content
	out["input_tokens"] = resp.Usage.InputTokens
	out["output_tokens"] = resp.Usage.OutputTokens
	return out
}

func (s adminPoolService) persist() error {
	raw, err := yaml.Marshal(s.rt.pools)
	if err != nil {
		return err
	}
	var nextRouter *router.Router
	var nextModels []string
	if s.rt.registry != nil {
		nextRouter, nextModels, err = RouterFromConfigPools(s.rt.pools.Pools, s.rt.registry)
		if err != nil {
			return err
		}
	}
	if err := os.WriteFile(s.rt.poolsPath, raw, 0o600); err != nil {
		return err
	}
	if nextRouter != nil && s.rt.liveRouter != nil {
		s.rt.liveRouter.Set(nextRouter, nextModels)
	}
	if s.rt.events != nil {
		s.rt.events.Publish(events.Event{Type: "pool_reloaded", Data: map[string]any{"ok": true}})
	}
	return nil
}

type adminCredentialService struct {
	rt *adminRuntime
}

type adminChatService struct {
	rt *adminRuntime
}

type adminEndpointService struct {
	rt *adminRuntime
}

func (s adminEndpointService) Info(context.Context) (adminapi.EndpointInfoResponse, error) {
	base := publicHTTPBase(s.rt.cfg.Server.Bind)
	return adminapi.EndpointInfoResponse{
		OpenAIBase:        base + "/v1",
		OpenAIChat:        base + "/v1/chat/completions",
		OpenAIModels:      base + "/v1/models",
		AnthropicBase:     base,
		AnthropicMessages: base + "/v1/messages",
	}, nil
}

func (s adminChatService) Test(ctx context.Context, req adminapi.ChatTestRequest) (adminapi.ChatTestResponse, error) {
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return adminapi.ChatTestResponse{}, fmt.Errorf("message is required")
	}
	model := strings.TrimSpace(req.Model)
	if model == "" && s.rt.liveRouter != nil {
		models := s.rt.liveRouter.Models()
		if len(models) > 0 {
			model = models[0]
		}
	}
	if model == "" {
		return adminapi.ChatTestResponse{}, fmt.Errorf("model is required")
	}
	if s.rt.liveRouter == nil {
		return adminapi.ChatTestResponse{}, fmt.Errorf("router not ready")
	}
	start := time.Now()
	requestID, err := randomID("admin_chat")
	if err != nil {
		return adminapi.ChatTestResponse{}, err
	}
	irReq := ir.Request{
		Version:       ir.Version,
		ID:            requestID,
		BridgeKeyID:   "admin-ui",
		ReceivedAt:    start.UTC(),
		IngressFormat: ir.IngressOpenAI,
		ModelAlias:    model,
		System:        strings.TrimSpace(req.System),
		MaxTokens:     512,
		Messages: []ir.Message{{
			Role: ir.RoleUser,
			Content: []ir.ContentBlock{{
				Type: ir.ContentText,
				Text: message,
			}},
		}},
		Temperature: req.Temperature,
		Extras:      map[string]any{},
	}
	resp, err := s.rt.liveRouter.Dispatch(ctx, irReq)
	if err != nil {
		return adminapi.ChatTestResponse{}, err
	}
	content := adminResponseText(resp)
	if strings.TrimSpace(content) == "" {
		return adminapi.ChatTestResponse{}, fmt.Errorf("%s upstream %s returned an empty text response", resp.UpstreamProvider, resp.UpstreamModel)
	}
	return adminapi.ChatTestResponse{
		ID:               resp.ID,
		Model:            model,
		UpstreamProvider: resp.UpstreamProvider,
		UpstreamModel:    resp.UpstreamModel,
		Content:          content,
		StopReason:       resp.StopReason,
		LatencyMs:        time.Since(start).Milliseconds(),
		InputTokens:      resp.Usage.InputTokens,
		OutputTokens:     resp.Usage.OutputTokens,
	}, nil
}

func adminResponseText(resp ir.Response) string {
	text := responseText(resp)
	if resp.UpstreamModel == "MiniMax-M2.5" || resp.UpstreamModel == "MiniMax-M2.7" || strings.HasPrefix(resp.UpstreamModel, "MiniMax-") {
		return cleanMiniMaxReasoningLeak(text)
	}
	return text
}

func cleanMiniMaxReasoningLeak(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	looksLikeReasoningLeak := strings.Contains(lower, "the user asks") ||
		strings.Contains(lower, "the user says") ||
		strings.Contains(lower, "reply with exactly") ||
		strings.Contains(lower, "thus answer") ||
		strings.Contains(lower, "thus responding")
	if !looksLikeReasoningLeak {
		return text
	}
	lines := strings.Split(trimmed, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if len(line) <= 200 {
			return line + "\n"
		}
		break
	}
	return text
}

func (s adminCredentialService) Delete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("credential id is required")
	}
	return s.rt.tokenVault.Delete(ctx, id)
}

func (s adminCredentialService) APIKeyCreate(ctx context.Context, req adminapi.APIKeyCredentialRequest) (map[string]any, error) {
	provider := strings.TrimSpace(req.Provider)
	provider, req.Model = normalizeCatalogCredential(provider, req.Name, req.BaseURL, req.Model)
	if _, ok := apiKeyProviderDefaults()[provider]; !ok {
		return nil, fmt.Errorf("unsupported api-key provider %q", provider)
	}
	name := cleanName(req.Name, "default")
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("api_key is required")
	}
	id := "vault://apikey/" + path.Clean(provider+"/"+name)
	metadata := map[string]string{"provider": provider, "kind": "api_key", "updated_at": time.Now().UTC().Format(time.RFC3339)}
	if err := s.rt.tokenVault.Put(ctx, id, []byte(apiKey), metadata); err != nil {
		return nil, err
	}
	out := map[string]any{"id": id, "provider": provider, "ok": true}
	attach := strings.TrimSpace(req.Pool) != "" || strings.TrimSpace(req.UpstreamID) != "" || strings.TrimSpace(req.Model) != "" || strings.TrimSpace(req.BaseURL) != ""
	if !attach {
		return out, nil
	}
	poolName := cleanName(req.Pool, providerPoolName(provider))
	upstreamID := cleanName(req.UpstreamID, provider+"-"+name)
	upstream := config.Upstream{
		ID:       upstreamID,
		Provider: provider,
		Priority: 1,
		Weight:   100,
		Config: map[string]any{
			"api_key_ref": id,
		},
	}
	if model := strings.TrimSpace(req.Model); model != "" {
		upstream.Config["model"] = model
	}
	if baseURL := strings.TrimSpace(req.BaseURL); baseURL != "" {
		upstream.Config["base_url"] = baseURL
	}
	if err := s.upsertUpstream(poolName, upstream); err != nil {
		return nil, err
	}
	out["pool"] = poolName
	out["upstream"] = upstreamID
	return out, nil
}

func normalizeCatalogCredential(provider, name, baseURL, model string) (string, string) {
	id := strings.ToLower(strings.TrimSpace(name))
	base := strings.ToLower(strings.TrimSpace(baseURL))
	switch {
	case id == "minimax-coding-plan" || strings.Contains(base, "api.minimax.io/anthropic") || strings.Contains(base, "api.minimaxi.com/anthropic"):
		return "anthropic_api", model
	case id == "kimi-for-coding" || strings.Contains(base, "api.kimi.com/coding"):
		if strings.TrimSpace(model) == "" || strings.EqualFold(strings.TrimSpace(model), "k2p6") || strings.EqualFold(strings.TrimSpace(model), "k2p5") {
			model = "kimi-for-coding"
		}
		return "anthropic_api", model
	default:
		return provider, model
	}
}

func (s adminCredentialService) SessionCreate(ctx context.Context, req adminapi.SessionCredentialRequest) (map[string]any, error) {
	provider := strings.TrimSpace(req.Provider)
	if provider != "claude_web" && provider != "chatgpt_web" {
		return nil, fmt.Errorf("session provider must be claude_web or chatgpt_web")
	}
	name := strings.Trim(req.Name, "/ ")
	if name == "" {
		name = "default"
	}
	if len(req.Cookies) == 0 {
		return nil, fmt.Errorf("at least one session cookie is required")
	}
	if strings.TrimSpace(req.UserAgent) == "" {
		return nil, fmt.Errorf("user_agent is required")
	}
	credential := sessionadapter.SessionCredential{
		Cookies:        req.Cookies,
		UserAgent:      strings.TrimSpace(req.UserAgent),
		OrganizationID: strings.TrimSpace(req.OrganizationID),
	}
	raw, err := json.Marshal(credential)
	if err != nil {
		return nil, err
	}
	id := "vault://" + provider + "/" + name
	metadata := map[string]string{"provider": provider, "kind": "browser_session", "updated_at": time.Now().UTC().Format(time.RFC3339)}
	if err := s.rt.tokenVault.Put(ctx, id, raw, metadata); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "provider": provider, "ok": true}, nil
}

func (s adminCredentialService) List(ctx context.Context) (map[string]any, error) {
	sessions, err := repos.NewSessions(s.rt.db).List(ctx)
	if err != nil {
		return nil, err
	}
	apiKeyDTOs := make([]map[string]any, 0)
	sessionDTOs := make([]map[string]any, 0, len(sessions))
	attachments := s.credentialAttachments()
	for _, session := range sessions {
		metadata := map[string]any{}
		if strings.TrimSpace(session.MetadataJSON) != "" {
			_ = json.Unmarshal([]byte(session.MetadataJSON), &metadata)
		}
		row := map[string]any{
			"id":                session.ID,
			"provider":          session.Provider,
			"created_at":        formatOptionalTime(session.CreatedAt),
			"last_refreshed_at": formatOptionalTime(session.LastRefreshedAt),
			"expires_at":        formatOptionalTime(session.ExpiresAt),
			"metadata":          metadata,
		}
		kind, _ := metadata["kind"].(string)
		if strings.HasPrefix(session.ID, "vault://apikey/") || kind == "api_key" {
			if provider, ok := metadata["provider"].(string); ok && provider != "" {
				row["provider"] = provider
			}
			if attached := attachments[session.ID]; len(attached) > 0 {
				row["attachments"] = attached
			}
			apiKeyDTOs = append(apiKeyDTOs, row)
			continue
		}
		sessionDTOs = append(sessionDTOs, row)
	}
	return map[string]any{
		"oauth_providers": s.oauthProviderDTOs(),
		"api_keys":        apiKeyDTOs,
		"sessions":        sessionDTOs,
		"cli":             s.cliStatus(),
		"catalog":         s.catalog(ctx),
	}, nil
}

func (s adminCredentialService) oauthProviderDTOs() []map[string]any {
	providers := map[string]oauth.Provider{}
	for id, provider := range oauthProviderTemplates() {
		providers[id] = provider
	}
	for _, provider := range s.rt.oauthReg.List() {
		if existing, ok := providers[provider.ID]; ok {
			provider = mergeOAuthTemplate(existing, provider)
		}
		providers[provider.ID] = provider
	}
	ids := make([]string, 0, len(providers))
	for id := range providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		provider := providers[id]
		missing := missingOAuthProviderFields(provider)
		out = append(out, map[string]any{
			"id":                  provider.ID,
			"display_name":        provider.DisplayName,
			"client_id":           provider.ClientID,
			"browser_bootstrap":   strings.TrimSpace(provider.AuthURL) != "" && strings.TrimSpace(provider.TokenURL) != "",
			"device_bootstrap":    strings.TrimSpace(provider.DeviceAuthURL) != "" && strings.TrimSpace(provider.TokenURL) != "",
			"configured_client":   strings.TrimSpace(provider.ClientID) != "",
			"metadata_configured": oauthMetadataConfigured(provider),
			"usable":              usableOAuthProvider(provider),
			"missing_fields":      missing,
			"auth_url":            provider.AuthURL,
			"token_url":           provider.TokenURL,
			"device_auth_url":     provider.DeviceAuthURL,
			"revoke_url":          provider.RevokeURL,
			"default_scopes":      provider.DefaultScopes,
		})
	}
	return out
}

func (s adminCredentialService) credentialAttachments() map[string][]map[string]any {
	out := map[string][]map[string]any{}
	if s.rt == nil || s.rt.pools == nil {
		return out
	}
	for _, pool := range s.rt.pools.Pools {
		for _, upstream := range pool.Upstreams {
			ref := strings.TrimSpace(stringFromMap(upstream.Config, "api_key_ref"))
			if ref == "" {
				continue
			}
			out[ref] = append(out[ref], map[string]any{
				"pool":        pool.Name,
				"upstream_id": upstream.ID,
				"provider":    upstream.Provider,
				"model":       stringFromMap(upstream.Config, "model"),
				"base_url":    stringFromMap(upstream.Config, "base_url"),
			})
		}
	}
	return out
}

func (s adminCredentialService) OAuthBootstrap(ctx context.Context, provider, name, mode string) (map[string]any, error) {
	if strings.TrimSpace(provider) == "" {
		return nil, fmt.Errorf("oauth provider is required")
	}
	registered, err := s.rt.oauthReg.Get(provider)
	if err != nil {
		return nil, err
	}
	if !usableOAuthProvider(registered) {
		return nil, fmt.Errorf("oauth provider %q is not configured with real endpoints and client_id", provider)
	}
	if mode != "" && mode != "browser" {
		return nil, fmt.Errorf("admin UI supports browser OAuth bootstrap; device flow must be run from the CLI")
	}
	redirectURI := publicHTTPBase(s.rt.cfg.Admin.Bind) + "/admin/v1/credentials/oauth/callback"
	result, err := s.rt.oauthMgr.BeginBrowser(ctx, provider, name, redirectURI)
	if err != nil {
		return nil, err
	}
	s.rt.pendingMu.Lock()
	s.rt.pendingOAuth[result.PKCE.State] = pendingOAuthBootstrap{
		Provider:    provider,
		Name:        name,
		RedirectURI: redirectURI,
		PKCE:        result.PKCE,
		VaultID:     result.VaultID,
		CreatedAt:   time.Now().UTC(),
	}
	s.rt.pendingMu.Unlock()
	return map[string]any{"vault_id": result.VaultID, "mode": result.Mode, "auth_url": result.AuthURL, "redirect_uri": redirectURI, "state": result.PKCE.State}, nil
}

func (s adminCredentialService) OAuthCallback(ctx context.Context, state, code, errorText string) (map[string]any, error) {
	if strings.TrimSpace(errorText) != "" {
		return nil, fmt.Errorf("provider returned oauth error: %s", errorText)
	}
	if strings.TrimSpace(state) == "" || strings.TrimSpace(code) == "" {
		return nil, fmt.Errorf("oauth callback requires state and code")
	}
	s.rt.pendingMu.Lock()
	pending, ok := s.rt.pendingOAuth[state]
	if ok {
		delete(s.rt.pendingOAuth, state)
	}
	s.rt.pendingMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("oauth state is unknown or expired")
	}
	if time.Since(pending.CreatedAt) > 15*time.Minute {
		return nil, fmt.Errorf("oauth state expired")
	}
	token, err := s.rt.oauthMgr.CompleteBrowser(ctx, pending.Provider, pending.Name, code, pending.RedirectURI, pending.PKCE)
	if err != nil {
		return nil, err
	}
	return map[string]any{"vault_id": pending.VaultID, "expires_at": formatOptionalTime(token.ExpiresAt)}, nil
}

func (s adminCredentialService) OAuthRefresh(ctx context.Context, id string) (map[string]any, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("credential id is required")
	}
	token, err := s.rt.oauthMgr.Refresh(ctx, id)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "vault_id": id, "expires_at": formatOptionalTime(token.ExpiresAt)}, nil
}

func (s adminCredentialService) OAuthRevoke(ctx context.Context, id string) (map[string]any, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("credential id is required")
	}
	if err := s.rt.oauthMgr.Revoke(ctx, id); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "vault_id": id}, nil
}

func (s adminCredentialService) OAuthProvidersRaw(context.Context) (map[string]any, error) {
	providersPath := config.ResolveRelative(s.rt.configPath, s.rt.cfg.OAuth.ProvidersFile)
	// #nosec G304 -- OAuth providers path is resolved from trusted local configuration.
	raw, err := os.ReadFile(providersPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		raw = []byte(oauthProvidersTemplate())
	}
	return map[string]any{"path": providersPath, "body": string(raw)}, nil
}

func (s adminCredentialService) OAuthProvidersSave(_ context.Context, body string) (map[string]any, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, fmt.Errorf("oauth provider metadata cannot be empty")
	}
	var doc struct {
		Providers []oauth.Provider `yaml:"providers"`
	}
	if err := yaml.Unmarshal([]byte(body), &doc); err != nil {
		return nil, fmt.Errorf("parse oauth provider metadata: %w", err)
	}
	if _, err := oauth.LoadRegistryFromBytes([]byte(body)); err != nil {
		return nil, err
	}
	providersPath := config.ResolveRelative(s.rt.configPath, s.rt.cfg.OAuth.ProvidersFile)
	if err := os.MkdirAll(filepath.Dir(providersPath), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(providersPath, []byte(body+"\n"), 0o600); err != nil {
		return nil, err
	}
	registry, err := oauth.LoadRegistry(providersPath)
	if err != nil {
		return nil, err
	}
	s.rt.oauthReg = registry
	s.rt.oauthMgr = oauth.NewManager(registry, s.rt.tokenVault, nil)
	if s.rt.events != nil {
		s.rt.events.Publish(events.Event{Type: "oauth_providers_reloaded", Data: map[string]any{"ok": true}})
	}
	return map[string]any{"path": providersPath, "ok": true, "providers": s.oauthProviderDTOs()}, nil
}

func (s adminCredentialService) CLIStatus(context.Context) (map[string]any, error) {
	return s.cliStatus(), nil
}

func (s adminCredentialService) CLIDetect(context.Context) (map[string]any, error) {
	return map[string]any{"agents": s.detectCLI()}, nil
}

func (s adminCredentialService) CLIEnable(_ context.Context, req adminapi.CLIEnableRequest) (map[string]any, error) {
	provider := strings.TrimSpace(req.Provider)
	defaults := cliDefaults()
	defaultSpec, ok := defaults[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported cli provider %q", provider)
	}
	command := strings.TrimSpace(req.Command)
	if command == "" {
		command = defaultSpec.Command
	}
	if _, err := exec.LookPath(command); err != nil {
		return nil, fmt.Errorf("%s is not available on PATH: %w", command, err)
	}
	protocol := strings.TrimSpace(req.Protocol)
	if protocol == "" {
		protocol = defaultSpec.Protocol
	}
	framing := strings.TrimSpace(req.Framing)
	if framing == "" {
		framing = defaultSpec.Framing
	}
	args := req.Args
	if len(args) == 0 {
		args = append([]string{}, defaultSpec.Args...)
	}
	poolName := cleanName(req.Pool, providerPoolName(provider))
	upstreamID := cleanName(req.UpstreamID, provider+"-local")
	upstream := config.Upstream{
		ID:       upstreamID,
		Provider: provider,
		Priority: 1,
		Weight:   100,
		Config: map[string]any{
			"command":  command,
			"protocol": protocol,
			"framing":  framing,
		},
	}
	if len(args) > 0 {
		upstream.Config["args"] = args
	}
	if model := strings.TrimSpace(req.Model); model != "" {
		upstream.Config["model"] = model
	}
	if err := s.upsertUpstream(poolName, upstream); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "provider": provider, "pool": poolName, "upstream": upstreamID}, nil
}

func (s adminCredentialService) ProviderCatalog(ctx context.Context) (map[string]any, error) {
	return s.catalog(ctx), nil
}

func (s adminCredentialService) cliStatus() map[string]any {
	agents := []map[string]any{}
	defaults := cliDefaults()
	for _, pool := range s.rt.pools.Pools {
		for _, upstream := range pool.Upstreams {
			defaultSpec, ok := defaults[upstream.Provider]
			if !ok {
				continue
			}
			command := defaultSpec.Command
			if raw, ok := upstream.Config["command"].(string); ok && strings.TrimSpace(raw) != "" {
				command = raw
			}
			protocol := defaultSpec.Protocol
			if raw, ok := upstream.Config["protocol"].(string); ok && strings.TrimSpace(raw) != "" {
				protocol = raw
			}
			framing := defaultSpec.Framing
			if raw, ok := upstream.Config["framing"].(string); ok && strings.TrimSpace(raw) != "" {
				framing = raw
			}
			args := defaultSpec.Args
			if rawArgs, ok := upstream.Config["args"].([]string); ok {
				args = rawArgs
			} else if anyArgs, ok := upstream.Config["args"].([]any); ok {
				args = stringsFromAny(anyArgs)
			}
			foundPath, lookupErr := exec.LookPath(command)
			agents = append(agents, map[string]any{
				"pool":        pool.Name,
				"upstream":    upstream.ID,
				"provider":    upstream.Provider,
				"name":        defaultSpec.Name,
				"command":     command,
				"protocol":    protocol,
				"framing":     framing,
				"args":        args,
				"path":        foundPath,
				"configured":  true,
				"available":   lookupErr == nil,
				"auth_status": defaultSpec.AuthStatus,
				"source":      defaultSpec.Source,
				"version":     defaultSpec.Version,
				"error":       errorString(lookupErr),
			})
		}
	}
	return map[string]any{
		"enabled":                      s.rt.cfg.CLIAgents.Enabled,
		"default_idle_timeout_seconds": s.rt.cfg.CLIAgents.DefaultIdleTimeoutSeconds,
		"stderr_capture_bytes":         s.rt.cfg.CLIAgents.DefaultStderrCaptureBytes,
		"health_check_interval":        s.rt.cfg.CLIAgents.HealthCheckIntervalSeconds,
		"agents":                       agents,
	}
}

func (s adminCredentialService) detectCLI() []map[string]any {
	out := []map[string]any{}
	for provider, spec := range cliDefaults() {
		foundPath, err := exec.LookPath(spec.Command)
		out = append(out, map[string]any{
			"provider":    provider,
			"name":        spec.Name,
			"command":     spec.Command,
			"protocol":    spec.Protocol,
			"framing":     spec.Framing,
			"args":        spec.Args,
			"path":        foundPath,
			"configured":  s.cliConfigured(provider),
			"available":   err == nil,
			"auth_status": spec.AuthStatus,
			"source":      spec.Source,
			"version":     spec.Version,
			"error":       errorString(err),
		})
	}
	return out
}

func (s adminCredentialService) cliConfigured(provider string) bool {
	for _, pool := range s.rt.pools.Pools {
		for _, upstream := range pool.Upstreams {
			if upstream.Provider == provider {
				return true
			}
		}
	}
	return false
}

func (s adminCredentialService) upsertUpstream(poolName string, upstream config.Upstream) error {
	for i := range s.rt.pools.Pools {
		if s.rt.pools.Pools[i].Name == poolName {
			replaced := false
			for j := range s.rt.pools.Pools[i].Upstreams {
				if s.rt.pools.Pools[i].Upstreams[j].ID == upstream.ID {
					s.rt.pools.Pools[i].Upstreams[j] = upstream
					replaced = true
					break
				}
			}
			if !replaced {
				upstream.Priority = len(s.rt.pools.Pools[i].Upstreams) + 1
				s.rt.pools.Pools[i].Upstreams = append(s.rt.pools.Pools[i].Upstreams, upstream)
			}
			return adminPoolService(s).persist()
		}
	}
	s.rt.pools.Pools = append(s.rt.pools.Pools, config.Pool{
		Name:      poolName,
		Strategy:  "priority_first",
		Upstreams: []config.Upstream{upstream},
	})
	return adminPoolService(s).persist()
}

func (s adminCredentialService) catalog(ctx context.Context) map[string]any {
	providers := localCatalogProviders(s.rt.pools.Pools)
	if remote, err := fetchModelsDev(ctx); err == nil && len(remote) > 0 {
		providers = mergeCatalog(providers, remote)
		return map[string]any{"source": "https://models.dev/api.json", "providers": providers}
	}
	return map[string]any{"source": "built-in", "providers": providers}
}

type adminAuditService struct {
	repo *repos.AuditIndex
}

func (s adminAuditService) Query(ctx context.Context, values map[string][]string) (map[string]any, error) {
	query, err := auditQueryFromValues(values)
	if err != nil {
		return nil, err
	}
	page, err := s.repo.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(page.Entries))
	for _, row := range page.Entries {
		item := map[string]any{
			"request_id":    row.RequestID,
			"time":          formatOptionalTime(row.TS),
			"bridge_key_id": row.BridgeKeyID,
			"pool_name":     row.PoolName,
			"upstream_id":   row.UpstreamID,
			"status":        row.Status,
			"cost_cents":    row.CostCents,
			"audit_file":    row.FilePath,
			"file_offset":   row.FileOffset,
			"file_length":   row.FileLength,
		}
		if record, err := readAuditRecord(row); err == nil {
			item["record"] = record
			mergeAuditRecordFields(item, record)
		} else {
			item["record_error"] = err.Error()
		}
		items = append(items, item)
	}
	return map[string]any{"items": items, "next_cursor": page.NextCursor}, nil
}

func mergeAuditRecordFields(item map[string]any, record map[string]any) {
	for _, key := range []string{
		"ingress_format",
		"model_alias",
		"upstream_provider",
		"upstream_model",
		"input_tokens",
		"output_tokens",
		"cache_read_tokens",
		"cache_write_tokens",
		"latency_ms",
		"ttfb_ms",
		"stream",
		"stop_reason",
		"user_agent",
		"content",
		"error",
	} {
		if value, ok := record[key]; ok {
			item[key] = value
		}
	}
}

func auditQueryFromValues(values map[string][]string) (repos.AuditQuery, error) {
	var query repos.AuditQuery
	var err error
	if raw := firstValue(values, "from"); raw != "" {
		query.From, err = parseAuditQueryTime(raw, false)
		if err != nil {
			return repos.AuditQuery{}, err
		}
	}
	if raw := firstValue(values, "to"); raw != "" {
		query.To, err = parseAuditQueryTime(raw, true)
		if err != nil {
			return repos.AuditQuery{}, err
		}
	}
	query.RequestID = strings.TrimSpace(firstValue(values, "request_id"))
	query.BridgeKeyID = strings.TrimSpace(firstValue(values, "key_id"))
	query.PoolName = strings.TrimSpace(firstValue(values, "pool"))
	query.UpstreamID = strings.TrimSpace(firstValue(values, "upstream_id"))
	query.Status = strings.TrimSpace(firstValue(values, "status"))
	query.Cursor = strings.TrimSpace(firstValue(values, "cursor"))
	if raw := strings.TrimSpace(firstValue(values, "limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit <= 0 {
			return repos.AuditQuery{}, fmt.Errorf("limit must be a positive integer")
		}
		query.Limit = limit
	}
	return query, nil
}

func parseAuditQueryTime(raw string, endOfDay bool) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	if day, err := time.Parse("2006-01-02", raw); err == nil {
		if endOfDay {
			return day.Add(24*time.Hour - time.Nanosecond), nil
		}
		return day, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid audit time %q: use RFC3339 or YYYY-MM-DD", raw)
	}
	return parsed, nil
}

func firstValue(values map[string][]string, key string) string {
	if len(values[key]) == 0 {
		return ""
	}
	return values[key][0]
}

func readAuditRecord(row repos.AuditEntry) (map[string]any, error) {
	if row.FilePath == "" || row.FileLength <= 0 {
		return nil, fmt.Errorf("audit record location is missing")
	}
	file, err := os.Open(row.FilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	section := io.NewSectionReader(file, row.FileOffset, row.FileLength)
	raw, err := io.ReadAll(section)
	if err != nil {
		return nil, err
	}
	record := map[string]any{}
	if err := json.Unmarshal(raw, &record); err != nil {
		return nil, err
	}
	return record, nil
}

type adminBudgetService struct {
	keys     *repos.BridgeKeys
	counters *repos.BudgetCounters
}

func (s adminBudgetService) Budgets(ctx context.Context) (map[string]any, error) {
	keys, err := s.keys.List(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	dailyBucket := now.Format("2006-01-02")
	monthlyBucket := now.Format("2006-01")
	var dailyLimit, monthlyLimit, dailyUsed, monthlyUsed int64
	for _, key := range keys {
		budgets, err := decodeKeyBudgets(key)
		if err != nil {
			return nil, err
		}
		dailyLimit += budgets.DailyCents
		monthlyLimit += budgets.MonthlyCents
		counters, err := s.counters.List(ctx, key.ID)
		if err != nil {
			return nil, err
		}
		for _, counter := range counters {
			if counter.Period == "daily" && counter.Bucket == dailyBucket {
				dailyUsed += counter.Cents
			}
			if counter.Period == "monthly" && counter.Bucket == monthlyBucket {
				monthlyUsed += counter.Cents
			}
		}
	}
	return map[string]any{
		"keys":               len(keys),
		"daily_cents":        dailyLimit,
		"monthly_cents":      monthlyLimit,
		"daily_used_cents":   dailyUsed,
		"monthly_used_cents": monthlyUsed,
	}, nil
}

func (s adminBudgetService) Usage(ctx context.Context) (map[string]any, error) {
	keys, err := s.keys.List(ctx)
	if err != nil {
		return nil, err
	}
	items := []map[string]any{}
	now := time.Now().UTC()
	dailyBucket := now.Format("2006-01-02")
	monthlyBucket := now.Format("2006-01")
	for _, key := range keys {
		budgets, err := decodeKeyBudgets(key)
		if err != nil {
			return nil, err
		}
		counters, err := s.counters.List(ctx, key.ID)
		if err != nil {
			return nil, err
		}
		var dailyUsed, monthlyUsed, totalUsed int64
		for _, counter := range counters {
			totalUsed += counter.Cents
			if counter.Period == "daily" && counter.Bucket == dailyBucket {
				dailyUsed += counter.Cents
			}
			if counter.Period == "monthly" && counter.Bucket == monthlyBucket {
				monthlyUsed += counter.Cents
			}
		}
		items = append(items, map[string]any{
			"key_id":               key.ID,
			"name":                 key.Name,
			"daily_cents":          dailyUsed,
			"monthly_cents":        monthlyUsed,
			"total_cents":          totalUsed,
			"daily_budget_cents":   budgets.DailyCents,
			"monthly_budget_cents": budgets.MonthlyCents,
			"hard_cap":             budgets.HardCap,
		})
	}
	return map[string]any{"items": items}, nil
}

func decodeKeyBudgets(key repos.BridgeKey) (auth.Budgets, error) {
	var budgets auth.Budgets
	raw := key.BudgetsJSON
	if strings.TrimSpace(raw) == "" {
		raw = "{}"
	}
	if err := json.Unmarshal([]byte(raw), &budgets); err != nil {
		return auth.Budgets{}, fmt.Errorf("decode budgets for key %q: %w", key.ID, err)
	}
	return budgets, nil
}

type adminHealthService struct {
	rt        *adminRuntime
	cooldowns *repos.Cooldowns
}

func (s adminHealthService) Detail(ctx context.Context) (map[string]any, error) {
	cooldowns, err := s.cooldowns.List(ctx)
	if err != nil {
		return nil, err
	}
	cooldownByUpstream := map[string]repos.Cooldown{}
	for _, cooldown := range cooldowns {
		cooldownByUpstream[cooldown.UpstreamID] = cooldown
	}
	upstreams := []map[string]any{}
	for _, pool := range s.rt.pools.Pools {
		for _, upstream := range pool.Upstreams {
			state := "configured"
			detail := map[string]any{
				"id":                 upstream.ID,
				"pool":               pool.Name,
				"provider":           upstream.Provider,
				"priority":           upstream.Priority,
				"weight":             upstream.Weight,
				"state":              state,
				"latency_p50_ms":     0,
				"latency_p95_ms":     0,
				"latency_p99_ms":     0,
				"in_flight":          0,
				"last_error":         "",
				"last_error_at":      "",
				"last_success_at":    "",
				"cooldown_until":     "",
				"circuit_open_until": "",
			}
			if cooldown, ok := cooldownByUpstream[upstream.ID]; ok {
				if strings.TrimSpace(cooldown.State) != "" {
					state = cooldown.State
				}
				detail["state"] = state
				detail["consecutive_failures"] = cooldown.ConsecutiveFailures
				detail["last_error"] = cooldown.LastError
				detail["last_error_at"] = formatOptionalTime(cooldown.LastErrorAt)
				detail["last_success_at"] = formatOptionalTime(cooldown.LastSuccessAt)
				detail["cooldown_until"] = formatOptionalTime(cooldown.CooldownUntil)
				detail["circuit_open_until"] = formatOptionalTime(cooldown.CircuitOpenUntil)
				detail["updated_at"] = formatOptionalTime(cooldown.UpdatedAt)
			}
			upstreams = append(upstreams, detail)
		}
	}
	cooldownDTOs := make([]map[string]any, 0, len(cooldowns))
	for _, cooldown := range cooldowns {
		cooldownDTOs = append(cooldownDTOs, map[string]any{
			"upstream_id": cooldown.UpstreamID,
			"pool":        cooldown.PoolName,
			"state":       cooldown.State,
			"last_error":  cooldown.LastError,
			"updated_at":  formatOptionalTime(cooldown.UpdatedAt),
		})
	}
	return map[string]any{"upstreams": upstreams, "cooldowns": cooldownDTOs}, nil
}

type adminReloadService struct {
	rt *adminRuntime
}

func (s adminReloadService) Reload(context.Context) (adminapi.ReloadResult, error) {
	nextCfg, err := config.Load(s.rt.configPath)
	if err != nil {
		return adminapi.ReloadResult{}, err
	}
	if fields := restartRequiredFields(s.rt.cfg, nextCfg); len(fields) > 0 {
		return adminapi.ReloadResult{OK: false, RestartRequiredFields: fields}, nil
	}
	nextPoolsPath := config.ResolveRelative(s.rt.configPath, nextCfg.PoolsFile)
	pools, err := config.LoadPools(nextPoolsPath, nextCfg.Vault.MasterKeyEnv)
	if err != nil {
		return adminapi.ReloadResult{}, err
	}
	tokenStore, err := auth.LoadAdminTokens(config.ResolveRelative(s.rt.configPath, nextCfg.Admin.TokensFile))
	if err != nil {
		return adminapi.ReloadResult{}, err
	}
	oauthReg, err := oauth.LoadRegistry(config.ResolveRelative(s.rt.configPath, nextCfg.OAuth.ProvidersFile))
	if err != nil {
		return adminapi.ReloadResult{}, err
	}
	oauthMgr := oauth.NewManager(oauthReg, s.rt.tokenVault, nil)
	registry, err := builtins.RegistryWithAuth(oauthMgr, s.rt.tokenVault)
	if err != nil {
		return adminapi.ReloadResult{}, err
	}
	nextRouter, nextModels, err := RouterFromConfigPools(pools.Pools, registry)
	if err != nil {
		return adminapi.ReloadResult{}, err
	}
	s.rt.cfg = nextCfg
	s.rt.poolsPath = nextPoolsPath
	s.rt.pools = pools
	s.rt.tokenStore = tokenStore
	s.rt.oauthReg = oauthReg
	s.rt.oauthMgr = oauthMgr
	s.rt.registry = registry
	if s.rt.liveRouter != nil {
		s.rt.liveRouter.Set(nextRouter, nextModels)
	}
	if s.rt.events != nil {
		s.rt.events.Publish(events.Event{Type: "reload", Data: map[string]any{"ok": true}})
	}
	return adminapi.ReloadResult{OK: true}, nil
}

func restartRequiredFields(oldCfg, nextCfg *config.Config) []string {
	if oldCfg == nil || nextCfg == nil {
		return nil
	}
	fields := []string{}
	if oldCfg.Server.Bind != nextCfg.Server.Bind {
		fields = append(fields, "server.bind")
	}
	if oldCfg.Admin.Bind != nextCfg.Admin.Bind {
		fields = append(fields, "admin.bind")
	}
	if oldCfg.Storage.Path != nextCfg.Storage.Path {
		fields = append(fields, "storage.path")
	}
	if oldCfg.Vault.MasterKeyEnv != nextCfg.Vault.MasterKeyEnv {
		fields = append(fields, "vault.master_key_env")
	}
	return fields
}

func keyDTO(row repos.BridgeKey) (adminapi.KeyDTO, error) {
	scopes, err := decodeObject(row.ScopesJSON)
	if err != nil {
		return adminapi.KeyDTO{}, fmt.Errorf("decode scopes for key %q: %w", row.ID, err)
	}
	budgets, err := decodeObject(row.BudgetsJSON)
	if err != nil {
		return adminapi.KeyDTO{}, fmt.Errorf("decode budgets for key %q: %w", row.ID, err)
	}
	rateLimits, err := decodeObject(row.RateLimitsJSON)
	if err != nil {
		return adminapi.KeyDTO{}, fmt.Errorf("decode rate limits for key %q: %w", row.ID, err)
	}
	metadata, err := decodeObject(row.MetadataJSON)
	if err != nil {
		return adminapi.KeyDTO{}, fmt.Errorf("decode metadata for key %q: %w", row.ID, err)
	}
	return adminapi.KeyDTO{
		ID:         row.ID,
		Hash:       row.Hash,
		Name:       row.Name,
		CreatedAt:  formatOptionalTime(row.CreatedAt),
		CreatedBy:  row.CreatedBy,
		LastUsedAt: formatOptionalTime(row.LastUsedAt),
		RevokedAt:  formatOptionalTime(row.RevokedAt),
		Scopes:     scopes,
		Budgets:    budgets,
		RateLimits: rateLimits,
		Metadata:   metadata,
	}, nil
}

func poolDTO(pool config.Pool) adminapi.PoolDTO {
	upstreams := make([]map[string]any, 0, len(pool.Upstreams))
	for _, upstream := range pool.Upstreams {
		row := map[string]any{
			"id":       upstream.ID,
			"provider": upstream.Provider,
			"priority": upstream.Priority,
			"weight":   upstream.Weight,
		}
		for key, value := range upstream.Config {
			row[key] = value
		}
		upstreams = append(upstreams, row)
	}
	return adminapi.PoolDTO{ID: pool.Name, Strategy: pool.Strategy, Upstreams: upstreams}
}

func dtoPool(dto adminapi.PoolDTO) config.Pool {
	upstreams := make([]config.Upstream, 0, len(dto.Upstreams))
	for _, row := range dto.Upstreams {
		upstream := config.Upstream{Config: map[string]any{}}
		if value, ok := row["id"].(string); ok {
			upstream.ID = value
		}
		if value, ok := row["provider"].(string); ok {
			upstream.Provider = value
		}
		upstream.Priority = intFromAny(row["priority"])
		upstream.Weight = intFromAny(row["weight"])
		for key, value := range row {
			if key != "id" && key != "provider" && key != "priority" && key != "weight" {
				upstream.Config[key] = value
			}
		}
		upstreams = append(upstreams, upstream)
	}
	return config.Pool{Name: dto.ID, Strategy: dto.Strategy, Upstreams: upstreams}
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func mustJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func decodeObject(raw string) (map[string]any, error) {
	out := map[string]any{}
	if strings.TrimSpace(raw) == "" {
		return out, nil
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func randomID(prefix string) (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(raw[:]), nil
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func usableOAuthProvider(provider oauth.Provider) bool {
	if strings.TrimSpace(provider.ID) == "" || strings.TrimSpace(provider.ClientID) == "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(provider.ClientID), "sigilbridge") {
		return false
	}
	for _, rawURL := range []string{provider.AuthURL, provider.TokenURL, provider.DeviceAuthURL, provider.RevokeURL} {
		if strings.Contains(rawURL, "example.invalid") || strings.Contains(rawURL, "provider.example") {
			return false
		}
	}
	return provider.AuthURL != "" && provider.TokenURL != ""
}

func oauthMetadataConfigured(provider oauth.Provider) bool {
	return strings.TrimSpace(provider.ID) != "" &&
		strings.TrimSpace(provider.AuthURL) != "" &&
		strings.TrimSpace(provider.TokenURL) != ""
}

func missingOAuthProviderFields(provider oauth.Provider) []string {
	missing := []string{}
	if strings.TrimSpace(provider.AuthURL) == "" {
		missing = append(missing, "auth_url")
	}
	if strings.TrimSpace(provider.TokenURL) == "" {
		missing = append(missing, "token_url")
	}
	if strings.TrimSpace(provider.ClientID) == "" {
		missing = append(missing, "client_id")
	}
	return missing
}

func mergeOAuthTemplate(base, override oauth.Provider) oauth.Provider {
	if override.ID == "" {
		override.ID = base.ID
	}
	if override.DisplayName == "" {
		override.DisplayName = base.DisplayName
	}
	if override.AuthURL == "" {
		override.AuthURL = base.AuthURL
	}
	if override.TokenURL == "" {
		override.TokenURL = base.TokenURL
	}
	if override.DeviceAuthURL == "" {
		override.DeviceAuthURL = base.DeviceAuthURL
	}
	if override.RevokeURL == "" {
		override.RevokeURL = base.RevokeURL
	}
	if override.DefaultScopes == nil {
		override.DefaultScopes = base.DefaultScopes
	}
	return override
}

func oauthProviderTemplates() map[string]oauth.Provider {
	return map[string]oauth.Provider{
		"claude_oauth": {
			ID:          "claude_oauth",
			DisplayName: "Claude",
		},
		// #nosec G101 -- public OAuth endpoint metadata, not hardcoded credentials.
		"copilot_oauth": {
			ID:            "copilot_oauth",
			DisplayName:   "GitHub Copilot",
			AuthURL:       "https://github.com/login/oauth/authorize",
			TokenURL:      "https://github.com/login/oauth/access_token",
			DeviceAuthURL: "https://github.com/login/device/code",
			DefaultScopes: []string{"read:user"},
		},
		"cursor_oauth": {
			ID:          "cursor_oauth",
			DisplayName: "Cursor",
		},
		// #nosec G101 -- public OAuth endpoint metadata, not hardcoded credentials.
		"gemini_oauth": {
			ID:            "gemini_oauth",
			DisplayName:   "Google Gemini",
			AuthURL:       "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:      "https://oauth2.googleapis.com/token",
			DeviceAuthURL: "https://oauth2.googleapis.com/device/code",
			RevokeURL:     "https://oauth2.googleapis.com/revoke",
			DefaultScopes: []string{"openid", "email", "profile"},
		},
	}
}

func apiKeyProviderDefaults() map[string]struct {
	Name    string
	BaseURL string
	Env     []string
} {
	return map[string]struct {
		Name    string
		BaseURL string
		Env     []string
	}{
		"anthropic_api": {Name: "Anthropic", BaseURL: "https://api.anthropic.com", Env: []string{"ANTHROPIC_API_KEY"}},
		"openai_api":    {Name: "OpenAI", BaseURL: "https://api.openai.com", Env: []string{"OPENAI_API_KEY"}},
		"groq":          {Name: "Groq", BaseURL: "https://api.groq.com/openai", Env: []string{"GROQ_API_KEY"}},
		"gemini_api":    {Name: "Google Gemini", BaseURL: "https://generativelanguage.googleapis.com", Env: []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"}},
		"mistral_api":   {Name: "Mistral", BaseURL: "https://api.mistral.ai", Env: []string{"MISTRAL_API_KEY"}},
		"deepseek_api":  {Name: "DeepSeek", BaseURL: "https://api.deepseek.com", Env: []string{"DEEPSEEK_API_KEY"}},
	}
}

type cliDefault struct {
	Command    string
	Name       string
	Protocol   string
	Framing    string
	Args       []string
	AuthStatus string
	Source     string
	Version    string
}

func cliDefaults() map[string]cliDefault {
	out := map[string]cliDefault{}
	for _, agent := range cliacpadapter.Defaults() {
		out[agent.ID] = cliDefault{
			Command:    agent.Command,
			Name:       agent.Name,
			Protocol:   agent.Protocol,
			Framing:    agent.Framing,
			Args:       agent.Args,
			AuthStatus: agent.AuthStatus,
			Source:     agent.Source,
			Version:    agent.Version,
		}
	}
	return out
}

func providerPoolName(provider string) string {
	return strings.TrimSuffix(strings.TrimSuffix(provider, "_api"), "_cli")
}

func cleanName(value, fallback string) string {
	value = strings.Trim(strings.ToLower(value), "/ ")
	if value == "" {
		value = fallback
	}
	replacer := strings.NewReplacer(" ", "-", "_", "-", "\\", "-", "/", "-")
	return strings.Trim(replacer.Replace(value), "-")
}

func localCatalogProviders(pools []config.Pool) []map[string]any {
	configured := map[string]bool{}
	for _, pool := range pools {
		for _, upstream := range pool.Upstreams {
			configured[upstream.Provider] = true
		}
	}
	out := []map[string]any{}
	for id, item := range apiKeyProviderDefaults() {
		out = append(out, map[string]any{"id": id, "name": item.Name, "provider": id, "category": "api_key", "base_url": item.BaseURL, "env": item.Env, "configured": configured[id]})
	}
	for provider, spec := range cliDefaults() {
		path, err := exec.LookPath(spec.Command)
		out = append(out, map[string]any{"id": provider, "name": valueOr(spec.Name, cliDisplayName(provider)), "provider": provider, "category": "cli_acp", "configured": configured[provider], "available": err == nil, "command": spec.Command, "protocol": spec.Protocol, "framing": spec.Framing, "args": spec.Args, "auth_status": spec.AuthStatus, "source": spec.Source, "version": spec.Version, "path": path})
	}
	return out
}

func cliDisplayName(provider string) string {
	switch provider {
	case "claude_code_cli":
		return "Claude Code"
	case "codex_cli":
		return "Codex CLI"
	case "gemini_cli":
		return "Gemini CLI"
	case "aider_cli":
		return "Aider"
	default:
		return provider
	}
}

func fetchModelsDev(ctx context.Context) ([]map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://models.dev/api.json", nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpclient.Default().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("models.dev returned %s", resp.Status)
	}
	var raw map[string]struct {
		ID     string   `json:"id"`
		Name   string   `json:"name"`
		API    string   `json:"api"`
		Env    []string `json:"env"`
		NPM    string   `json:"npm"`
		Models map[string]struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			LastUpdated string `json:"last_updated"`
			Limit       struct {
				Context int `json:"context"`
				Output  int `json:"output"`
			} `json:"limit"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(raw))
	for id, item := range raw {
		provider := mapModelsDevProvider(id, item.NPM)
		top := make([]map[string]any, 0, 32)
		if id == "kimi-for-coding" {
			top = append(top, map[string]any{"id": "kimi-for-coding", "name": "Kimi For Coding"})
		}
		models := make([]struct {
			ID          string
			Name        string
			LastUpdated string
			Context     int
			Output      int
		}, 0, len(item.Models))
		for _, model := range item.Models {
			if id == "kimi-for-coding" && model.ID != "kimi-for-coding" {
				continue
			}
			models = append(models, struct {
				ID          string
				Name        string
				LastUpdated string
				Context     int
				Output      int
			}{ID: model.ID, Name: model.Name, LastUpdated: model.LastUpdated, Context: model.Limit.Context, Output: model.Limit.Output})
		}
		sort.Slice(models, func(i, j int) bool {
			if models[i].LastUpdated != models[j].LastUpdated {
				return models[i].LastUpdated > models[j].LastUpdated
			}
			return models[i].ID < models[j].ID
		})
		for _, model := range models {
			top = append(top, map[string]any{"id": model.ID, "name": model.Name, "updated_at": model.LastUpdated, "context": model.Context, "output": model.Output})
		}
		out = append(out, map[string]any{"id": id, "name": valueOr(item.Name, id), "provider": provider, "category": "api_key", "base_url": item.API, "env": item.Env, "model_count": len(item.Models), "top_models": top})
	}
	return out, nil
}

func mergeCatalog(local, remote []map[string]any) []map[string]any {
	byID := map[string]map[string]any{}
	for _, item := range remote {
		byID[fmt.Sprint(item["id"])] = item
	}
	for _, item := range local {
		id := fmt.Sprint(item["id"])
		if existing, ok := byID[id]; ok {
			for key, value := range item {
				if key == "configured" || key == "available" || existing[key] == nil || existing[key] == "" {
					existing[key] = value
				}
			}
			continue
		}
		byID[id] = item
	}
	out := make([]map[string]any, 0, len(byID))
	for _, item := range byID {
		out = append(out, item)
	}
	return out
}

func mapModelsDevProvider(id, npm string) string {
	switch id {
	case "anthropic":
		return "anthropic_api"
	case "openai":
		return "openai_api"
	case "google", "google-generative-ai", "gemini":
		return "gemini_api"
	case "mistral":
		return "mistral_api"
	case "deepseek":
		return "deepseek_api"
	case "groq":
		return "groq"
	}
	switch strings.TrimSpace(npm) {
	case "@ai-sdk/anthropic":
		return "anthropic_api"
	default:
		return "openai_api"
	}
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func stringsFromAny(values []any) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if text := strings.TrimSpace(fmt.Sprint(value)); text != "" {
			out = append(out, text)
		}
	}
	return out
}
