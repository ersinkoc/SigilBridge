package ir

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
)

type anthropicRequest struct {
	Model         string             `json:"model"`
	Messages      []anthropicMessage `json:"messages"`
	System        json.RawMessage    `json:"system,omitempty"`
	MaxTokens     int                `json:"max_tokens,omitempty"`
	Temperature   *float32           `json:"temperature,omitempty"`
	TopP          *float32           `json:"top_p,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
	Tools         []anthropicTool    `json:"tools,omitempty"`
	MCPServers    []map[string]any   `json:"mcp_servers,omitempty"`
	Metadata      map[string]string  `json:"metadata,omitempty"`
	ToolChoice    json.RawMessage    `json:"tool_choice,omitempty"`
}

type anthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
}

func NormalizeAnthropicRequest(raw []byte, bridgeKeyID string, receivedAt time.Time) (Request, error) {
	var in anthropicRequest
	if err := json.Unmarshal(raw, &in); err != nil {
		return Request{}, fmt.Errorf("parse Anthropic request: %w", err)
	}
	req := NewRequest(ulid.Make().String(), IngressAnthropic, receivedAt)
	req.BridgeKeyID = bridgeKeyID
	req.ModelAlias = in.Model
	req.System = parseAnthropicSystem(in.System)
	req.MaxTokens = in.MaxTokens
	req.Temperature = in.Temperature
	req.TopP = in.TopP
	req.StopSequences = in.StopSequences
	req.Stream = in.Stream
	req.Metadata = in.Metadata
	if req.Metadata == nil {
		req.Metadata = map[string]string{}
	}
	if len(in.ToolChoice) > 0 {
		var choice any
		if err := json.Unmarshal(in.ToolChoice, &choice); err != nil {
			return Request{}, fmt.Errorf("parse Anthropic tool_choice: %w", err)
		}
		req.Extras["tool_choice"] = choice
	}
	for _, tool := range in.Tools {
		req.Tools = append(req.Tools, ToolDef{Name: tool.Name, Description: tool.Description, InputSchema: tool.InputSchema})
	}
	for _, server := range in.MCPServers {
		req.MCPServers = append(req.MCPServers, MCPServer{
			Type:   stringField(server, "type"),
			URL:    stringField(server, "url"),
			Name:   stringField(server, "name"),
			Extras: copyExtras(server, "type", "url", "name"),
		})
	}
	for _, msg := range in.Messages {
		blocks, err := normalizeAnthropicContent(msg.Content)
		if err != nil {
			return Request{}, err
		}
		req.Messages = append(req.Messages, Message{Role: msg.Role, Content: blocks})
	}
	return req, nil
}

func parseAnthropicSystem(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	blocks, err := normalizeAnthropicContent(raw)
	if err != nil {
		return ""
	}
	return blocksText(blocks)
}

func normalizeAnthropicContent(raw json.RawMessage) ([]ContentBlock, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return []ContentBlock{{Type: ContentText, Text: text}}, nil
	}
	var parts []map[string]any
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil, fmt.Errorf("parse Anthropic content: %w", err)
	}
	blocks := make([]ContentBlock, 0, len(parts))
	for _, part := range parts {
		typ := stringField(part, "type")
		extras := copyExtras(part, "type", "text", "source", "id", "name", "input", "tool_use_id", "content", "is_error")
		switch typ {
		case "text":
			blocks = append(blocks, ContentBlock{Type: ContentText, Text: stringField(part, "text"), Extras: extras})
		case "image":
			source, _ := part["source"].(map[string]any)
			block := ContentBlock{Type: ContentImage, MediaType: stringField(source, "media_type"), Extras: extras}
			switch stringField(source, "type") {
			case "url":
				block.ImageURL = stringField(source, "url")
			case "base64":
				decoded, err := base64.StdEncoding.DecodeString(stringField(source, "data"))
				if err != nil {
					return nil, fmt.Errorf("decode Anthropic image data: %w", err)
				}
				block.ImageB64 = decoded
			}
			blocks = append(blocks, block)
		case "tool_use":
			args, _ := part["input"].(map[string]any)
			blocks = append(blocks, ContentBlock{Type: ContentToolUse, Extras: extras, ToolUse: &ToolUse{
				ID:        stringField(part, "id"),
				Name:      stringField(part, "name"),
				Arguments: args,
			}})
		case "tool_result":
			contentRaw, _ := json.Marshal(part["content"])
			content, err := normalizeAnthropicContent(contentRaw)
			if err != nil {
				var contentText string
				if err := json.Unmarshal(contentRaw, &contentText); err == nil {
					content = []ContentBlock{{Type: ContentText, Text: contentText}}
				} else {
					return nil, err
				}
			}
			isError, _ := part["is_error"].(bool)
			blocks = append(blocks, ContentBlock{Type: ContentToolResult, Extras: extras, ToolResult: &ToolResult{
				ToolUseID: stringField(part, "tool_use_id"),
				Content:   content,
				IsError:   isError,
			}})
		case "document":
			source, _ := part["source"].(map[string]any)
			doc := &Document{Name: stringField(part, "title"), MediaType: stringField(source, "media_type"), URL: stringField(source, "url")}
			if data := stringField(source, "data"); data != "" {
				decoded, err := base64.StdEncoding.DecodeString(data)
				if err != nil {
					return nil, fmt.Errorf("decode Anthropic document data: %w", err)
				}
				doc.Data = decoded
			}
			blocks = append(blocks, ContentBlock{Type: ContentDocument, Extras: extras, Document: doc})
		}
	}
	return blocks, nil
}

func copyExtras(m map[string]any, skip ...string) map[string]any {
	if len(m) == 0 {
		return nil
	}
	skipSet := make(map[string]struct{}, len(skip))
	for _, key := range skip {
		skipSet[key] = struct{}{}
	}
	out := map[string]any{}
	for key, value := range m {
		if _, ok := skipSet[key]; ok {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
