package cli

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/render"
)

func TestApplyPlacement(t *testing.T) {
	cases := []struct {
		name, existing string
		p              placement
		wantPrefix     string
	}{
		{"above", "# P\n\nrules\n", placeAbove, render.Placeholder + "\n# P\n"},
		{"end", "# P\n\nrules\n", placeEnd, "# P\n\nrules\n" + render.Placeholder + "\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, map[string]string{"CLAUDE.md": tc.existing})
			d := Detected{Target: "claude", Path: "CLAUDE.md"}
			if err := applyPlacement(root, d, tc.p); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(got), tc.wantPrefix) {
				t.Errorf("got:\n%q\nwant prefix:\n%q", got, tc.wantPrefix)
			}
			if !strings.Contains(string(got), "rules\n") {
				t.Errorf("original content lost:\n%s", got)
			}
			if strings.Count(string(got), render.Placeholder) != 1 {
				t.Errorf("placeholder must appear exactly once:\n%s", got)
			}
		})
	}
}

func TestApplyPlacementDefaultWritesNothing(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"CLAUDE.md": "rules\n"})
	before, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err := applyPlacement(root, Detected{Target: "claude", Path: "CLAUDE.md"}, placeDefault); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if string(before) != string(after) {
		t.Error("the default placement must not touch the file")
	}
}

func TestApplyPlacementDefaultMatchesSyncBehavior(t *testing.T) {
	// placeDefault writing nothing is only correct if sync then puts the
	// block where the offer promised. Prove the promise rather than assuming
	// it: a file left untouched must still get its block after the title.
	out, err := render.Splice([]byte("# P\n\nrules\n"), "body\n", render.BlockMeta{Packs: []string{"p@1"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(out), "# P\n\n<!-- escapement:begin ") {
		t.Errorf("offer copy promises the block lands after the title, got:\n%s", out)
	}
}

// placeEnd corrupted any file with no trailing newline before this was
// fixed: "rules" (no newline) + Placeholder + "\n" produced
// "rules<!-- escapement:block -->", and when render.Splice later
// substitutes the block at that byte index, the begin marker lands
// mid-line, glued onto "rules" with no separator. render.Splice's own
// append-at-end path already guards exactly this (see block.go's Splice,
// the len(s)==at case); applyPlacement must mirror it.
func TestApplyPlacementEndPreservesNewlineSeparation(t *testing.T) {
	cases := []struct {
		name, existing, want string
	}{
		{"no trailing newline", "rules", "rules\n" + render.Placeholder + "\n"},
		{"empty file", "", render.Placeholder + "\n"},
		{"whitespace only, no newline", "   ", "   \n" + render.Placeholder + "\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, map[string]string{"CLAUDE.md": tc.existing})
			d := Detected{Target: render.TargetClaude, Path: "CLAUDE.md"}
			if err := applyPlacement(root, d, placeEnd); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
			// The corrupt pre-fix output also "preserved every original
			// byte" (that assertion alone was insufficient): additionally
			// prove render.Splice can still find and replace a single,
			// well-formed placeholder afterward.
			spliced, err := render.Splice(got, "body\n", render.BlockMeta{Packs: []string{"p@1"}})
			if err != nil {
				t.Fatalf("render.Splice rejected applyPlacement's output: %v", err)
			}
			if strings.Contains(string(spliced), "escapement:begin") && strings.Contains(string(spliced), "rules<!--") {
				t.Errorf("begin marker glued onto original content with no separator:\n%s", spliced)
			}
		})
	}
}

// placeAbove means "above the title", not "at byte 0". Writing at byte 0
// preserved every byte of a file with YAML frontmatter and still destroyed
// it: frontmatter is only frontmatter at byte 0, so a marker above it demotes
// `---\ntitle: x\n---` into a setext heading plus a horizontal rule. Same
// class of bug as the placeEnd newline case above: bytes preserved, semantics
// destroyed. The offset comes from render.FrontmatterEnd, the one parser
// render.Splice's own top placement already uses.
func TestApplyPlacementAboveStaysBelowFrontmatter(t *testing.T) {
	cases := []struct {
		name, existing, want string
	}{
		{
			"frontmatter",
			"---\ntitle: X\n---\nteam rules\n",
			"---\ntitle: X\n---\n" + render.Placeholder + "\nteam rules\n",
		},
		{
			"frontmatter and h1",
			"---\ntitle: X\n---\n# Rules\n\nteam\n",
			"---\ntitle: X\n---\n" + render.Placeholder + "\n# Rules\n\nteam\n",
		},
		{
			// An unterminated fence is not frontmatter, so there is nothing
			// to stay below: the marker goes at byte 0, as before.
			"unterminated frontmatter fence",
			"---\ntitle: X\n# Rules\n",
			render.Placeholder + "\n---\ntitle: X\n# Rules\n",
		},
		{
			// The fence itself has no trailing newline: the marker must
			// still start its own line rather than glue onto "---".
			"frontmatter with no trailing newline",
			"---\ntitle: X\n---",
			"---\ntitle: X\n---\n" + render.Placeholder + "\n",
		},
		{
			"no frontmatter at all",
			"# Rules\n\nteam\n",
			render.Placeholder + "\n# Rules\n\nteam\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, map[string]string{"CLAUDE.md": tc.existing})
			d := Detected{Target: render.TargetClaude, Path: "CLAUDE.md"}
			if err := applyPlacement(root, d, placeAbove); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("got  %q\nwant %q", got, tc.want)
			}
			// Frontmatter that survived the marker must still be at byte 0,
			// which is the whole point: the bytes being present is not the
			// property under test.
			if strings.HasPrefix(tc.existing, "---\n") && strings.Contains(tc.existing, "\n---") {
				if !strings.HasPrefix(string(got), "---\n") {
					t.Errorf("frontmatter no longer starts the file, so it is no longer frontmatter:\n%s", got)
				}
			}
			// And sync must still be able to fill the marker in place.
			if _, err := render.Splice(got, "body\n", render.BlockMeta{Packs: []string{"p@1"}}); err != nil {
				t.Fatalf("render.Splice rejected applyPlacement's output: %v", err)
			}
		})
	}
}

