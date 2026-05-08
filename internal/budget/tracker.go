package budget

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/auth"
	"github.com/sigilbridge/sigilbridge/internal/storage/repos"
)

var ErrBudgetExceeded = errors.New("budget exceeded")

type Tracker struct {
	repo *repos.BudgetCounters
	now  func() time.Time
}

func NewTracker(db *sql.DB, now func() time.Time) *Tracker {
	if now == nil {
		now = time.Now
	}
	return &Tracker{repo: repos.NewBudgetCounters(db), now: now}
}

func (t *Tracker) PreCheck(ctx context.Context, key auth.BridgeKey, estimatedCents int64) (bool, error) {
	now := t.now().UTC()
	dailyUsed, err := t.used(ctx, key.ID, "daily", now.Format("2006-01-02"))
	if err != nil {
		return false, err
	}
	monthlyUsed, err := t.used(ctx, key.ID, "monthly", now.Format("2006-01"))
	if err != nil {
		return false, err
	}
	overDaily := key.Budgets.DailyCents > 0 && dailyUsed+estimatedCents > key.Budgets.DailyCents
	overMonthly := key.Budgets.MonthlyCents > 0 && monthlyUsed+estimatedCents > key.Budgets.MonthlyCents
	if (overDaily || overMonthly) && key.Budgets.HardCap {
		return false, fmt.Errorf("%w: estimated %d cents exceeds configured hard cap", ErrBudgetExceeded, estimatedCents)
	}
	return overDaily || overMonthly, nil
}

func (t *Tracker) Commit(ctx context.Context, keyID string, actualCents int64) error {
	now := t.now().UTC()
	if err := t.repo.Increment(ctx, keyID, "daily", now.Format("2006-01-02"), actualCents); err != nil {
		return err
	}
	return t.repo.Increment(ctx, keyID, "monthly", now.Format("2006-01"), actualCents)
}

func (t *Tracker) used(ctx context.Context, keyID, period, bucket string) (int64, error) {
	counter, err := t.repo.Get(ctx, keyID, period, bucket)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return counter.Cents, nil
}
