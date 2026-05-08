package budget

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/storage/repos"
)

var ErrRateLimited = errors.New("rate limited")

type RateLimitError struct {
	Metric     string
	RetryAfter time.Duration
}

func (e RateLimitError) Error() string {
	return fmt.Sprintf("%s: %s retry after %s", ErrRateLimited, e.Metric, e.RetryAfter)
}

func (e RateLimitError) Unwrap() error {
	return ErrRateLimited
}

type RateLimiter struct {
	db   *sql.DB
	repo *repos.RateLimitBuckets
	now  func() time.Time
}

func NewRateLimiter(db *sql.DB, now func() time.Time) *RateLimiter {
	if now == nil {
		now = time.Now
	}
	return &RateLimiter{db: db, repo: repos.NewRateLimitBuckets(db), now: now}
}

func (l *RateLimiter) Allow(ctx context.Context, keyID string, rpmLimit, tpmLimit, tokens int64) error {
	now := l.now().UTC()
	bucket := now.Unix() / 60
	if rpmLimit > 0 {
		total, err := l.incrementAndTotal(ctx, keyID, "rpm", bucket, 1)
		if err != nil {
			return err
		}
		if total > rpmLimit {
			return RateLimitError{Metric: "rpm", RetryAfter: retryAfter(now)}
		}
	}
	if tpmLimit > 0 {
		total, err := l.incrementAndTotal(ctx, keyID, "tpm", bucket, tokens)
		if err != nil {
			return err
		}
		if total > tpmLimit {
			return RateLimitError{Metric: "tpm", RetryAfter: retryAfter(now)}
		}
	}
	return nil
}

func (l *RateLimiter) Prune(ctx context.Context) error {
	bucket := l.now().UTC().Unix()/60 - 10
	return l.repo.PruneBefore(ctx, bucket)
}

func (l *RateLimiter) incrementAndTotal(ctx context.Context, keyID, metric string, bucket, delta int64) (int64, error) {
	if _, err := l.db.ExecContext(ctx, `
INSERT INTO ratelimit_buckets (key_id, metric, bucket, value)
VALUES (?, ?, ?, ?)
ON CONFLICT(key_id, metric, bucket)
DO UPDATE SET value = value + excluded.value`, keyID, metric, bucket, delta); err != nil {
		return 0, err
	}
	var total int64
	if err := l.db.QueryRowContext(ctx, `
SELECT COALESCE(SUM(value), 0)
FROM ratelimit_buckets
WHERE key_id = ? AND metric = ? AND bucket >= ?`, keyID, metric, bucket).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func retryAfter(now time.Time) time.Duration {
	seconds := 60 - (now.Unix() % 60)
	if seconds <= 0 {
		seconds = 60
	}
	return time.Duration(seconds) * time.Second
}
