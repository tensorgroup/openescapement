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

func decodeManifestSkills(t *testing.T, doc string) ([]pack.SkillEntry, error) {
	t.Helper()
	var m struct {
		Skills []pack.SkillEntry `yaml:"skills"`
	}
	return m.Skills, yamlUnmarshalLoose([]byte(doc), &m)
}

func yamlUnmarshalLoose(b []byte, v any) error { return yaml.Unmarshal(b, v) }
