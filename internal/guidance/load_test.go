package guidance

import (
	"os"
	"path/filepath"
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
