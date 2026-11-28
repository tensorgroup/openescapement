package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/lockfile"
	"github.com/tensorgroup/openescapement/internal/render"
)

func fileArtifact(path, body string) Artifact {
	return Artifact{Path: path, Kind: KindFile, Body: body, Hash: esc.HashBytes(render.NormalizeEndings([]byte(body)))}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func saveLock(t *testing.T, root string, arts ...lockfile.LockArtifact) {
	t.Helper()
	if err := (&lockfile.Lock{Schema: 1, Artifacts: arts}).Save(root); err != nil {
		t.Fatal(err)
	}
}

func mustLock(t *testing.T, root string) *lockfile.Lock {
	t.Helper()
	l, err := lockfile.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// Occupied: no lock entry + unmanaged content on disk → skip; --force must not
// override; status reports Occupied.
func TestKindFileOccupiedSkipsAndForceDoesNotOverride(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "GOVERNANCE.md", "my own hand-written governance\n")
	a := fileArtifact("GOVERNANCE.md", "escapement content\n")
	plan := &PlanResult{Artifacts: []Artifact{a}}

	res, err := Apply(root, plan, false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Cause != SkipUnmanagedFileAtTarget {
		t.Fatalf("want one unmanaged-file skip, got %+v", res.Skipped)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "GOVERNANCE.md")); string(got) != "my own hand-written governance\n" {
		t.Errorf("occupied file was overwritten: %q", got)
	}
	res, err = Apply(root, plan, true)
	if err != nil {
		t.Fatalf("apply --force: %v", err)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Cause != SkipUnmanagedFileAtTarget {
		t.Fatalf("--force must not override occupied: %+v", res.Skipped)
	}
	if classify(root, a, mustLock(t, root)).State != Occupied {
		t.Errorf("status should report Occupied")
	}
}

// Byte-identical adoption: CRLF on disk, LF desired → normalized-equal → adopt.
func TestKindFileIdenticalAdopts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "GOVERNANCE.md", "escapement content\r\n")
	a := fileArtifact("GOVERNANCE.md", "escapement content\n")
	res, err := Apply(root, &PlanResult{Artifacts: []Artifact{a}}, false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Adopted) != 1 || res.Adopted[0] != "GOVERNANCE.md" {
		t.Fatalf("want adoption, got applied=%v adopted=%v skipped=%+v", res.Applied, res.Adopted, res.Skipped)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "GOVERNANCE.md")); string(got) != "escapement content\r\n" {
		t.Errorf("adoption must not rewrite disk: %q", got)
	}
	if classify(root, a, mustLock(t, root)).State != InSync {
		t.Error("adopted file must classify InSync")
	}
}

// Orphan removed when unedited; declined and carried forward when edited.
func TestKindFileOrphanRemovedAndEditedDeclined(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "GOVERNANCE.md", "escapement content\n")
	saveLock(t, root, lockfile.LockArtifact{Path: "GOVERNANCE.md", Kind: KindFile, Hash: esc.HashBytes([]byte("escapement content\n"))})
	if _, err := Apply(root, &PlanResult{}, false); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "GOVERNANCE.md")); !os.IsNotExist(err) {
		t.Errorf("unedited orphan file should be removed")
	}

	root2 := t.TempDir()
	writeFile(t, root2, "GOVERNANCE.md", "hand edited\n")
	saveLock(t, root2, lockfile.LockArtifact{Path: "GOVERNANCE.md", Kind: KindFile, Hash: esc.HashBytes([]byte("escapement content\n"))})
	res, err := Apply(root2, &PlanResult{}, false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Cause != SkipOrphanFileEdited {
		t.Fatalf("edited orphan must be declined: %+v", res.Skipped)
	}
	if _, err := os.Stat(filepath.Join(root2, "GOVERNANCE.md")); err != nil {
		t.Errorf("declined orphan must stay on disk: %v", err)
	}
	if mustLock(t, root2).Artifact("GOVERNANCE.md") == nil {
		t.Error("declined orphan lock entry must be carried forward")
	}
}

