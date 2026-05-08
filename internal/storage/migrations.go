package storage

import (
	"database/sql"
	"embed"
	"fmt"
	"io"
	"strings"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const migrationsDir = "migrations"

func Up(db *sql.DB) error {
	return withGoose(func() error {
		return goose.Up(db, migrationsDir)
	})
}

func DownToZero(db *sql.DB) error {
	return withGoose(func() error {
		return goose.DownTo(db, migrationsDir, 0)
	})
}

func CurrentVersion(db *sql.DB) (int64, error) {
	if err := goose.SetDialect("sqlite3"); err != nil {
		return 0, fmt.Errorf("set goose dialect: %w", err)
	}
	return goose.GetDBVersion(db)
}

func Status(db *sql.DB, out io.Writer) error {
	current, err := CurrentVersion(db)
	if err != nil {
		return err
	}
	entries, err := migrationsFS.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version := int64(0)
		_, _ = fmt.Sscanf(entry.Name(), "%d_", &version)
		state := "pending"
		if version <= current {
			state = "applied"
		}
		if _, err := fmt.Fprintf(out, "%05d %s %s\n", version, state, entry.Name()); err != nil {
			return fmt.Errorf("write migration status: %w", err)
		}
	}
	return nil
}

func withGoose(fn func() error) error {
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	goose.SetBaseFS(migrationsFS)
	if err := fn(); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
