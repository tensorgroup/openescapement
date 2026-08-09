package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/engine"
	"github.com/tensorgroup/openescapement/internal/lockfile"
	"github.com/tensorgroup/openescapement/internal/shapetest"
)

// TestStatusOrphanBlockRefusesEscapingLockPath: the orphan-block status
// pass read lockfile paths with no containment or symlink check, the same
// class as the Critical fixed on the dir pass. A hostile entry must not be
// read at all: the read-only surface skips silently, and Apply failing
// closed on the same entry is the signal that case gets (same ruling as
// the dir pass).
func TestStatusOrphanBlockRefusesEscapingLockPath(t *testing.T) {
	root := setupGovernedRepo(t)
	runEsc(t, root, "sync")
	victim := filepath.Join(filepath.Dir(root), "victim.md")
	if err := os.WriteFile(victim, shapetest.RenderedBlock("outside content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lock, err := lockfile.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	lock.Artifacts = append(lock.Artifacts, lockfile.LockArtifact{
		Path: "../victim.md", Kind: engine.KindBlock, Hash: "irrelevant",
	})
	if err := lock.Save(root); err != nil {
		t.Fatal(err)
	}
	out, code := runEscOut(t, root, "status", "--json")
	if code != 0 && code != 1 {
		t.Fatalf("status exit %d:\n%s", code, out)
	}
	if strings.Contains(out, "victim.md") {
		t.Errorf("status followed a lockfile path out of the repo to build a report:\n%s", out)
	}
}

// TestStatusOrphanBlockRefusesSymlinkedPath: same rule through a symlink.
func TestStatusOrphanBlockRefusesSymlinkedPath(t *testing.T) {
	root := setupGovernedRepo(t)
	runEsc(t, root, "sync")
	outside := filepath.Join(filepath.Dir(root), "outside.md")
	if err := os.WriteFile(outside, shapetest.RenderedBlock("outside content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "NOTES.md")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	lock, err := lockfile.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	lock.Artifacts = append(lock.Artifacts, lockfile.LockArtifact{
		Path: "NOTES.md", Kind: engine.KindBlock, Hash: "irrelevant",
	})
	if err := lock.Save(root); err != nil {
		t.Fatal(err)
	}
	out, code := runEscOut(t, root, "status", "--json")
	if code != 0 && code != 1 {
		t.Fatalf("status exit %d:\n%s", code, out)
	}
	if strings.Contains(out, "NOTES.md") {
		t.Errorf("status read through a symlink to build a report:\n%s", out)
	}
}

// TestStatusOrphanDirNotOwnedMatchesApplyRefusal: status used to advise
// `esc sync --force` for a lockfile dir entry Apply refuses to touch as
// not escapement-owned (exit 4). Advice must describe what sync will
// actually do.
func TestStatusOrphanDirNotOwnedMatchesApplyRefusal(t *testing.T) {
	root := setupGovernedRepo(t)
	runEsc(t, root, "sync")
	writeFiles(t, root, map[string]string{".claude/skills/team-notes/notes.md": "ours\n"})
	lock, err := lockfile.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	lock.Artifacts = append(lock.Artifacts, lockfile.LockArtifact{
		Path: ".claude/skills/team-notes", Kind: engine.KindDir,
		Hash: "irrelevant", Files: []string{"notes.md"},
	})
	if err := lock.Save(root); err != nil {
		t.Fatal(err)
	}
	out, _ := runEscOut(t, root, "status", "--json")
	var report struct {
		Findings []struct {
			Subject string `json:"subject"`
			Managed string `json:"managed"`
			Detail  string `json:"detail"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("status --json did not parse: %v\n%s", err, out)
	}
	var found bool
	for _, f := range report.Findings {
		if f.Subject != ".claude/skills/team-notes" {
			continue
		}
		found = true
		if f.Managed != "orphan" {
			t.Errorf("managed = %q, want orphan", f.Managed)
		}
		if strings.Contains(f.Detail, "--force") {
			t.Errorf("status advises --force where Apply refuses: %q", f.Detail)
		}
		if !strings.Contains(f.Detail, "refuses") {
			t.Errorf("detail must say sync refuses, got %q", f.Detail)
		}
	}
	if !found {
		t.Fatal("no finding for the non-owned lockfile dir entry; sync will exit 4 on it and status said nothing")
	}
}
