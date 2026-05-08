package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestManagerBootstrapCompleteRefreshRevoke(t *testing.T) {
	var refreshRequests, revokeRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm() error = %v", err)
			}
			switch r.Form.Get("grant_type") {
			case "authorization_code":
				_, _ = w.Write([]byte(`{"access_token":"access-1","refresh_token":"refresh-1","expires_in":60}`))
			case "refresh_token":
				refreshRequests++
				_, _ = w.Write([]byte(`{"access_token":"access-2","expires_in":120}`))
			default:
				t.Fatalf("grant_type = %s", r.Form.Get("grant_type"))
			}
		case "/revoke":
			revokeRequests++
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	registry := &Registry{providers: map[string]Provider{"stub": {ID: "stub", AuthURL: server.URL + "/auth", TokenURL: server.URL + "/token", RevokeURL: server.URL + "/revoke", ClientID: "client-1"}}}
	vault := newMemoryVault()
	manager := NewManager(registry, vault, server.Client())
	result, err := manager.Bootstrap(context.Background(), "stub", "main", "browser")
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	if result.AuthURL == "" || result.PKCE.Verifier == "" {
		t.Fatalf("result = %#v", result)
	}
	token, err := manager.CompleteBrowser(context.Background(), "stub", "main", "code-1", "http://127.0.0.1/callback", result.PKCE)
	if err != nil {
		t.Fatalf("CompleteBrowser() error = %v", err)
	}
	if token.AccessToken != "access-1" {
		t.Fatalf("token = %#v", token)
	}
	refreshed, err := manager.Refresh(context.Background(), result.VaultID)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if refreshed.AccessToken != "access-2" || refreshed.RefreshToken != "refresh-1" || refreshRequests != 1 {
		t.Fatalf("refreshed=%#v refreshRequests=%d", refreshed, refreshRequests)
	}
	if err := manager.Revoke(context.Background(), result.VaultID); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if revokeRequests != 1 {
		t.Fatalf("revokeRequests = %d", revokeRequests)
	}
}

func TestRefreshWorkerRefreshesDueTokens(t *testing.T) {
	var refreshRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshRequests++
		_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"refresh-1","expires_in":3600}`))
	}))
	defer server.Close()

	registry := &Registry{providers: map[string]Provider{"stub": {ID: "stub", TokenURL: server.URL, ClientID: "client-1"}}}
	vault := newMemoryVault()
	manager := NewManager(registry, vault, server.Client())
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	manager.now = func() time.Time { return now }
	old := Token{AccessToken: "old-access", RefreshToken: "refresh-1", ExpiresAt: now.Add(2 * time.Minute)}
	if err := manager.storeToken(context.Background(), VaultID("stub", "main"), registry.providers["stub"], old); err != nil {
		t.Fatalf("storeToken() error = %v", err)
	}
	var events []RefreshEvent
	worker := NewRefreshWorker(manager, time.Hour, 10*time.Minute, func(event RefreshEvent) { events = append(events, event) })
	worker.RefreshDue(context.Background())
	if refreshRequests != 1 || len(events) != 1 || !events[0].Success {
		t.Fatalf("refreshRequests=%d events=%#v", refreshRequests, events)
	}
	token, err := manager.Get(context.Background(), VaultID("stub", "main"))
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if token.AccessToken != "new-access" {
		t.Fatalf("token = %#v", token)
	}
}

func TestManagerDeviceBootstrapAndAccessToken(t *testing.T) {
	var polls int
	var notified atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/device":
			_, _ = w.Write([]byte(`{"device_code":"device-1","user_code":"ABCD","verification_uri":"https://login.example/device","verification_uri_complete":"https://login.example/device?user_code=ABCD","expires_in":600,"interval":0}`))
		case "/token":
			if !notified.Load() {
				t.Fatalf("device token was polled before caller was notified")
			}
			polls++
			_, _ = w.Write([]byte(`{"access_token":"device-access","refresh_token":"device-refresh","expires_in":3600}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	registry := &Registry{providers: map[string]Provider{"stub": {ID: "stub", DeviceAuthURL: server.URL + "/device", TokenURL: server.URL + "/token", ClientID: "client-1"}}}
	manager := NewManager(registry, newMemoryVault(), server.Client())
	result, err := manager.BootstrapDevice(context.Background(), "stub", "device", func(result BootstrapResult) {
		if result.UserCode != "ABCD" || result.Token.AccessToken != "" {
			t.Fatalf("notification result = %#v", result)
		}
		notified.Store(true)
	})
	if err != nil {
		t.Fatalf("BootstrapDevice() error = %v", err)
	}
	if result.Token.AccessToken != "device-access" || result.UserCode != "ABCD" || polls != 1 || !notified.Load() {
		t.Fatalf("result=%#v polls=%d", result, polls)
	}
	access, err := manager.AccessToken(context.Background(), result.VaultID)
	if err != nil {
		t.Fatalf("AccessToken() error = %v", err)
	}
	if access != "device-access" {
		t.Fatalf("access = %q", access)
	}
}

