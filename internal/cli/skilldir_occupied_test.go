package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupOccupied(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	src := writeLocalPack(t, root, "policy", "acme",
		"skills:\n  - path: skills/brainstorming\n    name: brainstorming\n")
	writeFiles(t, root, map[string]string{
		".escapement/config.yaml":               localConfig(src),
		".claude/skills/brainstorming/SKILL.md": "the user's own hand-installed copy\n",
	})
	return root
}

func TestSyncSkipsUnmanagedDirAtTarget(t *testing.T) {
	root := setupOccupied(t)
	for _, args := range [][]string{{"sync"}, {"sync", "--force"}} {
		out, code := runEscOut(t, root, args...)
		if code != 0 {
			t.Fatalf("esc %v exited %d:\n%s", args, code, out)
		}
		if !strings.Contains(out, "skipped .claude/skills/brainstorming") {
			t.Fatalf("esc %v printed no skip warning:\n%s", args, out)
		}
		got, err := os.ReadFile(filepath.Join(root, ".claude", "skills", "brainstorming", "SKILL.md"))
		if err != nil || string(got) != "the user's own hand-installed copy\n" {
			t.Fatalf("esc %v touched the unmanaged directory: %q, %v", args, got, err)
		}
	}
	// No adoption: the lockfile must carry no entry for the occupied path.
	lock, err := os.ReadFile(filepath.Join(root, ".escapement", "escapement.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(lock), ".claude/skills/brainstorming") {
		t.Fatalf("lockfile adopted an unmanaged directory:\n%s", lock)
	}
}

func TestStatusReportsOccupiedAndCheckGates(t *testing.T) {
	root := setupOccupied(t)
	runEscOut(t, root, "sync")
	out, code := runEscOut(t, root, "status")
	if code != 0 {
		t.Fatalf("bare status must exit 0 on drift-family findings, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "occupied") {
		t.Fatalf("status did not report the occupied state:\n%s", out)
	}
	if _, code := runEscOut(t, root, "status", "--check"); code != 1 {
		t.Fatalf("status --check must gate on occupied, got %d", code)
	}
}
