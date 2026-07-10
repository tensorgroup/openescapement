package updatecheck

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/lockfile"
)

func TestSemverOrdering(t *testing.T) {
	less := func(a, b string) bool {
		sa, _ := parseSemver(a)
		sb, _ := parseSemver(b)
		return semverLess(sa, sb)
	}
	if !less("v1.0.0", "v1.0.1") || !less("v1.9.0", "v2.0.0") {
		t.Error("core ordering wrong")
	}
	if !less("v1.0.0-rc1", "v1.0.0") {
		t.Error("prerelease should sort below release")
	}
	if less("v1.0.0", "v1.0.0-rc1") {
		t.Error("release should not sort below its prerelease")
	}
	if _, ok := parseSemver("main"); ok {
		t.Error("non-semver parsed as semver")
	}
	if _, ok := parseSemver("v1.0.0"); !ok {
		t.Error("semver rejected")
	}
	tags := map[string]string{"v1.0.0": "a", "v2.0.0": "b", "v2.1.0-rc1": "c"}
	got, ok := maxStableSemver(tags)
	if !ok || got != "v2.0.0" {
		t.Errorf("maxStableSemver = %q,%v; want v2.0.0 (prerelease excluded)", got, ok)
	}
}

// TestMaxStableSemverTieBreakDeterministic pins the winner when two tag names
// parse to the same semver (a "v"-prefixed name and a bare one): map
// iteration order must never decide the result.
func TestMaxStableSemverTieBreakDeterministic(t *testing.T) {
	tags := map[string]string{"v2.0.0": "a", "2.0.0": "b"}
	for i := 0; i < 20; i++ {
		got, ok := maxStableSemver(tags)
		if !ok || got != "v2.0.0" {
			t.Fatalf("maxStableSemver tie-break = %q,%v; want v2.0.0 (v-prefix preferred), iteration %d", got, ok, i)
		}
	}
}

// gitCommit stages all files and commits, returning nothing.
func gitCommit(t *testing.T, dir, msg string) {
	t.Helper()
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", msg}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// newRepo builds a git repo tagged v1.0.0 on branch main.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("one\n"), 0o644)
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "t@e.com"},
		{"config", "user.name", "T"},
		{"config", "commit.gpgsign", "false"},
		{"config", "tag.gpgsign", "false"},
		{"add", "-A"}, {"commit", "-q", "-m", "one"},
		{"tag", "-a", "v1.0.0", "-m", "v1"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func tagRepo(t *testing.T, dir, tag string) {
	t.Helper()
	cmd := exec.Command("git", "tag", "-a", tag, "-m", tag)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git tag %s: %v\n%s", tag, err, out)
	}
}

func TestCheckTagPinNewerTag(t *testing.T) {
	repo := newRepo(t)
	cfg := &config.Config{Schema: 1, Packs: []config.PackRef{
		{Source: "file://" + repo, Ref: "v1.0.0", Trust: "unsigned"},
	}}
	// No newer tag yet.
	st, err := check(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(st) != 1 || st[0].Kind != "tag" || st[0].Updates {
		t.Fatalf("want no updates, got %+v", st)
	}
	// Publish v2.0.0.
	os.WriteFile(filepath.Join(repo, "f.txt"), []byte("two\n"), 0o644)
	gitCommit(t, repo, "two")
	tagRepo(t, repo, "v2.0.0")
	st, _ = check(context.Background(), cfg, nil)
	if !st[0].Updates || st[0].Latest != "v2.0.0" {
		t.Fatalf("want updates to v2.0.0, got %+v", st[0])
	}
}

func TestCheckBranchPinMoved(t *testing.T) {
	repo := newRepo(t)
	head := func() string {
		cmd := exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = repo
		out, _ := cmd.Output()
		return string(out[:40])
	}
	cfg := &config.Config{Schema: 1, Packs: []config.PackRef{
		{Source: "file://" + repo, Ref: "main", Trust: "unsigned"},
	}}
	lock := &lockfile.Lock{Schema: 1, Packs: []lockfile.LockPack{
		{Source: "file://" + repo, Ref: "main", Commit: head()},
	}}
	// Locked at current tip → no updates.
	st, err := check(context.Background(), cfg, lock)
	if err != nil || st[0].Kind != "branch" || st[0].Updates {
		t.Fatalf("want no updates on branch, got %+v err %v", st, err)
	}
	// Advance main → updates.
	os.WriteFile(filepath.Join(repo, "f.txt"), []byte("two\n"), 0o644)
	gitCommit(t, repo, "two")
	st, _ = check(context.Background(), cfg, lock)
	if !st[0].Updates {
		t.Fatalf("want updates after branch moved, got %+v", st[0])
	}
}

func TestCheckSkipsLocalDirs(t *testing.T) {
	cfg := &config.Config{Schema: 1, Packs: []config.PackRef{
		{Source: "/abs/local/pack", Ref: "", Trust: "unsigned"},
	}}
	st, err := check(context.Background(), cfg, nil)
	if err != nil || len(st) != 0 {
		t.Fatalf("local dir should be skipped, got %+v err %v", st, err)
	}
}
