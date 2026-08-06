package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/tensorgroup/openescapement/internal/engine"
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
	for _, f := range st.Findings {
		if f.Path == "AGENTS.md" && f.State != engine.Altered {
			t.Errorf("state = %q, want altered after a skip", f.State)
		}
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
