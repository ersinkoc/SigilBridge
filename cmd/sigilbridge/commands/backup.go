package commands

import (
	"context"

	"github.com/sigilbridge/sigilbridge/internal/config"
	"github.com/sigilbridge/sigilbridge/internal/storage"
)

func Backup(srcPath, dstPath string) error {
	return storage.Backup(srcPath, dstPath)
}

func BackupConfig(_ context.Context, configPath, dstPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	srcPath := config.ResolveRelative(configPath, cfg.Storage.Path)
	return storage.Backup(srcPath, dstPath)
}
