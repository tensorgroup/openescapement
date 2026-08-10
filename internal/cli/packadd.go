package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tensorgroup/openescapement/internal/pack"
)

// cmdPackAddSkill vendors skills from an external repo into the pack repo:
// fetch, discover by SKILL.md, copy under skills/<name>/, append object
// entries to pack.yaml, record provenance in sources.yaml. It prints what
// was vendored at which commit so the author reviews the diff before
// committing (spec §4). All refusals happen before anything is written.
func cmdPackAddSkill(ctx context.Context, root string, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("pack add-skill", flag.ContinueOnError)
	fs.SetOutput(stderr)
	ref := fs.String("ref", "", "ref to vendor (default: highest semver tag, else default branch head)")
	only := fs.String("only", "", "comma-separated skill names to vendor from a multi-skill repo")
	if err := fs.Parse(reorderFlags(fs, args)); err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("%w: esc pack add-skill takes exactly one URL[#subdir]", errUsage)
	}
	p, err := loadAuthorPack(root, stderr)
	if err != nil {
		return err
	}
	url, subdir := splitSkillURL(fs.Arg(0))
	recorded, fetchRef, err := resolveSkillRef(ctx, url, *ref, stderr)
	if err != nil {
		return err
	}
	fr, err := fetchSkillSource(ctx, root, url, subdir, fetchRef)
	if err != nil {
		return err
	}
	fallback := repoBaseName(url)
	if subdir != "" {
		fallback = filepath.Base(filepath.FromSlash(subdir))
	}
	skills, err := discoverSkills(fr.Dir, fallback)
	if err != nil {
		return err
	}
	if *only != "" {
		skills, err = filterSkills(skills, strings.Split(*only, ","))
		if err != nil {
			return err
		}
	}
	// Refuse taken names BEFORE writing anything: resolved on-disk names of
	// existing entries, existing skills/<name> dirs, and sources.yaml entries
	// all count (spec §3: add-skill refuses a name already present).
	srcs, err := pack.LoadSources(root)
	if err != nil {
		return err
	}
	if srcs == nil {
		srcs = &pack.Sources{Schema: 1}
	}
	taken := map[string]bool{}
	for _, e := range p.Manifest.Skills {
		taken[e.DirName(p.Manifest.Name)] = true
	}
	for _, d := range skills {
		if taken[d.Name] {
			return fmt.Errorf("skill name %q is already present in this pack", d.Name)
		}
		if srcs.Skill(d.Name) != nil {
			return fmt.Errorf("skill name %q is already recorded in %s", d.Name, pack.SourcesFile)
		}
		if _, serr := os.Lstat(filepath.Join(root, "skills", d.Name)); serr == nil {
			return fmt.Errorf("skills/%s already exists in the pack repo", d.Name)
		}
	}
	var entries []pack.SkillEntry
	for _, d := range skills {
		files, hash, err := vendorCopy(d.Dir, filepath.Join(root, "skills", d.Name))
		if err != nil {
			return err
		}
		entries = append(entries, pack.SkillEntry{Path: "skills/" + d.Name, Name: d.Name})
		srcs.Upsert(pack.SourceSkill{
			Name: d.Name, Source: url, Subdir: relSkillSubdir(subdir, fr.Dir, d.Dir),
			Ref: recorded, Commit: fr.Commit, Hash: hash,
		})
		fmt.Fprintf(stdout, "vendored %s (%d files) from %s@%s at %s\n", d.Name, len(files), url, recorded, fr.Commit)
	}
	if err := appendSkillEntries(filepath.Join(root, "pack.yaml"), entries); err != nil {
		return err
	}
	if err := srcs.Save(root); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "review the diff, then commit the pack repo.")
	return nil
}

// filterSkills narrows to the named skills; an unknown name is an error, not
// a silent no-op.
func filterSkills(all []discoveredSkill, names []string) ([]discoveredSkill, error) {
	byName := map[string]discoveredSkill{}
	for _, d := range all {
		byName[d.Name] = d
	}
	var out []discoveredSkill
	for _, n := range names {
		n = strings.TrimSpace(n)
		d, ok := byName[n]
		if !ok {
			return nil, fmt.Errorf("--only %s: no such skill in the source (have: %s)", n, skillNames(all))
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func skillNames(ds []discoveredSkill) string {
	names := make([]string, len(ds))
	for i, d := range ds {
		names[i] = d.Name
	}
	return strings.Join(names, ", ")
}

// relSkillSubdir records where inside the upstream repo the skill lives:
// the add-time subdir joined with the skill dir's position under the fetch
// dir. Recorded so update-skill can re-fetch the same tree narrowly.
func relSkillSubdir(subdir, fetchDir, skillDir string) string {
	rel, err := filepath.Rel(fetchDir, skillDir)
	if err != nil || rel == "." {
		return subdir
	}
	rel = filepath.ToSlash(rel)
	if subdir == "" {
		return rel
	}
	return subdir + "/" + rel
}

// errUsage marks a flag/arity mistake in the pack command family so
// exitCode maps it to 2 via esc.ErrConfig-style handling.
var errUsage = errors.New("usage")

// reorderFlags moves flag tokens (and, for non-boolean flags, the value
// token immediately following) to the front of args, positional arguments
// after — so fs.Parse accepts the usage-string order this command family
// documents (`esc pack add-skill URL [--ref REF] [--only a,b]`), which the
// stdlib flag package cannot: Parse stops consuming flags at the first
// non-flag token, so a positional URL before a flag would otherwise strand
// that flag in Args() instead of setting it. Boolean flags are detected via
// the same interface{ IsBoolFlag() bool } hook the flag package itself uses
// internally, so a future boolean flag (e.g. update-skill's --all/--force)
// is not mistaken for taking a value.
func reorderFlags(fs *flag.FlagSet, args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if len(a) < 2 || a[0] != '-' {
			positional = append(positional, a)
			continue
		}
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		flags = append(flags, a)
		name := strings.TrimLeft(a, "-")
		if strings.Contains(name, "=") {
			continue // value embedded, e.g. --ref=v1.0.0
		}
		isBool := false
		if fl := fs.Lookup(name); fl != nil {
			if bf, ok := fl.Value.(interface{ IsBoolFlag() bool }); ok && bf.IsBoolFlag() {
				isBool = true
			}
		}
		if !isBool && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, positional...)
}
