package repos

import (
	"context"
	"testing"
)

func TestRateLimitBucketsIncrement(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	if err := NewBridgeKeys(db).Put(ctx, BridgeKey{ID: "key", Hash: "hash", Name: "key", CreatedAt: testTime(), ScopesJSON: "{}", BudgetsJSON: "{}", RateLimitsJSON: "{}", MetadataJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	repo := NewRateLimitBuckets(db)
	if err := repo.Increment(ctx, "key", "rpm", 123, 1); err != nil {
		t.Fatalf("Increment() error = %v", err)
	}
	if err := repo.Increment(ctx, "key", "rpm", 123, 2); err != nil {
		t.Fatalf("Increment() error = %v", err)
	}
	got, err := repo.Get(ctx, "key", "rpm", 123)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Value != 3 {
		t.Fatalf("Value = %d, want 3", got.Value)
	}
}
