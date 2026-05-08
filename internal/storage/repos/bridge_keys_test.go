package repos

import (
	"context"
	"testing"
)

func TestBridgeKeysCRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewBridgeKeys(newTestDB(t))
	key := BridgeKey{
		ID:             "01HXKEY",
		Hash:           "hash",
		Name:           "test",
		CreatedAt:      testTime(),
		ScopesJSON:     "{}",
		BudgetsJSON:    "{}",
		RateLimitsJSON: "{}",
		MetadataJSON:   "{}",
	}
	if err := repo.Put(ctx, key); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	got, err := repo.GetByHash(ctx, "hash")
	if err != nil {
		t.Fatalf("GetByHash() error = %v", err)
	}
	if got.ID != key.ID {
		t.Fatalf("GetByHash().ID = %q, want %q", got.ID, key.ID)
	}
	if err := repo.MarkUsed(ctx, key.ID, testTime().Add(1)); err != nil {
		t.Fatalf("MarkUsed() error = %v", err)
	}
	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List() len = %d, want 1", len(list))
	}
	if err := repo.Delete(ctx, key.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}
