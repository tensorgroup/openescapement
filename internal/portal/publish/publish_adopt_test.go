package publish

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/esc"
)

const testStarter = "---\ntargets: [claude, agents]\n---\n# Model x\n\n- Route work to x.\n"

func TestAddFragmentAdoptsNewFragment(t *testing.T) {
	mgr := NewManager(newPackClone(t))
	ctx := context.Background()
	dest := "rules/model-claude-sonnet-5.md"
	if err := mgr.AddFragment(ctx, "org-baseline", dest, []byte(testStarter), "1.3.0"); err != nil {
		t.Fatalf("AddFragment: %v", err)
	}
	info, err := mgr.Get(ctx, "org-baseline")
	if err != nil {
		t.Fatal(err)
	}
	if info.Version != "1.3.0" {
		t.Fatalf("version = %s, want 1.3.0", info.Version)
	}
	found := false
	for _, f := range info.Fragments {
		if f == dest {
			found = true
		}
	}
	if !found {
		t.Fatalf("adopted fragment not in manifest: %v", info.Fragments)
	}
	if len(info.Tags) == 0 || info.Tags[0].Name != "v1.3.0" {
		t.Fatalf("tags = %v, want newest v1.3.0", info.Tags)
	}
	got, err := mgr.ReadFragment("org-baseline", dest)
	if err != nil || string(got) != testStarter {
		t.Fatalf("ReadFragment = %q, %v", got, err)
	}
}

func TestAddFragmentCollisionRefuses(t *testing.T) {
	mgr := NewManager(newPackClone(t))
	ctx := context.Background()
	dest := "rules/model-claude-sonnet-5.md"
	if err := mgr.AddFragment(ctx, "org-baseline", dest, []byte(testStarter), "1.3.0"); err != nil {
		t.Fatal(err)
	}
	// Second adopt of the same path (new version so it clears the tag guard).
	err := mgr.AddFragment(ctx, "org-baseline", dest, []byte(testStarter), "1.4.0")
	if !errors.Is(err, ErrFragmentExists) {
		t.Fatalf("second adopt err = %v, want ErrFragmentExists", err)
	}
}

func TestAddFragmentInvalidContentRestores(t *testing.T) {
	mgr := NewManager(newPackClone(t))
	ctx := context.Background()
	bad := "---\ntargets: [nonsense-target]\n---\nbody\n"
	err := mgr.AddFragment(ctx, "org-baseline", "rules/model-bad.md", []byte(bad), "1.3.0")
	if !errors.Is(err, esc.ErrManifest) {
		t.Fatalf("err = %v, want esc.ErrManifest", err)
	}
	info, err := mgr.Get(ctx, "org-baseline")
	if err != nil {
		t.Fatal(err)
	}
	if info.Version != "1.2.0" {
		t.Fatalf("version = %s, want restored 1.2.0", info.Version)
	}
	if _, statErr := os.Stat(filepath.Join(info.Dir, "rules/model-bad.md")); statErr == nil {
		t.Fatal("failed adopt left the fragment on disk")
	}
	raw, _ := os.ReadFile(filepath.Join(info.Dir, "pack.yaml"))
	if strings.Contains(string(raw), "model-bad.md") {
		t.Fatal("failed adopt left the rule in the manifest")
	}
}
