package audit

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriterWritesAllRecords(t *testing.T) {
	dir := t.TempDir()
	writer, err := NewWriter(dir, nil)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	ts := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	for i := range 10_000 {
		if err := writer.Write(context.Background(), Record{TS: ts, RequestID: string(rune(i)), Status: "success"}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	file, err := os.Open(filepath.Join(dir, "2026-05-07.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 10_000 {
		t.Fatalf("line count = %d, want 10000", count)
	}
}
