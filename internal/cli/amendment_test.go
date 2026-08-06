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

// TestDroppedMCPServerNotMisattributedAsAmendment covers the gap between a
// pack dropping a server it used to own and the next `esc sync` actually
// removing that server's entry from .mcp.json. In that gap the server is
// still escapement's own content, rendered by an earlier sync, not
// something a team added, and it must not be reported as a local
// amendment: `esc update` (config + lockfile pin move, no re-render) is the
// standard way to get into that gap without a full sync.
func TestDroppedMCPServerNotMisattributedAsAmendment(t *testing.T) {
	packRepo := t.TempDir()
	writeFiles(t, packRepo, map[string]string{
		"org/pack.yaml": `schema: 1
name: acme-mcp
version: 1.0.0
description: Acme MCP fixture
mcp:
  servers:
    acme-paved-path:
      command: npx
      args: ["-y", "@acme/paved-path-mcp"]
    acme-extra:
      command: npx
      args: ["-y", "@acme/extra-mcp"]
`,
	})
	gitIn(t, packRepo, "init", "-q", "-b", "main")
	gitIn(t, packRepo, "config", "user.email", "t@e.com")
	gitIn(t, packRepo, "config", "user.name", "T")
	gitIn(t, packRepo, "config", "commit.gpgsign", "false")
	gitIn(t, packRepo, "config", "tag.gpgsign", "false")
	gitIn(t, packRepo, "add", "-A")
	gitIn(t, packRepo, "commit", "-q", "-m", "v1")
	gitIn(t, packRepo, "tag", "-a", "v1.0.0", "-m", "v1")

	root := newGoverned(t, packRepo, "v1.0.0")
	runEsc(t, root, "sync")
	if _, err := os.Stat(filepath.Join(root, ".mcp.json")); err != nil {
		t.Fatalf(".mcp.json not synced: %v", err)
	}

	// Publish v2.0.0, a new tag (not a moved one — moving v1.0.0 in place
	// would trip the integrity check, per TestMovedTagFailsClosed), that
	// drops acme-extra.
	writeFiles(t, packRepo, map[string]string{
		"org/pack.yaml": `schema: 1
name: acme-mcp
version: 2.0.0
description: Acme MCP fixture
mcp:
  servers:
    acme-paved-path:
      command: npx
      args: ["-y", "@acme/paved-path-mcp"]
`,
	})
	gitIn(t, packRepo, "add", "-A")
	gitIn(t, packRepo, "commit", "-q", "-m", "v2")
	gitIn(t, packRepo, "tag", "-a", "v2.0.0", "-m", "v2")

	// Re-pin to v2.0.0 without syncing: cmdUpdate moves the config pin and
	// the lockfile's pack pin, but deliberately leaves lock.Artifacts (the
	// per-artifact Hash/Keys recorded by the last sync) untouched — so the
	// plan's desired keys (v2.0.0: acme-paved-path only) diverge from both
	// the lockfile's recorded keys and the on-disk file (v1.0.0: both
	// servers) until the next sync.
	runEsc(t, root, "update", "--ref", "v2.0.0")

	st, err := engine.Status(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, f := range st.Findings {
		if f.Path != ".mcp.json" {
			continue
		}
		found = true
		if f.Local != engine.LocalNone {
			t.Errorf("local axis = %q, want none (escapement's own not-yet-synced server misattributed as a team amendment): amendment=%+v", f.Local, f.Amendment)
		}
		// The managed axis reports in-sync here, not stale: acme-paved-path
		// is the only key the v2.0.0 plan owns, its rendered content is
		// identical between v1.0.0 and v2.0.0, and the owned-subset hash
		// (render.OwnedMCPHash) is computed over that one key on both
		// sides. The point under test is the local axis: acme-extra must
		// not surface as a team amendment just because the pack dropped it
		// and no sync has run yet to remove it from disk.
		if f.State != engine.InSync {
			t.Errorf("managed axis = %q, want in-sync", f.State)
		}
	}
	if !found {
		t.Fatal("no finding for .mcp.json")
	}
}
