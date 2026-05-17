package ir

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

type anthropicResponse struct {
	ID           string           `json:"id"`
	Type         string           `json:"type"`
	Role         string           `json:"role"`
	Model        string           `json:"model"`
	Content      []map[string]any `json:"content"`
	StopReason   string           `json:"stop_reason,omitempty"`
	StopSequence string           `json:"stop_sequence,omitempty"`
	Usage        anthropicUsage   `json:"usage"`
}

type anthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
}

func DenormalizeAnthropicRequest(req Request) ([]byte, error) {
	out := anthropicRequest{
		Model:         req.ModelAlias,
		MaxTokens:     req.MaxTokens,
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		StopSequences: req.StopSequences,
		Stream:        req.Stream,
		Metadata:      req.Metadata,
		Tools:         denormalizeAnthropicTools(req.Tools),
	}
	if req.System != "" {
		out.System, _ = json.Marshal(req.System)
	}
	for _, server := range req.MCPServers {
		m := map[string]any{"type": server.Type}
		if server.URL != "" {
			m["url"] = server.URL
		}
		if server.Name != "" {
			m["name"] = server.Name
		}
		for key, value := range server.Extras {
			m[key] = value
		}
		out.MCPServers = append(out.MCPServers, m)
	}
	for _, msg := range req.Messages {
		content, err := denormalizeAnthropicContent(msg.Content)
		if err != nil {
			return nil, err
		}
		out.Messages = append(out.Messages, anthropicMessage{Role: msg.Role, Content: content})
	}
	if v, ok := req.Extras["tool_choice"]; ok {
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshal Anthropic tool_choice: %w", err)
		}
		out.ToolChoice = raw
	}
	return json.Marshal(out)
}

func DenormalizeAnthropicResponse(resp Response) ([]byte, error) {
	content, err := denormalizeAnthropicContent(resp.Content)
	if err != nil {
		return nil, err
	}
	var blocks []map[string]any
	if err := json.Unmarshal(content, &blocks); err != nil {
		return nil, err
	}
	out := anthropicResponse{
		ID:         resp.ID,
		Type:       "message",
		Role:       RoleAssistant,
		Model:      resp.UpstreamModel,
		Content:    blocks,
		StopReason: resp.StopReason,
		Usage: anthropicUsage{
			InputTokens:              resp.Usage.InputTokens,
			OutputTokens:             resp.Usage.OutputTokens,
			CacheReadInputTokens:     resp.Usage.CacheReadTokens,
			CacheCreationInputTokens: resp.Usage.CacheWriteTokens,
		},
	}
	return json.Marshal(out)
}

