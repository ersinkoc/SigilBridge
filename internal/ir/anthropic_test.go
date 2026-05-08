package ir

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNormalizeAnthropicPreservesMCPAndCacheControl(t *testing.T) {
	raw := []byte(`{
	  "model":"sonnet",
	  "system":"You are helpful.",
	  "messages":[
	    {"role":"user","content":[
	      {"type":"text","text":"hello","cache_control":{"type":"ephemeral"}},
	      {"type":"image","source":{"type":"url","url":"https://example.com/a.png","media_type":"image/png"}}
	    ]},
	    {"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"search","input":{"query":"sigil","limit":3}}]},
	    {"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"text","text":"result"}]}]}
	  ],
	  "tools":[{"name":"search","description":"Search","input_schema":{"type":"object"}}],
	  "mcp_servers":[{"type":"url","url":"https://mcp.example.com/sse","name":"example"}],
	  "max_tokens":128,
	  "stream":true
	}`)
	req, err := NormalizeAnthropicRequest(raw, "key", time.Time{})
	if err != nil {
		t.Fatalf("NormalizeAnthropicRequest() error = %v", err)
	}
	if req.System != "You are helpful." {
		t.Fatalf("System = %q", req.System)
	}
	if len(req.MCPServers) != 1 || req.MCPServers[0].URL == "" {
		t.Fatalf("MCPServers = %#v", req.MCPServers)
	}
	if req.Messages[0].Content[0].Extras["cache_control"] == nil {
		t.Fatalf("cache_control not preserved")
	}
	out, err := DenormalizeAnthropicRequest(req)
	if err != nil {
		t.Fatalf("DenormalizeAnthropicRequest() error = %v", err)
	}
	if !strings.Contains(string(out), "mcp_servers") || !strings.Contains(string(out), "cache_control") {
		t.Fatalf("denormalized Anthropic request missing expected fields: %s", out)
	}
}

func TestDenormalizeAnthropicResponse(t *testing.T) {
	raw, err := DenormalizeAnthropicResponse(Response{
		ID:            "msg_1",
		UpstreamModel: "claude-sonnet",
		StopReason:    StopEndTurn,
		Content:       []ContentBlock{{Type: ContentText, Text: "hello"}},
		Usage:         Usage{InputTokens: 3, OutputTokens: 4},
	})
	if err != nil {
		t.Fatalf("DenormalizeAnthropicResponse() error = %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out["type"] != "message" || out["role"] != RoleAssistant {
		t.Fatalf("bad response: %s", raw)
	}
}

func TestEventsToAnthropicStream(t *testing.T) {
	ch := make(chan Event, 3)
	ch <- Event{Type: EventStart}
	ch <- Event{Type: EventContentBlockDelta, Delta: &ContentBlock{Type: ContentText, Text: "hi"}}
	ch <- Event{Type: EventStop}
	close(ch)
	raw, err := EventsToAnthropicStream("msg", "model", ch)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "message_start") || !strings.Contains(string(raw), "content_block_delta") {
		t.Fatalf("bad stream:\n%s", raw)
	}
}