// A removed rule file leaves the .claude/rules directory empty; Apply cleans it.
func TestKindFileOrphanRuleRemovesEmptyDir(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".claude/rules/esc-acme-x.md", "body\n")
	saveLock(t, root, lockfile.LockArtifact{Path: ".claude/rules/esc-acme-x.md", Kind: KindFile, Hash: esc.HashBytes([]byte("body\n"))})
	if _, err := Apply(root, &PlanResult{}, false); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "rules")); !os.IsNotExist(err) {
		t.Errorf(".claude/rules should be removed once empty")
	}
}

// Ownership canonicality: every one of these lock paths must be refused before
// any read, leaving the file untouched.
func TestKindFileOrphanRefusesUncanonicalOrUnownedPaths(t *testing.T) {
	cases := []string{
		`.claude/rules/esc-x\..\..\VICTIM.md`, // backslash
		".claude/rules/./esc-x.md",            // ./ alias
		".claude/rules/team.md",               // unprefixed
		".claude/rules/sub/esc-x.md",          // nested
		"governance.md",                       // wrong case
		"docs/GOVERNANCE.md",                  // wrong dir
		"/GOVERNANCE.md",                      // leading slash
	}
	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "VICTIM.md", "not escapement's\n")
			saveLock(t, root, lockfile.LockArtifact{Path: p, Kind: KindFile, Hash: "sha256:whatever"})
			if _, err := Apply(root, &PlanResult{}, false); err == nil {
				t.Fatalf("apply must refuse to act on lock path %q", p)
			}
			if _, err := os.Stat(filepath.Join(root, "VICTIM.md")); err != nil {
				t.Errorf("nothing must be removed for %q: %v", p, err)
			}
		})
	}
}

func TestOwnedFilePathExactGovernanceOnly(t *testing.T) {
	if !ownedFilePath("GOVERNANCE.md") {
		t.Error("GOVERNANCE.md must be owned")
	}
	for _, p := range []string{"governance.md", "docs/GOVERNANCE.md", "./GOVERNANCE.md"} {
		if ownedFilePath(p) {
			t.Errorf("%q must not be owned", p)
		}
	}
	if !ownedRulePath(".claude/rules/esc-x.md") {
		t.Error("prefixed single-element rule path must be owned")
	}
	for _, p := range []string{".claude/rules/team.md", ".claude/rules/sub/esc-x.md", ".claude/rules/./esc-x.md", `.claude/rules/esc-x\y.md`, ".claude/rules/esc-"} {
		if ownedRulePath(p) {
			t.Errorf("%q must not be owned", p)
		}
	}
}

// identicalFileContent must refuse a symlink itself, so a symlink whose target
// hashes equal is never treated as an adoptable match.
func TestIdenticalFileContentRefusesSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "real.md")
	if err := os.WriteFile(target, []byte("escapement content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.md")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if identicalFileContent(link, esc.HashBytes([]byte("escapement content\n"))) {
		t.Error("a symlink must never be reported as identical content, even when its target matches")
	}
}

// prospectiveContent refuses a symlinked whole-file target at plan time, which
// is what makes plan/status/diff/sync exit 4 on it.
func TestKindFileSymlinkRefusedInPlan(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink("/etc/passwd", filepath.Join(root, "GOVERNANCE.md")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := prospectiveContent(root, fileArtifact("GOVERNANCE.md", "x\n")); err == nil {
		t.Fatal("prospectiveContent must refuse a symlinked KindFile target")
	}
}

func TestKindFileDesiredHashEqualsClassifyOnCRLF(t *testing.T) {
	root := t.TempDir()
	a := fileArtifact("GOVERNANCE.md", "line one\nline two\n")
	writeFile(t, root, "GOVERNANCE.md", "line one\r\nline two\r\n")
	saveLock(t, root, lockfile.LockArtifact{Path: "GOVERNANCE.md", Kind: KindFile, Hash: a.Hash})
	if classify(root, a, mustLock(t, root)).State != InSync {
		t.Error("CRLF checkout of a whole file must classify InSync")
	}
}
