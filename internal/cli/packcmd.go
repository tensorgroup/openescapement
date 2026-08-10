// Package-author command family: esc pack {add-skill, update-skill,
// outdated}. All run in the pack repo root (where pack.yaml lives), not in a
// governed repo, and reuse internal/source's cache and detached-checkout
// machinery plus the pack.DirFiles walk.
package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/engine"
	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/pack"
	"github.com/tensorgroup/openescapement/internal/source"
)

const packUsage = `usage:
  esc pack add-skill URL[#subdir] [--ref REF] [--only a,b]
  esc pack update-skill [name...] [--all] [--ref REF] [--force]
  esc pack outdated [--check]
`

func cmdPack(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, packUsage)
		return 2
	}
	switch args[0] {
	case "add-skill":
		return exitCode(cmdPackAddSkill(ctx, root, args[1:], stdout, stderr), stderr)
	case "update-skill":
		return exitCode(cmdPackUpdateSkill(ctx, root, args[1:], stdout, stderr), stderr)
	case "outdated":
		return exitCode(cmdPackOutdated(ctx, root, args[1:], stdout, stderr), stderr)
	default:
		fmt.Fprintf(stderr, "esc pack: unknown subcommand %q\n\n%s", args[0], packUsage)
		return 2
	}
}

// loadAuthorPack loads and validates the pack rooted at root, and prints any
// lint warnings (e.g. floating MCP pins) to stderr — the authoring commands
// are the authoring surface, so this is where an author hears about them.
func loadAuthorPack(root string, stderr io.Writer) (*pack.Pack, error) {
	p, err := pack.Load(root)
	if err != nil {
		return nil, err
	}
	for _, w := range p.Warnings {
		fmt.Fprintf(stderr, "warning: %s\n", w)
	}
	return p, nil
}

// resolveSkillRef picks what to vendor: an explicit --ref as given, else the
// highest semver tag, else the default branch head with a warning (spec §4:
// the pin is exact either way because the resolved commit is always
// recorded). recorded is what sources.yaml stores as ref ("HEAD" for the
// fallback); fetchRef is what source.Fetch checks out (the head's commit SHA
// for the fallback — "HEAD" itself is not a stable remote ref name to pin).
func resolveSkillRef(ctx context.Context, url, refFlag string, stderr io.Writer) (recorded, fetchRef string, err error) {
	if refFlag != "" {
		return refFlag, refFlag, nil
	}
	tags, err := source.LsRemoteTags(ctx, url)
	if err != nil {
		return "", "", err
	}
	if name, _, ok := source.HighestSemverTag(tags); ok {
		return name, name, nil
	}
	head, err := source.LsRemoteHash(ctx, url, "HEAD")
	if err != nil {
		return "", "", err
	}
	if head == "" {
		return "", "", fmt.Errorf("%w: %s has no resolvable HEAD", esc.ErrFetch, url)
	}
	fmt.Fprintf(stderr, "warning: %s has no semver tags; vendoring the default branch head (%s)\n", url, short(head))
	return "HEAD", head, nil
}

// fetchSkillSource materializes url (with optional subdir) at fetchRef via
// the shared pack cache. Plain local directories are refused: a vendored
// skill needs a resolvable commit for provenance, and file:// serves the
// local-repo case through git.
func fetchSkillSource(ctx context.Context, root, url, subdir, fetchRef string) (*source.FetchResult, error) {
	parsed, err := source.ParseSource(url)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", esc.ErrFetch, err)
	}
	if parsed.Local {
		return nil, fmt.Errorf("%w: %s is a plain directory; vendoring needs a git source (use file://) so a commit can be recorded", esc.ErrFetch, url)
	}
	cache, err := engine.CacheDir()
	if err != nil {
		return nil, err
	}
	src := url
	if subdir != "" {
		src = url + "//" + subdir
	}
	return source.Fetch(ctx, config.PackRef{Source: src, Ref: fetchRef}, cache, root)
}
