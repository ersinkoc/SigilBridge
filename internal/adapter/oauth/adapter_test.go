package oauthadapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/ir"
	coreoauth "github.com/sigilbridge/sigilbridge/internal/oauth"
)

func TestOAuthBaseRefreshesAndAddsBearerToken(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"chat_1","model":"copilot-test","choices":[{"message":{"content":"hello oauth"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":4}}`))
	}))
	defer server.Close()

	tokens := &fakeTokenAccessor{token: coreoauth.Token{AccessToken: "old", RefreshToken: "refresh", ExpiresAt: time.Now().Add(time.Minute)}, refreshed: coreoauth.Token{AccessToken: "new", RefreshToken: "refresh", ExpiresAt: time.Now().Add(time.Hour)}}
	provider := NewCopilot(tokens, server.URL)
	resp, err := provider.Chat(context.Background(), testRequest("copilot-test"), adapter.ProviderConfig{Raw: map[string]any{"credential_name": "main"}})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if gotAuth != "Bearer new" || tokens.refreshes != 1 {
		t.Fatalf("auth=%q refreshes=%d", gotAuth, tokens.refreshes)
	}
	if resp.UpstreamProvider != "copilot_oauth" || resp.Content[0].Text != "hello oauth" {
		t.Fatalf("response = %#v", resp)
	}
}

func TestClaudeOAuthUsesAnthropicSchema(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer access" || r.Header.Get("anthropic-version") == "" {
			t.Fatalf("headers = %#v", r.Header)
		}
		_, _ = w.Write([]byte(`{"id":"msg_1","model":"claude-test","stop_reason":"end_turn","content":[{"type":"text","text":"hello claude"}],"usage":{"input_tokens":5,"output_tokens":6}}`))
	}))
	defer server.Close()

	provider := NewClaude(&fakeTokenAccessor{token: coreoauth.Token{AccessToken: "access", ExpiresAt: time.Now().Add(time.Hour)}}, server.URL)
	resp, err := provider.Chat(context.Background(), testRequest("claude-test"), adapter.ProviderConfig{})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resp.UpstreamProvider != "claude_oauth" || resp.Content[0].Text != "hello claude" || resp.Usage.OutputTokens != 6 {
		t.Fatalf("response = %#v", resp)
	}
}

func TestOAuthProviderIDs(t *testing.T) {
	tokens := &fakeTokenAccessor{}
	if NewGemini(tokens, "").ID() != "gemini_oauth" || NewCursor(tokens, "").Capabilities().StabilityClass != "experimental" {
		t.Fatalf("oauth provider constructors returned wrong metadata")
	}
}

func TestCredentialIDAcceptsDocumentedConfigKeys(t *testing.T) {
	tests := []struct {
		name string
		raw  map[string]any
		want string
	}{
		{name: "credential", raw: map[string]any{"credential": "vault://oauth/claude-max/personal"}, want: "vault://oauth/claude-max/personal"},
		{name: "vault_id", raw: map[string]any{"vault_id": "vault://oauth/copilot/work"}, want: "vault://oauth/copilot/work"},
		{name: "credential_id", raw: map[string]any{"credential_id": "vault://oauth/gemini/default"}, want: "vault://oauth/gemini/default"},
		{name: "credential_name", raw: map[string]any{"credential_name": "main"}, want: "vault://oauth/claude_oauth/main"},
		{name: "default", raw: nil, want: "vault://oauth/claude_oauth/default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := credentialID(adapter.ProviderConfig{Raw: tt.raw}, "claude_oauth")
			if got != tt.want {
				t.Fatalf("credentialID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func testRequest(model string) ir.Request {
	return ir.Request{Version: ir.Version, ModelAlias: model, MaxTokens: 10, Messages: []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "hi"}}}}}
}

type fakeTokenAccessor struct {
	mu        sync.Mutex
	token     coreoauth.Token
	refreshed coreoauth.Token
	refreshes int
}

func (f *fakeTokenAccessor) Get(context.Context, string) (coreoauth.Token, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.token, nil
}

func (f *fakeTokenAccessor) AccessToken(ctx context.Context, id string) (string, error) {
	token, err := f.Get(ctx, id)
	return token.AccessToken, err
}

func (f *fakeTokenAccessor) Refresh(context.Context, string) (coreoauth.Token, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refreshes++
	f.token = f.refreshed
	return f.token, nil
}
