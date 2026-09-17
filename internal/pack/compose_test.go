package pack

import (
	"errors"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/esc"
)

func TestComposeFragmentsMergesFrontmatterAndBodies(t *testing.T) {
	a := []byte("---\ntargets: [claude, agents]\n---\n# A\n\nBody A.\n")
	b := []byte("---\ntargets: [claude, agents]\n---\n# B\n\nBody B.\n")
	got, err := ComposeFragments([][]byte{a, b})
	if err != nil {
		t.Fatal(err)
	}
	want := "---\ntargets: [claude, agents]\n---\n\n# A\n\nBody A.\n\n# B\n\nBody B.\n"
	if string(got) != want {
		t.Fatalf("composed:\n%q\nwant:\n%q", got, want)
	}
}

func TestComposeFragmentsUnionsDistinctTargets(t *testing.T) {
	a := []byte("---\ntargets: [claude]\n---\nA\n")
	b := []byte("---\ntargets: [agents]\n---\nB\n")
	got, err := ComposeFragments([][]byte{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(got), "---\ntargets: [claude, agents]\n---\n") {
		t.Fatalf("composed frontmatter wrong: %q", got)
	}
}

func TestComposeFragmentsSinglePartVerbatim(t *testing.T) {
	a := []byte("---\ntargets: [claude]\n---\nA body\n")
	got, err := ComposeFragments([][]byte{a})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(a) {
		t.Fatal("single part must pass through byte-identical")
	}
}

func TestComposeFragmentsNoFrontmatterPart(t *testing.T) {
	a := []byte("---\ntargets: [claude]\n---\nA\n")
	b := []byte("Plain body\n")
	got, err := ComposeFragments([][]byte{a, b})
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if !strings.HasPrefix(s, "---\ntargets: [claude]\n---\n") || !strings.Contains(s, "Plain body") {
		t.Fatalf("composed: %q", s)
	}
}

func TestComposeFragmentsBadFrontmatterErrors(t *testing.T) {
	bad := []byte("---\ntargets: [unclosed\n---\nX\n")
	if _, err := ComposeFragments([][]byte{[]byte("A\n"), bad}); err == nil {
		t.Fatal("expected error for unparseable frontmatter")
	}
}

func TestComposeFragmentsEmptyErrors(t *testing.T) {
	if _, err := ComposeFragments(nil); err == nil {
		t.Fatal("expected error for empty parts")
	}
}

func TestComposeFragmentsRejectsPaths(t *testing.T) {
	parts := [][]byte{
		[]byte("---\ntargets: [rules]\npaths:\n  - \"src/**\"\n---\nbody a\n"),
		[]byte("---\ntargets: [rules]\n---\nbody b\n"),
	}
	if _, err := ComposeFragments(parts); !errors.Is(err, esc.ErrManifest) {
		t.Fatalf("compose must reject a path-scoped part with ErrManifest, got %v", err)
	}
}

func TestComposeFragmentsRejectsSinglePathScopedPart(t *testing.T) {
	// The portal's resolveAdoptSelection can pass a single part, which used to
	// return byte-identical before any parse.
	one := [][]byte{[]byte("---\ntargets: [rules]\npaths:\n  - \"src/**\"\n---\nbody\n")}
	if _, err := ComposeFragments(one); !errors.Is(err, esc.ErrManifest) {
		t.Fatalf("single path-scoped part must be rejected, got %v", err)
	}
}
