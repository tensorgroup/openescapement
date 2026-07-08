package render

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

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
