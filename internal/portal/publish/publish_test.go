package publish

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func newPackClone(t *testing.T) (mgrDir string) {
	t.Helper()
	mgrDir = t.TempDir()
	dir := filepath.Join(mgrDir, "org-baseline")
	if err := os.MkdirAll(filepath.Join(dir, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"pack.yaml":         "schema: 1\nname: org-baseline\nversion: 1.2.0\n# keep this comment\nrules:\n  - rules/security.md\n",
		"rules/security.md": "---\ntargets: [claude, agents]\n---\n# Security\n\n- Never commit secrets.\n",
	}
	for p, c := range files {
		if err := os.WriteFile(filepath.Join(dir, p), []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "user.email", "t@t")
	git(t, dir, "config", "user.name", "t")
	git(t, dir, "config", "commit.gpgsign", "false")
	git(t, dir, "config", "tag.gpgsign", "false")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "init")
	git(t, dir, "tag", "-a", "v1.2.0", "-m", "v1.2.0")
	return mgrDir
}

func TestListAndGet(t *testing.T) {
	m := NewManager(newPackClone(t))
	infos, err := m.List(context.Background())
	if err != nil || len(infos) != 1 {
		t.Fatalf("infos=%v err=%v", infos, err)
	}
	p := infos[0]
	if p.Name != "org-baseline" || p.Version != "1.2.0" ||
		len(p.Fragments) != 1 || p.Fragments[0] != "rules/security.md" ||
		len(p.Tags) != 1 || p.Tags[0].Name != "v1.2.0" {
		t.Fatalf("info=%+v", p)
	}
}

func TestPublishHappyPath(t *testing.T) {
	m := NewManager(newPackClone(t))
	ctx := context.Background()
	newBody := "---\ntargets: [claude, agents]\n---\n# Security\n\n- Never commit secrets.\n- All new ports need review.\n"
	if err := m.Publish(ctx, "org-baseline", "rules/security.md", []byte(newBody), "1.3.0"); err != nil {
		t.Fatal(err)
	}
	p, err := m.Get(ctx, "org-baseline")
	if err != nil || p.Version != "1.3.0" || p.Tags[0].Name != "v1.3.0" {
		t.Fatalf("after publish: %+v err=%v", p, err)
	}
	manifest, _ := os.ReadFile(filepath.Join(m.Dir, "org-baseline", "pack.yaml"))
	if !strings.Contains(string(manifest), "version: 1.3.0") ||
		!strings.Contains(string(manifest), "# keep this comment") {
		t.Fatalf("manifest rewrite lost content:\n%s", manifest)
	}
	// worktree clean
	if out := git(t, filepath.Join(m.Dir, "org-baseline"), "status", "--porcelain"); out != "" {
		t.Fatalf("dirty worktree: %s", out)
	}
}

func TestPublishValidationFailureRestores(t *testing.T) {
	m := NewManager(newPackClone(t))
	ctx := context.Background()
	// Invalid: fragment declares a target outside ValidTargets.
	bad := "---\ntargets: [nonsense]\n---\nbody\n"
	err := m.Publish(ctx, "org-baseline", "rules/security.md", []byte(bad), "1.3.0")
	if err == nil {
		t.Fatal("expected validation error")
	}
	p, _ := m.Get(ctx, "org-baseline")
	if p.Version != "1.2.0" || len(p.Tags) != 1 {
		t.Fatalf("not restored: %+v", p)
	}
	body, _ := m.ReadFragment("org-baseline", "rules/security.md")
	if strings.Contains(string(body), "nonsense") {
		t.Fatal("fragment not restored")
	}
}

func TestPublishRejects(t *testing.T) {
	m := NewManager(newPackClone(t))
	ctx := context.Background()
	good := []byte("---\ntargets: [claude]\n---\nok\n")
	if err := m.Publish(ctx, "org-baseline", "../escape.md", good, "1.3.0"); err == nil {
		t.Fatal("path escape accepted")
	}
	if err := m.Publish(ctx, "org-baseline", "rules/security.md", good, "1.2.0"); err == nil {
		t.Fatal("existing version accepted")
	}
	if err := m.Publish(ctx, "org-baseline", "rules/security.md", good, "nope"); err == nil {
		t.Fatal("bad version accepted")
	}
}

func TestDiff(t *testing.T) {
	m := NewManager(newPackClone(t))
	ctx := context.Background()
	cur, _ := m.ReadFragment("org-baseline", "rules/security.md")
	same, err := m.Diff(ctx, "org-baseline", "rules/security.md", cur)
	if err != nil || same != "" {
		t.Fatalf("identical diff: %q err=%v", same, err)
	}
	d, err := m.Diff(ctx, "org-baseline", "rules/security.md", append(cur, []byte("- New rule.\n")...))
	if err != nil || !strings.Contains(d, "+- New rule.") {
		t.Fatalf("diff=%q err=%v", d, err)
	}
}
