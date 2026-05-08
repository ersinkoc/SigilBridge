package oauthadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/budget"
	"github.com/sigilbridge/sigilbridge/internal/ir"
	coreoauth "github.com/sigilbridge/sigilbridge/internal/oauth"
)

type schema string

const (
	schemaAnthropic schema = "anthropic"
	schemaOpenAI    schema = "openai"
)

type Provider struct {
	id          string
	oauthID     string
	baseURL     string
	category    string
	stability   string
	schema      schema
	client      *http.Client
	tokens      coreoauth.TokenAccessor
	now         func() time.Time
	refreshSkew time.Duration
}

type Option func(*Provider)

func WithClient(client *http.Client) Option {
	return func(p *Provider) {
		if client != nil {
			p.client = client
		}
	}
}

func WithTokenAccessor(tokens coreoauth.TokenAccessor) Option {
	return func(p *Provider) { p.tokens = tokens }
}

func New(id, oauthID, baseURL string, shape schema, opts ...Option) Provider {
	p := Provider{id: id, oauthID: oauthID, baseURL: strings.TrimRight(baseURL, "/"), schema: shape, client: http.DefaultClient, category: "oauth", stability: adapter.Stable, now: func() time.Time { return time.Now().UTC() }, refreshSkew: 10 * time.Minute}
	for _, opt := range opts {
		opt(&p)
	}
	return p
}

func (p Provider) ID() string { return p.id }

func (p Provider) Chat(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (ir.Response, error) {
	req = adapter.RequestWithConfiguredModel(req, cfg)
	var body []byte
	var err error
	var endpoint string
	switch p.schema {
	case schemaAnthropic:
		body, err = ir.DenormalizeAnthropicRequest(req)
		endpoint = p.base(cfg) + "/v1/messages"
	default:
		body, err = ir.DenormalizeOAIRequest(req)
		endpoint = p.base(cfg) + "/v1/chat/completions"
	}
	if err != nil {
		return ir.Response{}, err
	}
	raw, err := p.do(ctx, endpoint, body, cfg)
	if err != nil {
		return ir.Response{}, err
	}
	if p.schema == schemaAnthropic {
		return parseAnthropicResponse(raw, p.id)
	}
	return parseOpenAIResponse(raw, p.id)
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
			ch <- ir.Event{Version: ir.Version, Type: ir.EventContentBlockDelta, Index: i, Delta: &block}
		}
		ch <- ir.Event{Version: ir.Version, Type: ir.EventUsage, Usage: &resp.Usage}
		ch <- ir.Event{Version: ir.Version, Type: ir.EventStop, StopReason: resp.StopReason}
	}()
	return ch, nil
}

func (p Provider) CountTokens(_ context.Context, req ir.Request, cfg adapter.ProviderConfig) (int, error) {
	model := adapter.RawString(cfg.Raw, "model")
	if model == "" {
		model = req.ModelAlias
	}
	return budget.EstimateInputTokens(req, p.id, model)
}

func (p Provider) HealthCheck(ctx context.Context, cfg adapter.ProviderConfig) error {
	model := adapter.RawString(cfg.Raw, "model")
	if model == "" {
		model = "health"
	}
	_, err := p.Chat(ctx, ir.Request{Version: ir.Version, ModelAlias: model, MaxTokens: 1, Messages: []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "ping"}}}}}, cfg)
	return err
}

func (p Provider) Capabilities() adapter.Capabilities {
	return adapter.Capabilities{Streaming: true, ToolUse: true, Vision: true, StabilityClass: p.stability, Category: p.category}
}

func (p Provider) base(cfg adapter.ProviderConfig) string {
	if base := adapter.RawString(cfg.Raw, "base_url"); base != "" {
		return strings.TrimRight(base, "/")
	}
	return p.baseURL
}

