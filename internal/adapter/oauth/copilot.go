package oauthadapter

import coreoauth "github.com/sigilbridge/sigilbridge/internal/oauth"

func NewCopilot(tokens coreoauth.TokenAccessor, baseURL string, opts ...Option) Provider {
	if baseURL == "" {
		baseURL = "https://api.githubcopilot.com"
	}
	opts = append([]Option{WithTokenAccessor(tokens)}, opts...)
	return New("copilot_oauth", "copilot_oauth", baseURL, schemaOpenAI, opts...)
}
