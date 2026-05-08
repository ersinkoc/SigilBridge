package repos

import (
	"context"
	"testing"
)

func TestBudgetCountersIncrement(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	keys := NewBridgeKeys(db)
	if err := keys.Put(ctx, BridgeKey{ID: "key", Hash: "hash", Name: "key", CreatedAt: testTime(), ScopesJSON: "{}", BudgetsJSON: "{}", RateLimitsJSON: "{}", MetadataJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	repo := NewBudgetCounters(db)
	if err := repo.Increment(ctx, "key", "daily", "2026-05-07", 10); err != nil {
		t.Fatalf("Increment() error = %v", err)
	}
	if err := repo.Increment(ctx, "key", "daily", "2026-05-07", 15); err != nil {
		t.Fatalf("Increment() error = %v", err)
	}
	got, err := repo.Get(ctx, "key", "daily", "2026-05-07")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Cents != 25 {
		t.Fatalf("Cents = %d, want 25", got.Cents)
	}
}
