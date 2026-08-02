package render

import (
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/pack"
)

func testPacks() []*pack.Pack {
	org := &pack.Pack{
		Manifest: pack.Manifest{
			Name: "acme-org", Version: "1.4.0",
			Catalog: []pack.CatalogEntry{
				{Name: "Tailscale", Category: "hosting-exposure", Status: "preferred", Notes: "Org tailnet"},
				{Name: "Lovable", Category: "app-hosting", Status: "allowed", Notes: "POCs only"},
				{Name: "Cloudflare Tunnel", Category: "hosting-exposure", Status: "review-required", Notes: "Ask #platform"},
				{Name: "Raw port forwarding", Category: "hosting-exposure", Status: "banned"},
			},
		},
		Fragments: []pack.Fragment{
			{Path: "rules/secrets.md", Body: "## Secrets\nUse Vault.\n"},
			{Path: "rules/claude-only.md", Targets: []string{"claude"}, Body: "## Claude only\nBe careful.\n"},
			{Path: "rules/human-note.md", Targets: []string{"governance"}, Body: "## For humans\nRead the wiki.\n"},
		},
	}
	dept := &pack.Pack{
		Manifest: pack.Manifest{Name: "eng-dept", Version: "2.1.0"},
		Fragments: []pack.Fragment{
			{Path: "rules/ci.md", Body: "## CI\nAll repos use Actions.\n"},
		},
	}
	return []*pack.Pack{org, dept}
}

func TestComposeTargets(t *testing.T) {
	packs := testPacks()
	claude := Compose(packs, TargetClaude)
	if !strings.Contains(claude, "## Secrets") || !strings.Contains(claude, "## Claude only") {
		t.Errorf("claude missing fragments:\n%s", claude)
	}
	if strings.Contains(claude, "## For humans") {
		t.Errorf("governance-only fragment leaked into claude:\n%s", claude)
	}
	agents := Compose(packs, TargetAgents)
	if strings.Contains(agents, "## Claude only") {
		t.Errorf("claude-only fragment leaked into agents:\n%s", agents)
	}
	// Pack order preserved: org before dept.
	if strings.Index(claude, "## Secrets") > strings.Index(claude, "## CI") {
		t.Errorf("pack order wrong:\n%s", claude)
	}
	// Catalog rendered as directive text, grouped by status order.
	for _, want := range []string{"**Preferred:** Tailscale", "**Banned:** Raw port forwarding"} {
		if !strings.Contains(claude, want) {
			t.Errorf("catalog text missing %q:\n%s", want, claude)
		}
	}
	if strings.Index(claude, "Tailscale") > strings.Index(claude, "Raw port forwarding") {
		t.Errorf("catalog status order wrong:\n%s", claude)
	}
	// Managed notice present.
	if !strings.Contains(claude, "Managed by escapement") {
		t.Errorf("notice missing:\n%s", claude)
	}
}

func TestComposeDeterministic(t *testing.T) {
	a := Compose(testPacks(), TargetClaude)
	b := Compose(testPacks(), TargetClaude)
	if a != b {
		t.Error("Compose not deterministic")
	}
	if !strings.HasSuffix(a, "\n") {
		t.Error("Compose body must end with newline")
	}
}

func TestComposeCustom(t *testing.T) {
	body := ComposeCustom(customOwner(), "copilot")
	if !strings.Contains(body, "## Copilot") {
		t.Errorf("custom body missing explicitly-targeted fragment:\n%s", body)
	}
	if strings.Contains(body, "## Secrets") {
		t.Errorf("untargeted fragment leaked into custom target:\n%s", body)
	}
	if strings.Contains(body, "Tool & service policy") || strings.Contains(body, "Tailscale") {
		t.Errorf("catalog section must not appear in a custom target file (carve-out):\n%s", body)
	}
	if !strings.Contains(body, "Managed by escapement") {
		t.Errorf("notice missing:\n%s", body)
	}
	if !strings.HasSuffix(body, "\n") {
		t.Errorf("custom body must end with newline")
	}
}

func TestRemoveBlock(t *testing.T) {
	spliced, err := Splice([]byte("# Team\n\nkeep me\n"), "policy body\n", BlockMeta{Packs: []string{"p@1"}})
	if err != nil {
		t.Fatal(err)
	}
	out, removed, err := RemoveBlock(spliced)
	if err != nil || !removed {
		t.Fatalf("RemoveBlock: removed=%v err=%v", removed, err)
	}
	if !strings.Contains(string(out), "keep me") || strings.Contains(string(out), "policy body") {
		t.Errorf("RemoveBlock must strip only the managed block:\n%s", out)
	}
	// Idempotent: no block present now.
	if _, again, _ := RemoveBlock(out); again {
		t.Error("RemoveBlock reported a second block")
	}
	// A file that is only a managed block becomes byte-empty.
	only, err := Splice(nil, "x\n", BlockMeta{Packs: []string{"p@1"}})
	if err != nil {
		t.Fatal(err)
	}
	empty, removed, err := RemoveBlock(only)
	if err != nil || !removed {
		t.Fatalf("RemoveBlock only-block: %v %v", removed, err)
	}
	if len(empty) != 0 {
		t.Errorf("removing the sole block should leave byte-empty content, got %q", empty)
	}
}

func TestGovernance(t *testing.T) {
	g := Governance(testPacks())
	for _, want := range []string{
		"# Governance", "acme-org", "1.4.0",
		"## For humans",    // governance-targeted fragment included
		"| Tailscale",      // catalog table
		"| banned",         // status column
		"hosting-exposure", // category
		"## Secrets",       // untargeted fragments included for humans too
	} {
		if !strings.Contains(g, want) {
			t.Errorf("governance missing %q:\n%s", want, g)
		}
	}
	if strings.Contains(g, "## Claude only") {
		t.Errorf("claude-only fragment should not be in governance doc:\n%s", g)
	}
}
