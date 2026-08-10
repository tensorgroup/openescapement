package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/pack"
	"github.com/tensorgroup/openescapement/internal/source"
)

// cmdPackOutdated compares each vendored skill's pinned commit against
// upstream (highest semver tag, or default branch head when there are no
// tags — with a warning, spec §4). Output is deterministic: rows sorted by
// name (Sources.Save keeps them sorted), fixed columns via tabwriter.
// --check exits 1 when anything is behind: the routine, self-healing CI
// semantics exit 1 carries everywhere else (esc.ErrConstraint). A network
// failure is ErrFetch (exit 4): a dead remote must not read as up to date.
func cmdPackOutdated(ctx context.Context, root string, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("pack outdated", flag.ContinueOnError)
	fs.SetOutput(stderr)
	check := fs.Bool("check", false, "exit 1 when any vendored skill is behind upstream")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %v", esc.ErrUsage, err)
	}
	if _, err := loadAuthorPack(root, stderr); err != nil {
		return err
	}
	srcs, err := pack.LoadSources(root)
	if err != nil {
		return err
	}
	if srcs == nil || len(srcs.Skills) == 0 {
		fmt.Fprintln(stdout, "no vendored skills recorded in sources.yaml")
		return nil
	}
	// One ls-remote per distinct source URL, not per skill.
	tagsBySource := map[string]map[string]string{}
	behind := 0
	tw := tabwriter.NewWriter(stdout, 2, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SKILL\tPINNED\tLATEST\tSTATE")
	for _, e := range srcs.Skills {
		tags, ok := tagsBySource[e.Source]
		if !ok {
			tags, err = source.LsRemoteTags(ctx, e.Source)
			if err != nil {
				return err
			}
			tagsBySource[e.Source] = tags
		}
		latestName, latestCommit, hasTag := source.HighestSemverTag(tags)
		if !hasTag {
			fmt.Fprintf(stderr, "warning: %s has no semver tags; comparing against the default branch head\n", e.Source)
			latestName = "HEAD"
			latestCommit, err = source.LsRemoteHash(ctx, e.Source, "HEAD")
			if err != nil {
				return err
			}
		}
		state := "up to date"
		if latestCommit != e.Commit {
			state = "behind"
			behind++
		}
		fmt.Fprintf(tw, "%s\t%s (%s)\t%s\t%s\n", e.Name, e.Ref, short(e.Commit), latestName, state)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if *check && behind > 0 {
		return fmt.Errorf("%w: %d vendored skill(s) behind upstream", esc.ErrConstraint, behind)
	}
	return nil
}

func short(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}
