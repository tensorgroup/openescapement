// internal/cli/initoffer.go
package cli

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tensorgroup/openescapement/internal/render"
)

// placement is where `esc init` offers to put the managed-block placeholder
// in a file it detected without one already.
//
// There are exactly three options, not four. The design spec's "top
// (recommended)" and "after the leading title" are the same position:
// render.Splice's own top placement already inserts after any leading YAML
// frontmatter and a leading H1, so offering both would mean the recommended
// choice writes a placeholder that changes nothing. Collapsing them removes
// a distinction a user would otherwise have to think about for no gain, and
// it has a consequence worth keeping: placeAbove writes at byte 0 and
// placeDefault writes nothing, so neither needs any copy of the
// frontmatter-and-H1 parsing logic. That logic stays solely in
// render.Splice (see insertAt).
type placement string

const (
	// placeDefault keeps the shipped behavior: the block lands at the top,
	// after any frontmatter and title, at first sync. It writes nothing —
	// the same outcome as a declined, empty, or unrecognized answer.
	placeDefault placement = "k"
	// placeAbove writes the placeholder at byte 0, above the title, for a
	// team that wants policy to be the literal first thing in the file.
	placeAbove placement = "a"
	// placeEnd writes the placeholder at the end of the file.
	placeEnd placement = "e"
)

// initOffer is the placement-prompt seam: production reads a real TTY, tests
// inject a deterministic reader — or replace this var outright, the same
// pattern maybeUpdates uses — so no test can ever block on real stdin.
var initOffer = func(w io.Writer, in io.Reader, interactive bool, d Detected) placement {
	if !interactive {
		return placeDefault
	}
	fmt.Fprintf(w, "Where should the managed block go in %s?\n", d.Path)
	fmt.Fprintln(w, "  [k] keep default: top of the file, below any title (recommended)")
	fmt.Fprintln(w, "  [a] above the title, the very first thing in the file")
	fmt.Fprintln(w, "  [e] end of the file")
	fmt.Fprint(w, "> ")
	// A brand-new bufio.Reader wrapping a brand-new underlying reader on
	// every call would silently swallow buffered-ahead bytes the moment
	// init offers more than one file in a session: each fresh bufio.Reader
	// over-reads past the first line and discards what it didn't return
	// when it goes out of scope. Reuse the caller's *bufio.Reader when it
	// hands us one, so buffered state survives across files in the same
	// run; only wrap a raw reader (as direct calls and tests do) once.
	br, ok := in.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(in)
	}
	line, _ := br.ReadString('\n')
	switch strings.TrimSpace(line) {
	case string(placeAbove):
		return placeAbove
	case string(placeEnd):
		return placeEnd
	default:
		// Empty or unrecognized input returns placeDefault, same as a
		// read error (EOF/Ctrl-D): never guess, never block.
		return placeDefault
	}
}

// applyPlacement writes render.Placeholder into the detected file at the
// chosen position, preserving every original byte. placeDefault writes
// nothing: sync's own top placement (render.Splice) already produces the
// outcome the offer promised for that choice, so there is nothing to write
// and no parsing of "where does the top of this file begin" here — that
// logic stays solely in render.Splice.
func applyPlacement(root string, d Detected, p placement) error {
	if p == placeDefault {
		return nil
	}
	path := filepath.Join(root, d.Path)
	existing, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var out []byte
	switch p {
	case placeAbove:
		// Byte 0, above everything — no frontmatter/title parsing needed.
		out = append([]byte(render.Placeholder+"\n"), existing...)
	case placeEnd:
		out = make([]byte, 0, len(existing)+len(render.Placeholder)+1)
		out = append(out, existing...)
		out = append(out, []byte(render.Placeholder+"\n")...)
	default:
		return fmt.Errorf("applyPlacement: unknown placement %q", p)
	}
	// Same atomic-write discipline as restoreFile: temp file + rename in the
	// destination directory, so a crash mid-write never leaves a half-written
	// instruction file behind.
	return restoreFile(path, out)
}

