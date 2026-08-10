package pack

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// SourcesFile is authoring metadata at the pack root: per vendored skill, the
// upstream source, requested ref, resolved commit, and the dir hash of the
// vendored copy. Consumers never read it — but it travels inside the pack
// repo, so the signed tag and LockPack.Hash cover it automatically (spec §2).
// It deliberately lives OUTSIDE the skill directories so the vendored copy
// stays byte-identical to upstream and no directory walk ever has to filter
// it.
const SourcesFile = "sources.yaml"

type SourceSkill struct {
	Name   string `yaml:"name"`
	Source string `yaml:"source"`
	Subdir string `yaml:"subdir,omitempty"`
	Ref    string `yaml:"ref"`
	Commit string `yaml:"commit"`
	Hash   string `yaml:"hash"`
}

type Sources struct {
	Schema int           `yaml:"schema"`
	Skills []SourceSkill `yaml:"skills"`
}

// LoadSources reads dir/sources.yaml. Returns (nil, nil) when absent: a pack
// with no vendored skills simply has no provenance to record.
func LoadSources(dir string) (*Sources, error) {
	raw, err := os.ReadFile(filepath.Join(dir, SourcesFile))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s Sources
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", SourcesFile, err)
	}
	if s.Schema != 1 {
		return nil, fmt.Errorf("%s: unsupported schema %d (want 1)", SourcesFile, s.Schema)
	}
	// sources.yaml is committed and travels inside the pack repo — untrusted
	// input exactly like a lockfile's Files entries, and Name becomes a
	// filesystem path component in every consumer (vendoredDir's fallback,
	// update-skill's rename/delete). Reject an invalid name HERE, at load
	// time, so nothing downstream — including a --force path that skips the
	// divergence gate entirely — ever sees a name that could resolve outside
	// the pack directory. This is a containment refusal, not a manifest
	// content error: a plain error (exit 4), never esc.ErrConstraint.
	for _, sk := range s.Skills {
		if !ValidName.MatchString(sk.Name) {
			return nil, fmt.Errorf("%s: skill name %q must match %s (it becomes a filesystem path component)", SourcesFile, sk.Name, ValidName)
		}
	}
	return &s, nil
}

// Save writes sources.yaml deterministically (skills sorted by name) so
// repeated vendoring operations produce stable, reviewable diffs.
func (s *Sources) Save(dir string) error {
	sort.Slice(s.Skills, func(i, j int) bool { return s.Skills[i].Name < s.Skills[j].Name })
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(s); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return atomicWrite(filepath.Join(dir, SourcesFile), buf.Bytes())
}

// atomicWrite writes via a temp file + rename in the destination directory,
// so a process killed mid-write never leaves a truncated sources.yaml behind
// to poison the pack's signed hash.
func atomicWrite(path string, content []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".sources-*")
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

// Skill returns the entry named name, or nil.
func (s *Sources) Skill(name string) *SourceSkill {
	for i := range s.Skills {
		if s.Skills[i].Name == name {
			return &s.Skills[i]
		}
	}
	return nil
}

// Upsert replaces the entry with e.Name, or appends it.
func (s *Sources) Upsert(e SourceSkill) {
	if cur := s.Skill(e.Name); cur != nil {
		*cur = e
		return
	}
	s.Skills = append(s.Skills, e)
}
