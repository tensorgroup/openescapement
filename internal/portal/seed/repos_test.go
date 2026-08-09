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
	// This prose is signed into user repos and read by autonomous agents, so
	// it has to name the real marker syntax rather than leave an agent that
	// scans visually to infer that `escapement:begin` is an HTML comment,
	// and it has to say the marker lines are themselves off limits: the
	// begin line carries the load-bearing `hash=`, and an agent "tidying"
	// that comment would freeze the file's policy updates, the exact
	// outcome the skill warns against.
	for _, want := range []string{
		"<!-- escapement:begin packs=... hash=sha256:... -->",
		"<!-- escapement:end -->",
		"marker lines are part of the block",
		"`hash=` value on the begin line",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("skill must state %q:\n%s", want, body)
		}
	}
	if n := strings.Count(body, "\n") + 1; n > 40 {
		t.Errorf("skill body is %d lines, keep it under 40", n)
	}
	if strings.Contains(body, "\u2014") {
		t.Errorf("no em-dashes in shipped prose:\n%s", body)
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
