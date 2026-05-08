package config

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type PoolsFile struct {
	Pools Pools `yaml:"pools"`
}

type Pools []Pool

type Pool struct {
	Name           string               `yaml:"name"`
	Description    string               `yaml:"description"`
	Strategy       string               `yaml:"strategy"`
	Upstreams      []Upstream           `yaml:"upstreams"`
	Cooldown       CooldownConfig       `yaml:"cooldown"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
	Retry          RetryConfig          `yaml:"retry"`
}

type Upstream struct {
	ID       string         `yaml:"id"`
	Provider string         `yaml:"provider"`
	Config   map[string]any `yaml:"config"`
	Priority int            `yaml:"priority"`
	Weight   int            `yaml:"weight"`
}

type CooldownConfig struct {
	InitialSeconds int    `yaml:"initial_seconds"`
	MaxSeconds     int    `yaml:"max_seconds"`
	Backoff        string `yaml:"backoff"`
}

type CircuitBreakerConfig struct {
	FailureThreshold       int `yaml:"failure_threshold"`
	RecoveryTimeoutSeconds int `yaml:"recovery_timeout_seconds"`
}

type RetryConfig struct {
	MaxAttempts          int   `yaml:"max_attempts"`
	RetryableStatusCodes []int `yaml:"retryable_status_codes"`
}

func (p *Pools) UnmarshalYAML(node *yaml.Node) error {
	var list []Pool
	if err := node.Decode(&list); err == nil {
		*p = list
		return nil
	}

	var byName map[string]Pool
	if err := node.Decode(&byName); err != nil {
		return err
	}
	out := make([]Pool, 0, len(byName))
	for name, pool := range byName {
		if pool.Name == "" {
			pool.Name = name
		}
		out = append(out, pool)
	}
	*p = out
	return nil
}

func LoadPools(path, masterKeyEnv string) (*PoolsFile, error) {
	// #nosec G304 -- pools path is explicit local operator input.
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read pools %q: %w", path, err)
	}
	return LoadPoolsFromBytes(raw, masterKeyEnv)
}

func LoadPoolsFromBytes(raw []byte, masterKeyEnv string) (*PoolsFile, error) {
	dec := yaml.NewDecoder(bytes.NewReader([]byte(os.ExpandEnv(string(raw)))))
	dec.KnownFields(true)
	var pools PoolsFile
	if err := dec.Decode(&pools); err != nil {
		return nil, fmt.Errorf("parse pools: %w", err)
	}
	if err := ValidatePools(pools.Pools, masterKeyEnv); err != nil {
		return nil, err
	}
	return &pools, nil
}

func ValidatePools(pools []Pool, masterKeyEnv string) error {
	if masterKeyEnv == "" {
		masterKeyEnv = "SIGILBRIDGE_MASTER_KEY"
	}
	masterKeyPresent := os.Getenv(masterKeyEnv) != ""
	for _, pool := range pools {
		if strings.TrimSpace(pool.Name) == "" {
			return fmt.Errorf("pool name is required")
		}
		for _, upstream := range pool.Upstreams {
			if strings.TrimSpace(upstream.ID) == "" {
				return fmt.Errorf("pool %q has upstream with empty id", pool.Name)
			}
			if strings.TrimSpace(upstream.Provider) == "" {
				return fmt.Errorf("pool %q upstream %q provider is required", pool.Name, upstream.ID)
			}
			if requiresVault(upstream) && !masterKeyPresent {
				return fmt.Errorf("pool %q upstream %q requires %s", pool.Name, upstream.ID, masterKeyEnv)
			}
		}
	}
	return nil
}

func requiresVault(upstream Upstream) bool {
	provider := strings.ToLower(upstream.Provider)
	if strings.Contains(provider, "oauth") || strings.HasSuffix(provider, "_web") || provider == "claude_web" || provider == "chatgpt_web" {
		return true
	}
	for _, value := range upstream.Config {
		if s, ok := value.(string); ok && strings.HasPrefix(s, "vault://") {
			return true
		}
	}
	return false
}
