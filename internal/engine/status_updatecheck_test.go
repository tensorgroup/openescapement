package engine

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/tensorgroup/openescapement/internal/updatecheck"
)

// govWithUpdateCheck builds a governed repo synced against a pack repo that
// declares update_check, returning the governed root and the pack repo path.
func govWithUpdateCheck(t *testing.T) (root, packRepo string) {
	t.Helper()
	return govRepo(t, "schema: 1\nname: acme\nversion: 1.0.0\nrules: [rules/a.md]\n"+
		"update_check:\n  every: 7d\n")
}

// govRepo builds a governed repo synced against a pack repo whose pack.yaml
// is the given content, returning the governed root and the pack repo path.
func govRepo(t *testing.T, packYAML string) (root, packRepo string) {
	t.Helper()
	packRepo = t.TempDir()
	files := map[string]string{
		"org/pack.yaml":  packYAML,
		"org/rules/a.md": "## A\none\n",
	}
	for rel, c := range files {
		p := filepath.Join(packRepo, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(c), 0o644)
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "t@e.com"}, {"config", "user.name", "T"},
		{"config", "commit.gpgsign", "false"}, {"config", "tag.gpgsign", "false"},
		{"add", "-A"}, {"commit", "-q", "-m", "v1"}, {"tag", "-a", "v1.0.0", "-m", "v1"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = packRepo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	root = t.TempDir()
	os.MkdirAll(filepath.Join(root, ".escapement"), 0o755)
	os.WriteFile(filepath.Join(root, ".escapement", "config.yaml"),
		[]byte("schema: 1\npacks:\n  - source: file://"+packRepo+"//org\n    ref: v1.0.0\n    trust: unsigned\n"), 0o644)
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	if err := ApplyPlan(t, root); err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	return root, packRepo
}

// ApplyPlan runs Plan+Apply once (helper local to the test).
func ApplyPlan(t *testing.T, root string) error {
	t.Helper()
	p, err := Plan(context.Background(), root)
	if err != nil {
		return err
	}
	return Apply(root, p)
}

func TestStatusPackStale(t *testing.T) {
	root, _ := govWithUpdateCheck(t)
	// A recent successful check that saw updates available.
	appendTestLog(t, root, updatecheck.Entry{
		Time:    time.Now().UTC(),
		Outcome: updatecheck.OutcomeOKUpdates,
		Cadence: "7d",
		Prompt:  "none",
		Packs: []updatecheck.PackStatus{{
			Source: "file://x//org", Kind: "tag", Pinned: "v1.0.0", Latest: "v2.0.0", Updates: true,
		}},
	})
	st, err := Status(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !hasState(st, PackStale) {
		t.Fatalf("want pack-stale finding, got %+v", st.Findings)
	}
	if st.Clean() {
		t.Error("pack-stale should make status not clean")
	}
}

func TestStatusCheckOverdue(t *testing.T) {
	root, _ := govWithUpdateCheck(t)
	// Last success is ancient (> cadence) and the newest attempt errored.
	appendTestLog(t, root, updatecheck.Entry{
		Time: time.Now().Add(-100 * 24 * time.Hour).UTC(), Outcome: updatecheck.OutcomeOKCurrent,
		Cadence: "7d", Prompt: "none",
	})
	appendTestLog(t, root, updatecheck.Entry{
		Time: time.Now().UTC(), Outcome: updatecheck.OutcomeError, Cadence: "7d", Prompt: "none",
	})
	st, err := Status(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !hasState(st, CheckOverdue) {
		t.Fatalf("want check-overdue finding, got %+v", st.Findings)
	}
}

// A failed attempt after a within-cadence success is not overdue: the
// cadence window is still satisfied by the recent success.
func TestStatusErrorAfterFreshSuccessNotOverdue(t *testing.T) {
	root, _ := govWithUpdateCheck(t)
	appendTestLog(t, root, updatecheck.Entry{
		Time: time.Now().Add(-1 * time.Hour).UTC(), Outcome: updatecheck.OutcomeOKCurrent,
		Cadence: "7d", Prompt: "none",
	})
	appendTestLog(t, root, updatecheck.Entry{
		Time: time.Now().UTC(), Outcome: updatecheck.OutcomeError, Cadence: "7d", Prompt: "none",
	})
	st, err := Status(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if hasState(st, CheckOverdue) {
		t.Errorf("error after within-cadence success must not be overdue, got %+v", st.Findings)
	}
	if hasState(st, PackStale) {
		t.Errorf("unexpected pack-stale finding, got %+v", st.Findings)
	}
	if !st.Clean() {
		t.Errorf("status should be clean, got %+v", st.Findings)
	}
}

// The stale signal comes from the last SUCCESSFUL check only: a newer
// ok_current success supersedes an older ok_updates entry.
func TestStatusNewerSuccessSupersedesStale(t *testing.T) {
	root, _ := govWithUpdateCheck(t)
	appendTestLog(t, root, updatecheck.Entry{
		Time:    time.Now().Add(-48 * time.Hour).UTC(),
		Outcome: updatecheck.OutcomeOKUpdates,
		Cadence: "7d",
		Prompt:  "none",
		Packs: []updatecheck.PackStatus{{
			Source: "file://x//org", Kind: "tag", Pinned: "v1.0.0", Latest: "v2.0.0", Updates: true,
		}},
	})
	appendTestLog(t, root, updatecheck.Entry{
		Time: time.Now().Add(-1 * time.Hour).UTC(), Outcome: updatecheck.OutcomeOKCurrent,
		Cadence: "7d", Prompt: "none",
	})
	st, err := Status(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if hasState(st, PackStale) {
		t.Errorf("newer ok_current success must supersede older ok_updates, got %+v", st.Findings)
	}
	if !st.Clean() {
		t.Errorf("status should be clean, got %+v", st.Findings)
	}
}

// When no pack currently declares update_check (cadence 0), the freshness
// logic is inert even if a populated update log exists on disk.
func TestStatusNoCadenceInertDespiteLog(t *testing.T) {
	root, _ := govRepo(t, "schema: 1\nname: acme\nversion: 1.0.0\nrules: [rules/a.md]\n")
	appendTestLog(t, root, updatecheck.Entry{
		Time:    time.Now().Add(-100 * 24 * time.Hour).UTC(),
		Outcome: updatecheck.OutcomeOKUpdates,
		Cadence: "7d",
		Prompt:  "none",
		Packs: []updatecheck.PackStatus{{
			Source: "file://x//org", Kind: "tag", Pinned: "v1.0.0", Latest: "v2.0.0", Updates: true,
		}},
	})
	st, err := Status(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if hasState(st, PackStale) || hasState(st, CheckOverdue) {
		t.Errorf("no update-check findings expected without a declared cadence, got %+v", st.Findings)
	}
	if !st.Clean() {
		t.Errorf("status should be clean, got %+v", st.Findings)
	}
}

func hasState(st *StatusResult, s State) bool {
	for _, f := range st.Findings {
		if f.State == s {
			return true
		}
	}
	return false
}

func appendTestLog(t *testing.T, root string, e updatecheck.Entry) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(root, ".escapement", "update-log.jsonl"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b, _ := json.Marshal(e)
	f.Write(append(b, '\n'))
}
