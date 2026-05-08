package oauth

import (
	"context"
	"net/http"
	"time"
)

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type Provider struct {
	ID              string            `yaml:"id" json:"id"`
	DisplayName     string            `yaml:"display_name" json:"display_name"`
	AuthURL         string            `yaml:"auth_url" json:"auth_url"`
	TokenURL        string            `yaml:"token_url" json:"token_url"`
	DeviceAuthURL   string            `yaml:"device_auth_url" json:"device_auth_url"`
	RevokeURL       string            `yaml:"revoke_url" json:"revoke_url"`
	ClientID        string            `yaml:"client_id" json:"client_id"`
	DefaultScopes   []string          `yaml:"default_scopes" json:"default_scopes"`
	ExtraAuthParams map[string]string `yaml:"extra_auth_params" json:"extra_auth_params"`
}

type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	Scope        string    `json:"scope,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	Raw          []byte    `json:"raw,omitempty"`
}

type TokenVault interface {
	Put(ctx context.Context, id string, plaintext []byte, metadata map[string]string) error
	Get(ctx context.Context, id string) ([]byte, map[string]string, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, prefix string) ([]string, error)
}

type TokenAccessor interface {
	Get(ctx context.Context, id string) (Token, error)
	AccessToken(ctx context.Context, id string) (string, error)
	Refresh(ctx context.Context, id string) (Token, error)
}

type OAuthError struct {
	Code        string
	Description string
	Temporary   bool
}

func (e *OAuthError) Error() string {
	if e.Description == "" {
		return e.Code
	}
	return e.Code + ": " + e.Description
}
