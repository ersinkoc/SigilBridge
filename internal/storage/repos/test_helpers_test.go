package repos

import (
	"database/sql"
	"testing"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/storage"
	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	if err := storage.Up(db); err != nil {
		_ = db.Close()
		t.Fatalf("migrate memory db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func testTime() time.Time {
	return time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
}
