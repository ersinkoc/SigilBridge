package cloudiam

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/budget"
	"github.com/sigilbridge/sigilbridge/internal/ir"
)

type AzureOpenAI struct {
	client *http.Client
}

func NewAzureOpenAI() AzureOpenAI { return AzureOpenAI{client: http.DefaultClient} }
func (p AzureOpenAI) WithClient(client *http.Client) AzureOpenAI {
	if client != nil {
		p.client = client
	}
	return p
}
func (AzureOpenAI) ID() string { return "azure_openai" }

func (p AzureOpenAI) Chat(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (ir.Response, error) {
	body, err := ir.DenormalizeOAIRequest(req)
	if err != nil {
		return ir.Response{}, err
	}
	url := azureURL(cfg)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return ir.Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-key", adapter.RawString(cfg.Raw, "api_key"))
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return ir.Response{}, &adapter.Error{Class: adapter.Network, Provider: p.ID(), UpstreamID: cfg.UpstreamID, Message: err.Error(), Retryable: true, Wrapped: err}
	}
	raw, err := readHTTP(p.ID(), cfg, resp)
	if err != nil {
		return ir.Response{}, err
	}
	return parseOpenAIShape(raw, p.ID())
}

func (p AzureOpenAI) Stream(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (<-chan ir.Event, error) {
	return streamFromChat(ctx, func(ctx context.Context) (ir.Response, error) { return p.Chat(ctx, req, cfg) })
}

func (AzureOpenAI) CountTokens(_ context.Context, req ir.Request, cfg adapter.ProviderConfig) (int, error) {
	return budget.EstimateInputTokens(req, "azure_openai", adapter.RawString(cfg.Raw, "model"))
}
func (p AzureOpenAI) HealthCheck(ctx context.Context, cfg adapter.ProviderConfig) error {
	_, err := p.Chat(ctx, ir.Request{ModelAlias: adapter.RawString(cfg.Raw, "deployment"), MaxTokens: 1, Messages: []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "ping"}}}}}, cfg)
	return err
}
func (AzureOpenAI) Capabilities() adapter.Capabilities {
	return adapter.Capabilities{Streaming: true, ToolUse: true, Vision: true, StabilityClass: adapter.Stable, Category: "cloud_iam"}
}

func azureURL(cfg adapter.ProviderConfig) string {
	if base := adapter.RawString(cfg.Raw, "base_url"); base != "" {
		return strings.TrimRight(base, "/") + "/openai/deployments/" + adapter.RawString(cfg.Raw, "deployment") + "/chat/completions?api-version=" + valueOr(adapter.RawString(cfg.Raw, "api_version"), "2024-10-21")
	}
	resource := adapter.RawString(cfg.Raw, "resource")
	return fmt.Sprintf("https://%s.openai.azure.com/openai/deployments/%s/chat/completions?api-version=%s", resource, adapter.RawString(cfg.Raw, "deployment"), valueOr(adapter.RawString(cfg.Raw, "api_version"), "2024-10-21"))
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
