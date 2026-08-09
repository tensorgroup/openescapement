package cli

import (
	"bytes"
	"context"
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
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"CLAUDE.md": "# P\n\nrules\n"})
	out, err := render.Splice([]byte("# P\n\nrules\n"), "body\n", render.BlockMeta{Packs: []string{"p@1"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(out), "# P\n\n<!-- escapement:begin ") {
		t.Errorf("offer copy promises the block lands after the title, got:\n%s", out)
	}
	_ = root
}

// --- initOffer: the prompt-parsing seam itself, called directly with a
// controlled reader so no test depends on the real stdin/TTY plumbing. ---

func TestInitOfferAnswers(t *testing.T) {
	d := Detected{Target: render.TargetClaude, Path: "CLAUDE.md"}
	cases := []struct {
		name, input string
		want        placement
	}{
		{"above", "a\n", placeAbove},
		{"end", "e\n", placeEnd},
		{"keep", "k\n", placeDefault},
		{"empty", "\n", placeDefault},
		{"unrecognized", "zzz\n", placeDefault},
		{"eof no newline", "a", placeAbove},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			got := initOffer(&out, strings.NewReader(tc.input), true, d)
			if got != tc.want {
				t.Errorf("initOffer(%q) = %q, want %q", tc.input, got, tc.want)
			}
			if out.Len() == 0 {
				t.Error("an interactive offer must print the prompt")
			}
		})
	}
}

func TestInitOfferNonInteractiveIgnoresInputAndPrintsNothing(t *testing.T) {
	d := Detected{Target: render.TargetClaude, Path: "CLAUDE.md"}
	var out bytes.Buffer
	// The reader offers "a" (place above); a non-interactive call must never
	// read it and must never print the prompt that would ask for it.
	got := initOffer(&out, strings.NewReader("a\n"), false, d)
	if got != placeDefault {
		t.Errorf("non-interactive initOffer = %q, want placeDefault", got)
	}
	if out.Len() != 0 {
		t.Errorf("non-interactive initOffer must not print a prompt: %q", out.String())
	}
}

func TestInitInteractiveYesForcesFalse(t *testing.T) {
	// Go's short-circuit && guarantees --yes short-circuits before the real
	// TTY check runs, so this holds regardless of the test's own stdin.
	if got := initInteractive(true); got {
		t.Error("--yes must force non-interactive")
	}
}

// go test always substitutes /dev/null for the test binary's stdin — even
// when the outer shell has a real terminal — and /dev/null is itself a
// character device. Without excluding it explicitly, isInteractive(os.Stdin)
// is a false positive under `go test` (and under any real invocation with
// stdin redirected from /dev/null, a common CI pattern), which would make
// every init test in this package print the interactive prompt. This pins
// the exclusion directly rather than relying on it only being exercised
// incidentally by every other test in the package.
func TestIsInteractiveExcludesDevNull(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if isInteractive(f) {
		t.Error("os.DevNull must not be treated as an interactive terminal")
	}
}

func TestIsInteractiveNilIsFalse(t *testing.T) {
	if isInteractive(nil) {
		t.Error("a nil file must not be treated as interactive")
	}
}

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

func TestInitOfferAppliesInjectedAnswer(t *testing.T) {
	cases := []struct {
		name string
		p    placement
		want func(content string) bool
	}{
		{"above", placeAbove, func(c string) bool { return strings.HasPrefix(c, render.Placeholder+"\n") }},
		{"end", placeEnd, func(c string) bool { return strings.HasSuffix(c, render.Placeholder+"\n") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, map[string]string{"CLAUDE.md": "# P\n\nrules\n"})
			initGitRepo(t, root)
			commitAll(t, root, "init")

			old := initOffer
			initOffer = func(w io.Writer, in io.Reader, interactive bool, d Detected) placement { return tc.p }
			t.Cleanup(func() { initOffer = old })

			code, out := run(t, root, "init")
			if code != 0 {
				t.Fatalf("init exit %d:\n%s", code, out)
			}
			content, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !tc.want(string(content)) {
				t.Errorf("placement %q not applied as expected:\n%s", tc.p, content)
			}
			if strings.Count(string(content), render.Placeholder) != 1 {
				t.Errorf("placeholder must appear exactly once:\n%s", content)
			}
		})
	}
}

// The seam's own parsing of "k", empty, and unrecognized input is covered
// directly by TestInitOfferAnswers (no TTY to fake through Run() without a
// pty dependency, which the single-dependency policy rules out). This
// covers the other half: cmdInit's wiring must actually apply whatever
// initOffer returns, and placeDefault in particular must produce zero
// bytes written, end to end.
func TestInitOfferDefaultWritesNothingThroughRun(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"CLAUDE.md": "rules\n"})
	initGitRepo(t, root)
	commitAll(t, root, "init")

	old := initOffer
	initOffer = func(w io.Writer, in io.Reader, interactive bool, d Detected) placement { return placeDefault }
	t.Cleanup(func() { initOffer = old })

	before, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	code, out := run(t, root, "init")
	if code != 0 {
		t.Fatalf("init exit %d:\n%s", code, out)
	}
	after, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if string(before) != string(after) {
		t.Error("placeDefault must not modify the file")
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

	old := initOffer
	initOffer = func(w io.Writer, in io.Reader, interactive bool, d Detected) placement { return placeAbove }
	t.Cleanup(func() { initOffer = old })

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

			called := false
			old := initOffer
			initOffer = func(w io.Writer, in io.Reader, interactive bool, d Detected) placement {
				called = true
				return placeEnd
			}
			t.Cleanup(func() { initOffer = old })

			before, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
			code, out := run(t, root, "init")
			if code != 0 {
				t.Fatalf("init exit %d:\n%s", code, out)
			}
			if called {
				t.Error("a file that already has a block or placeholder must not be offered")
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

	old := initOffer
	initOffer = func(w io.Writer, in io.Reader, interactive bool, d Detected) placement { return placeEnd }
	t.Cleanup(func() { initOffer = old })

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
