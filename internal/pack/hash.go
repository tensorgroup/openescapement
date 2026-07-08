package pack

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// DirHash computes a deterministic content hash of every regular file under
// dir (sorted relative paths, path and content both hashed). .git is skipped.
// Symlinks fail closed: a pack must not be able to reference files outside
// itself, and link targets would make hashes host-dependent.
func DirHash(dir string) (string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("symlink %s: symlinks are not allowed in packs", path)
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	h := sha256.New()
	for _, rel := range files {
		io.WriteString(h, rel)
		h.Write([]byte{0})
		content, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return "", err
		}
		h.Write(content)
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
