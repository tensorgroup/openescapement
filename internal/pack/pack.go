// Package pack loads and validates rule-pack sources: pack.yaml manifest,
// markdown rule fragments, skills, catalog, and constraints.
package pack

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/tensorgroup/openescapement/internal/esc"
)

type Manifest struct {
	Schema      int            `yaml:"schema"`
	Name        string         `yaml:"name"`
	Version     string         `yaml:"version"`
	Description string         `yaml:"description"`
	Rules       []string       `yaml:"rules"`
	Skills      []string       `yaml:"skills"`
	MCP         MCPSpec        `yaml:"mcp"`
	Catalog     []CatalogEntry `yaml:"catalog"`
	Constraints Constraints    `yaml:"constraints"`
}

type MCPSpec struct {
	Servers map[string]map[string]any `yaml:"servers"`
}

type CatalogEntry struct {
	Name     string `yaml:"name"`
	Category string `yaml:"category"`
	Status   string `yaml:"status"`
	Notes    string `yaml:"notes"`
}

type Constraints struct {
	MaxFileBytes      int      `yaml:"max_file_bytes"`
	ForbiddenPatterns []string `yaml:"forbidden_patterns"`
}

type Pack struct {
	Dir       string
	Manifest  Manifest
	Fragments []Fragment
}

var validCatalogStatus = map[string]bool{
	"preferred": true, "allowed": true, "review-required": true, "banned": true,
}

// Load reads and validates the pack rooted at dir.
func Load(dir string) (*Pack, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "pack.yaml"))
	if err != nil {
		return nil, fmt.Errorf("%w: reading pack.yaml in %s: %v", esc.ErrManifest, dir, err)
	}
	var m Manifest
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("%w: parsing pack.yaml: %v", esc.ErrManifest, err)
	}
	if err := m.validate(dir); err != nil {
		return nil, err
	}
	p := &Pack{Dir: dir, Manifest: m}
	for _, rel := range m.Rules {
		frag, err := loadFragment(dir, rel)
		if err != nil {
			return nil, err
		}
		p.Fragments = append(p.Fragments, *frag)
	}
	return p, nil
}

func (m *Manifest) validate(dir string) error {
	fail := func(format string, a ...any) error {
		return fmt.Errorf("%w: %s", esc.ErrManifest, fmt.Sprintf(format, a...))
	}
	if m.Schema != 1 {
		return fail("unsupported schema %d (want 1)", m.Schema)
	}
	if m.Name == "" {
		return fail("name is required")
	}
	if m.Version == "" {
		return fail("version is required")
	}
	for _, rel := range m.Rules {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			return fail("rule file %s: %v", rel, err)
		}
	}
	for _, rel := range m.Skills {
		info, err := os.Stat(filepath.Join(dir, rel))
		if err != nil || !info.IsDir() {
			return fail("skill %s: not a directory", rel)
		}
	}
	for _, c := range m.Catalog {
		if c.Name == "" {
			return fail("catalog entry missing name")
		}
		if !validCatalogStatus[c.Status] {
			return fail("catalog entry %q: invalid status %q (want preferred|allowed|review-required|banned)", c.Name, c.Status)
		}
	}
	return nil
}
