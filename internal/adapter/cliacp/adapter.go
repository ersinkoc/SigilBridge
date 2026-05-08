package cliacp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/budget"
	corecliacp "github.com/sigilbridge/sigilbridge/internal/cliacp"
	"github.com/sigilbridge/sigilbridge/internal/ir"
)

type Adapter struct {
	id       string
	command  string
	args     []string
	pool     *corecliacp.Pool
	category string
}

func New(id, command string, args ...string) Adapter {
	return Adapter{id: id, command: command, args: args, pool: corecliacp.NewPool(nil), category: "cli_acp"}
}

func (a Adapter) WithPool(pool *corecliacp.Pool) Adapter {
	if pool != nil {
		a.pool = pool
	}
	return a
}

func (a Adapter) ID() string { return a.id }

func (a Adapter) Chat(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (ir.Response, error) {
	req = adapter.RequestWithConfiguredModel(req, cfg)
	switch adapter.RawString(cfg.Raw, "protocol") {
	case "acp":
		return a.chatACP(ctx, req, cfg)
	case "headless":
		return a.chatHeadless(ctx, req, cfg)
	}
	proc, err := a.process(ctx, cfg)
	if err != nil {
		return ir.Response{}, err
	}
	var result corecliacp.AgentMessageResult
	if err := proc.Call(ctx, corecliacp.MethodAgentMessage, corecliacp.AgentMessageParams{SessionID: req.ID, Request: req}, &result); err != nil {
		return ir.Response{}, processError(a.id, proc, err)
	}
	if result.Response.Version == "" {
		result.Response.Version = ir.Version
	}
	if result.Response.UpstreamProvider == "" {
		result.Response.UpstreamProvider = a.id
	}
	return result.Response, nil
}

func (a Adapter) chatACP(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (ir.Response, error) {
	resp, err := a.chatACPOnce(ctx, req, cfg)
	if err == nil || !retryableProcessError(err) {
		return resp, err
	}
	a.dropProcess(ctx, cfg)
	return a.chatACPOnce(ctx, req, cfg)
}

func (a Adapter) chatACPOnce(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (ir.Response, error) {
	proc, err := a.process(ctx, cfg)
	if err != nil {
		return ir.Response{}, err
	}
	var init corecliacp.ACPInitializeResult
	if err := proc.Call(ctx, corecliacp.MethodInitialize, corecliacp.ACPInitializeParams{
		ProtocolVersion:    1,
		ClientCapabilities: map[string]any{},
		ClientInfo:         corecliacp.ACPImplementation{Name: "sigilbridge", Title: "SigilBridge", Version: "dev"},
	}, &init); err != nil {
		return ir.Response{}, processError(a.id, proc, err)
	}
	if init.ProtocolVersion != 0 && init.ProtocolVersion != 1 {
		return ir.Response{}, fmt.Errorf("%s selected unsupported ACP protocol version %d", a.id, init.ProtocolVersion)
	}
	cwd := adapter.RawString(cfg.Raw, "cwd")
	if cwd == "" {
		cwd, err = os.Getwd()
		if err != nil {
			return ir.Response{}, err
		}
	}
	cwd, err = filepath.Abs(cwd)
	if err != nil {
		return ir.Response{}, err
	}
	var session corecliacp.ACPSessionNewResult
	if err := proc.Call(ctx, corecliacp.MethodSessionNew, corecliacp.ACPSessionNewParams{CWD: cwd, MCPServers: []any{}}, &session); err != nil {
		return ir.Response{}, processError(a.id, proc, err)
	}
	if session.SessionID == "" {
		return ir.Response{}, fmt.Errorf("%s ACP session/new returned an empty sessionId", a.id)
	}
	var text strings.Builder
	var promptResult corecliacp.ACPSessionPromptResult
	err = proc.CallWithHandler(ctx, corecliacp.MethodSessionPrompt, corecliacp.ACPSessionPromptParams{
		SessionID: session.SessionID,
		Prompt: []corecliacp.ACPContentBlock{{
			Type: "text",
			Text: promptText(req),
		}},
	}, &promptResult, func(message corecliacp.Message) error {
		if message.Method != corecliacp.MethodSessionUpdate {
			return nil
		}
		var update corecliacp.ACPSessionUpdateParams
		if err := json.Unmarshal(message.Params, &update); err != nil {
			return err
		}
		if update.Update.SessionUpdate == "agent_message_chunk" {
			for _, block := range update.Update.Content {
				if block.Type == "text" {
					text.WriteString(block.Text)
				}
			}
		}
		return nil
	})
	if err != nil {
		return ir.Response{}, processError(a.id, proc, err)
	}
	return a.responseFromText(req, text.String(), valueOr(promptResult.StopReason, ir.StopEndTurn)), nil
}

func (a Adapter) chatHeadless(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (ir.Response, error) {
	command := valueOr(adapter.RawString(cfg.Raw, "command"), a.command)
	if command == "" {
		return ir.Response{}, fmt.Errorf("%s command is required", a.id)
	}
	args, outputFile, err := a.headlessArgs(req, cfg)
	if err != nil {
		return ir.Response{}, err
	}
	if outputFile != "" {
		defer os.Remove(outputFile)
	}
	cmd := exec.CommandContext(ctx, command, args...)
	if cwd := adapter.RawString(cfg.Raw, "cwd"); cwd != "" {
		cmd.Dir = cwd
	}
	if stdin := adapter.RawString(cfg.Raw, "stdin"); stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return ir.Response{}, fmt.Errorf("%s headless command failed: %w: %s", a.id, err, strings.TrimSpace(stderr.String()))
	}
	text := strings.TrimSpace(stdout.String())
	if outputFile != "" {
		raw, err := os.ReadFile(outputFile)
		if err == nil && strings.TrimSpace(string(raw)) != "" {
			text = strings.TrimSpace(string(raw))
		}
	}
	return a.responseFromText(req, text, ir.StopEndTurn), nil
}

func (a Adapter) Stream(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (<-chan ir.Event, error) {
	resp, err := a.Chat(ctx, req, cfg)
	if err != nil {
		return nil, err
	}
	ch := make(chan ir.Event)
	go func() {
		defer close(ch)
		ch <- ir.Event{Version: ir.Version, Type: ir.EventStart}
		for i, block := range resp.Content {
			ch <- ir.Event{Version: ir.Version, Type: ir.EventContentBlockDelta, Index: i, Delta: &block}
		}
		ch <- ir.Event{Version: ir.Version, Type: ir.EventUsage, Usage: &resp.Usage}
		ch <- ir.Event{Version: ir.Version, Type: ir.EventStop, StopReason: resp.StopReason}
	}()
	return ch, nil
}

func (a Adapter) CountTokens(_ context.Context, req ir.Request, cfg adapter.ProviderConfig) (int, error) {
	model := adapter.RawString(cfg.Raw, "model")
	if model == "" {
		model = req.ModelAlias
	}
	return budget.EstimateInputTokens(req, a.id, model)
}

func (a Adapter) HealthCheck(ctx context.Context, cfg adapter.ProviderConfig) error {
	if adapter.RawString(cfg.Raw, "protocol") == "headless" {
		command := valueOr(adapter.RawString(cfg.Raw, "command"), a.command)
		if command == "" {
			return fmt.Errorf("%s command is required", a.id)
		}
		if _, err := exec.LookPath(command); err != nil {
			return err
		}
		return nil
	}
	proc, err := a.process(ctx, cfg)
	if err != nil {
		return err
	}
	var result corecliacp.InitializeResult
	return processError(a.id, proc, proc.Call(ctx, corecliacp.MethodInitialize, corecliacp.InitializeParams{ClientName: "sigilbridge", ClientVersion: "dev"}, &result))
}

func (a Adapter) Capabilities() adapter.Capabilities {
	return adapter.Capabilities{Streaming: true, ToolUse: true, Vision: false, StabilityClass: adapter.Stable, Category: a.category}
}

func (a Adapter) process(ctx context.Context, cfg adapter.ProviderConfig) (*corecliacp.Process, error) {
	command := valueOr(adapter.RawString(cfg.Raw, "command"), a.command)
	if command == "" {
		return nil, fmt.Errorf("%s command is required", a.id)
	}
	args := a.args
	if rawArgs, ok := stringSlice(cfg.Raw["args"]); ok {
		args = rawArgs
	}
	var env []string
	if rawEnv, ok := stringSlice(cfg.Raw["env"]); ok {
		env = rawEnv
	}
	framing := adapter.RawString(cfg.Raw, "framing")
	timeout := time.Duration(adapter.RawInt(cfg.Raw, "idle_timeout_seconds")) * time.Second
	return a.pool.Get(ctx, a.upstreamID(cfg), corecliacp.ProcessConfig{Command: command, Args: args, Env: env, Framing: framing, IdleTimeout: timeout})
}

func (a Adapter) dropProcess(ctx context.Context, cfg adapter.ProviderConfig) {
	a.pool.Drop(ctx, a.upstreamID(cfg), nil)
}

func (a Adapter) upstreamID(cfg adapter.ProviderConfig) string {
	return valueOr(cfg.UpstreamID, a.id)
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func processError(provider string, proc *corecliacp.Process, err error) error {
	if err == nil {
		return nil
	}
	if proc == nil {
		return err
	}
	stderr := strings.TrimSpace(proc.Stderr())
	if stderr == "" {
		return err
	}
	return fmt.Errorf("%s process error: %w: %s", provider, err, stderr)
}

func retryableProcessError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "file already closed") || strings.Contains(text, "eof") || strings.Contains(text, "broken pipe")
}

func (a Adapter) responseFromText(req ir.Request, text, stopReason string) ir.Response {
	if stopReason == "" {
		stopReason = ir.StopEndTurn
	}
	return ir.Response{
		Version:          ir.Version,
		ID:               req.ID,
		UpstreamProvider: a.id,
		UpstreamModel:    req.ModelAlias,
		StopReason:       stopReason,
		Content:          []ir.ContentBlock{{Type: ir.ContentText, Text: text}},
	}
}

func (a Adapter) headlessArgs(req ir.Request, cfg adapter.ProviderConfig) ([]string, string, error) {
	prompt := promptText(req)
	model := adapter.RawString(cfg.Raw, "model")
	switch a.id {
	case "claude_code_cli":
		if cfg.Raw == nil {
			cfg.Raw = map[string]any{}
		}
		cfg.Raw["stdin"] = prompt
		args := []string{"--print", "--output-format", "text", "--input-format", "text"}
		if model != "" {
			args = append(args, "--model", model)
		}
		return append(configuredArgs(cfg), args...), "", nil
	case "codex_cli":
		if cfg.Raw == nil {
			cfg.Raw = map[string]any{}
		}
		cfg.Raw["stdin"] = prompt
		tmp, err := os.CreateTemp("", "sigilbridge-codex-*.txt")
		if err != nil {
			return nil, "", err
		}
		if err := tmp.Close(); err != nil {
			_ = os.Remove(tmp.Name())
			return nil, "", err
		}
		args := []string{"exec", "--skip-git-repo-check", "--color", "never", "--sandbox", "read-only", "-o", tmp.Name()}
		if model != "" {
			args = append(args, "--model", model)
		}
		args = append(args, "-")
		return append(configuredArgs(cfg), args...), tmp.Name(), nil
	case "gemini_cli":
		args := []string{"--prompt", prompt, "--output-format", "text", "--skip-trust"}
		if model != "" {
			args = append(args, "--model", model)
		}
		return append(configuredArgs(cfg), args...), "", nil
	default:
		args := append(configuredArgs(cfg), prompt)
		return args, "", nil
	}
}

func configuredArgs(cfg adapter.ProviderConfig) []string {
	if rawArgs, ok := stringSlice(cfg.Raw["args"]); ok {
		return append([]string{}, rawArgs...)
	}
	return nil
}

func stringSlice(value any) ([]string, bool) {
	switch typed := value.(type) {
	case []string:
		return append([]string{}, typed...), true
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" {
				out = append(out, text)
			}
		}
		return out, true
	default:
		return nil, false
	}
}

func promptText(req ir.Request) string {
	var b strings.Builder
	if req.System != "" {
		b.WriteString("System:\n")
		b.WriteString(req.System)
		b.WriteString("\n\n")
	}
	for _, msg := range req.Messages {
		if msg.Role != "" {
			b.WriteString(strings.Title(msg.Role))
			b.WriteString(":\n")
		}
		for _, block := range msg.Content {
			if block.Type == ir.ContentText && block.Text != "" {
				b.WriteString(block.Text)
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		out = req.ModelAlias
	}
	return out
}
