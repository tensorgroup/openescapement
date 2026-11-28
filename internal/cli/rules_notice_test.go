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
	// Exactly one notice line, not merely "the phrase appears somewhere" —
	// the CLI prints one "  notice: " line per plan.Notices entry.
	if n := strings.Count(out, "  notice: "); n != 1 {
		t.Errorf("want exactly one notice line, got %d:\n%s", n, out)
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
	// "skipped" alone is vacuous — the summary line's own header always
	// contains that word ("%d artifact(s) applied, %d skipped:") regardless
	// of the count. Require the artifact's own path in a skip line instead.
	if code, out := run(t, root, "sync"); code != 0 || !strings.Contains(out, "skipped .claude/rules/") {
		t.Fatalf("hand-edited rule file must be skipped (code=%d):\n%s", code, out)
	}
	if got, _ := os.ReadFile(rule); string(got) != "hand edited\n" {
		t.Errorf("skip must not overwrite: %q", got)
	}
	// The skipped sync must not silently converge the lockfile hash — the
	// lock entry is carried forward unchanged, so the alteration keeps
	// reporting until someone resolves it (esc sync's hand-edit invariant).
	if code, out := run(t, root, "status"); code != 0 || !strings.Contains(out, "altered") {
		t.Fatalf("status after a skipped sync must still report the rule file altered (code=%d):\n%s", code, out)
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
	rule := filepath.Join(root, ".claude", "rules", "esc-acme-api.md")
	first, err := os.ReadFile(rule)
	if err != nil {
		t.Fatal(err)
	}
	// Not a no-op in the write sense: apply's KindFile case rewrites every
	// whole file on every sync today (no hash-equal short-circuit) and
	// counts it as Applied, not skipped. What must hold is that nothing is
	// skipped and the rewritten bytes are identical to what the first sync
	// produced.
	if code, out := run(t, root, "sync"); code != 0 || !strings.Contains(out, "0 skipped") {
		t.Fatalf("second sync must apply with 0 skipped: %d\n%s", code, out)
	}
	if code, out := run(t, root, "status", "--check"); code != 0 {
		t.Fatalf("status --check must still be clean after the second sync: %d\n%s", code, out)
	}
	second, err := os.ReadFile(rule)
	if err != nil {
		t.Fatal(err)
	}
	if string(second) != string(first) {
		t.Errorf("rule file bytes changed across an unmodified second sync:\nfirst:  %q\nsecond: %q", first, second)
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

// TestStatusReportsOrphanRuleFile switches the repo to targets: [claude],
// which orphans the rule file but also orphans AGENTS.md, GEMINI.md, and
// GOVERNANCE.md (every other built-in target this repo had). Asserting only
// a non-zero status --check exit would pass even if the rule file's own
// orphan detection were broken, since those other orphans alone already
// fail the gate. Assert the rule file's own finding by path instead.
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

	out, code := runEscOut(t, root, "status", "--json")
	if code != 0 {
		t.Fatalf("status --json exit %d, want 0 (bare status never gates on drift): %s", code, out)
	}
	f := findOrphan(t, out, ".claude/rules/esc-acme-api.md")
	if f == nil {
		t.Fatalf("no orphan finding for the rule file itself, got:\n%s", out)
	}
	if f.Managed != "orphan" {
		t.Errorf("managed = %q, want orphan", f.Managed)
	}
	if code, out := run(t, root, "status", "--check"); code != 1 {
		t.Errorf("status --check must exit 1 on an orphaned rule file, got %d:\n%s", code, out)
	}
}
