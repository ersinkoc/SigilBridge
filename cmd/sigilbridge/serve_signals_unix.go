//go:build !windows

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func serveSignals(parent context.Context) (context.Context, func(), <-chan struct{}) {
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	hup := make(chan os.Signal, 1)
	reload := make(chan struct{}, 1)
	signal.Notify(hup, syscall.SIGHUP)
	go func() {
		defer close(reload)
		for {
			select {
			case <-ctx.Done():
				signal.Stop(hup)
				return
			case <-hup:
				select {
				case reload <- struct{}{}:
				default:
				}
			}
		}
	}()
	cleanup := func() {
		signal.Stop(hup)
		stop()
	}
	return ctx, cleanup, reload
}
