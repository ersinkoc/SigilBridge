package apikey

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/ir"
)

func TestAnthropicChatCountAndStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("missing api key")
		}
		switch r.URL.Path {
		case "/v1/messages":
			_, _ = w.Write([]byte(`{"id":"msg_1","model":"claude-test","stop_reason":"end_turn","content":[{"type":"text","text":"hello"}],"usage":{"input_tokens":3,"output_tokens":4}}`))
		case "/v1/messages/count_tokens":
			_, _ = w.Write([]byte(`{"input_tokens":42}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	p := NewAnthropic(server.URL)
	cfg := adapter.ProviderConfig{Raw: map[string]any{"api_key": "test-key"}}
	req := ir.Request{ModelAlias: "claude-test", MaxTokens: 10, Messages: []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "hi"}}}}}
	resp, err := p.Chat(context.Background(), req, cfg)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resp.Content[0].Text != "hello" || resp.Usage.OutputTokens != 4 {
		t.Fatalf("response = %#v", resp)
	}
	count, err := p.CountTokens(context.Background(), req, cfg)
	if err != nil {
		t.Fatalf("CountTokens() error = %v", err)
	}
	if count != 42 {
		t.Fatalf("count = %d, want 42", count)
	}
}

func TestAnthropicUsesVaultKeyAndConfigBaseURL(t *testing.T) {
	var gotKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"msg_1","model":"claude-test","stop_reason":"end_turn","content":[{"type":"text","text":"vault ok"}],"usage":{"input_tokens":1,"output_tokens":2}}`))
	}))
	defer server.Close()

	p := NewAnthropic("https://unused.invalid").WithVault(secretVault{"vault://apikey/anthropic_api/main": "vault-key"})
	cfg := adapter.ProviderConfig{Raw: map[string]any{"api_key_ref": "vault://apikey/anthropic_api/main", "base_url": server.URL}}
	req := ir.Request{ModelAlias: "claude-test", MaxTokens: 10, Messages: []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "hi"}}}}}
	resp, err := p.Chat(context.Background(), req, cfg)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if gotKey != "vault-key" || resp.Content[0].Text != "vault ok" {
		t.Fatalf("key=%q response=%#v", gotKey, resp)
	}
}

func TestAnthropicUsesVersionedProviderBaseURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/anthropic/v1/messages":
			_, _ = w.Write([]byte(`{"id":"msg_1","model":"MiniMax-M2.5","stop_reason":"end_turn","content":[{"type":"text","text":"minimax ok"}],"usage":{"input_tokens":1,"output_tokens":2}}`))
		case "/anthropic/v1/messages/count_tokens":
			_, _ = w.Write([]byte(`{"input_tokens":9}`))
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer server.Close()

	p := NewAnthropic("https://unused.invalid")
	cfg := adapter.ProviderConfig{Raw: map[string]any{"api_key": "test-key", "base_url": server.URL + "/anthropic/v1"}}
	req := ir.Request{ModelAlias: "MiniMax-M2.5", MaxTokens: 10, Messages: []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "hi"}}}}}
	resp, err := p.Chat(context.Background(), req, cfg)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resp.Content[0].Text != "minimax ok" {
		t.Fatalf("response = %#v", resp)
	}
	count, err := p.CountTokens(context.Background(), req, cfg)
	if err != nil {
		t.Fatalf("CountTokens() error = %v", err)
	}
	if count != 9 {
		t.Fatalf("count = %d", count)
	}
}

func TestParseAnthropicCompatibleTextVariants(t *testing.T) {
	for name, raw := range map[string]string{
		"text":     `{"id":"msg_1","model":"m","stop_reason":"end_turn","content":[{"type":"text","text":"text ok"}],"usage":{"input_tokens":1,"output_tokens":2}}`,
		"content":  `{"id":"msg_1","model":"m","stop_reason":"end_turn","content":[{"type":"text","content":"content ok"}],"usage":{"input_tokens":1,"output_tokens":2}}`,
		"thinking": `{"id":"msg_1","model":"m","stop_reason":"end_turn","content":[{"type":"thinking","thinking":"thinking ok"}],"usage":{"input_tokens":1,"output_tokens":2}}`,
	} {
		t.Run(name, func(t *testing.T) {
			resp, err := parseAnthropicResponse([]byte(raw))
			if err != nil {
				t.Fatalf("parseAnthropicResponse() error = %v", err)
			}
			if len(resp.Content) != 1 || resp.Content[0].Text == "" {
				t.Fatalf("content = %#v", resp.Content)
			}
		})
	}
}
