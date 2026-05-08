package budget

import (
	"testing"

	"github.com/sigilbridge/sigilbridge/internal/ir"
)

func TestEstimateInputTokens(t *testing.T) {
	req := ir.Request{
		System:   "You are helpful.",
		Messages: []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "Hello world"}}}},
	}
	got, err := EstimateInputTokens(req, "openai_api", "gpt-4")
	if err != nil {
		t.Fatalf("EstimateInputTokens() error = %v", err)
	}
	if got <= 0 {
		t.Fatalf("tokens = %d, want > 0", got)
	}
	approx, err := EstimateInputTokens(req, "anthropic_api", "claude-sonnet")
	if err != nil {
		t.Fatal(err)
	}
	if approx <= 0 {
		t.Fatalf("approx = %d, want > 0", approx)
	}
}

func TestCalculateCost(t *testing.T) {
	got := CalculateCost(ir.Usage{InputTokens: 1_284_000, OutputTokens: 412_000}, ModelPricing{
		InputPerMTok:  300,
		OutputPerMTok: 1500,
	})
	if got != 1003 {
		t.Fatalf("CalculateCost() = %d, want 1003", got)
	}
}

func TestEstimateInputTokensIncludesToolsAndResults(t *testing.T) {
	req := ir.Request{
		Messages: []ir.Message{{
			Role: ir.RoleAssistant,
			Content: []ir.ContentBlock{
				{Type: ir.ContentToolUse, ToolUse: &ir.ToolUse{Name: "search", Arguments: map[string]any{"q": "sigilbridge"}}},
				{Type: ir.ContentToolResult, ToolResult: &ir.ToolResult{Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "result text"}}}},
			},
		}},
		Tools: []ir.ToolDef{{Name: "search", Description: "Search docs"}},
	}
	got, err := EstimateInputTokens(req, "unknown", "custom")
	if err != nil {
		t.Fatalf("EstimateInputTokens() error = %v", err)
	}
	if got <= 3 {
		t.Fatalf("tokens = %d, want tool and result text included", got)
	}
	if empty := approxTokens(""); empty != 0 {
		t.Fatalf("approxTokens(empty) = %d, want 0", empty)
	}
	if tiny := approxTokens("a"); tiny != 1 {
		t.Fatalf("approxTokens(tiny) = %d, want 1", tiny)
	}
}
