package admin

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/events"
)

const maxAdminJSONBodyBytes int64 = 1 << 20

type Services struct {
	Auth        AuthService
	Keys        KeyService
	Pools       PoolService
	Endpoints   EndpointService
	Chat        ChatService
	Credentials CredentialService
	Audit       AuditService
	Budgets     BudgetService
	Health      HealthService
	Reload      ReloadService
	Events      *events.Bus
}

type Server struct {
	services Services
	mux      *http.ServeMux
	rl       *adminRateLimiter
}

type adminRateLimiter struct {
	mu         sync.Mutex
	lastBucket map[string]int64
	counts     map[string]int64
}

func newAdminRateLimiter() *adminRateLimiter {
	return &adminRateLimiter{lastBucket: map[string]int64{}, counts: map[string]int64{}}
}

func (rl *adminRateLimiter) Allow(key string, limit int64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now().Unix() / 60
	if rl.lastBucket[key] != now {
		rl.lastBucket[key] = now
		rl.counts[key] = 0
	}
	rl.counts[key]++
	return rl.counts[key] <= limit
}

func New(services Services) *Server {
	s := &Server{services: services, mux: http.NewServeMux(), rl: newAdminRateLimiter()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	s.mux.HandleFunc("/admin/v1/auth/login", s.login)
	s.mux.HandleFunc("/admin/v1/auth/logout", s.logout)
	s.mux.HandleFunc("/admin/v1/keys", s.authenticate(s.keys))
	s.mux.HandleFunc("/admin/v1/keys/", s.authenticate(s.keyDetail))
	s.mux.HandleFunc("/admin/v1/endpoints", s.authenticate(s.endpoints))
	s.mux.HandleFunc("/admin/v1/pools", s.authenticate(s.pools))
	s.mux.HandleFunc("/admin/v1/pools/", s.authenticate(s.poolDetail))
	s.mux.HandleFunc("/admin/v1/chat/test", s.authenticate(s.chatTest))
	s.mux.HandleFunc("/admin/v1/credentials", s.authenticate(s.credentials))
	s.mux.HandleFunc("/admin/v1/credentials/api-key", s.authenticate(s.apiKeyCreate))
	s.mux.HandleFunc("/admin/v1/credentials/session", s.authenticate(s.sessionCreate))
	s.mux.HandleFunc("/admin/v1/credentials/oauth/callback", s.oauthCallback)
	s.mux.HandleFunc("/admin/v1/credentials/oauth/bootstrap", s.authenticate(s.oauthBootstrap))
	s.mux.HandleFunc("/admin/v1/credentials/oauth/providers", s.authenticate(s.oauthProviders))
	s.mux.HandleFunc("/admin/v1/credentials/oauth/refresh", s.authenticate(s.oauthRefresh))
	s.mux.HandleFunc("/admin/v1/credentials/oauth/revoke", s.authenticate(s.oauthRevoke))
	s.mux.HandleFunc("/admin/v1/credentials/cli", s.authenticate(s.cliStatus))
	s.mux.HandleFunc("/admin/v1/credentials/cli/detect", s.authenticate(s.cliDetect))
	s.mux.HandleFunc("/admin/v1/credentials/cli/enable", s.authenticate(s.cliEnable))
	s.mux.HandleFunc("/admin/v1/provider-catalog", s.authenticate(s.providerCatalog))
	s.mux.HandleFunc("/admin/v1/audit", s.authenticate(s.audit))
	s.mux.HandleFunc("/admin/v1/budgets", s.authenticate(s.budgets))
	s.mux.HandleFunc("/admin/v1/usage", s.authenticate(s.usage))
	s.mux.HandleFunc("/admin/v1/health", s.authenticate(s.health))
	s.mux.HandleFunc("/admin/v1/reload", s.authenticate(s.reload))
	s.mux.HandleFunc("/admin/v1/events/stream", s.authenticate(s.eventsStream))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeErr(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func decode(r *http.Request, dst any) error {
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxAdminJSONBodyBytes+1))
	if err != nil {
		return err
	}
	if int64(len(raw)) > maxAdminJSONBodyBytes {
		return errors.New("request body too large")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var extra struct{}
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("request body must contain a single JSON value")
	}
	return nil
}

func pathID(r *http.Request, prefix string) string {
	return strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
}
