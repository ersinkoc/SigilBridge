package repos

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Session struct {
	ID              string
	Provider        string
	CreatedAt       time.Time
	LastRefreshedAt time.Time
	ExpiresAt       time.Time
	Nonce           []byte
	Ciphertext      []byte
	MetadataJSON    string
}

type Sessions struct {
	db *sql.DB
}

func NewSessions(db *sql.DB) *Sessions {
	return &Sessions{db: db}
}

func (r *Sessions) Put(ctx context.Context, session Session) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO sessions (id, provider, created_at, last_refreshed_at, expires_at, nonce, ciphertext, metadata_json)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  provider = excluded.provider,
  last_refreshed_at = excluded.last_refreshed_at,
  expires_at = excluded.expires_at,
  nonce = excluded.nonce,
  ciphertext = excluded.ciphertext,
  metadata_json = excluded.metadata_json`,
		session.ID, session.Provider, formatTime(session.CreatedAt), nullTime(session.LastRefreshedAt),
		nullTime(session.ExpiresAt), session.Nonce, session.Ciphertext, session.MetadataJSON)
	if err != nil {
		return fmt.Errorf("put session %q: %w", session.ID, err)
	}
	return nil
}

func (r *Sessions) Get(ctx context.Context, id string) (Session, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, provider, created_at, last_refreshed_at, expires_at, nonce, ciphertext, metadata_json
FROM sessions WHERE id = ?`, id)
	return scanSession(row)
}

func (r *Sessions) List(ctx context.Context) ([]Session, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, provider, created_at, last_refreshed_at, expires_at, nonce, ciphertext, metadata_json
FROM sessions ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, session)
	}
	return out, rows.Err()
}

func (r *Sessions) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = ?", id)
	return err
}

func scanSession(scanner interface {
	Scan(dest ...any) error
}) (Session, error) {
	var session Session
	var createdAt string
	var lastRefreshedAt, expiresAt sql.NullString
	if err := scanner.Scan(&session.ID, &session.Provider, &createdAt, &lastRefreshedAt, &expiresAt, &session.Nonce, &session.Ciphertext, &session.MetadataJSON); err != nil {
		return Session{}, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return Session{}, err
	}
	session.CreatedAt = parsed
	session.LastRefreshedAt, err = scanNullTime(lastRefreshedAt)
	if err != nil {
		return Session{}, err
	}
	session.ExpiresAt, err = scanNullTime(expiresAt)
	if err != nil {
		return Session{}, err
	}
	return session, nil
}
