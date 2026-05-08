package config

import "path/filepath"

func DefaultPricingPath(home string) string {
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".sigilbridge", "pricing.yaml")
}
