package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/portal/publish"
	"github.com/tensorgroup/openescapement/internal/portal/seed"
)

// TestDemoPublishSyncLoop is the pitch loop as a test: portal publish -> esc
// sync -> rule text lands in CLAUDE.md, with no server-side push involved.
func TestDemoPublishSyncLoop(t *testing.T) {
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	dataDir := t.TempDir()
	packDir, demoRepo, err := seed.Repos(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	// 1. Initial sync: managed block lands at v1.2.0.
	code, out := run(t, demoRepo, "sync")
	if code != 0 {
		t.Fatalf("sync: %d %s", code, out)
	}
	claude, _ := os.ReadFile(filepath.Join(demoRepo, "CLAUDE.md"))
	if !strings.Contains(string(claude), "org-baseline@1.2.0") {
		t.Fatalf("initial block: %s", claude)
	}
	// 2. Publish v1.3.0 through the portal's publish manager.
	m := publish.NewManager(filepath.Dir(packDir))
	cur, err := m.ReadFragment("org-baseline", "rules/security.md")
	if err != nil {
		t.Fatal(err)
	}
	newBody := append(cur, []byte("- Model routing: use fast models for code, reasoning models for review.\n")...)
	if err := m.Publish(context.Background(), "org-baseline", "rules/security.md", newBody, "1.3.0"); err != nil {
		t.Fatal(err)
	}
	// 3. Re-pin demo repo to v1.3.0 and sync (the on-screen pitch step).
	cfgPath := filepath.Join(demoRepo, ".escapement", "config.yaml")
	cfg, _ := os.ReadFile(cfgPath)
	if err := os.WriteFile(cfgPath,
		[]byte(strings.ReplaceAll(string(cfg), "v1.2.0", "v1.3.0")), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out = run(t, demoRepo, "sync")
	if code != 0 {
		t.Fatalf("resync: %d %s", code, out)
	}
	claude, _ = os.ReadFile(filepath.Join(demoRepo, "CLAUDE.md"))
	if !strings.Contains(string(claude), "org-baseline@1.3.0") ||
		!strings.Contains(string(claude), "Model routing") {
		t.Fatalf("published rule did not arrive:\n%s", claude)
	}
}
