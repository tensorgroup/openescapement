package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/render"
)

// TestClassifyEndingsNormalization guards the same ruling render.BodyHash
// already carries for KindBlock (see block.go): line endings are git's
// presentation, not policy content. A checkout where core.autocrlf converted
// managed content to CRLF must classify in-sync, on both the managed-block
// axis and the whole-file (KindFile, e.g. GOVERNANCE.md) axis — and a real
// content edit must still be caught even when it arrives wrapped in CRLF.
//
// classify is driven directly, constructing content via render calls and
// []byte("\r\n") conversion, never a hand-typed hash — the same fixture
// style classify_alteration_test.go uses.
func TestClassifyEndingsNormalization(t *testing.T) {
	t.Run("block converted wholesale to CRLF classifies in-sync", func(t *testing.T) {
		root := t.TempDir()
		body := "policy line\n"
		lf, err := render.Splice(nil, body, render.BlockMeta{Packs: []string{"acme@1.0.0"}})
		if err != nil {
			t.Fatal(err)
		}
		crlf := bytes.ReplaceAll(lf, []byte("\n"), []byte("\r\n"))
		path := filepath.Join(root, "CLAUDE.md")
		if err := os.WriteFile(path, crlf, 0o644); err != nil {
			t.Fatal(err)
		}
		a := Artifact{Path: "CLAUDE.md", Kind: KindBlock, Hash: render.BodyHash(body)}
		f := classify(root, a, nil)
		if f.State != InSync {
			t.Fatalf("state = %q, want in-sync (this is pre-existing KindBlock behavior, not this task's fix)", f.State)
		}
	})

	t.Run("GOVERNANCE.md converted wholesale to CRLF classifies in-sync", func(t *testing.T) {
		root := t.TempDir()
		content := "# Governance\n\nRendered policy text.\n"
		crlf := bytes.ReplaceAll([]byte(content), []byte("\n"), []byte("\r\n"))
		path := filepath.Join(root, "GOVERNANCE.md")
		if err := os.WriteFile(path, crlf, 0o644); err != nil {
			t.Fatal(err)
		}
		// The expected hash is computed the way engine.go builds the KindFile
		// artifact: over the rendered (always-LF) content.
		a := Artifact{Path: "GOVERNANCE.md", Kind: KindFile, Hash: esc.HashBytes([]byte(content))}
		f := classify(root, a, nil)
		if f.State != InSync {
			t.Fatalf("state = %q, want in-sync: a wholesale CRLF conversion of GOVERNANCE.md must not report as drift", f.State)
		}
	})

	t.Run("GOVERNANCE.md converted to CRLF and content-edited classifies altered", func(t *testing.T) {
		root := t.TempDir()
		content := "# Governance\n\nRendered policy text.\n"
		edited := strings.Replace(content, "Rendered policy text.", "Rendered DIFFERENT text.", 1)
		if edited == content {
			t.Fatal("precondition: replacement text not found")
		}
		crlfEdited := bytes.ReplaceAll([]byte(edited), []byte("\n"), []byte("\r\n"))
		path := filepath.Join(root, "GOVERNANCE.md")
		if err := os.WriteFile(path, crlfEdited, 0o644); err != nil {
			t.Fatal(err)
		}
		a := Artifact{Path: "GOVERNANCE.md", Kind: KindFile, Hash: esc.HashBytes([]byte(content))}
		f := classify(root, a, nil)
		if f.State != Altered {
			t.Fatalf("state = %q, want altered: a genuine content edit must still be caught under CRLF endings", f.State)
		}
		if f.Alteration == nil {
			t.Error("genuine hash mismatch must set Alteration")
		}
	})
}
