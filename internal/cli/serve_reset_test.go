package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tensorgroup/openescapement/internal/guidance"
	"github.com/tensorgroup/openescapement/internal/portal/seed"
)

func TestResetDemoWipesDemoPathsAndPrints(t *testing.T) {
	dir := t.TempDir()
	// Populate every demo-owned path.
	for _, name := range []string{"registry.json", "events.jsonl"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, sub := range []string{"packs", "demo-repo", "guidance"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := resetDemo(dir, &out); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"registry.json", "events.jsonl", "packs", "demo-repo", "guidance"} {
		if _, err := os.Stat(filepath.Join(dir, p)); !os.IsNotExist(err) {
			t.Fatalf("demo path %s not removed: %v", p, err)
		}
	}
	if !strings.Contains(out.String(), "esc: demo data reset") {
		t.Fatalf("missing reset message: %q", out.String())
	}
}

// Mirrors cmdServe's demo seeding order. A modified guidance note and a
// modified demo-repo file are pristine again after a second demo startup.
func demoStartup(t *testing.T, dir string) {
	t.Helper()
	var out bytes.Buffer
	if err := resetDemo(dir, &out); err != nil {
		t.Fatal(err)
	}
	if err := guidance.Seed(dir); err != nil {
		t.Fatal(err)
	}
	if err := seed.Demo(dir, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := seed.Repos(dir); err != nil {
		t.Fatal(err)
	}
}

func TestDemoSecondStartupRestoresPristine(t *testing.T) {
	dir := t.TempDir()
	demoStartup(t, dir)

	note := filepath.Join(dir, "guidance", "anthropic.md")
	repoFile := filepath.Join(dir, "demo-repo", "CLAUDE.md")
	pristineNote, _ := os.ReadFile(note)
	pristineRepo, _ := os.ReadFile(repoFile)

	if err := os.WriteFile(note, []byte("# tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(repoFile, []byte("# tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	demoStartup(t, dir) // second startup

	if got, _ := os.ReadFile(note); string(got) != string(pristineNote) {
		t.Fatalf("guidance note not reset: %q", got)
	}
	if got, _ := os.ReadFile(repoFile); string(got) != string(pristineRepo) {
		t.Fatalf("demo-repo file not reset: %q", got)
	}
}

// Non-demo startup (guidance.Seed only) keeps a hand edit.
func TestNonDemoStartupKeepsEdits(t *testing.T) {
	dir := t.TempDir()
	if err := guidance.Seed(dir); err != nil {
		t.Fatal(err)
	}
	note := filepath.Join(dir, "guidance", "anthropic.md")
	if err := os.WriteFile(note, []byte("# mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := guidance.Seed(dir); err != nil { // second non-demo startup
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(note); string(got) != "# mine\n" {
		t.Fatalf("non-demo startup overwrote edit: %q", got)
	}
}
