package storage

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenDBAppliesPragmas(t *testing.T) {
	db, err := OpenDB(filepath.Join(t.TempDir(), "sigilbridge.db"))
	if err != nil {
		t.Fatalf("OpenDB() error = %v", err)
	}
	defer db.Close()

	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if strings.ToLower(journalMode) != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}

	var foreignKeys int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}

	var busyTimeout int
	if err := db.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("query busy_timeout: %v", err)
	}
	if busyTimeout != defaultBusyTimeoutMS {
		t.Fatalf("busy_timeout = %d, want %d", busyTimeout, defaultBusyTimeoutMS)
	}
}

func TestOpenDBMemory(t *testing.T) {
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB(:memory:) error = %v", err)
	}
	defer db.Close()
}
