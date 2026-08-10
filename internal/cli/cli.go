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
	"strings"
	"time"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/engine"
	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/lockfile"
	"github.com/tensorgroup/openescapement/internal/pack"
	"github.com/tensorgroup/openescapement/internal/publisher"
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
  esc init [--yes]                Scaffold .escapement/config.yaml
                                 --yes accepts the recommended (no-write)
                                 answer to every placement prompt
  esc sync [--force]             Fetch, verify, render, and write policy artifacts
                                 --force overwrites hand-edited managed regions
  esc status [--check]           Report drift; --check exits non-zero on findings
  esc diff [--against REF [--source SRC]]
                                 Show drift diffs, or policy changes vs another ref
  esc update [--source SRC] --ref REF
                                 Bump a pack pin in config + lock (run sync after)
  esc render --stdout            Print rendered targets without writing
  esc serve [--demo] [--addr ADDR] [--data-dir DIR] [--token TOKEN]
                                 Launch the admin portal server
                                 --token sets the auth token (default: random)
  esc pack add-skill URL[#subdir] [--ref REF] [--only a,b]
  esc pack update-skill [name...] [--all] [--ref REF] [--force]
  esc pack outdated [--check]
                                 Author commands (run in the pack repo):
                                 vendor and maintain external skills
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
		return cmdInit(ctx, root, args[1:], stdout, stderr)
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
	case "pack":
		return cmdPack(ctx, root, args[1:], stdout, stderr)
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
	case errors.Is(err, esc.ErrConfig), errors.Is(err, esc.ErrUsage):
		return 2
	default:
		return 4
	}
}

// configTemplate renders the scaffolded config.yaml. targets, when non-empty,
// pre-fills the targets list from what esc init detected already in the repo
// (see detectExisting) so a first sync does not conjure a target file the
// repo never had. An empty targets list is omitted entirely: empty means
// "all targets" (config.Config.Targets), the correct default for a
// greenfield repo, and writing an empty `targets:` key would instead mean
// zero targets.
func configTemplate(targets []string) string {
	var b strings.Builder
	b.WriteString(`schema: 1
packs: []
# Example:
#   packs:
#     - source: github.com/acme/policy-packs//org
#       ref: v1.0.0
#     - source: ../local-packs/team   # local dirs need explicit trust
#       ref: ""
#       trust: unsigned
allowed_signers_file: .escapement/allowed_signers
`)
	if len(targets) > 0 {
		b.WriteString("# Detected in this repo at `esc init`. Remove a line to stop managing that target.\ntargets:\n")
		for _, t := range targets {
			b.WriteString("  - " + t + "\n")
		}
	}
	return b.String()
}

const signersTemplate = `# SSH allowed signers for pack verification (see ssh-keygen -Y).
# Format: <principal> <key-type> <public-key>
# policy-team@acme.example ssh-ed25519 AAAA...
`

