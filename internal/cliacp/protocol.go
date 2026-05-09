package cliacp

import (
	"bytes"
	"encoding/json"

	"github.com/sigilbridge/sigilbridge/internal/ir"
)

const (
	MethodInitialize             = "initialize"
	MethodAuthenticate           = "authenticate"
	MethodSessionNew             = "session/new"
	MethodSessionPrompt          = "session/prompt"
	MethodSessionUpdate          = "session/update"
	MethodSessionCancel          = "session/cancel"
	MethodSessionClose           = "session/close"
	MethodSessionSetConfigOption = "session/set_config_option"
	MethodAgentMessage           = "agent.message"
	MethodMessageDelta           = "agent.message_delta"
	MethodMessageDone            = "agent.message_complete"
	MethodShutdown               = "shutdown"
	MethodError                  = "error"
)

type InitializeParams struct {
	ClientName    string `json:"client_name"`
	ClientVersion string `json:"client_version"`
}

type InitializeResult struct {
	AgentName       string   `json:"agent_name"`
	AgentVersion    string   `json:"agent_version"`
	ProtocolVersion string   `json:"protocol_version"`
	Capabilities    []string `json:"capabilities,omitempty"`
}

type ACPInitializeParams struct {
	ProtocolVersion    int               `json:"protocolVersion"`
	ClientCapabilities map[string]any    `json:"clientCapabilities,omitempty"`
	ClientInfo         ACPImplementation `json:"clientInfo"`
}

type ACPImplementation struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version,omitempty"`
}

type ACPInitializeResult struct {
	ProtocolVersion   int               `json:"protocolVersion"`
	AgentCapabilities map[string]any    `json:"agentCapabilities,omitempty"`
	AgentInfo         ACPImplementation `json:"agentInfo,omitempty"`
	AuthMethods       []map[string]any  `json:"authMethods,omitempty"`
}

type ACPSessionNewParams struct {
	CWD        string `json:"cwd"`
	MCPServers []any  `json:"mcpServers"`
}

type ACPSessionNewResult struct {
	SessionID     string                   `json:"sessionId"`
	ConfigOptions []ACPSessionConfigOption `json:"configOptions,omitempty"`
}

type ACPContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type ACPContentBlocks []ACPContentBlock

func (b *ACPContentBlocks) UnmarshalJSON(raw []byte) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		*b = nil
		return nil
	}
	if raw[0] == '[' {
		var blocks []ACPContentBlock
		if err := json.Unmarshal(raw, &blocks); err != nil {
			return err
		}
		*b = blocks
		return nil
	}
	var block ACPContentBlock
	if err := json.Unmarshal(raw, &block); err != nil {
		return err
	}
	*b = []ACPContentBlock{block}
	return nil
}

type ACPSessionPromptParams struct {
	SessionID string            `json:"sessionId"`
	Prompt    []ACPContentBlock `json:"prompt"`
}

type ACPSessionPromptResult struct {
	StopReason string `json:"stopReason"`
}

type ACPSessionConfigOption struct {
	ID           string          `json:"id"`
	Name         string          `json:"name,omitempty"`
	Category     string          `json:"category,omitempty"`
	Type         string          `json:"type,omitempty"`
	CurrentValue string          `json:"currentValue,omitempty"`
	Options      json.RawMessage `json:"options,omitempty"`
}

type ACPSetSessionConfigOptionParams struct {
	SessionID string `json:"sessionId"`
	ConfigID  string `json:"configId"`
	Value     string `json:"value"`
}

type ACPSetSessionConfigOptionResult struct {
	ConfigOptions []ACPSessionConfigOption `json:"configOptions"`
}

type ACPSessionCancelParams struct {
	SessionID string `json:"sessionId"`
}

type ACPSessionCloseParams struct {
	SessionID string `json:"sessionId"`
}

type ACPSessionUpdateParams struct {
	SessionID string `json:"sessionId,omitempty"`
	Update    struct {
		SessionUpdate string           `json:"sessionUpdate"`
		Content       ACPContentBlocks `json:"content,omitempty"`
	} `json:"update"`
}

type AgentMessageParams struct {
	SessionID string     `json:"session_id,omitempty"`
	Request   ir.Request `json:"request"`
}

type AgentMessageResult struct {
	Response ir.Response `json:"response"`
}

type AgentMessageDelta struct {
	SessionID string   `json:"session_id,omitempty"`
	Event     ir.Event `json:"event"`
}

type AgentMessageComplete struct {
	SessionID string      `json:"session_id,omitempty"`
	Response  ir.Response `json:"response"`
}

type ShutdownParams struct {
	Reason string `json:"reason,omitempty"`
}

type ErrorParams struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