// restoreFile must preserve the destination's existing permission bits
// rather than forcing 0o644, so a user's CLAUDE.md at, say, 0o600 is not
// silently made world-readable just because escapement wrote to it.
func TestApplyPlacementPreservesFileMode(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"CLAUDE.md": "rules\n"})
	path := filepath.Join(root, "CLAUDE.md")
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	d := Detected{Target: render.TargetClaude, Path: "CLAUDE.md"}
	if err := applyPlacement(root, d, placeAbove); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode changed from 0o600 to %o", fi.Mode().Perm())
	}
}

// --- askPlacement: the prompt itself, called directly with a controlled
// reader so no test depends on the real stdin/TTY plumbing. ---

func TestAskPlacementAnswers(t *testing.T) {
	targets := []Detected{{Target: render.TargetClaude, Path: "CLAUDE.md"}}
	cases := []struct {
		name, input string
		want        placement
	}{
		{"yes then above", "y\na\n", placeAbove},
		{"yes then end", "y\ne\n", placeEnd},
		{"yes spelled out", "yes\ne\n", placeEnd},
		{"uppercase yes", "Y\nA\n", placeAbove},
		{"declined", "n\n", placeDefault},
		{"empty declines", "\n", placeDefault},
		{"unrecognized gate answer declines", "zzz\n", placeDefault},
		{"eof declines", "", placeDefault},
		{"yes then eof", "y\n", placeDefault},
		{"yes then unrecognized position", "y\nq\n", placeDefault},
		{"eof after yes with no newline", "y", placeDefault},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			got := askPlacement(&out, bufio.NewReader(strings.NewReader(tc.input)), targets, false)
			if got != tc.want {
				t.Errorf("askPlacement(%q) = %q, want %q", tc.input, got, tc.want)
			}
			if !strings.Contains(out.String(), "Place the marker now? [y/N]") {
				t.Errorf("the gate must always be printed, got:\n%s", out.String())
			}
			asked := strings.Contains(out.String(), "[a] above the title")
			if wantAsked := strings.HasPrefix(strings.ToLower(tc.input), "y"); asked != wantAsked {
				t.Errorf("position question printed = %v, want %v:\n%s", asked, wantAsked, out.String())
			}
		})
	}
}

