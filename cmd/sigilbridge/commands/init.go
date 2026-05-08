package commands

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

type InitResult struct {
	Dir                string
	ConfigPath         string
	PoolsPath          string
	OAuthProvidersPath string
	AdminTokensPath    string
	AdminToken         string
	Created            bool
}

func InitConfig(dir string, force bool) (InitResult, error) {
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return InitResult{}, fmt.Errorf("create config directory %q: %w", dir, err)
	}
	token, err := randomToken()
	if err != nil {
		return InitResult{}, err
	}
	result := InitResult{
		Dir:                dir,
		ConfigPath:         filepath.Join(dir, "config.yaml"),
		PoolsPath:          filepath.Join(dir, "pools.yaml"),
		OAuthProvidersPath: filepath.Join(dir, "oauth_providers.yaml"),
		AdminTokensPath:    filepath.Join(dir, "admin_tokens.yaml"),
		AdminToken:         token,
		Created:            true,
	}
	files := map[string]string{
		result.ConfigPath:         configTemplate(),
		result.PoolsPath:          poolsTemplate(),
		result.OAuthProvidersPath: oauthProvidersTemplate(),
		result.AdminTokensPath: fmt.Sprintf(`tokens:
  - name: local-admin
    token: %s
`, token),
	}
	for path, body := range files {
		if !force {
			if _, err := os.Stat(path); err == nil {
				return InitResult{}, fmt.Errorf("%s already exists; use --force to overwrite", path)
			} else if !os.IsNotExist(err) {
				return InitResult{}, err
			}
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			return InitResult{}, fmt.Errorf("write %q: %w", path, err)
		}
	}
	return result, nil
}

func randomToken() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate admin token: %w", err)
	}
	return "admin_" + hex.EncodeToString(raw), nil
}

func configTemplate() string {
	return `server:
  bind: 127.0.0.1:8787
  max_concurrent_requests: 256
  request_timeout_seconds: 600
  idle_timeout_seconds: 120
  shutdown_grace_seconds: 30

admin:
  bind: 127.0.0.1:8788
  tokens_file: admin_tokens.yaml
  ui_enabled: true

storage:
  path: data/sigilbridge.db
  busy_timeout_ms: 5000
  cache_size_kb: 20000
  mmap_size_mb: 256
  backup:
    enabled: true
    interval_hours: 24
    retention_days: 14
    path: backup

audit:
  enabled: true
  path: audit
  content_mode: none
  retention_days: 30
  rotate_compress_after_days: 7

vault:
  master_key_env: SIGILBRIDGE_MASTER_KEY

oauth:
  refresh_check_interval_seconds: 300
  refresh_lead_time_seconds: 300
  bootstrap_listener_addr: 127.0.0.1:0
  providers_file: oauth_providers.yaml

cli_agents:
  enabled: true
  default_idle_timeout_seconds: 600
  default_stderr_capture_bytes: 4096
  health_check_interval_seconds: 60
  spawn_log_level: warn

subscription_adapters:
  enabled: false
  acknowledge_tos_risk: false
  refresh_interval_seconds: 21600

logging:
  level: info
  format: json

metrics:
  prometheus_enabled: true
  bind: ""

pools_file: pools.yaml
`
}

func oauthProvidersTemplate() string {
	return `providers:
  # Claude subscription OAuth does not expose stable public OAuth metadata.
  # Use Claude Code CLI from Credentials > CLI for local subscription-backed access.
  - id: claude_oauth
    display_name: Claude

  # GitHub OAuth metadata for Copilot-style account bootstrap. Add an operator-owned
  # OAuth client_id before using browser/device login.
  - id: copilot_oauth
    display_name: GitHub Copilot
    auth_url: https://github.com/login/oauth/authorize
    token_url: https://github.com/login/oauth/access_token
    device_auth_url: https://github.com/login/device/code
    default_scopes:
      - read:user

  # Google OAuth metadata. Add an operator-owned OAuth client_id and scopes matching
  # the concrete Gemini/GCP surface you intend to call.
  - id: gemini_oauth
    display_name: Google Gemini
    auth_url: https://accounts.google.com/o/oauth2/v2/auth
    token_url: https://oauth2.googleapis.com/token
    device_auth_url: https://oauth2.googleapis.com/device/code
    revoke_url: https://oauth2.googleapis.com/revoke
    default_scopes:
      - openid
      - email
      - profile

  # Cursor account OAuth metadata is not stable/public here. Use Cursor CLI/session
  # support when available or fill these fields with operator-owned metadata.
  - id: cursor_oauth
    display_name: Cursor
`
}

func poolsTemplate() string {
	return `pools:
  - name: mock
    description: Deterministic local test pool.
    strategy: priority_first
    upstreams:
      - id: mock-primary
        provider: mock
        priority: 1
        weight: 1
        config:
          latency_ms: 25
          input_tokens: 10
          output_tokens: 5
`
}
