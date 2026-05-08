package oauthadapter

import coreoauth "github.com/sigilbridge/sigilbridge/internal/oauth"

func NewClaude(tokens coreoauth.TokenAccessor, baseURL string, opts ...Option) Provider {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	opts = append([]Option{WithTokenAccessor(tokens)}, opts...)
	return New("claude_oauth", "claude_oauth", baseURL, schemaAnthropic, opts...)
}
