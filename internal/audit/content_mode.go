package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

type ContentMode string

const (
	ContentNone      ContentMode = "none"
	ContentHash      ContentMode = "hash"
	ContentTruncated ContentMode = "truncated"
	ContentFull      ContentMode = "full"
)

const TruncateChars = 500

type CapturedContent struct {
	Prompt       string `json:"prompt,omitempty"`
	Response     string `json:"response,omitempty"`
	PromptHash   string `json:"prompt_hash,omitempty"`
	ResponseHash string `json:"response_hash,omitempty"`
}

func CaptureContent(mode ContentMode, prompt, response string) (CapturedContent, error) {
	switch mode {
	case "", ContentNone:
		return CapturedContent{}, nil
	case ContentHash:
		return CapturedContent{PromptHash: hashContent(prompt), ResponseHash: hashContent(response)}, nil
	case ContentTruncated:
		return CapturedContent{Prompt: truncate(prompt, TruncateChars), Response: truncate(response, TruncateChars)}, nil
	case ContentFull:
		return CapturedContent{Prompt: prompt, Response: response}, nil
	default:
		return CapturedContent{}, fmt.Errorf("unsupported audit content mode %q", mode)
	}
}

func hashContent(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func truncate(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}
