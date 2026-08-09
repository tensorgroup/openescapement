package seed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSeededPackShipsReconcileSkill(t *testing.T) {
	dir := t.TempDir()
	packDir, _, err := Repos(dir)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(packDir, "skills", "esc-reconcile", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, want := range []string{"escapement:begin", "escapement:end", "never edit"} {
		if !strings.Contains(body, want) {
			t.Errorf("skill must mention %q:\n%s", want, body)
		}
	}
	manifest, err := os.ReadFile(filepath.Join(packDir, "pack.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifest), "skills/esc-reconcile") {
		t.Errorf("manifest must declare the skill:\n%s", manifest)
	}
}

func TestReposIdempotent(t *testing.T) {
	dir := t.TempDir()
	p1, r1, err := Repos(dir)
	if err != nil {
		t.Fatal(err)
	}
	p2, r2, err := Repos(dir)
	if err != nil || p1 != p2 || r1 != r2 {
		t.Fatalf("not idempotent: %v", err)
	}
	cfg, err := os.ReadFile(filepath.Join(r1, ".escapement", "config.yaml"))
	if err != nil || !strings.Contains(string(cfg), "trust: unsigned") {
		t.Fatalf("config: %s err=%v", cfg, err)
	}
	for _, name := range []string{"CLAUDE.md", "AGENTS.md", "GEMINI.md"} {
		if _, err := os.Stat(filepath.Join(r1, name)); err != nil {
			t.Fatalf("missing seeded %s: %v", name, err)
		}
	}
}
