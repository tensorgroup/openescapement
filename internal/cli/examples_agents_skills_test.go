package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExamplesAgentsSkillsVariant(t *testing.T) {
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
	cfgPath := filepath.Join(root, ".escapement", "config.yaml")
	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg = append(cfg, []byte("targets:\n  - claude\n  - agents\n  - gemini\n  - governance\n  - mcp\n  - skills\n  - rules\n  - agents-skills\n")...)
	if err := os.WriteFile(cfgPath, cfg, 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("sync: %d\n%s", code, out)
	}
	for _, p := range []string{
		".claude/skills/esc-acme-org-acme-vault/SKILL.md",
		".agents/skills/esc-acme-org-acme-vault/SKILL.md",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
	if code, out := run(t, root, "status", "--check"); code != 0 {
		t.Fatalf("status --check must be clean with agents-skills: %d\n%s", code, out)
	}
}
