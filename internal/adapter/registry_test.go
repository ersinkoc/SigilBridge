package adapter

import (
	"context"
	"testing"

	"github.com/sigilbridge/sigilbridge/internal/ir"
)

type testProvider struct{ id string }

func (p testProvider) ID() string { return p.id }
func (p testProvider) Chat(context.Context, ir.Request, ProviderConfig) (ir.Response, error) {
	return ir.Response{}, nil
}
func (p testProvider) Stream(context.Context, ir.Request, ProviderConfig) (<-chan ir.Event, error) {
	return nil, nil
}
func (p testProvider) CountTokens(context.Context, ir.Request, ProviderConfig) (int, error) {
	return 0, nil
}
func (p testProvider) HealthCheck(context.Context, ProviderConfig) error { return nil }
func (p testProvider) Capabilities() Capabilities                        { return Capabilities{} }

func TestRegistry(t *testing.T) {
	registry, err := NewRegistry(testProvider{id: "mock"})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	if _, err := registry.Get("mock"); err != nil {
		t.Fatalf("Get(mock) error = %v", err)
	}
	if _, err := registry.Get("missing"); err == nil {
		t.Fatal("Get(missing) error = nil")
	}
	if err := registry.Register(testProvider{id: "mock"}); err == nil {
		t.Fatal("duplicate Register() error = nil")
	}
}
