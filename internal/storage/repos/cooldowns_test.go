package repos

import (
	"context"
	"testing"
)

func TestCooldownsCRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewCooldowns(newTestDB(t))
	cooldown := Cooldown{UpstreamID: "upstream", PoolName: "pool", State: "healthy", UpdatedAt: testTime()}
	if err := repo.Put(ctx, cooldown); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	got, err := repo.Get(ctx, cooldown.UpstreamID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.State != "healthy" {
		t.Fatalf("State = %q, want healthy", got.State)
	}
	if err := repo.Delete(ctx, cooldown.UpstreamID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}
