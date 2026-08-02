package guidance

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Set is a disk-backed view of the guidance tree. dir is <data-dir>/guidance
// ("" = embedded only). Degraded names on-disk files that were missing or
// unparseable and fell back to the embedded copy, for the portal banner.
type Set struct {
	Registry *Registry
	Degraded []string
	dir      string
}

// Load reads the registry from disk (falling back to embedded) and returns a
// Set that reads notes and examples per request. Never fails: a broken
// content directory a human hand-edits must not take the portal down.
func Load(dir string) *Set {
	s := &Set{dir: dir}
	reg, degraded := loadRegistry(dir)
	s.Registry = reg
	if degraded {
		s.Degraded = append(s.Degraded, "models.yaml")
	}
	return s
}

func embeddedRegistry() *Registry {
	b, _ := embeddedModels.ReadFile("models/models.yaml")
	reg, _ := ParseRegistry(b) // embedded copy is validated in tests
	return reg
}

func loadRegistry(dir string) (*Registry, bool) {
	if dir != "" {
		if b, err := os.ReadFile(filepath.Join(dir, "models.yaml")); err == nil {
			if reg, perr := ParseRegistry(b); perr == nil {
				return reg, false
			}
		}
		return embeddedRegistry(), true // missing or unparseable on disk
	}
	return embeddedRegistry(), false
}

// KnownFiles is the fixed set of portal-editable guidance files: vendor notes
// plus every embedded example fragment. models.yaml is deliberately excluded.
func (s *Set) KnownFiles() map[string]bool {
	m := make(map[string]bool, len(VendorOrder))
	for _, key := range VendorOrder {
		m[key+".md"] = true
	}
	_ = fs.WalkDir(embeddedModels, "models/examples", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		m[strings.TrimPrefix(p, "models/")] = true
		return nil
	})
	return m
}

// ExampleFiles returns the vendor's example fragment rel paths, sorted.
func (s *Set) ExampleFiles(vendorKey string) []string {
	var out []string
	entries, _ := embeddedModels.ReadDir("models/examples/" + vendorKey)
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, "examples/"+vendorKey+"/"+e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// ReadFile returns a guidance file's content, disk-first with embedded
// fallback. rel must be a known file. degraded is true when the embedded copy
// was used because the disk copy was absent.
func (s *Set) ReadFile(rel string) ([]byte, bool, error) {
	if !s.KnownFiles()[rel] {
		return nil, false, fmt.Errorf("guidance: unknown file %q", rel)
	}
	if s.dir != "" {
		if b, err := os.ReadFile(filepath.Join(s.dir, filepath.FromSlash(rel))); err == nil {
			return b, false, nil
		}
		if b, err := embeddedModels.ReadFile("models/" + rel); err == nil {
			return b, true, nil
		}
		return nil, false, fmt.Errorf("guidance: %s not found", rel)
	}
	b, err := embeddedModels.ReadFile("models/" + rel)
	return b, false, err
}

// WriteFile atomically writes a known guidance file to disk. It never creates
// paths from user input: rel must be in KnownFiles, and dir must be set.
func (s *Set) WriteFile(rel string, content []byte) error {
	if !s.KnownFiles()[rel] {
		return fmt.Errorf("guidance: unknown file %q", rel)
	}
	if s.dir == "" {
		return fmt.Errorf("guidance: no data directory configured")
	}
	dst := filepath.Join(s.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".guidance-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, dst)
}

// VendorForModel finds the vendor key owning a model id.
func (s *Set) VendorForModel(id string) (string, bool) {
	for _, v := range s.Registry.Vendors {
		for _, m := range v.Models {
			if m.ID == id {
				return v.Key, true
			}
		}
	}
	return "", false
}

// Seed writes the embedded guidance tree into <dataDir>/guidance/,
// create-if-missing: existing files are left untouched (same posture as the
// demo repo seed; delete the directory to re-seed after an upgrade).
func Seed(dataDir string) error {
	root := filepath.Join(dataDir, "guidance")
	return fs.WalkDir(embeddedModels, "models", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "models" {
			return nil
		}
		rel := strings.TrimPrefix(p, "models/")
		dst := filepath.Join(root, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		if _, statErr := os.Stat(dst); statErr == nil {
			return nil // create-if-missing
		}
		b, readErr := embeddedModels.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, b, 0o644)
	})
}
