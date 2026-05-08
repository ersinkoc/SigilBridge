package plugin

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Manifest struct {
	ID           string   `yaml:"id" json:"id"`
	Name         string   `yaml:"name" json:"name"`
	Version      string   `yaml:"version" json:"version"`
	Protocol     string   `yaml:"protocol" json:"protocol"`
	Command      string   `yaml:"command" json:"command"`
	Args         []string `yaml:"args" json:"args,omitempty"`
	ProviderIDs  []string `yaml:"provider_ids" json:"provider_ids"`
	Capabilities []string `yaml:"capabilities" json:"capabilities,omitempty"`
}

func LoadManifest(path string) (Manifest, error) {
	// #nosec G304 -- plugin manifest path is explicit local operator configuration.
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	return ParseManifest(raw)
}

func ParseManifest(raw []byte) (Manifest, error) {
	var manifest Manifest
	if err := yaml.Unmarshal(raw, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse plugin manifest: %w", err)
	}
	return manifest, manifest.Validate()
}

func (m Manifest) Validate() error {
	if m.ID == "" {
		return fmt.Errorf("plugin id is required")
	}
	if m.Version == "" {
		return fmt.Errorf("plugin version is required")
	}
	if m.Protocol == "" {
		return fmt.Errorf("plugin protocol is required")
	}
	if m.Command == "" {
		return fmt.Errorf("plugin command is required")
	}
	if len(m.ProviderIDs) == 0 {
		return fmt.Errorf("plugin provider_ids is required")
	}
	return nil
}
