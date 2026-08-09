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
// in the files it detected without one already.
//
// The offer is one gate ("place the marker now?", default no) and, only if
// that gate is accepted, one position question whose answer applies to every
// eligible file. It is deliberately not asked per file. The recommended
// answer is to write nothing, and declining, an empty answer, EOF,
// unrecognized input, a non-interactive run and --yes all converge on that
// same outcome; asking that same question once per detected file meant a
// repo with four instruction files got four consecutive prompts that each
// recommended doing nothing, immediately after the explanation had already
// said how to place the marker by hand. A team that wants policy to be
// literally first in the file wants that in every one of its instruction
// files, so one position answer for all of them loses nothing real.
//
// The former third option ("keep the default", which wrote nothing) is gone
// because declining the gate IS keeping the default.
type placement string

const (
	// placeDefault keeps the shipped behavior: the block lands at the top,
	// after any frontmatter and title, at first sync. It writes nothing, and
	// it is what every path other than an explicit yes-plus-position
	// produces.
	placeDefault placement = ""
	// placeAbove writes the placeholder above the title (but below any YAML
	// frontmatter, see applyPlacement) for a team that wants policy to be
	// the first thing in the file a reader or an agent meets.
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

// initInput is the interactive seam for the whole placement offer:
// production prompts on the real stdin and asks the real TTY whether it is
// one; tests inject a deterministic reader and force the flag, so no test
// blocks on real stdin and the prompt's own parsing (the gate, the position
// question, the read bound, and the reader reuse between the two questions)
// is exercised as written rather than stubbed past. Same seam pattern as
// maybeUpdates.
var initInput = func() (io.Reader, bool) { return os.Stdin, tty.IsInteractive(os.Stdin) }

// askPlacement runs the offer: one gate, then one position question asked
// only if the gate was accepted, with the answer applying to every file in
// targets. Both reads come from the same *bufio.Reader (see offerPlacement)
// so the second question cannot lose input the first one buffered past.
//
// noGitUndo makes the gate say so when root is not a git repository git will
// vouch for. Everywhere else init refuses to write what git cannot undo (a
// file with uncommitted changes is skipped for exactly that reason), but in
// a plain directory there is no undo at all to lean on, and silently
// treating that as the safest case of all would be backwards. The write is
// still offered, since a directory with no version control is a legitimate
// place to run init, but the user opts in knowing.
func askPlacement(w io.Writer, br *bufio.Reader, targets []Detected, noGitUndo bool) placement {
	fmt.Fprintf(w, "\nOptional: write %s into %s now, to choose where the block lands.\n",
		render.Placeholder, describeTargets(targets))
	for _, d := range targets {
		fmt.Fprintf(w, "  %s\n", d.Path)
	}
	if noGitUndo {
		fmt.Fprintln(w, "Note: this directory is not a git repository, so there is no undo for that write.")
	}
	fmt.Fprint(w, "Place the marker now? [y/N] ")
	switch strings.ToLower(strings.TrimSpace(readBoundedLine(br, maxOfferAnswerBytes))) {
	case "y", "yes":
	default:
		// No, empty, EOF/Ctrl-D, the read bound, and anything unrecognized
		// all land here: the recommended answer, and the one that writes
		// nothing. Never guess, never block, never grow without limit.
		return placeDefault
	}
	fmt.Fprintf(w, "Where should it go in %s?\n", describeTargets(targets))
	fmt.Fprintln(w, "  [a] above the title, the first thing in the file (below any frontmatter)")
	fmt.Fprintln(w, "  [e] end of the file")
	fmt.Fprint(w, "> ")
	switch strings.ToLower(strings.TrimSpace(readBoundedLine(br, maxOfferAnswerBytes))) {
	case string(placeAbove):
		return placeAbove
	case string(placeEnd):
		return placeEnd
	default:
		// An unrecognized position is not a licence to pick one: fall back
		// to writing nothing, the same as declining the gate.
		return placeDefault
	}
}

// describeTargets names the files the one position answer will apply to:
// the single file by name, or a count, since listing four paths inline in
// two separate sentences reads worse than the list printed above the gate.
func describeTargets(targets []Detected) string {
	if len(targets) == 1 {
		return targets[0].Path
	}
	return fmt.Sprintf("all %d files", len(targets))
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
// outcome the offer promised for that choice, so there is nothing to write.
//
// placeAbove is "above the title", not "at byte 0". Writing at byte 0
// preserves every byte of a file that opens with YAML frontmatter and still
// destroys it: frontmatter is only frontmatter when it starts the file, so a
// marker above it demotes `---\ntitle: x\n---` to a setext heading plus a
// horizontal rule. The offset comes from render.FrontmatterEnd, the same
// parser render.Splice's own top placement uses, so there is exactly one
// answer in the codebase to "where does the frontmatter end".
func applyPlacement(root string, d Detected, p placement) error {
	if p == placeDefault {
		return nil
	}
	path := filepath.Join(root, d.Path)
	existing, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var at int
	switch p {
	case placeAbove:
		at = render.FrontmatterEnd(existing)
	case placeEnd:
		at = len(existing)
	default:
		return fmt.Errorf("applyPlacement: unknown placement %q", p)
	}
	// Mirrors render.Splice's own append-at-end guard (block.go's Splice,
	// the len(s)==at case), generalized to any insertion point: the marker
	// must start its own line. Without this, a file with no trailing newline
	// (or one whose frontmatter fence has none) would get the placeholder
	// glued onto the preceding byte — "rules" becoming
	// "rules<!-- escapement:block -->" — and when sync later substitutes the
	// block at that exact index, the begin marker lands mid-line.
	sep := ""
	if at > 0 && existing[at-1] != '\n' {
		sep = "\n"
	}
	out := make([]byte, 0, len(existing)+len(sep)+len(render.Placeholder)+1)
	out = append(out, existing[:at]...)
	out = append(out, sep...)
	out = append(out, render.Placeholder+"\n"...)
	out = append(out, existing[at:]...)
	// Same atomic-write discipline as restoreFile: temp file + rename in the
	// destination directory (preserving the original file's mode), so a
	// crash mid-write never leaves a half-written instruction file behind
	// and a write never widens the file's permissions.
	return restoreFile(path, out)
}

// gitCommand builds one git invocation scoped to root (`-C root`), with
// LC_ALL=C forced so that anything about git's behavior or output that does
// depend on its own messages doesn't also depend on the caller's locale.
// The one shared constructor both isGitRepo and fileIsClean use, so the
// locale setting can't be forgotten on one call site and not the other.
func gitCommand(ctx context.Context, root string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	return cmd
}

// isGitRepo reports whether root is inside a git work tree that git is
// currently willing to operate on, via `git rev-parse --is-inside-work-tree`
// and its exit code alone — never by matching any of git's several possible
// fatal messages, which are both locale-dependent and not worth keeping in
// sync with git's own wording as it changes across versions.
//
// Every non-zero exit is treated the same: no .git present at all, and a
// "detected dubious ownership" refusal (routine in containers and CI with
// mounted volumes, git >= 2.35.2) both mean git cannot vouch for this path
// right now, and get the same answer, false with no error — the caller
// decides what tolerant behavior that implies. The one exception is the git
// binary itself being missing, which is a Go-level check
// (errors.Is(err, exec.ErrNotFound)), not a message match, so it's
// distinguished as its own error instead of silently folded into "false".
func isGitRepo(ctx context.Context, root string) (bool, error) {
	if err := gitCommand(ctx, root, "rev-parse", "--is-inside-work-tree").Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return false, fmt.Errorf("git not found in PATH: %w", err)
		}
		return false, nil
	}
	return true, nil
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
// root not being (or not currently being usable as) a git work tree at all
// — see isGitRepo — is treated as CLEAN rather than erroring: `esc init`
// must work in a repo before its first commit, and the same tolerance
// extends to a repo git currently refuses for any other reason (dubious
// ownership, say), since there is no way to ask it for a real answer
// either way. Failing closed here instead would be worse: `esc init &&
// esc sync` or `esc init || exit 1` would break in exactly the
// environments where init otherwise did everything correctly, over a
// question (is this one file dirty) that was never load-bearing enough to
// justify that.
func fileIsClean(ctx context.Context, root, path string) (bool, error) {
	inRepo, err := isGitRepo(ctx, root)
	if err != nil {
		return false, err
	}
	if !inRepo {
		return true, nil
	}
	return gitFileIsClean(ctx, root, path)
}

// gitFileIsClean is fileIsClean's second half, split out so offerPlacement
// can ask the repo-ness question once for the whole run instead of paying a
// `git rev-parse` subprocess per detected file.
func gitFileIsClean(ctx context.Context, root, path string) (bool, error) {
	cmd := gitCommand(ctx, root, "status", "--porcelain", "--", path)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("git status %s: %w: %s", path, err, strings.TrimSpace(errOut.String()))
	}
	return out.Len() == 0, nil
}

