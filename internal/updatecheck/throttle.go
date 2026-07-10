// internal/updatecheck/throttle.go
package updatecheck

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/lockfile"
	"github.com/tensorgroup/openescapement/internal/pack"
)

// Decision is the throttle's result when a prompt happened.
type Decision struct {
	Accepted bool
	Packs    []PackStatus
}

// Maybe performs an update check if one is overdue, logs the outcome, prints a
// stderr notice when updates exist, and (interactive TTY only) prompts. It
// never blocks or fails the invoking command: all errors log an `error`
// outcome plus one short stderr line and return nil.
func Maybe(ctx context.Context, root string, stdin *os.File, stderr io.Writer) *Decision {
	entries, err := LoadLog(root)
	if err != nil {
		return nil
	}
	last := LastEntry(entries)
	if last == nil || last.Cadence == "" {
		return nil // inert: feature never activated (or explicitly deactivated)
	}
	cadence, err := pack.ParseEvery(last.Cadence)
	if err != nil {
		return nil
	}
	if ls := LastSuccess(entries); ls != nil && time.Since(ls.Time) < cadence {
		return nil // not overdue
	}

	cctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	cfg, err := config.Load(root)
	if err != nil {
		return nil
	}
	lock, _ := lockfile.Load(root)
	statuses, cerr := check(cctx, cfg, lock)

	entry := Entry{Time: time.Now().UTC(), Cadence: last.Cadence, Prompt: "none"}
	if cerr != nil {
		entry.Outcome = OutcomeError
		fmt.Fprintf(stderr, "esc: update check failed: %v\n", cerr)
		_ = appendLog(root, entry)
		return nil
	}
	entry.Packs = statuses
	if !anyUpdates(statuses) {
		entry.Outcome = OutcomeOKCurrent
		_ = appendLog(root, entry)
		return nil
	}
	entry.Outcome = OutcomeOKUpdates
	printNotice(stderr, statuses)
	if !isInteractive(stdin) {
		_ = appendLog(root, entry) // non-TTY: notice only, never a prompt
		return nil
	}
	accept := readPromptDecision(stdin, stderr)
	if accept {
		entry.Prompt = "accepted"
	} else {
		entry.Prompt = "declined"
	}
	_ = appendLog(root, entry)
	return &Decision{Accepted: accept, Packs: statuses}
}

// RecordSync records the check entry a successful sync implies (sync already
// fetched upstream state). It recomputes cadence from the just-synced
// manifests, so dropping update_check from all packs makes the feature inert.
func RecordSync(ctx context.Context, root string, packs []*pack.Pack) {
	cadence, cadStr := Cadence(packs)
	entries, _ := LoadLog(root)
	if cadence == 0 {
		if len(entries) > 0 {
			// Was active, now inert: write a terminal empty-cadence marker so
			// the throttle stops on the next command.
			_ = appendLog(root, Entry{Time: time.Now().UTC(), Outcome: OutcomeOKCurrent, Prompt: "none"})
		}
		return
	}
	cctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	cfg, err := config.Load(root)
	if err != nil {
		return
	}
	lock, _ := lockfile.Load(root)
	statuses, cerr := check(cctx, cfg, lock)
	entry := Entry{Time: time.Now().UTC(), Cadence: cadStr, Prompt: "none"}
	if cerr != nil {
		entry.Outcome = OutcomeError
		_ = appendLog(root, entry)
		return
	}
	entry.Packs = statuses
	if anyUpdates(statuses) {
		entry.Outcome = OutcomeOKUpdates
	} else {
		entry.Outcome = OutcomeOKCurrent
	}
	_ = appendLog(root, entry)
}

func anyUpdates(statuses []PackStatus) bool {
	for _, s := range statuses {
		if s.Updates {
			return true
		}
	}
	return false
}

func printNotice(w io.Writer, statuses []PackStatus) {
	fmt.Fprintln(w, "esc: policy pack updates available:")
	for _, s := range statuses {
		if s.Updates {
			fmt.Fprintf(w, "  %s: %s -> %s\n", s.Source, s.Pinned, s.Latest)
		}
	}
}

// isInteractive reports whether f is a character device (a real terminal).
// Uses only os.Stat — no golang.org/x/term dependency.
func isInteractive(f *os.File) bool {
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// readPromptDecision prints the prompt and reads one line: empty = accept,
// anything else (including an ESC escape sequence) = decline.
func readPromptDecision(r io.Reader, w io.Writer) bool {
	fmt.Fprint(w, "Update now? [Enter=yes, n=no]: ")
	line, _ := bufio.NewReader(r).ReadString('\n')
	return strings.TrimRight(line, "\r\n") == ""
}
