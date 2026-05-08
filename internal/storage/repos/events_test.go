package repos

import (
	"context"
	"testing"
)

func TestEventsCRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewEvents(newTestDB(t))
	event := Event{ID: "01HXEVT", TS: testTime(), Type: "admin_action", Severity: "info", PayloadJSON: "{}"}
	if err := repo.Put(ctx, event); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	got, err := repo.Get(ctx, event.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Type != event.Type {
		t.Fatalf("Type = %q, want %q", got.Type, event.Type)
	}
	list, err := repo.List(ctx, 10)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List() len = %d, want 1", len(list))
	}
	if err := repo.Delete(ctx, event.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}
