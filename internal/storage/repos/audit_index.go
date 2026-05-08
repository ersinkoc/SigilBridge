package repos

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

type AuditEntry struct {
	RequestID   string
	TS          time.Time
	BridgeKeyID string
	PoolName    string
	UpstreamID  string
	Status      string
	CostCents   int64
	FilePath    string
	FileOffset  int64
	FileLength  int64
}

type AuditIndex struct {
	db *sql.DB
}

type AuditQuery struct {
	From        time.Time
	To          time.Time
	RequestID   string
	BridgeKeyID string
	PoolName    string
	UpstreamID  string
	Status      string
	Limit       int
	Cursor      string
}

type AuditPage struct {
	Entries    []AuditEntry
	NextCursor string
}

func NewAuditIndex(db *sql.DB) *AuditIndex {
	return &AuditIndex{db: db}
}

func (r *AuditIndex) Put(ctx context.Context, entry AuditEntry) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO audit_index (request_id, ts, bridge_key_id, pool_name, upstream_id, status, cost_cents, file_path, file_offset, file_length)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(request_id) DO UPDATE SET
  ts = excluded.ts,
  bridge_key_id = excluded.bridge_key_id,
  pool_name = excluded.pool_name,
  upstream_id = excluded.upstream_id,
  status = excluded.status,
  cost_cents = excluded.cost_cents,
  file_path = excluded.file_path,
  file_offset = excluded.file_offset,
  file_length = excluded.file_length`,
		entry.RequestID, formatTime(entry.TS), nullString(entry.BridgeKeyID), nullString(entry.PoolName),
		nullString(entry.UpstreamID), entry.Status, entry.CostCents, entry.FilePath, entry.FileOffset, entry.FileLength)
	return err
}

func (r *AuditIndex) Get(ctx context.Context, requestID string) (AuditEntry, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT request_id, ts, bridge_key_id, pool_name, upstream_id, status, cost_cents, file_path, file_offset, file_length
FROM audit_index WHERE request_id = ?`, requestID)
	return scanAuditEntry(row)
}

func (r *AuditIndex) List(ctx context.Context, limit int) ([]AuditEntry, error) {
	page, err := r.Query(ctx, AuditQuery{Limit: limit})
	return page.Entries, err
}

func (r *AuditIndex) Query(ctx context.Context, query AuditQuery) (AuditPage, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	var where []string
	var args []any
	if !query.From.IsZero() {
		where = append(where, "ts >= ?")
		args = append(args, formatTime(query.From))
	}
	if !query.To.IsZero() {
		where = append(where, "ts <= ?")
		args = append(args, formatTime(query.To))
	}
	if query.RequestID != "" {
		where = append(where, "request_id = ?")
		args = append(args, query.RequestID)
	}
	if query.BridgeKeyID != "" {
		where = append(where, "bridge_key_id = ?")
		args = append(args, query.BridgeKeyID)
	}
	if query.PoolName != "" {
		where = append(where, "pool_name = ?")
		args = append(args, query.PoolName)
	}
	if query.UpstreamID != "" {
		where = append(where, "upstream_id = ?")
		args = append(args, query.UpstreamID)
	}
	if query.Status != "" {
		where = append(where, "status = ?")
		args = append(args, query.Status)
	}
	if query.Cursor != "" {
		cursorTS, cursorID, err := decodeAuditCursor(query.Cursor)
		if err != nil {
			return AuditPage{}, err
		}
		where = append(where, "(ts < ? OR (ts = ? AND request_id < ?))")
		formatted := formatTime(cursorTS)
		args = append(args, formatted, formatted, cursorID)
	}
	var sql strings.Builder
	sql.WriteString(`
SELECT request_id, ts, bridge_key_id, pool_name, upstream_id, status, cost_cents, file_path, file_offset, file_length
FROM audit_index`)
	if len(where) > 0 {
		sql.WriteString(" WHERE ")
		sql.WriteString(strings.Join(where, " AND "))
	}
	sql.WriteString(" ORDER BY ts DESC, request_id DESC LIMIT ?")
	args = append(args, limit+1)
	rows, err := r.db.QueryContext(ctx, sql.String(), args...)
	if err != nil {
		return AuditPage{}, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		entry, err := scanAuditEntry(rows)
		if err != nil {
			return AuditPage{}, err
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return AuditPage{}, err
	}
	nextCursor := ""
	if len(out) > limit {
		out = out[:limit]
		nextCursor = encodeAuditCursor(out[len(out)-1])
	}
	return AuditPage{Entries: out, NextCursor: nextCursor}, nil
}

func (r *AuditIndex) Delete(ctx context.Context, requestID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM audit_index WHERE request_id = ?", requestID)
	return err
}

func scanAuditEntry(scanner interface {
	Scan(dest ...any) error
}) (AuditEntry, error) {
	var entry AuditEntry
	var ts string
	var bridgeKeyID, poolName, upstreamID sql.NullString
	if err := scanner.Scan(&entry.RequestID, &ts, &bridgeKeyID, &poolName, &upstreamID, &entry.Status, &entry.CostCents, &entry.FilePath, &entry.FileOffset, &entry.FileLength); err != nil {
		return AuditEntry{}, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return AuditEntry{}, err
	}
	entry.TS = parsed
	entry.BridgeKeyID = scanNullString(bridgeKeyID)
	entry.PoolName = scanNullString(poolName)
	entry.UpstreamID = scanNullString(upstreamID)
	return entry, nil
}

func encodeAuditCursor(entry AuditEntry) string {
	raw := formatTime(entry.TS) + "\n" + entry.RequestID
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeAuditCursor(cursor string) (time.Time, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid audit cursor: %w", err)
	}
	parts := strings.SplitN(string(raw), "\n", 2)
	if len(parts) != 2 || parts[1] == "" {
		return time.Time{}, "", fmt.Errorf("invalid audit cursor")
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid audit cursor time: %w", err)
	}
	return ts, parts[1], nil
}
