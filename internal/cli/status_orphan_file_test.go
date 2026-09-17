package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/engine"
	"github.com/tensorgroup/openescapement/internal/lockfile"
)

// retireGovernance edits root's config.yaml to select an explicit target
// list that omits governance, so the next Plan (and therefore Status) no
// longer desires GOVERNANCE.md. This is the real-config equivalent of "the
// pack stopped providing it" that makes GOVERNANCE.md's existing lockfile
// entry an orphan, the same way dropping a rule from a pack orphans a rule
// file — without it there is no way to exercise Status's orphan-file pass
// against the one built-in whole-file target, since governance is otherwise
// always in the default (empty targets:) effective set.
func retireGovernance(t *testing.T, root string) {
	t.Helper()
	p := filepath.Join(root, ".escapement", "config.yaml")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	s += "targets:\n  - claude\n  - agents\n  - gemini\n  - mcp\n  - skills\n"
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

type orphanFinding struct {
	Subject    string `json:"subject"`
	Managed    string `json:"managed"`
	Detail     string `json:"detail"`
	Alteration *struct {
		ExpectedHash string `json:"expected_hash"`
		ActualHash   string `json:"actual_hash"`
	} `json:"alteration"`
}

func findOrphan(t *testing.T, out, subject string) *orphanFinding {
	t.Helper()
	var report struct {
		Findings []orphanFinding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("status --json did not parse: %v\n%s", err, out)
	}
	for i := range report.Findings {
		if report.Findings[i].Subject == subject {
			return &report.Findings[i]
		}
	}
	return nil
}

// TestStatusOrphanGovernanceReportsOrphan: (a) a retired GOVERNANCE.md whose
// lockfile entry still exists, but whose artifact Plan no longer produces,
// is reported as an unedited orphan — the KindFile mirror of the existing
// orphan-block/orphan-dir coverage, exercising status.go's new orphan-file
// pass (and, underneath it, inspectOrphanFile) end to end in a real repo.
func TestStatusOrphanGovernanceReportsOrphan(t *testing.T) {
	root := setupGovernedRepo(t)
	runEsc(t, root, "sync")
	if _, err := os.Stat(filepath.Join(root, "GOVERNANCE.md")); err != nil {
		t.Fatalf("precondition: GOVERNANCE.md must exist after the first sync: %v", err)
	}
	retireGovernance(t, root)

	out, code := runEscOut(t, root, "status", "--json")
	if code != 0 && code != 1 {
		t.Fatalf("status exit %d, want 0 or 1:\n%s", code, out)
	}
	f := findOrphan(t, out, "GOVERNANCE.md")
	if f == nil {
		t.Fatal("no orphan finding for GOVERNANCE.md after it left the effective set")
	}
	if f.Managed != "orphan" {
		t.Errorf("managed = %q, want orphan", f.Managed)
	}
	if f.Alteration != nil {
		t.Errorf("unedited orphan must not carry an alteration: %+v", f.Alteration)
	}
	if !strings.Contains(f.Detail, "esc sync") {
		t.Errorf("detail should point at `esc sync`, got %q", f.Detail)
	}
}

// TestStatusOrphanGovernanceEditedSetsAlteration: (b) the same retired
// GOVERNANCE.md, hand-edited since the last sync, reports orphan with
// Alteration set and advises --force rather than plain sync.
func TestStatusOrphanGovernanceEditedSetsAlteration(t *testing.T) {
	root := setupGovernedRepo(t)
	runEsc(t, root, "sync")
	retireGovernance(t, root)
	if err := os.WriteFile(filepath.Join(root, "GOVERNANCE.md"), []byte("hand edited after retirement\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := runEscOut(t, root, "status", "--json")
	if code != 0 && code != 1 {
		t.Fatalf("status exit %d, want 0 or 1:\n%s", code, out)
	}
	f := findOrphan(t, out, "GOVERNANCE.md")
	if f == nil {
		t.Fatal("no orphan finding for the hand-edited, retired GOVERNANCE.md")
	}
	if f.Managed != "orphan" {
		t.Errorf("managed = %q, want orphan", f.Managed)
	}
	if f.Alteration == nil {
		t.Fatal("a hand-edited orphan file must set Alteration")
	}
	if !strings.Contains(f.Detail, "--force") {
		t.Errorf("detail must advise --force for a hand-edited orphan, got %q", f.Detail)
	}
	// Fail-closed either way: a force-less sync must not delete a hand-edit.
	if _, code := runEscOut(t, root, "sync"); code != 0 {
		t.Fatalf("a force-less sync must skip (exit 0), not fail, a hand-edited orphan file")
	}
	if _, err := os.Stat(filepath.Join(root, "GOVERNANCE.md")); err != nil {
		t.Fatalf("hand-edited orphan file must survive a force-less sync: %v", err)
	}
}

// TestStatusOrphanFileUnownedLockEntryReported: (c) a lockfile KindFile entry
// naming a path escapement does not own (ownedFilePath false) is reported as
// an orphan status cannot ask sync to remove, matching Apply's refusal on
// the same entry.
func TestStatusOrphanFileUnownedLockEntryReported(t *testing.T) {
	root := setupGovernedRepo(t)
	runEsc(t, root, "sync")
	lock, err := lockfile.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	lock.Artifacts = append(lock.Artifacts, lockfile.LockArtifact{
		Path: "docs/GOVERNANCE.md", Kind: engine.KindFile, Hash: "sha256:irrelevant",
	})
	if err := lock.Save(root); err != nil {
		t.Fatal(err)
	}

	out, code := runEscOut(t, root, "status", "--json")
	if code != 0 && code != 1 {
		t.Fatalf("status exit %d, want 0 or 1:\n%s", code, out)
	}
	f := findOrphan(t, out, "docs/GOVERNANCE.md")
	if f == nil {
		t.Fatal("no finding for the unowned lockfile file entry")
	}
	if f.Managed != "orphan" {
		t.Errorf("managed = %q, want orphan", f.Managed)
	}
	if !strings.Contains(f.Detail, "does not own") || !strings.Contains(f.Detail, "refuses") {
		t.Errorf("detail must say escapement does not own this path and sync refuses it, got %q", f.Detail)
	}
	if _, code := runEscOut(t, root, "sync"); code != 4 {
		t.Fatalf("Apply must refuse this entry with exit 4, matching status's advice, got %d", code)
	}
}

// TestStatusOrphanGovernanceSymlinkFailsClosed: (d) a retired GOVERNANCE.md
// replaced by a symlink makes Status return an error (exit 4) rather than a
// clean report — the whole-file orphan pass propagates a containment or
// symlink failure instead of skipping it, unlike the block and dir passes
// (see TestStatusOrphanBlockRefusesSymlinkedPath and
// TestStatusOrphanDirSymlinkedPathStillContinues below).
func TestStatusOrphanGovernanceSymlinkFailsClosed(t *testing.T) {
	root := setupGovernedRepo(t)
	runEsc(t, root, "sync")
	retireGovernance(t, root)

	govPath := filepath.Join(root, "GOVERNANCE.md")
	if err := os.Remove(govPath); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(root), "outside-governance.md")
	if err := os.WriteFile(outside, []byte("not escapement's\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, govPath); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	out, code := runEscOut(t, root, "status", "--json")
	if code != 4 {
		t.Fatalf("status must fail closed (exit 4) on a symlinked orphan whole-file target, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "symlink") {
		t.Errorf("error should name the symlink refusal, got:\n%s", out)
	}
	if strings.Contains(out, `"findings"`) {
		t.Errorf("nothing may be reported after a containment failure, got:\n%s", out)
	}
}

// TestStatusOrphanDirSymlinkedPathStillContinues pins the deliberate
// divergence TestStatusOrphanGovernanceSymlinkFailsClosed relies on: unlike
// the whole-file orphan pass, the orphan skill-directory pass is a read-only
// advisory surface that skips a symlinked lockfile path silently (`continue`
// in status.go) rather than propagating. Both passes now share
// inspectOrphanFile's sibling containment logic only for the file case; this
// guards against a future refactor accidentally making the dir pass
// propagate too, which would be a behavior change no plan asked for.
func TestStatusOrphanDirSymlinkedPathStillContinues(t *testing.T) {
	root := setupGovernedRepo(t)
	runEsc(t, root, "sync")

	outside := filepath.Join(filepath.Dir(root), "outside-dir")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	dirPath := filepath.Join(root, ".claude", "skills", "esc-acme-retired")
	if err := os.MkdirAll(filepath.Dir(dirPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, dirPath); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	lock, err := lockfile.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	lock.Artifacts = append(lock.Artifacts, lockfile.LockArtifact{
		Path: ".claude/skills/esc-acme-retired", Kind: engine.KindDir, Hash: "irrelevant",
	})
	if err := lock.Save(root); err != nil {
		t.Fatal(err)
	}

	out, code := runEscOut(t, root, "status", "--json")
	if code != 0 && code != 1 {
		t.Fatalf("the dir pass must still continue past a symlinked lockfile path (exit 0 or 1), got %d:\n%s", code, out)
	}
}
