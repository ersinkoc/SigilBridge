package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sigilbridge/sigilbridge/internal/events"
)

func TestAdminHandlers(t *testing.T) {
	server := New(Services{Auth: fakeAuth{}, Keys: fakeKeys{}, Pools: fakePools{}, Endpoints: fakeEndpoints{}, Chat: fakeChat{}, Credentials: fakeCreds{}, Audit: fakeAudit{}, Budgets: fakeBudgets{}, Health: fakeHealth{}, Reload: fakeReload{}, Events: events.NewBus()})

	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/admin/v1/auth/login", strings.NewReader(`{"token":"bad"}`)))
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("bad login status = %d", resp.Code)
	}
	resp = httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/admin/v1/auth/login", strings.NewReader(`{"token":"ok"}`)))
	if resp.Code != http.StatusOK || resp.Result().Cookies()[0].Name != "sigilbridge_admin" {
		t.Fatalf("login status=%d cookies=%#v", resp.Code, resp.Result().Cookies())
	}
	sessionCookie := resp.Result().Cookies()[0]
	if sessionCookie.SameSite != http.SameSiteStrictMode || !sessionCookie.HttpOnly {
		t.Fatalf("session cookie flags = SameSite:%v HttpOnly:%v", sessionCookie.SameSite, sessionCookie.HttpOnly)
	}
	resp = httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/admin/v1/keys", nil))
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated keys status=%d body=%s", resp.Code, resp.Body.String())
	}
	resp = httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, authed(httptest.NewRequest(http.MethodPost, "/admin/v1/keys", strings.NewReader(`{"prefix":"test"}`)), sessionCookie))
	if resp.Code != http.StatusCreated || !strings.Contains(resp.Body.String(), "sb_test_secret") {
		t.Fatalf("create key status=%d body=%s", resp.Code, resp.Body.String())
	}
	resp = httptest.NewRecorder()
	crossOrigin := httptest.NewRequest(http.MethodPost, "/admin/v1/keys", strings.NewReader(`{"prefix":"test"}`))
	crossOrigin.Header.Set("Origin", "http://evil.example")
	crossOrigin.AddCookie(sessionCookie)
	server.Handler().ServeHTTP(resp, crossOrigin)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("cross-origin cookie write status=%d body=%s", resp.Code, resp.Body.String())
	}
	resp = httptest.NewRecorder()
	bearerCrossOrigin := httptest.NewRequest(http.MethodPost, "/admin/v1/keys", strings.NewReader(`{"prefix":"test"}`))
	bearerCrossOrigin.Header.Set("Origin", "http://evil.example")
	bearerCrossOrigin.Header.Set("Authorization", "Bearer ok")
	server.Handler().ServeHTTP(resp, bearerCrossOrigin)
	if resp.Code != http.StatusCreated {
		t.Fatalf("bearer cross-origin write status=%d body=%s", resp.Code, resp.Body.String())
	}
	resp = httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, authed(httptest.NewRequest(http.MethodGet, "/admin/v1/keys/key1", nil), sessionCookie))
	if resp.Code != http.StatusOK || strings.Contains(resp.Body.String(), "sb_test_secret") {
		t.Fatalf("get key status=%d body=%s", resp.Code, resp.Body.String())
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
		status int
	}{
		{http.MethodPost, "/admin/v1/pools", `{"id":"pool1"}`, http.StatusOK},
		{http.MethodPost, "/admin/v1/pools/pool1/probe", `{}`, http.StatusOK},
		{http.MethodGet, "/admin/v1/endpoints", ``, http.StatusOK},
		{http.MethodPost, "/admin/v1/chat/test", `{"model":"pool1","message":"hi"}`, http.StatusOK},
		{http.MethodGet, "/admin/v1/credentials", ``, http.StatusOK},
		{http.MethodDelete, "/admin/v1/credentials?id=vault://oauth/stub/main", ``, http.StatusOK},
		{http.MethodPost, "/admin/v1/credentials/api-key", `{"provider":"openai_api","name":"main","api_key":"sk-test","model":"gpt-test"}`, http.StatusCreated},
		{http.MethodPost, "/admin/v1/credentials/session", `{"provider":"claude_web","name":"main","cookies":{"session":"s1"},"user_agent":"UA"}`, http.StatusCreated},
		{http.MethodPost, "/admin/v1/credentials/oauth/bootstrap", `{"provider":"stub","name":"main","mode":"browser"}`, http.StatusOK},
		{http.MethodGet, "/admin/v1/credentials/oauth/providers", ``, http.StatusOK},
		{http.MethodPut, "/admin/v1/credentials/oauth/providers", `{"body":"providers: []"}`, http.StatusOK},
		{http.MethodPost, "/admin/v1/credentials/oauth/refresh", `{"id":"vault://oauth/stub/main"}`, http.StatusOK},
		{http.MethodPost, "/admin/v1/credentials/oauth/revoke", `{"id":"vault://oauth/stub/main"}`, http.StatusOK},
		{http.MethodGet, "/admin/v1/credentials/cli", ``, http.StatusOK},
		{http.MethodGet, "/admin/v1/credentials/cli/detect", ``, http.StatusOK},
		{http.MethodPost, "/admin/v1/credentials/cli/enable", `{"provider":"codex_cli","command":"codex"}`, http.StatusOK},
		{http.MethodGet, "/admin/v1/provider-catalog", ``, http.StatusOK},
		{http.MethodGet, "/admin/v1/audit?key_id=k1", ``, http.StatusOK},
		{http.MethodGet, "/admin/v1/budgets", ``, http.StatusOK},
		{http.MethodGet, "/admin/v1/usage", ``, http.StatusOK},
		{http.MethodGet, "/admin/v1/health", ``, http.StatusOK},
		{http.MethodPost, "/admin/v1/reload", `{}`, http.StatusOK},
	} {
		resp = httptest.NewRecorder()
		server.Handler().ServeHTTP(resp, authed(httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)), sessionCookie))
		if resp.Code != tc.status {
			t.Fatalf("%s %s status=%d body=%s", tc.method, tc.path, resp.Code, resp.Body.String())
		}
	}
}

