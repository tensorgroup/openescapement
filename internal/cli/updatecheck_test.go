// internal/cli/updatecheck_test.go
package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/updatecheck"
)

// packRepoWithCheck builds a pack repo whose manifest declares update_check.
func packRepoWithCheck(t *testing.T, version, every string) string {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"org/pack.yaml": "schema: 1\nname: acme-org\nversion: " + version +
			"\ndescription: d\nrules: [rules/a.md]\nupdate_check:\n  every: " + every + "\n",
		"org/rules/a.md": "## A\nv" + version + "\n",
	})
	gitIn(t, dir, "init", "-q", "-b", "main")
	gitIn(t, dir, "config", "user.email", "t@e.com")
	gitIn(t, dir, "config", "user.name", "T")
	gitIn(t, dir, "config", "commit.gpgsign", "false")
	gitIn(t, dir, "config", "tag.gpgsign", "false")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "v"+version)
	gitIn(t, dir, "tag", "-a", "v"+version, "-m", "v"+version)
	return dir
}

// backdateLog rewrites every log entry's timestamp to the distant past so the
// next command is guaranteed overdue.
func backdateLog(t *testing.T, root string) {
	t.Helper()
	p := filepath.Join(root, ".escapement", "update-log.jsonl")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	out := strings.ReplaceAll(string(data), `"time":"20`, `"time":"19`)
	os.WriteFile(p, []byte(out), 0o644)
}

func TestSyncRecordsCheckAndScaffoldsGitignore(t *testing.T) {
	repo := packRepoWithCheck(t, "1.0.0", "7d")
	root := newGoverned(t, repo, "v1.0.0")
	// newGoverned points at the org subdir already.
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("sync: %d\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(root, ".escapement", "update-log.jsonl")); err != nil {
		t.Fatalf("sync did not record a check entry: %v", err)
	}
	gi, err := os.ReadFile(filepath.Join(root, ".escapement", ".gitignore"))
	if err != nil || !strings.Contains(string(gi), "update-log.jsonl") {
		t.Errorf("gitignore not scaffolded: %q %v", gi, err)
	}
}

func TestNoUpdateCheckStaysInert(t *testing.T) {
	repo := newPackRepo(t, "1.0.0") // no update_check
	root := newGoverned(t, repo, "v1.0.0")
	run(t, root, "sync")
	if _, err := os.Stat(filepath.Join(root, ".escapement", "update-log.jsonl")); !os.IsNotExist(err) {
		t.Error("inert feature must not write a log")
	}
	if code, out := run(t, root, "status", "--check"); code != 0 {
		t.Errorf("inert repo status --check: exit %d\n%s", code, out)
	}
}

func TestStatusSurfacesPackStale(t *testing.T) {
	repo := packRepoWithCheck(t, "1.0.0", "7d")
	root := newGoverned(t, repo, "v1.0.0")
	run(t, root, "sync")
	// Publish v2.0.0.
	writeFiles(t, repo, map[string]string{
		"org/pack.yaml":  "schema: 1\nname: acme-org\nversion: 2.0.0\ndescription: d\nrules: [rules/a.md]\nupdate_check:\n  every: 7d\n",
		"org/rules/a.md": "## A\nv2.0.0\n",
	})
	gitIn(t, repo, "add", "-A")
	gitIn(t, repo, "commit", "-q", "-m", "v2")
	gitIn(t, repo, "tag", "-a", "v2.0.0", "-m", "v2")
	// Force the next command to be overdue; it runs a real ls-remote check.
	backdateLog(t, root)
	code, out := run(t, root, "status", "--check")
	if code != 1 {
		t.Fatalf("stale pack: want exit 1, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "pack-stale") || !strings.Contains(out, "v2.0.0") {
		t.Errorf("status should report pack-stale to v2.0.0:\n%s", out)
	}
}

func TestInitScaffoldsGitignore(t *testing.T) {
	root := t.TempDir()
	if code, out := run(t, root, "init"); code != 0 {
		t.Fatalf("init: %d\n%s", code, out)
	}
	gi, err := os.ReadFile(filepath.Join(root, ".escapement", ".gitignore"))
	if err != nil || !strings.Contains(string(gi), "update-log.jsonl") {
		t.Errorf("init gitignore: %q %v", gi, err)
	}
}

// bumpPins is the accept-path helper (esc update --ref equivalent): it must
// re-pin only stale tag packs, leaving branch pins untouched for a plain sync
// to pick up. Exercised directly because the interactive accept branch of
// checkForUpdates can only be reached from a real TTY, which cannot be
// injected in tests without a pty dependency.
func TestBumpPinsRePinsTagOnlyAndSkipsBranches(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".escapement"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgYAML := "schema: 1\npacks:\n" +
		"  - source: github.com/acme/policy-packs//org\n    ref: v1.0.0\n" +
		"  - source: github.com/acme/other//org\n    ref: main\n"
	if err := os.WriteFile(filepath.Join(root, ".escapement", "config.yaml"), []byte(cfgYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	statuses := []updatecheck.PackStatus{
		{Source: "github.com/acme/policy-packs//org", Kind: "tag", Pinned: "v1.0.0", Latest: "v2.0.0", Updates: true},
		{Source: "github.com/acme/other//org", Kind: "branch", Pinned: "main", Latest: "abc123def456", Updates: true},
	}
	if err := bumpPins(root, statuses); err != nil {
		t.Fatalf("bumpPins: %v", err)
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Packs[0].Ref != "v2.0.0" {
		t.Errorf("tag pack not re-pinned: got %q, want v2.0.0", cfg.Packs[0].Ref)
	}
	if cfg.Packs[1].Ref != "main" {
		t.Errorf("branch pack pin must be left unchanged: got %q", cfg.Packs[1].Ref)
	}
}

// TestBumpPinsNoStaleTagsLeavesConfigUntouched confirms the no-op path never
// rewrites config.yaml when nothing needs re-pinning.
func TestBumpPinsNoStaleTagsLeavesConfigUntouched(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".escapement"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgYAML := "schema: 1\npacks:\n  - source: github.com/acme/policy-packs//org\n    ref: v1.0.0\n"
	cfgPath := filepath.Join(root, ".escapement", "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	statuses := []updatecheck.PackStatus{
		{Source: "github.com/acme/policy-packs//org", Kind: "tag", Pinned: "v1.0.0", Latest: "v1.0.0", Updates: false},
	}
	if err := bumpPins(root, statuses); err != nil {
		t.Fatalf("bumpPins: %v", err)
	}
	after, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("config.yaml rewritten with no stale tags:\nbefore: %q\nafter:  %q", before, after)
	}
}

var _ = exec.Command // keep os/exec import if unused elsewhere
