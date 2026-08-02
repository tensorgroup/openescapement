package render

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/tensorgroup/openescapement/internal/pack"
)

var update = flag.Bool("update", false, "rewrite golden files")

// customOwner is a pack that both defines catalog entries and a fragment
// explicitly naming a custom target, so the golden proves the catalog
// carve-out: custom output carries the fragment but no catalog section.
func customOwner() *pack.Pack {
	return &pack.Pack{
		Manifest: pack.Manifest{
			Name: "acme-org", Version: "1.4.0",
			Catalog: []pack.CatalogEntry{
				{Name: "Tailscale", Category: "hosting-exposure", Status: "preferred", Notes: "Org tailnet"},
			},
			CustomTargets: []pack.CustomTarget{
				{Name: "copilot", File: ".github/copilot-instructions.md"},
			},
		},
		Fragments: []pack.Fragment{
			{Path: "rules/secrets.md", Body: "## Secrets\nUse Vault.\n"}, // no targets: never in custom
			{Path: "rules/copilot.md", Targets: []string{"copilot"}, Body: "## Copilot\nUse suggestions carefully.\n"},
		},
	}
}

// TestGolden locks the renderer's byte-exact output — determinism is the
// product claim. Regenerate deliberately with `go test ./internal/render
// -run TestGolden -update` and review the diff like a policy change.
func TestGolden(t *testing.T) {
	packs := testPacks()
	cases := map[string]string{
		"claude.golden.md":     Compose(packs, TargetClaude),
		"agents.golden.md":     Compose(packs, TargetAgents),
		"gemini.golden.md":     Compose(packs, TargetGemini),
		"governance.golden.md": Governance(packs),
		"copilot.golden.md":    ComposeCustom(customOwner(), "copilot"),
	}
	for name, got := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("testdata", name)
			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("missing golden file (run with -update): %v", err)
			}
			if got != string(want) {
				t.Errorf("output differs from %s — if intentional, re-run with -update and review the diff\ngot:\n%s", path, got)
			}
		})
	}
}
