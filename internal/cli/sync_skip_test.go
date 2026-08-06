package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/tensorgroup/openescapement/internal/engine"
	"github.com/tensorgroup/openescapement/internal/lockfile"
)

// TestSyncSkipsAlteredBlock confirms sync leaves a hand-edited managed block
// exactly as the human left it, converges everything else, warns, and still
// exits 0 — a repo that declines part of a policy update must not fail its
// own rollout. It also confirms the lockfile entry for the skipped artifact
// does not advance: classify (internal/engine/status.go) distinguishes stale
// from altered by comparing the on-disk hash against locked.Hash, so an
// entry that advanced past what was actually written would make the
// alteration vanish from the next status run.
//
// "Use Vault." (from rules/secrets.md, packRepoFiles in cli_test.go) is used
// as the hand-edit target rather than the "Be kind" text that
// TestAmendedBlockReportsInSync uses: that text is appended *outside* the
// managed block as a local amendment, whereas this test needs to edit
// *inside* the managed block to trigger Altered.
func TestSyncSkipsAlteredBlock(t *testing.T) {
	repo := setupGovernedRepo(t)
	runEsc(t, repo, "sync")

	path := filepath.Join(repo, "AGENTS.md")
	orig, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := bytes.Replace(orig, []byte("Use Vault"), []byte("Use Something Else"), 1)
	if bytes.Equal(orig, edited) {
		t.Fatal("precondition: fixture text not found")
	}
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	lockBefore, err := os.ReadFile(filepath.Join(repo, ".escapement", "escapement.lock"))
	if err != nil {
		t.Fatal(err)
	}

	out, code := runEscOut(t, repo, "sync")
	if code != 0 {
		t.Errorf("exit = %d, want 0; a declined artifact must not fail a rollout\n%s", code, out)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, edited) {
		t.Error("sync overwrote a hand-edited block")
	}
	lockAfter, err := os.ReadFile(filepath.Join(repo, ".escapement", "escapement.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(lockBefore, lockAfter) {
		t.Error("lock entry for a skipped artifact must not advance")
	}

	st, err := engine.Status(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range st.Findings {
		if f.Path != "AGENTS.md" {
			continue
		}
		found = true
		if f.State != engine.Altered {
			t.Errorf("state = %q, want altered after a skip", f.State)
		}
	}
	if !found {
		t.Fatal("no finding for AGENTS.md: assertion above proved nothing")
	}
}

// TestSyncForceOverwritesAlteredBlockKeepingAmendment confirms --force
// converges the managed block (our content) but never removes a local
// amendment living outside it (their content) — force is about escapement's
// own region, not the team's.
func TestSyncForceOverwritesAlteredBlockKeepingAmendment(t *testing.T) {
	repo := setupGovernedRepo(t)
	runEsc(t, repo, "sync")
	path := filepath.Join(repo, "AGENTS.md")
	orig, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := bytes.Replace(orig, []byte("Use Vault"), []byte("Use Something Else"), 1)
	if bytes.Equal(orig, edited) {
		t.Fatal("precondition: fixture text not found")
	}
	edited = append(edited, []byte("\n## Team rules\n\nours\n")...)
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatal(err)
	}

	runEsc(t, repo, "sync", "--force")

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(after, []byte("Use Something Else")) {
		t.Error("--force must converge the managed block")
	}
	if !bytes.Contains(after, []byte("## Team rules")) {
		t.Error("--force must not remove local amendments")
	}
}

// TestSyncCarriesForwardLockAcrossPackUpdate proves the carry-forward claim
// on a run where it can actually be distinguished from a no-op.
// TestSyncSkipsAlteredBlock syncs against an unchanged pack, so lockfile
// byte-equality there cannot tell "we correctly carried the previous entry
// forward" apart from "nothing moved anyway". Here the pack pin genuinely
// advances (a new tag, new rules content, a new rendered hash for
// AGENTS.md) while AGENTS.md stays hand-edited, so the only way the skipped
// artifact's lock entry can still hold the pre-update hash is if Apply
// actually carried it forward instead of recomputing it against the new
// plan.
//
// The new version is published under a new annotated tag (matching
// TestUpdateAndDiffAgainst / TestSkillDirRemovesPackDroppedFiles), not a
// force-retag: dropSkillFileFromPack's `tag -f` exists to exercise the
// moved-tag security check, which is a different concern from this test.
func TestSyncCarriesForwardLockAcrossPackUpdate(t *testing.T) {
	packRepo := newPackRepo(t, "1.0.0")
	repo := newGoverned(t, packRepo, "v1.0.0")
	runEsc(t, repo, "sync")

	lockBefore, err := lockfile.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	before := lockBefore.Artifact("AGENTS.md")
	if before == nil {
		t.Fatal("precondition: AGENTS.md not in lock after first sync")
	}
	hashBeforeUpdate := before.Hash

	// Hand-edit the managed block.
	path := filepath.Join(repo, "AGENTS.md")
	orig, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := bytes.Replace(orig, []byte("Use Vault"), []byte("Use Something Else"), 1)
	if bytes.Equal(orig, edited) {
		t.Fatal("precondition: fixture text not found")
	}
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatal(err)
	}

	// Publish v1.0.1: packRepoFiles bakes the version string into
	// rules/secrets.md ("Use Vault. Version " + version), so this genuinely
	// changes AGENTS.md's rendered hash, not just the pack's own version
	// pin.
	writeFiles(t, packRepo, packRepoFiles("1.0.1"))
	gitIn(t, packRepo, "add", "-A")
	gitIn(t, packRepo, "commit", "-m", "v1.0.1")
	gitIn(t, packRepo, "tag", "-a", "v1.0.1", "-m", "v1.0.1")

	if out, code := runEscOut(t, repo, "update", "--ref", "v1.0.1"); code != 0 {
		t.Fatalf("update --ref v1.0.1: exit %d\n%s", code, out)
	}
	out, code := runEscOut(t, repo, "sync")
	if code != 0 {
		t.Fatalf("sync after pack update exited %d:\n%s", code, out)
	}

	lockAfter, err := lockfile.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	advanced := false
	for _, p := range lockAfter.Packs {
		if p.Ref == "v1.0.1" {
			advanced = true
		}
	}
	if !advanced {
		t.Errorf("pack pin did not advance to v1.0.1: %+v", lockAfter.Packs)
	}
	after := lockAfter.Artifact("AGENTS.md")
	if after == nil {
		t.Fatal("AGENTS.md missing from lock after sync")
	}
	if after.Hash != hashBeforeUpdate {
		t.Errorf("skipped artifact's lock hash advanced across a pack update: before %s, after %s", hashBeforeUpdate, after.Hash)
	}

	st, err := engine.Status(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range st.Findings {
		if f.Path != "AGENTS.md" {
			continue
		}
		found = true
		if f.State != engine.Altered {
			t.Errorf("state = %q, want altered after a skip across a pack update", f.State)
		}
	}
	if !found {
		t.Fatal("no finding for AGENTS.md: assertion above proved nothing")
	}
}
