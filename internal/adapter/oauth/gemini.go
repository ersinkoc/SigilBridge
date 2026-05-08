package oauthadapter

import coreoauth "github.com/sigilbridge/sigilbridge/internal/oauth"

func NewGemini(tokens coreoauth.TokenAccessor, baseURL string, opts ...Option) Provider {
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}
	opts = append([]Option{WithTokenAccessor(tokens)}, opts...)
	return New("gemini_oauth", "gemini_oauth", baseURL, schemaOpenAI, opts...)
}
