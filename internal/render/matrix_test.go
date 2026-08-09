package render_test

import (
	"bytes"
	"testing"

	"github.com/tensorgroup/openescapement/internal/render"
	"github.com/tensorgroup/openescapement/internal/shapetest"
)

var meta = render.BlockMeta{Packs: []string{"acme-org@1.0.0"}}

const body = "policy line one\npolicy line two\n"

// TestShapetestBlockMatchesRender pins shapetest.RenderedBlock (a
// deliberate format duplicate, see its doc comment) to render's real
// output: Extract must find exactly the body it was built from, under the
// hash render itself would compute.
func TestShapetestBlockMatchesRender(t *testing.T) {
	blk, err := render.Extract(shapetest.RenderedBlock("old body\n"))
	if err != nil || blk == nil {
		t.Fatalf("render.Extract on shapetest's block: block=%v err=%v", blk, err)
	}
	if blk.Body != "old body\n" {
		t.Errorf("body drifted: %q", blk.Body)
	}
	if blk.Hash != render.BodyHash(blk.Body) {
		t.Errorf("hash drifted: marker %q, computed %q", blk.Hash, render.BodyHash(blk.Body))
	}
	direct, err := render.Splice(nil, "old body\n", render.BlockMeta{Packs: []string{"acme-org@1.0.0"}})
	if err != nil || !bytes.Equal(direct, shapetest.RenderedBlock("old body\n")) {
		t.Errorf("shapetest.RenderedBlock diverged from render.Splice output:\n%q\nvs\n%q", shapetest.RenderedBlock("old body\n"), direct)
	}
}

// TestSpliceShapeMatrix runs every catalog shape through Splice and a
// second Splice (the subsequent sync), asserting structural validity
// survives both. This is the AGENTS.md matrix for the block write path.
func TestSpliceShapeMatrix(t *testing.T) {
	for _, sh := range shapetest.Shapes() {
		t.Run(sh.Name, func(t *testing.T) {
			out, err := render.Splice(sh.Content, body, meta)
			if err != nil {
				t.Fatalf("Splice: %v", err)
			}
			checkStructure(t, sh, out)

			again, err := render.Splice(out, body, meta)
			if err != nil {
				t.Fatalf("second Splice (sync) errored: %v", err)
			}
			if !bytes.Equal(again, out) {
				t.Errorf("Splice is not idempotent on %s:\nfirst  %q\nsecond %q", sh.Name, out, again)
			}
			checkStructure(t, sh, again)
		})
	}
}

func checkStructure(t *testing.T, sh shapetest.Shape, out []byte) {
	t.Helper()
	blk, err := render.Extract(out)
	if err != nil || blk == nil {
		t.Fatalf("output has no extractable block: block=%v err=%v", blk, err)
	}
	if blk.Body != body {
		t.Errorf("block body drifted: %q", blk.Body)
	}
	if bytes.Contains(out, []byte(render.Placeholder)) {
		t.Errorf("placeholder survived the splice")
	}
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
	if sh.LeadingH1 {
		// Top placement contract: the block lands below a leading H1.
		h1 := bytes.Index(out, []byte("# "))
		begin := bytes.Index(out, []byte("<!-- escapement:begin"))
		if h1 < 0 || begin < h1 {
			t.Errorf("block did not land below the leading H1 (h1=%d begin=%d)", h1, begin)
		}
	}
}

// TestSplicePlaceholderShapeMatrix runs every non-HasBlock shape with the
// placeholder pre-inserted at top, asserting substitution keeps validity.
func TestSplicePlaceholderShapeMatrix(t *testing.T) {
	for _, sh := range shapetest.Shapes() {
		if sh.HasBlock {
			continue // a file never carries both; offerPlacement filters those.
		}
		t.Run(sh.Name, func(t *testing.T) {
			withMarker := append([]byte(render.Placeholder+"\n"), sh.Content...)
			out, err := render.Splice(withMarker, body, meta)
			if err != nil {
				t.Fatalf("Splice: %v", err)
			}
			checkStructure(t, shapetest.Shape{Name: sh.Name, Content: withMarker}, out)
		})
	}
}
