package audit

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/storage"
	"github.com/sigilbridge/sigilbridge/internal/storage/repos"
	_ "modernc.org/sqlite"
)

func TestWriterIndexesJSONLOffsets(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := storage.Up(db); err != nil {
		t.Fatal(err)
	}
	indexer := NewIndexer(db)
	writer, err := NewWriter(t.TempDir(), indexer)
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	for _, id := range []string{"req_1", "req_2", "req_3"} {
		if err := writer.Write(context.Background(), Record{TS: ts, RequestID: id, BridgeKeyID: "key", ModelAlias: "sonnet", Status: "success"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}
	if err := indexer.Close(); err != nil {
		t.Fatalf("indexer.Close() error = %v", err)
	}

	entries, err := repos.NewAuditIndex(db).List(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("entries len = %d, want 3", len(entries))
	}
	entry, err := repos.NewAuditIndex(db).Get(context.Background(), "req_2")
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(entry.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	buf := make([]byte, entry.FileLength)
	if _, err := file.ReadAt(buf, entry.FileOffset); err != nil {
		t.Fatal(err)
	}
	if got := string(buf); !strings.Contains(got, `"request_id":"req_2"`) {
		t.Fatalf("offset read wrong line: %s", got)
	}
}
