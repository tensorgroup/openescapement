package shapetest

import "testing"

func TestMarkerOwnLineCatchesGluedMarker(t *testing.T) {
	if err := MarkerOwnLine([]byte("rules<!-- escapement:block -->\n")); err == nil {
		t.Error("marker glued to user text must be reported")
	}
	if err := MarkerOwnLine([]byte("<!-- escapement:block -->tail\n")); err == nil {
		t.Error("user text glued after a marker must be reported")
	}
	if err := MarkerOwnLine([]byte("rules\n<!-- escapement:block -->\n")); err != nil {
		t.Errorf("marker on its own line must pass: %v", err)
	}
}

func TestFrontmatterPreservedCatchesDemotion(t *testing.T) {
	fm := []byte("---\ntitle: x\n---\nrules\n")
	demoted := []byte("<!-- escapement:block -->\n---\ntitle: x\n---\nrules\n")
	if err := FrontmatterPreserved(fm, demoted); err == nil {
		t.Error("frontmatter pushed off byte 0 must be reported")
	}
	if err := FrontmatterPreserved(fm, append([]byte{}, fm...)); err != nil {
		t.Errorf("untouched frontmatter must pass: %v", err)
	}
	crlf := []byte("---\r\ntitle: x\r\n---\r\nrules\r\n")
	if err := FrontmatterPreserved(crlf, []byte("junk\n"+string(crlf))); err == nil {
		t.Error("CRLF frontmatter pushed off byte 0 must be reported")
	}
}

func TestDashSafetyCatchesNewSetext(t *testing.T) {
	before := []byte("---\nrules\n") // dashes at top: thematic break, no text above
	after := []byte("inserted text\n---\nrules\n")
	if err := DashSafety(before, after); err == nil {
		t.Error("a dashes line gaining a text predecessor must be reported")
	}
	// A comment line above dashes is an HTML block, not a paragraph: no setext.
	ok := []byte("<!-- escapement:end -->\n---\nrules\n")
	if err := DashSafety(before, ok); err != nil {
		t.Errorf("comment line above dashes must pass: %v", err)
	}
}

func TestLinesPreservedCatchesCutLine(t *testing.T) {
	before := []byte("# T\n\nrules")
	if err := LinesPreserved(before, []byte("# T\n\nrules<!-- escapement:block -->\n")); err == nil {
		t.Error("a cut last line must be reported")
	}
	if err := LinesPreserved(before, []byte("# T\n\nrules\n<!-- escapement:block -->\n")); err != nil {
		t.Errorf("intact lines must pass: %v", err)
	}
	// Placeholder lines are exempt: substitution consumes them by contract.
	if err := LinesPreserved([]byte("a\n<!-- escapement:block -->\nb\n"), []byte("a\nBLOCK\nb\n")); err != nil {
		t.Errorf("substituted placeholder line must not count as lost: %v", err)
	}
}

// TestLinesPreservedBlockExemptionIsBounded proves the managed-block
// exemption covers only the block region, not the user lines around it on
// EITHER side: a user line dropped before the block, or after it, must
// still alarm, while a block replacement alone (user lines intact) must
// pass.
func TestLinesPreservedBlockExemptionIsBounded(t *testing.T) {
	before := append([]byte("mine\n"), RenderedBlock("old\n")...)

	// The block is legitimately replaced, but "mine" is gone too: must alarm.
	droppedUserLine := RenderedBlock("new\n")
	if err := LinesPreserved(before, droppedUserLine); err == nil {
		t.Error("a user line dropped alongside a block replacement must be reported")
	}

	// The block is legitimately replaced and "mine" survives: must pass.
	userLineIntact := append([]byte("mine\n"), RenderedBlock("new\n")...)
	if err := LinesPreserved(before, userLineIntact); err != nil {
		t.Errorf("a bare managed-block replacement must not count as lost: %v", err)
	}

	// A user line AFTER the block is dropped: must alarm too. This proves the
	// end boundary of the exemption is real — a LinesPreserved that never
	// clears inBlock would swallow "tail" along with the block and miss this.
	beforeTail := append(append([]byte{}, RenderedBlock("old\n")...), []byte("tail\n")...)
	droppedTailLine := RenderedBlock("new\n")
	if err := LinesPreserved(beforeTail, droppedTailLine); err == nil {
		t.Error("a user line dropped after a block replacement must be reported")
	}
}

func TestMarkerOwnLineCatchesGluedMarkerPair(t *testing.T) {
	glued := []byte("<!-- escapement:end --><!-- escapement:begin packs=x hash=y -->\n")
	if err := MarkerOwnLine(glued); err == nil {
		t.Error("two markers glued on one line must be reported")
	}
}

func TestLinesPreservedRefusesUnterminatedBlockExemption(t *testing.T) {
	// A begin marker that never closes exempts nothing: the lines after it
	// are treated as user content, so dropping one must alarm.
	before := []byte("<!-- escapement:begin packs=x hash=y -->\nmine\n")
	if err := LinesPreserved(before, []byte("replaced\n")); err == nil {
		t.Error("content after an unterminated begin marker must stay guarded")
	}
	// The same content kept intact must still pass: strictness may not
	// turn a faithful write into an alarm.
	if err := LinesPreserved(before, append([]byte{}, before...)); err != nil {
		t.Errorf("an untouched file must pass regardless of marker health: %v", err)
	}
}
