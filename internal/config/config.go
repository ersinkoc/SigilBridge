package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server               ServerConfig               `yaml:"server"`
	Admin                AdminConfig                `yaml:"admin"`
	Storage              StorageConfig              `yaml:"storage"`
	Audit                AuditConfig                `yaml:"audit"`
	Vault                VaultConfig                `yaml:"vault"`
	OAuth                OAuthConfig                `yaml:"oauth"`
	CLIAgents            CLIAgentsConfig            `yaml:"cli_agents"`
	SubscriptionAdapters SubscriptionAdaptersConfig `yaml:"subscription_adapters"`
	Logging              LoggingConfig              `yaml:"logging"`
	Metrics              MetricsConfig              `yaml:"metrics"`
	PoolsFile            string                     `yaml:"pools_file"`
}

type ServerConfig struct {
	Bind                  string    `yaml:"bind"`
	TLS                   TLSConfig `yaml:"tls"`
	MaxConcurrentRequests int       `yaml:"max_concurrent_requests"`
	RequestTimeoutSeconds int       `yaml:"request_timeout_seconds"`
	IdleTimeoutSeconds    int       `yaml:"idle_timeout_seconds"`
	ShutdownGraceSeconds  int       `yaml:"shutdown_grace_seconds"`
}

type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type AdminConfig struct {
	Bind       string `yaml:"bind"`
	TokensFile string `yaml:"tokens_file"`
	UIEnabled  bool   `yaml:"ui_enabled"`
}

type StorageConfig struct {
	Path          string              `yaml:"path"`
	BusyTimeoutMS int                 `yaml:"busy_timeout_ms"`
	CacheSizeKB   int                 `yaml:"cache_size_kb"`
	MmapSizeMB    int                 `yaml:"mmap_size_mb"`
	Backup        StorageBackupConfig `yaml:"backup"`
}

type StorageBackupConfig struct {
	Enabled       bool   `yaml:"enabled"`
	IntervalHours int    `yaml:"interval_hours"`
	RetentionDays int    `yaml:"retention_days"`
	Path          string `yaml:"path"`
}

type AuditConfig struct {
	Enabled                 bool   `yaml:"enabled"`
	Path                    string `yaml:"path"`
	ContentMode             string `yaml:"content_mode"`
	RetentionDays           int    `yaml:"retention_days"`
	RotateCompressAfterDays int    `yaml:"rotate_compress_after_days"`
}

type VaultConfig struct {
	MasterKeyEnv string `yaml:"master_key_env"`
}

type OAuthConfig struct {
	RefreshCheckIntervalSeconds int    `yaml:"refresh_check_interval_seconds"`
	RefreshLeadTimeSeconds      int    `yaml:"refresh_lead_time_seconds"`
	BootstrapListenerAddr       string `yaml:"bootstrap_listener_addr"`
	ProvidersFile               string `yaml:"providers_file"`
}

type CLIAgentsConfig struct {
	Enabled                    bool   `yaml:"enabled"`
	DefaultIdleTimeoutSeconds  int    `yaml:"default_idle_timeout_seconds"`
	DefaultStderrCaptureBytes  int    `yaml:"default_stderr_capture_bytes"`
	HealthCheckIntervalSeconds int    `yaml:"health_check_interval_seconds"`
	SpawnLogLevel              string `yaml:"spawn_log_level"`
}

