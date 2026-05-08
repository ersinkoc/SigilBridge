package cloudiam

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/budget"
	"github.com/sigilbridge/sigilbridge/internal/ir"
)

type VertexAI struct {
	client *http.Client
	now    func() time.Time
}

func NewVertexAI() VertexAI { return VertexAI{client: http.DefaultClient, now: time.Now} }
func (p VertexAI) WithClient(client *http.Client) VertexAI {
	if client != nil {
		p.client = client
	}
	return p
}
func (VertexAI) ID() string { return "vertex_ai" }

func (p VertexAI) Chat(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (ir.Response, error) {
	token, err := p.accessToken(ctx, cfg)
	if err != nil {
		return ir.Response{}, err
	}
	body, err := ir.DenormalizeAnthropicRequest(req)
	if err != nil {
		return ir.Response{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, vertexPredictURL(cfg), bytes.NewReader(body))
	if err != nil {
		return ir.Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return ir.Response{}, &adapter.Error{Class: adapter.Network, Provider: p.ID(), UpstreamID: cfg.UpstreamID, Message: err.Error(), Retryable: true, Wrapped: err}
	}
	raw, err := readHTTP(p.ID(), cfg, resp)
	if err != nil {
		return ir.Response{}, err
	}
	if parsed, err := parseAnthropicLike(raw, p.ID()); err == nil {
		return parsed, nil
	}
	return parseOpenAIShape(raw, p.ID())
}

func (p VertexAI) Stream(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (<-chan ir.Event, error) {
	return streamFromChat(ctx, func(ctx context.Context) (ir.Response, error) { return p.Chat(ctx, req, cfg) })
}
func (VertexAI) CountTokens(_ context.Context, req ir.Request, cfg adapter.ProviderConfig) (int, error) {
	return budget.EstimateInputTokens(req, "vertex_ai", adapter.RawString(cfg.Raw, "model"))
}
func (p VertexAI) HealthCheck(ctx context.Context, cfg adapter.ProviderConfig) error {
	_, err := p.Chat(ctx, ir.Request{ModelAlias: adapter.RawString(cfg.Raw, "model"), MaxTokens: 1, Messages: []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "ping"}}}}}, cfg)
	return err
}
func (VertexAI) Capabilities() adapter.Capabilities {
	return adapter.Capabilities{Streaming: true, ToolUse: true, Vision: true, StabilityClass: adapter.Stable, Category: "cloud_iam"}
}

func (p VertexAI) accessToken(ctx context.Context, cfg adapter.ProviderConfig) (string, error) {
	if token := adapter.RawString(cfg.Raw, "access_token"); token != "" {
		return token, nil
	}
	assertion, err := p.serviceAccountJWT(cfg)
	if err != nil {
		return "", err
	}
	form := "grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Ajwt-bearer&assertion=" + assertion
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, valueOr(adapter.RawString(cfg.Raw, "token_url"), "https://oauth2.googleapis.com/token"), strings.NewReader(form))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("vertex token endpoint returned %d: %s", resp.StatusCode, raw)
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("vertex token response missing access_token")
	}
	return out.AccessToken, nil
}

func (p VertexAI) serviceAccountJWT(cfg adapter.ProviderConfig) (string, error) {
	var sa struct {
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
		TokenURI    string `json:"token_uri"`
	}
	if err := json.Unmarshal([]byte(adapter.RawString(cfg.Raw, "service_account_json")), &sa); err != nil {
		return "", fmt.Errorf("parse service_account_json: %w", err)
	}
	block, _ := pem.Decode([]byte(sa.PrivateKey))
	if block == nil {
		return "", fmt.Errorf("service account private_key is not PEM")
	}
	keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}
	privateKey, ok := keyAny.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("service account key must be RSA")
	}
	now := p.now().UTC()
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	claims := map[string]any{"iss": sa.ClientEmail, "scope": valueOr(adapter.RawString(cfg.Raw, "scope"), "https://www.googleapis.com/auth/cloud-platform"), "aud": valueOr(sa.TokenURI, valueOr(adapter.RawString(cfg.Raw, "token_url"), "https://oauth2.googleapis.com/token")), "iat": now.Unix(), "exp": now.Add(time.Hour).Unix()}
	headerRaw, _ := json.Marshal(header)
	claimsRaw, _ := json.Marshal(claims)
	signed := base64.RawURLEncoding.EncodeToString(headerRaw) + "." + base64.RawURLEncoding.EncodeToString(claimsRaw)
	sum := sha256.Sum256([]byte(signed))
	sig, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return signed + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func vertexPredictURL(cfg adapter.ProviderConfig) string {
	if base := adapter.RawString(cfg.Raw, "base_url"); base != "" {
		return strings.TrimRight(base, "/") + "/predict"
	}
	return fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:predict", valueOr(adapter.RawString(cfg.Raw, "location"), "us-central1"), adapter.RawString(cfg.Raw, "project"), valueOr(adapter.RawString(cfg.Raw, "location"), "us-central1"), adapter.RawString(cfg.Raw, "model"))
}
