package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/budget"
	"github.com/sigilbridge/sigilbridge/internal/ir"
	pb "github.com/sigilbridge/sigilbridge/pkg/proto"
)

type GRPCAdapter struct {
	id     string
	client pb.ProviderPluginClient
}

func NewGRPCAdapter(id string, client pb.ProviderPluginClient) GRPCAdapter {
	return GRPCAdapter{id: id, client: client}
}

func (a GRPCAdapter) ID() string { return a.id }

func (a GRPCAdapter) Chat(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (ir.Response, error) {
	requestJSON, err := json.Marshal(req)
	if err != nil {
		return ir.Response{}, err
	}
	configJSON, err := json.Marshal(cfg)
	if err != nil {
		return ir.Response{}, err
	}
	out, err := a.client.Chat(ctx, &pb.ChatRequest{ProviderId: a.id, RequestJson: requestJSON, ConfigJson: configJSON})
	if err != nil {
		return ir.Response{}, err
	}
	var resp ir.Response
	if err := json.Unmarshal(out.ResponseJson, &resp); err != nil {
		return ir.Response{}, err
	}
	return resp, nil
}

func (a GRPCAdapter) Stream(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (<-chan ir.Event, error) {
	requestJSON, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	configJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	stream, err := a.client.Stream(ctx, &pb.ChatRequest{ProviderId: a.id, RequestJson: requestJSON, ConfigJson: configJSON})
	if err != nil {
		return nil, err
	}
	ch := make(chan ir.Event)
	go func() {
		defer close(ch)
		for {
			event, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				ch <- ir.Event{Version: ir.Version, Type: ir.EventError, Error: &ir.Error{Type: "plugin_stream_error", Message: err.Error(), Retryable: true, Class: ir.ErrorNetwork}}
				return
			}
			var decoded ir.Event
			if err := json.Unmarshal(event.EventJson, &decoded); err == nil {
				ch <- decoded
			}
		}
	}()
	return ch, nil
}

func (a GRPCAdapter) CountTokens(_ context.Context, req ir.Request, cfg adapter.ProviderConfig) (int, error) {
	model := adapter.RawString(cfg.Raw, "model")
	if model == "" {
		model = req.ModelAlias
	}
	return budget.EstimateInputTokens(req, a.id, model)
}

func (a GRPCAdapter) HealthCheck(ctx context.Context, cfg adapter.ProviderConfig) error {
	configJSON, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	resp, err := a.client.Health(ctx, &pb.HealthRequest{ProviderId: a.id, ConfigJson: configJSON})
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("plugin health failed: %s", resp.Message)
	}
	return nil
}

func (a GRPCAdapter) Capabilities() adapter.Capabilities {
	return adapter.Capabilities{Streaming: true, ToolUse: true, Vision: true, StabilityClass: adapter.Stable, Category: "plugin"}
}
