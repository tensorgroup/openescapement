// internal/cli/initoffer.go
package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tensorgroup/openescapement/internal/render"
	"github.com/tensorgroup/openescapement/internal/tty"
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

// maxOfferAnswerBytes bounds how much of one prompt answer initOffer reads
// before giving up and treating the input as unrecognized. tty.IsInteractive
// cannot distinguish a real terminal from something like /dev/zero, which
// stats as a character device and never produces a newline; without a
// bound, reading byte-by-byte looking for '\n' would run forever against
// such a stream (`esc init < /dev/zero`). The real answer is always a
// single character, so this cap is generous but finite.
const maxOfferAnswerBytes = 64

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
	switch strings.TrimSpace(readBoundedLine(br, maxOfferAnswerBytes)) {
	case string(placeAbove):
		return placeAbove
	case string(placeEnd):
		return placeEnd
	default:
		// Empty or unrecognized input returns placeDefault, same as a
		// read error (EOF/Ctrl-D) or hitting the byte bound: never guess,
		// never block, never grow without limit.
		return placeDefault
	}
}

// readBoundedLine reads up to max bytes from br looking for '\n' and returns
// whatever it collected either way (a short read on error, or a truncated
// answer at the bound, both just fail to match a recognized answer below).
func readBoundedLine(br *bufio.Reader, max int) string {
	buf := make([]byte, 0, max)
	for len(buf) < max {
		b, err := br.ReadByte()
		if err != nil || b == '\n' {
			break
		}
		buf = append(buf, b)
	}
	return string(buf)
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
		// Mirrors render.Splice's own append-at-end guard (block.go's
		// Splice, the len(s)==at case): only insert a separating newline
		// when the file doesn't already end in one. Without this, a file
		// with no trailing newline (or an empty file) would get the
		// placeholder glued onto its last byte — e.g. "rules" would become
		// "rules<!-- escapement:block -->", and when sync later substitutes
		// the block at that exact index, the managed block's begin marker
		// lands mid-line, immediately after "rules" with no separator.
		sep := ""
		if len(existing) > 0 && existing[len(existing)-1] != '\n' {
			sep = "\n"
		}
		out = make([]byte, 0, len(existing)+len(sep)+len(render.Placeholder)+1)
		out = append(out, existing...)
		out = append(out, sep...)
		out = append(out, []byte(render.Placeholder+"\n")...)
	default:
		return fmt.Errorf("applyPlacement: unknown placement %q", p)
	}
	// Same atomic-write discipline as restoreFile: temp file + rename in the
	// destination directory (preserving the original file's mode), so a
	// crash mid-write never leaves a half-written instruction file behind
	// and a write never widens the file's permissions.
	return restoreFile(path, out)
}

// fileIsClean reports whether path (root-relative, forward-slash separated,
// as stored in Detected.Path) has no uncommitted changes, via `git status
// --porcelain -- <path>`. Any output — modified, staged, or untracked — means
// dirty.
//
// The status is scoped to path via a pathspec (`-- <path>`) rather than run
// bare: a bare `git status --porcelain` in root would also report the
// .escapement/ directory and config files esc init itself just wrote as
// untracked changes, which would make every detected file look dirty on the
// very first run in an existing git repo. Pathspec-scoping keeps the check
// about the one file being offered.
//
// A root that is not a git repository at all is treated as CLEAN rather than
// erroring: `esc init` must work in a repo before its first commit, and with
// no git history yet, there is nothing for git to consider dirty against
// (and nothing for it to undo either, but that just means the guard this
// feeds does not apply yet — it doesn't mean init should refuse to run).
func fileIsClean(ctx context.Context, root, path string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", root, "status", "--porcelain", "--", path)
	// git's "not a git repository" message is locale-dependent (gettext);
	// matching it in the user's own language would silently miss and turn
	// a should-be-clean pre-first-commit repo into a hard error instead
	// (fail-safe, since the offer just gets skipped everywhere, but dead).
	// Force the C locale for this one invocation so the match is stable
	// regardless of the user's environment.
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return false, fmt.Errorf("git not found in PATH, cannot check %s for uncommitted changes: %w", path, err)
		}
		if strings.Contains(errOut.String(), "not a git repository") {
			return true, nil
		}
		return false, fmt.Errorf("git status %s: %w: %s", path, err, strings.TrimSpace(errOut.String()))
	}
	return out.Len() == 0, nil
}

// initInteractive decides whether `esc init` should prompt at all. --yes
// forces this false even on a real TTY: it exists so a scripted run never
// stalls waiting for input, not so it can accept a file write — the
// recommended answer (placeDefault) never writes one anyway. Go's
// short-circuit && guarantees this regardless of the actual terminal, which
// is what makes it safe to pin with a unit test that doesn't control stdin.
func initInteractive(yes bool) bool {
	return !yes && tty.IsInteractive(os.Stdin)
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
// reported (on stdout: expected, not a failure) and skipped even when
// interactive, because git is the undo mechanism for anything init writes,
// and init must not write somewhere git cannot undo it; the other detected
// files are still processed.
//
// A genuine failure — the git check itself erroring, or a placement write
// failing — is different from a routine skip: it is collected and returned
// rather than only ever printed to stdout, so cmdInit can turn it into a
// non-zero exit and a caller that isn't reading prose can still tell
// something went wrong. Each detected file is still attempted regardless of
// an earlier one's failure.
func offerPlacement(ctx context.Context, root string, stdout io.Writer, detected []Detected, yes bool) error {
	interactive := initInteractive(yes)
	var br *bufio.Reader
	if interactive {
		br = bufio.NewReader(os.Stdin)
	}
	var errs []error
	for _, d := range orderDetected(detected) {
		if !isFileTarget(d.Target) || d.HasBlock || d.HasPlaceholder {
			continue
		}
		clean, err := fileIsClean(ctx, root, d.Path)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: could not check git status, skipped the placement offer: %w", d.Path, err))
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
			errs = append(errs, fmt.Errorf("%s: could not write the placement marker: %w", d.Path, err))
		}
	}
	return errors.Join(errs...)
}
