// Package config loads and saves the governed repo's .escapement/config.yaml.
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const Dir = ".escapement"
const fileName = "config.yaml"

// PackRef is one pack source pin in config.yaml.
type PackRef struct {
	Source string `yaml:"source"`
	Ref    string `yaml:"ref"`
	Trust  string `yaml:"trust,omitempty"` // "" (signed, default) | "unsigned"
}

type Config struct {
	Schema                 int       `yaml:"schema"`
	Packs                  []PackRef `yaml:"packs"`
	AllowedSignersFile     string    `yaml:"allowed_signers_file,omitempty"`
	Targets                []string  `yaml:"targets,omitempty"`                   // empty = all targets
	AllowCustomTargetFiles []string  `yaml:"allow_custom_target_files,omitempty"` // acknowledged custom-target files (§2.3)
	ReportAmendments       string    `yaml:"report_amendments,omitempty"`         // "metrics" | "off"; may only clamp down
}

func Path(root string) string { return filepath.Join(root, Dir, fileName) }

// Load reads .escapement/config.yaml under root.
func Load(root string) (*Config, error) {
	raw, err := os.ReadFile(Path(root))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w (run `esc init`?)", Path(root), err)
	}
	var c Config
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", Path(root), err)
	}
	if c.Schema != 1 {
		return nil, fmt.Errorf("%s: unsupported schema %d (want 1)", Path(root), c.Schema)
	}
	return &c, nil
}

func (c *Config) Save(root string) error {
	out, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, Dir), 0o755); err != nil {
		return err
	}
	return os.WriteFile(Path(root), out, 0o644)
}