type SubscriptionAdaptersConfig struct {
	Enabled                bool `yaml:"enabled"`
	AcknowledgeTOSRisk     bool `yaml:"acknowledge_tos_risk"`
	RefreshIntervalSeconds int  `yaml:"refresh_interval_seconds"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	File   string `yaml:"file"`
}

type MetricsConfig struct {
	PrometheusEnabled bool   `yaml:"prometheus_enabled"`
	Bind              string `yaml:"bind"`
}

func Load(path string) (*Config, error) {
	// #nosec G304 -- configuration path is explicit local operator input.
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	cfg, err := Parse(raw)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func Parse(raw []byte) (*Config, error) {
	expanded := os.ExpandEnv(string(raw))
	dec := yaml.NewDecoder(bytes.NewReader([]byte(expanded)))
	dec.KnownFields(true)

	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Validate() error {
	if strings.TrimSpace(c.Server.Bind) == "" {
		return fmt.Errorf("server.bind is required")
	}
	if strings.TrimSpace(c.Admin.Bind) == "" {
		return fmt.Errorf("admin.bind is required")
	}
	if strings.TrimSpace(c.Storage.Path) == "" {
		return fmt.Errorf("storage.path is required")
	}
	if c.Storage.BusyTimeoutMS < 0 {
		return fmt.Errorf("storage.busy_timeout_ms must be non-negative")
	}
	if c.Server.MaxConcurrentRequests < 1 {
		return fmt.Errorf("server.max_concurrent_requests must be at least 1")
	}
	if c.Audit.ContentMode != "none" && c.Audit.ContentMode != "hash" && c.Audit.ContentMode != "truncated" && c.Audit.ContentMode != "full" {
		return fmt.Errorf("audit.content_mode must be one of none, hash, truncated, full")
	}
	if c.Audit.Enabled && strings.TrimSpace(c.Audit.Path) == "" {
		return fmt.Errorf("audit.path is required when audit.enabled is true")
	}
	if c.Logging.Format != "json" && c.Logging.Format != "text" {
		return fmt.Errorf("logging.format must be json or text")
	}
	if c.Vault.MasterKeyEnv == "" {
		return fmt.Errorf("vault.master_key_env is required")
	}
	return nil
}

func (c *Config) applyDefaults() {
	if c.Server.Bind == "" {
		c.Server.Bind = "0.0.0.0:8787"
	}
	if c.Server.MaxConcurrentRequests == 0 {
		c.Server.MaxConcurrentRequests = 1024
	}
	if c.Server.RequestTimeoutSeconds == 0 {
		c.Server.RequestTimeoutSeconds = 600
	}
	if c.Server.IdleTimeoutSeconds == 0 {
		c.Server.IdleTimeoutSeconds = 120
	}
	if c.Server.ShutdownGraceSeconds == 0 {
		c.Server.ShutdownGraceSeconds = 30
	}
	if c.Admin.Bind == "" {
		c.Admin.Bind = "127.0.0.1:8788"
	}
	if c.Admin.TokensFile == "" {
		c.Admin.TokensFile = "admin_tokens.yaml"
	}
	if c.Storage.Path == "" {
		c.Storage.Path = "data/sigilbridge.db"
	}
	if c.Storage.BusyTimeoutMS == 0 {
		c.Storage.BusyTimeoutMS = 5000
	}
	if c.Storage.CacheSizeKB == 0 {
		c.Storage.CacheSizeKB = 20000
	}
	if c.Storage.MmapSizeMB == 0 {
		c.Storage.MmapSizeMB = 256
	}
	if c.Storage.Backup.Path == "" {
		c.Storage.Backup.Path = "backup/"
	}
	if c.Audit.ContentMode == "" {
		c.Audit.ContentMode = "none"
	}
	if c.Audit.Path == "" {
		c.Audit.Path = "audit"
	}
	if c.Vault.MasterKeyEnv == "" {
		c.Vault.MasterKeyEnv = "SIGILBRIDGE_MASTER_KEY"
	}
	if c.OAuth.BootstrapListenerAddr == "" {
		c.OAuth.BootstrapListenerAddr = "127.0.0.1:0"
	}
	if c.OAuth.ProvidersFile == "" {
		c.OAuth.ProvidersFile = "oauth_providers.yaml"
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "json"
	}
	if c.PoolsFile == "" {
		c.PoolsFile = "pools.yaml"
	}
}

func ResolveRelative(configPath, value string) string {
	if value == "" || filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(filepath.Dir(configPath), value)
}
