package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const vaultPrefix = "vault://oauth/"

type Manager struct {
	registry *Registry
	vault    TokenVault
	client   HTTPClient
	now      func() time.Time
}

type BootstrapResult struct {
	VaultID                 string
	Mode                    string
	AuthURL                 string
	PKCE                    PKCE
	DeviceAuthorization     DeviceAuthorization
	Token                   Token
	VerificationURI         string
	VerificationURIComplete string
	UserCode                string
}

func NewManager(registry *Registry, vault TokenVault, client HTTPClient) *Manager {
	if client == nil {
		client = http.DefaultClient
	}
	return &Manager{registry: registry, vault: vault, client: client, now: func() time.Time { return time.Now().UTC() }}
}

func (m *Manager) Bootstrap(ctx context.Context, providerID, name, mode string) (BootstrapResult, error) {
	provider, err := m.registry.Get(providerID)
	if err != nil {
		return BootstrapResult{}, err
	}
	vaultID := VaultID(providerID, name)
	switch mode {
	case "device":
		return m.BootstrapDevice(ctx, providerID, name, nil)
	case "browser", "":
		pkce, err := NewPKCE()
		if err != nil {
			return BootstrapResult{}, err
		}
		authURL, err := buildAuthURL(provider, pkce, "")
		if err != nil {
			return BootstrapResult{}, err
		}
		return BootstrapResult{VaultID: vaultID, Mode: "browser", AuthURL: authURL, PKCE: pkce}, nil
	default:
		return BootstrapResult{}, fmt.Errorf("unknown oauth bootstrap mode %q", mode)
	}
}

func (m *Manager) BeginBrowser(ctx context.Context, providerID, name, redirectURI string) (BootstrapResult, error) {
	provider, err := m.registry.Get(providerID)
	if err != nil {
		return BootstrapResult{}, err
	}
	vaultID := VaultID(providerID, name)
	pkce, err := NewPKCE()
	if err != nil {
		return BootstrapResult{}, err
	}
	authURL, err := buildAuthURL(provider, pkce, redirectURI)
	if err != nil {
		return BootstrapResult{}, err
	}
	return BootstrapResult{VaultID: vaultID, Mode: "browser", AuthURL: authURL, PKCE: pkce}, nil
}

func (m *Manager) BootstrapDevice(ctx context.Context, providerID, name string, notify func(BootstrapResult)) (BootstrapResult, error) {
	provider, err := m.registry.Get(providerID)
	if err != nil {
		return BootstrapResult{}, err
	}
	vaultID := VaultID(providerID, name)
	device, err := StartDeviceAuthorization(ctx, m.client, provider, nil)
	if err != nil {
		return BootstrapResult{}, err
	}
	pending := BootstrapResult{VaultID: vaultID, Mode: "device", DeviceAuthorization: device, VerificationURI: device.VerificationURI, VerificationURIComplete: device.VerificationURIComplete, UserCode: device.UserCode}
	if notify != nil {
		notify(pending)
	}
	token, err := PollDeviceAuthorization(ctx, m.client, provider, device)
	if err != nil {
		return pending, err
	}
	if err := m.storeToken(ctx, vaultID, provider, token); err != nil {
		return BootstrapResult{}, err
	}
	pending.Token = token
	return pending, nil
}

func (m *Manager) BootstrapBrowser(ctx context.Context, providerID, name string, openBrowser bool) (BootstrapResult, error) {
	provider, err := m.registry.Get(providerID)
	if err != nil {
		return BootstrapResult{}, err
	}
	vaultID := VaultID(providerID, name)
	pkce, err := NewPKCE()
	if err != nil {
		return BootstrapResult{}, err
	}
	callback, err := StartCallbackServer(ctx, pkce.State)
	if err != nil {
		return BootstrapResult{}, err
	}
	authURL, err := buildAuthURL(provider, pkce, callback.RedirectURI)
	if err != nil {
		_ = callback.server.Shutdown(context.Background())
		return BootstrapResult{}, err
	}
	if openBrowser {
		if err := OpenBrowser(authURL); err != nil {
			_ = callback.server.Shutdown(context.Background())
			return BootstrapResult{VaultID: vaultID, Mode: "browser", AuthURL: authURL, PKCE: pkce}, err
		}
	}
	result, err := callback.Wait(ctx)
	if err != nil {
		return BootstrapResult{VaultID: vaultID, Mode: "browser", AuthURL: authURL, PKCE: pkce}, err
	}
	token, err := m.CompleteBrowser(ctx, providerID, name, result.Code, callback.RedirectURI, pkce)
	if err != nil {
		return BootstrapResult{VaultID: vaultID, Mode: "browser", AuthURL: authURL, PKCE: pkce}, err
	}
	return BootstrapResult{VaultID: vaultID, Mode: "browser", AuthURL: authURL, PKCE: pkce, Token: token}, nil
}

