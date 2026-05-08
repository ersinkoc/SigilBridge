package session

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
	"github.com/sigilbridge/sigilbridge/internal/ir"
)

func TestUTLSDialerUsesChromeHello(t *testing.T) {
	dialer := NewUTLSDialer()
	if dialer.HelloID.Version != "131" {
		t.Fatalf("HelloID = %#v", dialer.HelloID)
	}
}

func TestChromeArgs(t *testing.T) {
	args := strings.Join(ChromeArgs(ChromeBootstrapConfig{ProfileDir: "profile", LoginURL: "https://example.test"}), " ")
	if !strings.Contains(args, "--user-data-dir=profile") || !strings.Contains(args, "https://example.test") {
		t.Fatalf("args = %s", args)
	}
}

func TestClaudeWebSessionChatAndPacing(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/organizations/org-1/chat_conversations/completion" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if !strings.Contains(r.Header.Get("Cookie"), "session=s1") || r.Header.Get("User-Agent") != "UA" {
			t.Fatalf("headers = %#v", r.Header)
		}
		_, _ = w.Write([]byte(`{"id":"msg_1","model":"claude-web","stop_reason":"end_turn","content":[{"type":"text","text":"hello web"}],"usage":{"input_tokens":3,"output_tokens":4}}`))
	}))
	defer server.Close()

	vault := newSessionVault()
	vault.put("vault://claude_web/main", SessionCredential{Cookies: map[string]string{"session": "s1"}, UserAgent: "UA", OrganizationID: "org-1"})
	provider := NewClaudeWeb(server.URL, vault).WithMinInterval(20 * time.Millisecond)
	cfg := adapter.ProviderConfig{Raw: map[string]any{"vault_id": "vault://claude_web/main"}}
	resp, err := provider.Chat(context.Background(), sessionReq("claude-web"), cfg)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	start := time.Now()
	_, err = provider.Chat(context.Background(), sessionReq("claude-web"), cfg)
	if err != nil {
		t.Fatalf("Chat() second error = %v", err)
	}
	if time.Since(start) < 15*time.Millisecond {
		t.Fatalf("pacing did not wait")
	}
	if calls != 2 || resp.Content[0].Text != "hello web" {
		t.Fatalf("calls=%d response=%#v", calls, resp)
	}
}

func TestChatGPTWebSessionChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/backend-api/conversation" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"chat_1","model":"chatgpt-web","choices":[{"message":{"content":"hello chatgpt"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":6}}`))
	}))
	defer server.Close()

	vault := newSessionVault()
	vault.put("vault://chatgpt_web/main", SessionCredential{Cookies: map[string]string{"cf": "ok"}, UserAgent: "UA"})
	provider := NewChatGPTWeb(server.URL, vault).WithMinInterval(0)
	resp, err := provider.Chat(context.Background(), sessionReq("chatgpt-web"), adapter.ProviderConfig{Raw: map[string]any{"vault_id": "vault://chatgpt_web/main"}})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resp.UpstreamProvider != "chatgpt_web" || resp.Content[0].Text != "hello chatgpt" || resp.Usage.OutputTokens != 6 {
		t.Fatalf("response = %#v", resp)
	}
}

func sessionReq(model string) ir.Request {
	return ir.Request{Version: ir.Version, ModelAlias: model, MaxTokens: 8, Messages: []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "hi"}}}}}
}

type sessionVault struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newSessionVault() *sessionVault {
	return &sessionVault{data: map[string][]byte{}}
}

func (v *sessionVault) put(id string, cred SessionCredential) {
	raw, _ := json.Marshal(cred)
	v.mu.Lock()
	defer v.mu.Unlock()
	v.data[id] = raw
}

func (v *sessionVault) Get(_ context.Context, id string) ([]byte, map[string]string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return append([]byte(nil), v.data[id]...), map[string]string{}, nil
}
