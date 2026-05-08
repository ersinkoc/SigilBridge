package ingress

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/ir"
)

func TestMiddlewareFailures(t *testing.T) {
	s := New(fakeDispatcher{}).WithAuth(func(*http.Request) (string, error) { return "", errText("bad auth") })
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	s.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("auth status = %d", resp.Code)
	}

	s = New(fakeDispatcher{}).WithAuth(func(*http.Request) (string, error) { return "key", nil }).WithRateLimit(func(*http.Request, string) error { return errText("slow down") })
	resp = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	s.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("rate status = %d", resp.Code)
	}

	s = New(fakeDispatcher{}).WithBudget(func(*http.Request, string, ir.Request) error { return errText("budget") })
	resp = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`))
	s.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusPaymentRequired {
		t.Fatalf("budget status = %d", resp.Code)
	}
}

func TestOpenAIIngress(t *testing.T) {
	var observed bool
	s := New(fakeDispatcher{text: "hello"}).WithObserver(func(_ *http.Request, req ir.Request, resp ir.Response, err error, _ time.Duration) {
		observed = true
		if err != nil || req.ModelAlias != "m" || resp.Usage.OutputTokens != 2 {
			t.Fatalf("observer req=%#v resp=%#v err=%v", req, resp, err)
		}
	})
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`))
	s.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "hello") {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	if !observed {
		t.Fatal("observer was not called")
	}
}

func TestAnthropicIngressAndHealth(t *testing.T) {
	s := New(fakeDispatcher{text: "hello anthropic"})
	for _, path := range []string{"/healthz", "/readyz"} {
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		s.Handler().ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("%s status = %d", path, resp.Code)
		}
	}
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`))
	s.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "hello anthropic") {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestOpenAIStreamIngress(t *testing.T) {
	s := New(fakeDispatcher{text: "hi"})
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"m","stream":true,"messages":[{"role":"user","content":"hi"}]}`))
	s.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "data:") {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
}

type errText string

func (e errText) Error() string { return string(e) }

type fakeDispatcher struct {
	text string
}

func (d fakeDispatcher) Dispatch(context.Context, ir.Request) (ir.Response, error) {
	return ir.Response{Version: ir.Version, ID: "resp", UpstreamProvider: "fake", UpstreamModel: "m", StopReason: ir.StopEndTurn, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: d.text}}, Usage: ir.Usage{InputTokens: 1, OutputTokens: 2}}, nil
}

func (d fakeDispatcher) Stream(ctx context.Context, req ir.Request) (<-chan ir.Event, error) {
	resp, _ := d.Dispatch(ctx, req)
	ch := make(chan ir.Event, 3)
	ch <- ir.Event{Version: ir.Version, Type: ir.EventStart}
	ch <- ir.Event{Version: ir.Version, Type: ir.EventContentBlockDelta, Delta: &resp.Content[0]}
	ch <- ir.Event{Version: ir.Version, Type: ir.EventStop, StopReason: ir.StopEndTurn}
	close(ch)
	return ch, nil
}
