package source

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/esc"
)

func TestParseSource(t *testing.T) {
	cases := []struct {
		in          string
		local       bool
		url, subdir string
	}{
		{"github.com/acme/policy-packs//org", false, "https://github.com/acme/policy-packs", "org"},
		{"github.com/acme/policy-packs", false, "https://github.com/acme/policy-packs", ""},
		{"git@github.com:acme/packs.git//org", false, "git@github.com:acme/packs.git", "org"},
		{"https://gitlab.com/a/b.git//x/y", false, "https://gitlab.com/a/b.git", "x/y"},
		{"file:///tmp/packs//org", false, "file:///tmp/packs", "org"},
		{"./local-packs/team", true, "./local-packs/team", ""},
		{"../packs/org", true, "../packs/org", ""},
		{"/abs/path/pack", true, "/abs/path/pack", ""},
	}
	for _, tc := range cases {
		got, err := ParseSource(tc.in)
		if err != nil {
			t.Errorf("%s: %v", tc.in, err)
			continue
		}
		if got.Local != tc.local || got.URL != tc.url || got.Subdir != tc.subdir {
			t.Errorf("%s: got %+v", tc.in, got)
		}
	}
}

// initGitRepo creates a git repo with the given files committed and tagged.
func initGitRepo(t *testing.T, files map[string]string, tag string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
		{"config", "commit.gpgsign", "false"},
		{"config", "tag.gpgsign", "false"},
		{"add", "-A"},
		{"commit", "-q", "-m", "init"},
		{"tag", "-a", tag, "-m", tag},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func packFiles() map[string]string {
	return map[string]string{
		"org/pack.yaml":  "schema: 1\nname: acme-org\nversion: 1.0.0\nrules: [rules/a.md]\n",
		"org/rules/a.md": "## A\nrule a\n",
	}
}

func TestFetchLocalGitRepo(t *testing.T) {
	repo := initGitRepo(t, packFiles(), "v1.0.0")
	cache := t.TempDir()
	res, err := Fetch(context.Background(), config.PackRef{
		Source: "file://" + repo + "//org", Ref: "v1.0.0",
	}, cache, t.TempDir())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if res.Commit == "" || len(res.Commit) != 40 {
		t.Errorf("commit SHA: %q", res.Commit)
	}
	if _, err := os.Stat(filepath.Join(res.Dir, "pack.yaml")); err != nil {
		t.Errorf("pack.yaml not at res.Dir: %v", err)
	}
	// Second fetch (cache warm) works and agrees.
	res2, err := Fetch(context.Background(), config.PackRef{
		Source: "file://" + repo + "//org", Ref: "v1.0.0",
	}, cache, t.TempDir())
	if err != nil {
		t.Fatalf("second Fetch: %v", err)
	}
	if res2.Commit != res.Commit {
		t.Errorf("commit changed on warm fetch: %s vs %s", res2.Commit, res.Commit)
	}
}

func TestFetchRefMismatch(t *testing.T) {
	repo := initGitRepo(t, packFiles(), "v1.0.0")
	_, err := Fetch(context.Background(), config.PackRef{
		Source: "file://" + repo + "//org", Ref: "v9.9.9",
	}, t.TempDir(), t.TempDir())
	if !errors.Is(err, esc.ErrFetch) {
		t.Fatalf("want ErrFetch, got %v", err)
	}
}

func TestFetchLocalPlainDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "team"), 0o755)
	os.WriteFile(filepath.Join(dir, "team", "pack.yaml"), []byte("x"), 0o644)
	res, err := Fetch(context.Background(), config.PackRef{Source: "./packs/team"}, t.TempDir(), dir)
	// relative to repoRoot: dir/packs/team doesn't exist -> error
	if err == nil {
		t.Fatal("nonexistent local path should error")
	}
	res, err = Fetch(context.Background(), config.PackRef{Source: filepath.Join(dir, "team")}, t.TempDir(), dir)
	if err != nil {
		t.Fatalf("abs local: %v", err)
	}
	if res.Commit != "" {
		t.Errorf("plain dir should have empty commit, got %q", res.Commit)
	}
	if res.Dir != filepath.Join(dir, "team") {
		t.Errorf("dir: %q", res.Dir)
	}
}
