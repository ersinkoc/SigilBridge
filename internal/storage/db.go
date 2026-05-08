package storage

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const (
	defaultBusyTimeoutMS = 5000
	defaultCacheSizeKB   = 20000
	defaultMmapSizeBytes = 268435456
)

func OpenDB(path string) (*sql.DB, error) {
	if path == "" {
		return nil, fmt.Errorf("database path is required")
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	if err := applyPragmas(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func applyPragmas(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA foreign_keys = ON",
		fmt.Sprintf("PRAGMA busy_timeout = %d", defaultBusyTimeoutMS),
		"PRAGMA temp_store = MEMORY",
		fmt.Sprintf("PRAGMA cache_size = -%d", defaultCacheSizeKB),
		fmt.Sprintf("PRAGMA mmap_size = %d", defaultMmapSizeBytes),
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("apply %s: %w", pragma, err)
		}
	}
	return nil
}