// cmdInit parses init's own flags and returns the process exit code
// directly, matching cmdSync/cmdStatus/cmdDiff: a flag-parse failure (e.g.
// `esc init -h`) is a usage error (exit 2), not routed through exitCode's
// generic default (exit 4).
func cmdInit(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	yes := fs.Bool("yes", false, "take the recommended answer instead of asking\n(the recommended answer declines the placement marker, so --yes never modifies a detected file)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	// init takes no positional arguments, and silently ignoring one is not a
	// harmless nicety: `esc init /some/other/repo` looks like it targeted
	// that path and instead scaffolds .escapement/ into the current
	// directory. That exact mistake happened while this command was being
	// built. Refuse it as a usage error rather than write somewhere the
	// caller plainly did not mean.
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "esc init takes no arguments (got %q); run it from the repository root, or use `cd`\n", fs.Arg(0))
		return 2
	}
	cfgPath := config.Path(root)
	if _, err := os.Stat(cfgPath); err == nil {
		return exitCode(fmt.Errorf("%s already exists", cfgPath), stderr)
	}
	// Detection is read-only and runs before anything is written: a
	// detection error must abort before the repo is touched.
	detected, err := detectExisting(root)
	if err != nil {
		return exitCode(err, stderr)
	}
	if err := os.MkdirAll(filepath.Join(root, config.Dir), 0o755); err != nil {
		return exitCode(err, stderr)
	}
	if err := os.WriteFile(cfgPath, []byte(configTemplate(detectedTargets(detected))), 0o644); err != nil {
		return exitCode(err, stderr)
	}
	signers := filepath.Join(root, config.Dir, "allowed_signers")
	if _, err := os.Stat(signers); os.IsNotExist(err) {
		if err := os.WriteFile(signers, []byte(signersTemplate), 0o644); err != nil {
			return exitCode(err, stderr)
		}
	}
	gitignore := filepath.Join(root, config.Dir, ".gitignore")
	if _, err := os.Stat(gitignore); os.IsNotExist(err) {
		if err := os.WriteFile(gitignore, []byte("update-log.jsonl\n"), 0o644); err != nil {
			return exitCode(err, stderr)
		}
	}
	if len(detected) == 0 {
		fmt.Fprintf(stdout, "Initialized %s\nAdd pack sources to the config, then run `esc sync`.\n", cfgPath)
		return 0
	}
	fmt.Fprintf(stdout, "Initialized %s\n\n", cfgPath)
	explainDetection(stdout, detected)
	// Offer comes last: everything above it is config scaffolding that a
	// user who declines every offer still ends up with, working. A failure
	// here (as opposed to a routine dirty-file skip, which offerPlacement
	// only prints) still leaves that scaffolding in place, but must not
	// exit 0: a script relying on the exit code needs to be able to tell a
	// placement write actually failed.
	return exitCode(offerPlacement(ctx, root, stdout, detected, *yes), stderr)
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
// The destination's existing permission bits are preserved rather than
// forced to 0o644: a caller's CLAUDE.md at 0o600, say, must not be silently
// widened to world-readable just because escapement rewrote it. A
// destination that doesn't exist yet falls back to 0o644.
func restoreFile(path string, content []byte) error {
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
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
	if err := os.Chmod(tmp.Name(), mode); err != nil {
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
		return exitCode(syncJSON(ctx, root, *force, stdout, stderr), stderr)
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
func syncJSON(ctx context.Context, root string, force bool, stdout, stderr io.Writer) error {
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
	rep, coll, err := writeJSONReport(ctx, root, stdout, st, res)
	if err != nil {
		return err
	}
	// Publish runs only after the lockfile write (engine.Apply, above),
	// updatecheck.RecordSync, and every byte of stdout output: nothing past
	// this point may change the exit code (see publisher.Publish's doc
	// comment for why it has no error return at all).
	publisher.Publish(ctx, root, rep, packsOf(st), coll, stderr, time.Now())
	return nil
}

// buildPublishReport assembles the same Report document writeJSONReport
// puts on the wire (NewReport, then PopulateDiffs for the diff-bearing
// finding kinds) so the JSON path and the two human-output paths that also
// publish (syncOnce, cmdStatus's text branch) share one assembly pipeline
// rather than growing a second, divergent one.
func buildPublishReport(ctx context.Context, root string, st *engine.StatusResult, coll engine.Collection, sync *engine.SyncResult) (*engine.Report, error) {
	rep := engine.NewReport(st, coll, sync)
	if err := engine.PopulateDiffs(ctx, root, st.Plan, rep); err != nil {
		return nil, err
	}
	return rep, nil
}

// packsOf returns the packs behind st's Plan, or nil if st carries no Plan
// (st.Plan is never nil after a successful engine.Status in production, but
// every caller here guards it anyway rather than assume a hand-built
// StatusResult always sets it, matching writeJSONReport's own guard).
func packsOf(st *engine.StatusResult) []*pack.Pack {
	if st.Plan == nil {
		return nil
	}
	return st.Plan.PackObjs
}

// writeJSONReport resolves the reporting Collection, assembles the Report
// document via buildPublishReport, and writes it to stdout as indented JSON
// for stable, diffable output. Shared by cmdStatus and cmdSync's --json
// paths; sync is nil for a status report. Returns the assembled Report and
// Collection so callers can hand them to publisher.Publish without
// rebuilding either.
func writeJSONReport(ctx context.Context, root string, stdout io.Writer, st *engine.StatusResult, sync *engine.SyncResult) (*engine.Report, engine.Collection, error) {
	// st.Plan is never nil after a successful engine.Status in production
	// (Status always sets it before returning a nil error), but NewReport
	// itself guards against a nil Plan, so this call site should too rather
	// than assume a caller never passes a hand-built StatusResult.
	var coll engine.Collection
	if st.Plan != nil {
		var err error
		coll, err = engine.ResolveReporting(st.Plan.PackObjs, st.Plan.Config)
		if err != nil {
			return nil, engine.Collection{}, err
		}
	}
	rep, err := buildPublishReport(ctx, root, st, coll, sync)
	if err != nil {
		return nil, engine.Collection{}, err
	}
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return nil, engine.Collection{}, err
	}
	if _, err := stdout.Write(data); err != nil {
		return nil, engine.Collection{}, err
	}
	if _, err := stdout.Write([]byte("\n")); err != nil {
		return nil, engine.Collection{}, err
	}
	return rep, coll, nil
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
		skipped[s.Subject] = true
	}
	adopted := make(map[string]bool, len(res.Adopted))
	for _, s := range res.Adopted {
		adopted[s] = true
	}
	// Applied and skipped counts both go on stdout: len(plan.Artifacts)
	// alone would report a declined artifact as "synced" to anyone reading
	// only stdout, when its content is stderr-only. Adopted artifacts are
	// folded into neither count — nothing was written (unlike Applied) and
	// nothing was declined (unlike Skipped) — so they get their own mark
	// below and their own stderr notice, not a count in this line.
	fmt.Fprintf(stdout, "Synced %d pack(s), %d artifact(s) applied, %d skipped:\n",
		len(plan.Packs), len(res.Applied), len(res.Skipped))
	for _, a := range plan.Artifacts {
		mark := ""
		switch {
		case skipped[a.Path]:
			mark = "  (skipped, see warning below)"
		case adopted[a.Path]:
			mark = "  (adopted, see notice below)"
		}
		fmt.Fprintf(stdout, "  %-10s %s%s\n", a.Kind, a.Path, mark)
	}
	// A declined artifact must not fail the rollout: sync still exits 0.
	// Exit-0 from sync no longer asserts the repo matches policy; compliance
	// gating belongs on `esc status --check`.
	for _, s := range res.Skipped {
		fmt.Fprintf(stderr, "  skipped %s: %s\n", s.Subject, s.Reason)
		fmt.Fprintf(stderr, "    %s\n", skipHint(s))
	}
	for _, s := range res.Adopted {
		fmt.Fprintf(stderr, "  adopted %s: existing directory matches pack content exactly\n", s)
	}
	publishSyncResult(ctx, root, res, stderr)
	return nil
}

