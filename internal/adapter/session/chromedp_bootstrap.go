package session

import (
	"context"
	"fmt"
	"os/exec"
)

type ChromeBootstrapConfig struct {
	ChromePath string
	ProfileDir string
	LoginURL   string
}

func ChromeArgs(cfg ChromeBootstrapConfig) []string {
	args := []string{"--new-window"}
	if cfg.ProfileDir != "" {
		args = append(args, "--user-data-dir="+cfg.ProfileDir)
	}
	if cfg.LoginURL != "" {
		args = append(args, cfg.LoginURL)
	}
	return args
}

func StartChromeBootstrap(ctx context.Context, cfg ChromeBootstrapConfig) (*exec.Cmd, error) {
	if cfg.ChromePath == "" {
		return nil, fmt.Errorf("chrome path is required")
	}
	// #nosec G204 -- ChromePath is an explicit local admin/browser bootstrap setting, not remote input.
	cmd := exec.CommandContext(ctx, cfg.ChromePath, ChromeArgs(cfg)...)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}