// initInteractive decides whether `esc init` should prompt at all, and with
// what reader. --yes forces this false even on a real TTY: it exists so a
// scripted run never stalls waiting for input, not so it can accept a file
// write — the recommended answer never writes one anyway. The early return
// on yes also guarantees the seam is not consulted at all, which is what
// makes it safe to pin with a unit test that doesn't control stdin.
func initInteractive(yes bool) (io.Reader, bool) {
	if yes {
		return nil, false
	}
	return initInput()
}

// offerPlacement asks once whether to write the placeholder marker, once
// where it should go, and applies that one answer to every eligible detected
// file. Called only after detection and config scaffolding are already
// written (see cmdInit): the offer can only add a placeholder, never undoes
// anything, so a user who declines still ends up with a working config.
//
// It returns immediately when the run is not interactive. Nothing below the
// gate can write a byte in that case, so doing the work anyway meant `esc
// init --yes` spending up to one git subprocess per detected file and
// printing per-file "skipping the placement offer" lines about a prompt that
// was never going to be shown.
//
// A file already carrying a managed block or a placeholder is never
// offered — detectExisting already found the right sync clause for it, and
// writing a second placeholder or a stray marker into a file that already
// has one would just corrupt it. A file with uncommitted changes is
// reported (on stdout: expected, not a failure) and left out of the offer,
// because git is the undo mechanism for anything init writes; the other
// detected files are still offered. Where there is no git at all, the gate
// says so instead (see askPlacement): the honest statement of the rule is
// that init does not write over changes git cannot restore, and does not
// write anywhere at all without being asked.
//
// A placement write actually failing (applyPlacement returning an error) is
// different from a routine skip: it is collected and returned rather than
// only ever printed to stdout, so cmdInit can turn it into a non-zero exit
// and a caller that isn't reading prose can still tell something went
// wrong. A git-check failure (fileIsClean erroring — git missing, or some
// other unexpected git failure) is NOT treated the same way: it is printed
// and that one file is skipped, exactly like a routine dirty-file skip,
// and never fails the overall command. init's config scaffolding already
// fully succeeded by the time the offer runs (see cmdInit's ordering), and
// the placement offer is best-effort on top of it — the same reasoning
// that makes a repo git can't currently vouch for tolerated as "clean"
// (see fileIsClean) means a failure answering that question can't be
// allowed to fail `esc init` outright either; `esc init && esc sync` (or
// `esc init || exit 1`) must not break just because the git check itself
// couldn't run. Each detected file is still attempted regardless of an
// earlier one's outcome, failure or not.
func offerPlacement(ctx context.Context, root string, stdout io.Writer, detected []Detected, yes bool) error {
	in, interactive := initInteractive(yes)
	if !interactive {
		return nil
	}
	// One bufio.Reader for the whole offer. A fresh one per question would
	// silently swallow whatever the first read buffered past its newline,
	// losing the position answer whenever both arrive together (a pasted
	// "y\na\n", or any piped input).
	br := bufio.NewReader(in)

	// The repo-ness question is asked once for the whole offer, not once per
	// file: it is the same answer every time, it decides what the gate says
	// about undo, and a failure to answer it (git missing from PATH) is a
	// property of the environment rather than of any one file, so it is
	// reported once and skips the offer entirely at exit 0 — init's config
	// scaffolding already succeeded, and a git-check failure must not turn
	// that into a non-zero exit.
	inRepo, err := isGitRepo(ctx, root)
	if err != nil {
		fmt.Fprintf(stdout, "  could not check git status, skipping the placement offer: %v\n", err)
		return nil
	}
	var eligible []Detected
	for _, d := range orderDetected(detected) {
		if !isFileTarget(d.Target) || d.HasBlock || d.HasPlaceholder {
			continue
		}
		if inRepo {
			clean, err := gitFileIsClean(ctx, root, d.Path)
			if err != nil {
				fmt.Fprintf(stdout, "  %s: could not check git status, skipping the placement offer: %v\n", d.Path, err)
				continue
			}
			if !clean {
				fmt.Fprintf(stdout, "  %s: uncommitted changes, skipping the placement offer (git could not undo a write here)\n", d.Path)
				continue
			}
		}
		eligible = append(eligible, d)
	}
	if len(eligible) == 0 {
		return nil
	}
	p := askPlacement(stdout, br, eligible, !inRepo)
	if p == placeDefault {
		return nil
	}
	var errs []error
	for _, d := range eligible {
		if err := applyPlacement(root, d, p); err != nil {
			errs = append(errs, fmt.Errorf("%s: could not write the placement marker: %w", d.Path, err))
		}
	}
	return errors.Join(errs...)
}
