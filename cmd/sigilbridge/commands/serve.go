package commands

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/adapter/builtins"
	adminapi "github.com/sigilbridge/sigilbridge/internal/admin"
	adminui "github.com/sigilbridge/sigilbridge/internal/admin/ui"
	"github.com/sigilbridge/sigilbridge/internal/audit"
	"github.com/sigilbridge/sigilbridge/internal/auth"
	"github.com/sigilbridge/sigilbridge/internal/budget"
	"github.com/sigilbridge/sigilbridge/internal/config"
	"github.com/sigilbridge/sigilbridge/internal/events"
	"github.com/sigilbridge/sigilbridge/internal/ingress"
	"github.com/sigilbridge/sigilbridge/internal/ir"
	"github.com/sigilbridge/sigilbridge/internal/pricing"
	"github.com/sigilbridge/sigilbridge/internal/router"
	"github.com/sigilbridge/sigilbridge/internal/storage"
	"github.com/sigilbridge/sigilbridge/internal/storage/repos"
)

func Serve(ctx context.Context, server *http.Server) error {
	return ServeServers(ctx, server)
}

func ServeServers(ctx context.Context, servers ...*http.Server) error {
	errCh := make(chan error, 1)
	for _, server := range servers {
		server := server
		go func() {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				errCh <- err
				return
			}
			errCh <- nil
		}()
	}
	select {
	case err := <-errCh:
		shutdownServers(servers...)
		return err
	case <-ctx.Done():
		return shutdownServers(servers...)
	}
}

func shutdownServers(servers ...*http.Server) error {
	var first error
	for _, server := range servers {
		if err := server.Shutdown(context.Background()); err != nil && first == nil {
			first = err
		}
	}
	return first
}

type ServeOption func(*serveOptions)

type serveOptions struct {
	reload <-chan struct{}
}

func WithReloadTrigger(reload <-chan struct{}) ServeOption {
	return func(opts *serveOptions) {
		opts.reload = reload
	}
}

