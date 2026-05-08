package session

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/budget"
	"github.com/sigilbridge/sigilbridge/internal/ir"
)

type Vault interface {
	Get(ctx context.Context, id string) ([]byte, map[string]string, error)
}

type WebAdapter struct {
	id          string
	baseURL     string
	path        string
	schema      string
	vault       Vault
	client      *http.Client
	minInterval time.Duration
	mu          sync.Mutex
	last        map[string]time.Time
}

type SessionCredential struct {
	Cookies        map[string]string `json:"cookies"`
	UserAgent      string            `json:"user_agent"`
	OrganizationID string            `json:"organization_id,omitempty"`
}

func NewWebAdapter(id, baseURL, endpointPath, schema string, vault Vault) *WebAdapter {
	return &WebAdapter{id: id, baseURL: strings.TrimRight(baseURL, "/"), path: endpointPath, schema: schema, vault: vault, client: http.DefaultClient, minInterval: time.Second, last: map[string]time.Time{}}
}

func (a *WebAdapter) WithClient(client *http.Client) *WebAdapter {
	if client != nil {
		a.client = client
	}
	return a
}

func (a *WebAdapter) WithMinInterval(interval time.Duration) *WebAdapter {
	a.minInterval = interval
	return a
}

func (a *WebAdapter) ID() string { return a.id }

func (a *WebAdapter) Chat(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (ir.Response, error) {
	req = adapter.RequestWithConfiguredModel(req, cfg)
	credentialID := sessionID(cfg, a.id)
	cred, err := a.credential(ctx, credentialID)
	if err != nil {
		return ir.Response{}, err
	}
	a.pace(ctx, credentialID)
	body, err := a.body(req)
	if err != nil {
		return ir.Response{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint(cred, cfg), bytes.NewReader(body))
	if err != nil {
		return ir.Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if cred.UserAgent != "" {
		httpReq.Header.Set("User-Agent", cred.UserAgent)
	}
	if cookie := cookieHeader(cred.Cookies); cookie != "" {
		httpReq.Header.Set("Cookie", cookie)
	}
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return ir.Response{}, &adapter.Error{Class: adapter.Network, Provider: a.id, UpstreamID: cfg.UpstreamID, Message: err.Error(), Retryable: true, Wrapped: err}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		class := adapter.ClassifyHTTP(resp.StatusCode)
		return ir.Response{}, &adapter.Error{Class: class, Provider: a.id, UpstreamID: cfg.UpstreamID, HTTPStatus: resp.StatusCode, Message: string(raw), Retryable: adapter.Retryable(class)}
	}
	if a.schema == "anthropic" {
		return parseAnthropic(raw, a.id)
	}
	return parseOpenAI(raw, a.id)
}

func (a *WebAdapter) Stream(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (<-chan ir.Event, error) {
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

func (a *WebAdapter) CountTokens(_ context.Context, req ir.Request, cfg adapter.ProviderConfig) (int, error) {
	model := adapter.RawString(cfg.Raw, "model")
	if model == "" {
		model = req.ModelAlias
	}
	return budget.EstimateInputTokens(req, a.id, model)
}

func (a *WebAdapter) HealthCheck(ctx context.Context, cfg adapter.ProviderConfig) error {
	_, err := a.credential(ctx, sessionID(cfg, a.id))
	return err
}

func (a *WebAdapter) Capabilities() adapter.Capabilities {
	return adapter.Capabilities{Streaming: true, ToolUse: true, StabilityClass: "experimental", Category: "session"}
}

func (a *WebAdapter) credential(ctx context.Context, id string) (SessionCredential, error) {
	if a.vault == nil {
		return SessionCredential{}, fmt.Errorf("session vault is required")
	}
	raw, _, err := a.vault.Get(ctx, id)
	if err != nil {
		return SessionCredential{}, err
	}
	var cred SessionCredential
	if err := json.Unmarshal(raw, &cred); err != nil {
		return SessionCredential{}, err
	}
	return cred, nil
}

func (a *WebAdapter) body(req ir.Request) ([]byte, error) {
	if a.schema == "anthropic" {
		return ir.DenormalizeAnthropicRequest(req)
	}
	return ir.DenormalizeOAIRequest(req)
}

func (a *WebAdapter) endpoint(cred SessionCredential, cfg adapter.ProviderConfig) string {
	endpoint := a.path
	endpoint = strings.ReplaceAll(endpoint, "{organization_id}", cred.OrganizationID)
	return a.base(cfg) + endpoint
}

func (a *WebAdapter) base(cfg adapter.ProviderConfig) string {
	if base := adapter.RawString(cfg.Raw, "base_url"); base != "" {
		return strings.TrimRight(base, "/")
	}
	return a.baseURL
}

func (a *WebAdapter) pace(ctx context.Context, id string) {
	if a.minInterval <= 0 {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	wait := a.last[id].Add(a.minInterval).Sub(time.Now())
	if wait > 0 {
		timer := time.NewTimer(wait)
		a.mu.Unlock()
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
		}
		a.mu.Lock()
	}
	a.last[id] = time.Now()
}

func sessionID(cfg adapter.ProviderConfig, providerID string) string {
	if id := adapter.RawString(cfg.Raw, "vault_id"); id != "" {
		return id
	}
	return "vault://" + providerID + "/" + valueOr(adapter.RawString(cfg.Raw, "session_name"), "default")
}

func cookieHeader(cookies map[string]string) string {
	parts := make([]string, 0, len(cookies))
	for name, value := range cookies {
		parts = append(parts, name+"="+value)
	}
	return strings.Join(parts, "; ")
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
