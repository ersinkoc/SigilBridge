package repos

import (
	"context"
	"database/sql"
	"time"
)

type Event struct {
	ID          string
	TS          time.Time
	Type        string
	Severity    string
	PayloadJSON string
	Actor       string
}

type Events struct {
	db *sql.DB
}

func NewEvents(db *sql.DB) *Events {
	return &Events{db: db}
}

func (r *Events) Put(ctx context.Context, event Event) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO events (id, ts, type, severity, payload_json, actor)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  ts = excluded.ts,
  type = excluded.type,
  severity = excluded.severity,
  payload_json = excluded.payload_json,
  actor = excluded.actor`,
		event.ID, formatTime(event.TS), event.Type, event.Severity, event.PayloadJSON, nullString(event.Actor))
	return err
}

func (r *Events) Get(ctx context.Context, id string) (Event, error) {
	row := r.db.QueryRowContext(ctx, "SELECT id, ts, type, severity, payload_json, actor FROM events WHERE id = ?", id)
	return scanEvent(row)
}

func (r *Events) List(ctx context.Context, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, "SELECT id, ts, type, severity, payload_json, actor FROM events ORDER BY ts DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, rows.Err()
}

func (r *Events) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM events WHERE id = ?", id)
	return err
}

func scanEvent(scanner interface {
	Scan(dest ...any) error
}) (Event, error) {
	var event Event
	var ts string
	var actor sql.NullString
	if err := scanner.Scan(&event.ID, &ts, &event.Type, &event.Severity, &event.PayloadJSON, &actor); err != nil {
		return Event{}, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return Event{}, err
	}
	event.TS = parsed
	event.Actor = scanNullString(actor)
	return event, nil
}
