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
