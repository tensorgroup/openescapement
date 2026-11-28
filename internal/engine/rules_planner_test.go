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

func writeRulePack(t *testing.T, name, version string, rules map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	man := "schema: 1\nname: " + name + "\nversion: " + version + "\nrules:\n"
	for rel := range rules {
		man += "  - " + rel + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(man), 0o644); err != nil {
		t.Fatal(err)
	}
	for rel, content := range rules {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func planLocal(t *testing.T, cfg *config.Config) (*PlanResult, error) {
	t.Helper()
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	return planFromConfig(context.Background(), t.TempDir(), cfg)
}

func ruleCfg(targetsList []string, packs ...string) *config.Config {
	c := &config.Config{Targets: targetsList}
	for _, pd := range packs {
		c.Packs = append(c.Packs, config.PackRef{Source: pd, Ref: "", Trust: "unsigned"})
	}
	return c
}

func TestRulesPlannerEmitsRuleFile(t *testing.T) {
	pd := writeRulePack(t, "acme", "1.0.0", map[string]string{
		"rules/api.md": "---\ntargets: [rules]\npaths:\n  - \"src/**\"\n---\nbody\n",
	})
	res, err := planLocal(t, ruleCfg(nil, pd)) // empty targets = defaults, includes rules
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	var found *Artifact
	for i := range res.Artifacts {
		if res.Artifacts[i].Path == ".claude/rules/esc-acme-api.md" {
			found = &res.Artifacts[i]
		}
	}
	if found == nil {
		t.Fatalf("rule file artifact missing")
	}
	if found.Kind != KindFile || !strings.Contains(found.Body, "src/**") {
		t.Errorf("rule artifact wrong: %+v", found)
	}
}

func TestRulesCrossPackCollisionIsConfigError(t *testing.T) {
	// Two DIFFERENT packs, same name, same stem → same rule path → ErrConfig.
	a := writeRulePack(t, "shared", "1.0.0", map[string]string{"rules/api.md": "---\ntargets: [rules]\n---\na\n"})
	b := writeRulePack(t, "shared", "2.0.0", map[string]string{"rules/api.md": "---\ntargets: [rules]\n---\nb\n"})
	if _, err := planLocal(t, ruleCfg(nil, a, b)); !errors.Is(err, esc.ErrConfig) {
		t.Fatalf("cross-pack rule collision must be ErrConfig, got %v", err)
	}
}

func TestRulesCrossPackCollisionCaseFolded(t *testing.T) {
	a := writeRulePack(t, "Acme", "1.0.0", map[string]string{"rules/api.md": "---\ntargets: [rules]\n---\na\n"})
	b := writeRulePack(t, "acme", "1.0.0", map[string]string{"rules/api.md": "---\ntargets: [rules]\n---\nb\n"})
	if _, err := planLocal(t, ruleCfg(nil, a, b)); !errors.Is(err, esc.ErrConfig) {
		t.Fatalf("case-folded cross-pack collision must be ErrConfig, got %v", err)
	}
}

// TestUntargetedFragmentDoesNotRenderAsRuleFile is the engine-level half of
// the spec's "a fragment with no frontmatter does not render as a rule file"
// claim. internal/pack already covers that a no-frontmatter fragment gets no
// Targets/Paths (TestNoFrontmatterFragmentHasNoPaths); this plans a pack
// containing one under the default target set (which includes rules) and
// asserts no .claude/rules/ artifact is produced for it — an untargeted
// fragment is never treated as implicitly naming rules, unlike the "applies
// to every built-in target" rule that governs the block targets.
func TestUntargetedFragmentDoesNotRenderAsRuleFile(t *testing.T) {
	pd := writeRulePack(t, "acme", "1.0.0", map[string]string{
		"rules/api.md": "no frontmatter here, just a plain body\n",
	})
	res, err := planLocal(t, ruleCfg(nil, pd)) // empty targets = defaults, includes rules
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	for i := range res.Artifacts {
		if strings.HasPrefix(res.Artifacts[i].Path, ".claude/rules/") {
			t.Errorf("an untargeted (no-frontmatter) fragment must not render as a rule file: %+v", res.Artifacts[i])
		}
	}
}

func TestRulesFilterExcludedNotice(t *testing.T) {
	pd := writeRulePack(t, "acme", "1.0.0", map[string]string{"rules/api.md": "---\ntargets: [rules]\n---\na\n"})
	res, err := planLocal(t, ruleCfg([]string{"claude"}, pd))
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	for i := range res.Artifacts {
		if strings.HasPrefix(res.Artifacts[i].Path, ".claude/rules/") {
			t.Fatal("no rule file when rules is filtered out")
		}
	}
	if len(res.Notices) == 0 || !strings.Contains(res.Notices[0], "targets rules") {
		t.Errorf("want a filter-excludes-rules notice, got %v", res.Notices)
	}
}
