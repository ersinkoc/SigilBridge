package ir

const (
	EventStart             = "start"
	EventDelta             = "delta"
	EventContentBlockStart = "content_block_start"
	EventContentBlockDelta = "content_block_delta"
	EventContentBlockStop  = "content_block_stop"
	EventMessageDelta      = "message_delta"
	EventStop              = "stop"
	EventUsage             = "usage"
	EventError             = "error"
)

type Event struct {
	Version    string        `json:"version"`
	Type       string        `json:"type"`
	Index      int           `json:"index,omitempty"`
	Delta      *ContentBlock `json:"delta,omitempty"`
	StopReason string        `json:"stop_reason,omitempty"`
	Usage      *Usage        `json:"usage,omitempty"`
	Error      *Error        `json:"error,omitempty"`
}
