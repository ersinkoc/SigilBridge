package cloudiam

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/budget"
	"github.com/sigilbridge/sigilbridge/internal/httpclient"
	"github.com/sigilbridge/sigilbridge/internal/ir"
)

type Bedrock struct {
	client *http.Client
	now    func() time.Time
}

func NewBedrock() Bedrock { return Bedrock{client: httpclient.Default(), now: time.Now} }
func (p Bedrock) WithClient(client *http.Client) Bedrock {
	if client != nil {
		p.client = client
	}
	return p
}
func (Bedrock) ID() string { return "bedrock" }

func (p Bedrock) Chat(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (ir.Response, error) {
	body, err := ir.DenormalizeAnthropicRequest(req)
	if err != nil {
		return ir.Response{}, err
	}
	endpoint := bedrockURL(cfg)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ir.Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if err := p.sign(httpReq, body, cfg); err != nil {
		return ir.Response{}, err
	}
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return ir.Response{}, &adapter.Error{Class: adapter.Network, Provider: p.ID(), UpstreamID: cfg.UpstreamID, Message: err.Error(), Retryable: true, Wrapped: err}
	}
	raw, err := readHTTP(p.ID(), cfg, resp)
	if err != nil {
		return ir.Response{}, err
	}
	return parseBedrockAnthropic(raw)
}

func (p Bedrock) Stream(ctx context.Context, req ir.Request, cfg adapter.ProviderConfig) (<-chan ir.Event, error) {
	return streamFromChat(ctx, func(ctx context.Context) (ir.Response, error) { return p.Chat(ctx, req, cfg) })
}
func (Bedrock) CountTokens(_ context.Context, req ir.Request, cfg adapter.ProviderConfig) (int, error) {
	return budget.EstimateInputTokens(req, "bedrock", adapter.RawString(cfg.Raw, "model_id"))
}
func (p Bedrock) HealthCheck(ctx context.Context, cfg adapter.ProviderConfig) error {
	_, err := p.Chat(ctx, ir.Request{ModelAlias: adapter.RawString(cfg.Raw, "model_id"), MaxTokens: 1, Messages: []ir.Message{{Role: ir.RoleUser, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "ping"}}}}}, cfg)
	return err
}
func (Bedrock) Capabilities() adapter.Capabilities {
	return adapter.Capabilities{Streaming: true, ToolUse: true, Vision: true, PromptCaching: true, StabilityClass: adapter.Stable, Category: "cloud_iam"}
}

func (p Bedrock) sign(req *http.Request, payload []byte, cfg adapter.ProviderConfig) error {
	accessKey := adapter.RawString(cfg.Raw, "access_key_id")
	secretKey := adapter.RawString(cfg.Raw, "secret_access_key")
	region := valueOr(adapter.RawString(cfg.Raw, "region"), "us-east-1")
	if accessKey == "" || secretKey == "" {
		return fmt.Errorf("bedrock access_key_id and secret_access_key are required")
	}
	now := p.now().UTC()
	amzDate := now.Format("20060102T150405Z")
	date := now.Format("20060102")
	req.Header.Set("x-amz-date", amzDate)
	if token := adapter.RawString(cfg.Raw, "session_token"); token != "" {
		req.Header.Set("x-amz-security-token", token)
	}
	payloadHash := sha256Hex(payload)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	host := req.URL.Host
	canonicalHeaders := "host:" + host + "\n" + "x-amz-content-sha256:" + payloadHash + "\n" + "x-amz-date:" + amzDate + "\n"
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalRequest := strings.Join([]string{req.Method, req.URL.EscapedPath(), req.URL.RawQuery, canonicalHeaders, signedHeaders, payloadHash}, "\n")
	scope := date + "/" + region + "/bedrock/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + sha256Hex([]byte(canonicalRequest))
	signingKey := awsSigningKey(secretKey, date, region, "bedrock")
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+accessKey+"/"+scope+", SignedHeaders="+signedHeaders+", Signature="+signature)
	return nil
}

func bedrockURL(cfg adapter.ProviderConfig) string {
	base := adapter.RawString(cfg.Raw, "base_url")
	if base == "" {
		base = "https://bedrock-runtime." + valueOr(adapter.RawString(cfg.Raw, "region"), "us-east-1") + ".amazonaws.com"
	}
	modelID := url.PathEscape(adapter.RawString(cfg.Raw, "model_id"))
	return strings.TrimRight(base, "/") + "/model/" + modelID + "/invoke"
}

func parseBedrockAnthropic(raw []byte) (ir.Response, error) {
	resp, err := parseAnthropicLike(raw, "bedrock")
	if err == nil {
		return resp, nil
	}
	return parseOpenAIShape(raw, "bedrock")
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func awsSigningKey(secret, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}
