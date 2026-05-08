package cloudiam

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
)

func TestBedrockChatSignsInvokeRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/model/anthropic.claude-test/invoke" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Header.Get("x-amz-date") != "20260102T030405Z" {
			t.Fatalf("x-amz-date = %s", r.Header.Get("x-amz-date"))
		}
		if r.Header.Get("x-amz-content-sha256") == "" {
			t.Fatalf("missing payload hash")
		}
		auth := r.Header.Get("Authorization")
		for _, want := range []string{
			"AWS4-HMAC-SHA256",
			"Credential=AKIA_TEST/20260102/us-west-2/bedrock/aws4_request",
			"SignedHeaders=host;x-amz-content-sha256;x-amz-date",
			"Signature=",
		} {
			if !strings.Contains(auth, want) {
				t.Fatalf("Authorization missing %q: %s", want, auth)
			}
		}
		_, _ = w.Write([]byte(`{"id":"msg_bedrock","model":"anthropic.claude-test","stop_reason":"end_turn","content":[{"type":"text","text":"hello from bedrock"}],"usage":{"input_tokens":7,"output_tokens":8}}`))
	}))
	defer server.Close()

	provider := NewBedrock()
	provider.now = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
	cfg := adapter.ProviderConfig{Raw: map[string]any{
		"base_url":          server.URL,
		"model_id":          "anthropic.claude-test",
		"region":            "us-west-2",
		"access_key_id":     "AKIA_TEST",
		"secret_access_key": "secret",
	}}
	resp, err := provider.Chat(context.Background(), chatRequest("anthropic.claude-test"), cfg)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resp.UpstreamProvider != "bedrock" || resp.Content[0].Text != "hello from bedrock" || resp.Usage.OutputTokens != 8 {
		t.Fatalf("response = %#v", resp)
	}
}
