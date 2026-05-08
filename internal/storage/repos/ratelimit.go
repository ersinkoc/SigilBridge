package repos

import (
	"context"
	"database/sql"
)

type RateLimitBucket struct {
	KeyID  string
	Metric string
	Bucket int64
	Value  int64
}

type RateLimitBuckets struct {
	db *sql.DB
}

func NewRateLimitBuckets(db *sql.DB) *RateLimitBuckets {
	return &RateLimitBuckets{db: db}
}

func (r *RateLimitBuckets) Put(ctx context.Context, bucket RateLimitBucket) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO ratelimit_buckets (key_id, metric, bucket, value)
VALUES (?, ?, ?, ?)
ON CONFLICT(key_id, metric, bucket) DO UPDATE SET value = excluded.value`,
		bucket.KeyID, bucket.Metric, bucket.Bucket, bucket.Value)
	return err
}

func (r *RateLimitBuckets) Increment(ctx context.Context, keyID, metric string, bucket, delta int64) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO ratelimit_buckets (key_id, metric, bucket, value)
VALUES (?, ?, ?, ?)
ON CONFLICT(key_id, metric, bucket)
DO UPDATE SET value = value + excluded.value`, keyID, metric, bucket, delta)
	return err
}

func (r *RateLimitBuckets) Get(ctx context.Context, keyID, metric string, bucket int64) (RateLimitBucket, error) {
	var out RateLimitBucket
	err := r.db.QueryRowContext(ctx, `
SELECT key_id, metric, bucket, value FROM ratelimit_buckets
WHERE key_id = ? AND metric = ? AND bucket = ?`, keyID, metric, bucket).
		Scan(&out.KeyID, &out.Metric, &out.Bucket, &out.Value)
	return out, err
}

func (r *RateLimitBuckets) List(ctx context.Context, keyID string) ([]RateLimitBucket, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT key_id, metric, bucket, value FROM ratelimit_buckets
WHERE key_id = ? ORDER BY bucket, metric`, keyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RateLimitBucket
	for rows.Next() {
		var bucket RateLimitBucket
		if err := rows.Scan(&bucket.KeyID, &bucket.Metric, &bucket.Bucket, &bucket.Value); err != nil {
			return nil, err
		}
		out = append(out, bucket)
	}
	return out, rows.Err()
}

func (r *RateLimitBuckets) Delete(ctx context.Context, keyID, metric string, bucket int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM ratelimit_buckets WHERE key_id = ? AND metric = ? AND bucket = ?", keyID, metric, bucket)
	return err
}

func (r *RateLimitBuckets) PruneBefore(ctx context.Context, bucket int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM ratelimit_buckets WHERE bucket < ?", bucket)
	return err
}
