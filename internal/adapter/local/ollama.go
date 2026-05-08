package local

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/budget"
	"github.com/sigilbridge/sigilbridge/internal/httpclient"
	"github.com/sigilbridge/sigilbridge/internal/ir"
)

type Ollama struct {
	baseURL string
	client  *http.Client
}

func NewOllama(baseURL string) Ollama {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return Ollama{baseURL: strings.TrimRight(baseURL, "/"), client: httpclient.Default()}
}

func (p Ollama) WithClient(client *http.Client) Ollama {
	if client != nil {
		p.client = client
	}
	return p
}

func (Ollama) ID() string { return "ollama" }

func (p Ollama) Chat(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (ir.Response, error) {
	req = adapter.RequestWithConfiguredModel(req, cfg)
	req.Stream = false
	body, err := marshalOllamaRequest(req)
	if err != nil {
		return ir.Response{}, err
	}
	raw, err := p.do(ctx, body, cfg)
	if err != nil {
		return ir.Response{}, err
	}
	return parseOllamaResponse(raw)
}

func (p Ollama) Stream(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (<-chan ir.Event, error) {
	req = adapter.RequestWithConfiguredModel(req, cfg)
	req.Stream = true
	body, err := marshalOllamaRequest(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, &adapter.Error{Class: adapter.Network, Provider: p.ID(), UpstreamID: cfg.UpstreamID, Message: err.Error(), Retryable: true, Wrapped: err}
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		class := adapter.ClassifyHTTP(resp.StatusCode)
		return nil, &adapter.Error{Class: class, Provider: p.ID(), UpstreamID: cfg.UpstreamID, HTTPStatus: resp.StatusCode, Message: string(raw), Retryable: adapter.Retryable(class)}
	}
	ch := make(chan ir.Event)
	go parseOllamaStream(resp.Body, ch)
	return ch, nil
}

func (Ollama) CountTokens(_ context.Context, req ir.Request, cfg adapter.ProviderConfig) (int, error) {
	model := adapter.RawString(cfg.Raw, "model")
	if model == "" {
		model = req.ModelAlias
	}
	return budget.EstimateInputTokens(req, "ollama", model)
}

func (p Ollama) HealthCheck(ctx context.Context, cfg adapter.ProviderConfig) error {
	model := adapter.RawString(cfg.Raw, "model")
	if model == "" {
		model = "llama3.2"
	}
	_, err := p.Chat(ctx, ir.Request{Version: ir.Version, ModelAlias: model, MaxTokens: 1, Messages: []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "ping"}}}}}, cfg)
	return err
}

func (Ollama) Capabilities() adapter.Capabilities {
	return adapter.Capabilities{Streaming: true, ToolUse: true, Vision: true, StabilityClass: adapter.Stable, Category: "local"}
}

func (p Ollama) do(ctx context.Context, body []byte, cfg adapter.ProviderConfig) ([]byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, &adapter.Error{Class: adapter.Network, Provider: p.ID(), UpstreamID: cfg.UpstreamID, Message: err.Error(), Retryable: true, Wrapped: err}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		class := adapter.ClassifyHTTP(resp.StatusCode)
		return nil, &adapter.Error{Class: class, Provider: p.ID(), UpstreamID: cfg.UpstreamID, HTTPStatus: resp.StatusCode, Message: string(raw), Retryable: adapter.Retryable(class)}
	}
	return raw, nil
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Tools    []ollamaTool    `json:"tools,omitempty"`
	Options  map[string]any  `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	Images    []string         `json:"images,omitempty"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaTool struct {
	Type     string         `json:"type"`
	Function ollamaFunction `json:"function"`
}

type ollamaFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type ollamaToolCall struct {
	Function struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments,omitempty"`
	} `json:"function"`
}

