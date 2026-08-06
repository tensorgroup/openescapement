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

// DirHashOf computes the same canonical hash as DirHash, but only over the
// given slash-separated paths relative to dir, rather than every file in the
// tree. This is what lets a dir artifact's managed hash cover exactly the
// pack-provided files: a team adding an extra file under dir must not flip
// the whole directory to altered, so the managed hash can only ever be a
// function of the files escapement itself wrote. rel is sorted into a local
// copy before hashing so DirHashOf(dir, allFiles) agrees byte-for-byte with
// DirHash(dir) regardless of the caller's ordering.
func DirHashOf(dir string, rel []string) (string, error) {
	files := append([]string(nil), rel...)
	sort.Strings(files)
	h := sha256.New()
	for _, r := range files {
		io.WriteString(h, r)
		h.Write([]byte{0})
		content, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(r)))
		if err != nil {
			return "", err
		}
		h.Write(content)
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
