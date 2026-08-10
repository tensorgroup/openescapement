package pack

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/tensorgroup/openescapement/internal/esc"
)

func decodeSkills(t *testing.T, doc string) []SkillEntry {
	t.Helper()
	var m struct {
		Skills []SkillEntry `yaml:"skills"`
	}
	dec := yaml.NewDecoder(strings.NewReader(doc))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return m.Skills
}

func TestSkillEntryPlainStringKeepsLegacyNaming(t *testing.T) {
	got := decodeSkills(t, "skills:\n  - skills/vault-usage\n")
	if len(got) != 1 || got[0].Path != "skills/vault-usage" || got[0].Name != "" {
		t.Fatalf("plain entry parsed wrong: %+v", got)
	}
	if dn := got[0].DirName("acme-org"); dn != "esc-acme-org-vault-usage" {
		t.Fatalf("legacy DirName = %q, want esc-acme-org-vault-usage", dn)
	}
}

func TestSkillEntryObjectSetsName(t *testing.T) {
	got := decodeSkills(t, "skills:\n  - path: skills/brainstorming\n    name: brainstorming\n")
	if len(got) != 1 || got[0].Path != "skills/brainstorming" || got[0].Name != "brainstorming" {
		t.Fatalf("object entry parsed wrong: %+v", got)
	}
	if dn := got[0].DirName("acme-org"); dn != "brainstorming" {
		t.Fatalf("override DirName = %q, want brainstorming", dn)
	}
}

func TestSkillEntryRejectsUnknownKeysAndBadShapes(t *testing.T) {
	for _, doc := range []string{
		"skills:\n  - path: skills/x\n    nmae: typo\n", // unknown key
		"skills:\n  - name: only-a-name\n",              // path required
		"skills:\n  - [not, a, mapping]\n",              // wrong node kind
	} {
		var m struct {
			Skills []SkillEntry `yaml:"skills"`
		}
		if err := yaml.Unmarshal([]byte(doc), &m); err == nil {
			t.Errorf("decode accepted %q", doc)
		}
	}
}

func TestSkillEntryMarshalRoundTrips(t *testing.T) {
	in := []SkillEntry{{Path: "skills/vault-usage"}, {Path: "skills/brainstorming", Name: "brainstorming"}}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(map[string][]SkillEntry{"skills": in}); err != nil {
		t.Fatal(err)
	}
	enc.Close()
	out := buf.String()
	if !strings.Contains(out, "- skills/vault-usage\n") {
		t.Errorf("plain entry did not marshal as a plain string:\n%s", out)
	}
	if !strings.Contains(out, "path: skills/brainstorming") || !strings.Contains(out, "name: brainstorming") {
		t.Errorf("object entry did not marshal as a mapping:\n%s", out)
	}
	got := decodeSkills(t, out)
	if len(got) != 2 || got[0] != in[0] || got[1] != in[1] {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}

func TestManifestRejectsDuplicateResolvedSkillDirs(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{"skills/a/SKILL.md", "other/brainstorming/SKILL.md"} {
		writeSkillFile(t, dir, p)
	}
	manifest := `schema: 1
name: acme
version: 1.0.0
skills:
  - path: skills/a
    name: brainstorming
  - path: other/brainstorming
    name: brainstorming
`
	if err := os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil || !errors.Is(err, esc.ErrManifest) {
		t.Fatalf("want ErrManifest for duplicate resolved dir, got %v", err)
	}
}

// TestManifestRejectsInvalidSkillName is the negative test for the
// ValidName check on an object-form {path, name} skill entry (pack.go's
// validate, "skill %s: name %q must match..."). This is the control that
// backstops both Important-1 (add-skill validates before writing) and the
// sources.yaml traversal fix: an invalid name reaching pack.yaml at all
// must fail Load, and until now nothing exercised that branch directly.
func TestManifestRejectsInvalidSkillName(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "skills/a/SKILL.md")
	manifest := `schema: 1
name: acme
version: 1.0.0
skills:
  - path: skills/a
    name: ../../evil
`
	if err := os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil || !errors.Is(err, esc.ErrManifest) {
		t.Fatalf("want ErrManifest for an invalid skill name, got %v", err)
	}
}

func writeSkillFile(t *testing.T, root, rel string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte("---\nname: x\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