func marshalOllamaRequest(req ir.Request) ([]byte, error) {
	out := ollamaRequest{
		Model:    req.ModelAlias,
		Stream:   req.Stream,
		Tools:    ollamaTools(req.Tools),
		Messages: make([]ollamaMessage, 0, len(req.Messages)+1),
	}
	options := map[string]any{}
	if req.MaxTokens > 0 {
		options["num_predict"] = req.MaxTokens
	}
	if req.Temperature != nil {
		options["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		options["top_p"] = *req.TopP
	}
	if len(options) > 0 {
		out.Options = options
	}
	if req.System != "" {
		out.Messages = append(out.Messages, ollamaMessage{Role: ir.RoleSystem, Content: req.System})
	}
	for _, msg := range req.Messages {
		out.Messages = append(out.Messages, ollamaMessageFromIR(msg))
	}
	return json.Marshal(out)
}

func ollamaMessageFromIR(msg ir.Message) ollamaMessage {
	out := ollamaMessage{Role: msg.Role}
	for _, block := range msg.Content {
		switch block.Type {
		case ir.ContentText:
			out.Content += block.Text
		case ir.ContentImage:
			switch {
			case len(block.ImageB64) > 0:
				out.Images = append(out.Images, base64.StdEncoding.EncodeToString(block.ImageB64))
			case strings.HasPrefix(block.ImageURL, "data:"):
				if _, encoded, ok := strings.Cut(block.ImageURL, ","); ok {
					out.Images = append(out.Images, encoded)
				}
			}
		case ir.ContentToolUse:
			if block.ToolUse == nil {
				continue
			}
			call := ollamaToolCall{}
			call.Function.Name = block.ToolUse.Name
			call.Function.Arguments = block.ToolUse.Arguments
			out.ToolCalls = append(out.ToolCalls, call)
		case ir.ContentToolResult:
			if block.ToolResult != nil {
				out.Content += toolResultText(block.ToolResult.Content)
			}
		}
	}
	return out
}

func toolResultText(blocks []ir.ContentBlock) string {
	var text string
	for _, block := range blocks {
		if block.Type == ir.ContentText {
			text += block.Text
		}
	}
	return text
}

func ollamaTools(tools []ir.ToolDef) []ollamaTool {
	out := make([]ollamaTool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, ollamaTool{Type: "function", Function: ollamaFunction{Name: tool.Name, Description: tool.Description, Parameters: tool.InputSchema}})
	}
	return out
}

func parseOllamaResponse(raw []byte) (ir.Response, error) {
	var in struct {
		Model           string        `json:"model"`
		Message         ollamaMessage `json:"message"`
		DoneReason      string        `json:"done_reason"`
		PromptEvalCount int           `json:"prompt_eval_count"`
		EvalCount       int           `json:"eval_count"`
		TotalDuration   int64         `json:"total_duration"`
		LoadDuration    int64         `json:"load_duration"`
		PromptEvalDur   int64         `json:"prompt_eval_duration"`
		EvalDuration    int64         `json:"eval_duration"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return ir.Response{}, fmt.Errorf("parse Ollama response: %w", err)
	}
	return ir.Response{
		Version:          ir.Version,
		UpstreamProvider: "ollama",
		UpstreamModel:    in.Model,
		StopReason:       ollamaStopReason(in.DoneReason),
		Content:          ollamaContent(in.Message),
		Usage:            ir.Usage{InputTokens: in.PromptEvalCount, OutputTokens: in.EvalCount},
		LatencyMs:        in.TotalDuration / int64(timeMillisecond),
	}, nil
}

const timeMillisecond = 1_000_000

func ollamaContent(message ollamaMessage) []ir.ContentBlock {
	content := []ir.ContentBlock{}
	if message.Content != "" {
		content = append(content, ir.ContentBlock{Type: ir.ContentText, Text: message.Content})
	}
	for i, call := range message.ToolCalls {
		content = append(content, ir.ContentBlock{Type: ir.ContentToolUse, ToolUse: &ir.ToolUse{ID: fmt.Sprintf("ollama_tool_%d", i), Name: call.Function.Name, Arguments: call.Function.Arguments}})
	}
	return content
}

func parseOllamaStream(body io.ReadCloser, ch chan<- ir.Event) {
	defer body.Close()
	defer close(ch)
	ch <- ir.Event{Version: ir.Version, Type: ir.EventStart}
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var chunk struct {
			Model           string        `json:"model"`
			Message         ollamaMessage `json:"message"`
			Done            bool          `json:"done"`
			DoneReason      string        `json:"done_reason"`
			PromptEvalCount int           `json:"prompt_eval_count"`
			EvalCount       int           `json:"eval_count"`
			Error           string        `json:"error"`
		}
		if json.Unmarshal(line, &chunk) != nil {
			continue
		}
		if chunk.Error != "" {
			ch <- ir.Event{Version: ir.Version, Type: ir.EventError, Error: &ir.Error{Type: "upstream_error", Message: chunk.Error, Retryable: false, Class: ir.ErrorServer}}
			return
		}
		if chunk.Message.Content != "" {
			ch <- ir.Event{Version: ir.Version, Type: ir.EventContentBlockDelta, Delta: &ir.ContentBlock{Type: ir.ContentText, Text: chunk.Message.Content}}
		}
		if chunk.Done {
			if chunk.PromptEvalCount > 0 || chunk.EvalCount > 0 {
				ch <- ir.Event{Version: ir.Version, Type: ir.EventUsage, Usage: &ir.Usage{InputTokens: chunk.PromptEvalCount, OutputTokens: chunk.EvalCount}}
			}
			ch <- ir.Event{Version: ir.Version, Type: ir.EventStop, StopReason: ollamaStopReason(chunk.DoneReason)}
			return
		}
	}
}

func ollamaStopReason(reason string) string {
	switch reason {
	case "length":
		return ir.StopMaxTokens
	default:
		return ir.StopEndTurn
	}
}
