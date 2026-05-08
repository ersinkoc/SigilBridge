package admin

import (
	"context"
	"net/http"
)

type AuthService interface {
	Login(ctx context.Context, token string) (session string, err error)
	Verify(ctx context.Context, r *http.Request) (subject string, err error)
}

type KeyService interface {
	List(ctx context.Context) ([]KeyDTO, error)
	Create(ctx context.Context, req CreateKeyRequest) (CreateKeyResponse, error)
	Get(ctx context.Context, id string) (KeyDTO, error)
	Patch(ctx context.Context, id string, patch map[string]any) (KeyDTO, error)
	Delete(ctx context.Context, id string) error
}

type PoolService interface {
	List(ctx context.Context) ([]PoolDTO, error)
	Upsert(ctx context.Context, pool PoolDTO) (PoolDTO, error)
	Delete(ctx context.Context, id string) error
	Probe(ctx context.Context, id string) (map[string]any, error)
}

type EndpointService interface {
	Info(ctx context.Context) (EndpointInfoResponse, error)
}

type ChatService interface {
	Test(ctx context.Context, req ChatTestRequest) (ChatTestResponse, error)
}

type CredentialService interface {
	List(ctx context.Context) (map[string]any, error)
	Delete(ctx context.Context, id string) error
	APIKeyCreate(ctx context.Context, req APIKeyCredentialRequest) (map[string]any, error)
	SessionCreate(ctx context.Context, req SessionCredentialRequest) (map[string]any, error)
	OAuthBootstrap(ctx context.Context, provider, name, mode string) (map[string]any, error)
	OAuthCallback(ctx context.Context, state, code, errorText string) (map[string]any, error)
	OAuthRefresh(ctx context.Context, id string) (map[string]any, error)
	OAuthRevoke(ctx context.Context, id string) (map[string]any, error)
	OAuthProvidersRaw(ctx context.Context) (map[string]any, error)
	OAuthProvidersSave(ctx context.Context, body string) (map[string]any, error)
	CLIStatus(ctx context.Context) (map[string]any, error)
	CLIDetect(ctx context.Context) (map[string]any, error)
	CLIEnable(ctx context.Context, req CLIEnableRequest) (map[string]any, error)
	ProviderCatalog(ctx context.Context) (map[string]any, error)
}

type AuditService interface {
	Query(ctx context.Context, values map[string][]string) (map[string]any, error)
}

type BudgetService interface {
	Budgets(ctx context.Context) (map[string]any, error)
	Usage(ctx context.Context) (map[string]any, error)
}

type HealthService interface {
	Detail(ctx context.Context) (map[string]any, error)
}

type ReloadService interface {
	Reload(ctx context.Context) (ReloadResult, error)
}

type KeyDTO struct {
	ID         string         `json:"id"`
	Hash       string         `json:"hash,omitempty"`
	Name       string         `json:"name,omitempty"`
	CreatedAt  string         `json:"created_at,omitempty"`
	CreatedBy  string         `json:"created_by,omitempty"`
	LastUsedAt string         `json:"last_used_at,omitempty"`
	RevokedAt  string         `json:"revoked_at,omitempty"`
	Scopes     map[string]any `json:"scopes,omitempty"`
	Budgets    map[string]any `json:"budgets,omitempty"`
	RateLimits map[string]any `json:"rate_limits,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type CreateKeyRequest struct {
	Prefix     string         `json:"prefix"`
	Scopes     map[string]any `json:"scopes,omitempty"`
	Budgets    map[string]any `json:"budgets,omitempty"`
	RateLimits map[string]any `json:"rate_limits,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type CreateKeyResponse struct {
	KeyDTO
	Plaintext string `json:"plaintext"`
}

type PoolDTO struct {
	ID        string           `json:"id"`
	Strategy  string           `json:"strategy,omitempty"`
	Upstreams []map[string]any `json:"upstreams,omitempty"`
}

type ReloadResult struct {
	OK                    bool     `json:"ok"`
	RestartRequiredFields []string `json:"restart_required_fields,omitempty"`
}

type EndpointInfoResponse struct {
	OpenAIBase        string `json:"openai_base"`
	OpenAIChat        string `json:"openai_chat"`
	OpenAIModels      string `json:"openai_models"`
	AnthropicBase     string `json:"anthropic_base"`
	AnthropicMessages string `json:"anthropic_messages"`
}

type ChatTestRequest struct {
	Model       string   `json:"model"`
	Message     string   `json:"message"`
	System      string   `json:"system,omitempty"`
	Temperature *float32 `json:"temperature,omitempty"`
}

type ChatTestResponse struct {
	ID               string `json:"id,omitempty"`
	Model            string `json:"model,omitempty"`
	UpstreamProvider string `json:"upstream_provider,omitempty"`
	UpstreamModel    string `json:"upstream_model,omitempty"`
	Content          string `json:"content"`
	StopReason       string `json:"stop_reason,omitempty"`
	LatencyMs        int64  `json:"latency_ms,omitempty"`
	InputTokens      int    `json:"input_tokens,omitempty"`
	OutputTokens     int    `json:"output_tokens,omitempty"`
}

type SessionCredentialRequest struct {
	Provider       string            `json:"provider"`
	Name           string            `json:"name"`
	Cookies        map[string]string `json:"cookies"`
	UserAgent      string            `json:"user_agent"`
	OrganizationID string            `json:"organization_id,omitempty"`
}

type APIKeyCredentialRequest struct {
	Provider   string `json:"provider"`
	Name       string `json:"name"`
	APIKey     string `json:"api_key"`
	Model      string `json:"model,omitempty"`
	BaseURL    string `json:"base_url,omitempty"`
	Pool       string `json:"pool,omitempty"`
	UpstreamID string `json:"upstream_id,omitempty"`
}

