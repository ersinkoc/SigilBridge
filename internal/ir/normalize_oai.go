package ir

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
)

type oaiChatRequest struct {
	Model          string            `json:"model"`
	Messages       []oaiMessage      `json:"messages"`
	Temperature    *float32          `json:"temperature,omitempty"`
	TopP           *float32          `json:"top_p,omitempty"`
	MaxTokens      int               `json:"max_tokens,omitempty"`
	Stop           any               `json:"stop,omitempty"`
	Stream         bool              `json:"stream,omitempty"`
	Tools          []oaiTool         `json:"tools,omitempty"`
	ToolChoice     json.RawMessage   `json:"tool_choice,omitempty"`
	ResponseFormat json.RawMessage   `json:"response_format,omitempty"`
	User           string            `json:"user,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

type oaiMessage struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content,omitempty"`
	Name       string          `json:"name,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	ToolCalls  []oaiToolCall   `json:"tool_calls,omitempty"`
}

type oaiTool struct {
	Type     string      `json:"type"`
	Function oaiFunction `json:"function"`
}

type oaiFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type oaiToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function oaiFunctionCall `json:"function"`
}

type oaiFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func NormalizeOAIRequest(raw []byte, bridgeKeyID string, receivedAt time.Time) (Request, error) {
	var in oaiChatRequest
	if err := json.Unmarshal(raw, &in); err != nil {
		return Request{}, fmt.Errorf("parse OpenAI chat request: %w", err)
	}
	req := NewRequest(ulid.Make().String(), IngressOpenAI, receivedAt)
	req.BridgeKeyID = bridgeKeyID
	req.ModelAlias = in.Model
	req.Temperature = in.Temperature
	req.TopP = in.TopP
	req.MaxTokens = in.MaxTokens
	req.Stream = in.Stream
	req.StopSequences = parseStopSequences(in.Stop)
	req.Metadata = in.Metadata
	if req.Metadata == nil {
		req.Metadata = map[string]string{}
	}
	if in.User != "" {
		req.Metadata["user"] = in.User
	}
	if len(in.ToolChoice) > 0 {
		var choice any
		if err := json.Unmarshal(in.ToolChoice, &choice); err != nil {
			return Request{}, fmt.Errorf("parse OpenAI tool_choice: %w", err)
		}
		req.Extras["tool_choice"] = choice
	}
	if len(in.ResponseFormat) > 0 {
		var responseFormat any
		if err := json.Unmarshal(in.ResponseFormat, &responseFormat); err != nil {
			return Request{}, fmt.Errorf("parse OpenAI response_format: %w", err)
		}
		req.Extras["response_format"] = responseFormat
	}
	for _, tool := range in.Tools {
		if tool.Type != "" && tool.Type != "function" {
			continue
		}
		req.Tools = append(req.Tools, ToolDef{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			InputSchema: tool.Function.Parameters,
		})
	}
	for _, msg := range in.Messages {
		blocks, err := normalizeOAIContent(msg.Content)
		if err != nil {
			return Request{}, err
		}
		if msg.Role == RoleSystem || msg.Role == "developer" {
			text := blocksText(blocks)
			if req.System == "" {
				req.System = text
			} else if text != "" {
				req.System += "\n" + text
			}
			continue
		}
		for _, call := range msg.ToolCalls {
			args := map[string]any{}
			if call.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
					return Request{}, fmt.Errorf("parse OpenAI tool call arguments: %w", err)
				}
			}
			blocks = append(blocks, ContentBlock{
				Type: ContentToolUse,
				ToolUse: &ToolUse{
					ID:        call.ID,
					Name:      call.Function.Name,
					Arguments: args,
				},
			})
		}
		if msg.Role == RoleTool {
			blocks = []ContentBlock{{
				Type: ContentToolResult,
				ToolResult: &ToolResult{
					ToolUseID: msg.ToolCallID,
					Content:   blocks,
				},
			}}
		}
		req.Messages = append(req.Messages, Message{Role: msg.Role, Content: blocks})
	}
	return req, nil
}

func normalizeOAIContent(raw json.RawMessage) ([]ContentBlock, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if text == "" {
			return nil, nil
		}
		return []ContentBlock{{Type: ContentText, Text: text}}, nil
	}
	var parts []map[string]any
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil, fmt.Errorf("parse OpenAI message content: %w", err)
	}
	blocks := make([]ContentBlock, 0, len(parts))
	for _, part := range parts {
		typ, _ := part["type"].(string)
		switch typ {
		case "text", "input_text":
			blocks = append(blocks, ContentBlock{Type: ContentText, Text: stringField(part, "text")})
		case "image_url":
			imageURL, _ := part["image_url"].(map[string]any)
			blocks = append(blocks, ContentBlock{Type: ContentImage, ImageURL: stringField(imageURL, "url")})
		case "input_image":
			blocks = append(blocks, ContentBlock{Type: ContentImage, ImageURL: stringField(part, "image_url")})
		}
	}
	return blocks, nil
}

func parseStopSequences(value any) []string {
	switch v := value.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func blocksText(blocks []ContentBlock) string {
	var text string
	for _, block := range blocks {
		if block.Type == ContentText {
			if text != "" {
				text += "\n"
			}
			text += block.Text
		}
	}
	return text
}

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	value, _ := m[key].(string)
	return value
}