// fileIsClean reports whether path (root-relative, forward-slash separated,
// as stored in Detected.Path) has no uncommitted changes, via `git status
// --porcelain -- <path>`. Any output — modified, staged, or untracked — means
// dirty.
//
// A root that is not a git repository at all is treated as CLEAN rather than
// erroring: `esc init` must work in a repo before its first commit, and with
// no git history yet, there is nothing for git to consider dirty against
// (and nothing for it to undo either, but that just means the guard this
// feeds does not apply yet — it doesn't mean init should refuse to run).
func fileIsClean(ctx context.Context, root, path string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", root, "status", "--porcelain", "--", path)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		if strings.Contains(errOut.String(), "not a git repository") {
			return true, nil
		}
		return false, fmt.Errorf("git status %s: %w: %s", path, err, strings.TrimSpace(errOut.String()))
	}
	return out.Len() == 0, nil
}

// isInteractive reports whether f is a real terminal. Starts from the same
// char-device stat check updatecheck.isInteractive uses (no golang.org/x/term
// dependency), but additionally excludes os.DevNull.
//
// /dev/null is itself a character device, so the stat check alone would
// call it interactive. That false positive is latent but rare for
// updatecheck's throttle, which only reaches its own interactive branch
// when an update check is genuinely overdue. The placement offer has no
// such gate — it is reachable on nearly every first `esc init` in a repo
// with existing instruction files — and `go test` always substitutes
// /dev/null for the test binary's stdin (verified: redirecting a test
// binary's stdin from a terminal still stats as /dev/null), so without this
// exclusion every such test, and every real invocation with stdin
// redirected from /dev/null (a common CI pattern), would print the prompt
// it should never see. Reading from /dev/null returns EOF immediately, so
// it was never a hang risk — only a spurious prompt.
func isInteractive(f *os.File) bool {
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	if fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	if null, err := os.Stat(os.DevNull); err == nil && os.SameFile(fi, null) {
		return false
	}
	return true
}

// initInteractive decides whether `esc init` should prompt at all. --yes
// forces this false even on a real TTY: it exists so a scripted run never
// stalls waiting for input, not so it can accept a file write — the
// recommended answer (placeDefault) never writes one anyway. Go's
// short-circuit && guarantees this regardless of the actual terminal, which
// is what makes it safe to pin with a unit test that doesn't control stdin.
func initInteractive(yes bool) bool {
	return !yes && isInteractive(os.Stdin)
}

// offerPlacement asks, for each detected file that could still take the
// placeholder, where the managed block should go, and applies the answer.
// Called only after detection and config scaffolding are already written
// (see cmdInit): the offer can only add a placeholder, never undoes
// anything, so a user who declines everything still ends up with a working
// config.
//
// A file already carrying a managed block or a placeholder is never
// offered — detectExisting already found the right sync clause for it, and
// writing a second placeholder or a stray marker into a file that already
// has one would just corrupt it. A file with uncommitted changes is
// reported and skipped even when interactive, because git is the undo
// mechanism for anything init writes, and init must not write somewhere git
// cannot undo it; the other detected files are still processed.
func offerPlacement(ctx context.Context, root string, stdout io.Writer, detected []Detected, yes bool) {
	interactive := initInteractive(yes)
	var br *bufio.Reader
	if interactive {
		br = bufio.NewReader(os.Stdin)
	}
	for _, d := range orderDetected(detected) {
		if !isFileTarget(d.Target) || d.HasBlock || d.HasPlaceholder {
			continue
		}
		clean, err := fileIsClean(ctx, root, d.Path)
		if err != nil {
			fmt.Fprintf(stdout, "  %s: could not check git status, skipping the placement offer: %v\n", d.Path, err)
			continue
		}
		if !clean {
			fmt.Fprintf(stdout, "  %s: uncommitted changes, skipping the placement offer (git could not undo a write here)\n", d.Path)
			continue
		}
		var in io.Reader
		if interactive {
			in = br
		}
		p := initOffer(stdout, in, interactive, d)
		if err := applyPlacement(root, d, p); err != nil {
			fmt.Fprintf(stdout, "  %s: could not write the placement marker: %v\n", d.Path, err)
		}
	}
}
