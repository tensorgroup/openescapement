package guidance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	if err := os.Rename(tmpName, dst); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
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

const seedManifest = ".seeded.json"

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// atomicWrite writes b to dst via temp-plus-rename, so a concurrent reader
// never sees a partial file. Callers guarantee filepath.Dir(dst) exists.
func atomicWrite(dst string, b []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".seed-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Chmod(name, perm); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, dst); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// readManifest loads the seed manifest. An absent or unparseable manifest
// returns (nil, nil): the caller treats nil as "pre-manifest, migrate". A
// corrupt machine-managed file is not a hard failure (fail-soft posture).
func readManifest(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, nil
	}
	return m, nil
}

// writeManifest atomically persists the manifest. Map keys are marshaled in
// sorted order, so the output is deterministic.
func writeManifest(path string, m map[string]string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return atomicWrite(path, append(data, '\n'), 0o644)
}

// migrateManifest records disk files under root that still byte-match the
// current embedded content (provably unmodified). Non-matching and absent
// files are left unrecorded: old-seed-unmodified and user-edited are
// indistinguishable without history.
func migrateManifest(root string, recorded map[string]string) error {
	return fs.WalkDir(embeddedModels, "models", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || p == "models" {
			return err
		}
		rel := strings.TrimPrefix(p, "models/")
		emb, rerr := embeddedModels.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		disk, derr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if derr != nil {
			return nil // absent -> not recorded
		}
		if sha256Hex(disk) == sha256Hex(emb) {
			recorded[rel] = sha256Hex(emb)
		}
		return nil
	})
}

// Seed writes the embedded guidance tree into <dataDir>/guidance/ with
// create-or-refresh-unmodified semantics tracked by a hash manifest at
// <dataDir>/guidance/.seeded.json. Absent files are written and recorded.
// A file whose disk hash matches its recorded hash (never edited) is
// refreshed when the embedded content has changed, and its record updated.
// A file whose disk hash differs from its recorded hash (hand-edited or
// portal-edited) is left untouched. Files no longer in the embedded tree are
// left on disk with their record retained. Pre-manifest dirs are migrated:
// only files still matching embedded are recorded.
func Seed(dataDir string) error {
	root := filepath.Join(dataDir, "guidance")
	manifestPath := filepath.Join(root, seedManifest)

	recorded, err := readManifest(manifestPath)
	if err != nil {
		return err
	}
	if recorded == nil {
		recorded = map[string]string{}
		if err := migrateManifest(root, recorded); err != nil {
			return err
		}
	}

	err = fs.WalkDir(embeddedModels, "models", func(p string, d fs.DirEntry, err error) error {
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
		emb, rerr := embeddedModels.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		sum := sha256Hex(emb)
		disk, derr := os.ReadFile(dst)
		switch {
		case os.IsNotExist(derr):
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			if err := atomicWrite(dst, emb, 0o644); err != nil {
				return err
			}
			recorded[rel] = sum
		case derr != nil:
			return derr
		default:
			diskSum := sha256Hex(disk)
			if diskSum == recorded[rel] { // unedited
				if diskSum != sum { // embedded changed -> refresh
					if err := atomicWrite(dst, emb, 0o644); err != nil {
						return err
					}
				}
				recorded[rel] = sum
			}
			// else: hand-edited -> leave, record unchanged
		}
		return nil
	})
	if err != nil {
		return err
	}
	return writeManifest(manifestPath, recorded)
}
