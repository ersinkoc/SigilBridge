package plugin

import (
	"context"
	"os/exec"
	"sync"
	"time"
)

type Host struct {
	manifest Manifest
	cmd      *exec.Cmd
	mu       sync.Mutex
	backoff  time.Duration
	onCrash  func(Manifest)
}

func NewHost(manifest Manifest, onCrash func(Manifest)) *Host {
	return &Host{manifest: manifest, backoff: time.Second, onCrash: onCrash}
}

func (h *Host) Start(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	cmd := exec.CommandContext(ctx, h.manifest.Command, h.manifest.Args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	h.cmd = cmd
	go h.watch(ctx, cmd)
	return nil
}

func (h *Host) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cmd == nil || h.cmd.Process == nil {
		return nil
	}
	return h.cmd.Process.Kill()
}

func (h *Host) watch(ctx context.Context, cmd *exec.Cmd) {
	err := cmd.Wait()
	if err == nil || ctx.Err() != nil {
		return
	}
	if h.onCrash != nil {
		h.onCrash(h.manifest)
	}
	timer := time.NewTimer(h.backoff)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		return
	}
	if h.backoff < 5*time.Minute {
		h.backoff *= 2
		if h.backoff > 5*time.Minute {
			h.backoff = 5 * time.Minute
		}
	}
	_ = h.Start(ctx)
}