func ServeConfig(ctx context.Context, configPath string, options ...ServeOption) error {
	opts := serveOptions{}
	for _, option := range options {
		option(&opts)
	}
	explicitConfig := configPath != ""
	if configPath == "" {
		configPath = DefaultConfigPath()
	}
	if !explicitConfig {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			result, err := InitConfig(filepath.Dir(configPath), false)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "initialized local SigilBridge config at %s\nadmin token: %s\n", result.ConfigPath, result.AdminToken)
		} else if err != nil {
			return err
		}
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	dbPath := config.ResolveRelative(configPath, cfg.Storage.Path)
	if err := ensureParentDir(dbPath); err != nil {
		return err
	}
	db, err := storage.OpenDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := storage.Up(db); err != nil {
		return err
	}
	var auditWriter *audit.Writer
	if cfg.Audit.Enabled {
		indexer := audit.NewIndexer(db)
		writer, err := audit.NewWriter(config.ResolveRelative(configPath, cfg.Audit.Path), indexer)
		if err != nil {
			_ = indexer.Close()
			return err
		}
		auditWriter = writer
		defer func() {
			_ = writer.Close()
			_ = indexer.Close()
		}()
	}
	poolsPath := config.ResolveRelative(configPath, cfg.PoolsFile)
	poolsFile, err := config.LoadPools(poolsPath, cfg.Vault.MasterKeyEnv)
	if err != nil {
		return err
	}
	eventBus := events.NewBus()
	adminRuntime, err := newAdminRuntime(db, configPath, cfg, poolsPath, poolsFile, eventBus)
	if err != nil {
		return err
	}
	defer adminRuntime.close()
	registry, err := builtins.RegistryWithAuth(adminRuntime.oauthMgr, adminRuntime.tokenVault)
	if err != nil {
		return err
	}
	r, models, err := RouterFromConfigPools(poolsFile.Pools, registry)
	if err != nil {
		return err
	}
	live := newLiveRouter(r, models)
	adminRuntime.registry = registry
	adminRuntime.liveRouter = live
	adminServer := adminapi.New(adminRuntime.services())
	keyStore := auth.NewBridgeKeyStore(db, auth.NewCache(auth.DefaultCacheSize, auth.DefaultCacheTTL, time.Now))
	keyRepo := repos.NewBridgeKeys(db)
	rateLimiter := budget.NewRateLimiter(db, time.Now)
	budgetTracker := budget.NewTracker(db, time.Now)
	pricingTable, err := pricing.LoadWithOverride("")
	if err != nil {
		return err
	}
	var keyCache sync.Map
	ingressServer := ingress.New(live).WithModels(models).WithAuth(func(req *http.Request) (string, error) {
		token := bridgeToken(req)
		if token == "" {
			return "", auth.ErrInvalidBridgeKey
		}
		key, err := keyStore.Validate(req.Context(), token)
		if err != nil {
			return "", err
		}
		keyCache.Store(key.ID, *key)
		_ = keyRepo.MarkUsed(req.Context(), key.ID, time.Now().UTC())
		return key.ID, nil
	}).WithRateLimit(func(req *http.Request, keyID string) error {
		key, ok := cachedBridgeKey(&keyCache, keyID)
		if !ok || key.RateLimits.RPM <= 0 {
			return nil
		}
		return rateLimiter.Allow(req.Context(), keyID, key.RateLimits.RPM, 0, 0)
	}).WithBudget(func(req *http.Request, keyID string, irReq ir.Request) error {
		key, ok := cachedBridgeKey(&keyCache, keyID)
		if !ok {
			return nil
		}
		if key.RateLimits.TPM > 0 {
			tokens, _ := budget.EstimateInputTokens(irReq, "", irReq.ModelAlias)
			if err := rateLimiter.Allow(req.Context(), keyID, 0, key.RateLimits.TPM, int64(tokens)); err != nil {
				return err
			}
		}
		_, err := budgetTracker.PreCheck(req.Context(), key, 0)
		return err
	}).WithObserver(func(req *http.Request, irReq ir.Request, resp ir.Response, dispatchErr error, latency time.Duration) {
		costCents := actualCostCents(pricingTable, resp)
		if dispatchErr == nil && costCents > 0 {
			_ = budgetTracker.Commit(context.Background(), irReq.BridgeKeyID, costCents)
		}
		if auditWriter != nil {
			_ = auditWriter.Write(context.Background(), auditRecord(req, irReq, resp, dispatchErr, latency, costCents, audit.ContentMode(cfg.Audit.ContentMode)))
		}
	})
	ingressMux := http.NewServeMux()
	ingressMux.Handle("/", ingressServer.Handler())
	servers := []*http.Server{{Addr: cfg.Server.Bind, Handler: ingressMux, ReadHeaderTimeout: 5 * time.Second}}
	if cfg.Admin.UIEnabled {
		if sameBind(cfg.Server.Bind, cfg.Admin.Bind) {
			mountAdmin(ingressMux, adminServer.Handler(), false)
		} else {
			adminMux := http.NewServeMux()
			mountAdmin(adminMux, adminServer.Handler(), true)
			servers = append(servers, &http.Server{Addr: cfg.Admin.Bind, Handler: adminMux, ReadHeaderTimeout: 5 * time.Second})
		}
	}
	startReloadLoop(ctx, os.Stderr, adminRuntime, opts.reload)
	printServeURLs(os.Stderr, cfg.Server.Bind, cfg.Admin.Bind, cfg.Admin.UIEnabled)
	return ServeServers(ctx, servers...)
}

func startReloadLoop(ctx context.Context, out *os.File, rt *adminRuntime, reload <-chan struct{}) {
	if reload == nil {
		return
	}
	go func() {
		service := adminReloadService{rt: rt}
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-reload:
				if !ok {
					return
				}
				result, err := service.Reload(ctx)
				if err != nil {
					fmt.Fprintf(out, "reload failed: %v\n", err)
					continue
				}
				if !result.OK {
					fmt.Fprintf(out, "reload requires restart: %s\n", strings.Join(result.RestartRequiredFields, ", "))
					continue
				}
				fmt.Fprintln(out, "reload complete")
			}
		}
	}()
}

type liveRouter struct {
	mu     sync.RWMutex
	router *router.Router
	models []string
}

func newLiveRouter(router *router.Router, models []string) *liveRouter {
	live := &liveRouter{}
	live.Set(router, models)
	return live
}

func (l *liveRouter) Set(next *router.Router, models []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.router = next
	l.models = append([]string(nil), models...)
}

func (l *liveRouter) Dispatch(ctx context.Context, req ir.Request) (ir.Response, error) {
	l.mu.RLock()
	next := l.router
	l.mu.RUnlock()
	if next == nil {
		return ir.Response{}, fmt.Errorf("router not ready")
	}
	return next.Dispatch(ctx, req)
}

