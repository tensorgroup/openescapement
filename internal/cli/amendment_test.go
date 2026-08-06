package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/engine"
)

// TestAmendedBlockReportsInSync confirms the two axes stay independent: a
// team appending its own content next to a managed block reports in-sync on
// the managed axis and amended on the local axis, and does not flip
// StatusResult.Clean() to unclean. AGENTS.md is used because it is a
// KindBlock artifact the fixture pack (target "agents") actually produces on
// sync — see packRepoFiles's "targets: [claude, agents, gemini, governance]"
// in cli_test.go and render.TargetFile["agents"] = "AGENTS.md".
func TestAmendedBlockReportsInSync(t *testing.T) {
	repo := setupGovernedRepo(t)
	runEsc(t, repo, "sync")

	path := filepath.Join(repo, "AGENTS.md")
	existing, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(existing, []byte("\n## Team rules\n\nBe kind.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := engine.Status(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, f := range st.Findings {
		if f.Path != "AGENTS.md" {
			continue
		}
		found = true
		if f.State != engine.InSync {
			t.Errorf("managed axis = %q, want in-sync", f.State)
		}
		if f.Local != engine.LocalAmended {
			t.Errorf("local axis = %q, want amended", f.Local)
		}
		if f.Amendment == nil || !strings.Contains(f.Amendment.Content, "Be kind.") {
			t.Errorf("amendment = %+v", f.Amendment)
		}
	}
	if !found {
		t.Fatal("no finding for AGENTS.md")
	}
	if !st.Clean() {
		t.Error("an amendment alone must not make status unclean")
	}
}
