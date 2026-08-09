// Package shapetest is a test-only catalog of awkward instruction-file
// shapes and structural-validity oracles for the write-path matrix
// mandated by AGENTS.md ("byte preservation is necessary but NOT
// sufficient"). Three bugs in one cycle preserved every byte outside a
// managed region while destroying the file's meaning; every write path
// runs its output through these checks so the class stays dead.
//
// The oracles deliberately do NOT import internal/render: first, render's
// own in-package tests import this package, so the reverse import would be
// a cycle; second, and load-bearing, a detection bug in render (CRLF
// frontmatter, say) must FAIL here, not be mirrored here. This is a test
// oracle, not a second production parser: the single production answer to
// "where does frontmatter end" remains render.FrontmatterEnd.
package shapetest

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/tensorgroup/openescapement/internal/esc"
)

// Shape is one awkward file content a write path must survive.
type Shape struct {
	Name    string
	Content []byte
	// Frontmatter: Content opens with a closed YAML fence at byte 0
	// (LF or CRLF). Any write must keep that fence at byte 0.
	Frontmatter bool
	// LeadingH1: a real leading H1 ("# " line) after any frontmatter.
	LeadingH1 bool
	// HasBlock: Content already carries a rendered managed block.
	HasBlock bool
}

// Shapes is the catalog. Add a shape whenever a new structural hazard is
// discovered; every matrix test picks the catalog up automatically.
func Shapes() []Shape {
	return []Shape{
		{Name: "empty", Content: []byte("")},
		{Name: "whitespace-only", Content: []byte("\n   \n\t\n")},
		{Name: "no-trailing-newline", Content: []byte("# Project\n\nrules")},
		{Name: "crlf", Content: []byte("---\r\ntitle: x\r\n---\r\n\r\n# Project\r\n\r\nrules\r\n"), Frontmatter: true, LeadingH1: true},
		{Name: "frontmatter", Content: []byte("---\ntitle: x\n---\n\nrules\n"), Frontmatter: true},
		{Name: "frontmatter-h1", Content: []byte("---\ntitle: x\n---\n\n# Project\n\nrules\n"), Frontmatter: true, LeadingH1: true},
		{Name: "unterminated-fence", Content: []byte("---\ntitle: x\nrules\n")},
		{Name: "leading-h1", Content: []byte("# Project\n\nrules\n"), LeadingH1: true},
		// The two h1-only shapes are what drive Splice's append-at-end
		// branch (insertAt lands at len(s)); no other shape reaches it.
		{Name: "h1-only", Content: []byte("# Project\n"), LeadingH1: true},
		{Name: "h1-only-no-newline", Content: []byte("# Project"), LeadingH1: true},
		{Name: "hashtag", Content: []byte("#hashtag\n\nrules\n")},
		{Name: "h2-only", Content: []byte("## Sub\n\nrules\n")},
		{Name: "only-managed-block", Content: RenderedBlock("old body\n"), HasBlock: true},
		// A managed block converted wholesale to CRLF (core.autocrlf on a
		// Windows checkout): line endings are presentation, not policy
		// content, so this must still splice cleanly like its LF twin.
		{Name: "only-managed-block-crlf", Content: bytes.ReplaceAll(RenderedBlock("old body\n"), []byte("\n"), []byte("\r\n")), HasBlock: true},
	}
}

// RenderedBlock builds a byte-exact managed block in render's format
// without importing render (see the package comment for why). Task 3 pins
// this duplicate to render's real output with render.Extract, so drift in
// the format fails a test instead of silently staling the catalog.
func RenderedBlock(body string) []byte {
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return []byte("<!-- escapement:begin packs=acme-org@1.0.0 hash=" +
		esc.HashBytes([]byte(body)) + " -->\n" + body + "<!-- escapement:end -->\n")
}

// Lines splits on '\n' only; oracles TrimSuffix "\r" per line, so CRLF
// input is examined line-accurately without a second split rule.
func Lines(b []byte) []string { return strings.Split(string(b), "\n") }

// MarkerOwnLine reports an escapement marker sharing a line with anything
// else. This is the placeEnd bug's oracle: "rules<!-- escapement:block -->"
// preserved every byte and spliced the next sync into the user's last line.
func MarkerOwnLine(out []byte) error {
	for i, ln := range Lines(out) {
		t := strings.TrimSuffix(ln, "\r")
		if !strings.Contains(t, "<!-- escapement:") {
			continue
		}
		if !strings.HasPrefix(t, "<!-- escapement:") || !strings.HasSuffix(t, "-->") {
			return fmt.Errorf("line %d: escapement marker shares its line with other content: %q", i+1, ln)
		}
		if strings.Count(t, "<!--") != 1 {
			return fmt.Errorf("line %d: more than one marker on a single line: %q", i+1, ln)
		}
	}
	return nil
}

