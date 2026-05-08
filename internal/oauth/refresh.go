package oauth

import (
	"context"
	"time"
)

type RefreshWorker struct {
	manager     *Manager
	interval    time.Duration
	refreshSkew time.Duration
	onEvent     func(RefreshEvent)
}

type RefreshEvent struct {
	VaultID string
	Success bool
	Error   error
}

func NewRefreshWorker(manager *Manager, interval, refreshSkew time.Duration, onEvent func(RefreshEvent)) *RefreshWorker {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	if refreshSkew <= 0 {
		refreshSkew = 10 * time.Minute
	}
	return &RefreshWorker{manager: manager, interval: interval, refreshSkew: refreshSkew, onEvent: onEvent}
}

func (w *RefreshWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		w.RefreshDue(ctx)
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
}

func (w *RefreshWorker) RefreshDue(ctx context.Context) {
	ids, err := w.manager.vault.List(ctx, vaultPrefix)
	if err != nil {
		w.emit(RefreshEvent{Error: err})
		return
	}
	now := w.manager.now()
	for _, id := range ids {
		token, _, err := w.manager.loadToken(ctx, id)
		if err != nil {
			w.emit(RefreshEvent{VaultID: id, Error: err})
			continue
		}
		if token.ExpiresAt.IsZero() || token.ExpiresAt.After(now.Add(w.refreshSkew)) {
			continue
		}
		_, err = w.manager.Refresh(ctx, id)
		w.emit(RefreshEvent{VaultID: id, Success: err == nil, Error: err})
	}
}

func (w *RefreshWorker) emit(event RefreshEvent) {
	if w.onEvent != nil {
		w.onEvent(event)
	}
}