// The gate is asked once for the whole run, not once per file, and its one
// position answer names every file it will apply to.
func TestAskPlacementAsksOnceForAllFiles(t *testing.T) {
	targets := []Detected{
		{Target: render.TargetClaude, Path: "CLAUDE.md"},
		{Target: render.TargetAgents, Path: "AGENTS.md"},
		{Target: render.TargetGemini, Path: "GEMINI.md"},
		{Target: render.TargetGovernance, Path: "GOVERNANCE.md"},
	}
	var out bytes.Buffer
	if got := askPlacement(&out, bufio.NewReader(strings.NewReader("y\na\n")), targets, false); got != placeAbove {
		t.Fatalf("got %q, want placeAbove", got)
	}
	s := out.String()
	if n := strings.Count(s, "Place the marker now?"); n != 1 {
		t.Errorf("the gate must be asked exactly once for %d files, asked %d times:\n%s", len(targets), n, s)
	}
	if n := strings.Count(s, "[a] above the title"); n != 1 {
		t.Errorf("the position must be asked exactly once, asked %d times:\n%s", n, s)
	}
	for _, d := range targets {
		if !strings.Contains(s, "  "+d.Path+"\n") {
			t.Errorf("the gate must list %s as a file it will write to:\n%s", d.Path, s)
		}
	}
	if !strings.Contains(s, "all 4 files") {
		t.Errorf("the position question should say the answer applies to all of them:\n%s", s)
	}
}

// Item C: in a directory git cannot undo a write in, the gate says so rather
// than treating "no version control at all" as the safest case of all.
func TestAskPlacementWarnsWhenNoGitUndo(t *testing.T) {
	targets := []Detected{{Target: render.TargetClaude, Path: "CLAUDE.md"}}
	var warned, quiet bytes.Buffer
	askPlacement(&warned, bufio.NewReader(strings.NewReader("n\n")), targets, true)
	askPlacement(&quiet, bufio.NewReader(strings.NewReader("n\n")), targets, false)
	if !strings.Contains(warned.String(), "not a git repository") || !strings.Contains(warned.String(), "no undo") {
		t.Errorf("expected a no-undo warning at the gate, got:\n%s", warned.String())
	}
	if strings.Contains(quiet.String(), "not a git repository") {
		t.Errorf("a real git repo must not be warned about:\n%s", quiet.String())
	}
}

// The gate and the position question read from ONE bufio.Reader. A fresh
// reader per question would buffer past "y\n" and discard the "a\n" behind
// it, silently turning every piped or pasted answer into a decline. This
// fails if that reuse is ever dropped.
func TestAskPlacementReusesOneReaderAcrossBothQuestions(t *testing.T) {
	targets := []Detected{{Target: render.TargetClaude, Path: "CLAUDE.md"}}
	var out bytes.Buffer
	// One plain reader holding both answers at once, exactly as a pipe or a
	// paste delivers them.
	if got := askPlacement(&out, bufio.NewReader(strings.NewReader("y\ne\n")), targets, false); got != placeEnd {
		t.Errorf("got %q, want placeEnd: the position answer buffered behind the gate answer was lost", got)
	}
}

// infiniteReader never yields a newline and never ends, standing in for
// `esc init < /dev/zero`, which tty.IsInteractive cannot distinguish from a
// terminal (a character device that simply never sends '\n').
type infiniteReader struct{ b byte }

func (r infiniteReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = r.b
	}
	return len(p), nil
}

// Without the byte bound this hangs forever instead of failing, which is the
// honest test: the guard's whole job is that the read terminates.
func TestReadBoundedLineStopsAtTheBound(t *testing.T) {
	got := readBoundedLine(bufio.NewReader(infiniteReader{'y'}), maxOfferAnswerBytes)
	if len(got) != maxOfferAnswerBytes {
		t.Errorf("read %d bytes, want the bound of %d", len(got), maxOfferAnswerBytes)
	}
}

// A stream that never terminates a line must not be read as consent: the
// truncated answer matches nothing, so the gate declines.
func TestAskPlacementUnterminatedStreamDeclines(t *testing.T) {
	targets := []Detected{{Target: render.TargetClaude, Path: "CLAUDE.md"}}
	var out bytes.Buffer
	if got := askPlacement(&out, bufio.NewReader(infiniteReader{'y'}), targets, false); got != placeDefault {
		t.Errorf("got %q, want placeDefault for an answer that never ends", got)
	}
}

func TestInitInteractiveYesForcesFalse(t *testing.T) {
	// --yes returns before the seam is consulted at all, so this holds
	// regardless of the test process's own stdin. Pinned by making the seam
	// fail the test if it is reached.
	old := initInput
	initInput = func() (io.Reader, bool) {
		t.Error("--yes must not consult the interactivity seam at all")
		return strings.NewReader("y\na\n"), true
	}
	t.Cleanup(func() { initInput = old })
	if _, got := initInteractive(true); got {
		t.Error("--yes must force non-interactive")
	}
}

