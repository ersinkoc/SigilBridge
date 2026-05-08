package storage

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func Backup(srcPath, dstPath string) error {
	if srcPath == "" {
		return fmt.Errorf("source database path is required")
	}
	if dstPath == "" {
		return fmt.Errorf("destination database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o700); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}

	db, err := OpenDB(srcPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec("VACUUM INTO ?", dstPath); err != nil {
		return fmt.Errorf("vacuum database into %q: %w", dstPath, err)
	}
	return nil
}

func Restore(srcPath, dstPath string) error {
	if srcPath == "" {
		return fmt.Errorf("source backup path is required")
	}
	if dstPath == "" {
		return fmt.Errorf("destination database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o700); err != nil {
		return fmt.Errorf("create restore directory: %w", err)
	}

	// #nosec G304 -- restore source is an explicit local backup path supplied by the operator.
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open backup %q: %w", srcPath, err)
	}
	defer src.Close()

	tmp := dstPath + ".restore-tmp"
	// #nosec G304 -- restore destination is an explicit local database path supplied by the operator.
	dst, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create restore temp %q: %w", tmp, err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("copy backup: %w", err)
	}
	if err := dst.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close restore temp: %w", err)
	}

	db, err := OpenDB(tmp)
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("verify restored database: %w", err)
	}
	version, err := CurrentVersion(db)
	_ = db.Close()
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("verify migration version: %w", err)
	}
	if version != 4 {
		_ = os.Remove(tmp)
		return fmt.Errorf("backup migration version = %d, want 4", version)
	}

	if err := os.Rename(tmp, dstPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace database %q: %w", dstPath, err)
	}
	return nil
}

func mustExec(db *sql.DB, query string, args ...any) error {
	_, err := db.Exec(query, args...)
	return err
}
