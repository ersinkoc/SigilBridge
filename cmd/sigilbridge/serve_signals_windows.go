//go:build windows

package main

import (
	"context"
	"os"
	"os/signal"
)

func serveSignals(parent context.Context) (context.Context, func(), <-chan struct{}) {
	ctx, stop := signal.NotifyContext(parent, os.Interrupt)
	return ctx, stop, nil
}
