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

// TestSyncAdoptsIdenticalDirAtTarget covers the narrow exception to the
// occupied gate: a pre-existing, unmanaged target directory that is
// byte-for-byte identical to what the pack would have written is adopted —
// lock entry recorded, nothing written to disk, one stderr notice — rather
// than skipped. This is what lets a sync interrupted between writing a dir
// and saving the lockfile converge cleanly on the next run instead of
// dead-ending as permanently "occupied" with --force excluded from
// resolving it.
func TestSyncAdoptsIdenticalDirAtTarget(t *testing.T) {
	root := t.TempDir()
	src := writeLocalPack(t, root, "policy", "acme",
		"skills:\n  - path: skills/brainstorming\n    name: brainstorming\n")
	writeFiles(t, root, map[string]string{
		".escapement/config.yaml": localConfig(src),
		// Byte-identical to writeLocalPack's own skill fixture content.
		".claude/skills/brainstorming/SKILL.md": "---\nname: brainstorming\n---\n\nUpstream method.\n",
	})
	out, code := runEscOut(t, root, "sync")
	if code != 0 {
		t.Fatalf("esc sync exited %d:\n%s", code, out)
	}
	if !strings.Contains(out, "adopted .claude/skills/brainstorming") {
		t.Fatalf("esc sync printed no adoption notice:\n%s", out)
	}
	if strings.Contains(out, "skipped .claude/skills/brainstorming") {
		t.Fatalf("esc sync skipped an identical directory instead of adopting it:\n%s", out)
	}
	lock, err := os.ReadFile(filepath.Join(root, ".escapement", "escapement.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(lock), ".claude/skills/brainstorming") {
		t.Fatalf("lockfile carries no entry for the adopted directory:\n%s", lock)
	}
	// Second sync converges: the directory is now tracked and in sync.
	out, code = runEscOut(t, root, "status")
	if code != 0 {
		t.Fatalf("status after adoption exited %d:\n%s", code, out)
	}
	if strings.Contains(out, "occupied") {
		t.Fatalf("status still reports occupied after adoption:\n%s", out)
	}
	if !strings.Contains(out, "✓ .claude/skills/brainstorming") {
		t.Fatalf("status did not report the adopted directory in sync:\n%s", out)
	}
}
