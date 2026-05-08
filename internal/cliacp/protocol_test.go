package cliacp

import (
	"encoding/json"
	"testing"

	"github.com/sigilbridge/sigilbridge/internal/ir"
)

func TestProtocolMessageTypesMarshal(t *testing.T) {
	cases := []any{
		InitializeParams{ClientName: "sigilbridge", ClientVersion: "dev"},
		InitializeResult{AgentName: "stub", AgentVersion: "1", ProtocolVersion: "1"},
		AgentMessageParams{SessionID: "s1", Request: ir.Request{Version: ir.Version, ModelAlias: "m"}},
		AgentMessageDelta{SessionID: "s1", Event: ir.Event{Version: ir.Version, Type: ir.EventStart}},
		AgentMessageComplete{SessionID: "s1", Response: ir.Response{Version: ir.Version, StopReason: ir.StopEndTurn}},
		ShutdownParams{Reason: "idle"},
		ErrorParams{Type: "error", Message: "boom"},
	}
	for _, tc := range cases {
		raw, err := json.Marshal(tc)
		if err != nil {
			t.Fatalf("Marshal(%T) error = %v", tc, err)
		}
		if !json.Valid(raw) {
			t.Fatalf("invalid JSON for %T: %s", tc, raw)
		}
	}
}

func TestACPSessionUpdateContentAcceptsObjectOrArray(t *testing.T) {
	for name, raw := range map[string]string{
		"object": `{"sessionId":"s1","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"hello"}}}`,
		"array":  `{"sessionId":"s1","update":{"sessionUpdate":"agent_message_chunk","content":[{"type":"text","text":"hello"},{"type":"text","text":" world"}]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			var update ACPSessionUpdateParams
			if err := json.Unmarshal([]byte(raw), &update); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if len(update.Update.Content) == 0 || update.Update.Content[0].Text != "hello" {
				t.Fatalf("content = %#v", update.Update.Content)
			}
		})
	}
}
