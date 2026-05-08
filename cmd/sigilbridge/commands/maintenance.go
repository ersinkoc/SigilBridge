package commands

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/audit"
	"github.com/sigilbridge/sigilbridge/internal/config"
)

func MaintenanceVacuum(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, "VACUUM")
	return err
}

func MaintenanceVacuumConfig(ctx context.Context, configPath string) error {
	db, err := openConfiguredDB(configPath)
	if err != nil {
		return err
	}
	defer db.Close()
	return MaintenanceVacuum(ctx, db)
}

func MaintenancePruneAuditConfig(_ context.Context, configPath string, now time.Time) error {
	if configPath == "" {
		return fmt.Errorf("config path is required")
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	if !cfg.Audit.Enabled {
		return nil
	}
	auditDir := config.ResolveRelative(configPath, cfg.Audit.Path)
	if err := audit.RotateAndPrune(auditDir, now, cfg.Audit.RotateCompressAfterDays, cfg.Audit.RetentionDays); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return nil
}
