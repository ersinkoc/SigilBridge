package apikey

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/ir"
)

func TestOpenAIChatAndStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing auth header")
		}
		if strings.Contains(r.URL.Path, "chat/completions") && r.Header.Get("Accept") == "text/event-stream" {
			w.Header().Set("Content-Type", "text/event-stream")
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"chat_1","model":"gpt-test","choices":[{"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":4}}`))
	}))
	defer server.Close()

	p := NewOpenAI("openai_api", server.URL)
	resp, err := p.Chat(context.Background(), ir.Request{ModelAlias: "gpt-test", Messages: []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "hi"}}}}}, adapter.ProviderConfig{Raw: map[string]any{"api_key": "test-key"}})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resp.Content[0].Text != "hello" || resp.Usage.OutputTokens != 4 {
		t.Fatalf("response = %#v", resp)
	}
}

func TestOpenAICompatibleIDs(t *testing.T) {
	if NewGroq("").ID() != "groq" || NewMistral("").ID() != "mistral_api" || NewDeepSeek("").ID() != "deepseek_api" || NewGemini("").ID() != "gemini_api" {
		t.Fatalf("compatible provider IDs wrong")
	}
}

func TestOpenAIUsesVaultKeyAndConfigBaseURL(t *testing.T) {
	var gotAuth string
	var gotModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotModel = body.Model
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"chat_1","model":"gpt-test","choices":[{"message":{"role":"assistant","content":"vault ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2}}`))
	}))
	defer server.Close()

	p := NewOpenAI("openai_api", "https://unused.invalid").WithVault(secretVault{"vault://apikey/openai_api/main": "vault-key"})
	resp, err := p.Chat(context.Background(), ir.Request{ModelAlias: "pool-alias", Messages: []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "hi"}}}}}, adapter.ProviderConfig{Raw: map[string]any{"api_key_ref": "vault://apikey/openai_api/main", "base_url": server.URL, "model": "gpt-test"}})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if gotAuth != "Bearer vault-key" || gotModel != "gpt-test" || resp.Content[0].Text != "vault ok" {
		t.Fatalf("auth=%q model=%q response=%#v", gotAuth, gotModel, resp)
	}
}

func TestOpenAIUsesVersionedProviderBaseURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/coding/paas/v4/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"chat_1","model":"glm-test","choices":[{"message":{"role":"assistant","content":"zai ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2}}`))
	}))
	defer server.Close()

	p := NewOpenAI("openai_api", "https://unused.invalid")
	resp, err := p.Chat(context.Background(), ir.Request{ModelAlias: "glm-test", Messages: []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "hi"}}}}}, adapter.ProviderConfig{Raw: map[string]any{"api_key": "test-key", "base_url": server.URL + "/api/coding/paas/v4"}})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resp.Content[0].Text != "zai ok" {
		t.Fatalf("response = %#v", resp)
	}
}

func TestParseOpenAICompatibleContentVariants(t *testing.T) {
	for name, raw := range map[string]string{
		"parts":     `{"id":"chat_1","model":"m","choices":[{"message":{"content":[{"type":"text","text":"part ok"}]},"finish_reason":"stop"}]}`,
		"reasoning": `{"id":"chat_1","model":"m","choices":[{"message":{"content":"","reasoning_content":"reasoning ok"},"finish_reason":"stop"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			resp, err := parseOAIResponse([]byte(raw), "openai_api")
			if err != nil {
				t.Fatalf("parseOAIResponse() error = %v", err)
			}
			if len(resp.Content) != 1 || resp.Content[0].Text == "" {
				t.Fatalf("content = %#v", resp.Content)
			}
		})
	}
}

type secretVault map[string]string

func (v secretVault) Get(_ context.Context, id string) ([]byte, map[string]string, error) {
	return []byte(v[id]), map[string]string{}, nil
}
