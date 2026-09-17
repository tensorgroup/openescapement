package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// governRulePack writes a temp repo governed by a local pack with one rules
// fragment, and returns the repo root. targetsLine, if non-empty, is appended
// verbatim to config.yaml.
func governRulePack(t *testing.T, fragBody, targetsLine string) string {
	t.Helper()
	pd := t.TempDir()
	man := "schema: 1\nname: acme\nversion: 1.0.0\nrules:\n  - rules/api.md\n"
	if err := os.WriteFile(filepath.Join(pd, "pack.yaml"), []byte(man), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(pd, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pd, "rules", "api.md"), []byte(fragBody), 0o644); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".escapement"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "schema: 1\npacks:\n  - source: " + pd + "\n    ref: \"\"\n    trust: unsigned\n" + targetsLine
	if err := os.WriteFile(filepath.Join(root, ".escapement", "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestSyncPrintsRulesFilterNotice(t *testing.T) {
	root := governRulePack(t, "---\ntargets: [rules]\n---\nbody\n", "targets:\n  - claude\n")
	code, out := run(t, root, "sync")
	if code != 0 {
		t.Fatalf("sync: %d\n%s", code, out)
	}
	if !strings.Contains(out, "targets rules") {
		t.Errorf("sync must warn that the filter excludes rules:\n%s", out)
	}
}

func TestRuleFileHandEditSkipAndForce(t *testing.T) {
	root := governRulePack(t, "---\ntargets: [rules]\n---\nbody\n", "")
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("sync: %d\n%s", code, out)
	}
	rule := filepath.Join(root, ".claude", "rules", "esc-acme-api.md")
	if err := os.WriteFile(rule, []byte("hand edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := run(t, root, "sync"); code != 0 || !strings.Contains(out, "skipped") {
		t.Fatalf("hand-edited rule file must be skipped (code=%d):\n%s", code, out)
	}
	if got, _ := os.ReadFile(rule); string(got) != "hand edited\n" {
		t.Errorf("skip must not overwrite: %q", got)
	}
	if code, out := run(t, root, "sync", "--force"); code != 0 {
		t.Fatalf("sync --force: %d\n%s", code, out)
	}
	if got, _ := os.ReadFile(rule); string(got) == "hand edited\n" {
		t.Error("--force must overwrite the rule file")
	}
}

func TestRuleFileCRLFRoundTrip(t *testing.T) {
	root := governRulePack(t, "---\r\ntargets: [rules]\r\n---\r\nbody line\r\n", "")
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("sync: %d\n%s", code, out)
	}
	if code, out := run(t, root, "status", "--check"); code != 0 {
		t.Fatalf("status --check must be clean after a CRLF fragment sync: %d\n%s", code, out)
	}
	if code, out := run(t, root, "sync"); code != 0 || !strings.Contains(out, "0 skipped") {
		t.Fatalf("second sync should be a clean no-op: %d\n%s", code, out)
	}
}

func TestSymlinkedRulesDirRefusedExit4(t *testing.T) {
	root := governRulePack(t, "---\ntargets: [rules]\n---\nbody\n", "")
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, ".claude", "rules")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	for _, cmd := range [][]string{{"sync"}, {"status"}, {"diff"}} {
		if code, out := run(t, root, cmd...); code != 4 {
			t.Errorf("esc %v on a symlinked .claude/rules must exit 4, got %d:\n%s", cmd, code, out)
		}
	}
}

func TestStatusReportsOrphanRuleFile(t *testing.T) {
	root := governRulePack(t, "---\ntargets: [rules]\n---\nbody\n", "")
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("sync: %d\n%s", code, out)
	}
	cfgPath := filepath.Join(root, ".escapement", "config.yaml")
	cfg, _ := os.ReadFile(cfgPath)
	if err := os.WriteFile(cfgPath, append(cfg, []byte("targets:\n  - claude\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := run(t, root, "status", "--check"); code == 0 {
		t.Errorf("an orphaned rule file must fail status --check:\n%s", out)
	}
}
