package ir

type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
	Extras      map[string]any `json:"extras,omitempty"`
}

type ToolUse struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type ToolResult struct {
	ToolUseID string         `json:"tool_use_id"`
	Content   []ContentBlock `json:"content,omitempty"`
	IsError   bool           `json:"is_error,omitempty"`
}

type Document struct {
	Name      string `json:"name,omitempty"`
	MediaType string `json:"media_type,omitempty"`
	Data      []byte `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

type MCPServer struct {
	Type   string         `json:"type"`
	URL    string         `json:"url,omitempty"`
	Name   string         `json:"name,omitempty"`
	Extras map[string]any `json:"extras,omitempty"`
}

type ErrorClass int

const (
	ErrorSuccess ErrorClass = iota
	ErrorClient
	ErrorAuth
	ErrorConfig
	ErrorRateLimited
	ErrorServer
	ErrorTimeout
	ErrorNetwork
	ErrorClientCanceled
	ErrorBotDetected
	ErrorBudgetExceeded
)

type Error struct {
	Type       string     `json:"type"`
	Message    string     `json:"message"`
	Retryable  bool       `json:"retryable"`
	UpstreamID string     `json:"upstream_id,omitempty"`
	Class      ErrorClass `json:"class"`
}
