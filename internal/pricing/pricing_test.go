package pricing

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefault(t *testing.T) {
	table, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error = %v", err)
	}
	if _, ok := table.Pricing["anthropic_api"]["claude-sonnet-4-5"]; !ok {
		t.Fatalf("embedded pricing missing anthropic sonnet")
	}
}

func TestCostCents(t *testing.T) {
	table, err := LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	got, err := table.CostCents("openai_api", "gpt-5", Usage{
		InputTokens:  1_000_000,
		OutputTokens: 2_000_000,
	})
	if err != nil {
		t.Fatalf("CostCents() error = %v", err)
	}
	if got != 2200 {
		t.Fatalf("CostCents() = %d, want 2200", got)
	}
}

func TestLoadWithOverride(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".sigilbridge")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pricing.yaml"), []byte(`
pricing:
  test_provider:
    test_model:
      input_per_mtok_cents: 1
      output_per_mtok_cents: 2
`), 0o600); err != nil {
		t.Fatal(err)
	}
	table, err := LoadWithOverride(home)
	if err != nil {
		t.Fatalf("LoadWithOverride() error = %v", err)
	}
	if _, ok := table.Pricing["test_provider"]["test_model"]; !ok {
		t.Fatalf("override pricing was not loaded")
	}
}
