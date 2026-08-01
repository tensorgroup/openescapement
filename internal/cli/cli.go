// Package cli implements the esc command-line interface.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/engine"
	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/lockfile"
	"github.com/tensorgroup/openescapement/internal/updatecheck"
)

// Version, Commit, and Date are stamped at release time via -ldflags
// (see .goreleaser.yaml). Defaults identify from-source dev builds.
var (
	Version = "0.1.0-dev"
	Commit  = "unknown"
	Date    = "unknown"
)

const usage = `esc — deterministic governance for your AI usage

Usage:
  esc init                       Scaffold .escapement/config.yaml
  esc sync                       Fetch, verify, render, and write policy artifacts
  esc status [--check]           Report drift; --check exits non-zero on findings
  esc diff [--against REF [--source SRC]]
                                 Show drift diffs, or policy changes vs another ref
  esc update [--source SRC] --ref REF
                                 Bump a pack pin in config + lock (run sync after)
  esc render --stdout            Print rendered targets without writing
  esc serve [--demo] [--addr ADDR] [--data-dir DIR]
                                 Launch the admin portal server
  esc version                    Print version

Exit codes: 0 ok · 1 drift/constraint findings · 2 usage · 3 integrity/signature · 4 error
`

// Run executes esc in the repo rooted at root. Returns the process exit code.
func Run(root string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	// Bound all git/network work so a hostile or dead remote can't hang CI.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	var err error
	switch args[0] {
	case "init":
		err = cmdInit(root, stdout)
	case "sync":
		err = cmdSync(ctx, root, stdout)
	case "status":
		return cmdStatus(ctx, root, args[1:], stdout, stderr)
	case "diff":
		return cmdDiff(ctx, root, args[1:], stdout, stderr)
	case "update":
		err = cmdUpdate(ctx, root, args[1:], stdout, stderr)
	case "render":
		err = cmdRender(ctx, root, args[1:], stdout, stderr)
	case "serve":
		// No ctx: serve runs until SIGINT/SIGTERM, well past the 10-minute
		// timeout above, and builds its own signal-bound context.
		return cmdServe(root, args[1:], stdout, stderr)
	case "version":
		fmt.Fprintf(stdout, "esc %s (commit %s, built %s)\n", Version, Commit, Date)
		return 0
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "esc: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
	return exitCode(err, stderr)
}

func exitCode(err error, stderr io.Writer) int {
	if err == nil {
		return 0
	}
	fmt.Fprintf(stderr, "esc: %v\n", err)
	switch {
	case errors.Is(err, esc.ErrSignature), errors.Is(err, esc.ErrLockMismatch):
		return 3
	case errors.Is(err, esc.ErrConstraint):
		return 1
	default:
		return 4
	}
}

const configTemplate = `schema: 1
packs: []
# Example:
#   packs:
#     - source: github.com/acme/policy-packs//org
#       ref: v1.0.0
#     - source: ../local-packs/team   # local dirs need explicit trust
#       ref: ""
#       trust: unsigned
allowed_signers_file: .escapement/allowed_signers
`

const signersTemplate = `# SSH allowed signers for pack verification (see ssh-keygen -Y).
# Format: <principal> <key-type> <public-key>
# policy-team@acme.example ssh-ed25519 AAAA...
`

func cmdInit(root string, stdout io.Writer) error {
	cfgPath := config.Path(root)
	if _, err := os.Stat(cfgPath); err == nil {
		return fmt.Errorf("%s already exists", cfgPath)
	}
	if err := os.MkdirAll(filepath.Join(root, config.Dir), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(cfgPath, []byte(configTemplate), 0o644); err != nil {
		return err
	}
	signers := filepath.Join(root, config.Dir, "allowed_signers")
	if _, err := os.Stat(signers); os.IsNotExist(err) {
		if err := os.WriteFile(signers, []byte(signersTemplate), 0o644); err != nil {
			return err
		}
	}
	gitignore := filepath.Join(root, config.Dir, ".gitignore")
	if _, err := os.Stat(gitignore); os.IsNotExist(err) {
		if err := os.WriteFile(gitignore, []byte("update-log.jsonl\n"), 0o644); err != nil {
			return err
		}
	}
	fmt.Fprintf(stdout, "Initialized %s\nAdd pack sources to the config, then run `esc sync`.\n", cfgPath)
	return nil
}

// maybeUpdates is the update-check seam: production wires the real TTY-backed
// throttle; tests inject updatecheck.MaybeIO with a deterministic interactivity
// flag and prompt reader so no test can ever block on real stdin.
var maybeUpdates = func(ctx context.Context, root string, stderr io.Writer) *updatecheck.Decision {
	return updatecheck.Maybe(ctx, root, os.Stdin, stderr)
}

// checkForUpdates runs the overdue throttle before a command's main logic. On
// an interactive accept it re-pins each stale tag pack to its latest tag and
// syncs (a bare sync would keep the old pin for tag-pinned packs). If that
// sync fails, the original config is restored so an automatic prompt can never
// strand a pin that was never verified. It never fails the invoking command.
func checkForUpdates(ctx context.Context, root string, stderr io.Writer) {
	d := maybeUpdates(ctx, root, stderr)
	if d == nil || !d.Accepted {
		return
	}
	cfgPath := config.Path(root)
	orig, err := os.ReadFile(cfgPath)
	if err != nil {
		fmt.Fprintf(stderr, "esc: applying updates failed: %v\n", err)
		return
	}
	changed, err := bumpPins(root, d.Packs)
	if err != nil {
		fmt.Fprintf(stderr, "esc: applying updates failed: %v\n", err)
		return
	}
	if err := cmdSync(ctx, root, stderr); err != nil {
		if !changed {
			// bumpPins never touched config.yaml, so there is nothing to
			// restore — reporting a restore would be spurious.
			fmt.Fprintf(stderr, "esc: update sync failed: %v\n", err)
			return
		}
		if rerr := restoreFile(cfgPath, orig); rerr != nil {
			fmt.Fprintf(stderr, "esc: update sync failed: %v (config restore also failed: %v)\n", err, rerr)
			return
		}
		fmt.Fprintf(stderr, "esc: update aborted, config restored: %v\n", err)
	}
}

// restoreFile atomically writes content back to path (temp file + rename in
// the destination directory, matching the repo's atomic-write invariant).
func restoreFile(path string, content []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".esc-restore-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// bumpPins re-pins each stale tag pack in config.yaml to its latest tag.
// Branch pins are left unchanged — a plain sync picks up the new tip. The
// returned changed is true iff config.yaml was modified and saved, so callers
// can tell a genuine rewrite apart from a no-op (e.g. only branch pins were
// stale).
func bumpPins(root string, statuses []updatecheck.PackStatus) (changed bool, err error) {
	cfg, err := config.Load(root)
	if err != nil {
		return false, err
	}
	for _, s := range statuses {
		if !s.Updates || s.Kind != "tag" {
			continue
		}
		for i := range cfg.Packs {
			if cfg.Packs[i].Source == s.Source {
				cfg.Packs[i].Ref = s.Latest
				changed = true
			}
		}
	}
	if !changed {
		return false, nil
	}
	if err := cfg.Save(root); err != nil {
		return false, err
	}
	return true, nil
}

func cmdSync(ctx context.Context, root string, stdout io.Writer) error {
	plan, err := engine.Plan(ctx, root)
	if err != nil {
		return err
	}
	if err := engine.Apply(root, plan); err != nil {
		return err
	}
	updatecheck.RecordSync(ctx, root, plan.PackObjs)
	fmt.Fprintf(stdout, "Synced %d pack(s), %d artifact(s):\n", len(plan.Packs), len(plan.Artifacts))
	for _, a := range plan.Artifacts {
		fmt.Fprintf(stdout, "  %-10s %s\n", a.Kind, a.Path)
	}
	return nil
}

func cmdStatus(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	check := fs.Bool("check", false, "exit non-zero when any artifact is not in sync")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	checkForUpdates(ctx, root, stderr)
	st, err := engine.Status(ctx, root)
	if err != nil {
		return exitCode(err, stderr)
	}
	hasViolation := false
	for _, f := range st.Findings {
		if f.State == engine.InSync {
			fmt.Fprintf(stdout, "  ✓ %-20s in sync\n", f.Path)
		} else {
			fmt.Fprintf(stdout, "  ✗ %-20s %s: %s\n", f.Path, f.State, f.Detail)
		}
		if f.State == engine.ConstraintViolated || f.State == engine.Orphan {
			hasViolation = true
		}
	}
	if st.Clean() {
		fmt.Fprintln(stdout, "All policy artifacts in sync.")
		return 0
	}
	fmt.Fprintln(stdout, "Drift detected. Run `esc diff` to inspect, `esc sync` to reconcile.")
	// Constraint violations (e.g. an unacknowledged custom target, §2.3) and
	// orphaned managed blocks (a target that left the effective set but still
	// has stale content on disk, §3) are fail-closed: they represent content
	// that should not exist as-is, so status must surface them as a non-zero
	// exit even without --check, not just as ordinary drift.
	if *check || hasViolation {
		return 1
	}
	return 0
}

func cmdDiff(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	against := fs.String("against", "", "compare rendered policy against another pack ref")
	src := fs.String("source", "", "which configured pack source to re-pin (for --against)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	checkForUpdates(ctx, root, stderr)
	cur, err := engine.Plan(ctx, root)
	if err != nil {
		return exitCode(err, stderr)
	}
	var out string
	if *against == "" {
		out, err = engine.DriftDiff(ctx, root, cur)
	} else {
		var next *engine.PlanResult
		next, err = engine.PlanWithRef(ctx, root, *src, *against)
		if err == nil {
			out, err = engine.PolicyDiff(ctx, cur, next)
		}
	}
	if err != nil {
		return exitCode(err, stderr)
	}
	fmt.Fprint(stdout, out)
	if out != "" {
		return 1
	}
	return 0
}

func cmdUpdate(ctx context.Context, root string, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(stdout)
	src := fs.String("source", "", "which configured pack source to update")
	ref := fs.String("ref", "", "new ref to pin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	checkForUpdates(ctx, root, stderr)
	if *ref == "" {
		return errors.New("update: --ref is required")
	}
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	idx, err := engine.SelectSource(cfg, *src)
	if err != nil {
		return err
	}
	old := cfg.Packs[idx].Ref
	cfg.Packs[idx].Ref = *ref
	if err := cfg.Save(root); err != nil {
		return err
	}
	// Refresh the lock's pack resolution; artifacts stay as last synced.
	plan, err := engine.Plan(ctx, root)
	if err != nil {
		return err
	}
	prev, err := lockfile.Load(root)
	if err != nil {
		return err
	}
	lock := &lockfile.Lock{Schema: 1, Packs: plan.Packs}
	if prev != nil {
		lock.Artifacts = prev.Artifacts
	}
	if err := lock.Save(root); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Updated %s: %s → %s\nReview with `esc diff` and apply with `esc sync`.\n", cfg.Packs[idx].Source, old, *ref)
	return nil
}

func cmdRender(ctx context.Context, root string, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	fs.SetOutput(stdout)
	toStdout := fs.Bool("stdout", false, "print rendered targets to stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	checkForUpdates(ctx, root, stderr)
	if !*toStdout {
		return errors.New("render: only --stdout is supported (sync writes files)")
	}
	plan, err := engine.Plan(ctx, root)
	if err != nil {
		return err
	}
	for _, a := range plan.Artifacts {
		if a.Kind != engine.KindBlock && a.Kind != engine.KindFile {
			continue
		}
		fmt.Fprintf(stdout, "===== %s =====\n%s\n", a.Path, a.Body)
	}
	return nil
}
