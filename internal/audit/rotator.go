package audit

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func RotateAndPrune(dir string, now time.Time, compressAfterDays, retentionDays int) error {
	if dir == "" {
		return fmt.Errorf("audit directory is required")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read audit directory: %w", err)
	}
	today := dateOnly(now.UTC())
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		day, ok := auditFileDay(name)
		if !ok {
			continue
		}
		ageDays := int(today.Sub(day).Hours() / 24)
		path := filepath.Join(dir, name)
		if retentionDays > 0 && ageDays > retentionDays {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("prune audit file %q: %w", path, err)
			}
			continue
		}
		if compressAfterDays > 0 && ageDays > compressAfterDays && strings.HasSuffix(name, ".jsonl") {
			if err := gzipFile(path); err != nil {
				return err
			}
		}
	}
	return nil
}

func auditFileDay(name string) (time.Time, bool) {
	if len(name) < len("2006-01-02.jsonl") {
		return time.Time{}, false
	}
	if !strings.HasSuffix(name, ".jsonl") && !strings.HasSuffix(name, ".jsonl.gz") {
		return time.Time{}, false
	}
	day, err := time.Parse("2006-01-02", name[:10])
	return day, err == nil
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func gzipFile(path string) error {
	in, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open audit file for gzip: %w", err)
	}

	outPath := path + ".gz"
	out, err := os.OpenFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		_ = in.Close()
		return fmt.Errorf("create gzip audit file: %w", err)
	}
	gz := gzip.NewWriter(out)
	if _, err := io.Copy(gz, in); err != nil {
		_ = gz.Close()
		_ = out.Close()
		_ = in.Close()
		return fmt.Errorf("gzip audit file: %w", err)
	}
	if err := gz.Close(); err != nil {
		_ = out.Close()
		_ = in.Close()
		return fmt.Errorf("close gzip writer: %w", err)
	}
	if err := out.Close(); err != nil {
		_ = in.Close()
		return fmt.Errorf("close gzip audit file: %w", err)
	}
	if err := in.Close(); err != nil {
		return fmt.Errorf("close source audit file: %w", err)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove uncompressed audit file: %w", err)
	}
	return nil
}
