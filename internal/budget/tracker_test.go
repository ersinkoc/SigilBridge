package budget

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/auth"
	"github.com/sigilbridge/sigilbridge/internal/storage"
	"github.com/sigilbridge/sigilbridge/internal/storage/repos"
	_ "modernc.org/sqlite"
)

func TestTrackerPreCheckAndCommit(t *testing.T) {
	ctx := context.Background()
	db := budgetTestDB(t)
	key := auth.BridgeKey{ID: "key", Budgets: auth.Budgets{DailyCents: 100, MonthlyCents: 1000, HardCap: true}}
	if err := repos.NewBridgeKeys(db).Put(ctx, repos.BridgeKey{ID: key.ID, Hash: "hash", Name: "key", CreatedAt: budgetNow(), ScopesJSON: "{}", BudgetsJSON: "{}", RateLimitsJSON: "{}", MetadataJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	tracker := NewTracker(db, budgetNow)
	if warn, err := tracker.PreCheck(ctx, key, 50); err != nil || warn {
		t.Fatalf("PreCheck() = %v, %v", warn, err)
	}
	if err := tracker.Commit(ctx, key.ID, 90); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if _, err := tracker.PreCheck(ctx, key, 11); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("PreCheck(over) error = %v", err)
	}
	key.Budgets.HardCap = false
	if warn, err := tracker.PreCheck(ctx, key, 11); err != nil || !warn {
		t.Fatalf("soft PreCheck() = %v, %v; want warning", warn, err)
	}
}

func budgetTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := storage.OpenDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := storage.Up(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func budgetNow() time.Time {
	return time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
}
