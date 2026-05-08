package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCompleteConfigRoundTrip(t *testing.T) {
	t.Setenv("SIGILBRIDGE_TEST_DB", "data/test.db")
	cfg, err := Parse([]byte(`
server:
  bind: "0.0.0.0:8787"
  tls:
    enabled: false
    cert_file: ""
    key_file: ""
  max_concurrent_requests: 1024
  request_timeout_seconds: 600
  idle_timeout_seconds: 120
  shutdown_grace_seconds: 30
admin:
  bind: "127.0.0.1:8788"
  tokens_file: "admin_tokens.yaml"
  ui_enabled: true
storage:
  path: "${SIGILBRIDGE_TEST_DB}"
  busy_timeout_ms: 5000
  cache_size_kb: 20000
  mmap_size_mb: 256
  backup:
    enabled: true
    interval_hours: 24
    retention_days: 14
    path: "backup/"
audit:
  enabled: true
  path: audit
  content_mode: none
  retention_days: 90
  rotate_compress_after_days: 7
vault:
  master_key_env: SIGILBRIDGE_MASTER_KEY
oauth:
  refresh_check_interval_seconds: 300
  refresh_lead_time_seconds: 300
  bootstrap_listener_addr: "127.0.0.1:0"
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
  file: ""
metrics:
  prometheus_enabled: true
  bind: ""
pools_file: pools.yaml
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.Storage.Path != "data/test.db" {
		t.Fatalf("Storage.Path = %q, want env-expanded path", cfg.Storage.Path)
	}
	if cfg.Audit.Path != "audit" {
		t.Fatalf("Audit.Path = %q, want audit", cfg.Audit.Path)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() after parse = %v", err)
	}
}

func TestParseRejectsUnknownFields(t *testing.T) {
	_, err := Parse([]byte(`
server:
  bind: "127.0.0.1:1"
  typo: true
admin:
  bind: "127.0.0.1:2"
storage:
  path: "x.db"
vault:
  master_key_env: SIGILBRIDGE_MASTER_KEY
`))
	if err == nil {
		t.Fatal("Parse() error = nil, want unknown-field error")
	}
}

func TestLoadReadsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(`
server:
  bind: "127.0.0.1:1"
admin:
  bind: "127.0.0.1:2"
storage:
  path: "x.db"
vault:
  master_key_env: SIGILBRIDGE_MASTER_KEY
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
}
