package builtins

import (
	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/adapter/apikey"
	cliacpadapter "github.com/sigilbridge/sigilbridge/internal/adapter/cliacp"
	"github.com/sigilbridge/sigilbridge/internal/adapter/local"
	"github.com/sigilbridge/sigilbridge/internal/adapter/mock"
	oauthadapter "github.com/sigilbridge/sigilbridge/internal/adapter/oauth"
	sessionadapter "github.com/sigilbridge/sigilbridge/internal/adapter/session"
	coreoauth "github.com/sigilbridge/sigilbridge/internal/oauth"
)

func Registry() (*adapter.Registry, error) {
	return RegistryWithAuth(nil, nil)
}

func RegistryWithAuth(tokens coreoauth.TokenAccessor, secretVault apikey.SecretVault) (*adapter.Registry, error) {
	providers := []adapter.Provider{
		mock.New(),
		apikey.NewAnthropic("").WithVault(secretVault),
		apikey.NewOpenAI("openai_api", "").WithVault(secretVault),
		apikey.NewGroq("").WithVault(secretVault),
		apikey.NewGemini("").WithVault(secretVault),
		apikey.NewMistral("").WithVault(secretVault),
		apikey.NewDeepSeek("").WithVault(secretVault),
		local.NewOllama(""),
		oauthadapter.NewClaude(tokens, ""),
		oauthadapter.NewCopilot(tokens, ""),
		oauthadapter.NewGemini(tokens, ""),
		oauthadapter.NewCursor(tokens, ""),
		sessionadapter.NewClaudeWeb("", secretVault),
		sessionadapter.NewChatGPTWeb("", secretVault),
	}
	for _, agent := range cliacpadapter.Defaults() {
		providers = append(providers, cliacpadapter.New(agent.ID, agent.Command, agent.Args...))
	}
	return adapter.NewRegistry(providers...)
}
