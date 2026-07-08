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
		abs := filepath.Join(root, filepath.FromSlash(a.Path))
		switch a.Kind {
		case "block":
			existing, err := os.ReadFile(abs)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			out, err := render.Splice(existing, a.Body, render.BlockMeta{Packs: render.PackLabels(p.PackObjs)})
			if err != nil {
				return fmt.Errorf("%s: %w", a.Path, err)
			}
			if err := atomicWrite(abs, out); err != nil {
				return err
			}
		case "file":
			if err := atomicWrite(abs, []byte(a.Body)); err != nil {
				return err
			}
		case "dir":
			desiredDirs[a.Path] = true
			if err := os.RemoveAll(abs); err != nil {
				return err
			}
			if err := copyDir(a.SrcDir, abs); err != nil {
				return err
			}
		case "json-keys":
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

	// Remove owned skill dirs that no longer exist in any pack.
	if prevLock != nil {
		for _, prev := range prevLock.Artifacts {
			if prev.Kind == "dir" && !desiredDirs[prev.Path] {
				if err := os.RemoveAll(filepath.Join(root, filepath.FromSlash(prev.Path))); err != nil {
					return err
				}
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

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
}
