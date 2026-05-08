package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReloaderReloadSwapsSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	write := func(bind string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(`
server:
  bind: "`+bind+`"
admin:
  bind: "127.0.0.1:2"
storage:
  path: "x.db"
vault:
  master_key_env: SIGILBRIDGE_MASTER_KEY
`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("127.0.0.1:1")
	reloader, err := NewReloader(path)
	if err != nil {
		t.Fatalf("NewReloader() error = %v", err)
	}
	write("127.0.0.1:3")
	cfg, err := reloader.Reload()
	if err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	if cfg.Server.Bind != "127.0.0.1:3" || reloader.Current().Server.Bind != "127.0.0.1:3" {
		t.Fatalf("snapshot was not swapped")
	}
}
