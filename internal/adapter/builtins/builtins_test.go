package builtins

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	sessionadapter "github.com/sigilbridge/sigilbridge/internal/adapter/session"
	"github.com/sigilbridge/sigilbridge/internal/ir"
	coreoauth "github.com/sigilbridge/sigilbridge/internal/oauth"
)

func TestRegistry(t *testing.T) {
	registry, err := Registry()
	if err != nil {
		t.Fatalf("Registry() error = %v", err)
	}
	for _, id := range []string{"mock", "anthropic_api", "openai_api", "groq", "gemini_api", "mistral_api", "deepseek_api", "ollama", "claude_oauth", "copilot_oauth", "gemini_oauth", "cursor_oauth", "claude_code_cli", "codex_cli", "gemini_cli", "aider_cli", "github-copilot-cli", "qwen-code", "opencode", "claude_web", "chatgpt_web"} {
		if _, err := registry.Get(id); err != nil {
			t.Fatalf("missing builtin %q: %v", id, err)
		}
	}
}

func TestRegistryWithAuthWiresOAuthAndSessionVaults(t *testing.T) {
	tokens := &registryTokenAccessor{
		token: coreoauth.Token{AccessToken: "oauth-token", ExpiresAt: time.Now().Add(time.Hour)},
	}
	sessions := newRegistrySessionVault()
	sessions.put("vault://chatgpt_web/main", sessionadapter.SessionCredential{Cookies: map[string]string{"session": "web-token"}, UserAgent: "UA"})

	var gotOAuthAuth, gotSessionCookie, gotSessionUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/messages":
			gotOAuthAuth = r.Header.Get("Authorization")
			_, _ = w.Write([]byte(`{"id":"msg_1","model":"claude-test","stop_reason":"end_turn","content":[{"type":"text","text":"hello oauth"}],"usage":{"input_tokens":1,"output_tokens":2}}`))
		case "/backend-api/conversation":
			gotSessionCookie = r.Header.Get("Cookie")
			gotSessionUA = r.Header.Get("User-Agent")
			_, _ = w.Write([]byte(`{"id":"chat_1","model":"chatgpt-test","choices":[{"message":{"content":"hello session"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":4}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	registry, err := RegistryWithAuth(tokens, sessions)
	if err != nil {
		t.Fatalf("RegistryWithAuth() error = %v", err)
	}
	req := registryTestRequest()

	oauthProvider, err := registry.Get("claude_oauth")
	if err != nil {
		t.Fatalf("Get(claude_oauth) error = %v", err)
	}
	oauthResp, err := oauthProvider.Chat(context.Background(), req, adapter.ProviderConfig{Raw: map[string]any{"credential_name": "main", "base_url": server.URL}})
	if err != nil {
		t.Fatalf("oauth Chat() error = %v", err)
	}
	if gotOAuthAuth != "Bearer oauth-token" || tokens.lastID != "vault://oauth/claude_oauth/main" || oauthResp.Content[0].Text != "hello oauth" {
		t.Fatalf("oauth wiring failed auth=%q id=%q resp=%#v", gotOAuthAuth, tokens.lastID, oauthResp)
	}

	sessionProvider, err := registry.Get("chatgpt_web")
	if err != nil {
		t.Fatalf("Get(chatgpt_web) error = %v", err)
	}
	sessionResp, err := sessionProvider.Chat(context.Background(), req, adapter.ProviderConfig{Raw: map[string]any{"vault_id": "vault://chatgpt_web/main", "base_url": server.URL}})
	if err != nil {
		t.Fatalf("session Chat() error = %v", err)
	}
	if !strings.Contains(gotSessionCookie, "session=web-token") || gotSessionUA != "UA" || sessions.lastID != "vault://chatgpt_web/main" || sessionResp.Content[0].Text != "hello session" {
		t.Fatalf("session wiring failed cookie=%q ua=%q id=%q resp=%#v", gotSessionCookie, gotSessionUA, sessions.lastID, sessionResp)
	}
}

func registryTestRequest() ir.Request {
	return ir.Request{Version: ir.Version, ModelAlias: "test", MaxTokens: 8, Messages: []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "hi"}}}}}
}

type registryTokenAccessor struct {
	mu     sync.Mutex
	token  coreoauth.Token
	lastID string
}

func (f *registryTokenAccessor) Get(_ context.Context, id string) (coreoauth.Token, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastID = id
	return f.token, nil
}

func (f *registryTokenAccessor) AccessToken(ctx context.Context, id string) (string, error) {
	token, err := f.Get(ctx, id)
	return token.AccessToken, err
}

func (f *registryTokenAccessor) Refresh(ctx context.Context, id string) (coreoauth.Token, error) {
	return f.Get(ctx, id)
}

type registrySessionVault struct {
	mu     sync.Mutex
	data   map[string][]byte
	lastID string
}

func newRegistrySessionVault() *registrySessionVault {
	return &registrySessionVault{data: map[string][]byte{}}
}

func (v *registrySessionVault) put(id string, cred sessionadapter.SessionCredential) {
	raw, _ := json.Marshal(cred)
	v.mu.Lock()
	defer v.mu.Unlock()
	v.data[id] = raw
}

func (v *registrySessionVault) Get(_ context.Context, id string) ([]byte, map[string]string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.lastID = id
	return append([]byte(nil), v.data[id]...), map[string]string{}, nil
}
