package apikey

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/budget"
	"github.com/sigilbridge/sigilbridge/internal/ir"
)

type Anthropic struct {
	baseURL string
	client  *http.Client
	vault   SecretVault
}

func NewAnthropic(baseURL string) Anthropic {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	return Anthropic{baseURL: strings.TrimRight(baseURL, "/"), client: http.DefaultClient}
}

func (p Anthropic) WithClient(client *http.Client) Anthropic {
	if client != nil {
		p.client = client
	}
	return p
}

func (p Anthropic) WithVault(vault SecretVault) Anthropic {
	p.vault = vault
	return p
}

func (Anthropic) ID() string { return "anthropic_api" }

func (p Anthropic) Chat(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (ir.Response, error) {
	req = adapter.RequestWithConfiguredModel(req, cfg)
	body, err := ir.DenormalizeAnthropicRequest(req)
	if err != nil {
		return ir.Response{}, err
	}
	raw, err := p.do(ctx, anthropicMessagesURL(p.base(cfg)), body, cfg)
	if err != nil {
		return ir.Response{}, err
	}
	return parseAnthropicResponse(raw)
}

func (p Anthropic) Stream(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (<-chan ir.Event, error) {
	req = adapter.RequestWithConfiguredModel(req, cfg)
	req.Stream = true
	body, err := ir.DenormalizeAnthropicRequest(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicMessagesURL(p.base(cfg)), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if err := p.setHeaders(ctx, httpReq, cfg); err != nil {
		return nil, err
	}
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, &adapter.Error{Class: adapter.Network, Provider: p.ID(), UpstreamID: cfg.UpstreamID, Message: err.Error(), Retryable: true, Wrapped: err}
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		msg, _ := io.ReadAll(resp.Body)
		class := adapter.ClassifyHTTP(resp.StatusCode)
		return nil, &adapter.Error{Class: class, Provider: p.ID(), UpstreamID: cfg.UpstreamID, HTTPStatus: resp.StatusCode, Message: string(msg), Retryable: adapter.Retryable(class)}
	}
	ch := make(chan ir.Event)
	go parseAnthropicStream(resp.Body, ch)
	return ch, nil
}

func (p Anthropic) CountTokens(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (int, error) {
	req = adapter.RequestWithConfiguredModel(req, cfg)
	body, err := ir.DenormalizeAnthropicRequest(req)
	if err != nil {
		return 0, err
	}
	raw, err := p.do(ctx, anthropicCountTokensURL(p.base(cfg)), body, cfg)
	if err == nil {
		var out struct {
			InputTokens int `json:"input_tokens"`
		}
		if json.Unmarshal(raw, &out) == nil && out.InputTokens > 0 {
			return out.InputTokens, nil
		}
	}
	model := adapter.RawString(cfg.Raw, "model")
	if model == "" {
		model = req.ModelAlias
	}
	return budget.EstimateInputTokens(req, p.ID(), model)
}

func (p Anthropic) HealthCheck(ctx context.Context, cfg adapter.ProviderConfig) error {
	req := ir.Request{ModelAlias: valueOr(adapter.RawString(cfg.Raw, "health_model"), "claude-haiku-4-5"), MaxTokens: 1, Messages: []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "ping"}}}}}
	_, err := p.Chat(ctx, req, cfg)
	return err
}

func (Anthropic) Capabilities() adapter.Capabilities {
	return adapter.Capabilities{Streaming: true, ToolUse: true, Vision: true, PromptCaching: true, MCPServers: true, DocumentInput: true, StabilityClass: adapter.Stable, Category: adapter.CategoryAPIKey}
}

func (p Anthropic) do(ctx context.Context, url string, body []byte, cfg adapter.ProviderConfig) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if err := p.setHeaders(ctx, req, cfg); err != nil {
		return nil, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, &adapter.Error{Class: adapter.Network, Provider: p.ID(), UpstreamID: cfg.UpstreamID, Message: err.Error(), Retryable: true, Wrapped: err}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		class := adapter.ClassifyHTTP(resp.StatusCode)
		return nil, &adapter.Error{Class: class, Provider: p.ID(), UpstreamID: cfg.UpstreamID, HTTPStatus: resp.StatusCode, Message: string(raw), Retryable: adapter.Retryable(class)}
	}
	return raw, nil
}

