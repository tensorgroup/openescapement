package cli

import (
	"strings"
	"testing"
)

// setupDupHome syncs a named local-pack skill, then plants a copy of it under
// a fake $HOME/.claude/skills. os.UserHomeDir honors $HOME on unix.
func setupDupHome(t *testing.T, mutate bool) string {
	t.Helper()
	root := t.TempDir()
	src := writeLocalPack(t, root, "policy", "acme",
		"skills:\n  - path: skills/brainstorming\n    name: brainstorming\n")
	writeFiles(t, root, map[string]string{".escapement/config.yaml": localConfig(src)})
	runEsc(t, root, "sync")
	home := t.TempDir()
	t.Setenv("HOME", home)
	content := "---\nname: brainstorming\n---\n\nUpstream method.\n"
	if mutate {
		content += "user's local tweak\n"
	}
	writeFiles(t, home, map[string]string{".claude/skills/brainstorming/SKILL.md": content})
	return root
}

func TestStatusReportsUserLevelDuplicate(t *testing.T) {
	root := setupDupHome(t, false)
	out, code := runEscOut(t, root, "status")
	if code != 0 {
		t.Fatalf("informational finding must not change the exit code, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "also installed at user level (same content)") {
		t.Fatalf("missing duplicate notice:\n%s", out)
	}
	if _, code := runEscOut(t, root, "status", "--check"); code != 0 {
		t.Fatal("duplicate finding must never gate --check")
	}
}

func TestStatusReportsUserLevelDuplicateDiffers(t *testing.T) {
	root := setupDupHome(t, true)
	out, _ := runEscOut(t, root, "status")
	if !strings.Contains(out, "also installed at user level (differs)") {
		t.Fatalf("missing differs notice:\n%s", out)
	}
}

func TestStatusJSONDuplicateIsMetricsGradeOnly(t *testing.T) {
	root := setupDupHome(t, true)
	out, _ := runEscOut(t, root, "status", "--json")
	if !strings.Contains(out, `"duplicate"`) || !strings.Contains(out, `"same": false`) {
		t.Fatalf("json report missing duplicate payload:\n%s", out)
	}
	if strings.Contains(out, "user's local tweak") {
		t.Fatalf("duplicate finding leaked skill content:\n%s", out)
	}
}