func EventsToAnthropicStream(responseID, model string, events <-chan Event) ([]byte, error) {
	var buf bytes.Buffer
	for event := range events {
		typ := event.Type
		payload := map[string]any{"type": typ}
		switch event.Type {
		case EventStart:
			typ = "message_start"
			payload = map[string]any{
				"type": "message_start",
				"message": map[string]any{
					"id":            responseID,
					"type":          "message",
					"role":          RoleAssistant,
					"model":         model,
					"content":       []any{},
					"stop_reason":   nil,
					"stop_sequence": nil,
				},
			}
		case EventContentBlockStart:
			typ = "content_block_start"
			block, err := anthropicBlock(event.Delta)
			if err != nil {
				return nil, err
			}
			payload = map[string]any{"type": typ, "index": event.Index, "content_block": block}
		case EventDelta, EventContentBlockDelta:
			typ = "content_block_delta"
			if event.Delta != nil {
				switch event.Delta.Type {
				case ContentText:
					payload = map[string]any{"type": typ, "index": event.Index, "delta": map[string]any{"type": "text_delta", "text": event.Delta.Text}}
				case ContentToolUse:
					delta := map[string]any{"type": "input_json_delta"}
					if event.Delta.ToolUse != nil {
						if event.Delta.ToolUse.Name != "" {
							delta["name"] = event.Delta.ToolUse.Name
						}
						if event.Delta.ToolUse.ID != "" {
							delta["id"] = event.Delta.ToolUse.ID
						}
						if partial, ok := event.Delta.ToolUse.Arguments["__partial"].(string); ok {
							delta["partial_json"] = partial
						}
					}
					payload = map[string]any{"type": typ, "index": event.Index, "delta": delta}
				default:
					payload = map[string]any{"type": typ, "index": event.Index, "delta": map[string]any{"type": "text_delta", "text": ""}}
				}
			} else {
				payload = map[string]any{"type": typ, "index": event.Index, "delta": map[string]any{"type": "text_delta", "text": ""}}
			}
		case EventContentBlockStop:
			typ = "content_block_stop"
			payload = map[string]any{"type": typ, "index": event.Index}
		case EventStop:
			typ = "message_stop"
			payload = map[string]any{"type": typ}
		case EventUsage:
			typ = "message_delta"
			payload = map[string]any{"type": typ, "delta": map[string]any{"stop_reason": event.StopReason}, "usage": event.Usage}
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		buf.WriteString("event: ")
		buf.WriteString(typ)
		buf.WriteString("\n")
		buf.WriteString("data: ")
		buf.Write(raw)
		buf.WriteString("\n\n")
	}
	return buf.Bytes(), nil
}

func denormalizeAnthropicTools(tools []ToolDef) []anthropicTool {
	out := make([]anthropicTool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, anthropicTool{Name: tool.Name, Description: tool.Description, InputSchema: tool.InputSchema})
	}
	return out
}

func denormalizeAnthropicContent(blocks []ContentBlock) (json.RawMessage, error) {
	out := make([]map[string]any, 0, len(blocks))
	for i := range blocks {
		block, err := anthropicBlock(&blocks[i])
		if err != nil {
			return nil, err
		}
		if block != nil {
			out = append(out, block)
		}
	}
	return json.Marshal(out)
}

func anthropicBlock(block *ContentBlock) (map[string]any, error) {
	if block == nil {
		return nil, nil
	}
	out := map[string]any{}
	for key, value := range block.Extras {
		out[key] = value
	}
	switch block.Type {
	case ContentText:
		out["type"] = "text"
		out["text"] = block.Text
	case ContentImage:
		out["type"] = "image"
		source := map[string]any{"media_type": block.MediaType}
		if len(block.ImageB64) > 0 {
			source["type"] = "base64"
			source["data"] = base64.StdEncoding.EncodeToString(block.ImageB64)
		} else {
			source["type"] = "url"
			source["url"] = block.ImageURL
		}
		out["source"] = source
	case ContentToolUse:
		if block.ToolUse == nil {
			return nil, nil
		}
		out["type"] = "tool_use"
		out["id"] = block.ToolUse.ID
		out["name"] = block.ToolUse.Name
		out["input"] = block.ToolUse.Arguments
	case ContentToolResult:
		if block.ToolResult == nil {
			return nil, nil
		}
		content, err := denormalizeAnthropicContent(block.ToolResult.Content)
		if err != nil {
			return nil, err
		}
		var contentAny any
		if err := json.Unmarshal(content, &contentAny); err != nil {
			return nil, err
		}
		out["type"] = "tool_result"
		out["tool_use_id"] = block.ToolResult.ToolUseID
		out["content"] = contentAny
		if block.ToolResult.IsError {
			out["is_error"] = true
		}
	case ContentDocument:
		if block.Document == nil {
			return nil, nil
		}
		out["type"] = "document"
		if block.Document.Name != "" {
			out["title"] = block.Document.Name
		}
		source := map[string]any{"media_type": block.Document.MediaType}
		if len(block.Document.Data) > 0 {
			source["type"] = "base64"
			source["data"] = base64.StdEncoding.EncodeToString(block.Document.Data)
		} else {
			source["type"] = "url"
			source["url"] = block.Document.URL
		}
		out["source"] = source
	}
	return out, nil
}