// fakeStdin makes a run interactive with a scripted answer, driving the real
// prompt code (gate parsing, position parsing, reader reuse) end to end
// instead of stubbing past it.
func fakeStdin(t *testing.T, input string) {
	t.Helper()
	old := initInput
	r := strings.NewReader(input)
	initInput = func() (io.Reader, bool) { return r, true }
	t.Cleanup(func() { initInput = old })
}

// tty.IsInteractive itself (including the /dev/null exclusion this package
// depends on) is tested directly in internal/tty; internal/cli no longer
// has its own copy to test (see item 3 of the review that added the shared
// package).

// --- fileIsClean ---

func initGitRepo(t *testing.T, root string) {
	t.Helper()
	gitIn(t, root, "init", "-q", "-b", "main")
	gitIn(t, root, "config", "user.email", "t@e.com")
	gitIn(t, root, "config", "user.name", "T")
	gitIn(t, root, "config", "commit.gpgsign", "false")
}

func commitAll(t *testing.T, root, msg string) {
	t.Helper()
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", msg)
}

func TestFileIsCleanNotAGitRepo(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"CLAUDE.md": "rules\n"})
	clean, err := fileIsClean(context.Background(), root, "CLAUDE.md")
	if err != nil {
		t.Fatal(err)
	}
	if !clean {
		t.Error("a repo with no .git at all must be treated as clean, not errored")
	}
}

func TestFileIsCleanCommitted(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"CLAUDE.md": "rules\n"})
	initGitRepo(t, root)
	commitAll(t, root, "init")
	clean, err := fileIsClean(context.Background(), root, "CLAUDE.md")
	if err != nil {
		t.Fatal(err)
	}
	if !clean {
		t.Error("a committed file with no local edits must be clean")
	}
}

func TestFileIsCleanDirty(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"CLAUDE.md": "rules\n"})
	initGitRepo(t, root)
	commitAll(t, root, "init")
	writeFiles(t, root, map[string]string{"CLAUDE.md": "rules\nmore\n"})
	clean, err := fileIsClean(context.Background(), root, "CLAUDE.md")
	if err != nil {
		t.Fatal(err)
	}
	if clean {
		t.Error("a locally-edited file must be reported dirty")
	}
}

// fileIsClean is pathspec-scoped (`-- <path>`), not a bare `git status`: a
// bare status in root would also report the .escapement/ config esc init
// itself is about to write as untracked, making CLAUDE.md look dirty on
// the very first run of a fresh repo. This is also exercised implicitly by
// every CLI-integration test below that runs `esc init` inside a
// committed repo (cmdInit's own config scaffolding lands as untracked
// files before offerPlacement's fileIsClean call ever runs), but is worth
// pinning directly.
func TestFileIsCleanIgnoresUnrelatedUntrackedFiles(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"CLAUDE.md": "rules\n"})
	initGitRepo(t, root)
	commitAll(t, root, "init")
	writeFiles(t, root, map[string]string{".escapement/config.yaml": "schema: 1\npacks: []\n"})
	clean, err := fileIsClean(context.Background(), root, "CLAUDE.md")
	if err != nil {
		t.Fatal(err)
	}
	if !clean {
		t.Error("an unrelated untracked file must not make CLAUDE.md look dirty")
	}
}

// isGitRepo (which fileIsClean now delegates the repo-ness question to)
// decides "is this a git repo" purely by the exit code of `git rev-parse
// --is-inside-work-tree`, never by matching any of git's (locale-dependent)
// fatal messages — including "not a git repository" and "detected dubious
// ownership" alike. That structural change makes the message-matching test
// this comment used to describe unnecessary. What's still worth pinning
// directly: git missing from PATH entirely must produce a clear,
// distinguishable error (the one case isGitRepo does distinguish, via
// errors.Is(exec.ErrNotFound), a Go-level check, not a message match).
func TestFileIsCleanGitNotFound(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"CLAUDE.md": "rules\n"})
	t.Setenv("PATH", t.TempDir()) // a directory with no git binary in it
	_, err := fileIsClean(context.Background(), root, "CLAUDE.md")
	if err == nil {
		t.Fatal("expected an error when git is not in PATH")
	}
	if !strings.Contains(err.Error(), "git not found") {
		t.Errorf("expected a clear 'git not found' error, got: %v", err)
	}
}

