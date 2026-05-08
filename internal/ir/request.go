package ir

import "time"

const Version = "v1"

const (
	IngressOpenAI    = "openai"
	IngressAnthropic = "anthropic"
)

const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

const (
	ContentText       = "text"
	ContentImage      = "image"
	ContentToolUse    = "tool_use"
	ContentToolResult = "tool_result"
	ContentDocument   = "document"
)

type Request struct {
	Version       string            `json:"version"`
	ID            string            `json:"id"`
	BridgeKeyID   string            `json:"bridge_key_id,omitempty"`
	ReceivedAt    time.Time         `json:"received_at"`
	IngressFormat string            `json:"ingress_format"`
	ModelAlias    string            `json:"model_alias"`
	System        string            `json:"system,omitempty"`
	Messages      []Message         `json:"messages"`
	Tools         []ToolDef         `json:"tools,omitempty"`
	MCPServers    []MCPServer       `json:"mcp_servers,omitempty"`
	MaxTokens     int               `json:"max_tokens,omitempty"`
	Temperature   *float32          `json:"temperature,omitempty"`
	TopP          *float32          `json:"top_p,omitempty"`
	StopSequences []string          `json:"stop_sequences,omitempty"`
	Stream        bool              `json:"stream"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Extras        map[string]any    `json:"extras,omitempty"`
}

type Message struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

type ContentBlock struct {
	Type       string         `json:"type"`
	Text       string         `json:"text,omitempty"`
	ImageURL   string         `json:"image_url,omitempty"`
	ImageB64   []byte         `json:"image_b64,omitempty"`
	MediaType  string         `json:"media_type,omitempty"`
	ToolUse    *ToolUse       `json:"tool_use,omitempty"`
	ToolResult *ToolResult    `json:"tool_result,omitempty"`
	Document   *Document      `json:"document,omitempty"`
	Extras     map[string]any `json:"extras,omitempty"`
}

func NewRequest(id, ingressFormat string, receivedAt time.Time) Request {
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}
	return Request{
		Version:       Version,
		ID:            id,
		ReceivedAt:    receivedAt.UTC(),
		IngressFormat: ingressFormat,
		Messages:      []Message{},
		Metadata:      map[string]string{},
		Extras:        map[string]any{},
	}
}
