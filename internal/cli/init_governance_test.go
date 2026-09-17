package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitExplainsGovernanceAsOccupied(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "GOVERNANCE.md"), []byte("# my governance\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := run(t, root, "init", "--yes")
	if code != 0 {
		t.Fatalf("init: %d\n%s", code, out)
	}
	if !strings.Contains(out, "occupied") || !strings.Contains(out, "move or delete") {
		t.Errorf("init should explain GOVERNANCE.md as occupied with the remedy; got:\n%s", out)
	}
	if strings.Contains(out, "insert a managed block at the top of GOVERNANCE.md") {
		t.Errorf("GOVERNANCE.md must not be described as a managed-block target:\n%s", out)
	}
	if strings.Contains(out, "preserved byte for byte") {
		t.Errorf("a governance-only detection must not claim amendment preservation:\n%s", out)
	}
}
