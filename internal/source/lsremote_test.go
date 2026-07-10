package source

import (
	"context"
	"testing"
)

func TestLsRemoteTags(t *testing.T) {
	repo := initGitRepo(t, packFiles(), "v1.0.0")
	url := "file://" + repo
	tags, err := LsRemoteTags(context.Background(), url)
	if err != nil {
		t.Fatalf("LsRemoteTags: %v", err)
	}
	h, ok := tags["v1.0.0"]
	if !ok || len(h) != 40 {
		t.Fatalf("v1.0.0 not resolved: %v", tags)
	}
	// Option-shaped URL is rejected before reaching git.
	if _, err := LsRemoteTags(context.Background(), "--upload-pack=evil"); err == nil {
		t.Error("option-shaped url: want error")
	}
}

func TestLsRemoteHash(t *testing.T) {
	repo := initGitRepo(t, packFiles(), "v1.0.0")
	url := "file://" + repo
	h, err := LsRemoteHash(context.Background(), url, "main")
	if err != nil || len(h) != 40 {
		t.Fatalf("LsRemoteHash(main) = %q, %v", h, err)
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