// publishSyncResult resolves fresh status and the reporting collection
// purely to feed publisher.Publish: syncOnce's own stdout/stderr output
// above is already complete, and the lockfile write and RecordSync already
// happened in syncOnce before this call, so everything from here on is the
// publish side channel. Any failure resolving status or the reporting
// level here is reported on stderr and swallowed rather than returned:
// nothing this deep in an already-successful sync may change its exit
// code, the same non-fatality Publish itself guarantees for the network
// half of this path.
func publishSyncResult(ctx context.Context, root string, sync *engine.SyncResult, stderr io.Writer) {
	st, err := engine.Status(ctx, root)
	if err != nil {
		fmt.Fprintf(stderr, "esc: resolving status for publish failed: %v\n", err)
		return
	}
	var coll engine.Collection
	if st.Plan != nil {
		coll, err = engine.ResolveReporting(st.Plan.PackObjs, st.Plan.Config)
		if err != nil {
			fmt.Fprintf(stderr, "esc: resolving reporting level for publish failed: %v\n", err)
			return
		}
	}
	rep, err := buildPublishReport(ctx, root, st, coll, sync)
	if err != nil {
		fmt.Fprintf(stderr, "esc: building publish report failed: %v\n", err)
		return
	}
	publisher.Publish(ctx, root, rep, packsOf(st), coll, stderr, time.Now())
}

