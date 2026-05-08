package local

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/ir"
)

func TestOllamaChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "" {
			t.Fatalf("ollama should not send auth header")
		}
		var req struct {
			Model    string `json:"model"`
			Stream   bool   `json:"stream"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Options map[string]any `json:"options"`
		}
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Fatalf("request JSON: %v", err)
		}
		if req.Model != "llama-test" || req.Stream {
			t.Fatalf("request = %#v", req)
		}
		if len(req.Messages) != 2 || req.Messages[0].Role != ir.RoleSystem || req.Messages[1].Content != "hi" {
			t.Fatalf("messages = %#v", req.Messages)
		}
		if req.Options["num_predict"].(float64) != 12 {
			t.Fatalf("options = %#v", req.Options)
		}
		_, _ = w.Write([]byte(`{"model":"llama-test","message":{"role":"assistant","content":"hello from ollama"},"done":true,"done_reason":"stop","prompt_eval_count":3,"eval_count":4}`))
	}))
	defer server.Close()

	provider := NewOllama(server.URL)
	resp, err := provider.Chat(context.Background(), ir.Request{
		Version:    ir.Version,
		ModelAlias: "llama-test",
		System:     "be helpful",
		MaxTokens:  12,
		Messages: []ir.Message{{
			Role:    ir.RoleUser,
			Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "hi"}},
		}},
	}, adapter.ProviderConfig{})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resp.UpstreamProvider != "ollama" || resp.Content[0].Text != "hello from ollama" || resp.Usage.OutputTokens != 4 {
		t.Fatalf("response = %#v", resp)
	}
}

func TestOllamaStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"stream":true`) {
			t.Fatalf("stream request body = %s", body)
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(`{"model":"llama-test","message":{"role":"assistant","content":"hel"},"done":false}` + "\n"))
		_, _ = w.Write([]byte(`{"model":"llama-test","message":{"role":"assistant","content":"lo"},"done":false}` + "\n"))
		_, _ = w.Write([]byte(`{"model":"llama-test","done":true,"done_reason":"length","prompt_eval_count":5,"eval_count":6}` + "\n"))
	}))
	defer server.Close()

	provider := NewOllama(server.URL)
	events, err := provider.Stream(context.Background(), ir.Request{Version: ir.Version, ModelAlias: "llama-test", Messages: []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "hi"}}}}}, adapter.ProviderConfig{})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	var text string
	var usage ir.Usage
	var stop string
	for event := range events {
		if event.Delta != nil {
			text += event.Delta.Text
		}
		if event.Usage != nil {
			usage = *event.Usage
		}
		if event.StopReason != "" {
			stop = event.StopReason
		}
	}
	if text != "hello" || usage.OutputTokens != 6 || stop != ir.StopMaxTokens {
		t.Fatalf("stream text=%q usage=%#v stop=%q", text, usage, stop)
	}
}
