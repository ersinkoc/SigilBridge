package oauthadapter

import coreoauth "github.com/sigilbridge/sigilbridge/internal/oauth"

func NewCursor(tokens coreoauth.TokenAccessor, baseURL string, opts ...Option) Provider {
	if baseURL == "" {
		baseURL = "https://api.cursor.com"
	}
	opts = append([]Option{WithTokenAccessor(tokens)}, opts...)
	provider := New("cursor_oauth", "cursor_oauth", baseURL, schemaOpenAI, opts...)
	provider.stability = "experimental"
	return provider
}