func (p Anthropic) base(cfg adapter.ProviderConfig) string {
	if base := adapter.RawString(cfg.Raw, "base_url"); base != "" {
		return strings.TrimRight(base, "/")
	}
	return p.baseURL
}

func (p Anthropic) setHeaders(ctx context.Context, req *http.Request, cfg adapter.ProviderConfig) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", valueOr(adapter.RawString(cfg.Raw, "anthropic_version"), "2023-06-01"))
	apiKey, err := p.apiKeyValue(ctx, cfg)
	if err != nil {
		return err
	}
	if apiKey != "" {
		req.Header.Set("x-api-key", apiKey)
	}
	return nil
}

func (p Anthropic) apiKeyValue(ctx context.Context, cfg adapter.ProviderConfig) (string, error) {
	if apiKey := adapter.RawString(cfg.Raw, "api_key"); apiKey != "" {
		return apiKey, nil
	}
	for _, key := range []string{"api_key_ref", "credential", "credential_id", "vault_id"} {
		if id := adapter.RawString(cfg.Raw, key); id != "" {
			if p.vault == nil {
				return "", fmt.Errorf("api key vault is required")
			}
			raw, _, err := p.vault.Get(ctx, id)
			if err != nil {
				return "", err
			}
			return strings.TrimSpace(string(raw)), nil
		}
	}
	return "", nil
}

func parseAnthropicResponse(raw []byte) (ir.Response, error) {
	var in struct {
		ID         string           `json:"id"`
		Model      string           `json:"model"`
		StopReason string           `json:"stop_reason"`
		Content    []map[string]any `json:"content"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return ir.Response{}, fmt.Errorf("parse Anthropic response: %w", err)
	}
	var content []ir.ContentBlock
	for _, block := range in.Content {
		blockType, _ := block["type"].(string)
		switch blockType {
		case "text":
			if text := anthropicBlockText(block); text != "" {
				content = append(content, ir.ContentBlock{Type: ir.ContentText, Text: text})
			}
		case "tool_use":
			input, _ := block["input"].(map[string]any)
			content = append(content, ir.ContentBlock{Type: ir.ContentToolUse, ToolUse: &ir.ToolUse{ID: anthropicString(block, "id"), Name: anthropicString(block, "name"), Arguments: input}})
		default:
			if text := anthropicBlockText(block); text != "" {
				content = append(content, ir.ContentBlock{Type: ir.ContentText, Text: text})
			}
		}
	}
	return ir.Response{Version: ir.Version, ID: in.ID, UpstreamProvider: "anthropic_api", UpstreamModel: in.Model, StopReason: in.StopReason, Content: content, Usage: ir.Usage{InputTokens: in.Usage.InputTokens, OutputTokens: in.Usage.OutputTokens}}, nil
}

func anthropicBlockText(block map[string]any) string {
	for _, key := range []string{"text", "content", "thinking"} {
		if text := anthropicString(block, key); text != "" {
			return text
		}
	}
	return ""
}

func anthropicString(block map[string]any, key string) string {
	value, _ := block[key].(string)
	return value
}

func parseAnthropicStream(body io.ReadCloser, ch chan<- ir.Event) {
	defer body.Close()
	defer close(ch)
	scanner := bufio.NewScanner(body)
	var event string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event:") {
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		switch event {
		case "message_start":
			ch <- ir.Event{Version: ir.Version, Type: ir.EventStart}
		case "content_block_delta":
			var payload struct {
				Index int `json:"index"`
				Delta struct {
					Text string `json:"text"`
				} `json:"delta"`
			}
			if json.Unmarshal([]byte(data), &payload) == nil {
				ch <- ir.Event{Version: ir.Version, Type: ir.EventContentBlockDelta, Index: payload.Index, Delta: &ir.ContentBlock{Type: ir.ContentText, Text: payload.Delta.Text}}
			}
		case "message_stop":
			ch <- ir.Event{Version: ir.Version, Type: ir.EventStop, StopReason: ir.StopEndTurn}
		}
	}
}
