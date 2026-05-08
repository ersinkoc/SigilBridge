package ir

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestCrossFormatRoundTrips(t *testing.T) {
	payloads := make([][]byte, 0, 24)
	for i := range 24 {
		toolChoice := `"auto"`
		if i%3 == 0 {
			toolChoice = `{"type":"function","function":{"name":"search"}}`
		}
		image := ""
		if i%2 == 0 {
			image = `,{"type":"image_url","image_url":{"url":"https://example.com/img-` + fmt.Sprint(i) + `.png"}}`
		}
		assistantTool := ""
		if i%4 == 0 {
			assistantTool = `,{"role":"assistant","content":"let me check","tool_calls":[{"id":"call_` + fmt.Sprint(i) + `","type":"function","function":{"name":"search","arguments":"{\"query\":\"sigil ` + fmt.Sprint(i) + `\",\"limit\":3,\"filters\":{\"fresh\":true}}"}}]}`
		}
		payloads = append(payloads, []byte(`{
		  "model":"sonnet",
		  "messages":[
		    {"role":"system","content":"You are helpful `+fmt.Sprint(i)+`."},
		    {"role":"user","content":[{"type":"text","text":"hello `+fmt.Sprint(i)+`"}`+image+`]}
		    `+assistantTool+`
		  ],
		  "tools":[{"type":"function","function":{"name":"search","description":"Search","parameters":{"type":"object","properties":{"query":{"type":"string"},"limit":{"type":"integer"},"filters":{"type":"object"}}}}}],
		  "tool_choice":`+toolChoice+`,
		  "temperature":0.3,
		  "max_tokens":128,
		  "stream":`+fmt.Sprint(i%2 == 0)+`
		}`))
	}

	for i, payload := range payloads {
		t.Run(fmt.Sprintf("payload_%02d", i), func(t *testing.T) {
			first, err := NormalizeOAIRequest(payload, "key", time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC))
			if err != nil {
				t.Fatalf("NormalizeOAIRequest() error = %v", err)
			}
			anthropicRaw, err := DenormalizeAnthropicRequest(first)
			if err != nil {
				t.Fatalf("DenormalizeAnthropicRequest() error = %v", err)
			}
			middle, err := NormalizeAnthropicRequest(anthropicRaw, "key", time.Date(2026, 5, 7, 12, 0, 1, 0, time.UTC))
			if err != nil {
				t.Fatalf("NormalizeAnthropicRequest() error = %v\npayload: %s", err, anthropicRaw)
			}
			oaiRaw, err := DenormalizeOAIRequest(middle)
			if err != nil {
				t.Fatalf("DenormalizeOAIRequest() error = %v", err)
			}
			final, err := NormalizeOAIRequest(oaiRaw, "key", time.Date(2026, 5, 7, 12, 0, 2, 0, time.UTC))
			if err != nil {
				t.Fatalf("NormalizeOAIRequest(final) error = %v\npayload: %s", err, oaiRaw)
			}
			if !reflect.DeepEqual(semanticRequest(first), semanticRequest(final)) {
				firstJSON, _ := json.MarshalIndent(semanticRequest(first), "", "  ")
				finalJSON, _ := json.MarshalIndent(semanticRequest(final), "", "  ")
				t.Fatalf("semantic mismatch\nfirst: %s\nfinal: %s", firstJSON, finalJSON)
			}
		})
	}
}

func semanticRequest(req Request) map[string]any {
	return map[string]any{
		"model":          req.ModelAlias,
		"system":         req.System,
		"messages":       semanticMessages(req.Messages),
		"tools":          semanticTools(req.Tools),
		"tool_choice":    normalizeJSONValue(req.Extras["tool_choice"]),
		"max_tokens":     req.MaxTokens,
		"stream":         req.Stream,
		"stop_sequences": req.StopSequences,
	}
}

func semanticMessages(messages []Message) []map[string]any {
	out := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		blocks := make([]map[string]any, 0, len(msg.Content))
		for _, block := range msg.Content {
			blocks = append(blocks, semanticBlock(block))
		}
		out = append(out, map[string]any{"role": msg.Role, "content": blocks})
	}
	return out
}

func semanticBlock(block ContentBlock) map[string]any {
	out := map[string]any{"type": block.Type}
	switch block.Type {
	case ContentText:
		out["text"] = block.Text
	case ContentImage:
		out["image_url"] = block.ImageURL
		out["media_type"] = block.MediaType
	case ContentToolUse:
		out["id"] = block.ToolUse.ID
		out["name"] = block.ToolUse.Name
		out["arguments"] = normalizeJSONValue(block.ToolUse.Arguments)
	case ContentToolResult:
		out["tool_use_id"] = block.ToolResult.ToolUseID
		out["content"] = semanticMessages([]Message{{Role: RoleTool, Content: block.ToolResult.Content}})[0]["content"]
	}
	return normalizeJSONValue(out).(map[string]any)
}

func semanticTools(tools []ToolDef) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		out = append(out, map[string]any{
			"name":         tool.Name,
			"description":  tool.Description,
			"input_schema": normalizeJSONValue(tool.InputSchema),
		})
	}
	return out
}

func normalizeJSONValue(value any) any {
	raw, _ := json.Marshal(value)
	var out any
	_ = json.Unmarshal(raw, &out)
	return out
}
