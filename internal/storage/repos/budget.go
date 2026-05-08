package repos

import (
	"context"
	"database/sql"
	"fmt"
)

type BudgetCounter struct {
	KeyID  string
	Period string
	Bucket string
	Cents  int64
}

type BudgetCounters struct {
	db *sql.DB
}

func NewBudgetCounters(db *sql.DB) *BudgetCounters {
	return &BudgetCounters{db: db}
}

func (r *BudgetCounters) Put(ctx context.Context, counter BudgetCounter) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO budget_counters (key_id, period, bucket, cents)
VALUES (?, ?, ?, ?)
ON CONFLICT(key_id, period, bucket) DO UPDATE SET cents = excluded.cents`,
		counter.KeyID, counter.Period, counter.Bucket, counter.Cents)
	return err
}

func (r *BudgetCounters) Increment(ctx context.Context, keyID, period, bucket string, delta int64) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO budget_counters (key_id, period, bucket, cents)
VALUES (?, ?, ?, ?)
ON CONFLICT(key_id, period, bucket)
DO UPDATE SET cents = cents + excluded.cents`, keyID, period, bucket, delta)
	return err
}

func (r *BudgetCounters) Get(ctx context.Context, keyID, period, bucket string) (BudgetCounter, error) {
	var counter BudgetCounter
	err := r.db.QueryRowContext(ctx, `
SELECT key_id, period, bucket, cents FROM budget_counters
WHERE key_id = ? AND period = ? AND bucket = ?`, keyID, period, bucket).
		Scan(&counter.KeyID, &counter.Period, &counter.Bucket, &counter.Cents)
	return counter, err
}

func (r *BudgetCounters) List(ctx context.Context, keyID string) ([]BudgetCounter, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT key_id, period, bucket, cents FROM budget_counters
WHERE key_id = ? ORDER BY period, bucket`, keyID)
	if err != nil {
		return nil, fmt.Errorf("list budget counters: %w", err)
	}
	defer rows.Close()
	var out []BudgetCounter
	for rows.Next() {
		var counter BudgetCounter
		if err := rows.Scan(&counter.KeyID, &counter.Period, &counter.Bucket, &counter.Cents); err != nil {
			return nil, err
		}
		out = append(out, counter)
	}
	return out, rows.Err()
}

func (r *BudgetCounters) Delete(ctx context.Context, keyID, period, bucket string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM budget_counters WHERE key_id = ? AND period = ? AND bucket = ?", keyID, period, bucket)
	return err
}
