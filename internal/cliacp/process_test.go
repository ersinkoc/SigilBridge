package cliacp

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/ir"
)

func TestProcessPoolSpawnReuseIdleRespawn(t *testing.T) {
	if os.Getenv("SIGILBRIDGE_STUB_ACP") == "1" {
		runStubACP(t)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("Executable() error = %v", err)
	}
	pool := NewPool(nil)
	cfg := ProcessConfig{Command: exe, Args: []string{"-test.run=TestProcessPoolSpawnReuseIdleRespawn"}, Env: []string{"SIGILBRIDGE_STUB_ACP=1"}, IdleTimeout: 100 * time.Millisecond}
	ctx := context.Background()
	first, err := pool.Get(ctx, "upstream-1", cfg)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	second, err := pool.Get(ctx, "upstream-1", cfg)
	if err != nil {
		t.Fatalf("Get() second error = %v", err)
	}
	if first != second {
		t.Fatalf("pool did not reuse process")
	}
	var result AgentMessageResult
	if err := first.Call(ctx, MethodAgentMessage, AgentMessageParams{Request: ir.Request{Version: ir.Version, ModelAlias: "stub"}}, &result); err != nil {
		t.Fatalf("Call() error = %v; stderr=%s", err, first.Stderr())
	}
	if result.Response.Content[0].Text != "stub response" {
		t.Fatalf("result = %#v", result)
	}
	select {
	case <-first.done:
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout waiting for idle shutdown; stderr=%s", first.Stderr())
	}
	third, err := pool.Get(ctx, "upstream-1", cfg)
	if err != nil {
		t.Fatalf("Get() third error = %v", err)
	}
	if third == first {
		t.Fatalf("expected idle timeout to respawn process")
	}
	pool.Close(ctx)
}

func runStubACP(t *testing.T) {
	t.Helper()
	codec := NewCodec(stdioRWC{Reader: os.Stdin, Writer: os.Stdout})
	for {
		msg, err := codec.Recv()
		if err != nil {
			return
		}
		switch msg.Method {
		case MethodInitialize:
			raw, _ := json.Marshal(InitializeResult{AgentName: "stub", AgentVersion: "test", ProtocolVersion: "1"})
			_ = codec.Send(Message{ID: msg.ID, Result: raw})
		case MethodAgentMessage:
			raw, _ := json.Marshal(AgentMessageResult{Response: ir.Response{Version: ir.Version, UpstreamProvider: "stub", UpstreamModel: "stub", StopReason: ir.StopEndTurn, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "stub response"}}}})
			_ = codec.Send(Message{ID: msg.ID, Result: raw})
		case MethodShutdown:
			return
		}
	}
}

type stdioRWC struct {
	io.Reader
	io.Writer
}

func (stdioRWC) Close() error { return nil }
