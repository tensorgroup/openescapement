package engine

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/lockfile"
	"github.com/tensorgroup/openescapement/internal/pack"
	"github.com/tensorgroup/openescapement/internal/render"
)

// containedPath resolves rel under root and guarantees the result cannot
// escape root — the last line of defense against pack-controlled path
// components, regardless of what upstream validation missed.
func containedPath(root, rel string) (string, error) {
	abs := filepath.Join(root, filepath.FromSlash(rel))
	check, err := filepath.Rel(root, abs)
	if err != nil || check == ".." || strings.HasPrefix(check, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("artifact path %q escapes the repository root", rel)
	}
	return abs, nil
}

// refuseSymlinks fails closed if any existing path component of rel under root
// is a symlink, immediately before a write. It never follows a symlinked
// parent or target file (§2.1). A residual race between this check and the
// rename remains on shared checkouts and is accepted, documented as the same
// class as any local tooling.
//
// The refusal is a plain error (exit 4), deliberately, and matches the
// containment refusals in mergeDir below. Exit 1 is the drift-and-constraint
// class, which a CI gate reads as routine and self-healing: run sync and the
// repo converges. A symlink standing where escapement is about to write is a
// containment failure, possibly a hostile one, and no amount of syncing
// resolves it. Exit 3 would be worse than exit 1 here rather than better: it
// is the pack-integrity class (a moved tag, a tampered fetch, a bad
// signature), documented in README.md and SECURITY.md as a statement that the
// pack you pinned cannot be trusted. A symlink is a property of the local
// checkout, not of the fetched pack, so exit 3 would report the wrong
// incident.
func refuseSymlinks(root, rel string) error {
	cur := root
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		cur = filepath.Join(cur, part)
		fi, err := os.Lstat(cur)
		if err != nil {
			if os.IsNotExist(err) {
				return nil // this and deeper components do not exist yet
			}
			return err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s: refusing to write through symlink %s", rel, cur)
		}
	}
	return nil
}

// dirEntryTarget resolves one manifest entry f (a pack-relative path from a
// lockfile dir artifact) against the skill directory dir, and refuses it if
// it escapes dir, resolves to dir itself, or is reached through a symlink.
// artPath only names the artifact in the error text.
//
// A lockfile is a committed, PR-reachable artifact, so its Files entries are
// not trusted input. Both rejections return a plain error (exit 4), not
// esc.ErrConstraint: see the exit-class note on refuseSymlinks. An escaping
// or self-targeting entry aborts the whole sync rather than being silently
// skipped, because the caller needs to know its lockfile is carrying a
// hostile entry.
//
// Callers must run this over every entry BEFORE reading any of them, and
// again immediately before each filesystem call. The second pass is not
// redundant: a check is only as good as its distance from the operation it
// guards, and the first pass exists because reads (hashing) happen before
// the removal loop is reached at all.
func dirEntryTarget(dir, artPath, f string) (string, error) {
	target, err := containedPath(dir, f)
	if err != nil {
		return "", fmt.Errorf("lockfile entry %q for %s: %w", f, artPath, err)
	}
	if target == dir {
		return "", fmt.Errorf("lockfile entry %q for %s resolves to the skill directory itself; refusing to remove it", f, artPath)
	}
	if err := refuseSymlinks(dir, f); err != nil {
		return "", err
	}
	return target, nil
}

// validateDirEntries runs dirEntryTarget over every entry and discards the
// resolved paths. This is the "before anything reads them" pass.
func validateDirEntries(dir, artPath string, files []string) error {
	for _, f := range files {
		if _, err := dirEntryTarget(dir, artPath, f); err != nil {
			return err
		}
	}
	return nil
}

// Skipped is one artifact Apply declined to write or remove because local
// work stands in the way: a human hand-edited its managed region, or a
// retiring skill directory still holds files the team added. Sync leaves it
// exactly as found, converges everything else, and reports it here rather
// than silently overwriting or deleting local work.
//
// Subject names the artifact, matching Finding.Subject: the two describe the
// same concept in one JSON document (a status report's findings[] and a sync
// report's skipped[]), so they must not disagree on what to call it.
//
// Cause is the machine-readable discriminator. Reason is prose written for a
// human reading stderr; nothing can branch on it, and Kind cannot tell the
// declines apart either (two different block declines and two different dir
// declines share a Kind). Surfaces need the distinction because the advice
// differs: `esc diff` only ever covers a hand-edited managed region, since
// PopulateDiffs populates diffs for Altered findings only, so pointing every
// decline at it sends most of them to a dead end.
type Skipped struct {
	Subject      string    `json:"subject"`
	Kind         string    `json:"kind"`
	Cause        SkipCause `json:"cause"`
	Reason       string    `json:"reason"`
	ExpectedHash string    `json:"expected_hash"`
	ActualHash   string    `json:"actual_hash"`
}

