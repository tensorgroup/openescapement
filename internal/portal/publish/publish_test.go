package publish

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// containsArg reports whether want appears as an exact element of args.
func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

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

// TestPublishPostAddFailureRestores forces the commit step to fail after
// `git add -A` has already staged the fragment write and manifest version
// bump. A restore that only does `git checkout -- .` (restoring tracked
// files from the index, not the index itself) would leave those staged
// changes in place; `git reset --hard HEAD` must undo them too.
func TestPublishPostAddFailureRestores(t *testing.T) {
	m := NewManager(newPackClone(t))
	ctx := context.Background()
	dir := filepath.Join(m.Dir, "org-baseline")

	real := gitRun
	defer func() { gitRun = real }()
	gitRun = func(ctx context.Context, d string, args ...string) (string, error) {
		if d == dir && containsArg(args, "commit") {
			return "", fmt.Errorf("simulated commit failure")
		}
		return real(ctx, d, args...)
	}

	good := []byte("---\ntargets: [claude]\n---\nok\n")
	if err := m.Publish(ctx, "org-baseline", "rules/security.md", good, "1.3.0"); err == nil {
		t.Fatal("expected simulated commit failure to propagate")
	}

	if out := git(t, dir, "status", "--porcelain"); out != "" {
		t.Fatalf("dirty worktree after failed commit: %s", out)
	}
	p, err := m.Get(ctx, "org-baseline")
	if err != nil || p.Version != "1.2.0" || len(p.Tags) != 1 {
		t.Fatalf("not restored: %+v err=%v", p, err)
	}
}

// TestPublishCommitSucceedsTagFailsRestores simulates a race where the tag
// name is taken by something else between the step-1 guard and the step-4
// tag creation: the commit lands, but the tag step then fails. Publish must
// unwind the commit (git reset --hard to the pre-commit SHA), not merely
// the worktree — otherwise the clone is left committed-but-untagged, i.e.
// half-published.
func TestPublishCommitSucceedsTagFailsRestores(t *testing.T) {
	m := NewManager(newPackClone(t))
	ctx := context.Background()
	dir := filepath.Join(m.Dir, "org-baseline")
	tagName := "v1.3.0"

	real := gitRun
	defer func() { gitRun = real }()
	gitRun = func(ctx context.Context, d string, args ...string) (string, error) {
		if d == dir {
			for i, a := range args {
				if a == "tag" && i+1 < len(args) && args[i+1] == "-a" {
					// Simulate a concurrent actor winning the race for
					// this tag name after our step-1 guard already
					// checked it was free.
					raceCmd := exec.CommandContext(ctx, "git", "tag", tagName)
					raceCmd.Dir = d
					if out, rerr := raceCmd.CombinedOutput(); rerr != nil {
						t.Fatalf("seeding race tag: %v\n%s", rerr, out)
					}
					break
				}
			}
		}
		return real(ctx, d, args...)
	}

	good := []byte("---\ntargets: [claude]\n---\nok\n")
	if err := m.Publish(ctx, "org-baseline", "rules/security.md", good, "1.3.0"); err == nil {
		t.Fatal("expected simulated tag race to propagate an error")
	}

	if out := git(t, dir, "status", "--porcelain"); out != "" {
		t.Fatalf("dirty worktree after failed tag: %s", out)
	}
	manifest, _ := os.ReadFile(filepath.Join(dir, "pack.yaml"))
	if !strings.Contains(string(manifest), "version: 1.2.0") {
		t.Fatalf("manifest version not restored:\n%s", manifest)
	}
	if out := strings.TrimSpace(git(t, dir, "log", "-1", "--format=%s")); out != "init" {
		t.Fatalf("commit not unwound: HEAD is %q, want the pre-publish \"init\" commit", out)
	}
}

// TestPublishRejectsGitDir guards against a fragment path that reaches into
// the clone's own .git directory: writing e.g. .git/config could inject
// arbitrary git config (such as a core.fsmonitor hook) that survives
// checkout/clean and executes on the next git invocation against the clone.
func TestPublishRejectsGitDir(t *testing.T) {
	m := NewManager(newPackClone(t))
	ctx := context.Background()
	good := []byte("---\ntargets: [claude]\n---\nok\n")
	if err := m.Publish(ctx, "org-baseline", ".git/config", good, "1.3.0"); err == nil {
		t.Fatal(".git write accepted")
	}
	if _, err := m.ReadFragment("org-baseline", ".git/config"); err == nil {
		t.Fatal(".git read accepted")
	}
}

// TestPublishRejectsSymlinkedParentEscape covers a fragment path whose leaf
// doesn't exist yet but whose parent directory is a symlink pointing
// outside the clone. EvalSymlinks on the (nonexistent) full path alone
// would error and skip the escape check entirely; safeFragPath must walk up
// to the nearest existing ancestor instead.
func TestPublishRejectsSymlinkedParentEscape(t *testing.T) {
	m := NewManager(newPackClone(t))
	ctx := context.Background()
	dir := filepath.Join(m.Dir, "org-baseline")
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "sneaky")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	good := []byte("---\ntargets: [claude]\n---\nok\n")
	if err := m.Publish(ctx, "org-baseline", "sneaky/newfile.md", good, "1.3.0"); err == nil {
		t.Fatal("symlinked-parent escape accepted")
	}
	if _, err := os.Stat(filepath.Join(outside, "newfile.md")); err == nil {
		t.Fatal("fragment was written outside the clone via a symlinked parent directory")
	}
}

// TestPublishConcurrentSerialized publishes two different versions of the
// same pack concurrently. Without per-clone locking, the step-1 tag-exists
// guard and step-4 tag creation race, and one call's `git add -A` can sweep
// up the other's in-progress write; both should succeed cleanly when
// serialized.
func TestPublishConcurrentSerialized(t *testing.T) {
	m := NewManager(newPackClone(t))
	ctx := context.Background()
	good1 := []byte("---\ntargets: [claude]\n---\nv1.3\n")
	good2 := []byte("---\ntargets: [claude]\n---\nv1.4\n")

	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs[0] = m.Publish(ctx, "org-baseline", "rules/security.md", good1, "1.3.0")
	}()
	go func() {
		defer wg.Done()
		errs[1] = m.Publish(ctx, "org-baseline", "rules/security.md", good2, "1.4.0")
	}()
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: %v", i, err)
		}
	}

	dir := filepath.Join(m.Dir, "org-baseline")
	if out := git(t, dir, "status", "--porcelain"); out != "" {
		t.Fatalf("dirty worktree after concurrent publish: %s", out)
	}
	p, err := m.Get(ctx, "org-baseline")
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tg := range p.Tags {
		names[tg.Name] = true
	}
	if !names["v1.3.0"] || !names["v1.4.0"] {
		t.Fatalf("expected both v1.3.0 and v1.4.0 tags, got %+v", p.Tags)
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
