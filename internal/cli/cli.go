// Package cli implements the esc command-line interface.
package cli

import (
	"context"
	"encoding/json"
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
  esc sync [--force]             Fetch, verify, render, and write policy artifacts
                                 --force overwrites hand-edited managed regions
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
		return cmdSync(ctx, root, args[1:], stdout, stderr)
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
	case errors.Is(err, esc.ErrConfig):
		return 2
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
	if err := syncOnce(ctx, root, false, stderr, stderr); err != nil {
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

// cmdSync parses sync's own flags and returns the process exit code
// directly, matching cmdStatus/cmdDiff: a flag-parse failure is a usage
// error (exit 2), not routed through exitCode's default (exit 4).
func cmdSync(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(stderr)
	force := fs.Bool("force", false, "overwrite hand-edited managed regions instead of skipping them")
	asJSON := fs.Bool("json", false, "print a machine-readable JSON report instead of human-readable output")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *asJSON {
		return exitCode(syncJSON(ctx, root, *force, stdout), stderr)
	}
	return exitCode(syncOnce(ctx, root, *force, stdout, stderr), stderr)
}

// syncJSON runs the same fetch/verify/write pipeline as syncOnce but skips
// every human-readable progress line: --json's contract is a single JSON
// document on stdout, nothing on stderr. Findings are re-derived via
// engine.Status after Apply rather than hand-rolled from SyncResult, so the
// report reflects the actual post-write state on disk (including any
// artifact Apply declined to touch) instead of an assumption about what
// Apply did.
func syncJSON(ctx context.Context, root string, force bool, stdout io.Writer) error {
	plan, err := engine.Plan(ctx, root)
	if err != nil {
		return err
	}
	res, err := engine.Apply(root, plan, force)
	if err != nil {
		return err
	}
	updatecheck.RecordSync(ctx, root, plan.PackObjs)
	st, err := engine.Status(ctx, root)
	if err != nil {
		return err
	}
	return writeJSONReport(ctx, root, stdout, st, res)
}

// writeJSONReport resolves the reporting Collection, assembles the Report
// document, fills in Alteration.Diff for the kinds where it's meaningful
// (see engine.PopulateDiffs), and writes it to stdout as indented JSON for
// stable, diffable output. Shared by cmdStatus and cmdSync's --json paths;
// sync is nil for a status report.
func writeJSONReport(ctx context.Context, root string, stdout io.Writer, st *engine.StatusResult, sync *engine.SyncResult) error {
	// st.Plan is never nil after a successful engine.Status in production
	// (Status always sets it before returning a nil error), but NewReport
	// itself guards against a nil Plan, so this call site should too rather
	// than assume a caller never passes a hand-built StatusResult.
	var coll engine.Collection
	if st.Plan != nil {
		var err error
		coll, err = engine.ResolveReporting(st.Plan.PackObjs, st.Plan.Config)
		if err != nil {
			return err
		}
	}
	rep := engine.NewReport(st, coll, sync)
	if err := engine.PopulateDiffs(ctx, root, st.Plan, rep); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	if _, err := stdout.Write(data); err != nil {
		return err
	}
	_, err = stdout.Write([]byte("\n"))
	return err
}

// syncOnce runs one sync (fetch, verify, render, write) and reports the
// result. Shared by cmdSync (which owns --force and usage-error handling)
// and checkForUpdates' auto-sync after an accepted interactive update,
// which has no flags of its own and needs the underlying error for its own
// restore-on-failure message rather than a bare exit code.
func syncOnce(ctx context.Context, root string, force bool, stdout, stderr io.Writer) error {
	plan, err := engine.Plan(ctx, root)
	if err != nil {
		return err
	}
	res, err := engine.Apply(root, plan, force)
	if err != nil {
		return err
	}
	updatecheck.RecordSync(ctx, root, plan.PackObjs)
	skipped := make(map[string]bool, len(res.Skipped))
	for _, s := range res.Skipped {
		skipped[s.Path] = true
	}
	// Applied and skipped counts both go on stdout: len(plan.Artifacts)
	// alone would report a declined artifact as "synced" to anyone reading
	// only stdout, when its content is stderr-only.
	fmt.Fprintf(stdout, "Synced %d pack(s), %d artifact(s) applied, %d skipped:\n",
		len(plan.Packs), len(res.Applied), len(res.Skipped))
	for _, a := range plan.Artifacts {
		mark := ""
		if skipped[a.Path] {
			mark = "  (skipped, see warning below)"
		}
		fmt.Fprintf(stdout, "  %-10s %s%s\n", a.Kind, a.Path, mark)
	}
	// A declined artifact must not fail the rollout: sync still exits 0.
	// Exit-0 from sync no longer asserts the repo matches policy; compliance
	// gating belongs on `esc status --check`.
	for _, s := range res.Skipped {
		fmt.Fprintf(stderr, "  skipped %s: %s\n", s.Path, s.Reason)
	}
	if len(res.Skipped) > 0 {
		fmt.Fprintln(stderr, "  `esc diff` to inspect, `esc sync --force` to overwrite")
	}
	return nil
}

func cmdStatus(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	check := fs.Bool("check", false, "exit non-zero when any artifact is not in sync")
	asJSON := fs.Bool("json", false, "print a machine-readable JSON report instead of human-readable output")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	// The interactive update-check prompt writes progress to stderr and can
	// read from stdin; --json's contract is a clean, non-interactive
	// machine-readable document, so it must never trigger that prompt.
	if !*asJSON {
		checkForUpdates(ctx, root, stderr)
	}
	st, err := engine.Status(ctx, root)
	if err != nil {
		return exitCode(err, stderr)
	}
	// st.Plan is never nil after a successful engine.Status in production,
	// but guard it anyway rather than assume every caller (or future test)
	// passes a fully-populated StatusResult (see writeJSONReport's identical
	// guard).
	var coll engine.Collection
	if st.Plan != nil {
		coll, err = engine.ResolveReporting(st.Plan.PackObjs, st.Plan.Config)
		if err != nil {
			return exitCode(err, stderr)
		}
	}
	hasViolation := false
	for _, f := range st.Findings {
		if f.State == engine.ConstraintViolated {
			hasViolation = true
		}
	}
	if *asJSON {
		if err := writeJSONReport(ctx, root, stdout, st, nil); err != nil {
			return exitCode(err, stderr)
		}
	} else {
		for _, f := range st.Findings {
			suffix := ""
			if f.Amendment != nil {
				switch {
				case len(f.Amendment.Items) > 0:
					suffix = fmt.Sprintf("  ·  %d unmanaged %s preserved",
						len(f.Amendment.Items), plural(len(f.Amendment.Items), "file", "files"))
				default:
					suffix = fmt.Sprintf("  ·  +%d lines local", f.Amendment.Lines)
				}
			}
			if f.State == engine.InSync {
				fmt.Fprintf(stdout, "  ✓ %-20s in sync%s\n", f.Subject, suffix)
				continue
			}
			fmt.Fprintf(stdout, "  ✗ %-20s %s: %s%s\n", f.Subject, f.State, f.Detail, suffix)
			if f.State == engine.Altered {
				fmt.Fprintf(stdout, "  %-22s `esc diff` to inspect · `esc sync --force` to overwrite\n", "")
			}
		}
		if st.Clean() {
			fmt.Fprintln(stdout, "All policy artifacts in sync.")
		} else {
			fmt.Fprintln(stdout, "Drift detected. Run `esc diff` to inspect, `esc sync` to reconcile.")
		}
		// Unconditional: a team must be able to discover that its own
		// additions are reported upstream without reading the pack
		// manifest. Never gated behind a verbose flag.
		if coll.Amendments != engine.ReportOff && anyAmendment(st.Findings) {
			what := "counts and hashes only"
			if coll.Amendments == engine.ReportContent {
				what = "including content"
			}
			fmt.Fprintf(stdout, "\nLocal amendments are reported upstream, %s.\n", what)
			fmt.Fprintln(stdout, "(pack policy; set report_amendments: metrics or off in .escapement.yaml to withhold)")
		}
	}
	if st.Clean() {
		return 0
	}
	// Constraint violations (e.g. an unacknowledged custom target, §2.3) are
	// fail-closed: they block Apply outright, so status must surface them as a
	// non-zero exit even without --check, not just as ordinary drift. Orphaned
	// managed blocks (§3) are drift-family, not violations — the next sync
	// self-heals them, so they are reported but do not force a non-zero exit
	// without --check.
	if *check || hasViolation {
		return 1
	}
	return 0
}

// plural returns one when n == 1, many otherwise.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// anyAmendment reports whether any finding carries a local amendment,
// gating the collection notice in cmdStatus.
func anyAmendment(fs []engine.Finding) bool {
	for _, f := range fs {
		if f.Amendment != nil {
			return true
		}
	}
	return false
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
