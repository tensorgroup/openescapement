package source

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

// gitOut runs a git command in dir and returns its trimmed output.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// lsRemoteRepo builds a repo with an annotated tag (v1.0.0) and a lightweight
// tag (v1.1.0) on the same commit, returning its file:// URL and commit SHA.
func lsRemoteRepo(t *testing.T) (url, commit string) {
	t.Helper()
	repo := initGitRepo(t, packFiles(), "v1.0.0") // annotated (git tag -a)
	gitOut(t, repo, "tag", "v1.1.0")              // lightweight
	return "file://" + repo, gitOut(t, repo, "rev-parse", "HEAD")
}

func TestLsRemoteTags(t *testing.T) {
	url, commit := lsRemoteRepo(t)
	tags, err := LsRemoteTags(context.Background(), url)
	if err != nil {
		t.Fatalf("LsRemoteTags: %v", err)
	}
	// Annotated tag must resolve to the peeled commit, not the tag object.
	if h := tags["v1.0.0"]; h != commit {
		t.Errorf("annotated v1.0.0 = %q, want commit %q", h, commit)
	}
	// Lightweight tag has no ^{} line and points at the commit directly.
	if h := tags["v1.1.0"]; h != commit {
		t.Errorf("lightweight v1.1.0 = %q, want commit %q", h, commit)
	}
	// Option-shaped URL is rejected before reaching git.
	if _, err := LsRemoteTags(context.Background(), "--upload-pack=evil"); err == nil {
		t.Error("option-shaped url: want error")
	}
}

func TestLsRemoteHash(t *testing.T) {
	url, commit := lsRemoteRepo(t)
	h, err := LsRemoteHash(context.Background(), url, "main")
	if err != nil || h != commit {
		t.Fatalf("LsRemoteHash(main) = %q, %v; want %q", h, err, commit)
	}
	// Annotated tag must resolve to the peeled commit so it agrees with
	// LsRemoteTags (regression: single-pattern ls-remote returns the tag
	// object hash and never the ^{} line).
	h, err = LsRemoteHash(context.Background(), url, "v1.0.0")
	if err != nil || h != commit {
		t.Fatalf("LsRemoteHash(v1.0.0) = %q, %v; want peeled commit %q", h, err, commit)
	}
	// Lightweight tag behaves as before.
	h, err = LsRemoteHash(context.Background(), url, "v1.1.0")
	if err != nil || h != commit {
		t.Fatalf("LsRemoteHash(v1.1.0) = %q, %v; want %q", h, err, commit)
	}
	// Absent ref → empty, no error.
	h, err = LsRemoteHash(context.Background(), url, "nope")
	if err != nil || h != "" {
		t.Fatalf("absent ref = %q, %v; want '', nil", h, err)
	}
	// Injection-shaped ref is rejected.
	if _, err := LsRemoteHash(context.Background(), url, "--exec=evil"); err == nil {
		t.Error("bad ref: want error")
	}
}
