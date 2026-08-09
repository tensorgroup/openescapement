package publisher

import (
	"context"
	"os/exec"
	"testing"
)

func TestNormalizeRemote(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "scp-like ssh shorthand keeps path case",
			raw:  "git@host.edu:Org/Repo.git",
			want: "host.edu/Org/Repo",
		},
		{
			name: "https with userinfo and trailing slash",
			raw:  "https://user:tok@host.edu/org/repo/",
			want: "host.edu/org/repo",
		},
		{
			name: "ssh scheme with trailing .git",
			raw:  "ssh://git@host.edu/org/repo.git",
			want: "host.edu/org/repo",
		},
		{
			name: "http scheme, no userinfo, no suffix",
			raw:  "http://host.edu/org/repo",
			want: "host.edu/org/repo",
		},
		{
			name: "empty input stays empty",
			raw:  "",
			want: "",
		},
		{
			name: "garbage without host/path shape passes through stripped",
			raw:  "  not-a-remote  ",
			want: "not-a-remote",
		},
		{
			name: "https userinfo with a literal @ in the password",
			raw:  "https://x:p@ss@host.edu/o/r.git",
			want: "host.edu/o/r",
		},
		{
			name: "scp-like shorthand with a literal @ in the password",
			raw:  "x:p@ss@host.edu:o/r.git",
			want: "host.edu/o/r",
		},
		{
			name: "an @ in the path is not userinfo",
			raw:  "https://host.edu/org/repo@2",
			want: "host.edu/org/repo@2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeRemote(tc.raw)
			if got != tc.want {
				t.Errorf("NormalizeRemote(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestRepoRemote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}

	t.Run("repo with a configured remote", func(t *testing.T) {
		root := t.TempDir()
		runGit(t, root, "init")
		runGit(t, root, "remote", "add", "origin", "https://host.edu/org/repo.git")

		got := RepoRemote(context.Background(), root)
		want := "https://host.edu/org/repo.git"
		if got != want {
			t.Errorf("RepoRemote() = %q, want %q", got, want)
		}
	})

	t.Run("repo with no remote returns empty", func(t *testing.T) {
		root := t.TempDir()
		runGit(t, root, "init")

		got := RepoRemote(context.Background(), root)
		if got != "" {
			t.Errorf("RepoRemote() = %q, want empty string", got)
		}
	})

	t.Run("not a git repo returns empty", func(t *testing.T) {
		root := t.TempDir()

		got := RepoRemote(context.Background(), root)
		if got != "" {
			t.Errorf("RepoRemote() = %q, want empty string", got)
		}
	})
}

// runGit runs a git command in dir, failing the test on error. Test helper
// only; production code has its own gitCommand in identity.go.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
