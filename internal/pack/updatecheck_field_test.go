package pack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseEvery(t *testing.T) {
	ok := map[string]time.Duration{
		"7d":    7 * 24 * time.Hour,
		"1d":    24 * time.Hour,
		"24h":   24 * time.Hour,
		"90m":   90 * time.Minute,
		"30s":   30 * time.Second,
		"3650d": 3650 * 24 * time.Hour,
	}
	for in, want := range ok {
		got, err := ParseEvery(in)
		if err != nil || got != want {
			t.Errorf("ParseEvery(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "0d", "-1h", "7", "d", "7w", "1.5d", "abc", "99999999999d"} {
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

func TestManifestReportingEndpoint(t *testing.T) {
	base := "schema: 1\nname: p\nversion: 1.0.0\nrules: [a.md]\n"
	// Valid: https endpoint accepted.
	p, err := loadManifestPack(t, base+"reporting:\n  endpoint: https://telemetry.example/ingest\n")
	if err != nil {
		t.Fatalf("valid reporting.endpoint rejected: %v", err)
	}
	if p.Manifest.Reporting == nil || p.Manifest.Reporting.Endpoint != "https://telemetry.example/ingest" {
		t.Fatalf("reporting.endpoint not parsed: %+v", p.Manifest.Reporting)
	}
	// Empty endpoint accepted: absent means nothing is sent.
	p, err = loadManifestPack(t, base+"reporting:\n  amendments: metrics\n")
	if err != nil {
		t.Fatalf("empty reporting.endpoint rejected: %v", err)
	}
	if p.Manifest.Reporting == nil || p.Manifest.Reporting.Endpoint != "" {
		t.Fatalf("empty reporting.endpoint: %+v", p.Manifest.Reporting)
	}
	// Non-https endpoint → manifest error, same message style as update_check.
	_, err = loadManifestPack(t, base+"reporting:\n  endpoint: http://telemetry.example\n")
	if err == nil {
		t.Fatal("http endpoint: want error")
	}
	const wantMsg = `reporting.endpoint "http://telemetry.example": must be an https:// URL`
	if !strings.Contains(err.Error(), wantMsg) {
		t.Errorf("error message = %q, want it to contain %q", err.Error(), wantMsg)
	}
}
