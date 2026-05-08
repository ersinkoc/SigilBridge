package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestRunVersionAndKeys(t *testing.T) {
	if err := run([]string{"version"}); err != nil {
		t.Fatalf("version error = %v", err)
	}
	if err := run([]string{"keys", "create", "test"}); err != nil {
		t.Fatalf("keys error = %v", err)
	}
}

func TestRunOAuthListWithConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	writeTestConfig(t, configPath)
	key := make([]byte, 32)
	t.Setenv("SIGILBRIDGE_MASTER_KEY", base64.StdEncoding.EncodeToString(key))
	if err := run([]string{"oauth", "list", "--config", configPath}); err != nil {
		t.Fatalf("oauth list error = %v", err)
	}
}

func TestFirstPositionalSkipsFlags(t *testing.T) {
	got := firstPositional([]string{"--config", "config.yaml", "--name", "personal", "claude_max"})
	if got != "claude_max" {
		t.Fatalf("firstPositional() = %q", got)
	}
	got = firstPositional([]string{"-c", "config.yaml", "vault://oauth/claude_max/personal"})
	if got != "vault://oauth/claude_max/personal" {
		t.Fatalf("firstPositional() = %q", got)
	}
}

func writeTestConfig(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(`
server:
  bind: 127.0.0.1:8787
admin:
  bind: 127.0.0.1:8788
storage:
  path: data/sigilbridge.db
vault:
  master_key_env: SIGILBRIDGE_MASTER_KEY
logging:
  format: json
`), 0o600); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
}
