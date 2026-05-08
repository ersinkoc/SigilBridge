package budget

import (
	"fmt"
	"strings"

	"github.com/pkoukk/tiktoken-go"
	"github.com/sigilbridge/sigilbridge/internal/ir"
)

type ModelPricing struct {
	InputPerMTok      int64
	OutputPerMTok     int64
	CacheReadPerMTok  int64
	CacheWritePerMTok int64
}

func EstimateInputTokens(req ir.Request, provider, model string) (int, error) {
	text := requestText(req)
	switch provider {
	case "openai_api", "azure_openai", "groq", "deepseek_api":
		enc, err := tiktoken.EncodingForModel(model)
		if err != nil {
			enc, err = tiktoken.GetEncoding("cl100k_base")
			if err != nil {
				return 0, fmt.Errorf("load tiktoken encoding: %w", err)
			}
		}
		return len(enc.Encode(text, nil, nil)), nil
	case "anthropic_api", "claude_oauth", "bedrock":
		return approxTokens(text), nil
	default:
		return approxTokens(text), nil
	}
}

func CalculateCost(usage ir.Usage, p ModelPricing) int64 {
	cents := int64(usage.InputTokens)*p.InputPerMTok +
		int64(usage.OutputTokens)*p.OutputPerMTok +
		int64(usage.CacheReadTokens)*p.CacheReadPerMTok +
		int64(usage.CacheWriteTokens)*p.CacheWritePerMTok
	return (cents + 500_000) / 1_000_000
}

func requestText(req ir.Request) string {
	var b strings.Builder
	b.WriteString(req.System)
	for _, msg := range req.Messages {
		for _, block := range msg.Content {
			switch block.Type {
			case ir.ContentText:
				b.WriteString(block.Text)
			case ir.ContentToolUse:
				if block.ToolUse != nil {
					b.WriteString(block.ToolUse.Name)
					for key, value := range block.ToolUse.Arguments {
						b.WriteString(key)
						b.WriteString(fmt.Sprint(value))
					}
				}
			case ir.ContentToolResult:
				if block.ToolResult != nil {
					for _, resultBlock := range block.ToolResult.Content {
						b.WriteString(resultBlock.Text)
					}
				}
			}
			b.WriteByte('\n')
		}
	}
	for _, tool := range req.Tools {
		b.WriteString(tool.Name)
		b.WriteString(tool.Description)
	}
	return b.String()
}

func approxTokens(text string) int {
	if text == "" {
		return 0
	}
	tokens := int(float64(len([]rune(text))) / 3.5)
	if tokens < 1 {
		return 1
	}
	return tokens
}
