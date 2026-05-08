package repos

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type BridgeKey struct {
	ID             string
	Hash           string
	Name           string
	CreatedAt      time.Time
	CreatedBy      string
	LastUsedAt     time.Time
	RevokedAt      time.Time
	ScopesJSON     string
	BudgetsJSON    string
	RateLimitsJSON string
	MetadataJSON   string
}

type BridgeKeys struct {
	db *sql.DB
}

func NewBridgeKeys(db *sql.DB) *BridgeKeys {
	return &BridgeKeys{db: db}
}

func (r *BridgeKeys) Put(ctx context.Context, key BridgeKey) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO bridge_keys (
  id, hash, name, created_at, created_by, last_used_at, revoked_at,
  scopes_json, budgets_json, rate_limits_json, metadata_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  hash = excluded.hash,
  name = excluded.name,
  created_by = excluded.created_by,
  last_used_at = excluded.last_used_at,
  revoked_at = excluded.revoked_at,
  scopes_json = excluded.scopes_json,
  budgets_json = excluded.budgets_json,
  rate_limits_json = excluded.rate_limits_json,
  metadata_json = excluded.metadata_json`,
		key.ID, key.Hash, key.Name, formatTime(key.CreatedAt), nullString(key.CreatedBy),
		nullTime(key.LastUsedAt), nullTime(key.RevokedAt), key.ScopesJSON, key.BudgetsJSON,
		key.RateLimitsJSON, key.MetadataJSON)
	if err != nil {
		return fmt.Errorf("put bridge key %q: %w", key.ID, err)
	}
	return nil
}

func (r *BridgeKeys) Get(ctx context.Context, id string) (BridgeKey, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, hash, name, created_at, created_by, last_used_at, revoked_at,
       scopes_json, budgets_json, rate_limits_json, metadata_json
FROM bridge_keys WHERE id = ?`, id)
	return scanBridgeKey(row)
}

func (r *BridgeKeys) GetByHash(ctx context.Context, hash string) (BridgeKey, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, hash, name, created_at, created_by, last_used_at, revoked_at,
       scopes_json, budgets_json, rate_limits_json, metadata_json
FROM bridge_keys WHERE hash = ? AND revoked_at IS NULL`, hash)
	return scanBridgeKey(row)
}

func (r *BridgeKeys) List(ctx context.Context) ([]BridgeKey, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, hash, name, created_at, created_by, last_used_at, revoked_at,
       scopes_json, budgets_json, rate_limits_json, metadata_json
FROM bridge_keys ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("list bridge keys: %w", err)
	}
	defer rows.Close()

	var keys []BridgeKey
	for rows.Next() {
		key, err := scanBridgeKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (r *BridgeKeys) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM bridge_keys WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete bridge key %q: %w", id, err)
	}
	return nil
}

func (r *BridgeKeys) MarkUsed(ctx context.Context, id string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, "UPDATE bridge_keys SET last_used_at = ? WHERE id = ?", formatTime(at), id)
	return err
}

func scanBridgeKey(scanner interface {
	Scan(dest ...any) error
}) (BridgeKey, error) {
	var key BridgeKey
	var createdAt string
	var createdBy, lastUsedAt, revokedAt sql.NullString
	if err := scanner.Scan(&key.ID, &key.Hash, &key.Name, &createdAt, &createdBy, &lastUsedAt, &revokedAt, &key.ScopesJSON, &key.BudgetsJSON, &key.RateLimitsJSON, &key.MetadataJSON); err != nil {
		return BridgeKey{}, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return BridgeKey{}, err
	}
	key.CreatedAt = parsed
	key.CreatedBy = scanNullString(createdBy)
	key.LastUsedAt, err = scanNullTime(lastUsedAt)
	if err != nil {
		return BridgeKey{}, err
	}
	key.RevokedAt, err = scanNullTime(revokedAt)
	if err != nil {
		return BridgeKey{}, err
	}
	return key, nil
}