func TestAdminDecodeRejectsMalformedBodies(t *testing.T) {
	server := New(Services{Auth: fakeAuth{}, Keys: fakeKeys{}, Pools: fakePools{}, Endpoints: fakeEndpoints{}, Chat: fakeChat{}, Credentials: fakeCreds{}, Audit: fakeAudit{}, Budgets: fakeBudgets{}, Health: fakeHealth{}, Reload: fakeReload{}, Events: events.NewBus()})

	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "unknown field", body: `{"token":"ok","extra":true}`},
		{name: "multiple json values", body: `{"token":"ok"} {"token":"ok"}`},
		{name: "too large", body: `{"token":"` + strings.Repeat("a", int(maxAdminJSONBodyBytes)) + `"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			server.Handler().ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/admin/v1/auth/login", strings.NewReader(tc.body)))
			if resp.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
			}
		})
	}
}

func authed(req *http.Request, cookie *http.Cookie) *http.Request {
	req.AddCookie(cookie)
	switch req.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
	default:
		req.Header.Set("Origin", "http://example.com")
	}
	return req
}

type fakeAuth struct{}

func (fakeAuth) Login(_ context.Context, token string) (string, error) {
	if token != "ok" {
		return "", errText("bad token")
	}
	return "jwt", nil
}

func (fakeAuth) Verify(_ context.Context, r *http.Request) (string, error) {
	if cookie, err := r.Cookie("sigilbridge_admin"); err == nil && cookie.Value == "jwt" {
		return "admin", nil
	}
	if r.Header.Get("Authorization") == "Bearer ok" {
		return "admin", nil
	}
	return "", errText("unauthorized")
}

type fakeKeys struct{}

func (fakeKeys) List(context.Context) ([]KeyDTO, error) {
	return []KeyDTO{{ID: "key1", Hash: "sha256:x"}}, nil
}
func (fakeKeys) Create(context.Context, CreateKeyRequest) (CreateKeyResponse, error) {
	return CreateKeyResponse{KeyDTO: KeyDTO{ID: "key1", Hash: "sha256:x"}, Plaintext: "sb_test_secret"}, nil
}
func (fakeKeys) Get(context.Context, string) (KeyDTO, error) {
	return KeyDTO{ID: "key1", Hash: "sha256:x"}, nil
}
func (fakeKeys) Patch(context.Context, string, map[string]any) (KeyDTO, error) {
	return KeyDTO{ID: "key1", Hash: "sha256:x"}, nil
}
func (fakeKeys) Delete(context.Context, string) error { return nil }

type fakePools struct{}

func (fakePools) List(context.Context) ([]PoolDTO, error)                 { return []PoolDTO{{ID: "pool1"}}, nil }
func (fakePools) Upsert(_ context.Context, pool PoolDTO) (PoolDTO, error) { return pool, nil }
func (fakePools) Delete(context.Context, string) error                    { return nil }
func (fakePools) Probe(context.Context, string) (map[string]any, error) {
	return map[string]any{"ok": true}, nil
}

type fakeEndpoints struct{}

func (fakeEndpoints) Info(context.Context) (EndpointInfoResponse, error) {
	return EndpointInfoResponse{OpenAIBase: "http://127.0.0.1:8187/v1", OpenAIChat: "http://127.0.0.1:8187/v1/chat/completions", OpenAIModels: "http://127.0.0.1:8187/v1/models", AnthropicBase: "http://127.0.0.1:8187", AnthropicMessages: "http://127.0.0.1:8187/v1/messages"}, nil
}

type fakeChat struct{}

func (fakeChat) Test(context.Context, ChatTestRequest) (ChatTestResponse, error) {
	return ChatTestResponse{Model: "pool1", Content: "hello"}, nil
}

type fakeCreds struct{}

func (fakeCreds) List(context.Context) (map[string]any, error) {
	return map[string]any{"oauth": []any{}, "sessions": []any{}, "cli": []any{}}, nil
}
func (fakeCreds) Delete(context.Context, string) error { return nil }
func (fakeCreds) APIKeyCreate(context.Context, APIKeyCredentialRequest) (map[string]any, error) {
	return map[string]any{"id": "vault://apikey/openai_api/main"}, nil
}
func (fakeCreds) SessionCreate(context.Context, SessionCredentialRequest) (map[string]any, error) {
	return map[string]any{"id": "vault://claude_web/main"}, nil
}
func (fakeCreds) OAuthBootstrap(context.Context, string, string, string) (map[string]any, error) {
	return map[string]any{"auth_url": "https://example.test"}, nil
}
func (fakeCreds) OAuthCallback(context.Context, string, string, string) (map[string]any, error) {
	return map[string]any{"vault_id": "vault://oauth/stub/main"}, nil
}
func (fakeCreds) OAuthRefresh(context.Context, string) (map[string]any, error) {
	return map[string]any{"ok": true}, nil
}
func (fakeCreds) OAuthRevoke(context.Context, string) (map[string]any, error) {
	return map[string]any{"ok": true}, nil
}
func (fakeCreds) OAuthProvidersRaw(context.Context) (map[string]any, error) {
	return map[string]any{"body": "providers: []"}, nil
}
func (fakeCreds) OAuthProvidersSave(context.Context, string) (map[string]any, error) {
	return map[string]any{"ok": true}, nil
}
func (fakeCreds) CLIStatus(context.Context) (map[string]any, error) {
	return map[string]any{"running": true}, nil
}
func (fakeCreds) CLIDetect(context.Context) (map[string]any, error) {
	return map[string]any{"agents": []any{}}, nil
}
func (fakeCreds) CLIEnable(context.Context, CLIEnableRequest) (map[string]any, error) {
	return map[string]any{"ok": true}, nil
}
func (fakeCreds) ProviderCatalog(context.Context) (map[string]any, error) {
	return map[string]any{"providers": []any{}}, nil
}

type fakeAudit struct{}

func (fakeAudit) Query(_ context.Context, values map[string][]string) (map[string]any, error) {
	keyID := ""
	if len(values["key_id"]) > 0 {
		keyID = values["key_id"][0]
	}
	return map[string]any{"items": []any{}, "key_id": keyID}, nil
}

type fakeBudgets struct{}

func (fakeBudgets) Budgets(context.Context) (map[string]any, error) {
	return map[string]any{"daily": 1}, nil
}
func (fakeBudgets) Usage(context.Context) (map[string]any, error) {
	return map[string]any{"top": []any{}}, nil
}

type fakeHealth struct{}

func (fakeHealth) Detail(context.Context) (map[string]any, error) {
	return map[string]any{"upstreams": []any{}}, nil
}

type fakeReload struct{}

func (fakeReload) Reload(context.Context) (ReloadResult, error) { return ReloadResult{OK: true}, nil }

type errText string

func (e errText) Error() string { return string(e) }
