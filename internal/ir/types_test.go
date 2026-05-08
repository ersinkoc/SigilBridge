package ir

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestIRJSONRoundTripStable(t *testing.T) {
	temp := float32(0.7)
	req := Request{
		Version:       Version,
		ID:            "01HXREQ",
		BridgeKeyID:   "key",
		ReceivedAt:    time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC),
		IngressFormat: IngressOpenAI,
		ModelAlias:    "sonnet",
		System:        "You are helpful.",
		Messages: []Message{{
			Role: RoleUser,
			Content: []ContentBlock{
				{Type: ContentText, Text: "hello", Extras: map[string]any{"cache_control": map[string]any{"type": "ephemeral"}}},
				{Type: ContentToolResult, ToolResult: &ToolResult{ToolUseID: "tool_1", Content: []ContentBlock{{Type: ContentText, Text: "done"}}}},
			},
		}},
		Tools:       []ToolDef{{Name: "search", Description: "Search", InputSchema: map[string]any{"type": "object"}}},
		MCPServers:  []MCPServer{{Type: "url", URL: "https://mcp.example.com/sse", Name: "example"}},
		MaxTokens:   100,
		Temperature: &temp,
		Metadata:    map[string]string{"user_id": "u1"},
		Extras:      map[string]any{"tool_choice": "auto"},
	}

	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var got Request
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !reflect.DeepEqual(normalizeJSONNumbers(req), normalizeJSONNumbers(got)) {
		t.Fatalf("round trip mismatch:\nwant %#v\n got %#v", req, got)
	}
}

func normalizeJSONNumbers[T any](value T) any {
	raw, _ := json.Marshal(value)
	var out any
	_ = json.Unmarshal(raw, &out)
	return out
}
