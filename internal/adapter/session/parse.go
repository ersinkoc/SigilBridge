package session

import (
	"encoding/json"
	"fmt"

	"github.com/sigilbridge/sigilbridge/internal/ir"
)

func parseAnthropic(raw []byte, provider string) (ir.Response, error) {
	var in struct {
		ID         string `json:"id"`
		Model      string `json:"model"`
		StopReason string `json:"stop_reason"`
		Content    []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return ir.Response{}, fmt.Errorf("parse Anthropic web response: %w", err)
	}
	content := []ir.ContentBlock{}
	for _, block := range in.Content {
		if block.Type == "text" {
			content = append(content, ir.ContentBlock{Type: ir.ContentText, Text: block.Text})
		}
	}
	return ir.Response{Version: ir.Version, ID: in.ID, UpstreamProvider: provider, UpstreamModel: in.Model, StopReason: valueOr(in.StopReason, ir.StopEndTurn), Content: content, Usage: ir.Usage{InputTokens: in.Usage.InputTokens, OutputTokens: in.Usage.OutputTokens}}, nil
}

func parseOpenAI(raw []byte, provider string) (ir.Response, error) {
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
		return ir.Response{}, fmt.Errorf("parse OpenAI web response: %w", err)
	}
	content := []ir.ContentBlock{}
	if len(in.Choices) > 0 && in.Choices[0].Message.Content != "" {
		content = append(content, ir.ContentBlock{Type: ir.ContentText, Text: in.Choices[0].Message.Content})
	}
	stop := ir.StopEndTurn
	if len(in.Choices) > 0 && in.Choices[0].FinishReason == "length" {
		stop = ir.StopMaxTokens
	}
	return ir.Response{Version: ir.Version, ID: in.ID, UpstreamProvider: provider, UpstreamModel: in.Model, StopReason: stop, Content: content, Usage: ir.Usage{InputTokens: in.Usage.PromptTokens, OutputTokens: in.Usage.CompletionTokens}}, nil
}
