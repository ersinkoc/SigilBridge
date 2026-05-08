package budget

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/storage/repos"
)

func TestRateLimiterRPM(t *testing.T) {
	ctx := context.Background()
	db := budgetTestDB(t)
	if err := repos.NewBridgeKeys(db).Put(ctx, repos.BridgeKey{ID: "key", Hash: "hash", Name: "key", CreatedAt: budgetNow(), ScopesJSON: "{}", BudgetsJSON: "{}", RateLimitsJSON: "{}", MetadataJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	now := budgetNow()
	limiter := NewRateLimiter(db, func() time.Time { return now })
	for range 60 {
		if err := limiter.Allow(ctx, "key", 60, 0, 0); err != nil {
			t.Fatalf("Allow() unexpected error = %v", err)
		}
	}
	if err := limiter.Allow(ctx, "key", 60, 0, 0); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("Allow(61st) error = %v, want ErrRateLimited", err)
	}
	now = now.Add(time.Minute)
	if err := limiter.Allow(ctx, "key", 60, 0, 0); err != nil {
		t.Fatalf("Allow(next minute) error = %v", err)
	}
}

func TestRateLimiterPrune(t *testing.T) {
	ctx := context.Background()
	db := budgetTestDB(t)
	if err := repos.NewBridgeKeys(db).Put(ctx, repos.BridgeKey{ID: "key", Hash: "hash", Name: "key", CreatedAt: budgetNow(), ScopesJSON: "{}", BudgetsJSON: "{}", RateLimitsJSON: "{}", MetadataJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	repo := repos.NewRateLimitBuckets(db)
	if err := repo.Increment(ctx, "key", "rpm", 1, 1); err != nil {
		t.Fatal(err)
	}
	activeBucket := budgetNow().Unix() / 60
	if err := repo.Increment(ctx, "key", "rpm", activeBucket, 1); err != nil {
		t.Fatal(err)
	}
	limiter := NewRateLimiter(db, budgetNow)
	if err := limiter.Prune(ctx); err != nil {
		t.Fatalf("Prune() error = %v", err)
	}
	list, err := repo.List(ctx, "key")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Bucket != activeBucket {
		t.Fatalf("buckets after prune = %#v", list)
	}
}

func TestRateLimiterTPMAndDefaultClock(t *testing.T) {
	ctx := context.Background()
	db := budgetTestDB(t)
	if err := repos.NewBridgeKeys(db).Put(ctx, repos.BridgeKey{ID: "key", Hash: "hash", Name: "key", CreatedAt: budgetNow(), ScopesJSON: "{}", BudgetsJSON: "{}", RateLimitsJSON: "{}", MetadataJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	limiter := NewRateLimiter(db, nil)
	if err := limiter.Allow(ctx, "key", 0, 100, 60); err != nil {
		t.Fatalf("Allow(first tokens) error = %v", err)
	}
	err := limiter.Allow(ctx, "key", 0, 100, 50)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("Allow(over tpm) error = %v, want ErrRateLimited", err)
	}
	rateErr, ok := err.(RateLimitError)
	if !ok || rateErr.Metric != "tpm" || rateErr.Error() == "" {
		t.Fatalf("rate error = %#v", err)
	}
}
