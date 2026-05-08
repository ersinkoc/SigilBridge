package oauth

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed oauth_providers.yaml
var embeddedProviders embed.FS

type Registry struct {
	providers map[string]Provider
}

func LoadRegistry(overridePath string) (*Registry, error) {
	raw, err := embeddedProviders.ReadFile("oauth_providers.yaml")
	if err != nil {
		return nil, err
	}
	registry := &Registry{providers: map[string]Provider{}}
	if err := registry.merge(raw); err != nil {
		return nil, err
	}
	if overridePath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			overridePath = filepath.Join(home, ".sigilbridge", "oauth_providers.yaml")
		}
	}
	if overridePath != "" {
		// #nosec G304 -- OAuth provider override is an explicit local operator-supplied file path.
		if raw, err := os.ReadFile(overridePath); err == nil {
			if err := registry.merge(raw); err != nil {
				return nil, err
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return registry, nil
}

func LoadRegistryFromBytes(raw []byte) (*Registry, error) {
	registry := &Registry{providers: map[string]Provider{}}
	if err := registry.merge(raw); err != nil {
		return nil, err
	}
	return registry, nil
}

func (r *Registry) Get(id string) (Provider, error) {
	provider, ok := r.providers[id]
	if !ok {
		return Provider{}, fmt.Errorf("oauth provider %q is not registered", id)
	}
	return provider, nil
}

func (r *Registry) List() []Provider {
	out := make([]Provider, 0, len(r.providers))
	for _, provider := range r.providers {
		out = append(out, provider)
	}
	return out
}

func (r *Registry) merge(raw []byte) error {
	var doc struct {
		Providers []Provider `yaml:"providers"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parse oauth providers: %w", err)
	}
	for _, provider := range doc.Providers {
		if provider.ID == "" {
			return fmt.Errorf("oauth provider id is required")
		}
		if existing, ok := r.providers[provider.ID]; ok {
			provider = mergeProvider(existing, provider)
		}
		r.providers[provider.ID] = provider
	}
	return nil
}

func mergeProvider(base, override Provider) Provider {
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
	if override.ClientID == "" {
		override.ClientID = base.ClientID
	}
	if override.DefaultScopes == nil {
		override.DefaultScopes = base.DefaultScopes
	}
	if override.ExtraAuthParams == nil {
		override.ExtraAuthParams = base.ExtraAuthParams
	}
	return override
}