// fenceEnd is the oracle's own frontmatter parser: offset one past a
// closed fence opening at byte 0, or 0. Both fences must use the same
// line ending, matching render's deliberately closed rule set.
func fenceEnd(b []byte) int {
	s := string(b)
	for _, nl := range []string{"\r\n", "\n"} {
		open := "---" + nl
		if !strings.HasPrefix(s, open) {
			continue
		}
		i := strings.Index(s[len(open):], nl+"---")
		if i < 0 {
			return 0
		}
		end := len(open) + i + len(nl) + 3
		rest := s[end:]
		if rest != "" && !strings.HasPrefix(rest, nl) {
			return 0
		}
		return end
	}
	return 0
}

// FrontmatterPreserved is the placeAbove bug's oracle: frontmatter is only
// frontmatter at byte 0, so a file that opened with a closed fence must
// still open with those exact bytes after any write.
func FrontmatterPreserved(before, after []byte) error {
	n := fenceEnd(before)
	if n == 0 {
		return nil
	}
	if fenceEnd(after) == 0 {
		return fmt.Errorf("file opened with YAML frontmatter before the write and does not after")
	}
	if !bytes.HasPrefix(after, before[:n]) {
		return fmt.Errorf("frontmatter no longer starts the file byte-for-byte")
	}
	return nil
}

// DashSafety reports a dashes-only line that gained a non-blank,
// non-comment predecessor it did not have before the write. A paragraph
// line directly above `---` turns it into a setext underline; an HTML
// comment (any escapement marker) or another dashes line does not.
// Note: inside legitimate frontmatter the closing fence always has a text
// predecessor, so before-hazard == after-hazard there and this oracle is
// intentionally silent; FrontmatterPreserved owns that case.
func DashSafety(before, after []byte) error {
	if setextHazard(after) && !setextHazard(before) {
		return fmt.Errorf("a dashes line gained a text line directly above it; the write manufactured a setext heading")
	}
	return nil
}

func setextHazard(b []byte) bool {
	prevIsText := false
	for _, ln := range Lines(b) {
		t := strings.TrimSuffix(ln, "\r")
		dashes := len(t) >= 3 && strings.Trim(t, "-") == ""
		if dashes && prevIsText {
			return true
		}
		prevIsText = t != "" && !dashes && !strings.HasPrefix(t, "<!--")
	}
	return false
}

// LinesPreserved checks every original line survives whole and in order.
// This oracle guards USER-owned lines: lines containing the placeholder are
// exempt (substitution consumes them by contract), and lines inside a
// pre-existing managed block — from the begin marker through the end
// marker, inclusive — are exempt too, since escapement's own managed
// region is replaceable by contract; that replacement is the entire point
// of a sync. Catches both mid-line splices and dropped user content.
func LinesPreserved(before, after []byte) error {
	af := Lines(after)
	j := 0
	// An unterminated begin marker earns no exemption at all: exempting
	// everything after a begin that never closes would silently pass a
	// write that destroyed the rest of the file. Refusing the exemption
	// fails toward a false alarm, the safe direction for a guard oracle.
	exempt := blockTerminated(Lines(before))
	inBlock := false
	// The forward-only j cursor below is exact, not merely greedy: for
	// subsequence testing, matching each wanted line at its earliest
	// remaining occurrence can never starve a later line that some other
	// alignment would have satisfied.
	for _, want := range Lines(before) {
		t := strings.TrimSuffix(want, "\r")
		if strings.Contains(want, "<!-- escapement:block -->") {
			continue
		}
		if exempt && strings.HasPrefix(t, "<!-- escapement:begin") {
			inBlock = true
			continue
		}
		if inBlock {
			if t == "<!-- escapement:end -->" {
				inBlock = false
			}
			continue
		}
		found := false
		for j < len(af) {
			if af[j] == want {
				found = true
				j++
				break
			}
			j++
		}
		if !found {
			return fmt.Errorf("original line %q was lost or cut by the write", want)
		}
	}
	return nil
}

// blockTerminated reports whether every begin marker in lines is closed by
// a later end marker, so LinesPreserved knows the block exemption is safe
// to apply.
func blockTerminated(lines []string) bool {
	in := false
	for _, ln := range lines {
		t := strings.TrimSuffix(ln, "\r")
		if strings.HasPrefix(t, "<!-- escapement:begin") {
			in = true
		} else if t == "<!-- escapement:end -->" {
			in = false
		}
	}
	return !in
}
