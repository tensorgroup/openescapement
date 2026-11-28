package render_test

import (
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/pack"
	"github.com/tensorgroup/openescapement/internal/render"
)

func TestGovernanceListsRulesFragments(t *testing.T) {
	p := &pack.Pack{
		Manifest: pack.Manifest{Name: "acme", Version: "1.0.0"},
		Fragments: []pack.Fragment{
			{Path: "rules/api.md", Targets: []string{"rules"}, Paths: []string{"src/api/**"}, Body: "scope the API\n"},
			{Path: "rules/all.md", Targets: []string{"rules"}, Body: "always on\n"},
			{Path: "rules/blk.md", Targets: []string{"claude"}, Body: "block only\n"},
		},
	}
	g := render.Governance([]*pack.Pack{p})
	if !strings.Contains(g, "esc-acme-api.md") || !strings.Contains(g, "src/api/**") {
		t.Errorf("governance must list the scoped rule file and its paths:\n%s", g)
	}
	if !strings.Contains(g, "esc-acme-all.md") {
		t.Errorf("governance must list the unconditional rule file:\n%s", g)
	}
	if !strings.Contains(g, "scope the API") || !strings.Contains(g, "always on") {
		t.Errorf("governance must include rule bodies:\n%s", g)
	}
}

func TestBlockNeverLeaksPathsFrontmatter(t *testing.T) {
	p := &pack.Pack{
		Manifest: pack.Manifest{Name: "acme", Version: "1.0.0"},
		Fragments: []pack.Fragment{
			{Path: "rules/api.md", Targets: []string{"rules", "claude"}, Paths: []string{"src/api/**"}, Body: "shared body\n"},
		},
	}
	block := render.Compose([]*pack.Pack{p}, render.TargetClaude)
	if strings.Contains(block, "paths:") || strings.Contains(block, "---") {
		t.Errorf("block must not carry rule frontmatter:\n%s", block)
	}
	if !strings.Contains(block, "shared body") {
		t.Errorf("block should still contain the shared body:\n%s", block)
	}
}
