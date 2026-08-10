package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/tensorgroup/openescapement/internal/pack"
)

// cmdPackUpdateSkill re-vendors named skills (or --all) at the requested ref
// (default: highest semver tag, else default branch head). Divergence gate,
// mirroring sync's hand-edit rule (spec §4): if the vendored copy no longer
// matches its recorded hash, the author edited it after vendoring, so the
// skill is warned about and skipped — and sources.yaml is left untouched so
// the divergence keeps reporting — unless --force.
func cmdPackUpdateSkill(ctx context.Context, root string, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("pack update-skill", flag.ContinueOnError)
	fs.SetOutput(stderr)
	all := fs.Bool("all", false, "update every vendored skill")
	ref := fs.String("ref", "", "ref to vendor (default: highest semver tag, else default branch head)")
	force := fs.Bool("force", false, "overwrite a vendored copy that was edited after vendoring")
	if err := fs.Parse(reorderFlags(fs, args)); err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if fs.NArg() == 0 && !*all {
		return fmt.Errorf("%w: name at least one skill, or pass --all", errUsage)
	}
	p, err := loadAuthorPack(root, stderr)
	if err != nil {
		return err
	}
	srcs, err := pack.LoadSources(root)
	if err != nil {
		return err
	}
	if srcs == nil || len(srcs.Skills) == 0 {
		return fmt.Errorf("no vendored skills recorded in %s", pack.SourcesFile)
	}
	names := fs.Args()
	if *all {
		names = nil
		for _, e := range srcs.Skills {
			names = append(names, e.Name)
		}
	}
	sort.Strings(names)
	// Validate the whole requested name set against sources.yaml before any
	// write happens: without this, an unknown name discovered mid-loop would
	// abort the command after earlier names in the same invocation had
	// already been re-vendored, leaving a partially-applied --all or
	// multi-name update behind a non-zero exit.
	for _, name := range names {
		if srcs.Skill(name) == nil {
			return fmt.Errorf("%s is not a vendored skill (not in %s)", name, pack.SourcesFile)
		}
	}
	for _, name := range names {
		entry := srcs.Skill(name)
		dir := vendoredDir(root, p, name)
		cur, err := pack.DirHash(dir)
		if err != nil {
			return fmt.Errorf("hashing skills/%s: %w", name, err)
		}
		if cur != entry.Hash && !*force {
			fmt.Fprintf(stderr, "skipped %s: the vendored copy was edited after vendoring (hash mismatch)\n", name)
			fmt.Fprintf(stderr, "  restore it as vendored, or `esc pack update-skill %s --force` to overwrite\n", name)
			continue
		}
		recorded, fetchRef, err := resolveSkillRef(ctx, entry.Source, *ref, stderr)
		if err != nil {
			return err
		}
		fr, err := fetchSkillSource(ctx, root, entry.Source, entry.Subdir, fetchRef)
		if err != nil {
			return err
		}
		// entry.Subdir points at the skill's own directory (relSkillSubdir),
		// so the fetched dir either IS the skill or holds exactly it.
		found, err := discoverSkills(fr.Dir, name)
		if err != nil {
			return err
		}
		src := ""
		for _, d := range found {
			if d.Name == name {
				src = d.Dir
			}
		}
		if src == "" {
			return fmt.Errorf("%s no longer exists upstream at %s@%s", name, entry.Source, recorded)
		}
		oldHashes, err := fileHashes(dir)
		if err != nil {
			return err
		}
		// vendorCopy refuses a pre-existing destination, and this directory
		// already holds the previous vendored copy — so stage the new copy
		// alongside it and swap only once the copy fully succeeds. A failed
		// vendorCopy leaves the original dir untouched; a failed rename is
		// reported directly rather than silently leaving a "<name>.new"
		// staging directory behind.
		staging := dir + ".update-new"
		if _, serr := os.Lstat(staging); serr == nil {
			if err := os.RemoveAll(staging); err != nil {
				return fmt.Errorf("clearing stale staging dir %s: %w", staging, err)
			}
		}
		_, hash, err := vendorCopy(src, staging)
		if err != nil {
			return err
		}
		// Diff against the staged copy, not the live directory: this is the
		// last read before the swap, so nothing fallible sits between "we
		// know what's about to land on disk" and the swap itself.
		newHashes, err := fileHashes(staging)
		if err != nil {
			os.RemoveAll(staging)
			return err
		}
		added, removed, changed := diffstat(oldHashes, newHashes)
		backup := dir + ".update-old"
		if err := os.RemoveAll(backup); err != nil {
			os.RemoveAll(staging)
			return fmt.Errorf("clearing stale backup dir %s: %w", backup, err)
		}
		if err := os.Rename(dir, backup); err != nil {
			os.RemoveAll(staging)
			return fmt.Errorf("staging old copy of %s aside: %w", name, err)
		}
		if err := os.Rename(staging, dir); err != nil {
			// Restore the previous vendored copy so a failed update never
			// leaves skills/<name> half-replaced or missing.
			if rerr := os.Rename(backup, dir); rerr != nil {
				fmt.Fprintf(stderr, "esc: restoring %s after a failed update also failed: %v\n", dir, rerr)
			}
			os.RemoveAll(staging)
			return fmt.Errorf("replacing %s with the updated copy: %w", dir, err)
		}
		// The swap is done: skills/<name> now holds the new content. The
		// invariant from here on is that sources.yaml agrees with what's on
		// disk before any further fallible step runs, so Save happens next,
		// with nothing else fallible in between — and a failed Save rolls
		// the swap back rather than stranding the new content under the old
		// recorded hash (the divergence gate would otherwise misread that as
		// a hand-edit and skip it forever).
		srcs.Upsert(pack.SourceSkill{
			Name: name, Source: entry.Source, Subdir: entry.Subdir,
			Ref: recorded, Commit: fr.Commit, Hash: hash,
		})
		if err := srcs.Save(root); err != nil {
			if rerr := os.RemoveAll(dir); rerr != nil {
				fmt.Fprintf(stderr, "esc: restoring %s after a failed sources.yaml save also failed: %v\n", dir, rerr)
			} else if rerr := os.Rename(backup, dir); rerr != nil {
				fmt.Fprintf(stderr, "esc: restoring %s after a failed sources.yaml save also failed: %v\n", dir, rerr)
			}
			return err
		}
		if err := os.RemoveAll(backup); err != nil {
			fmt.Fprintf(stderr, "esc: warning: could not remove backup dir %s: %v\n", backup, err)
		}
		fmt.Fprintf(stdout, "%s: %s -> %s (%s), %d added, %d removed, %d changed\n",
			name, entry.Ref, recorded, fr.Commit[:12], added, removed, changed)
	}
	return nil
}

// vendoredDir resolves the pack-relative directory for a vendored skill from
// the manifest entry whose resolved name matches; falls back to the
// skills/<name>/ convention add-skill writes.
func vendoredDir(root string, p *pack.Pack, name string) string {
	for _, e := range p.Manifest.Skills {
		if e.DirName(p.Manifest.Name) == name {
			return filepath.Join(root, filepath.FromSlash(e.Path))
		}
	}
	return filepath.Join(root, "skills", name)
}
