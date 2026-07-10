package pack

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseEvery(t *testing.T) {
	ok := map[string]time.Duration{
		"7d":  7 * 24 * time.Hour,
		"1d":  24 * time.Hour,
		"24h": 24 * time.Hour,
		"90m": 90 * time.Minute,
		"30s": 30 * time.Second,
	}
	for in, want := range ok {
		got, err := ParseEvery(in)
		if err != nil || got != want {
			t.Errorf("ParseEvery(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "0d", "-1h", "7", "d", "7w", "1.5d", "abc"} {
		if _, err := ParseEvery(bad); err == nil {
			t.Errorf("ParseEvery(%q): want error", bad)
		}
	}
}

// loadManifestPack writes a one-file pack with the given pack.yaml and loads it.
func loadManifestPack(t *testing.T, manifest string) (*Pack, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("## A\nx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return Load(dir)
}

func TestManifestUpdateCheck(t *testing.T) {
	base := "schema: 1\nname: p\nversion: 1.0.0\nrules: [a.md]\n"
	// Valid: every + https endpoint.
	p, err := loadManifestPack(t, base+"update_check:\n  every: 7d\n  endpoint: https://cp.example/check\n")
	if err != nil {
		t.Fatalf("valid update_check rejected: %v", err)
	}
	if p.Manifest.UpdateCheck == nil || p.Manifest.UpdateCheck.Every != "7d" {
		t.Fatalf("update_check not parsed: %+v", p.Manifest.UpdateCheck)
	}
	// Absent → nil, no error.
	p, err = loadManifestPack(t, base)
	if err != nil || p.Manifest.UpdateCheck != nil {
		t.Fatalf("absent update_check: %+v err %v", p, err)
	}
	// Bad every → manifest error.
	if _, err := loadManifestPack(t, base+"update_check:\n  every: 7w\n"); err == nil {
		t.Error("bad every: want error")
	}
	// Non-https endpoint → manifest error.
	if _, err := loadManifestPack(t, base+"update_check:\n  every: 7d\n  endpoint: http://cp.example\n"); err == nil {
		t.Error("http endpoint: want error")
	}
}
