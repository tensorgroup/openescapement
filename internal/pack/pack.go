// Package pack loads and validates rule-pack sources: pack.yaml manifest,
// markdown rule fragments, skills, catalog, and constraints.
package pack

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/tensorgroup/openescapement/internal/esc"
)

// Manifest is the parsed pack.yaml.
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
	UpdateCheck *UpdateCheck   `yaml:"update_check,omitempty"`

	CustomTargets []CustomTarget `yaml:"custom_targets,omitempty"`
}

// MCPSpec declares MCP server entries a pack injects into .mcp.json.
type MCPSpec struct {
	Servers map[string]map[string]any `yaml:"servers"`
}

// CatalogEntry is one tool/service in the allow-deny catalog.
type CatalogEntry struct {
	Name     string `yaml:"name"`
	Category string `yaml:"category"`
	Status   string `yaml:"status"`
	Notes    string `yaml:"notes"`
}

// Constraints bound what may land on disk after merging (validated against
// final file contents, including team-owned portions).
type Constraints struct {
	MaxFileBytes      int      `yaml:"max_file_bytes"`
	ForbiddenPatterns []string `yaml:"forbidden_patterns"`
}

// CustomTarget is a pack-defined managed-block markdown target (§1.2). Full
// name/file validation lives in internal/targets.ValidateCustom and runs in
// the engine after verifyTrust; the manifest only checks presence here.
type CustomTarget struct {
	Name        string `yaml:"name"`
	File        string `yaml:"file"`
	Doc         string `yaml:"doc,omitempty"`
	Description string `yaml:"description,omitempty"`
}

// UpdateCheck declares how often clients should probe this pack's source for
// newer versions. Endpoint is parsed for forward-compatibility (a future
// control-plane check surface) but is NEVER contacted in v0.1 — the client
// always uses `git ls-remote` against the pack's git source.
type UpdateCheck struct {
	Every    string `yaml:"every"`
	Endpoint string `yaml:"endpoint,omitempty"`
}

// Pack is a fully loaded, validated rule pack.
type Pack struct {
	Dir       string
	Manifest  Manifest
	Fragments []Fragment
}

var validCatalogStatus = map[string]bool{
	"preferred": true, "allowed": true, "review-required": true, "banned": true,
}

// validName constrains pack names, which become filesystem path components.
var validName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// safeRel rejects manifest path entries that could escape the pack dir:
// absolute paths and any path containing a ".." segment.
func safeRel(entry string) error {
	clean := filepath.Clean(filepath.FromSlash(entry))
	if filepath.IsAbs(clean) {
		return fmt.Errorf("absolute path %q not allowed", entry)
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q escapes the pack directory", entry)
	}
	return nil
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
	customNames := map[string]bool{}
	for _, ct := range m.CustomTargets {
		customNames[ct.Name] = true
	}
	for _, rel := range m.Rules {
		frag, err := loadFragment(dir, rel, customNames)
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
	if !validName.MatchString(m.Name) {
		return fail("name %q: must match %s (it becomes a filesystem path component)", m.Name, validName)
	}
	if m.Version == "" {
		return fail("version is required")
	}
	for _, rel := range m.Rules {
		if err := safeRel(rel); err != nil {
			return fail("rule %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			return fail("rule file %s: %v", rel, err)
		}
	}
	for _, rel := range m.Skills {
		if err := safeRel(rel); err != nil {
			return fail("skill %v", err)
		}
		info, err := os.Stat(filepath.Join(dir, rel))
		if err != nil || !info.IsDir() {
			return fail("skill %s: not a directory", rel)
		}
		if !validName.MatchString(filepath.Base(rel)) {
			return fail("skill %s: directory name must match %s", rel, validName)
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
	if m.UpdateCheck != nil {
		if _, err := ParseEvery(m.UpdateCheck.Every); err != nil {
			return fail("update_check.every: %v", err)
		}
		if e := m.UpdateCheck.Endpoint; e != "" && !strings.HasPrefix(e, "https://") {
			return fail("update_check.endpoint %q: must be an https:// URL", e)
		}
	}
	for _, ct := range m.CustomTargets {
		if ct.Name == "" {
			return fail("custom target: name is required")
		}
		if ct.File == "" {
			return fail("custom target %q: file is required", ct.Name)
		}
	}
	return nil
}

// maxDaysDuration is the largest day count that can be multiplied by 24h
// without overflowing a time.Duration (int64 nanoseconds).
const maxDaysDuration = int64(math.MaxInt64) / int64(24*time.Hour)

// ParseEvery parses an update-check cadence. Go's time.ParseDuration has no
// day unit, so "<n>d" is handled explicitly; all other units delegate to the
// stdlib. The result must be strictly positive.
func ParseEvery(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid day duration %q (use e.g. 7d)", s)
		}
		// n*24h is computed in int64 nanoseconds, which overflows (and can
		// wrap to an arbitrary, even positive, value) well before n reaches
		// the top of the int range. Bound n against the largest day count
		// that cannot overflow, rather than trusting the sign of the result.
		if int64(n) > maxDaysDuration {
			return 0, fmt.Errorf("invalid day duration %q: too large", s)
		}
		d := time.Duration(n) * 24 * time.Hour
		if d <= 0 {
			return 0, fmt.Errorf("invalid day duration %q (use e.g. 7d)", s)
		}
		return d, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q (use e.g. 7d, 24h, 90m)", s)
	}
	if d <= 0 {
		return 0, fmt.Errorf("duration must be positive: %q", s)
	}
	return d, nil
}
