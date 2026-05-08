package cloudiam

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/ir"
)

func TestAzureOpenAIChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openai/deployments/dep/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("api-version") != "2024-10-21" {
			t.Fatalf("api-version = %s", r.URL.Query().Get("api-version"))
		}
		if r.Header.Get("api-key") != "azure-key" {
			t.Fatalf("missing api-key header")
		}
		_, _ = w.Write([]byte(`{"id":"chat_azure","model":"dep","choices":[{"message":{"content":"hello from azure"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":6}}`))
	}))
	defer server.Close()

	provider := NewAzureOpenAI()
	cfg := adapter.ProviderConfig{Raw: map[string]any{"base_url": server.URL, "deployment": "dep", "api_key": "azure-key"}}
	resp, err := provider.Chat(context.Background(), chatRequest("dep"), cfg)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resp.UpstreamProvider != "azure_openai" || resp.Content[0].Text != "hello from azure" || resp.Usage.OutputTokens != 6 {
		t.Fatalf("response = %#v", resp)
	}
}

func chatRequest(model string) ir.Request {
	return ir.Request{
		Version:    ir.Version,
		ModelAlias: model,
		MaxTokens:  16,
		Messages: []ir.Message{{
			Role:    ir.RoleUser,
			Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "hello"}},
		}},
	}
}