func (p Provider) do(ctx context.Context, endpoint string, body []byte, cfg adapter.ProviderConfig) ([]byte, error) {
	token, err := p.bearerToken(ctx, cfg)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	if p.schema == schemaAnthropic {
		req.Header.Set("anthropic-version", valueOr(adapter.RawString(cfg.Raw, "anthropic_version"), "2023-06-01"))
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, &adapter.Error{Class: adapter.Network, Provider: p.id, UpstreamID: cfg.UpstreamID, Message: err.Error(), Retryable: true, Wrapped: err}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		class := adapter.ClassifyHTTP(resp.StatusCode)
		return nil, &adapter.Error{Class: class, Provider: p.id, UpstreamID: cfg.UpstreamID, HTTPStatus: resp.StatusCode, Message: string(raw), Retryable: adapter.Retryable(class)}
	}
	return raw, nil
}

func (p Provider) bearerToken(ctx context.Context, cfg adapter.ProviderConfig) (string, error) {
	if direct := adapter.RawString(cfg.Raw, "access_token"); direct != "" {
		return direct, nil
	}
	if p.tokens == nil {
		return "", fmt.Errorf("oauth token accessor is required")
	}
	id := credentialID(cfg, p.oauthID)
	token, err := p.tokens.Get(ctx, id)
	if err != nil {
		return "", err
	}
	if !token.ExpiresAt.IsZero() && !token.ExpiresAt.After(p.now().Add(p.refreshSkew)) {
		token, err = p.tokens.Refresh(ctx, id)
		if err != nil {
			return "", err
		}
	}
	if token.AccessToken == "" {
		return "", fmt.Errorf("oauth token %q has no access_token", id)
	}
	return token.AccessToken, nil
}

func credentialID(cfg adapter.ProviderConfig, oauthID string) string {
	if id := adapter.RawString(cfg.Raw, "credential"); id != "" {
		return id
	}
	if id := adapter.RawString(cfg.Raw, "vault_id"); id != "" {
		return id
	}
	if id := adapter.RawString(cfg.Raw, "credential_id"); id != "" {
		return id
	}
	name := valueOr(adapter.RawString(cfg.Raw, "credential_name"), "default")
	return coreoauth.VaultID(oauthID, name)
}

func parseAnthropicResponse(raw []byte, provider string) (ir.Response, error) {
	var in struct {
		ID         string `json:"id"`
		Model      string `json:"model"`
		StopReason string `json:"stop_reason"`
		Content    []struct {
			Type  string         `json:"type"`
			Text  string         `json:"text"`
			ID    string         `json:"id"`
			Name  string         `json:"name"`
			Input map[string]any `json:"input"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return ir.Response{}, fmt.Errorf("parse Anthropic response: %w", err)
	}
	content := []ir.ContentBlock{}
	for _, block := range in.Content {
		switch block.Type {
		case "text":
			content = append(content, ir.ContentBlock{Type: ir.ContentText, Text: block.Text})
		case "tool_use":
			content = append(content, ir.ContentBlock{Type: ir.ContentToolUse, ToolUse: &ir.ToolUse{ID: block.ID, Name: block.Name, Arguments: block.Input}})
		}
	}
	return ir.Response{Version: ir.Version, ID: in.ID, UpstreamProvider: provider, UpstreamModel: in.Model, StopReason: valueOr(in.StopReason, ir.StopEndTurn), Content: content, Usage: ir.Usage{InputTokens: in.Usage.InputTokens, OutputTokens: in.Usage.OutputTokens}}, nil
}

func parseOpenAIResponse(raw []byte, provider string) (ir.Response, error) {
	var in struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return ir.Response{}, fmt.Errorf("parse OpenAI response: %w", err)
	}
	content := []ir.ContentBlock{}
	if len(in.Choices) > 0 && in.Choices[0].Message.Content != "" {
		content = append(content, ir.ContentBlock{Type: ir.ContentText, Text: in.Choices[0].Message.Content})
	}
	return ir.Response{Version: ir.Version, ID: in.ID, UpstreamProvider: provider, UpstreamModel: in.Model, StopReason: stopFromOpenAI(in.Choices), Content: content, Usage: ir.Usage{InputTokens: in.Usage.PromptTokens, OutputTokens: in.Usage.CompletionTokens}}, nil
}

func stopFromOpenAI(choices []struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	FinishReason string `json:"finish_reason"`
}) string {
	if len(choices) == 0 {
		return ir.StopEndTurn
	}
	if choices[0].FinishReason == "length" {
		return ir.StopMaxTokens
	}
	return ir.StopEndTurn
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
