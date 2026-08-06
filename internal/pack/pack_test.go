package pack

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/esc"
)

// writePack creates a pack directory from a map of relative path -> content.
func writePack(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const validManifest = `schema: 1
name: acme-org
version: 1.4.0
description: Acme baseline
rules:
  - rules/secrets.md
  - rules/hosting.md
skills:
  - skills/vault-usage
mcp:
  servers:
    acme-paved-path:
      command: npx
      args: ["-y", "@acme/paved-path-mcp"]
catalog:
  - name: Tailscale
    category: hosting-exposure
    status: preferred
    notes: Org tailnet
  - name: Raw port forwarding
    category: hosting-exposure
    status: banned
constraints:
  max_file_bytes: 20000
  forbidden_patterns:
    - "ignore (the )?(above|governance)"
`

func validFiles() map[string]string {
	return map[string]string{
		"pack.yaml":                   validManifest,
		"rules/secrets.md":            "---\ntargets: [claude, agents]\n---\n## Secrets\nUse Vault.\n",
		"rules/hosting.md":            "## Hosting\nTailscale preferred.\n",
		"skills/vault-usage/SKILL.md": "---\nname: vault-usage\ndescription: how to use vault\n---\nUse vault.\n",
	}
}

func TestLoadManifest(t *testing.T) {
	dir := writePack(t, validFiles())
	p, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	m := p.Manifest
	if m.Name != "acme-org" || m.Version != "1.4.0" || m.Schema != 1 {
		t.Errorf("manifest basics wrong: %+v", m)
	}
	if len(m.Rules) != 2 || len(m.Skills) != 1 {
		t.Errorf("rules/skills wrong: %+v", m)
	}
	if len(m.Catalog) != 2 || m.Catalog[0].Status != "preferred" {
		t.Errorf("catalog wrong: %+v", m.Catalog)
	}
	if m.Constraints.MaxFileBytes != 20000 || len(m.Constraints.ForbiddenPatterns) != 1 {
		t.Errorf("constraints wrong: %+v", m.Constraints)
	}
	if _, ok := m.MCP.Servers["acme-paved-path"]; !ok {
		t.Errorf("mcp servers wrong: %+v", m.MCP)
	}
	if len(p.Fragments) != 2 {
		t.Fatalf("fragments: got %d want 2", len(p.Fragments))
	}
	if p.Fragments[0].Body != "## Secrets\nUse Vault.\n" {
		t.Errorf("fragment body not byte-exact: %q", p.Fragments[0].Body)
	}
}

func TestLoadManifestErrors(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(map[string]string)
	}{
		{"missing name", func(f map[string]string) {
			f["pack.yaml"] = "schema: 1\nversion: 1.0.0\nrules: [rules/secrets.md]\n"
		}},
		{"missing version", func(f map[string]string) {
			f["pack.yaml"] = "schema: 1\nname: x\nrules: [rules/secrets.md]\n"
		}},
		{"bad schema", func(f map[string]string) {
			f["pack.yaml"] = "schema: 99\nname: x\nversion: 1.0.0\n"
		}},
		{"missing rule file", func(f map[string]string) { delete(f, "rules/hosting.md") }},
		{"missing skill dir", func(f map[string]string) { delete(f, "skills/vault-usage/SKILL.md") }},
		{"unknown manifest field", func(f map[string]string) {
			f["pack.yaml"] = "schema: 1\nname: x\nversion: 1.0.0\nbogus: true\n"
		}},
		{"invalid catalog status", func(f map[string]string) {
			f["pack.yaml"] = "schema: 1\nname: x\nversion: 1.0.0\ncatalog:\n  - name: T\n    category: c\n    status: great\n"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := validFiles()
			tc.mutate(files)
			dir := writePack(t, files)
			_, err := Load(dir)
			if !errors.Is(err, esc.ErrManifest) {
				t.Fatalf("want ErrManifest, got %v", err)
			}
		})
	}
}

