package oauth

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestNewPKCE(t *testing.T) {
	pkce, err := NewPKCE()
	if err != nil {
		t.Fatalf("NewPKCE() error = %v", err)
	}
	if len(pkce.Verifier) < 43 || len(pkce.Verifier) > 128 {
		t.Fatalf("verifier length = %d", len(pkce.Verifier))
	}
	if pkce.Challenge == "" || pkce.State == "" || pkce.Method != "S256" {
		t.Fatalf("pkce = %#v", pkce)
	}
}

func TestCallbackServerCapturesCode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server, err := StartCallbackServer(ctx, "state-1")
	if err != nil {
		t.Fatalf("StartCallbackServer() error = %v", err)
	}
	go func() {
		_, _ = http.Get(server.RedirectURI + "?code=abc&state=state-1")
	}()
	result, err := server.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if result.Code != "abc" || result.State != "state-1" {
		t.Fatalf("result = %#v", result)
	}
}
