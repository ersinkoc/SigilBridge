package ir

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

type oaiChatResponse struct {
	ID      string      `json:"id"`
	Object  string      `json:"object"`
	Created int64       `json:"created"`
	Model   string      `json:"model"`
	Choices []oaiChoice `json:"choices"`
	Usage   oaiUsage    `json:"usage"`
}

type oaiChoice struct {
	Index        int        `json:"index"`
	Message      oaiMessage `json:"message,omitempty"`
	Delta        oaiMessage `json:"delta,omitempty"`
	FinishReason string     `json:"finish_reason,omitempty"`
}

type oaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func DenormalizeOAIRequest(req Request) ([]byte, error) {
	out := oaiChatRequest{
		Model:       req.ModelAlias,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
		Tools:       denormalizeOAITools(req.Tools),
		Metadata:    req.Metadata,
	}
	if len(req.StopSequences) == 1 {
		out.Stop = req.StopSequences[0]
	} else if len(req.StopSequences) > 1 {
		stops := make([]any, len(req.StopSequences))
		for i, stop := range req.StopSequences {
			stops[i] = stop
		}
		out.Stop = stops
	}
	if req.System != "" {
		raw, _ := json.Marshal(req.System)
		out.Messages = append(out.Messages, oaiMessage{Role: RoleSystem, Content: raw})
	}
	for _, msg := range req.Messages {
		out.Messages = append(out.Messages, denormalizeOAIMessage(msg))
	}
	if v, ok := req.Extras["tool_choice"]; ok {
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshal tool_choice: %w", err)
		}
		out.ToolChoice = raw
	}
	if v, ok := req.Extras["response_format"]; ok {
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshal response_format: %w", err)
		}
		out.ResponseFormat = raw
	}
	if user := req.Metadata["user"]; user != "" {
		out.User = user
	}
	return json.Marshal(out)
}

func DenormalizeOAIResponse(resp Response) ([]byte, error) {
	message := oaiMessage{Role: RoleAssistant}
	content, toolCalls, err := denormalizeOAIContentAndTools(resp.Content)
	if err != nil {
		return nil, err
	}
	message.Content = content
	message.ToolCalls = toolCalls
	out := oaiChatResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   resp.UpstreamModel,
		Choices: []oaiChoice{{
			Index:        0,
			Message:      message,
			FinishReason: oaiFinishReason(resp.StopReason),
		}},
		Usage: oaiUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}
	return json.Marshal(out)
}

func EventsToOAIStream(responseID, model string, events <-chan Event) ([]byte, error) {
	var buf bytes.Buffer
	for event := range events {
		payload := map[string]any{
			"id":      responseID,
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   model,
		}
		choice := map[string]any{"index": event.Index}
		switch event.Type {
		case EventStart:
			choice["delta"] = map[string]any{"role": RoleAssistant}
		case EventDelta, EventContentBlockDelta:
			if event.Delta != nil {
				switch event.Delta.Type {
				case ContentText:
					choice["delta"] = map[string]any{"content": event.Delta.Text}
				case ContentToolUse:
					if event.Delta.ToolUse != nil {
						choice["delta"] = map[string]any{
							"role": RoleAssistant,
							"tool_calls": []map[string]any{
								{
									"id":       event.Delta.ToolUse.ID,
									"index":    event.Index,
									"type":     "function",
									"function": map[string]any{
										"name":      event.Delta.ToolUse.Name,
										"arguments": event.Delta.ToolUse.Arguments["__partial"],
									},
								},
							},
						}
					} else {
						choice["delta"] = map[string]any{}
					}
				default:
					choice["delta"] = map[string]any{}
				}
			} else {
				choice["delta"] = map[string]any{}
			}
		case EventStop:
			choice["delta"] = map[string]any{}
			choice["finish_reason"] = oaiFinishReason(event.StopReason)
		case EventError:
			choice["delta"] = map[string]any{}
			payload["error"] = event.Error
		default:
			choice["delta"] = map[string]any{}
		}
		payload["choices"] = []any{choice}
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		buf.WriteString("data: ")
		buf.Write(raw)
		buf.WriteString("\n\n")
	}
	buf.WriteString("data: [DONE]\n\n")
	return buf.Bytes(), nil
}

func denormalizeOAIMessage(msg Message) oaiMessage {
	out := oaiMessage{Role: msg.Role}
	if msg.Role == RoleTool {
		for _, block := range msg.Content {
			if block.ToolResult != nil {
				out.ToolCallID = block.ToolResult.ToolUseID
				out.Content = marshalOAIContent(block.ToolResult.Content)
				return out
			}
		}
	}
	out.Content = marshalOAIContent(msg.Content)
	for _, block := range msg.Content {
		if block.ToolUse == nil {
			continue
		}
		args, _ := json.Marshal(block.ToolUse.Arguments)
		out.ToolCalls = append(out.ToolCalls, oaiToolCall{
			ID:   block.ToolUse.ID,
			Type: "function",
			Function: oaiFunctionCall{
				Name:      block.ToolUse.Name,
				Arguments: string(args),
			},
		})
	}
	return out
}

func denormalizeOAITools(tools []ToolDef) []oaiTool {
	out := make([]oaiTool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, oaiTool{Type: "function", Function: oaiFunction{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.InputSchema,
		}})
	}
	return out
}

func marshalOAIContent(blocks []ContentBlock) json.RawMessage {
	textOnly := true
	var text string
	for _, block := range blocks {
		if block.Type != ContentText {
			textOnly = false
			break
		}
		text += block.Text
	}
	if textOnly {
		raw, _ := json.Marshal(text)
		return raw
	}
	parts := make([]map[string]any, 0, len(blocks))
	for _, block := range blocks {
		switch block.Type {
		case ContentText:
			parts = append(parts, map[string]any{"type": "text", "text": block.Text})
		case ContentImage:
			parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]any{"url": block.ImageURL}})
		}
	}
	raw, _ := json.Marshal(parts)
	return raw
}

func denormalizeOAIContentAndTools(blocks []ContentBlock) (json.RawMessage, []oaiToolCall, error) {
	contentBlocks := make([]ContentBlock, 0, len(blocks))
	var toolCalls []oaiToolCall
	for _, block := range blocks {
		if block.ToolUse != nil {
			args, err := json.Marshal(block.ToolUse.Arguments)
			if err != nil {
				return nil, nil, err
			}
			toolCalls = append(toolCalls, oaiToolCall{
				ID:   block.ToolUse.ID,
				Type: "function",
				Function: oaiFunctionCall{
					Name:      block.ToolUse.Name,
					Arguments: string(args),
				},
			})
			continue
		}
		contentBlocks = append(contentBlocks, block)
	}
	return marshalOAIContent(contentBlocks), toolCalls, nil
}

func oaiFinishReason(stopReason string) string {
	switch stopReason {
	case StopMaxTokens:
		return "length"
	case StopToolUse:
		return "tool_calls"
	case StopSequence:
		return "stop"
	case StopError:
		return "error"
	default:
		return "stop"
	}
}
