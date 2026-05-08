package storage

import (
	"bytes"
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrationsUpDownStatus(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	defer db.Close()

	if err := Up(db); err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	for _, table := range []string{"bridge_keys", "sessions", "budget_counters", "ratelimit_buckets", "audit_index", "cooldowns", "events"} {
		var name string
		if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&name); err != nil {
			t.Fatalf("table %q missing after Up(): %v", table, err)
		}
	}
	version, err := CurrentVersion(db)
	if err != nil {
		t.Fatalf("CurrentVersion() error = %v", err)
	}
	if version != 4 {
		t.Fatalf("version = %d, want 4", version)
	}

	var status bytes.Buffer
	if err := Status(db, &status); err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if !strings.Contains(status.String(), "00004") {
		t.Fatalf("status output did not include latest migration:\n%s", status.String())
	}

	if err := DownToZero(db); err != nil {
		t.Fatalf("DownToZero() error = %v", err)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'bridge_keys'").Scan(&count); err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	if count != 0 {
		t.Fatalf("bridge_keys table still exists after DownToZero")
	}
}
