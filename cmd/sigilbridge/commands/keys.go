package commands

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/sigilbridge/sigilbridge/internal/auth"
	"github.com/sigilbridge/sigilbridge/internal/config"
	"github.com/sigilbridge/sigilbridge/internal/storage"
	"github.com/sigilbridge/sigilbridge/internal/storage/repos"
)

func KeysCreate(prefix string) (plaintext, hash string, err error) {
	return auth.Generate(prefix)
}

func KeysCreateStored(ctx context.Context, configPath, prefix, name string) (plaintext, id, hash string, err error) {
	if name == "" {
		name = "cli-created"
	}
	db, err := openConfiguredDB(configPath)
	if err != nil {
		return "", "", "", err
	}
	defer db.Close()
	plaintext, hash, err = auth.Generate(prefix)
	if err != nil {
		return "", "", "", err
	}
	id = ulid.Make().String()
	now := time.Now().UTC()
	key := repos.BridgeKey{
		ID:             id,
		Hash:           hash,
		Name:           name,
		CreatedAt:      now,
		CreatedBy:      "cli",
		ScopesJSON:     "{}",
		BudgetsJSON:    "{}",
		RateLimitsJSON: "{}",
		MetadataJSON:   "{}",
	}
	if err := repos.NewBridgeKeys(db).Put(ctx, key); err != nil {
		return "", "", "", err
	}
	return plaintext, id, hash, nil
}

func KeysListStored(ctx context.Context, configPath string) ([]repos.BridgeKey, error) {
	db, err := openConfiguredDB(configPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return repos.NewBridgeKeys(db).List(ctx)
}

func KeysRevokeStored(ctx context.Context, configPath, id string) error {
	if id == "" {
		return fmt.Errorf("bridge key id is required")
	}
	db, err := openConfiguredDB(configPath)
	if err != nil {
		return err
	}
	defer db.Close()
	repo := repos.NewBridgeKeys(db)
	key, err := repo.Get(ctx, id)
	if err != nil {
		return err
	}
	key.RevokedAt = time.Now().UTC()
	return repo.Put(ctx, key)
}

func openConfiguredDB(configPath string) (*sql.DB, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	dbPath := config.ResolveRelative(configPath, cfg.Storage.Path)
	if err := ensureParentDir(dbPath); err != nil {
		return nil, err
	}
	db, err := storage.OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	if err := storage.Up(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func ensureParentDir(path string) error {
	if path == "" || path == ":memory:" {
		return nil
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create database directory %q: %w", dir, err)
	}
	return nil
}
