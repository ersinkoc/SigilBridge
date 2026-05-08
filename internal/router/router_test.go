package router

import (
	"context"
	"fmt"
	"testing"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/ir"
)

func TestRouterResolveFallbackAndRetry(t *testing.T) {
	primary := &stubProvider{id: "primary", err: &adapter.Error{Class: adapter.RateLimited, Provider: "primary", Message: "rate limited", Retryable: true}}
	secondary := &stubProvider{id: "secondary", text: "ok"}
	router := New([]Pool{{
		ID:           "p",
		ModelAliases: []string{"alias"},
		Strategy:     "priority",
		Upstreams: []Upstream{
			{ID: "primary", Priority: 1, Provider: primary},
			{ID: "secondary", Priority: 2, Provider: secondary},
		},
	}})
	pool, err := router.Resolve("alias")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if pool.ID != "p" {
		t.Fatalf("pool = %#v", pool)
	}
	resp, err := router.Dispatch(context.Background(), ir.Request{Version: ir.Version, ModelAlias: "alias"})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if resp.Content[0].Text != "ok" || primary.calls != 1 || secondary.calls != 1 {
		t.Fatalf("resp=%#v primary=%d secondary=%d", resp, primary.calls, secondary.calls)
	}
}

func TestRouterAllowsSingleCoolingUpstreamRecovery(t *testing.T) {
	provider := &stubProvider{id: "primary", text: "ok"}
	router := New([]Pool{{
		ID:           "p",
		ModelAliases: []string{"alias"},
		Strategy:     "priority",
		Upstreams:    []Upstream{{ID: "primary", Priority: 1, Provider: provider}},
	}})
	router.health["primary"].Cooldown()
	resp, err := router.Dispatch(context.Background(), ir.Request{Version: ir.Version, ModelAlias: "alias"})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if resp.Content[0].Text != "ok" || provider.calls != 1 {
		t.Fatalf("resp=%#v calls=%d", resp, provider.calls)
	}
}

type stubProvider struct {
	id    string
	text  string
	err   error
	calls int
}

func (p *stubProvider) ID() string { return p.id }
func (p *stubProvider) Chat(context.Context, ir.Request, adapter.ProviderConfig) (ir.Response, error) {
	p.calls++
	if p.err != nil {
		return ir.Response{}, p.err
	}
	return ir.Response{Version: ir.Version, UpstreamProvider: p.id, StopReason: ir.StopEndTurn, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: p.text}}}, nil
}
func (p *stubProvider) Stream(context.Context, ir.Request, adapter.ProviderConfig) (<-chan ir.Event, error) {
	return nil, fmt.Errorf("not implemented")
}
func (p *stubProvider) CountTokens(context.Context, ir.Request, adapter.ProviderConfig) (int, error) {
	return 0, nil
}
func (p *stubProvider) HealthCheck(context.Context, adapter.ProviderConfig) error { return nil }
func (p *stubProvider) Capabilities() adapter.Capabilities                        { return adapter.Capabilities{} }