// isGitRepo returns (false, nil) — not an error — for any git-repo-check
// failure other than the binary being missing, which is exactly what makes
// a "detected dubious ownership" refusal (git >= 2.35.2, routine in
// containers and CI with mounted volumes) tolerated the same way a missing
// .git is, rather than propagating as an error. Reproducing dubious
// ownership itself needs a file genuinely owned by a different user, not
// practical to set up portably in this test environment, but the exit-code
// contract it relies on doesn't: any non-zero, non-ErrNotFound exit from
// `git rev-parse --is-inside-work-tree` must come back false/nil. Pin that
// directly against a real git failure mode that IS easy to produce: root
// pointed at a path with no .git and where "git status" would also fail
// for unrelated reasons (a file, not a directory, since that's simplest to
// construct) — isGitRepo must still say "not a repo, no error" rather than
// surface whatever git's actual complaint was.
func TestIsGitRepoNonZeroExitOtherThanMissingBinaryIsTolerated(t *testing.T) {
	root := t.TempDir()
	notADir := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	inRepo, err := isGitRepo(context.Background(), notADir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if inRepo {
		t.Error("expected isGitRepo to report false for a path git cannot operate in")
	}
}

// Cheap, locale-independent proxy for the LC_ALL=C fix (reproducing the
// original locale bug needs a second gettext catalog installed for git,
// not practical here): pin that the shared command constructor actually
// sets it, so the fix can't be quietly dropped later.
func TestGitCommandSetsLocale(t *testing.T) {
	cmd := gitCommand(context.Background(), t.TempDir(), "status")
	found := false
	for _, e := range cmd.Env {
		if e == "LC_ALL=C" {
			found = true
		}
	}
	if !found {
		t.Errorf("gitCommand must set LC_ALL=C, got env: %v", cmd.Env)
	}
}

// --- offerPlacement: error surfacing (item 7) ---

// A genuine failure (as opposed to a routine dirty-file skip) must be
// returned, not just printed, so a caller that isn't reading prose output
// can still detect it.
func TestOfferPlacementSurfacesWriteFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses the permission check this test relies on")
	}
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"CLAUDE.md": "rules\n"})
	initGitRepo(t, root)
	commitAll(t, root, "init")
	d := Detected{Target: render.TargetClaude, Path: "CLAUDE.md"}
	fakeStdin(t, "y\na\n")

	// applyPlacement's atomic write needs to create a temp file in root;
	// removing write permission on root makes that fail, without touching
	// CLAUDE.md's own readability.
	if err := os.Chmod(root, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(root, 0o700) })

	var stdout bytes.Buffer
	err := offerPlacement(context.Background(), root, &stdout, []Detected{d}, false)
	if err == nil {
		t.Fatal("expected offerPlacement to return an error when the write fails")
	}
	if !strings.Contains(err.Error(), "CLAUDE.md") {
		t.Errorf("the error should name the file: %v", err)
	}
}

// End-to-end proof that a placement write failure is both visible (named
// in the combined output) and detectable by exit code (non-zero), not
// silently swallowed with an exit 0 a script would read as success.
func TestInitOfferWriteFailureIsVisibleAndNonZero(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses the permission check this test relies on")
	}
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"CLAUDE.md": "rules\n"})
	initGitRepo(t, root)
	commitAll(t, root, "init")
	// Pre-create .escapement so cmdInit's config scaffolding (mkdir is a
	// no-op on an existing dir, and the dir itself stays writable) succeeds
	// even after root is made read-only below; only creating CLAUDE.md's
	// temp file needs write access to root, since CLAUDE.md lives directly
	// there.
	if err := os.MkdirAll(filepath.Join(root, ".escapement"), 0o755); err != nil {
		t.Fatal(err)
	}

	fakeStdin(t, "y\na\n")

	if err := os.Chmod(root, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(root, 0o700) })

	code, out := run(t, root, "init")
	if code == 0 {
		t.Fatalf("a placement write failure must not exit 0, got 0:\n%s", out)
	}
	if !strings.Contains(out, "CLAUDE.md") {
		t.Errorf("the failure must name the file, got:\n%s", out)
	}
}