func TestManagerErrorPathsAndAuthURL(t *testing.T) {
	manager := NewManager(&Registry{providers: map[string]Provider{"stub": {ID: "stub", AuthURL: "%", ClientID: "client-1"}}}, newMemoryVault(), nil)
	if _, err := manager.Bootstrap(context.Background(), "missing", "main", "browser"); err == nil {
		t.Fatal("Bootstrap(missing provider) error = nil")
	}
	if _, err := manager.Bootstrap(context.Background(), "stub", "main", "browser"); err == nil {
		t.Fatal("Bootstrap(invalid auth URL) error = nil")
	}
	registry := &Registry{providers: map[string]Provider{"stub": {
		ID: "stub", AuthURL: "https://auth.example/authorize", ClientID: "client-1",
		DefaultScopes: []string{"offline_access"}, ExtraAuthParams: map[string]string{"audience": "models"},
	}}}
	pkce := PKCE{Challenge: "challenge", Method: "S256", State: "state"}
	rawURL, err := buildAuthURL(registry.providers["stub"], pkce, "http://127.0.0.1/callback")
	if err != nil {
		t.Fatalf("buildAuthURL() error = %v", err)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("Parse(authURL) error = %v", err)
	}
	query := parsed.Query()
	if query.Get("scope") != "offline_access" || query.Get("audience") != "models" || query.Get("redirect_uri") == "" {
		t.Fatalf("query = %v", query)
	}
	if got := VaultID("/stub/", ""); got != "vault://oauth/stub/default" {
		t.Fatalf("VaultID() = %q", got)
	}
}

func TestManagerBeginBrowserAndList(t *testing.T) {
	registry := &Registry{providers: map[string]Provider{"stub": {
		ID: "stub", AuthURL: "https://auth.example/authorize", TokenURL: "https://auth.example/token", ClientID: "client-1",
	}}}
	vault := newMemoryVault()
	manager := NewManager(registry, vault, nil)
	result, err := manager.BeginBrowser(context.Background(), "stub", "main", "http://127.0.0.1/callback")
	if err != nil {
		t.Fatalf("BeginBrowser() error = %v", err)
	}
	if result.Mode != "browser" || result.AuthURL == "" || result.PKCE.Verifier == "" || result.VaultID != "vault://oauth/stub/main" {
		t.Fatalf("result = %#v", result)
	}
	if err := manager.storeToken(context.Background(), result.VaultID, registry.providers["stub"], Token{AccessToken: "access-1"}); err != nil {
		t.Fatalf("storeToken() error = %v", err)
	}
	ids, err := manager.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(ids) != 1 || ids[0] != result.VaultID {
		t.Fatalf("List() = %#v", ids)
	}
}

func TestOAuthErrorString(t *testing.T) {
	err := &OAuthError{Code: "invalid_grant", Description: "expired"}
	if err.Error() != "invalid_grant: expired" {
		t.Fatalf("Error() = %q", err.Error())
	}
}

func TestRefreshWorkerRunStopsOnContext(t *testing.T) {
	registry := &Registry{providers: map[string]Provider{}}
	manager := NewManager(registry, newMemoryVault(), nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		NewRefreshWorker(manager, time.Millisecond, time.Millisecond, nil).Run(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("RefreshWorker.Run did not stop after context cancellation")
	}
}

type memoryVault struct {
	mu       sync.Mutex
	payloads map[string][]byte
	metadata map[string]map[string]string
}

func newMemoryVault() *memoryVault {
	return &memoryVault{payloads: map[string][]byte{}, metadata: map[string]map[string]string{}}
}

func (v *memoryVault) Put(_ context.Context, id string, plaintext []byte, metadata map[string]string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.payloads[id] = append([]byte(nil), plaintext...)
	v.metadata[id] = map[string]string{}
	for key, value := range metadata {
		v.metadata[id][key] = value
	}
	return nil
}

func (v *memoryVault) Get(_ context.Context, id string) ([]byte, map[string]string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	payload, ok := v.payloads[id]
	if !ok {
		return nil, nil, &OAuthError{Code: "not_found"}
	}
	metadata := map[string]string{}
	for key, value := range v.metadata[id] {
		metadata[key] = value
	}
	return append([]byte(nil), payload...), metadata, nil
}

func (v *memoryVault) Delete(_ context.Context, id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.payloads, id)
	delete(v.metadata, id)
	return nil
}

func (v *memoryVault) List(_ context.Context, prefix string) ([]string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	var ids []string
	for id := range v.payloads {
		if prefix == "" || len(id) >= len(prefix) && id[:len(prefix)] == prefix {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
