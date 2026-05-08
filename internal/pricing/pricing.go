package pricing

import (
	"bytes"
	"embed"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed pricing.yaml
var embedded embed.FS

type Table struct {
	Pricing map[string]ProviderPricing `yaml:"pricing"`
}

type ProviderPricing map[string]ModelPrice

type ModelPrice struct {
	InputPerMTokCents      int64  `yaml:"input_per_mtok_cents"`
	OutputPerMTokCents     int64  `yaml:"output_per_mtok_cents"`
	CacheReadPerMTokCents  int64  `yaml:"cache_read_per_mtok_cents"`
	CacheWritePerMTokCents int64  `yaml:"cache_write_per_mtok_cents"`
	SubscriptionMetering   string `yaml:"subscription_metering"`
}

type Usage struct {
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
}

func LoadDefault() (*Table, error) {
	raw, err := embedded.ReadFile("pricing.yaml")
	if err != nil {
		return nil, fmt.Errorf("read embedded pricing: %w", err)
	}
	return Parse(raw)
}

func LoadFile(path string) (*Table, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read pricing %q: %w", path, err)
	}
	return Parse(raw)
}

func LoadWithOverride(home string) (*Table, error) {
	if home != "" {
		override := filepath.Join(home, ".sigilbridge", "pricing.yaml")
		if _, err := os.Stat(override); err == nil {
			return LoadFile(override)
		}
	}
	return LoadDefault()
}

func Parse(raw []byte) (*Table, error) {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	var table Table
	if err := dec.Decode(&table); err != nil {
		return nil, fmt.Errorf("parse pricing: %w", err)
	}
	if len(table.Pricing) == 0 {
		return nil, fmt.Errorf("pricing table is empty")
	}
	return &table, nil
}

func (t *Table) CostCents(provider, model string, usage Usage) (int64, error) {
	if t == nil {
		return 0, fmt.Errorf("pricing table is nil")
	}
	providerPricing, ok := t.Pricing[provider]
	if !ok {
		return 0, fmt.Errorf("provider %q not found in pricing table", provider)
	}
	price, ok := providerPricing[model]
	if !ok {
		return 0, fmt.Errorf("model %q not found in pricing table for provider %q", model, provider)
	}

	total := usage.InputTokens*price.InputPerMTokCents +
		usage.OutputTokens*price.OutputPerMTokCents +
		usage.CacheReadTokens*price.CacheReadPerMTokCents +
		usage.CacheWriteTokens*price.CacheWritePerMTokCents
	return int64(math.Round(float64(total) / 1_000_000)), nil
}
