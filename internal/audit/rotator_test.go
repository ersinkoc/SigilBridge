package audit

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRotateAndPrune(t *testing.T) {
	dir := t.TempDir()
	write := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte("line\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("2026-05-06.jsonl")
	write("2026-04-30.jsonl")
	write("2026-02-01.jsonl.gz")

	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	if err := RotateAndPrune(dir, now, 3, 30); err != nil {
		t.Fatalf("RotateAndPrune() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "2026-05-06.jsonl")); err != nil {
		t.Fatalf("recent file should remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "2026-04-30.jsonl.gz")); err != nil {
		t.Fatalf("old file should be gzipped: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "2026-04-30.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("uncompressed old file should be removed, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "2026-02-01.jsonl.gz")); !os.IsNotExist(err) {
		t.Fatalf("expired file should be pruned, stat err=%v", err)
	}
}