// Regression test for a bug introduced by the item-7 fix round and caught
// on re-review: fileIsClean erroring (e.g. git missing from PATH) was
// briefly promoted into offerPlacement's returned error, the same path as
// an actual write failure — so a git CHECK failure made the whole `esc
// init` exit non-zero, even though config scaffolding had already fully
// succeeded and nothing was ever written or corrupted. `esc init && esc
// sync` or `esc init || exit 1` would have broken on any run where the git
// check itself couldn't run (missing git, or the more realistic "detected
// dubious ownership" case in containers/CI). A git-check failure must
// behave like a routine skip: printed, and non-fatal, exactly as it did
// before that fix round.
func TestInitExitsZeroWhenGitCheckFailsButConfigWritten(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"CLAUDE.md": "rules\n"})
	t.Setenv("PATH", t.TempDir()) // no git binary anywhere on PATH
	// Interactive, because a non-interactive run now returns before doing
	// any git work at all (there is nothing it could write); the git check
	// only runs on the path where the offer is about to be made.
	fakeStdin(t, "y\na\n")

	code, out := run(t, root, "init")
	if code != 0 {
		t.Fatalf("init must exit 0 when only the git check fails, got %d:\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(root, ".escapement", "config.yaml")); err != nil {
		t.Errorf("config scaffolding should still have succeeded: %v", err)
	}
	if !strings.Contains(out, "could not check git status") {
		t.Errorf("the skip should still be reported, got:\n%s", out)
	}
	after, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if string(after) != "rules\n" {
		t.Errorf("nothing may be written when the git check could not run:\n%s", after)
	}
}

func TestInitHelpFlagExitsUsage(t *testing.T) {
	root := t.TempDir()
	code, _ := run(t, root, "init", "-h")
	if code != 2 {
		t.Errorf("esc init -h should exit 2 (usage), got %d", code)
	}
}

// `esc init /some/other/repo` used to exit 0 and scaffold the CURRENT
// directory instead, which is how a live .escapement/ once landed in this
// project's own repo. A positional argument is a usage error.
func TestInitRejectsPositionalArguments(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	code, out := run(t, root, "init", other)
	if code != 2 {
		t.Fatalf("esc init <path> should exit 2 (usage), got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "takes no arguments") {
		t.Errorf("the error should say why, got:\n%s", out)
	}
	for _, dir := range []string{root, other} {
		if _, err := os.Stat(filepath.Join(dir, ".escapement")); !os.IsNotExist(err) {
			t.Errorf("a rejected init must not scaffold anything in %s (err=%v)", dir, err)
		}
	}
}

// The whole offer for the worst case: all six targets detected, four of them
// eligible files. One gate, one position question, one answer applied to
// every file. Pinned as exact output, since the entire point of the reshape
// was what this reads like.
func TestInitOfferFullFlowForSixTargets(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"CLAUDE.md":                      "# P\n\nrules\n",
		"AGENTS.md":                      "# P\n\nrules\n",
		"GEMINI.md":                      "# P\n\nrules\n",
		"GOVERNANCE.md":                  "# P\n\nrules\n",
		".mcp.json":                      `{"mcpServers":{}}`,
		".claude/skills/team-x/SKILL.md": "---\nname: team-x\n---\n",
	})
	initGitRepo(t, root)
	commitAll(t, root, "init")
	fakeStdin(t, "y\na\n")

	code, out := run(t, root, "init")
	if code != 0 {
		t.Fatalf("init exit %d:\n%s", code, out)
	}
	wantOffer := "\nOptional: write " + render.Placeholder + " into all 4 files now, to choose where the block lands.\n" +
		"  CLAUDE.md\n  AGENTS.md\n  GEMINI.md\n  GOVERNANCE.md\n" +
		"Place the marker now? [y/N] " +
		"Where should it go in all 4 files?\n" +
		"  [a] above the title, the first thing in the file (below any frontmatter)\n" +
		"  [e] end of the file\n> "
	if !strings.HasSuffix(out, wantOffer) {
		t.Errorf("offer output =\n%q\nwant suffix\n%q", out, wantOffer)
	}
	// mcp and skills are never offered a marker, and the one answer reached
	// every eligible file.
	for _, name := range []string{"CLAUDE.md", "AGENTS.md", "GEMINI.md", "GOVERNANCE.md"} {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != render.Placeholder+"\n# P\n\nrules\n" {
			t.Errorf("%s: one answer must apply to every eligible file, got:\n%q", name, content)
		}
	}
}

