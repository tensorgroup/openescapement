package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readConfig(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, ".escapement", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestInitWritesRulesInExplicitList(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := run(t, root, "init", "--yes"); code != 0 {
		t.Fatalf("init: %d\n%s", code, out)
	}
	if !strings.Contains(readConfig(t, root), "- rules") {
		t.Errorf("init must add rules to an explicit targets list:\n%s", readConfig(t, root))
	}
}

func TestInitPrefillsAgentsSkillsForRealDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agents", "skills", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".agents", "skills", "x", "SKILL.md"), []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := run(t, root, "init", "--yes"); code != 0 {
		t.Fatalf("init: %d\n%s", code, out)
	}
	if !strings.Contains(readConfig(t, root), "agents-skills") {
		t.Errorf("init must pre-fill agents-skills for a real .agents/skills dir")
	}
}

func TestInitSkipsSymlinkedAgentsSkills(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude", "skills", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, ".claude", "skills"), filepath.Join(root, ".agents", "skills")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if code, out := run(t, root, "init", "--yes"); code != 0 {
		t.Fatalf("init: %d\n%s", code, out)
	}
	if strings.Contains(readConfig(t, root), "agents-skills") {
		t.Errorf("init must NOT pre-fill agents-skills for a symlinked .agents/skills")
	}
}
