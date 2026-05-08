package ir

const (
	StopEndTurn   = "end_turn"
	StopMaxTokens = "max_tokens"
	StopSequence  = "stop_sequence"
	StopToolUse   = "tool_use"
	StopError     = "error"
)

type Response struct {
	Version          string         `json:"version"`
	ID               string         `json:"id"`
	UpstreamProvider string         `json:"upstream_provider"`
	UpstreamID       string         `json:"upstream_id,omitempty"`
	UpstreamModel    string         `json:"upstream_model"`
	StopReason       string         `json:"stop_reason"`
	Content          []ContentBlock `json:"content"`
	Usage            Usage          `json:"usage"`
	LatencyMs        int64          `json:"latency_ms"`
	TTFBMs           int64          `json:"ttfb_ms"`
	CostCents        int            `json:"cost_cents"`
	Error            *Error         `json:"error,omitempty"`
}

type Usage struct {
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
	CacheReadTokens  int `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int `json:"cache_write_tokens,omitempty"`
}
