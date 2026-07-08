// Package lockfile reads and writes .escapement/escapement.lock — the
// resolution record: which commits and content hashes each pinned pack
// resolved to, and the expected hash of every rendered artifact.
package lockfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/tensorgroup/openescapement/internal/config"
)

const fileName = "escapement.lock"

type LockPack struct {
	Source string `json:"source"`
	Ref    string `json:"ref"`
	Commit string `json:"commit,omitempty"` // empty for plain local dirs
	Hash   string `json:"hash"`             // pack content hash
}

type LockArtifact struct {
	Path string   `json:"path"`
	Kind string   `json:"kind"` // block | file | dir | json-keys
	Hash string   `json:"hash"`
	Keys []string `json:"keys,omitempty"` // owned keys for json-keys artifacts
}

type Lock struct {
	Schema    int            `json:"schema"`
	Packs     []LockPack     `json:"packs"`
	Artifacts []LockArtifact `json:"artifacts"`
}

func Path(root string) string { return filepath.Join(root, config.Dir, fileName) }

// Load returns (nil, nil) when no lockfile exists yet.
func Load(root string) (*Lock, error) {
	raw, err := os.ReadFile(Path(root))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var l Lock
	if err := json.Unmarshal(raw, &l); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", Path(root), err)
	}
	return &l, nil
}

// Save writes the lock deterministically (sorted artifacts, stable JSON).
func (l *Lock) Save(root string) error {
	sort.Slice(l.Artifacts, func(i, j int) bool { return l.Artifacts[i].Path < l.Artifacts[j].Path })
	out, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, config.Dir), 0o755); err != nil {
		return err
	}
	return os.WriteFile(Path(root), append(out, '\n'), 0o644)
}

// Pack returns the locked entry for a source+ref, or nil.
func (l *Lock) Pack(src, ref string) *LockPack {
	if l == nil {
		return nil
	}
	for i := range l.Packs {
		if l.Packs[i].Source == src && l.Packs[i].Ref == ref {
			return &l.Packs[i]
		}
	}
	return nil
}

// Artifact returns the locked entry for a path, or nil.
func (l *Lock) Artifact(path string) *LockArtifact {
	if l == nil {
		return nil
	}
	for i := range l.Artifacts {
		if l.Artifacts[i].Path == path {
			return &l.Artifacts[i]
		}
	}
	return nil
}