// --- CLI integration: through Run(), using the newGoverned/run helpers. ---

func TestInitNonInteractiveDoesNotModifyFile(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"CLAUDE.md": "# P\n\nrules\n"})
	before, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	code, out := run(t, root, "init")
	if code != 0 {
		t.Fatalf("init exit %d:\n%s", code, out)
	}
	after, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if string(before) != string(after) {
		t.Errorf("non-interactive init modified the file:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestInitYesWritesNothing(t *testing.T) {
	// --yes selects placeDefault, which never writes. This looks like a bug
	// (why have a flag that never does anything to the file?) and is not:
	// --yes exists so a scripted run does not stall on the prompt, not so it
	// can accept a file write. Pinned explicitly so a later reader doesn't
	// "fix" this into applying placeAbove/placeEnd by default.
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"CLAUDE.md": "rules\n"})
	before, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	code, out := run(t, root, "init", "--yes")
	if code != 0 {
		t.Fatalf("init --yes exit %d:\n%s", code, out)
	}
	after, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if string(before) != string(after) {
		t.Errorf("--yes must not modify the file: before %q after %q", before, after)
	}
}

func TestInitOfferAppliesTheAnswer(t *testing.T) {
	cases := []struct {
		name, input string
		want        func(content string) bool
	}{
		{"above", "y\na\n", func(c string) bool { return strings.HasPrefix(c, render.Placeholder+"\n") }},
		{"end", "y\ne\n", func(c string) bool { return strings.HasSuffix(c, render.Placeholder+"\n") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, map[string]string{"CLAUDE.md": "# P\n\nrules\n"})
			initGitRepo(t, root)
			commitAll(t, root, "init")
			fakeStdin(t, tc.input)

			code, out := run(t, root, "init")
			if code != 0 {
				t.Fatalf("init exit %d:\n%s", code, out)
			}
			content, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !tc.want(string(content)) {
				t.Errorf("answer %q not applied as expected:\n%s", tc.input, content)
			}
			if strings.Count(string(content), render.Placeholder) != 1 {
				t.Errorf("placeholder must appear exactly once:\n%s", content)
			}
		})
	}
}

// Declining the gate is the recommended answer and the default, and it must
// produce zero bytes written, end to end, through the real prompt.
func TestInitDeclinedGateWritesNothingThroughRun(t *testing.T) {
	for _, input := range []string{"n\n", "\n", "", "zzz\n"} {
		t.Run(fmt.Sprintf("%q", input), func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, map[string]string{"CLAUDE.md": "rules\n"})
			initGitRepo(t, root)
			commitAll(t, root, "init")
			fakeStdin(t, input)

			before, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
			code, out := run(t, root, "init")
			if code != 0 {
				t.Fatalf("init exit %d:\n%s", code, out)
			}
			after, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
			if string(before) != string(after) {
				t.Errorf("a declined gate must not modify the file, input %q", input)
			}
		})
	}
}

// Item C, the other half of the honest resolution: in a plain directory with
// no git at all, the write is still offered (running init there is
// legitimate) but the gate states there is no undo.
func TestInitWarnsThereIsNoUndoOutsideAGitRepo(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"CLAUDE.md": "# P\n\nrules\n"})
	fakeStdin(t, "y\ne\n")

	code, out := run(t, root, "init")
	if code != 0 {
		t.Fatalf("init exit %d:\n%s", code, out)
	}
	if !strings.Contains(out, "not a git repository") || !strings.Contains(out, "no undo") {
		t.Errorf("the gate must say there is no undo here, got:\n%s", out)
	}
	content, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if !strings.HasSuffix(string(content), render.Placeholder+"\n") {
		t.Errorf("an explicit yes must still be honored outside a git repo:\n%s", content)
	}
}