func (m *Manager) CompleteBrowser(ctx context.Context, providerID, name, code, redirectURI string, pkce PKCE) (Token, error) {
	provider, err := m.registry.Get(providerID)
	if err != nil {
		return Token{}, err
	}
	token, err := ExchangeCode(ctx, m.client, ExchangeRequest{TokenURL: provider.TokenURL, ClientID: provider.ClientID, Code: code, CodeVerifier: pkce.Verifier, RedirectURI: redirectURI})
	if err != nil {
		return Token{}, err
	}
	return token, m.storeToken(ctx, VaultID(providerID, name), provider, token)
}

func (m *Manager) Refresh(ctx context.Context, id string) (Token, error) {
	token, metadata, err := m.loadToken(ctx, id)
	if err != nil {
		return Token{}, err
	}
	provider, err := m.registry.Get(metadata["provider"])
	if err != nil {
		return Token{}, err
	}
	if token.RefreshToken == "" {
		return Token{}, &OAuthError{Code: "missing_refresh_token"}
	}
	refreshed, err := RefreshToken(ctx, m.client, ExchangeRequest{TokenURL: provider.TokenURL, ClientID: provider.ClientID, RefreshToken: token.RefreshToken})
	if err != nil {
		return Token{}, err
	}
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = token.RefreshToken
	}
	return refreshed, m.storeToken(ctx, id, provider, refreshed)
}

func (m *Manager) Revoke(ctx context.Context, id string) error {
	token, metadata, err := m.loadToken(ctx, id)
	if err != nil {
		return err
	}
	provider, err := m.registry.Get(metadata["provider"])
	if err != nil {
		return err
	}
	revokeToken := token.RefreshToken
	if revokeToken == "" {
		revokeToken = token.AccessToken
	}
	if err := RevokeToken(ctx, m.client, provider.RevokeURL, provider.ClientID, revokeToken); err != nil {
		return err
	}
	return m.vault.Delete(ctx, id)
}

func (m *Manager) Get(ctx context.Context, id string) (Token, error) {
	token, _, err := m.loadToken(ctx, id)
	return token, err
}

func (m *Manager) List(ctx context.Context) ([]string, error) {
	return m.vault.List(ctx, vaultPrefix)
}

func (m *Manager) AccessToken(ctx context.Context, id string) (string, error) {
	token, err := m.Get(ctx, id)
	if err != nil {
		return "", err
	}
	return token.AccessToken, nil
}

func (m *Manager) storeToken(ctx context.Context, id string, provider Provider, token Token) error {
	raw, err := json.Marshal(token)
	if err != nil {
		return err
	}
	metadata := map[string]string{"provider": provider.ID, "updated_at": m.now().Format(time.RFC3339)}
	if !token.ExpiresAt.IsZero() {
		metadata["expires_at"] = token.ExpiresAt.UTC().Format(time.RFC3339)
	}
	return m.vault.Put(ctx, id, raw, metadata)
}

func (m *Manager) loadToken(ctx context.Context, id string) (Token, map[string]string, error) {
	raw, metadata, err := m.vault.Get(ctx, id)
	if err != nil {
		return Token{}, nil, err
	}
	var token Token
	if err := json.Unmarshal(raw, &token); err != nil {
		return Token{}, nil, err
	}
	return token, metadata, nil
}

func VaultID(providerID, name string) string {
	providerID = strings.Trim(providerID, "/")
	name = strings.Trim(name, "/")
	if name == "" {
		name = "default"
	}
	return vaultPrefix + path.Clean(providerID+"/"+name)
}

func buildAuthURL(provider Provider, pkce PKCE, redirectURI string) (string, error) {
	rawURL, err := url.Parse(provider.AuthURL)
	if err != nil {
		return "", err
	}
	values := rawURL.Query()
	values.Set("response_type", "code")
	values.Set("client_id", provider.ClientID)
	values.Set("code_challenge", pkce.Challenge)
	values.Set("code_challenge_method", pkce.Method)
	values.Set("state", pkce.State)
	if redirectURI != "" {
		values.Set("redirect_uri", redirectURI)
	}
	if len(provider.DefaultScopes) > 0 {
		values.Set("scope", strings.Join(provider.DefaultScopes, " "))
	}
	for key, value := range provider.ExtraAuthParams {
		values.Set(key, value)
	}
	rawURL.RawQuery = values.Encode()
	return rawURL.String(), nil
}
