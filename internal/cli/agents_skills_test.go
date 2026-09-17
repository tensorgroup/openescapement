package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func governSkillPack(t *testing.T, targetsLine string) (root, packDir string) {
	t.Helper()
	packDir = t.TempDir()
	man := "schema: 1\nname: acme\nversion: 1.0.0\nskills:\n  - skills/demo\n"
	if err := os.WriteFile(filepath.Join(packDir, "pack.yaml"), []byte(man), 0o644); err != nil {
		t.Fatal(err)
	}
	sk := filepath.Join(packDir, "skills", "demo")
	if err := os.MkdirAll(sk, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sk, "SKILL.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".escapement"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeSkillConfig(t, root, packDir, targetsLine)
	return root, packDir
}

func writeSkillConfig(t *testing.T, root, packDir, targetsLine string) {
	t.Helper()
	cfg := "schema: 1\npacks:\n  - source: " + packDir + "\n    ref: \"\"\n    trust: unsigned\n" + targetsLine
	if err := os.WriteFile(filepath.Join(root, ".escapement", "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAgentsSkillsBothRootsAndIndependentRetirement(t *testing.T) {
	root, packDir := governSkillPack(t, "targets:\n  - skills\n  - agents-skills\n")
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("sync: %d\n%s", code, out)
	}
	claude := filepath.Join(root, ".claude", "skills", "esc-acme-demo")
	agents := filepath.Join(root, ".agents", "skills", "esc-acme-demo")
	for _, p := range []string{claude, agents} {
		if _, err := os.Stat(filepath.Join(p, "SKILL.md")); err != nil {
			t.Fatalf("missing %s: %v", p, err)
		}
		if err := os.WriteFile(filepath.Join(p, "TEAM.md"), []byte("mine\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Drop agents-skills; the .agents root retires, .claude root stays.
	writeSkillConfig(t, root, packDir, "targets:\n  - skills\n")
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("resync: %d\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(agents, "SKILL.md")); !os.IsNotExist(err) {
		t.Error(".agents SKILL.md should be retired")
	}
	if _, err := os.Stat(filepath.Join(agents, "TEAM.md")); err != nil {
		t.Error(".agents TEAM.md (team-added) must survive retirement")
	}
	if _, err := os.Stat(agents); err != nil {
		t.Error(".agents skill directory must remain because a team file is left in it")
	}
	if _, err := os.Stat(filepath.Join(claude, "SKILL.md")); err != nil {
		t.Error(".claude SKILL.md must remain")
	}
	if _, err := os.Stat(filepath.Join(claude, "TEAM.md")); err != nil {
		t.Error(".claude TEAM.md must remain")
	}
}
