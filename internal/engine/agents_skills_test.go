package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/esc"
)

func writeSkillPack(t *testing.T, name string, skillFiles map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	man := "schema: 1\nname: " + name + "\nversion: 1.0.0\nskills:\n  - skills/demo\n"
	if err := os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(man), 0o644); err != nil {
		t.Fatal(err)
	}
	for rel, content := range skillFiles {
		p := filepath.Join(dir, "skills", "demo", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func skillCfg(pd string, targetsList ...string) *config.Config {
	return &config.Config{
		Schema:  1, // config.Load rejects schema 0
		Packs:   []config.PackRef{{Source: pd, Ref: "", Trust: "unsigned"}},
		Targets: targetsList,
	}
}

// writeNamedSkillPack writes a pack whose single skill entry sets an explicit
// on-disk name via the {path, name} object form (pack.SkillEntry), so two
// packs can be made to collide (or not) on a chosen target path independent
// of their pack names.
func writeNamedSkillPack(t *testing.T, packName, skillName string, skillFiles map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	man := "schema: 1\nname: " + packName + "\nversion: 1.0.0\nskills:\n  - path: skills/demo\n    name: " + skillName + "\n"
	if err := os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(man), 0o644); err != nil {
		t.Fatal(err)
	}
	for rel, content := range skillFiles {
		p := filepath.Join(dir, "skills", "demo", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func twoPackCfg(pdA, pdB string, targetsList ...string) *config.Config {
	return &config.Config{
		Schema: 1,
		Packs: []config.PackRef{
			{Source: pdA, Ref: "", Trust: "unsigned"},
			{Source: pdB, Ref: "", Trust: "unsigned"},
		},
		Targets: targetsList,
	}
}

// TestPlanSkillDirsCaseFoldedCollisionSameRoot covers the case-folded
// collision map inside planSkillDirs (engine.go): two packs whose skill
// names differ only by case resolve to the same on-disk path under one root
// (macOS/Windows fold case at the filesystem), so this must fail closed with
// ErrConfig naming both packs — not silently let the second one clobber the
// first on disk.
func TestPlanSkillDirsCaseFoldedCollisionSameRoot(t *testing.T) {
	pdA := writeNamedSkillPack(t, "acme", "Foo", map[string]string{"SKILL.md": "hi\n"})
	pdB := writeNamedSkillPack(t, "beta", "foo", map[string]string{"SKILL.md": "hi\n"})
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	_, err := planFromConfig(context.Background(), t.TempDir(), twoPackCfg(pdA, pdB, "skills"))
	if err == nil {
		t.Fatal("expected a case-folded collision error, got nil")
	}
	if !errors.Is(err, esc.ErrConfig) {
		t.Errorf("expected errors.Is(err, esc.ErrConfig); got %v", err)
	}
	if !strings.Contains(err.Error(), "acme") || !strings.Contains(err.Error(), "beta") {
		t.Errorf("error must name both packs (acme, beta): %v", err)
	}
}

// TestPlanSkillDirsDifferentNamesSameRootNoCollision is the companion case:
// two names that differ beyond case must plan both without error, so the
// case-fold isn't over-triggering on ordinary distinct names.
func TestPlanSkillDirsDifferentNamesSameRootNoCollision(t *testing.T) {
	pdA := writeNamedSkillPack(t, "acme", "Foo", map[string]string{"SKILL.md": "hi\n"})
	pdB := writeNamedSkillPack(t, "beta", "Bar", map[string]string{"SKILL.md": "hi\n"})
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	res, err := planFromConfig(context.Background(), t.TempDir(), twoPackCfg(pdA, pdB, "skills"))
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	want := map[string]bool{".claude/skills/Foo": false, ".claude/skills/Bar": false}
	for _, a := range res.Artifacts {
		if _, ok := want[a.Path]; ok {
			want[a.Path] = true
		}
	}
	for p, seen := range want {
		if !seen {
			t.Errorf("missing artifact %s", p)
		}
	}
}

func TestAgentsSkillsRendersSecondRoot(t *testing.T) {
	pd := writeSkillPack(t, "acme", map[string]string{"SKILL.md": "hi\n"})
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	res, err := planFromConfig(context.Background(), t.TempDir(), skillCfg(pd, "skills", "agents-skills"))
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	want := map[string]bool{".claude/skills/esc-acme-demo": false, ".agents/skills/esc-acme-demo": false}
	for _, a := range res.Artifacts {
		if _, ok := want[a.Path]; ok {
			if a.Kind != KindDir {
				t.Errorf("%s should be KindDir", a.Path)
			}
			want[a.Path] = true
		}
	}
	for p, seen := range want {
		if !seen {
			t.Errorf("missing artifact %s", p)
		}
	}
}

func TestAgentsSkillsExcludedFromDefaultSet(t *testing.T) {
	pd := writeSkillPack(t, "acme", map[string]string{"SKILL.md": "hi\n"})
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	res, err := planFromConfig(context.Background(), t.TempDir(), skillCfg(pd)) // empty = defaults
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	for _, a := range res.Artifacts {
		if a.Path == ".agents/skills/esc-acme-demo" {
			t.Fatal("agents-skills must not render under the default target set")
		}
	}
}

func TestOwnedSkillPathBothRoots(t *testing.T) {
	if !ownedSkillPath(".claude/skills/x") || !ownedSkillPath(".agents/skills/x") {
		t.Error("both skill roots must be owned")
	}
	if ownedSkillPath(".claude/skills/x/y") || ownedSkillPath("other/x") {
		t.Error("nested or foreign paths must not be owned")
	}
}

func TestSkillDuplicateNeverAttachesToAgentsSkills(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".claude", "skills", "esc-acme-demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pd := writeSkillPack(t, "acme", map[string]string{"SKILL.md": "hi\n"})
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".escapement"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := skillCfg(pd, "skills", "agents-skills").Save(root); err != nil {
		t.Fatal(err)
	}
	plan, err := Plan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(root, plan, false); err != nil {
		t.Fatal(err)
	}
	st, err := Status(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range st.Findings {
		if f.Subject == ".agents/skills/esc-acme-demo" && f.Duplicate != nil {
			t.Error(".agents/skills copy must never carry a SkillDuplicate")
		}
	}
}
