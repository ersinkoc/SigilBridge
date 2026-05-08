package repos

import (
	"context"
	"database/sql"
	"time"
)

type Cooldown struct {
	UpstreamID          string
	PoolName            string
	State               string
	ConsecutiveFailures int64
	LastError           string
	LastErrorAt         time.Time
	LastSuccessAt       time.Time
	CooldownUntil       time.Time
	CircuitOpenUntil    time.Time
	UpdatedAt           time.Time
}

type Cooldowns struct {
	db *sql.DB
}

func NewCooldowns(db *sql.DB) *Cooldowns {
	return &Cooldowns{db: db}
}

func (r *Cooldowns) Put(ctx context.Context, cooldown Cooldown) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO cooldowns (
  upstream_id, pool_name, state, consecutive_failures, last_error, last_error_at,
  last_success_at, cooldown_until, circuit_open_until, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(upstream_id) DO UPDATE SET
  pool_name = excluded.pool_name,
  state = excluded.state,
  consecutive_failures = excluded.consecutive_failures,
  last_error = excluded.last_error,
  last_error_at = excluded.last_error_at,
  last_success_at = excluded.last_success_at,
  cooldown_until = excluded.cooldown_until,
  circuit_open_until = excluded.circuit_open_until,
  updated_at = excluded.updated_at`,
		cooldown.UpstreamID, cooldown.PoolName, cooldown.State, cooldown.ConsecutiveFailures,
		nullString(cooldown.LastError), nullTime(cooldown.LastErrorAt), nullTime(cooldown.LastSuccessAt),
		nullTime(cooldown.CooldownUntil), nullTime(cooldown.CircuitOpenUntil), formatTime(cooldown.UpdatedAt))
	return err
}

func (r *Cooldowns) Get(ctx context.Context, upstreamID string) (Cooldown, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT upstream_id, pool_name, state, consecutive_failures, last_error, last_error_at,
       last_success_at, cooldown_until, circuit_open_until, updated_at
FROM cooldowns WHERE upstream_id = ?`, upstreamID)
	return scanCooldown(row)
}

func (r *Cooldowns) List(ctx context.Context) ([]Cooldown, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT upstream_id, pool_name, state, consecutive_failures, last_error, last_error_at,
       last_success_at, cooldown_until, circuit_open_until, updated_at
FROM cooldowns ORDER BY pool_name, upstream_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Cooldown
	for rows.Next() {
		cooldown, err := scanCooldown(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, cooldown)
	}
	return out, rows.Err()
}

func (r *Cooldowns) Delete(ctx context.Context, upstreamID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM cooldowns WHERE upstream_id = ?", upstreamID)
	return err
}

func scanCooldown(scanner interface {
	Scan(dest ...any) error
}) (Cooldown, error) {
	var cooldown Cooldown
	var lastError, lastErrorAt, lastSuccessAt, cooldownUntil, circuitOpenUntil sql.NullString
	var updatedAt string
	if err := scanner.Scan(&cooldown.UpstreamID, &cooldown.PoolName, &cooldown.State, &cooldown.ConsecutiveFailures, &lastError, &lastErrorAt, &lastSuccessAt, &cooldownUntil, &circuitOpenUntil, &updatedAt); err != nil {
		return Cooldown{}, err
	}
	var err error
	cooldown.LastError = scanNullString(lastError)
	cooldown.LastErrorAt, err = scanNullTime(lastErrorAt)
	if err != nil {
		return Cooldown{}, err
	}
	cooldown.LastSuccessAt, err = scanNullTime(lastSuccessAt)
	if err != nil {
		return Cooldown{}, err
	}
	cooldown.CooldownUntil, err = scanNullTime(cooldownUntil)
	if err != nil {
		return Cooldown{}, err
	}
	cooldown.CircuitOpenUntil, err = scanNullTime(circuitOpenUntil)
	if err != nil {
		return Cooldown{}, err
	}
	cooldown.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return Cooldown{}, err
	}
	return cooldown, nil
}