func TestCustomTargets(t *testing.T) {
	base := func() map[string]string {
		f := validFiles()
		f["pack.yaml"] = `schema: 1
name: acme-org
version: 1.4.0
custom_targets:
  - name: copilot
    file: .github/copilot-instructions.md
    doc: https://docs.github.com/copilot
    description: Copilot instructions.
rules:
  - rules/secrets.md
`
		f["rules/secrets.md"] = "---\ntargets: [claude, copilot]\n---\nbody\n"
		delete(f, "rules/hosting.md")
		return f
	}

	t.Run("parses and accepts own custom target in fragment", func(t *testing.T) {
		p, err := Load(writePack(t, base()))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if len(p.Manifest.CustomTargets) != 1 {
			t.Fatalf("custom targets: %+v", p.Manifest.CustomTargets)
		}
		ct := p.Manifest.CustomTargets[0]
		if ct.Name != "copilot" || ct.File != ".github/copilot-instructions.md" {
			t.Errorf("custom target fields: %+v", ct)
		}
		if got := p.Fragments[0].Targets; len(got) != 2 || got[1] != "copilot" {
			t.Errorf("fragment targets: %v", got)
		}
	})

	t.Run("unknown/foreign fragment target is a constraint error naming pack and rule", func(t *testing.T) {
		f := base()
		f["rules/secrets.md"] = "---\ntargets: [nope]\n---\nbody\n"
		_, err := Load(writePack(t, f))
		if !errors.Is(err, esc.ErrConstraint) {
			t.Fatalf("want ErrConstraint, got %v", err)
		}
		if !strings.Contains(err.Error(), "acme-org") {
			t.Errorf("error should name the pack (acme-org): %v", err)
		}
		if !strings.Contains(err.Error(), "rules/secrets.md") {
			t.Errorf("error should name the rule (rules/secrets.md): %v", err)
		}
	})

	t.Run("missing custom target name is a manifest error naming the entry", func(t *testing.T) {
		f := base()
		f["pack.yaml"] = "schema: 1\nname: x\nversion: 1.0.0\ncustom_targets:\n  - file: X.md\n"
		delete(f, "rules/secrets.md")
		_, err := Load(writePack(t, f))
		if !errors.Is(err, esc.ErrManifest) {
			t.Fatalf("want ErrManifest, got %v", err)
		}
		if !strings.Contains(err.Error(), "0") {
			t.Errorf("error should identify the offending entry (index 0): %v", err)
		}
	})

	t.Run("duplicate custom target name (case-folded) is a manifest error", func(t *testing.T) {
		f := base()
		f["pack.yaml"] = `schema: 1
name: acme-org
version: 1.4.0
custom_targets:
  - name: copilot
    file: .github/copilot-instructions.md
  - name: Copilot
    file: .github/other.md
rules: []
`
		delete(f, "rules/secrets.md")
		_, err := Load(writePack(t, f))
		if !errors.Is(err, esc.ErrManifest) {
			t.Fatalf("want ErrManifest, got %v", err)
		}
		if !strings.Contains(err.Error(), "Copilot") {
			t.Errorf("error should name the duplicated value: %v", err)
		}
	})

	t.Run("duplicate custom target file (case-folded) is a manifest error", func(t *testing.T) {
		f := base()
		f["pack.yaml"] = `schema: 1
name: acme-org
version: 1.4.0
custom_targets:
  - name: copilot
    file: .github/copilot-instructions.md
  - name: copilot-2
    file: .GITHUB/Copilot-Instructions.md
rules: []
`
		delete(f, "rules/secrets.md")
		_, err := Load(writePack(t, f))
		if !errors.Is(err, esc.ErrManifest) {
			t.Fatalf("want ErrManifest, got %v", err)
		}
		if !strings.Contains(err.Error(), "copilot-2") {
			t.Errorf("error should name the duplicated entry: %v", err)
		}
	})

	t.Run("unknown manifest field still rejected (KnownFields stays true)", func(t *testing.T) {
		f := base()
		f["pack.yaml"] = "schema: 1\nname: x\nversion: 1.0.0\nbogus: true\n"
		delete(f, "rules/secrets.md")
		_, err := Load(writePack(t, f))
		if !errors.Is(err, esc.ErrManifest) {
			t.Fatalf("want ErrManifest, got %v", err)
		}
	})
}

func TestFragmentFrontmatter(t *testing.T) {
	files := validFiles()
	files["rules/secrets.md"] = "---\ntargets: [claude]\n---\nbody line\n"
	files["rules/hosting.md"] = "no frontmatter body\n"
	dir := writePack(t, files)
	p, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Fragments[0].Targets; len(got) != 1 || got[0] != "claude" {
		t.Errorf("targets: %v", got)
	}
	if p.Fragments[0].Body != "body line\n" {
		t.Errorf("body: %q", p.Fragments[0].Body)
	}
	// No frontmatter: applies to all agent-file targets.
	if got := p.Fragments[1].Targets; len(got) != 0 {
		t.Errorf("no-frontmatter targets should be empty (=all): %v", got)
	}
	if p.Fragments[1].Body != "no frontmatter body\n" {
		t.Errorf("body: %q", p.Fragments[1].Body)
	}
}

func TestDirHashDeterministic(t *testing.T) {
	files := validFiles()
	d1 := writePack(t, files)
	d2 := writePack(t, files)
	h1, err := DirHash(d1)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := DirHash(d2)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Errorf("same content, different hash: %s vs %s", h1, h2)
	}
	files["rules/secrets.md"] += "x"
	d3 := writePack(t, files)
	h3, err := DirHash(d3)
	if err != nil {
		t.Fatal(err)
	}
	if h3 == h1 {
		t.Error("changed content, same hash")
	}
	if len(h1) < 10 || h1[:7] != "sha256:" {
		t.Errorf("hash format: %s", h1)
	}
}

// TestDirHashOfAgreesWithDirHash is the load-bearing property for Task 6:
// DirHashOf(dir, rel) must be the same canonical hash as DirHash(dir) when
// rel is the full file list of dir, in any order, so a dir artifact's
// managed hash (computed at plan time by DirHash over the pack's own tree)
// agrees with the status-time hash (computed by DirHashOf over the same
// files read back off disk). A divergence here would make every dir
// artifact report altered on every sync.
func TestDirHashOfAgreesWithDirHash(t *testing.T) {
	files := validFiles()
	dir := writePack(t, files)

	full, err := DirHash(dir)
	if err != nil {
		t.Fatal(err)
	}

	rel := []string{
		"skills/vault-usage/SKILL.md",
		"rules/hosting.md",
		"rules/secrets.md",
		"pack.yaml",
	}
	of, err := DirHashOf(dir, rel)
	if err != nil {
		t.Fatal(err)
	}
	if of != full {
		t.Errorf("DirHashOf(dir, allFiles) = %s, want DirHash(dir) = %s", of, full)
	}

	// A subset must hash differently from the full tree (this is the whole
	// point: an added file must not be silently absorbed into the hash).
	subset, err := DirHashOf(dir, []string{"pack.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if subset == full {
		t.Error("hashing a subset of files produced the same hash as the whole tree")
	}
}
