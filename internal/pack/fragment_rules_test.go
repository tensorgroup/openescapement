package pack

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tensorgroup/openescapement/internal/esc"
)

func loadOneFragment(t *testing.T, name, content string) (*Fragment, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rules", name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return loadFragment(dir, "acme", "rules/"+name, map[string]bool{})
}

func TestPathsParsedAndBodyStrippedOfFrontmatter(t *testing.T) {
	f, err := loadOneFragment(t, "api.md", "---\ntargets: [rules]\npaths:\n  - \"src/api/**\"\n---\n# API\n\nbody\n")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(f.Paths) != 1 || f.Paths[0] != "src/api/**" {
		t.Errorf("paths not parsed: %v", f.Paths)
	}
	if f.Body != "# API\n\nbody\n" {
		t.Errorf("body = %q", f.Body)
	}
}

func mustFail(t *testing.T, name, content string) {
	t.Helper()
	if _, err := loadOneFragment(t, name, content); !errors.Is(err, esc.ErrManifest) {
		t.Fatalf("want ErrManifest, got %v", err)
	}
}

func TestPathsWithoutRulesTargetIsError(t *testing.T) {
	mustFail(t, "api.md", "---\ntargets: [claude]\npaths:\n  - \"src/**\"\n---\nbody\n")
}
func TestEmptyPathsListIsError(t *testing.T) {
	mustFail(t, "api.md", "---\ntargets: [rules]\npaths: []\n---\nbody\n")
}
func TestNullPathsIsError(t *testing.T) {
	mustFail(t, "api.md", "---\ntargets: [rules]\npaths:\n---\nbody\n")
}
func TestTildeNullPathsIsError(t *testing.T) {
	mustFail(t, "api.md", "---\ntargets: [rules]\npaths: ~\n---\nbody\n")
}
func TestScalarPathsIsError(t *testing.T) {
	mustFail(t, "api.md", "---\ntargets: [rules]\npaths: foo\n---\nbody\n")
}
func TestNonStringPathsElemIsError(t *testing.T) {
	mustFail(t, "api.md", "---\ntargets: [rules]\npaths:\n  - 123\n---\nbody\n")
}
func TestEmptyPathsEntryIsError(t *testing.T) {
	mustFail(t, "api.md", "---\ntargets: [rules]\npaths:\n  - \"\"\n---\nbody\n")
}
func TestNewlinePathsEntryIsError(t *testing.T) {
	mustFail(t, "api.md", "---\ntargets: [rules]\npaths:\n  - \"a\\nb\"\n---\nbody\n")
}
func TestMappingPathsIsError(t *testing.T) {
	mustFail(t, "api.md", "---\ntargets: [rules]\npaths:\n  a: b\n---\nbody\n")
}
func TestBadRuleStemIsError(t *testing.T) {
	mustFail(t, "API_Handlers.md", "---\ntargets: [rules]\n---\nbody\n")
}
func TestEmptyRulesBodyIsError(t *testing.T) {
	mustFail(t, "api.md", "---\ntargets: [rules]\n---\n\n")
}
func TestWhitespaceOnlyRulesBodyIsError(t *testing.T) {
	mustFail(t, "api.md", "---\ntargets: [rules]\n---\n   \n\t\n")
}

// A merge key (<<:) is invisible to the yaml.Node walk parsePaths performs:
// the typed struct decode resolves a merged paths: or targets: field, but the
// node walk sees only the literal "<<" key, so a merged paths: would silently
// vanish and the fragment would render unscoped. Reject any merge key outright.
func TestMergeKeyMergingPathsIsRejected(t *testing.T) {
	mustFail(t, "api.md", "---\ntargets: [rules]\ndefaults: &d\n  paths: [\"src/api/**\"]\n<<: *d\n---\nbody\n")
}
func TestMergeKeyMergingTargetsIsRejected(t *testing.T) {
	mustFail(t, "api.md", "---\ndefaults: &d\n  targets: [claude]\n<<: *d\n---\nbody\n")
}

// A plain anchor/alias used as a *value* (not a merge key) is a different
// shape: the yaml.Node at that position is an AliasNode, which already fails
// the existing paths: must-be-a-sequence check. This must keep failing via
// that check, not change behavior.
func TestAliasValueForPathsStillRejectedBySequenceCheck(t *testing.T) {
	mustFail(t, "api.md", "---\ndefaults: &d\n  - \"src/api/**\"\ntargets: [rules]\npaths: *d\n---\nbody\n")
}

func TestRulesAndClaudeTogetherIsValid(t *testing.T) {
	f, err := loadOneFragment(t, "api.md", "---\ntargets: [rules, claude]\npaths:\n  - \"src/**\"\n---\nbody\n")
	if err != nil {
		t.Fatalf("rules+claude must be valid: %v", err)
	}
	if len(f.Targets) != 2 {
		t.Errorf("both targets should survive: %v", f.Targets)
	}
	if len(f.Paths) != 1 || f.Paths[0] != "src/**" {
		t.Errorf("paths not parsed alongside a second target: %v", f.Paths)
	}
}

func TestNoFrontmatterFragmentHasNoPaths(t *testing.T) {
	f, err := loadOneFragment(t, "api.md", "just a body\n")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if f.Paths != nil || len(f.Targets) != 0 {
		t.Errorf("no-frontmatter fragment must have no targets/paths: %+v", f)
	}
}

func TestCRLFFrontmatterSplitAfterNormalize(t *testing.T) {
	f, err := loadOneFragment(t, "api.md", "---\r\ntargets: [rules]\r\n---\r\nbody line\r\n")
	if err != nil {
		t.Fatalf("CRLF frontmatter must split after normalize: %v", err)
	}
	if len(f.Targets) != 1 || f.Targets[0] != "rules" || f.Body != "body line\n" {
		t.Errorf("targets/body lost on CRLF fragment: %v / %q", f.Targets, f.Body)
	}
}

func TestInPackRuleStemCollisionIsError(t *testing.T) {
	dir := t.TempDir()
	man := "schema: 1\nname: acme\nversion: 1.0.0\nrules:\n  - rules/api.md\n  - sub/api.md\n"
	if err := os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(man), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"rules/api.md", "sub/api.md"} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("---\ntargets: [rules]\n---\nx\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Load(dir); !errors.Is(err, esc.ErrManifest) {
		t.Fatalf("two rules fragments with the same stem must be ErrManifest, got %v", err)
	}
}

func TestNonRulesStemCollisionIsAccepted(t *testing.T) {
	dir := t.TempDir()
	man := "schema: 1\nname: acme\nversion: 1.0.0\nrules:\n  - rules/api.md\n  - sub/api.md\n"
	if err := os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(man), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"rules/api.md", "sub/api.md"} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("---\ntargets: [claude]\n---\nx\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Load(dir); err != nil {
		t.Fatalf("two same-stem fragments that don't name the rules target must not collide: %v", err)
	}
}
