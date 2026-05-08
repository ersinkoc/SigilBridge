package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/pricing"
)

const maxPricingSourceBytes = 1 << 20

func PricingShow() ([]byte, error) {
	table, err := pricing.LoadWithOverride("")
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(table, "", "  ")
}

func PricingUpdate(source, output string) error {
	if source == "" {
		return fmt.Errorf("pricing source URL or path is required")
	}
	raw, err := readPricingSource(source)
	if err != nil {
		return err
	}
	if _, err := pricing.Parse(raw); err != nil {
		return err
	}
	if output == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home directory: %w", err)
		}
		output = filepath.Join(home, ".sigilbridge", "pricing.yaml")
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o700); err != nil {
		return fmt.Errorf("create pricing directory: %w", err)
	}
	if err := os.WriteFile(output, raw, 0o600); err != nil {
		return fmt.Errorf("write pricing %q: %w", output, err)
	}
	return nil
}

func readPricingSource(source string) ([]byte, error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(source)
		if err != nil {
			return nil, fmt.Errorf("download pricing: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("download pricing returned HTTP %d", resp.StatusCode)
		}
		raw, err := io.ReadAll(io.LimitReader(resp.Body, maxPricingSourceBytes+1))
		if err != nil {
			return nil, fmt.Errorf("read pricing response: %w", err)
		}
		if len(raw) > maxPricingSourceBytes {
			return nil, fmt.Errorf("pricing response exceeds %d bytes", maxPricingSourceBytes)
		}
		return raw, nil
	}
	raw, err := os.ReadFile(source)
	if err != nil {
		return nil, fmt.Errorf("read pricing source %q: %w", source, err)
	}
	return raw, nil
}