// SkipCause names why Apply declined. One value per decline site.
type SkipCause string

const (
	// SkipHandEdited: the artifact is in the effective set and its managed
	// region was hand-edited. The only cause `esc diff` can show.
	SkipHandEdited SkipCause = "hand-edited"
	// SkipOrphanDirUnmanaged: the pack retired a skill directory whose
	// pack-provided files were removed, but team-added files remain, so the
	// directory itself stays.
	SkipOrphanDirUnmanaged SkipCause = "orphan-dir-unmanaged"
	// SkipOrphanDirEdited: the pack retired a skill directory in which a
	// pack-provided file was edited or removed. Nothing was removed.
	SkipOrphanDirEdited SkipCause = "orphan-dir-edited"
	// SkipOrphanBlockEdited: the target left the effective set and its
	// managed block was hand-edited, so the block was not removed.
	SkipOrphanBlockEdited SkipCause = "orphan-block-edited"
)

// SyncResult reports what a sync wrote and what it declined to write.
type SyncResult struct {
	Applied []string  `json:"applied"`
	Skipped []Skipped `json:"skipped,omitempty"`
}

// alterationActual returns the on-disk hash recorded on an Altered finding,
// or "" when the finding carries no Alteration (e.g. an unreadable dir).
func alterationActual(f Finding) string {
	if f.Alteration != nil {
		return f.Alteration.ActualHash
	}
	return ""
}

// handEdited reports whether f describes a genuine human hand-edit of
// content escapement previously wrote — the only condition under which
// Apply declines to write an artifact. Three independent concerns, each an
// early return so a future change to one can't silently paper over another:
func handEdited(f Finding, prev *lockfile.LockArtifact) bool {
	// 1. Only Altered is ever skip-worthy. Stale (pack moved on, no human
	// touched anything) must keep converging silently — that's the tool's
	// entire point — and every other state (Missing, InSync, ...) has
	// nothing to decline.
	if f.State != Altered {
		return false
	}
	// 2. Alteration is set only on classify's hash-mismatch branches, never
	// on its error branches (corrupt block markers, an unreadable dir via
	// e.g. a hijacked symlink, unparsable JSON). Treating an error branch as
	// a decline would invert fail-closed into skip-and-exit-0 for exactly
	// the hostile cases that must not be silently accepted.
	if f.Alteration == nil {
		return false
	}
	// 3. A prior lock entry is required: a target that pre-exists with
	// unrelated content before its first-ever sync (e.g. onboarding
	// escapement against an existing .mcp.json) has no managed region yet
	// to have deviated from, so it must converge normally rather than being
	// mistaken for a decline.
	if prev == nil {
		return false
	}
	return true
}

