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

type OpenAI struct {
	id       string
	baseURL  string
	client   *http.Client
	apiKey   string
	vault    SecretVault
	category string
}

type SecretVault interface {
	Get(ctx context.Context, id string) ([]byte, map[string]string, error)
}

func NewOpenAI(id, baseURL string) OpenAI {
	if id == "" {
		id = "openai_api"
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	return OpenAI{id: id, baseURL: strings.TrimRight(baseURL, "/"), client: http.DefaultClient, category: adapter.CategoryAPIKey}
}

func (p OpenAI) WithClient(client *http.Client) OpenAI {
	if client != nil {
		p.client = client
	}
	return p
}

func (p OpenAI) WithVault(vault SecretVault) OpenAI {
	p.vault = vault
	return p
}

func (p OpenAI) ID() string { return p.id }

func (p OpenAI) Chat(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (ir.Response, error) {
	req = adapter.RequestWithConfiguredModel(req, cfg)
	body, err := ir.DenormalizeOAIRequest(req)
	if err != nil {
		return ir.Response{}, err
	}
	raw, err := p.do(ctx, http.MethodPost, openAIChatCompletionsURL(p.base(cfg)), body, cfg)
	if err != nil {
		return ir.Response{}, err
	}
	return parseOAIResponse(raw, p.id)
}

func (p OpenAI) Stream(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (<-chan ir.Event, error) {
	req = adapter.RequestWithConfiguredModel(req, cfg)
	req.Stream = true
	body, err := ir.DenormalizeOAIRequest(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIChatCompletionsURL(p.base(cfg)), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if err := p.setHeaders(ctx, httpReq, cfg); err != nil {
		return nil, err
	}
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, &adapter.Error{Class: adapter.Network, Provider: p.id, UpstreamID: cfg.UpstreamID, Message: err.Error(), Retryable: true, Wrapped: err}
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		msg, _ := io.ReadAll(resp.Body)
		class := adapter.ClassifyHTTP(resp.StatusCode)
		return nil, &adapter.Error{Class: class, Provider: p.id, UpstreamID: cfg.UpstreamID, HTTPStatus: resp.StatusCode, Message: string(msg), Retryable: adapter.Retryable(class)}
	}
	ch := make(chan ir.Event)
	go parseOAIStream(resp.Body, ch)
	return ch, nil
}

func (p OpenAI) CountTokens(_ context.Context, req ir.Request, cfg adapter.ProviderConfig) (int, error) {
	model := adapter.RawString(cfg.Raw, "model")
	if model == "" {
		model = req.ModelAlias
	}
	return budget.EstimateInputTokens(req, p.id, model)
}

func (p OpenAI) HealthCheck(ctx context.Context, cfg adapter.ProviderConfig) error {
	req := ir.Request{ModelAlias: valueOr(adapter.RawString(cfg.Raw, "health_model"), "gpt-5-nano"), MaxTokens: 1, Messages: []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "ping"}}}}}
	_, err := p.Chat(ctx, req, cfg)
	return err
}

func (p OpenAI) Capabilities() adapter.Capabilities {
	return adapter.Capabilities{Streaming: true, ToolUse: true, Vision: true, StabilityClass: adapter.Stable, Category: p.category}
}

func (p OpenAI) do(ctx context.Context, method, url string, body []byte, cfg adapter.ProviderConfig) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if err := p.setHeaders(ctx, req, cfg); err != nil {
		return nil, err
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

func (p OpenAI) base(cfg adapter.ProviderConfig) string {
	if base := adapter.RawString(cfg.Raw, "base_url"); base != "" {
		return strings.TrimRight(base, "/")
	}
	return p.baseURL
}

func (p OpenAI) setHeaders(ctx context.Context, req *http.Request, cfg adapter.ProviderConfig) error {
	apiKey, err := p.apiKeyValue(ctx, cfg)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	return nil
}

func (p OpenAI) apiKeyValue(ctx context.Context, cfg adapter.ProviderConfig) (string, error) {
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
	return p.apiKey, nil
}

func parseOAIResponse(raw []byte, provider string) (ir.Response, error) {
	var in struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content          any    `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
				ToolCalls        []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
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
	if len(in.Choices) > 0 {
		text := oaiContentText(in.Choices[0].Message.Content)
		if text == "" {
			text = in.Choices[0].Message.ReasoningContent
		}
		if text != "" {
			content = append(content, ir.ContentBlock{Type: ir.ContentText, Text: text})
		}
		for _, call := range in.Choices[0].Message.ToolCalls {
			args := map[string]any{}
			_ = json.Unmarshal([]byte(call.Function.Arguments), &args)
			content = append(content, ir.ContentBlock{Type: ir.ContentToolUse, ToolUse: &ir.ToolUse{ID: call.ID, Name: call.Function.Name, Arguments: args}})
		}
	}
	return ir.Response{Version: ir.Version, ID: in.ID, UpstreamProvider: provider, UpstreamModel: in.Model, StopReason: stopFromOAI(in.Choices), Content: content, Usage: ir.Usage{InputTokens: in.Usage.PromptTokens, OutputTokens: in.Usage.CompletionTokens}}, nil
}

func oaiContentText(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []any:
		var text string
		for _, item := range v {
			part, ok := item.(map[string]any)
			if !ok {
				continue
			}
			switch part["type"] {
			case "text", "output_text":
				if s, ok := part["text"].(string); ok {
					text += s
				}
			}
		}
		return text
	case map[string]any:
		if s, ok := v["text"].(string); ok {
			return s
		}
	}
	return ""
}

func parseOAIStream(body io.ReadCloser, ch chan<- ir.Event) {
	defer body.Close()
	defer close(ch)
	ch <- ir.Event{Version: ir.Version, Type: ir.EventStart}
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			ch <- ir.Event{Version: ir.Version, Type: ir.EventStop, StopReason: ir.StopEndTurn}
			return
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(data), &chunk) == nil && len(chunk.Choices) > 0 {
			if chunk.Choices[0].Delta.Content != "" {
				ch <- ir.Event{Version: ir.Version, Type: ir.EventContentBlockDelta, Delta: &ir.ContentBlock{Type: ir.ContentText, Text: chunk.Choices[0].Delta.Content}}
			}
			if chunk.Choices[0].FinishReason != "" {
				ch <- ir.Event{Version: ir.Version, Type: ir.EventStop, StopReason: stopFromOAIFinish(chunk.Choices[0].FinishReason)}
			}
		}
	}
}

func stopFromOAI(choices []struct {
	Message struct {
		Content          any    `json:"content"`
		ReasoningContent string `json:"reasoning_content"`
		ToolCalls        []struct {
			ID       string `json:"id"`
			Function struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"function"`
		} `json:"tool_calls"`
	} `json:"message"`
	FinishReason string `json:"finish_reason"`
}) string {
	if len(choices) == 0 {
		return ir.StopEndTurn
	}
	return stopFromOAIFinish(choices[0].FinishReason)
}

func stopFromOAIFinish(reason string) string {
	switch reason {
	case "length":
		return ir.StopMaxTokens
	case "tool_calls":
		return ir.StopToolUse
	default:
		return ir.StopEndTurn
	}
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
