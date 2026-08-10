package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeLocalPack writes a minimal unsigned local pack under root/<dir> with
// one named skill, and returns the pack-relative source ("./<dir>").
func writeLocalPack(t *testing.T, root, dir, packName, skillYAML string) string {
	t.Helper()
	writeFiles(t, root, map[string]string{
		dir + "/pack.yaml":                     "schema: 1\nname: " + packName + "\nversion: 1.0.0\n" + skillYAML,
		dir + "/skills/brainstorming/SKILL.md": "---\nname: brainstorming\n---\n\nUpstream method.\n",
	})
	return "./" + dir
}

func localConfig(sources ...string) string {
	var b strings.Builder
	b.WriteString("schema: 1\npacks:\n")
	for _, s := range sources {
		b.WriteString("  - source: " + s + "\n    ref: \"\"\n    trust: unsigned\n")
	}
	return b.String()
}

func TestNamedSkillSyncsToUnprefixedPath(t *testing.T) {
	root := t.TempDir()
	src := writeLocalPack(t, root, "policy", "acme",
		"skills:\n  - path: skills/brainstorming\n    name: brainstorming\n")
	writeFiles(t, root, map[string]string{".escapement/config.yaml": localConfig(src)})
	runEsc(t, root, "sync")
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "brainstorming", "SKILL.md")); err != nil {
		t.Fatalf("named skill did not land at upstream name: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "esc-acme-brainstorming")); !os.IsNotExist(err) {
		t.Fatalf("prefixed directory must not exist for a named entry")
	}
}

func TestCrossPackSkillPathCollisionFailsPlan(t *testing.T) {
	root := t.TempDir()
	a := writeLocalPack(t, root, "pa", "alpha",
		"skills:\n  - path: skills/brainstorming\n    name: brainstorming\n")
	b := writeLocalPack(t, root, "pb", "beta",
		"skills:\n  - path: skills/brainstorming\n    name: brainstorming\n")
	writeFiles(t, root, map[string]string{".escapement/config.yaml": localConfig(a, b)})
	out, code := runEscOut(t, root, "sync")
	if code != 2 {
		t.Fatalf("collision must be a config error (exit 2), got %d:\n%s", code, out)
	}
	if !strings.Contains(out, ".claude/skills/brainstorming") ||
		!strings.Contains(out, "alpha") || !strings.Contains(out, "beta") {
		t.Fatalf("error must name the path and both packs:\n%s", out)
	}
}

func TestNamedSkillRetiresCleanly(t *testing.T) {
	root := t.TempDir()
	src := writeLocalPack(t, root, "policy", "acme",
		"skills:\n  - path: skills/brainstorming\n    name: brainstorming\n")
	writeFiles(t, root, map[string]string{".escapement/config.yaml": localConfig(src)})
	runEsc(t, root, "sync")
	// Drop the skill from the pack and sync again: the unprefixed directory
	// must retire exactly like an esc- prefixed one — via the manifest-file
	// removal pass, not a refusal.
	writeFiles(t, root, map[string]string{
		"policy/pack.yaml": "schema: 1\nname: acme\nversion: 1.0.1\n",
	})
	runEsc(t, root, "sync")
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "brainstorming")); !os.IsNotExist(err) {
		t.Fatalf("retired named skill dir still present (err=%v)", err)
	}
}