// Apply writes the planned artifacts and the lockfile. It refuses to write
// anything when the plan has constraint violations.
//
// Unless force is set, an artifact whose managed region a human hand-edited
// (classify reports Altered) is left exactly as found: sync converges every
// other artifact, records the skip in the returned SyncResult, and still
// succeeds. A repo that declines part of a policy update must not fail its
// own rollout — that is enablement over enforcement, the product's whole
// framing. force overwrites the managed region but never touches content
// outside it (mergeDir, render.Splice, and render.MergeMCP already preserve
// local amendments regardless of force; force only bypasses the classify
// skip gate above them).
func Apply(root string, p *PlanResult, force bool) (*SyncResult, error) {
	if len(p.Violations) > 0 {
		msgs := make([]string, len(p.Violations))
		for i, v := range p.Violations {
			msgs[i] = v.String()
		}
		return nil, fmt.Errorf("%w:\n  %s", esc.ErrConstraint, strings.Join(msgs, "\n  "))
	}
	prevLock, err := lockfile.Load(root)
	if err != nil {
		return nil, err
	}

	res := &SyncResult{}
	var arts []lockfile.LockArtifact
	desiredDirs := map[string]bool{}
	for _, a := range p.Artifacts {
		abs, err := containedPath(root, a.Path)
		if err != nil {
			return nil, err
		}
		if err := refuseSymlinks(root, a.Path); err != nil {
			return nil, err
		}
		if a.Kind == KindDir {
			// Set regardless of a skip below: a directory left untouched
			// because it's altered is still in the effective set and must
			// not be swept up by the orphaned-dir removal pass further down.
			desiredDirs[a.Path] = true
		}
		if !force {
			prev := prevLock.Artifact(a.Path)
			if f := classify(root, a, prevLock); handEdited(f, prev) {
				res.Skipped = append(res.Skipped, Skipped{
					Subject: a.Path, Kind: a.Kind, Cause: SkipHandEdited, Reason: f.Detail,
					ExpectedHash: a.Hash, ActualHash: alterationActual(f),
				})
				// Carry the previous lock entry forward unchanged. classify
				// distinguishes stale from altered by comparing against
				// locked.Hash, so an entry that advanced past what we last
				// actually wrote would make the alteration vanish from the
				// next status run.
				arts = append(arts, *prev)
				continue
			}
		}
		switch a.Kind {
		case KindBlock:
			existing, err := os.ReadFile(abs)
			if err != nil && !os.IsNotExist(err) {
				return nil, err
			}
			out, err := render.Splice(existing, a.Body, render.BlockMeta{Packs: a.BlockPacks})
			if err != nil {
				return nil, fmt.Errorf("%s: %w", a.Path, err)
			}
			if err := atomicWrite(abs, out); err != nil {
				return nil, err
			}
		case KindFile:
			if err := atomicWrite(abs, []byte(a.Body)); err != nil {
				return nil, err
			}
		case KindDir:
			var prevFiles []string
			if prev := prevLock.Artifact(a.Path); prev != nil {
				prevFiles = prev.Files
			}
			files, err := mergeDir(root, a.Path, a.SrcDir, abs, prevFiles)
			if err != nil {
				return nil, err
			}
			a.Files = files
		case KindJSONKeys:
			existing, err := os.ReadFile(abs)
			if err != nil && !os.IsNotExist(err) {
				return nil, err
			}
			var prevOwned []string
			if prev := prevLock.Artifact(a.Path); prev != nil {
				prevOwned = prev.Keys
			}
			merged, owned, err := render.MergeMCP(existing, a.Servers, prevOwned)
			if err != nil {
				return nil, err
			}
			if err := atomicWrite(abs, merged); err != nil {
				return nil, err
			}
			a.Keys = owned
		}
		arts = append(arts, lockfile.LockArtifact{Path: a.Path, Kind: a.Kind, Hash: a.Hash, Keys: a.Keys, Files: a.Files})
		res.Applied = append(res.Applied, a.Path)
	}

	// Remove owned skill dirs that no longer exist in any pack — but only the
	// files escapement itself wrote there.
	//
	// This used to be a single os.RemoveAll over the whole tree, which is
	// precisely the data loss mergeDir exists to prevent: mergeDir carefully
	// never deletes an unmanaged file on an ordinary sync, and then a pack
	// dropping the skill deleted the team's work wholesale, silently, at exit
	// 0. The retirement of a directory is not consent to delete what the team
	// put in it. So: remove exactly the paths the previous manifest recorded,
	// and remove the directory itself only if that leaves it empty.
	//
	// Every removal carries the same two checks the sibling block-removal loop
	// below and mergeDir already use, for the same reasons documented there.
	if prevLock != nil {
		for _, prev := range prevLock.Artifacts {
			if prev.Kind != KindDir || desiredDirs[prev.Path] {
				continue
			}
			// Ownership is decided on the repo-relative artifact path, never
			// on the absolute one. An absolute-path test is vacuous: a repo
			// that merely happens to live under a directory named e.g.
			// "esc-tools" satisfies it for every lockfile entry, so a hostile
			// lockfile could aim these removals at any in-repo path. Skill
			// dirs are always .claude/skills/esc-<pack>-<base> (engine.go's
			// TargetSkills case), so the "esc-" prefix on the final element
			// is the ownership marker — checked there, not anywhere in the
			// string, so a team-owned dir nested under an owned one
			// (".claude/skills/esc-x/team-notes") can never match either.
			if !strings.HasPrefix(path.Base(prev.Path), "esc-") {
				return nil, fmt.Errorf("refusing to remove %q: not an escapement-owned directory", prev.Path)
			}
			abs, err := containedPath(root, prev.Path)
			if err != nil {
				return nil, err
			}
			// containedPath is purely lexical, so it alone cannot see a
			// symlinked parent (".claude -> /etc"): without refuseSymlinks
			// these removals reach outside the repo entirely.
			if err := refuseSymlinks(root, prev.Path); err != nil {
				return nil, err
			}
			// Enumerate before removing: what is left afterwards is the
			// team's, and this is the only place that can still name it.
			unmanaged, err := unmanagedDirFiles(abs, prev.Files)
			if err != nil {
				if os.IsNotExist(err) {
					continue // directory already gone
				}
				return nil, err
			}
			// The last hole in "we never destroy your work". The loop below
			// removes every pack-provided file, and one of them may have been
			// hand-edited: that edit would go at exit 0 with nothing having
			// warned first, which is the same defect the unmanaged-file case
			// below already fixed for team-added files. The lockfile's dir
			// hash covers exactly prev.Files, so a mismatch against it means
			// at least one pack-provided file changed since escapement wrote
			// it. Decline the whole retirement in that case: nothing is
			// removed, so the edit survives along with everything beside it.
			//
			// Gated on force for the same reason every other skip is: force
			// is consent to overwrite escapement's own content, and an edited
			// pack-provided file is escapement's content.
			//
			// Gated on len(prev.Files) too. A pre-manifest lockfile records no
			// manifest, so its hash covers a file list we don't have and a
			// comparison here would be meaningless; that path already removes
			// nothing and reports every on-disk file as unmanaged below, which
			// is the conservative answer it was designed to give.
			//
			// Every read error fails closed rather than removing anyway,
			// whatever its kind: a file the team deleted, a path that is now a
			// directory, a permission failure. The reason is the same in all
			// three, and it is not that deleting is itself work to preserve —
			// it is that an unreadable manifest entry makes every OTHER entry
			// unverifiable, so the hash cannot clear them. An unverifiable
			// directory is exactly what this pass must not guess about, and
			// hard-erroring instead would fail a rollout over a state --force
			// resolves.
			//
			// Containment runs over the whole manifest before this, and that
			// order is load-bearing: DirHashOf READS every entry, so
			// validating only in the removal loop below would let a symlinked
			// entry be read through to out-of-repo content, hashed, and
			// reported as a routine exit-0 skip rather than the containment
			// error refuseSymlinks owes the caller.
			if err := validateDirEntries(abs, prev.Path, prev.Files); err != nil {
				return nil, err
			}
			if !force && len(prev.Files) > 0 {
				actual, herr := pack.DirHashOf(abs, prev.Files)
				if herr != nil || actual != prev.Hash {
					reason := "pack no longer provides this skill directory, but a pack-provided file in it was edited since the last sync; left in place instead of being deleted"
					if herr != nil {
						// Name the offending file. The whole-manifest hash
						// cannot say which file was edited, but a read error
						// carries its own path, and that is the one sub-case
						// where the user can be told exactly what to look at.
						reason = fmt.Sprintf("pack no longer provides this skill directory, but a pack-provided file in it is unreadable, so the others cannot be verified either (%v); left in place instead of being deleted", herr)
					}
					res.Skipped = append(res.Skipped, Skipped{
						Subject: prev.Path, Kind: prev.Kind, Cause: SkipOrphanDirEdited,
						Reason:       reason,
						ExpectedHash: prev.Hash, ActualHash: actual,
					})
					// Carry the entry forward, unlike the unmanaged-file skip
					// below. There, the pack-provided files really were
					// removed and only the team's own remain, so the entry has
					// nothing left to describe. Here nothing was removed: the
					// files are still on disk and still escapement's, and
					// dropping the entry would lose both the manifest a later
					// --force needs and the condition itself, leaving an
					// undeleted owned directory that no run ever mentions
					// again. Carrying it forward re-reports every sync until
					// someone resolves it, and three things resolve it:
					// `esc sync --force`, reverting the edit, or deleting the
					// directory.
					arts = append(arts, prev)
					continue
				}
			}
			for _, f := range prev.Files {
				// Re-resolved immediately before the removal, not reused from
				// the validation pass above: the check has to sit as close to
				// the filesystem call as it can.
				target, err := dirEntryTarget(abs, prev.Path, f)
				if err != nil {
					return nil, err
				}
				if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
					return nil, err
				}
			}
			if len(unmanaged) > 0 {
				// Reported rather than merely left behind: a directory the
				// user expects to disappear is still there, and without this
				// the only way to find out why is to go looking. Skipped is
				// the existing channel for "declined because local work is in
				// the way" and already reaches both stderr and the JSON
				// report, so this needs no new surface and, like every other
				// skip, does not fail the rollout.
				res.Skipped = append(res.Skipped, Skipped{
					Subject: prev.Path, Kind: prev.Kind, Cause: SkipOrphanDirUnmanaged,
					Reason: fmt.Sprintf("pack no longer provides this skill directory; kept because it still holds %d unmanaged file(s): %s",
						len(unmanaged), strings.Join(unmanaged, ", ")),
				})
				continue
			}
			// Best effort: os.Remove succeeds only on an empty directory, so
			// this deletes the directory exactly when nothing is left in it.
			// A lingering empty subdirectory leaves it behind with ENOTEMPTY,
			// which is cosmetic; no user content was lost either way.
			_ = os.Remove(abs)
		}
	}

	// Remove stale managed blocks for targets that left the effective set.
	// Bytes outside the block are preserved; a file left byte-empty is deleted.
	// Each removal is a write and carries the same symlink protection.
	desiredBlocks := map[string]bool{}
	for _, a := range p.Artifacts {
		if a.Kind == KindBlock {
			desiredBlocks[a.Path] = true
		}
	}
	if prevLock != nil {
		for _, prev := range prevLock.Artifacts {
			if prev.Kind != KindBlock || desiredBlocks[prev.Path] {
				continue
			}
			abs, err := containedPath(root, prev.Path)
			if err != nil {
				return nil, err
			}
			if err := refuseSymlinks(root, prev.Path); err != nil {
				return nil, err
			}
			existing, err := os.ReadFile(abs)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			// The skip gate applies here too. The main path declines to
			// overwrite a hand-edited managed region; removing that same
			// region outright is strictly more destructive, so it cannot be
			// the one deletion that bypasses the gate. Compared against
			// prev.Hash (what escapement last actually wrote) for the same
			// reason classify does: an unedited orphan still matches it and
			// is removed silently, which is the self-healing behavior the
			// orphan pass exists for. Only Extract's success path is gated;
			// its error branches (corrupt markers) fall through to
			// RemoveBlock, which fails closed as before, rather than being
			// converted into a silent skip.
			if !force {
				if block, berr := render.Extract(existing); berr == nil && block != nil {
					if actual := render.BodyHash(block.Body); actual != prev.Hash {
						res.Skipped = append(res.Skipped, Skipped{
							Subject: prev.Path, Kind: prev.Kind, Cause: SkipOrphanBlockEdited,
							Reason:       "managed block was hand-edited and its target has left the effective set; left in place instead of being removed",
							ExpectedHash: prev.Hash, ActualHash: actual,
						})
						// Carry the entry forward. The block is still on disk
						// and still escapement's content; dropping the entry
						// would make status stop reporting the orphan
						// entirely, so the next run would show a clean repo
						// with an undeleted managed block sitting in it. The
						// orphaned-dir pass above drops its entry instead,
						// and correctly: there, the managed files really were
						// removed and only the team's own remain.
						arts = append(arts, prev)
						continue
					}
				}
			}
			out, removed, err := render.RemoveBlock(existing)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", prev.Path, err)
			}
			if !removed {
				continue
			}
			if len(out) == 0 {
				if err := os.Remove(abs); err != nil {
					return nil, err
				}
				continue
			}
			if err := atomicWrite(abs, out); err != nil {
				return nil, err
			}
		}
	}

	lock := &lockfile.Lock{Schema: 1, Packs: p.Packs, Artifacts: arts}
	if err := lock.Save(root); err != nil {
		return nil, err
	}
	return res, nil
}

