package commands

import (
	"context"
	"os/exec"

	"github.com/sigilbridge/sigilbridge/internal/adapter/session"
)

func StartSessionBootstrap(ctx context.Context, cfg session.ChromeBootstrapConfig) (*exec.Cmd, error) {
	return session.StartChromeBootstrap(ctx, cfg)
}
