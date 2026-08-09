package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/tensorgroup/openescapement/internal/render"
	"github.com/tensorgroup/openescapement/internal/shapetest"
)

// TestApplyPlacementShapeMatrix crosses both real placements with every
// catalog shape, then runs the subsequent sync (render.Splice) over the
// marked file: the write must be structurally safe on its own AND still
// safe after sync substitutes the block. HasBlock shapes are excluded the
// way production excludes them: offerPlacement never offers a file that
// already carries a block or placeholder (initoffer.go).
func TestApplyPlacementShapeMatrix(t *testing.T) {
	const body = "policy line one\n"
	meta := render.BlockMeta{Packs: []string{"acme-org@1.0.0"}}
	for _, p := range []placement{placeAbove, placeEnd} {
		for _, sh := range shapetest.Shapes() {
			if sh.HasBlock {
				continue
			}
			t.Run(string(p)+"/"+sh.Name, func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, "CLAUDE.md")
				if err := os.WriteFile(path, sh.Content, 0o644); err != nil {
					t.Fatal(err)
				}
				if err := applyPlacement(root, Detected{Path: "CLAUDE.md", Target: "claude"}, p); err != nil {
					t.Fatalf("applyPlacement: %v", err)
				}
				marked, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Contains(marked, []byte(render.Placeholder)) {
					t.Fatalf("marker not written:\n%q", marked)
				}
				checkMarked(t, sh, marked)

				out, err := render.Splice(marked, body, meta)
				if err != nil {
					t.Fatalf("sync after placement: %v", err)
				}
				if bytes.Contains(out, []byte(render.Placeholder)) {
					t.Errorf("sync left the placeholder behind")
				}
				checkMarked(t, sh, out)
				if blk, err := render.Extract(out); err != nil || blk == nil || blk.Body != body {
					t.Errorf("sync did not produce an intact block: blk=%v err=%v", blk, err)
				}
			})
		}
	}
}

func checkMarked(t *testing.T, sh shapetest.Shape, out []byte) {
	t.Helper()
	if err := shapetest.MarkerOwnLine(out); err != nil {
		t.Error(err)
	}
	if err := shapetest.FrontmatterPreserved(sh.Content, out); err != nil {
		t.Error(err)
	}
	if err := shapetest.DashSafety(sh.Content, out); err != nil {
		t.Error(err)
	}
	if err := shapetest.LinesPreserved(sh.Content, out); err != nil {
		t.Error(err)
	}
}