func (l *liveRouter) Stream(ctx context.Context, req ir.Request) (<-chan ir.Event, error) {
	l.mu.RLock()
	next := l.router
	l.mu.RUnlock()
	if next == nil {
		return nil, fmt.Errorf("router not ready")
	}
	return next.Stream(ctx, req)
}

func (l *liveRouter) Models() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return append([]string(nil), l.models...)
}

func mountAdmin(mux *http.ServeMux, apiHandler http.Handler, redirectRoot bool) {
	mux.Handle("/admin/v1/", adminSecurityHeaders(apiHandler))
	uiHandler := adminSecurityHeaders(adminui.Handler())
	mux.Handle("/assets/", uiHandler)
	mux.Handle("/admin/ui/", http.StripPrefix("/admin/ui", uiHandler))
	mux.HandleFunc("/admin/ui", func(w http.ResponseWriter, r *http.Request) {
		setAdminSecurityHeaders(w.Header())
		http.Redirect(w, r, "/admin/ui/", http.StatusMovedPermanently)
	})
	if redirectRoot {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			setAdminSecurityHeaders(w.Header())
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			http.Redirect(w, r, "/admin/ui/", http.StatusMovedPermanently)
		})
	}
}

func adminSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setAdminSecurityHeaders(w.Header())
		next.ServeHTTP(w, r)
	})
}

func setAdminSecurityHeaders(header http.Header) {
	header.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'")
	header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "DENY")
}

func mountAdminUI(mux *http.ServeMux, redirectRoot bool) {
	mountAdmin(mux, http.NotFoundHandler(), redirectRoot)
}

func printServeURLs(w interface{ Write([]byte) (int, error) }, apiBind, adminBind string, uiEnabled bool) {
	apiBase := publicHTTPBase(apiBind)
	fmt.Fprintf(w, "SigilBridge API listening on %s\n", apiBase)
	fmt.Fprintf(w, "OpenAI-compatible API: %s/v1\n", apiBase)
	if uiEnabled {
		fmt.Fprintf(w, "Admin UI: %s/admin/ui/\n", publicHTTPBase(adminBind))
	}
}

func publicHTTPBase(bind string) string {
	host, port, err := strings.Cut(bind, ":")
	if !err {
		return "http://" + bind
	}
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "127.0.0.1"
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return "http://" + host + ":" + port
}

func sameBind(left, right string) bool {
	return strings.TrimSpace(left) == strings.TrimSpace(right)
}

func cachedBridgeKey(cache *sync.Map, id string) (auth.BridgeKey, bool) {
	value, ok := cache.Load(id)
	if !ok {
		return auth.BridgeKey{}, false
	}
	key, ok := value.(auth.BridgeKey)
	return key, ok
}

func actualCostCents(table *pricing.Table, resp ir.Response) int64 {
	if resp.CostCents > 0 {
		return int64(resp.CostCents)
	}
	if table == nil || resp.UpstreamProvider == "" {
		return 0
	}
	model := resp.UpstreamModel
	if model == "" {
		model = "default"
	}
	usage := pricing.Usage{
		InputTokens:      int64(resp.Usage.InputTokens),
		OutputTokens:     int64(resp.Usage.OutputTokens),
		CacheReadTokens:  int64(resp.Usage.CacheReadTokens),
		CacheWriteTokens: int64(resp.Usage.CacheWriteTokens),
	}
	cost, err := table.CostCents(resp.UpstreamProvider, model, usage)
	if err == nil {
		return cost
	}
	if model != "default" {
		if cost, err := table.CostCents(resp.UpstreamProvider, "default", usage); err == nil {
			return cost
		}
	}
	return 0
}

func auditRecord(req *http.Request, irReq ir.Request, resp ir.Response, dispatchErr error, latency time.Duration, costCents int64, mode audit.ContentMode) audit.Record {
	status := "ok"
	var recordErr *audit.RecordError
	if dispatchErr != nil {
		status = "error"
		recordErr = &audit.RecordError{Type: "dispatch", Message: dispatchErr.Error()}
	}
	content, _ := audit.CaptureContent(mode, requestText(irReq), responseText(resp))
	return audit.Record{
		TS:               time.Now().UTC(),
		RequestID:        irReq.ID,
		BridgeKeyID:      irReq.BridgeKeyID,
		IngressFormat:    irReq.IngressFormat,
		ModelAlias:       irReq.ModelAlias,
		UpstreamProvider: resp.UpstreamProvider,
		UpstreamID:       resp.UpstreamID,
		UpstreamModel:    resp.UpstreamModel,
		InputTokens:      resp.Usage.InputTokens,
		OutputTokens:     resp.Usage.OutputTokens,
		CacheReadTokens:  resp.Usage.CacheReadTokens,
		CacheWriteTokens: resp.Usage.CacheWriteTokens,
		CostCents:        int(costCents),
		LatencyMs:        latency.Milliseconds(),
		TTFBMs:           resp.TTFBMs,
		Stream:           irReq.Stream,
		StopReason:       resp.StopReason,
		Status:           status,
		Error:            recordErr,
		UserAgent:        req.UserAgent(),
		Content:          content,
	}
}

