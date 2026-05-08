package vault

import (
	"bytes"
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/storage"
	_ "modernc.org/sqlite"
)

func newVaultTestDB(t *testing.T) *sql.DB {
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

func TestVaultRoundTrip(t *testing.T) {
	ctx := context.Background()
	key := bytes.Repeat([]byte{7}, MasterKeySize)
	v, err := New(newVaultTestDB(t), key)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer v.Close()
	v.now = func() time.Time { return time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC) }

	id := "vault://claude_web/personal"
	metadata := map[string]string{"account_email_hash": "sha256:abc"}
	if err := v.Put(ctx, id, []byte("credential bundle"), metadata); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	plaintext, gotMetadata, err := v.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(plaintext) != "credential bundle" {
		t.Fatalf("plaintext = %q", plaintext)
	}
	if gotMetadata["account_email_hash"] != metadata["account_email_hash"] {
		t.Fatalf("metadata = %#v", gotMetadata)
	}
}

func TestVaultListPrefix(t *testing.T) {
	ctx := context.Background()
	v, err := New(newVaultTestDB(t), bytes.Repeat([]byte{1}, MasterKeySize))
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()

	for _, id := range []string{"vault://claude_web/a", "vault://claude_web/b", "vault://chatgpt_web/a"} {
		if err := v.Put(ctx, id, []byte(id), nil); err != nil {
			t.Fatalf("Put(%q) error = %v", id, err)
		}
	}
	got, err := v.List(ctx, "vault://claude_web/")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List() len = %d, want 2: %#v", len(got), got)
	}
}

func TestVaultDelete(t *testing.T) {
	ctx := context.Background()
	v, err := New(newVaultTestDB(t), bytes.Repeat([]byte{1}, MasterKeySize))
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()

	id := "vault://claude_web/delete-me"
	if err := v.Put(ctx, id, []byte("secret"), nil); err != nil {
		t.Fatal(err)
	}
	if err := v.Delete(ctx, id); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := v.Get(ctx, id); err == nil {
		t.Fatal("Get() error = nil after delete")
	}
}

func TestVaultRejectsBadID(t *testing.T) {
	v, err := New(newVaultTestDB(t), bytes.Repeat([]byte{1}, MasterKeySize))
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	if err := v.Put(context.Background(), "claude_web/bad", []byte("secret"), nil); err == nil {
		t.Fatal("Put() error = nil, want bad id error")
	}
}

func TestVaultAADPreventsSwap(t *testing.T) {
	ctx := context.Background()
	db := newVaultTestDB(t)
	v, err := New(db, bytes.Repeat([]byte{2}, MasterKeySize))
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()

	first := "vault://claude_web/first"
	second := "vault://claude_web/second"
	if err := v.Put(ctx, first, []byte("first"), nil); err != nil {
		t.Fatal(err)
	}
	if err := v.Put(ctx, second, []byte("second"), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
UPDATE sessions
SET nonce = (SELECT nonce FROM sessions WHERE id = ?),
    ciphertext = (SELECT ciphertext FROM sessions WHERE id = ?)
WHERE id = ?`, first, first, second); err != nil {
		t.Fatal(err)
	}
	if _, _, err := v.Get(ctx, second); err == nil {
		t.Fatal("Get() error = nil after ciphertext swap, want AAD failure")
	}
}
