package audit

import (
	"strings"
	"testing"
)

func TestCaptureContentModes(t *testing.T) {
	none, err := CaptureContent(ContentNone, "prompt", "response")
	if err != nil {
		t.Fatal(err)
	}
	if none != (CapturedContent{}) {
		t.Fatalf("none = %#v", none)
	}
	hashed, err := CaptureContent(ContentHash, "prompt", "response")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hashed.PromptHash, "sha256:") || hashed.Prompt != "" {
		t.Fatalf("hash mode = %#v", hashed)
	}
	long := strings.Repeat("x", TruncateChars+10)
	truncated, err := CaptureContent(ContentTruncated, long, long)
	if err != nil {
		t.Fatal(err)
	}
	if len([]rune(truncated.Prompt)) != TruncateChars {
		t.Fatalf("truncated len = %d", len([]rune(truncated.Prompt)))
	}
	full, err := CaptureContent(ContentFull, "prompt", "response")
	if err != nil {
		t.Fatal(err)
	}
	if full.Prompt != "prompt" || full.Response != "response" {
		t.Fatalf("full = %#v", full)
	}
}
