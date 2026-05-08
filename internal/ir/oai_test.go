package ir

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNormalizeOAIPreservesToolsAndChoice(t *testing.T) {
	raw := []byte(`{
	  "model":"sonnet",
	  "messages":[
	    {"role":"system","content":"You are helpful."},
	    {"role":"user","content":[{"type":"text","text":"look"},{"type":"image_url","image_url":{"url":"https://example.com/cat.png"}}]},
	    {"role":"assistant","content":"calling","tool_calls":[{"id":"call_1","type":"function","function":{"name":"search","arguments":"{\"query\":\"sigil\",\"limit\":3}"}}]},
	    {"role":"tool","tool_call_id":"call_1","content":"result"}
	  ],
	  "tools":[{"type":"function","function":{"name":"search","description":"Search","parameters":{"type":"object","properties":{"query":{"type":"string"},"limit":{"type":"integer"}}}}}],
	  "tool_choice":{"type":"function","function":{"name":"search"}},
	  "temperature":0.2,
	  "max_tokens":64,
	  "stream":true,
	  "user":"u1"
	}`)
	req, err := NormalizeOAIRequest(raw, "key", time.Time{})
	if err != nil {
		t.Fatalf("NormalizeOAIRequest() error = %v", err)
	}
	if req.System != "You are helpful." {
		t.Fatalf("System = %q", req.System)
	}
	if len(req.Tools) != 1 || req.Tools[0].Name != "search" {
		t.Fatalf("Tools = %#v", req.Tools)
	}
	toolUse := req.Messages[1].Content[1].ToolUse
	if toolUse == nil || toolUse.Arguments["query"] != "sigil" || toolUse.Arguments["limit"].(float64) != 3 {
		t.Fatalf("tool args not preserved: %#v", toolUse)
	}
	if req.Extras["tool_choice"] == nil {
		t.Fatalf("tool_choice not preserved")
	}
	out, err := DenormalizeOAIRequest(req)
	if err != nil {
		t.Fatalf("DenormalizeOAIRequest() error = %v", err)
	}
	if !strings.Contains(string(out), "tool_choice") || !strings.Contains(string(out), "image_url") {
		t.Fatalf("denormalized request missing expected fields: %s", out)
	}
}

func TestDenormalizeOAIResponse(t *testing.T) {
	raw, err := DenormalizeOAIResponse(Response{
		ID:            "chatcmpl_1",
		UpstreamModel: "claude-sonnet",
		StopReason:    StopToolUse,
		Content: []ContentBlock{
			{Type: ContentText, Text: "hello"},
			{Type: ContentToolUse, ToolUse: &ToolUse{ID: "call_1", Name: "search", Arguments: map[string]any{"q": "x"}}},
		},
		Usage: Usage{InputTokens: 10, OutputTokens: 5},
	})
	if err != nil {
		t.Fatalf("DenormalizeOAIResponse() error = %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out["object"] != "chat.completion" {
		t.Fatalf("object = %v", out["object"])
	}
	if !strings.Contains(string(raw), "tool_calls") {
		t.Fatalf("response missing tool_calls: %s", raw)
	}
}

func TestEventsToOAIStream(t *testing.T) {
	ch := make(chan Event, 2)
	ch <- Event{Type: EventStart}
	ch <- Event{Type: EventDelta, Delta: &ContentBlock{Type: ContentText, Text: "hi"}}
	close(ch)
	raw, err := EventsToOAIStream("id", "model", ch)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "data:") || !strings.Contains(string(raw), "[DONE]") {
		t.Fatalf("bad stream:\n%s", raw)
	}
}
