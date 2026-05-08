export type ApiError = {
  error?: string | { message?: string };
};

export type KeyDTO = {
  id: string;
  hash?: string;
  name?: string;
  created_at?: string;
  created_by?: string;
  last_used_at?: string;
  revoked_at?: string;
  scopes?: Record<string, unknown>;
  budgets?: Record<string, unknown>;
  rate_limits?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
};

export type CreateKeyRequest = {
  prefix: string;
  scopes?: Record<string, unknown>;
  budgets?: Record<string, unknown>;
  rate_limits?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
};

export type CreateKeyResponse = KeyDTO & {
  plaintext: string;
};

export type PoolDTO = {
  id: string;
  strategy?: string;
  upstreams?: Array<Record<string, unknown>>;
};

export type ReloadResult = {
  ok: boolean;
  restart_required_fields?: Array<string>;
};

export type EndpointInfoResponse = {
  openai_base: string;
  openai_chat: string;
  openai_models: string;
  anthropic_base: string;
  anthropic_messages: string;
};

export type ChatTestRequest = {
  model: string;
  message: string;
  system?: string;
  temperature?: unknown;
};

export type ChatTestResponse = {
  id?: string;
  model?: string;
  upstream_provider?: string;
  upstream_model?: string;
  content: string;
  stop_reason?: string;
  latency_ms?: number;
  input_tokens?: number;
  output_tokens?: number;
};

export type SessionCredentialRequest = {
  provider: string;
  name: string;
  cookies: Record<string, string>;
  user_agent: string;
  organization_id?: string;
};

export type APIKeyCredentialRequest = {
  provider: string;
  name: string;
  api_key: string;
  model?: string;
  base_url?: string;
  pool?: string;
  upstream_id?: string;
};

export type CLIEnableRequest = {
  provider: string;
  command?: string;
  protocol?: string;
  framing?: string;
  args?: Array<string>;
  model?: string;
  pool?: string;
  upstream_id?: string;
};

export type AuditResponse = {
  items: Array<Record<string, unknown>>;
  next_cursor?: string;
};

export type HealthResponse = {
  upstreams?: Array<Record<string, unknown>>;
  cooldowns?: Array<Record<string, unknown>>;
};

export type BudgetResponse = {
  keys?: number;
  daily_cents?: number;
  monthly_cents?: number;
  daily_used_cents?: number;
  monthly_used_cents?: number;
};

export type UsageResponse = {
  items?: Array<Record<string, unknown>>;
};

export type CredentialsResponse = {
  oauth_providers?: Array<OAuthProviderDTO>;
  api_keys?: Array<CredentialSessionDTO>;
  sessions?: Array<CredentialSessionDTO>;
  cli?: CLIStatusDTO;
  catalog?: ProviderCatalogDTO;
};

export type OAuthProviderDTO = {
  id: string;
  display_name?: string;
  client_id?: string;
  browser_bootstrap?: boolean;
  device_bootstrap?: boolean;
  configured_client?: boolean;
  metadata_configured?: boolean;
  usable?: boolean;
  missing_fields?: Array<string>;
  auth_url?: string;
  token_url?: string;
  device_auth_url?: string;
  revoke_url?: string;
  default_scopes?: Array<string>;
};

export type CredentialSessionDTO = {
  id: string;
  provider?: string;
  created_at?: string;
  last_refreshed_at?: string;
  expires_at?: string;
  metadata?: Record<string, unknown>;
  attachments?: Array<CredentialAttachmentDTO>;
};

export type CredentialAttachmentDTO = {
  pool?: string;
  upstream_id?: string;
  provider?: string;
  model?: string;
  base_url?: string;
};

export type CLIStatusDTO = {
  enabled?: boolean;
  default_idle_timeout_seconds?: number;
  stderr_capture_bytes?: number;
  health_check_interval?: number;
  agents?: Array<CLIAgentDTO>;
};

export type CLIAgentDTO = {
  pool?: string;
  upstream?: string;
  provider?: string;
  name?: string;
  command?: string;
  protocol?: string;
  framing?: string;
  args?: Array<string>;
  path?: string;
  configured?: boolean;
  available?: boolean;
  auth_status?: string;
  source?: string;
  version?: string;
  error?: string;
};

export type ProviderCatalogDTO = {
  source?: string;
  providers?: Array<CatalogProviderDTO>;
};

export type CatalogProviderDTO = {
  id: string;
  name?: string;
  provider?: string;
  category?: string;
  base_url?: string;
  env?: Array<string>;
  model_count?: number;
  top_models?: Array<CatalogModelDTO>;
  configured?: boolean;
  available?: boolean;
  description?: string;
};

export type CatalogModelDTO = {
  id: string;
  name?: string;
  updated_at?: string;
  context?: number;
  output?: number;
};