// Item D: a non-interactive run must not do git work or print per-file skip
// noise about a prompt that was never going to be shown and a write that was
// never contemplated.
func TestInitNonInteractivePrintsNoOfferNoise(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"CLAUDE.md":     "# P\n\nrules\n",
		"AGENTS.md":     "# P\n\nrules\n",
		"GEMINI.md":     "# P\n\nrules\n",
		"GOVERNANCE.md": "# P\n\nrules\n",
	})
	initGitRepo(t, root)
	commitAll(t, root, "init")
	// Dirty every file: pre-fix, this printed four skip lines and ran eight
	// git subprocesses for a prompt --yes had already ruled out.
	writeFiles(t, root, map[string]string{
		"CLAUDE.md":     "# P\n\nrules\nmore\n",
		"AGENTS.md":     "# P\n\nrules\nmore\n",
		"GEMINI.md":     "# P\n\nrules\nmore\n",
		"GOVERNANCE.md": "# P\n\nrules\nmore\n",
	})

	code, out := run(t, root, "init", "--yes")
	if code != 0 {
		t.Fatalf("init --yes exit %d:\n%s", code, out)
	}
	for _, noise := range []string{"skipping the placement offer", "Place the marker now?"} {
		if strings.Contains(out, noise) {
			t.Errorf("--yes must not print %q:\n%s", noise, out)
		}
	}
}

func TestInitSkipsDirtyFileButProcessesOthers(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"CLAUDE.md": "# P\n\nrules\n",
		"AGENTS.md": "# P\n\nrules\n",
	})
	initGitRepo(t, root)
	commitAll(t, root, "init")
	// Dirty CLAUDE.md after the commit: an uncommitted local edit.
	writeFiles(t, root, map[string]string{"CLAUDE.md": "# P\n\nrules\nmore\n"})
	fakeStdin(t, "y\na\n")

	dirtyBefore, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	code, out := run(t, root, "init")
	if code != 0 {
		t.Fatalf("init exit %d:\n%s", code, out)
	}
	if !strings.Contains(out, "CLAUDE.md") || !strings.Contains(out, "uncommitted") {
		t.Errorf("expected a stated skip reason naming CLAUDE.md, got:\n%s", out)
	}
	dirtyAfter, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if string(dirtyBefore) != string(dirtyAfter) {
		t.Error("the dirty file must not be modified even though the answer was 'a'")
	}
	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(agents), render.Placeholder+"\n") {
		t.Errorf("the clean file must still be offered and get the placement applied:\n%s", agents)
	}
}

func TestInitNoOfferWhenAlreadyPlaced(t *testing.T) {
	block, err := render.Splice(nil, "body\n", render.BlockMeta{Packs: []string{"p@1"}})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, content string
	}{
		{"placeholder", render.Placeholder + "\n# P\n\nrules\n"},
		{"managed block", string(block) + "# P\n\nrules\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, map[string]string{"CLAUDE.md": tc.content})
			initGitRepo(t, root)
			commitAll(t, root, "init")

			fakeStdin(t, "y\ne\n")

			before, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
			code, out := run(t, root, "init")
			if code != 0 {
				t.Fatalf("init exit %d:\n%s", code, out)
			}
			if strings.Contains(out, "Place the marker now?") {
				t.Errorf("a file that already has a block or placeholder must not be offered:\n%s", out)
			}
			after, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
			if string(before) != string(after) {
				t.Error("a file that already has a block or placeholder must not be modified")
			}
		})
	}
}

func TestInitOfferPlacementThenSyncFillsIt(t *testing.T) {
	repo := newPackRepo(t, "1.0.0")
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"CLAUDE.md": "# Team notes\n\nOur build uses pnpm.\n"})
	initGitRepo(t, root)
	commitAll(t, root, "init")
	fakeStdin(t, "y\ne\n")

	if code, out := run(t, root, "init"); code != 0 {
		t.Fatalf("init exit %d:\n%s", code, out)
	}

	// esc init scaffolds an empty-packs config; point it at a real pack repo
	// before syncing, same as newGoverned does for other end-to-end tests.
	writeFiles(t, root, map[string]string{
		".escapement/config.yaml": `schema: 1
packs:
  - source: file://` + repo + `//org
    ref: v1.0.0
    trust: unsigned
`,
	})
	t.Setenv("ESC_CACHE_DIR", t.TempDir())

	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("sync exit %d:\n%s", code, out)
	}
	content, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)
	if !strings.HasPrefix(s, "# Team notes\n\nOur build uses pnpm.\n") {
		t.Errorf("original content before the placeholder must survive unchanged:\n%s", s)
	}
	if !strings.Contains(s, "<!-- escapement:begin ") {
		t.Errorf("sync should have filled the end placeholder with the managed block:\n%s", s)
	}
	if strings.Index(s, "<!-- escapement:begin ") < strings.Index(s, "Our build uses pnpm.") {
		t.Errorf("block should land at the end (where the placeholder was), not at the top:\n%s", s)
	}
}