// skipHint is the one-line remedy printed under a declined artifact.
//
// This used to be a single blanket line, `esc diff` to inspect · `esc sync
// --force` to overwrite, printed once after every skip. It is only true of a
// hand-edited artifact still in the effective set. engine.PopulateDiffs
// populates diffs for Altered findings only, so for either orphan decline
// `esc diff` prints nothing at all, and for the unmanaged-file decline
// --force is not the answer either: force is consent to overwrite
// escapement's own content, and those files are the team's. Each line names
// what is actually on disk and what would resolve it.
func skipHint(s engine.Skipped) string {
	switch s.Cause {
	case engine.SkipOrphanDirUnmanaged:
		return "the pack files are gone and the rest is yours · delete that directory to be rid of it"
	case engine.SkipOrphanDirEdited:
		// Not "revert the edit": the same cause fires when a pack-provided
		// file was deleted or became unreadable, and there is nothing to
		// revert then. The whole-manifest hash cannot name the file either,
		// so the remedy has to describe the end state rather than the action.
		return "nothing was removed · restore the pack files as synced, or `esc sync --force` to retire the directory"
	case engine.SkipOrphanBlockEdited:
		return "the block is still in that file · `esc sync --force` to remove it"
	case engine.SkipUnmanagedDirAtTarget:
		return "escapement never owned that directory · move it aside, then `esc sync` (`--force` does not override this)"
	default:
		return "`esc diff` to inspect · `esc sync --force` to overwrite"
	}
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
	hasViolation := false
	for _, f := range st.Findings {
		if f.State == engine.ConstraintViolated {
			hasViolation = true
		}
	}
	if *asJSON {
		rep, coll, err := writeJSONReport(ctx, root, stdout, st, nil)
		if err != nil {
			return exitCode(err, stderr)
		}
		// After all stdout output: see syncJSON's identical placement note.
		publisher.Publish(ctx, root, rep, packsOf(st), coll, stderr, time.Now())
	} else {
		// st.Plan is never nil after a successful engine.Status in production,
		// but guard it anyway rather than assume every caller (or future test)
		// passes a fully-populated StatusResult (see writeJSONReport's identical
		// guard). Resolved here, not before the *asJSON branch above, because
		// only this human-output path consumes coll; writeJSONReport resolves
		// its own independently, and the JSON path must not pay for a second
		// resolution it never uses.
		var coll engine.Collection
		if st.Plan != nil {
			coll, err = engine.ResolveReporting(st.Plan.PackObjs, st.Plan.Config)
			if err != nil {
				return exitCode(err, stderr)
			}
		}
		for _, f := range st.Findings {
			suffix := ""
			if f.Amendment != nil {
				// Items and Content are mutually exclusive in every producer
				// today (newAmendment, internal/engine/amendment.go): block/file
				// findings set Content, dir/json-keys findings set Items, never
				// both. If a future amendment ever carried both, this switch
				// would silently drop the Lines count in favor of the Items
				// count; revisit this comment if that invariant ever changes.
				switch {
				case len(f.Amendment.Items) > 0:
					suffix = fmt.Sprintf("  ·  %d unmanaged %s preserved",
						len(f.Amendment.Items), plural(len(f.Amendment.Items), "file", "files"))
				default:
					suffix = fmt.Sprintf("  ·  +%d lines local", f.Amendment.Lines)
				}
			}
			if f.Duplicate != nil {
				if f.Duplicate.Same {
					suffix += "  ·  also installed at user level (same content)"
				} else {
					suffix += "  ·  also installed at user level (differs)"
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
		//
		// Future tense, deliberately, even now that Publish exists (below):
		// whether THIS run actually transmits anything still depends on the
		// pack declaring reporting.endpoint and ESC_PORTAL_TOKEN being set,
		// neither of which this notice's condition (coll.Amendments,
		// anyAmendment) inspects. A present-tense claim here would overstate
		// what a repo with a resolved level but no endpoint/token configured
		// actually sends, which is nothing.
		if coll.Amendments != engine.ReportOff && anyAmendment(st.Findings) {
			what := "counts and hashes only"
			if coll.Amendments == engine.ReportContent {
				what = "including content"
			}
			fmt.Fprintf(stdout, "\nLocal amendments will be reported upstream once a publisher is configured, %s.\n", what)
			fmt.Fprintln(stdout, "(pack policy; set report_amendments: metrics or off in .escapement.yaml to withhold)")
		}
		// After all stdout output above, exactly like the *asJSON branch:
		// build the same Report shape writeJSONReport would have (via the
		// shared buildPublishReport) purely to feed Publish. A failure here
		// is reported and swallowed, never returned, so it cannot change
		// this command's exit code any more than a publish failure itself
		// could.
		if rep, err := buildPublishReport(ctx, root, st, coll, nil); err != nil {
			fmt.Fprintf(stderr, "esc: building publish report failed: %v\n", err)
		} else {
			publisher.Publish(ctx, root, rep, packsOf(st), coll, stderr, time.Now())
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
