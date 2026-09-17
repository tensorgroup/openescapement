package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/tensorgroup/openescapement/internal/lockfile"
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

	// The two roots must render byte-identical content: same pack source,
	// same skill, only the root differs.
	claudeBytes, err := os.ReadFile(filepath.Join(claude, "SKILL.md"))
	if err != nil {
		t.Fatalf("reading %s: %v", claude, err)
	}
	agentsBytes, err := os.ReadFile(filepath.Join(agents, "SKILL.md"))
	if err != nil {
		t.Fatalf("reading %s: %v", agents, err)
	}
	if !bytes.Equal(claudeBytes, agentsBytes) {
		t.Errorf("SKILL.md bytes differ between roots: claude=%q agents=%q", claudeBytes, agentsBytes)
	}

	// The lockfile carries two independent entries for the skill, one per
	// root, each with its own Files manifest — that's what lets retirement
	// of one root leave the other's lock entry (and team-added files) alone.
	lock, err := lockfile.Load(root)
	if err != nil {
		t.Fatalf("loading lockfile: %v", err)
	}
	if lock == nil {
		t.Fatal("expected a lockfile after sync")
	}
	var claudeEntry, agentsEntry *lockfile.LockArtifact
	for i := range lock.Artifacts {
		switch lock.Artifacts[i].Path {
		case ".claude/skills/esc-acme-demo":
			claudeEntry = &lock.Artifacts[i]
		case ".agents/skills/esc-acme-demo":
			agentsEntry = &lock.Artifacts[i]
		}
	}
	if claudeEntry == nil {
		t.Fatal("lockfile missing .claude/skills/esc-acme-demo entry")
	}
	if agentsEntry == nil {
		t.Fatal("lockfile missing .agents/skills/esc-acme-demo entry")
	}
	if len(claudeEntry.Files) == 0 {
		t.Error(".claude lock entry must carry a Files manifest")
	}
	if len(agentsEntry.Files) == 0 {
		t.Error(".agents lock entry must carry a Files manifest")
	}
	claudeFiles := append([]string(nil), claudeEntry.Files...)
	agentsFiles := append([]string(nil), agentsEntry.Files...)
	sort.Strings(claudeFiles)
	sort.Strings(agentsFiles)
	if len(claudeFiles) != len(agentsFiles) {
		t.Errorf("Files manifests differ in length: claude=%v agents=%v", claudeFiles, agentsFiles)
	} else {
		for i := range claudeFiles {
			if claudeFiles[i] != agentsFiles[i] {
				t.Errorf("Files manifests differ: claude=%v agents=%v", claudeFiles, agentsFiles)
				break
			}
		}
	}

	for _, p := range []string{claude, agents} {
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

// TestAgentsSkillsReverseRetirement is the mirror of
// TestAgentsSkillsBothRootsAndIndependentRetirement: drop `skills` and keep
// `agents-skills`, so the .claude root retires and .agents is untouched.
// Retirement must not privilege either root.
func TestAgentsSkillsReverseRetirement(t *testing.T) {
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
	// Drop skills, keep agents-skills; the .claude root retires, .agents stays.
	writeSkillConfig(t, root, packDir, "targets:\n  - agents-skills\n")
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("resync: %d\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(claude, "SKILL.md")); !os.IsNotExist(err) {
		t.Error(".claude SKILL.md should be retired")
	}
	if _, err := os.Stat(filepath.Join(claude, "TEAM.md")); err != nil {
		t.Error(".claude TEAM.md (team-added) must survive retirement")
	}
	if _, err := os.Stat(claude); err != nil {
		t.Error(".claude skill directory must remain because a team file is left in it")
	}
	if _, err := os.Stat(filepath.Join(agents, "SKILL.md")); err != nil {
		t.Error(".agents SKILL.md must remain")
	}
	if _, err := os.Stat(filepath.Join(agents, "TEAM.md")); err != nil {
		t.Error(".agents TEAM.md must remain")
	}
}
