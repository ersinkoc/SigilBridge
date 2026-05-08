package repos

import (
	"bytes"
	"context"
	"testing"
)

func TestSessionsCRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewSessions(newTestDB(t))
	session := Session{
		ID:           "vault://claude/test",
		Provider:     "claude_web",
		CreatedAt:    testTime(),
		Nonce:        []byte("123456789012"),
		Ciphertext:   []byte("ciphertext"),
		MetadataJSON: "{}",
	}
	if err := repo.Put(ctx, session); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	got, err := repo.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !bytes.Equal(got.Ciphertext, session.Ciphertext) {
		t.Fatalf("ciphertext mismatch")
	}
	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List() len = %d, want 1", len(list))
	}
	if err := repo.Delete(ctx, session.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}
