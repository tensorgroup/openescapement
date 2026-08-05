package guidance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := Seed(dir); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "guidance")
}

func TestSeedCreatesTreeAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := Seed(dir); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(dir, "guidance", "anthropic.md")
	if err := os.WriteFile(f, []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Seed(dir); err != nil { // second seed must not clobber
		t.Fatal(err)
	}
	got, _ := os.ReadFile(f)
	if string(got) != "edited" {
		t.Fatalf("seed clobbered an existing file: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "guidance", "models.yaml")); err != nil {
		t.Fatalf("models.yaml not seeded: %v", err)
	}
}

func TestLoadDiskAndFallback(t *testing.T) {
	gdir := seedTemp(t)
	s := Load(gdir)
	if len(s.Degraded) != 0 {
		t.Fatalf("clean load degraded: %v", s.Degraded)
	}
	if _, ok := s.VendorForModel("claude-sonnet-5"); !ok {
		t.Fatal("seeded model id not found in registry")
	}
	// Corrupt models.yaml on disk: registry falls back, banner records it.
	if err := os.WriteFile(filepath.Join(gdir, "models.yaml"), []byte("vendors: [oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	s = Load(gdir)
	if len(s.Degraded) != 1 || s.Degraded[0] != "models.yaml" {
		t.Fatalf("expected degraded models.yaml, got %v", s.Degraded)
	}
	if _, ok := s.VendorForModel("claude-sonnet-5"); !ok {
		t.Fatal("fallback registry missing seeded model")
	}
}

func TestReadFileFallbackAndWriteRoundTrip(t *testing.T) {
	gdir := seedTemp(t)
	s := Load(gdir)
	if err := s.WriteFile("anthropic.md", []byte("# Anthropic\n\n## Sources\n- x\n")); err != nil {
		t.Fatal(err)
	}
	got, deg, err := s.ReadFile("anthropic.md")
	if err != nil || deg {
		t.Fatalf("read after write: deg=%v err=%v", deg, err)
	}
	if string(got) == "" || string(got)[0] != '#' {
		t.Fatalf("unexpected content %q", got)
	}
	// Removing the disk copy triggers embedded fallback (degraded=true).
	if err := os.Remove(filepath.Join(gdir, "anthropic.md")); err != nil {
		t.Fatal(err)
	}
	if _, deg, _ := s.ReadFile("anthropic.md"); !deg {
		t.Fatal("expected degraded read after removing disk file")
	}
}

func TestWriteFileRejectsUnknownPath(t *testing.T) {
	s := Load(seedTemp(t))
	for _, bad := range []string{"models.yaml", "../escape.md", "unknown.md", "examples/anthropic/../../x"} {
		if err := s.WriteFile(bad, []byte("x")); err == nil {
			t.Errorf("WriteFile(%q) should have been rejected", bad)
		}
	}
}

func readManifestForTest(t *testing.T, gdir string) map[string]string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(gdir, ".seeded.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return m
}

func TestSeedRecordsManifestOnCreate(t *testing.T) {
	gdir := seedTemp(t)
	m := readManifestForTest(t, gdir)
	emb, _ := embeddedModels.ReadFile("models/models.yaml")
	if m["models.yaml"] != sha256Hex(emb) {
		t.Fatalf("manifest missing/incorrect models.yaml hash: %v", m["models.yaml"])
	}
	// No temp files left behind by the atomic writes.
	entries, _ := os.ReadDir(gdir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".seeded-") || strings.HasPrefix(e.Name(), ".guidance-") {
			t.Fatalf("leftover temp file: %s", e.Name())
		}
	}
}

// Spec §3 regression: an unedited file whose recorded hash predates an embedded
// upgrade is refreshed, and its record is updated.
func TestSeedRefreshesUnmodifiedFile(t *testing.T) {
	dir := t.TempDir()
	if err := Seed(dir); err != nil {
		t.Fatal(err)
	}
	gdir := filepath.Join(dir, "guidance")

	// Simulate a pre-upgrade world: disk holds OLD content and the manifest
	// records the OLD hash (the file was seeded, never edited).
	old := []byte("schema: 1\nvendors: []\n")
	if err := os.WriteFile(filepath.Join(gdir, "models.yaml"), old, 0o644); err != nil {
		t.Fatal(err)
	}
	m := readManifestForTest(t, gdir)
	m["models.yaml"] = sha256Hex(old)
	data, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(gdir, ".seeded.json"), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Seed(dir); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(gdir, "models.yaml"))
	emb, _ := embeddedModels.ReadFile("models/models.yaml")
	if string(got) != string(emb) {
		t.Fatalf("unmodified file not refreshed to embedded content")
	}
	if reg, err := ParseRegistry(got); err != nil || len(reg.Vendors) == 0 {
		t.Fatalf("refreshed registry lost its vendors: err=%v", err)
	}
	if readManifestForTest(t, gdir)["models.yaml"] != sha256Hex(emb) {
		t.Fatal("manifest not updated after refresh")
	}
}

// Spec §1: a hand-edited file (disk differs from its recorded hash) is left
// untouched and its record is not changed.
func TestSeedLeavesEditedFile(t *testing.T) {
	dir := t.TempDir()
	if err := Seed(dir); err != nil {
		t.Fatal(err)
	}
	gdir := filepath.Join(dir, "guidance")
	edited := []byte("# My notes\n")
	if err := os.WriteFile(filepath.Join(gdir, "anthropic.md"), edited, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Seed(dir); err != nil { // disk != recorded -> leave
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(gdir, "anthropic.md"))
	if string(got) != string(edited) {
		t.Fatalf("edited file was overwritten: %q", got)
	}
}

// Spec §1 migration: a pre-manifest dir records only files that still match
// the current embedded content; non-matching files are left alone and
// unrecorded.
func TestSeedMigratesPreManifestDir(t *testing.T) {
	dir := t.TempDir()
	gdir := filepath.Join(dir, "guidance")
	if err := os.MkdirAll(gdir, 0o755); err != nil {
		t.Fatal(err)
	}
	// One file that matches embedded (provably unmodified)...
	emb, _ := embeddedModels.ReadFile("models/models.yaml")
	if err := os.WriteFile(filepath.Join(gdir, "models.yaml"), emb, 0o644); err != nil {
		t.Fatal(err)
	}
	// ...and one that does not (old-seed or user-edit, indistinguishable).
	custom := []byte("# custom anthropic notes\n")
	if err := os.WriteFile(filepath.Join(gdir, "anthropic.md"), custom, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Seed(dir); err != nil {
		t.Fatal(err)
	}
	m := readManifestForTest(t, gdir)
	if m["models.yaml"] != sha256Hex(emb) {
		t.Fatal("matching pre-manifest file not recorded")
	}
	if _, ok := m["anthropic.md"]; ok {
		t.Fatal("non-matching pre-manifest file should not be recorded")
	}
	got, _ := os.ReadFile(filepath.Join(gdir, "anthropic.md"))
	if string(got) != string(custom) {
		t.Fatalf("non-matching file was overwritten: %q", got)
	}
}
