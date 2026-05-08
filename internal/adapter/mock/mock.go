package mock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/ir"
)

type Provider struct{}

func New() Provider { return Provider{} }

func (Provider) ID() string { return "mock" }

func (Provider) Chat(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (ir.Response, error) {
	if errType := adapter.RawString(cfg.Raw, "error_type"); errType != "" {
		return ir.Response{}, &adapter.Error{Class: adapter.ServerError, UpstreamID: cfg.UpstreamID, Provider: "mock", Message: errType, Retryable: true}
	}
	if latency := adapter.RawInt(cfg.Raw, "latency_ms"); latency > 0 {
		select {
		case <-ctx.Done():
			return ir.Response{}, ctx.Err()
		case <-time.After(time.Duration(latency) * time.Millisecond):
		}
	}
	hash := requestHash(req)
	input := adapter.RawInt(cfg.Raw, "input_tokens")
	if input == 0 {
		input = 10
	}
	output := adapter.RawInt(cfg.Raw, "output_tokens")
	if output == 0 {
		output = 5
	}
	return ir.Response{
		Version:          ir.Version,
		ID:               "mock_" + hash[:16],
		UpstreamProvider: "mock",
		UpstreamModel:    req.ModelAlias,
		StopReason:       ir.StopEndTurn,
		Content:          []ir.ContentBlock{{Type: ir.ContentText, Text: "mock:" + hash[:12]}},
		Usage:            ir.Usage{InputTokens: input, OutputTokens: output},
	}, nil
}

func (p Provider) Stream(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (<-chan ir.Event, error) {
	resp, err := p.Chat(ctx, req, cfg)
	if err != nil {
		return nil, err
	}
	ch := make(chan ir.Event)
	go func() {
		defer close(ch)
		ch <- ir.Event{Version: ir.Version, Type: ir.EventStart}
		for i, block := range resp.Content {
			ch <- ir.Event{Version: ir.Version, Type: ir.EventContentBlockStart, Index: i, Delta: &ir.ContentBlock{Type: block.Type}}
			ch <- ir.Event{Version: ir.Version, Type: ir.EventContentBlockDelta, Index: i, Delta: &block}
			ch <- ir.Event{Version: ir.Version, Type: ir.EventContentBlockStop, Index: i}
		}
		ch <- ir.Event{Version: ir.Version, Type: ir.EventUsage, Usage: &resp.Usage}
		ch <- ir.Event{Version: ir.Version, Type: ir.EventStop, StopReason: resp.StopReason}
	}()
	return ch, nil
}

func (Provider) CountTokens(_ context.Context, req ir.Request, _ adapter.ProviderConfig) (int, error) {
	raw, _ := json.Marshal(req.Messages)
	return len(raw) / 4, nil
}

func (Provider) HealthCheck(context.Context, adapter.ProviderConfig) error { return nil }

func (Provider) Capabilities() adapter.Capabilities {
	return adapter.Capabilities{Streaming: true, ToolUse: true, Vision: true, PromptCaching: true, MCPServers: true, DocumentInput: true, StabilityClass: adapter.Stable, Category: adapter.CategoryMock}
}

func requestHash(req ir.Request) string {
	raw, _ := json.Marshal(req)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (Provider) String() string { return fmt.Sprintf("Provider(%s)", "mock") }
