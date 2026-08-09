package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/engine"
)

// TestSkipHintPerCause pins the exact remedy line printed under each kind of
// declined artifact. `esc sync` used to print one blanket line after every
// skip — "`esc diff` to inspect · `esc sync --force` to overwrite" — which is
// true only of a hand-edited artifact still in the effective set.
// engine.PopulateDiffs populates diffs for Altered findings only, so for
// either orphan decline `esc diff` prints nothing whatsoever, and for the
// unmanaged-file decline --force is wrong on top of that: force is consent to
// overwrite escapement's own content, never the team's.
func TestSkipHintPerCause(t *testing.T) {
	cases := []struct {
		cause engine.SkipCause
		want  string
	}{
		{engine.SkipHandEdited, "`esc diff` to inspect · `esc sync --force` to overwrite"},
		{engine.SkipOrphanDirUnmanaged, "the pack files are gone and the rest is yours · delete that directory to be rid of it"},
		// Deliberately not "revert the edit": this cause also fires when a
		// pack-provided file was deleted or became unreadable, and there is
		// nothing to revert then.
		{engine.SkipOrphanDirEdited, "nothing was removed · restore the pack files as synced, or `esc sync --force` to retire the directory"},
		{engine.SkipOrphanBlockEdited, "the block is still in that file · `esc sync --force` to remove it"},
	}
	seen := map[string]engine.SkipCause{}
	for _, c := range cases {
		got := skipHint(engine.Skipped{Cause: c.cause})
		if got != c.want {
			t.Errorf("skipHint(%q) =\n  %q\nwant\n  %q", c.cause, got, c.want)
		}
		if other, dup := seen[got]; dup {
			t.Errorf("causes %q and %q share a hint, so the discriminator buys nothing: %q", other, c.cause, got)
		}
		seen[got] = c.cause
		// The whole point of the change: only the one cause `esc diff` can
		// actually show may send the user there.
		if strings.Contains(got, "esc diff") && c.cause != engine.SkipHandEdited {
			t.Errorf("cause %q points at `esc diff`, which shows nothing for it: %q", c.cause, got)
		}
	}
	// An unrecognized cause falls back to the hand-edit line rather than
	// printing nothing. Documented, not incidental: a decline with no remedy
	// at all is worse than a slightly wrong one.
	if got := skipHint(engine.Skipped{Cause: "not-a-real-cause"}); got != cases[0].want {
		t.Errorf("unknown cause fallback = %q, want the hand-edit line", got)
	}
}

// TestSyncSkipRendersHintUnderEachSkip pins the rendered shape: every skip
// gets its own remedy line, indented under it, rather than one shared line
// printed once at the end for all of them.
func TestSyncSkipRendersHintUnderEachSkip(t *testing.T) {
	repo := setupGovernedRepo(t)
	runEsc(t, repo, "sync")
	// Same hand-edit TestSyncSkipsAlteredBlock uses: "Use Vault." lives
	// inside the managed block, so changing it triggers Altered.
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

	out, code := runEscOut(t, repo, "sync")
	if code != 0 {
		t.Fatalf("exit = %d, want 0:\n%s", code, out)
	}
	const wantHint = "    `esc diff` to inspect · `esc sync --force` to overwrite"
	lines := strings.Split(out, "\n")
	found := false
	for i, l := range lines {
		if !strings.HasPrefix(l, "  skipped AGENTS.md: ") {
			continue
		}
		found = true
		if i+1 >= len(lines) || lines[i+1] != wantHint {
			t.Errorf("line after the skip = %q, want %q", lines[i+1], wantHint)
		}
	}
	if !found {
		t.Fatalf("no skip line for AGENTS.md in:\n%s", out)
	}
	if n := strings.Count(out, "esc diff` to inspect"); n != 1 {
		t.Errorf("want exactly one remedy line for one skip, got %d:\n%s", n, out)
	}
}
