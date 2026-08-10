package pack

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourcesRoundTripSortedAndAbsent(t *testing.T) {
	dir := t.TempDir()
	got, err := LoadSources(dir)
	if err != nil || got != nil {
		t.Fatalf("absent sources.yaml must be (nil, nil), got %v, %v", got, err)
	}
	s := &Sources{Schema: 1}
	s.Upsert(SourceSkill{Name: "writing-plans", Source: "github.com/obra/superpowers", Subdir: "skills/writing-plans", Ref: "v6.2.0", Commit: "abc123", Hash: "sha256:aa"})
	s.Upsert(SourceSkill{Name: "brainstorming", Source: "github.com/obra/superpowers", Subdir: "skills/brainstorming", Ref: "v6.2.0", Commit: "abc123", Hash: "sha256:bb"})
	if err := s.Save(dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadSources(dir)
	if err != nil || loaded == nil || len(loaded.Skills) != 2 {
		t.Fatalf("reload failed: %+v, %v", loaded, err)
	}
	if loaded.Skills[0].Name != "brainstorming" || loaded.Skills[1].Name != "writing-plans" {
		t.Fatalf("skills not sorted by name: %+v", loaded.Skills)
	}
	// Upsert replaces in place.
	s.Upsert(SourceSkill{Name: "brainstorming", Source: "github.com/obra/superpowers", Subdir: "skills/brainstorming", Ref: "v6.3.0", Commit: "def456", Hash: "sha256:cc"})
	if len(s.Skills) != 2 || s.Skill("brainstorming").Ref != "v6.3.0" {
		t.Fatalf("upsert did not replace: %+v", s.Skills)
	}
	// Deterministic bytes: saving the reloaded state reproduces the file
	// byte-for-byte, so repeated vendoring operations diff cleanly.
	first, err := os.ReadFile(filepath.Join(dir, "sources.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := loaded.Save(dir); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(dir, "sources.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("sources.yaml is not deterministic:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

func TestSourcesSaveLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	s := &Sources{Schema: 1}
	s.Upsert(SourceSkill{Name: "brainstorming", Source: "github.com/obra/superpowers", Subdir: "skills/brainstorming", Ref: "v6.2.0", Commit: "abc123", Hash: "sha256:bb"})
	if err := s.Save(dir); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != SourcesFile {
		t.Fatalf("Save left unexpected directory contents: %v", entries)
	}
}

func TestLoadSourcesRejectsUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sources.yaml"), []byte("schema: 1\nbogus: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSources(dir); err == nil {
		t.Fatal("unknown key accepted")
	}
}
