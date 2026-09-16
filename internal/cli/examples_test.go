package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExamplesSync runs the shipped examples end-to-end in a temp copy.
func TestExamplesSync(t *testing.T) {
	src, err := filepath.Abs("../../examples")
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := copyTree(src, tmp); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	root := filepath.Join(tmp, "governed-service")

	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("examples sync failed (%d):\n%s", code, out)
	}
	if code, out := run(t, root, "status", "--check"); code != 0 {
		t.Fatalf("examples not clean after sync (%d):\n%s", code, out)
	}
	claude, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"pnpm install", "vault.acme.example", "Tailscale", "escapement:begin packs=acme-org@0.1.0"} {
		if !strings.Contains(string(claude), want) {
			t.Errorf("example CLAUDE.md missing %q", want)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/skills/esc-acme-org-acme-vault/SKILL.md")); err != nil {
		t.Errorf("example skill not rendered: %v", err)
	}
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
}

// TestEveryExamplePackSyncs governs a fresh temp repo with each pack under
// examples/packs in turn, so a pack that fails to parse or render is caught
// here rather than by a reader copying it. The packs are shipped as starters;
// each must sync and leave the repo clean.
func TestEveryExamplePackSyncs(t *testing.T) {
	packs, err := filepath.Abs("../../examples/packs")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(packs)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, ".escapement"), 0o755); err != nil {
				t.Fatal(err)
			}
			cfg := "schema: 1\npacks:\n  - source: " + filepath.Join(packs, name) + "\n    ref: \"\"\n    trust: unsigned\n"
			if err := os.WriteFile(filepath.Join(root, ".escapement", "config.yaml"), []byte(cfg), 0o644); err != nil {
				t.Fatal(err)
			}
			if code, out := run(t, root, "sync"); code != 0 {
				t.Fatalf("%s: sync failed (%d):\n%s", name, code, out)
			}
			if code, out := run(t, root, "status", "--check"); code != 0 {
				t.Fatalf("%s: not clean after sync (%d):\n%s", name, code, out)
			}
			if _, err := os.Stat(filepath.Join(root, "GOVERNANCE.md")); err != nil {
				t.Errorf("%s: GOVERNANCE.md not rendered: %v", name, err)
			}
		})
	}
}
