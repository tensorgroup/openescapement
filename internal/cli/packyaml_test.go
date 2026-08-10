package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/pack"
	"gopkg.in/yaml.v3"
)

func TestAppendSkillEntriesPreservesCommentsAndExisting(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "pack.yaml")
	orig := `schema: 1
# the team's own comment about this pack
name: acme
version: 1.0.0
skills:
  - skills/vault-usage # keep our vault rules
`
	if err := os.WriteFile(p, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	err := appendSkillEntries(p, []pack.SkillEntry{{Path: "skills/brainstorming", Name: "brainstorming"}})
	if err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(p)
	s := string(out)
	for _, want := range []string{
		"the team's own comment about this pack",
		"keep our vault rules",
		"- skills/vault-usage",
		"path: skills/brainstorming",
		"name: brainstorming",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in rewritten pack.yaml:\n%s", want, s)
		}
	}
	// The rewritten file must still load as a valid manifest shape.
	if _, err := decodeManifestSkills(t, s); err != nil {
		t.Fatalf("rewritten pack.yaml no longer parses: %v", err)
	}
}

func TestAppendSkillEntriesCreatesSkillsKey(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "pack.yaml")
	if err := os.WriteFile(p, []byte("schema: 1\nname: acme\nversion: 1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := appendSkillEntries(p, []pack.SkillEntry{{Path: "skills/x", Name: "x"}}); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(p)
	if !strings.Contains(string(out), "skills:") || !strings.Contains(string(out), "name: x") {
		t.Fatalf("skills key not created:\n%s", out)
	}
}

// TestAppendSkillEntriesFileShapes is the write-path x file-shape matrix
// AGENTS.md requires for every new write path. appendSkillEntries is a
// yaml.v3 Node round trip, not a byte-preserving splice like the renderer's
// managed blocks — so what must hold across each shape is reparseability,
// preserved comments and key ordering, and semantic identity of everything
// this call did not touch, not byte-for-byte identity. Each case pins the
// exact allowance the doc comment now claims, so a future change to the
// encoder's behavior (or a mistaken tightening back to "byte-preserving")
// shows up here first.
func TestAppendSkillEntriesFileShapes(t *testing.T) {
	newEntry := []pack.SkillEntry{{Path: "skills/new", Name: "new"}}

	t.Run("no trailing newline", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "pack.yaml")
		orig := "schema: 1\nname: acme\nversion: 1.0.0"
		if err := os.WriteFile(p, []byte(orig), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := appendSkillEntries(p, newEntry); err != nil {
			t.Fatal(err)
		}
		out, _ := os.ReadFile(p)
		s := string(out)
		if !strings.HasSuffix(s, "\n") {
			t.Fatalf("rewritten file must end with a newline, got %q", s)
		}
		if !strings.HasPrefix(s, "schema: 1\nname: acme\nversion: 1.0.0\n") {
			t.Fatalf("original content before the skills key must be unchanged:\n%s", s)
		}
		got, err := decodeManifestSkills(t, s)
		if err != nil {
			t.Fatalf("rewritten pack.yaml no longer parses: %v", err)
		}
		if len(got) != 1 || got[0].Name != "new" {
			t.Fatalf("appended entry missing or wrong: %+v", got)
		}
	})

	t.Run("CRLF line endings", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "pack.yaml")
		orig := "schema: 1\r\nname: acme\r\nversion: 1.0.0\r\n"
		if err := os.WriteFile(p, []byte(orig), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := appendSkillEntries(p, newEntry); err != nil {
			t.Fatal(err)
		}
		out, _ := os.ReadFile(p)
		s := string(out)
		// yaml.v3 normalizes CRLF to LF on re-encode; this is not a claim
		// this function preserves CRLF, only that the result still parses
		// and still carries every original key with its original value.
		got, err := decodeManifestSkills(t, s)
		if err != nil {
			t.Fatalf("rewritten pack.yaml no longer parses: %v", err)
		}
		if len(got) != 1 || got[0].Name != "new" {
			t.Fatalf("appended entry missing or wrong: %+v", got)
		}
		var m struct {
			Schema  int    `yaml:"schema"`
			Name    string `yaml:"name"`
			Version string `yaml:"version"`
		}
		if err := yamlUnmarshalLoose([]byte(s), &m); err != nil {
			t.Fatal(err)
		}
		if m.Schema != 1 || m.Name != "acme" || m.Version != "1.0.0" {
			t.Fatalf("original fields lost across the CRLF round trip: %+v", m)
		}
	})

	t.Run("blank lines between top-level keys are not preserved", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "pack.yaml")
		orig := "schema: 1\n\nname: acme\n\nversion: 1.0.0\n"
		if err := os.WriteFile(p, []byte(orig), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := appendSkillEntries(p, newEntry); err != nil {
			t.Fatal(err)
		}
		out, _ := os.ReadFile(p)
		s := string(out)
		// This is the documented, accepted divergence from byte
		// preservation: yaml.v3's encoder drops blank lines between
		// top-level mapping keys. Pinning it here means a future yaml.v3
		// upgrade that changes this behavior is caught by a test failure,
		// not silently.
		if strings.Contains(s, "schema: 1\n\n") {
			t.Fatalf("expected yaml.v3 to drop the blank line (documented divergence); got it preserved:\n%s", s)
		}
		got, err := decodeManifestSkills(t, s)
		if err != nil {
			t.Fatalf("rewritten pack.yaml no longer parses: %v", err)
		}
		if len(got) != 1 || got[0].Name != "new" {
			t.Fatalf("appended entry missing or wrong: %+v", got)
		}
	})

	t.Run("folded block scalar reflows but stays semantically identical", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "pack.yaml")
		orig := "schema: 1\nname: acme\nversion: 1.0.0\ndescription: >\n  folded\n  scalar\n  text\n"
		if err := os.WriteFile(p, []byte(orig), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := appendSkillEntries(p, newEntry); err != nil {
			t.Fatal(err)
		}
		out, _ := os.ReadFile(p)
		s := string(out)
		// Documented divergence: yaml.v3 re-flows a ">" folded scalar onto
		// one physical line. A folded scalar's MEANING is "words joined by
		// single spaces" regardless of how many source lines it spanned, so
		// this is not a content change — only a test that it still decodes
		// to the same string proves that, not a substring check on layout.
		var m struct {
			Description string `yaml:"description"`
		}
		if err := yamlUnmarshalLoose([]byte(s), &m); err != nil {
			t.Fatal(err)
		}
		if m.Description != "folded scalar text\n" {
			t.Fatalf("folded scalar content changed meaning: got %q", m.Description)
		}
		got, err := decodeManifestSkills(t, s)
		if err != nil {
			t.Fatalf("rewritten pack.yaml no longer parses: %v", err)
		}
		if len(got) != 1 || got[0].Name != "new" {
			t.Fatalf("appended entry missing or wrong: %+v", got)
		}
	})

	t.Run("empty skills-less file gains a skills key", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "pack.yaml")
		if err := os.WriteFile(p, []byte("schema: 1\nname: acme\nversion: 1.0.0\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := appendSkillEntries(p, newEntry); err != nil {
			t.Fatal(err)
		}
		out, _ := os.ReadFile(p)
		got, err := decodeManifestSkills(t, string(out))
		if err != nil {
			t.Fatalf("rewritten pack.yaml no longer parses: %v", err)
		}
		if len(got) != 1 || got[0].Name != "new" {
			t.Fatalf("appended entry missing or wrong: %+v", got)
		}
	})
}

func decodeManifestSkills(t *testing.T, doc string) ([]pack.SkillEntry, error) {
	t.Helper()
	var m struct {
		Skills []pack.SkillEntry `yaml:"skills"`
	}
	return m.Skills, yamlUnmarshalLoose([]byte(doc), &m)
}

func yamlUnmarshalLoose(b []byte, v any) error { return yaml.Unmarshal(b, v) }
