package adapter

import (
	"context"

	"github.com/sigilbridge/sigilbridge/internal/ir"
)

const (
	CategoryAPIKey = "api_key"
	CategoryMock   = "mock"
	Stable         = "stable"
)

type Provider interface {
	ID() string
	Chat(ctx context.Context, req ir.Request, cfg ProviderConfig) (ir.Response, error)
	Stream(ctx context.Context, req ir.Request, cfg ProviderConfig) (<-chan ir.Event, error)
	CountTokens(ctx context.Context, req ir.Request, cfg ProviderConfig) (int, error)
	HealthCheck(ctx context.Context, cfg ProviderConfig) error
	Capabilities() Capabilities
}

type Capabilities struct {
	Streaming        bool
	ToolUse          bool
	Vision           bool
	PromptCaching    bool
	MCPServers       bool
	DocumentInput    bool
	MaxContextTokens int
	StabilityClass   string
	Category         string
}

type ProviderConfig struct {
	UpstreamID string
	Raw        map[string]any
}

func RawString(raw map[string]any, key string) string {
	value, _ := raw[key].(string)
	return value
}

func RawInt(raw map[string]any, key string) int {
	switch v := raw[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func RequestWithConfiguredModel(req ir.Request, cfg ProviderConfig) ir.Request {
	if model := RawString(cfg.Raw, "model"); model != "" {
		req.ModelAlias = model
	}
	return req
}
