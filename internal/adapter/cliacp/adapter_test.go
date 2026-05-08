package cliacp

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	corecliacp "github.com/sigilbridge/sigilbridge/internal/cliacp"
	"github.com/sigilbridge/sigilbridge/internal/ir"
)

func TestAdapterChatAgainstStubACP(t *testing.T) {
	if os.Getenv("SIGILBRIDGE_ADAPTER_STUB_ACP") == "1" {
		runAdapterStubACP(t)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("Executable() error = %v", err)
	}
	provider := NewCodex(exe, "-test.run=TestAdapterChatAgainstStubACP")
	resp, err := provider.Chat(context.Background(), ir.Request{Version: ir.Version, ID: "r1", ModelAlias: "stub", Messages: []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "hi"}}}}}, adapter.ProviderConfig{UpstreamID: "stub", Raw: map[string]any{"args": []any{"-test.run=TestAdapterChatAgainstStubACP"}, "command": exe, "env": []any{"SIGILBRIDGE_ADAPTER_STUB_ACP=1"}}})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resp.UpstreamProvider != "codex_cli" || resp.Content[0].Text != "adapter response" {
		t.Fatalf("response = %#v", resp)
	}
}

func TestRetryableProcessError(t *testing.T) {
	for _, message := range []string{"read |0: file already closed", "EOF", "write: broken pipe"} {
		if !retryableProcessError(errText(message)) {
			t.Fatalf("retryableProcessError(%q) = false", message)
		}
	}
	if retryableProcessError(errText("json-rpc error -32602")) {
		t.Fatalf("protocol validation errors should not be retried")
	}
}

func TestClaudeHeadlessUsesStdinPrompt(t *testing.T) {
	provider := NewClaudeCode("claude")
	cfg := adapter.ProviderConfig{Raw: map[string]any{}}
	args, outputFile, err := provider.headlessArgs(ir.Request{ModelAlias: "claude-code", Messages: []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "hello claude"}}}}}, cfg)
	if err != nil {
		t.Fatalf("headlessArgs() error = %v", err)
	}
	if outputFile != "" {
		t.Fatalf("outputFile = %q", outputFile)
	}
	for _, arg := range args {
		if arg == "hello claude" {
			t.Fatalf("prompt leaked into args: %#v", args)
		}
	}
	if got := adapter.RawString(cfg.Raw, "stdin"); got == "" || got != "User:\nhello claude" {
		t.Fatalf("stdin = %q", got)
	}
}

func TestCodexHeadlessUsesStdinPrompt(t *testing.T) {
	provider := NewCodex("codex")
	cfg := adapter.ProviderConfig{Raw: map[string]any{}}
	args, outputFile, err := provider.headlessArgs(ir.Request{ModelAlias: "codex", Messages: []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "hello codex"}}}}}, cfg)
	if err != nil {
		t.Fatalf("headlessArgs() error = %v", err)
	}
	if outputFile == "" {
		t.Fatalf("outputFile is empty")
	}
	defer os.Remove(outputFile)
	for _, arg := range args {
		if arg == "hello codex" || arg == "User:\nhello codex" {
			t.Fatalf("prompt leaked into args: %#v", args)
		}
	}
	if args[len(args)-1] != "-" {
		t.Fatalf("last arg = %q, want stdin marker", args[len(args)-1])
	}
	if got := adapter.RawString(cfg.Raw, "stdin"); got == "" || got != "User:\nhello codex" {
		t.Fatalf("stdin = %q", got)
	}
}

func runAdapterStubACP(t *testing.T) {
	t.Helper()
	codec := corecliacp.NewCodec(adapterStdioRWC{Reader: os.Stdin, Writer: os.Stdout})
	for {
		msg, err := codec.Recv()
		if err != nil {
			return
		}
		if msg.Method == corecliacp.MethodShutdown {
			return
		}
		raw, _ := json.Marshal(corecliacp.AgentMessageResult{Response: ir.Response{Version: ir.Version, UpstreamProvider: "codex_cli", UpstreamModel: "stub", StopReason: ir.StopEndTurn, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "adapter response"}}}})
		_ = codec.Send(corecliacp.Message{ID: msg.ID, Result: raw})
	}
}

type adapterStdioRWC struct {
	io.Reader
	io.Writer
}

func (adapterStdioRWC) Close() error { return nil }

type errText string

func (e errText) Error() string { return string(e) }
