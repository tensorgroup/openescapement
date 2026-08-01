package engine

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
			if err := stageDir(a.SrcDir, abs); err != nil {
				return err
			}
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
		arts = append(arts, lockfile.LockArtifact{Path: a.Path, Kind: a.Kind, Hash: a.Hash, Keys: a.Keys})
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

// stageDir replaces dst with a copy of src, staging the copy next to dst
// first so a mid-copy failure never leaves dst half-written or deleted.
func stageDir(src, dst string) error {
	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(parent, ".esc-stage-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	staged := filepath.Join(tmp, filepath.Base(dst))
	if err := copyDir(src, staged); err != nil {
		return err
	}
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return os.Rename(staged, dst)
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
