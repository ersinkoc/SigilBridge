package auth

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/sigilbridge/sigilbridge/internal/storage"
	"github.com/sigilbridge/sigilbridge/internal/storage/repos"
)

func TestGenerateAndHash(t *testing.T) {
	token, hash, err := Generate(PrefixLive)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if err := ValidateFormat(token); err != nil {
		t.Fatalf("generated token invalid: %v", err)
	}
	if hash != Hash(token) {
		t.Fatalf("hash mismatch")
	}
	if _, _, err := Generate("dev"); err == nil {
		t.Fatal("Generate() error = nil for bad prefix")
	}
	if err := ValidateFormat("sb_dev_0123456789abcdef0123456789abcdef"); err == nil {
		t.Fatal("ValidateFormat() error = nil for bad prefix")
	}
}

func TestBridgeKeyValidateAndScope(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := storage.Up(db); err != nil {
		t.Fatal(err)
	}
	token, hash, err := Generate(PrefixLive)
	if err != nil {
		t.Fatal(err)
	}
	scopes, _ := json.Marshal(Scopes{AllowedPools: []string{"sonnet"}, AllowedModels: []string{"claude-sonnet"}, IPAllowlist: []string{"10.0.0.0/8"}})
	if err := repos.NewBridgeKeys(db).Put(ctx, repos.BridgeKey{
		ID:             "key",
		Hash:           hash,
		Name:           "key",
		CreatedAt:      testTime(),
		ScopesJSON:     string(scopes),
		BudgetsJSON:    "{}",
		RateLimitsJSON: "{}",
		MetadataJSON:   "{}",
	}); err != nil {
		t.Fatal(err)
	}
	store := NewBridgeKeyStore(db, NewCache(1000, fiveMinutes, nil))
	key, err := store.Validate(ctx, token)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if err := CheckScope(key, "sonnet", "claude-sonnet", "10.1.2.3"); err != nil {
		t.Fatalf("CheckScope() error = %v", err)
	}
	if err := CheckScope(key, "haiku", "claude-sonnet", "10.1.2.3"); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("CheckScope(pool) error = %v, want ErrScopeDenied", err)
	}
}

func TestBridgeKeyValidateRejectsRevoked(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := storage.Up(db); err != nil {
		t.Fatal(err)
	}
	token, hash, err := Generate(PrefixLive)
	if err != nil {
		t.Fatal(err)
	}
	if err := repos.NewBridgeKeys(db).Put(ctx, repos.BridgeKey{
		ID:             "key",
		Hash:           hash,
		Name:           "key",
		CreatedAt:      testTime(),
		RevokedAt:      testTime(),
		ScopesJSON:     "{}",
		BudgetsJSON:    "{}",
		RateLimitsJSON: "{}",
		MetadataJSON:   "{}",
	}); err != nil {
		t.Fatal(err)
	}
	_, err = NewBridgeKeyStore(db, nil).Validate(ctx, token)
	if !errors.Is(err, ErrBridgeKeyRevoked) {
		t.Fatalf("Validate() error = %v, want ErrBridgeKeyRevoked", err)
	}
}
