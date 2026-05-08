package cloudiam

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
)

func TestVertexAIChatUsesServiceAccountToken(t *testing.T) {
	var tokenRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			tokenRequests++
			if r.Method != http.MethodPost {
				t.Fatalf("token method = %s", r.Method)
			}
			if !strings.Contains(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
				t.Fatalf("token content type = %s", r.Header.Get("Content-Type"))
			}
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "assertion=") {
				t.Fatalf("token request missing assertion")
			}
			_, _ = w.Write([]byte(`{"access_token":"ya29.test"}`))
		case "/predict":
			if r.Header.Get("Authorization") != "Bearer ya29.test" {
				t.Fatalf("Authorization = %s", r.Header.Get("Authorization"))
			}
			_, _ = w.Write([]byte(`{"id":"msg_vertex","model":"claude-test","stop_reason":"end_turn","content":[{"type":"text","text":"hello from vertex"}],"usage":{"input_tokens":9,"output_tokens":10}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	provider := NewVertexAI()
	provider.now = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
	cfg := adapter.ProviderConfig{Raw: map[string]any{
		"base_url":             server.URL,
		"token_url":            server.URL + "/token",
		"service_account_json": testServiceAccountJSON(t, server.URL+"/token"),
		"model":                "claude-test",
	}}
	resp, err := provider.Chat(context.Background(), chatRequest("claude-test"), cfg)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if tokenRequests != 1 {
		t.Fatalf("token requests = %d", tokenRequests)
	}
	if resp.UpstreamProvider != "vertex_ai" || resp.Content[0].Text != "hello from vertex" || resp.Usage.OutputTokens != 10 {
		t.Fatalf("response = %#v", resp)
	}
}

func testServiceAccountJSON(t *testing.T, tokenURL string) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	rawKey, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey() error = %v", err)
	}
	pemKey := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: rawKey}))
	raw, err := json.Marshal(map[string]string{
		"client_email": "sigilbridge-test@example.iam.gserviceaccount.com",
		"private_key":  pemKey,
		"token_uri":    tokenURL,
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return string(raw)
}