type CLIEnableRequest struct {
	Provider   string   `json:"provider"`
	Command    string   `json:"command,omitempty"`
	Protocol   string   `json:"protocol,omitempty"`
	Framing    string   `json:"framing,omitempty"`
	Args       []string `json:"args,omitempty"`
	Model      string   `json:"model,omitempty"`
	Pool       string   `json:"pool,omitempty"`
	UpstreamID string   `json:"upstream_id,omitempty"`
}

type AuditResponse struct {
	Items      []map[string]any `json:"items"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

type HealthResponse struct {
	Upstreams []map[string]any `json:"upstreams,omitempty"`
	Cooldowns []map[string]any `json:"cooldowns,omitempty"`
}

type BudgetResponse struct {
	Keys             int `json:"keys,omitempty"`
	DailyCents       int `json:"daily_cents,omitempty"`
	MonthlyCents     int `json:"monthly_cents,omitempty"`
	DailyUsedCents   int `json:"daily_used_cents,omitempty"`
	MonthlyUsedCents int `json:"monthly_used_cents,omitempty"`
}

type UsageResponse struct {
	Items []map[string]any `json:"items,omitempty"`
}

type CredentialsResponse struct {
	OAuthProviders []OAuthProviderDTO     `json:"oauth_providers,omitempty"`
	APIKeys        []CredentialSessionDTO `json:"api_keys,omitempty"`
	Sessions       []CredentialSessionDTO `json:"sessions,omitempty"`
	CLI            CLIStatusDTO           `json:"cli,omitempty"`
	Catalog        ProviderCatalogDTO     `json:"catalog,omitempty"`
}

type OAuthProviderDTO struct {
	ID                 string   `json:"id"`
	DisplayName        string   `json:"display_name,omitempty"`
	ClientID           string   `json:"client_id,omitempty"`
	BrowserBootstrap   bool     `json:"browser_bootstrap,omitempty"`
	DeviceBootstrap    bool     `json:"device_bootstrap,omitempty"`
	ConfiguredClient   bool     `json:"configured_client,omitempty"`
	MetadataConfigured bool     `json:"metadata_configured,omitempty"`
	Usable             bool     `json:"usable,omitempty"`
	MissingFields      []string `json:"missing_fields,omitempty"`
	AuthURL            string   `json:"auth_url,omitempty"`
	TokenURL           string   `json:"token_url,omitempty"`
	DeviceAuthURL      string   `json:"device_auth_url,omitempty"`
	RevokeURL          string   `json:"revoke_url,omitempty"`
	DefaultScopes      []string `json:"default_scopes,omitempty"`
}

type CredentialSessionDTO struct {
	ID              string                    `json:"id"`
	Provider        string                    `json:"provider,omitempty"`
	CreatedAt       string                    `json:"created_at,omitempty"`
	LastRefreshedAt string                    `json:"last_refreshed_at,omitempty"`
	ExpiresAt       string                    `json:"expires_at,omitempty"`
	Metadata        map[string]any            `json:"metadata,omitempty"`
	Attachments     []CredentialAttachmentDTO `json:"attachments,omitempty"`
}

type CredentialAttachmentDTO struct {
	Pool       string `json:"pool,omitempty"`
	UpstreamID string `json:"upstream_id,omitempty"`
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
	BaseURL    string `json:"base_url,omitempty"`
}

type CLIStatusDTO struct {
	Enabled                   bool          `json:"enabled,omitempty"`
	DefaultIdleTimeoutSeconds int           `json:"default_idle_timeout_seconds,omitempty"`
	StderrCaptureBytes        int           `json:"stderr_capture_bytes,omitempty"`
	HealthCheckInterval       int           `json:"health_check_interval,omitempty"`
	Agents                    []CLIAgentDTO `json:"agents,omitempty"`
}

type CLIAgentDTO struct {
	Pool       string   `json:"pool,omitempty"`
	Upstream   string   `json:"upstream,omitempty"`
	Provider   string   `json:"provider,omitempty"`
	Name       string   `json:"name,omitempty"`
	Command    string   `json:"command,omitempty"`
	Protocol   string   `json:"protocol,omitempty"`
	Framing    string   `json:"framing,omitempty"`
	Args       []string `json:"args,omitempty"`
	Path       string   `json:"path,omitempty"`
	Configured bool     `json:"configured,omitempty"`
	Available  bool     `json:"available,omitempty"`
	AuthStatus string   `json:"auth_status,omitempty"`
	Source     string   `json:"source,omitempty"`
	Version    string   `json:"version,omitempty"`
	Error      string   `json:"error,omitempty"`
}

type ProviderCatalogDTO struct {
	Source    string               `json:"source,omitempty"`
	Providers []CatalogProviderDTO `json:"providers,omitempty"`
}

type CatalogProviderDTO struct {
	ID          string            `json:"id"`
	Name        string            `json:"name,omitempty"`
	Provider    string            `json:"provider,omitempty"`
	Category    string            `json:"category,omitempty"`
	BaseURL     string            `json:"base_url,omitempty"`
	Env         []string          `json:"env,omitempty"`
	ModelCount  int               `json:"model_count,omitempty"`
	TopModels   []CatalogModelDTO `json:"top_models,omitempty"`
	Configured  bool              `json:"configured,omitempty"`
	Available   bool              `json:"available,omitempty"`
	Description string            `json:"description,omitempty"`
}

type CatalogModelDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
	Context   int    `json:"context,omitempty"`
	Output    int    `json:"output,omitempty"`
}

func sessionCookie(value string, secure bool) *http.Cookie {
	return &http.Cookie{Name: "sigilbridge_admin", Value: value, Path: "/admin", HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode}
}
