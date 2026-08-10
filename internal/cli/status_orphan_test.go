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
//
// Ownership (apply.go's ownedSkillPath, mirrored by status.go) is "exactly
// one path element below .claude/skills" — the parent-dir rule that replaced
// the old "esc-" name-prefix marker so a name-overridden skill (Task 1/2)
// can retire cleanly. A dir nested one level deeper than that is still never
// owned regardless of name, which is the scenario this test drives.
func TestStatusOrphanDirNotOwnedMatchesApplyRefusal(t *testing.T) {
	root := setupGovernedRepo(t)
	runEsc(t, root, "sync")
	const artPath = ".claude/skills/esc-acme/team-notes"
	writeFiles(t, root, map[string]string{artPath + "/notes.md": "ours\n"})
	lock, err := lockfile.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	lock.Artifacts = append(lock.Artifacts, lockfile.LockArtifact{
		Path: artPath, Kind: engine.KindDir,
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
		if f.Subject != artPath {
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
	if _, code := runEscOut(t, root, "sync"); code != 4 {
		t.Fatalf("Apply must still refuse this entry with exit 4, got %d", code)
	}
}

// TestStatusOrphanDirHashMismatchAdvisesForceMatchingApplySkip: a dir
// exactly one level below .claude/skills IS escapement-owned under the new
// parent-dir rule, even with no "esc-" prefix — the whole point of Task 3.
// This used to be indistinguishable from the not-owned case above (both hit
// the same prefix check and refused). Now ownership passes, and the
// recorded-hash gate (SkipOrphanDirEdited) is what stays fail-closed: it
// declines the retirement (skip, exit 0, entry carried forward) because the
// lockfile's hash does not match what's on disk. Status's advice must match
// that real behavior — `esc sync --force` removes it, plain `esc sync`
// does not — never the "refuses" wording, which no longer applies here.
func TestStatusOrphanDirHashMismatchAdvisesForceMatchingApplySkip(t *testing.T) {
	root := setupGovernedRepo(t)
	runEsc(t, root, "sync")
	const artPath = ".claude/skills/team-notes"
	writeFiles(t, root, map[string]string{artPath + "/notes.md": "ours\n"})
	lock, err := lockfile.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	lock.Artifacts = append(lock.Artifacts, lockfile.LockArtifact{
		Path: artPath, Kind: engine.KindDir,
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
		if f.Subject != artPath {
			continue
		}
		found = true
		if f.Managed != "orphan" {
			t.Errorf("managed = %q, want orphan", f.Managed)
		}
		if strings.Contains(f.Detail, "refuses") {
			t.Errorf("status must not claim Apply refuses this entry: %q", f.Detail)
		}
		if !strings.Contains(f.Detail, "--force") {
			t.Errorf("detail must advise --force, matching Apply's real skip behavior, got %q", f.Detail)
		}
	}
	if !found {
		t.Fatal("no finding for the hash-mismatched owned lockfile dir entry")
	}
	// Fail-closed either way: a force-less sync must not delete the file.
	if _, code := runEscOut(t, root, "sync"); code != 0 {
		t.Fatalf("Apply must skip (exit 0), not refuse, an owned dir with a stale hash")
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(artPath), "notes.md")); err != nil {
		t.Fatalf("hash-mismatched pack-provided file must survive a force-less sync: %v", err)
	}
}