func requestText(req ir.Request) string {
	var b strings.Builder
	if req.System != "" {
		b.WriteString(req.System)
		b.WriteByte('\n')
	}
	for _, msg := range req.Messages {
		for _, block := range msg.Content {
			b.WriteString(block.Text)
			if block.ToolUse != nil {
				b.WriteString(block.ToolUse.Name)
			}
			if block.ToolResult != nil {
				for _, resultBlock := range block.ToolResult.Content {
					b.WriteString(resultBlock.Text)
				}
			}
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func responseText(resp ir.Response) string {
	var b strings.Builder
	for _, block := range resp.Content {
		b.WriteString(block.Text)
		if block.ToolUse != nil {
			b.WriteString(block.ToolUse.Name)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func RouterFromConfigPools(configPools []config.Pool, registry *adapter.Registry) (*router.Router, []string, error) {
	if registry == nil {
		return nil, nil, fmt.Errorf("adapter registry is required")
	}
	routerPools := make([]router.Pool, 0, len(configPools))
	models := make([]string, 0, len(configPools))
	seenModels := map[string]bool{}
	for _, pool := range configPools {
		if pool.Name == "" {
			return nil, nil, fmt.Errorf("pool name is required")
		}
		upstreams := make([]router.Upstream, 0, len(pool.Upstreams))
		for _, upstream := range pool.Upstreams {
			provider, err := registry.Get(upstream.Provider)
			if err != nil {
				return nil, nil, fmt.Errorf("pool %q upstream %q: %w", pool.Name, upstream.ID, err)
			}
			weight := upstream.Weight
			if weight <= 0 {
				weight = 1
			}
			upstreams = append(upstreams, router.Upstream{
				ID:       upstream.ID,
				Priority: upstream.Priority,
				Weight:   weight,
				Provider: provider,
				Config:   adapter.ProviderConfig{UpstreamID: upstream.ID, Raw: upstream.Config},
			})
		}
		strategy := pool.Strategy
		if strategy == "" {
			strategy = "priority_first"
		}
		aliases := poolModelAliases(pool)
		routerPools = append(routerPools, router.Pool{
			ID:           pool.Name,
			ModelAliases: aliases,
			Strategy:     strategy,
			Upstreams:    upstreams,
		})
		for _, alias := range aliases {
			if alias != "" && !seenModels[alias] {
				models = append(models, alias)
				seenModels[alias] = true
			}
		}
	}
	return router.New(routerPools), models, nil
}

func poolModelAliases(pool config.Pool) []string {
	aliases := []string{pool.Name}
	seen := map[string]bool{pool.Name: true}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			aliases = append(aliases, value)
			seen[value] = true
		}
	}
	for _, upstream := range pool.Upstreams {
		add(stringFromMap(upstream.Config, "model"))
		switch raw := upstream.Config["model_aliases"].(type) {
		case []string:
			for _, alias := range raw {
				add(alias)
			}
		case []any:
			for _, alias := range raw {
				if s, ok := alias.(string); ok {
					add(s)
				}
			}
		}
	}
	return aliases
}

func stringFromMap(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, _ := values[key].(string)
	return value
}

func bridgeToken(req *http.Request) string {
	if token := strings.TrimSpace(req.Header.Get("x-api-key")); token != "" {
		return token
	}
	authz := strings.TrimSpace(req.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		return strings.TrimSpace(authz[len("bearer "):])
	}
	return ""
}

func DefaultConfigPath() string {
	candidates := []string{"config.yaml"}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, ".sigilbridge", "config.yaml"))
	}
	candidates = append(candidates, filepath.Join(string(filepath.Separator), "etc", "sigilbridge", "config.yaml"))
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "config.yaml"
}