// atomicWrite writes via a temp file + rename in the destination directory.
func atomicWrite(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".esc-tmp-*")
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

// mergeDir reconciles dst (the repo-root-relative artifact path artPath,
// resolved under root) against the pack tree at src. Files the pack
// provides are written; files the previous manifest recorded but the pack no
// longer provides are removed; anything else on disk is left alone as a local
// amendment. Returns the pack-relative paths written, for the manifest.
//
// A pre-manifest lockfile has no prevFiles, so nothing unknown is removed on
// the first sync after upgrade. That direction can leave one pack-dropped
// file behind, reported as an amendment, rather than deleting a team's work.
//
// Atomicity tradeoff: stageDir used to swap the whole tree in one rename,
// which is exactly what destroyed unmanaged files — a directory-level
// rename has no way to skip files it didn't create. That guarantee is
// deliberately weakened here: the pack tree is staged and fully validated in
// a temp directory first, so a mid-copy pack failure can never touch dst,
// but once that staging succeeds each file is written into dst individually
// via atomicWrite (temp file + rename within dst). The result is "each file
// swaps atomically, and dst is never touched until the source is
// known-good" rather than "the whole tree swaps at once."
//
// Security: prevFiles comes from the lockfile, a committed, PR-reachable
// artifact — it is not trusted input. Every removal path is resolved with
// containedPath(dst, prev), i.e. contained against dst itself, not root:
// joining prev onto artPath first and only then containing the result
// against root would let ".." collapse before containment is ever checked,
// so an entry with enough ".." segments to land back inside the repo (but
// outside dst) would sail through — root still contains it, only dst
// doesn't. Checking straight against dst is what actually stops a lockfile
// entry like "../../../VICTIM.md" from reaching anything outside the skill
// directory, in-repo or not. An entry that normalizes to dst itself (".",
// "", or enough ".." to land exactly on dst) is rejected explicitly too,
// rather than relying on os.Remove's ENOTEMPTY to incidentally block it —
// an empty dst would otherwise be removed outright. An escaping or
// self-targeting entry aborts the whole sync (the error propagates) rather
// than being silently skipped: the caller needs to know its lockfile is
// carrying a hostile entry, the same fail-closed posture Apply already
// takes for prev.Path and for a moved tag (ErrLockMismatch). Both
// rejections deliberately return a plain error (exit 4), not
// esc.ErrConstraint: exit 1 is the drift-and-constraint class, which any CI
// gate reads as routine and expected, and a lockfile carrying a path
// traversal is neither. Every path under dst — both files staged for removal
// and files staged for writing — is also re-checked with refuseSymlinks
// immediately before the filesystem call: a symlink committed inside an
// escapement-owned dir (e.g. a nested "sub -> /etc") is not something the
// initial refuseSymlinks(root, a.Path) call in Apply can see, because that
// call only walks down to dst itself, not the files mergeDir discovers
// underneath it. The removal loop's refuseSymlinks call uses the same
// dst-relative base as its containedPath call, so the two checks agree on
// what the boundary is.
func mergeDir(root, artPath, src, dst string, prevFiles []string) ([]string, error) {
	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return nil, err
	}
	staged, err := os.MkdirTemp(parent, ".esc-stage-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(staged)
	tree := filepath.Join(staged, filepath.Base(dst))
	if err := copyDir(src, tree); err != nil {
		return nil, err
	}

	var written []string
	if err := filepath.WalkDir(tree, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(tree, p)
		if err != nil {
			return err
		}
		written = append(written, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(written)

	if err := os.MkdirAll(dst, 0o755); err != nil {
		return nil, err
	}
	// Remove what the pack dropped, before writing what it provides — a file
	// that is both dropped and re-added under a different case or path would
	// otherwise race the write below.
	nowProvided := make(map[string]bool, len(written))
	for _, f := range written {
		nowProvided[f] = true
	}
	for _, prev := range prevFiles {
		if nowProvided[prev] {
			continue
		}
		target, err := dirEntryTarget(dst, artPath, prev)
		if err != nil {
			return nil, err
		}
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	for _, f := range written {
		rel := path.Join(artPath, f)
		if err := refuseSymlinks(root, rel); err != nil {
			return nil, err
		}
		from := filepath.Join(tree, filepath.FromSlash(f))
		to := filepath.Join(dst, filepath.FromSlash(f))
		content, err := os.ReadFile(from)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(from)
		if err != nil {
			return nil, err
		}
		if err := atomicWrite(to, content); err != nil {
			return nil, err
		}
		if err := os.Chmod(to, info.Mode().Perm()); err != nil {
			return nil, err
		}
	}
	return written, nil
}

// copyDir copies a tree, preserving file modes. Symlinks fail closed — packs
// must not reference anything outside themselves.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("symlink %s: symlinks are not allowed in packs", path)
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, info.Mode().Perm())
	})
}
