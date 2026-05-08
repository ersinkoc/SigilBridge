package commands

import (
	"context"

	"github.com/sigilbridge/sigilbridge/internal/config"
	"github.com/sigilbridge/sigilbridge/internal/storage"
)

func Restore(srcPath, dstPath string) error {
	return storage.Restore(srcPath, dstPath)
}

func RestoreConfig(_ context.Context, configPath, srcPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	dstPath := config.ResolveRelative(configPath, cfg.Storage.Path)
	return storage.Restore(srcPath, dstPath)
}
