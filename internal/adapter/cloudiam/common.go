package cloudiam

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/ir"
)

func httpError(provider string, cfg adapter.ProviderConfig, status int, body []byte) error {
	class := adapter.ClassifyHTTP(status)
	return &adapter.Error{Class: class, Provider: provider, UpstreamID: cfg.UpstreamID, HTTPStatus: status, Message: string(body), Retryable: adapter.Retryable(class)}
}

func readHTTP(provider string, cfg adapter.ProviderConfig, resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, httpError(provider, cfg, resp.StatusCode, raw)
	}
	return raw, nil
}

func parseOpenAIShape(raw []byte, provider string) (ir.Response, error) {
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
		return ir.Response{}, fmt.Errorf("parse OpenAI-shaped response: %w", err)
	}
	content := []ir.ContentBlock{}
	if len(in.Choices) > 0 && in.Choices[0].Message.Content != "" {
		content = append(content, ir.ContentBlock{Type: ir.ContentText, Text: in.Choices[0].Message.Content})
	}
	return ir.Response{Version: ir.Version, ID: in.ID, UpstreamProvider: provider, UpstreamModel: in.Model, StopReason: ir.StopEndTurn, Content: content, Usage: ir.Usage{InputTokens: in.Usage.PromptTokens, OutputTokens: in.Usage.CompletionTokens}}, nil
}

func parseAnthropicLike(raw []byte, provider string) (ir.Response, error) {
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
		return ir.Response{}, fmt.Errorf("parse Anthropic-shaped response: %w", err)
	}
	if in.ID == "" && len(in.Content) == 0 {
		return ir.Response{}, fmt.Errorf("response is not Anthropic-shaped")
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
	stopReason := in.StopReason
	if stopReason == "" {
		stopReason = ir.StopEndTurn
	}
	return ir.Response{Version: ir.Version, ID: in.ID, UpstreamProvider: provider, UpstreamModel: in.Model, StopReason: stopReason, Content: content, Usage: ir.Usage{InputTokens: in.Usage.InputTokens, OutputTokens: in.Usage.OutputTokens}}, nil
}

func streamFromChat(ctx context.Context, chat func(context.Context) (ir.Response, error)) (<-chan ir.Event, error) {
	resp, err := chat(ctx)
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
