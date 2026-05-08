package storage

import (
	"path/filepath"
	"testing"
)

func TestBackupRestore(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.db")
	backupPath := filepath.Join(dir, "backup.db")
	restoredPath := filepath.Join(dir, "restored.db")

	db, err := OpenDB(srcPath)
	if err != nil {
		t.Fatalf("OpenDB() error = %v", err)
	}
	if err := Up(db); err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	if err := mustExec(db, `
INSERT INTO bridge_keys (id, hash, name, created_at, scopes_json, budgets_json, rate_limits_json, metadata_json)
VALUES ('key', 'hash', 'name', '2026-05-07T12:00:00Z', '{}', '{}', '{}', '{}')`); err != nil {
		t.Fatalf("insert test row: %v", err)
	}
	_ = db.Close()

	if err := Backup(srcPath, backupPath); err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	if err := Restore(backupPath, restoredPath); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	restored, err := OpenDB(restoredPath)
	if err != nil {
		t.Fatalf("OpenDB(restored) error = %v", err)
	}
	defer restored.Close()
	var name string
	if err := restored.QueryRow("SELECT name FROM bridge_keys WHERE id = 'key'").Scan(&name); err != nil {
		t.Fatalf("query restored row: %v", err)
	}
	if name != "name" {
		t.Fatalf("restored name = %q, want name", name)
	}
}
