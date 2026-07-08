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
		err = cmdUpdate(ctx, root, args[1:], stdout)
	case "render":
		err = cmdRender(ctx, root, args[1:], stdout)
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
	fmt.Fprintf(stdout, "Initialized %s\nAdd pack sources to the config, then run `esc sync`.\n", cfgPath)
	return nil
}

func cmdSync(ctx context.Context, root string, stdout io.Writer) error {
	plan, err := engine.Plan(ctx, root)
	if err != nil {
		return err
	}
	if err := engine.Apply(root, plan); err != nil {
		return err
	}
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
	st, err := engine.Status(ctx, root)
	if err != nil {
		return exitCode(err, stderr)
	}
	for _, f := range st.Findings {
		if f.State == engine.InSync {
			fmt.Fprintf(stdout, "  ✓ %-20s in sync\n", f.Path)
		} else {
			fmt.Fprintf(stdout, "  ✗ %-20s %s: %s\n", f.Path, f.State, f.Detail)
		}
	}
	if st.Clean() {
		fmt.Fprintln(stdout, "All policy artifacts in sync.")
		return 0
	}
	fmt.Fprintln(stdout, "Drift detected. Run `esc diff` to inspect, `esc sync` to reconcile.")
	if *check {
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

func cmdUpdate(ctx context.Context, root string, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(stdout)
	src := fs.String("source", "", "which configured pack source to update")
	ref := fs.String("ref", "", "new ref to pin")
	if err := fs.Parse(args); err != nil {
		return err
	}
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

func cmdRender(ctx context.Context, root string, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	fs.SetOutput(stdout)
	toStdout := fs.Bool("stdout", false, "print rendered targets to stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
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
