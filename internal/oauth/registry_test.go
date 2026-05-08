package oauth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRegistryOverrideMerge(t *testing.T) {
	dir := t.TempDir()
	override := filepath.Join(dir, "oauth_providers.yaml")
	if err := os.WriteFile(override, []byte(`
providers:
  - id: first
    display_name: First
    auth_url: https://first.test/auth
    token_url: https://first.test/token
    client_id: first-client
  - id: second
    display_name: Second
    auth_url: https://second.test/auth
    token_url: https://second.test/token
    client_id: second-client
`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	registry, err := LoadRegistry(override)
	if err != nil {
		t.Fatalf("LoadRegistry() error = %v", err)
	}
	first, err := registry.Get("first")
	if err != nil {
		t.Fatalf("Get(first) error = %v", err)
	}
	if first.ClientID != "first-client" || first.AuthURL == "" {
		t.Fatalf("first = %#v", first)
	}
	second, err := registry.Get("second")
	if err != nil {
		t.Fatalf("Get(second) error = %v", err)
	}
	if second.ClientID != "second-client" {
		t.Fatalf("second = %#v", second)
	}
	if len(registry.List()) != 2 {
		t.Fatalf("List() = %#v", registry.List())
	}
	if _, err := registry.Get("missing"); err == nil {
		t.Fatal("Get(missing) error = nil")
	}
}

func TestLoadRegistryRejectsBadOverride(t *testing.T) {
	dir := t.TempDir()
	override := filepath.Join(dir, "oauth_providers.yaml")
	if err := os.WriteFile(override, []byte(`providers: [}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := LoadRegistry(override); err == nil {
		t.Fatal("LoadRegistry(bad override) error = nil")
	}

	blocked := dir
	if _, err := LoadRegistry(blocked); err == nil {
		t.Fatal("LoadRegistry(unreadable override path) error = nil")
	}
}

func TestMergeProviderPreservesDefaults(t *testing.T) {
	base := Provider{
		ID:              "stub",
		DisplayName:     "Stub",
		AuthURL:         "https://default.example/auth",
		TokenURL:        "https://default.example/token",
		ClientID:        "default-client",
		DefaultScopes:   []string{"default"},
		ExtraAuthParams: map[string]string{"audience": "default"},
	}
	override := Provider{
		ID:              "stub",
		DisplayName:     "Override",
		RevokeURL:       "https://override.example/revoke",
		DefaultScopes:   []string{"override"},
		ExtraAuthParams: map[string]string{"prompt": "consent"},
	}
	got := mergeProvider(base, override)
	if got.DisplayName != "Override" || got.AuthURL != base.AuthURL || got.ClientID != base.ClientID || got.RevokeURL != "https://override.example/revoke" {
		t.Fatalf("mergeProvider() = %#v", got)
	}
	if len(got.DefaultScopes) != 1 || got.DefaultScopes[0] != "override" || got.ExtraAuthParams["prompt"] != "consent" {
		t.Fatalf("merged scopes/params = %#v %#v", got.DefaultScopes, got.ExtraAuthParams)
	}
}
