package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/render"
)

// TestClassifyAlterationInvariant guards the invariant Apply's handEdited
// gate (apply.go) depends on for its fail-closed behavior: classify sets
// Finding.Alteration if and only if the Altered state came from a genuine
// hash mismatch (a human edited content escapement previously wrote), never
// from an error encountered while trying to classify the artifact (corrupt
// block markers, an unreadable dir, unparsable JSON). If a future Altered
// branch in classify (internal/engine/status.go) ever sets State: Altered
// without also setting Alteration on a genuine mismatch, or sets it on an
// error branch, Apply's skip gate silently drifts: either a real hand-edit
// stops being declined (data loss returns) or a classification error gets
// swallowed as a polite decline instead of failing closed (the exact
// regression the symlink test in internal/cli/skilldir_test.go guards from
// the write side).
//
// Covers all four kinds' genuine-mismatch branches (Alteration must be set)
// and, for the two kinds where it's easy to construct without touching the
// filesystem's permission bits, an error branch too (Alteration must be
// nil). KindFile has no error branch to cover (an os.ReadFile failure there
// reports Missing, not Altered) and KindDir's other error branch
// (unmanagedDirFiles) needs a filesystem-walk failure that isn't worth
// contriving here — the DirHashOf error branch below already proves the
// same point for that kind (classify's error branches never set
// Alteration).
func TestClassifyAlterationInvariant(t *testing.T) {
	t.Run("block genuine mismatch sets Alteration", func(t *testing.T) {
		root := t.TempDir()
		content, err := render.Splice(nil, "original body\n", render.BlockMeta{Packs: []string{"acme@1.0.0"}})
		if err != nil {
			t.Fatal(err)
		}
		// Simulate a hand-edit: mutate the body directly on disk, the way a
		// human would, rather than re-rendering through Splice.
		edited := strings.Replace(string(content), "original body", "edited body", 1)
		if edited == string(content) {
			t.Fatal("precondition: body text not found in rendered block")
		}
		path := filepath.Join(root, "AGENTS.md")
		if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
			t.Fatal(err)
		}
		a := Artifact{Path: "AGENTS.md", Kind: KindBlock, Hash: render.BodyHash("original body\n")}
		f := classify(root, a, nil)
		if f.State != Altered {
			t.Fatalf("state = %q, want altered", f.State)
		}
		if f.Alteration == nil {
			t.Error("genuine hash mismatch must set Alteration")
		}
	})

	t.Run("block corrupt marker (error) leaves Alteration nil", func(t *testing.T) {
		root := t.TempDir()
		// A begin marker with no matching end marker: render.Extract fails
		// with "begin without end marker", classify's error branch for
		// KindBlock (status.go).
		broken := []byte("<!-- escapement:begin packs=acme@1.0.0 hash=sha256:00 -->\nbody\n")
		path := filepath.Join(root, "AGENTS.md")
		if err := os.WriteFile(path, broken, 0o644); err != nil {
			t.Fatal(err)
		}
		a := Artifact{Path: "AGENTS.md", Kind: KindBlock, Hash: "sha256:doesnotmatter"}
		f := classify(root, a, nil)
		if f.State != Altered {
			t.Fatalf("state = %q, want altered", f.State)
		}
		if f.Alteration != nil {
			t.Errorf("a classification error must not set Alteration, got %+v", f.Alteration)
		}
	})

	t.Run("file genuine mismatch sets Alteration", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "GOVERNANCE.md")
		if err := os.WriteFile(path, []byte("on disk\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		a := Artifact{Path: "GOVERNANCE.md", Kind: KindFile, Hash: esc.HashBytes([]byte("expected\n"))}
		f := classify(root, a, nil)
		if f.State != Altered {
			t.Fatalf("state = %q, want altered", f.State)
		}
		if f.Alteration == nil {
			t.Error("genuine hash mismatch must set Alteration")
		}
	})

	t.Run("dir genuine mismatch sets Alteration", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, ".claude", "skills", "esc-x")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("on disk\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		a := Artifact{
			Path: ".claude/skills/esc-x", Kind: KindDir,
			Files: []string{"SKILL.md"}, Hash: "sha256:doesnotmatch",
		}
		f := classify(root, a, nil)
		if f.State != Altered {
			t.Fatalf("state = %q, want altered", f.State)
		}
		if f.Alteration == nil {
			t.Error("genuine hash mismatch must set Alteration")
		}
	})

	t.Run("dir unreadable file (error) leaves Alteration nil", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, ".claude", "skills", "esc-x")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		// a.Files names a file the pack expects but the dir doesn't have:
		// pack.DirHashOf fails to read it, classify's error branch.
		a := Artifact{
			Path: ".claude/skills/esc-x", Kind: KindDir,
			Files: []string{"missing.md"}, Hash: "sha256:doesnotmatter",
		}
		f := classify(root, a, nil)
		if f.State != Altered {
			t.Fatalf("state = %q, want altered", f.State)
		}
		if f.Alteration != nil {
			t.Errorf("a classification error must not set Alteration, got %+v", f.Alteration)
		}
	})

	t.Run("json-keys genuine mismatch sets Alteration", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, ".mcp.json")
		if err := os.WriteFile(path, []byte(`{"mcpServers": {}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		a := Artifact{Path: ".mcp.json", Kind: KindJSONKeys, Keys: []string{"acme-paved-path"}, Hash: "sha256:doesnotmatch"}
		f := classify(root, a, nil)
		if f.State != Altered {
			t.Fatalf("state = %q, want altered", f.State)
		}
		if f.Alteration == nil {
			t.Error("genuine hash mismatch must set Alteration")
		}
	})

	t.Run("json-keys unparsable JSON (error) leaves Alteration nil", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, ".mcp.json")
		if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
			t.Fatal(err)
		}
		a := Artifact{Path: ".mcp.json", Kind: KindJSONKeys, Keys: []string{"acme-paved-path"}, Hash: "sha256:doesnotmatter"}
		f := classify(root, a, nil)
		if f.State != Altered {
			t.Fatalf("state = %q, want altered", f.State)
		}
		if f.Alteration != nil {
			t.Errorf("a classification error must not set Alteration, got %+v", f.Alteration)
		}
	})
}
