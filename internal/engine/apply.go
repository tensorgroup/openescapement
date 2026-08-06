package engine

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/lockfile"
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
			return fmt.Errorf("%w: %s: refusing to write through symlink %s", esc.ErrConstraint, rel, cur)
		}
	}
	return nil
}

// Apply writes the planned artifacts and the lockfile. It refuses to write
// anything when the plan has constraint violations.
func Apply(root string, p *PlanResult) error {
	if len(p.Violations) > 0 {
		msgs := make([]string, len(p.Violations))
		for i, v := range p.Violations {
			msgs[i] = v.String()
		}
		return fmt.Errorf("%w:\n  %s", esc.ErrConstraint, strings.Join(msgs, "\n  "))
	}
	prevLock, err := lockfile.Load(root)
	if err != nil {
		return err
	}

	var arts []lockfile.LockArtifact
	desiredDirs := map[string]bool{}
	for _, a := range p.Artifacts {
		abs, err := containedPath(root, a.Path)
		if err != nil {
			return err
		}
		if err := refuseSymlinks(root, a.Path); err != nil {
			return err
		}
		switch a.Kind {
		case KindBlock:
			existing, err := os.ReadFile(abs)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			out, err := render.Splice(existing, a.Body, render.BlockMeta{Packs: a.BlockPacks})
			if err != nil {
				return fmt.Errorf("%s: %w", a.Path, err)
			}
			if err := atomicWrite(abs, out); err != nil {
				return err
			}
		case KindFile:
			if err := atomicWrite(abs, []byte(a.Body)); err != nil {
				return err
			}
		case KindDir:
			desiredDirs[a.Path] = true
			var prevFiles []string
			if prev := prevLock.Artifact(a.Path); prev != nil {
				prevFiles = prev.Files
			}
			files, err := mergeDir(a.SrcDir, abs, prevFiles)
			if err != nil {
				return err
			}
			a.Files = files
		case KindJSONKeys:
			existing, err := os.ReadFile(abs)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			var prevOwned []string
			if prev := prevLock.Artifact(a.Path); prev != nil {
				prevOwned = prev.Keys
			}
			merged, owned, err := render.MergeMCP(existing, a.Servers, prevOwned)
			if err != nil {
				return err
			}
			if err := atomicWrite(abs, merged); err != nil {
				return err
			}
			a.Keys = owned
		}
		arts = append(arts, lockfile.LockArtifact{Path: a.Path, Kind: a.Kind, Hash: a.Hash, Keys: a.Keys, Files: a.Files})
	}

	// Remove owned skill dirs that no longer exist in any pack. Only paths
	// carrying the escapement ownership prefix are ever removed.
	if prevLock != nil {
		for _, prev := range prevLock.Artifacts {
			if prev.Kind != KindDir || desiredDirs[prev.Path] {
				continue
			}
			abs, err := containedPath(root, prev.Path)
			if err != nil {
				return err
			}
			if !strings.Contains(abs, string(filepath.Separator)+"esc-") {
				return fmt.Errorf("refusing to remove %q: not an escapement-owned directory", prev.Path)
			}
			if err := os.RemoveAll(abs); err != nil {
				return err
			}
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
				return err
			}
			if err := refuseSymlinks(root, prev.Path); err != nil {
				return err
			}
			existing, err := os.ReadFile(abs)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			out, removed, err := render.RemoveBlock(existing)
			if err != nil {
				return fmt.Errorf("%s: %w", prev.Path, err)
			}
			if !removed {
				continue
			}
			if len(out) == 0 {
				if err := os.Remove(abs); err != nil {
					return err
				}
				continue
			}
			if err := atomicWrite(abs, out); err != nil {
				return err
			}
		}
	}

	lock := &lockfile.Lock{Schema: 1, Packs: p.Packs, Artifacts: arts}
	return lock.Save(root)
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

// mergeDir reconciles dst against the pack tree at src. Files the pack
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
func mergeDir(src, dst string, prevFiles []string) ([]string, error) {
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
		if err := os.Remove(filepath.Join(dst, filepath.FromSlash(prev))); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	for _, f := range written {
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
