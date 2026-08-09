# Structural Validity Matrix + Residual Sweep Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Guard the recurring "bytes preserved, meaning destroyed" bug class with a write-path × file-shape test matrix, fix the CRLF instance it confirms, and clear the eight residuals parked in the last two SDD ledgers.

**Architecture:** A new test-only package `internal/shapetest` holds one catalog of awkward file shapes and independent structural-validity oracles (deliberately NOT reusing `internal/render`'s parsers, so a render bug cannot vouch for itself). Each write path — `render.Splice`, `applyPlacement`, `render.MergeMCP`, `mergeDir` — gets a matrix test in its own package that runs every applicable shape through the write AND a subsequent sync, then through the oracles. Production fixes (CRLF frontmatter detection, status-path guards, init confirmation lines) are separate small TDD tasks.

**Tech Stack:** Go 1.24, stdlib only (`gopkg.in/yaml.v3` is the sole allowed dependency and none of this work touches YAML parsing).

## Global Constraints

- Single external dependency policy: stdlib only for everything in this plan.
- `go test ./...`, `go vet ./...`, `gofmt -w .` must pass before any task is claimed done.
- Sentinel errors in `internal/esc` map to exit codes (0 ok, 1 drift/constraint, 2 usage, 3 integrity, 4 other). Containment/symlink refusals are plain errors (exit 4), never `ErrConstraint` — see AGENTS.md.
- Renderer invariants: bytes outside a managed block never modified; writes atomic; output deterministic. NEW (already in AGENTS.md, working tree): byte preservation is necessary but NOT sufficient — structural meaning must survive every write, or the tool fails closed.
- **Adjudicated rulings in `.superpowers/sdd/2026-08-05-local-amendment-model/progress.md` and `.superpowers/sdd/2026-08-08-onboarding-existing-repos/progress.md` are settled. Do not re-litigate them.**
- **Matrix failures are findings, not inconveniences.** If a matrix case fails against current code: fix it in-task only when the fix is small and unambiguous; otherwise report to the orchestrator for adjudication. NEVER weaken an assertion to match broken behavior.
- **NO COMMITS in this session.** The owner has not authorized commits this cycle. Skip every "Commit" step; instead snapshot `git diff > .superpowers/sdd/2026-08-09-structural-validity-and-residuals/diffs/task-N.diff` at task end (create the dir first; also snapshot `git diff --stat` output into the ledger). If the owner later authorizes commits, the Commit steps show the intended messages.
- Line-ending ruling (pre-adjudicated for this plan, do not reopen): escapement always RENDERS LF. CRLF support means *detection* becomes CRLF-tolerant so nothing is structurally destroyed; an LF block inside a CRLF file (mixed endings) is accepted, not an error. Byte-exact hashing stays: an autocrlf-converted managed block still reports `altered` (truthful; known papercut, surfaced to the owner at check-in, out of scope here).
- New prose written by this plan (notice text, CHANGELOG bullets, user-facing detail strings) carries no em-dashes. Existing em-dashes elsewhere stay.

---

### Task 1: `internal/shapetest` — shape catalog and structural oracles

**Files:**
- Create: `internal/shapetest/shapetest.go`
- Test: `internal/shapetest/shapetest_test.go`

**Interfaces:**
- Consumes: `internal/esc` (`esc.HashBytes([]byte) string`) and stdlib only. MUST NOT import `internal/render` — in-package tests in `render` will import shapetest, and shapetest importing render would be an import cycle.
- Produces (used verbatim by Tasks 3, 4, 5, 6):
  - `type Shape struct { Name string; Content []byte; Frontmatter, LeadingH1, HasBlock bool }`
  - `func Shapes() []Shape`
  - `func RenderedBlock(body string) []byte`
  - `func Lines(b []byte) []string`
  - `func MarkerOwnLine(out []byte) error`
  - `func FrontmatterPreserved(before, after []byte) error`
  - `func DashSafety(before, after []byte) error`
  - `func LinesPreserved(before, after []byte) error`

- [ ] **Step 1: Write the failing self-test first** (`internal/shapetest/shapetest_test.go`). Each oracle must catch the violation it exists for, and pass on a clean case — a matrix built on vacuous oracles proves nothing:

```go
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
```

- [ ] **Step 2: Run to verify failure.** `go test ./internal/shapetest/` — FAIL: undefined symbols.

- [ ] **Step 3: Implement** `internal/shapetest/shapetest.go`:

```go
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
// Lines containing the placeholder are exempt: substitution consumes them
// by contract. Catches both mid-line splices and dropped content.
func LinesPreserved(before, after []byte) error {
	af := Lines(after)
	j := 0
	for _, want := range Lines(before) {
		if strings.Contains(want, "<!-- escapement:block -->") {
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
```

- [ ] **Step 4: Run to verify pass.** `go test ./internal/shapetest/` — PASS. Then `go vet ./internal/shapetest/ && gofmt -l internal/shapetest/` (no output).

- [ ] **Step 5: Snapshot the diff** (see Global Constraints; commit message if later authorized: `test: add shapetest, a shape catalog and structural-validity oracles`).

---

### Task 2: CRLF-tolerant frontmatter and top-placement detection, blank-line seam fix

Two NEW instances of the bug class, both found while designing the matrix (handoff P1c):

1. **CRLF (predicted):** `frontmatterEnd` matches only `"---\n"`, so a CRLF file's `---\r\n` fence is invisible, and both `Splice`'s top placement and init's `placeAbove` insert at byte 0, above the fence — the placeAbove demotion bug again, on every CRLF file.
2. **Blank-line seam:** `insertAt` detects a leading H1 only when `"# "` starts IMMEDIATELY after the fence end. On `---\ntitle: x\n---\n\n# Title\n` (blank line between fence and title, the common real-world shape) the H1 is not seen and the block wedges between frontmatter and title, violating the documented contract "new managed blocks are inserted at the TOP of a file, after YAML frontmatter and a leading H1" (AGENTS.md). Not destructive, but the matrix's `LeadingH1` assertion is written to the contract, so this must be fixed or the contract re-adjudicated; fixed is the ruling (a single-blank-line skip mirrors the closed rule the post-H1 skip already uses).

**Files:**
- Modify: `internal/render/block.go:46-82` (`frontmatterEnd`, `insertAt`)
- Test: `internal/render/block_test.go` (extend `TestFrontmatterEnd` and `TestSpliceTopPlacement`)

**Interfaces:**
- Consumes: nothing new.
- Produces: `FrontmatterEnd([]byte) int` and `insertAt(string) int` now recognize CRLF fences/H1 blank lines. Signatures unchanged; Tasks 3 and 4 rely on the behavior. Rendered output stays LF-only (Global Constraints ruling).

- [ ] **Step 1: Write the failing tests.** Add cases to the existing tables in `internal/render/block_test.go` (match the table style already there; the case content is what matters):

To `TestFrontmatterEnd`'s table:

```go
{"crlf fence", "---\r\ntitle: x\r\n---\r\nrules\r\n", len("---\r\ntitle: x\r\n---\r\n")},
{"crlf fence at eof", "---\r\ntitle: x\r\n---", len("---\r\ntitle: x\r\n---")},
{"crlf unterminated", "---\r\ntitle: x\r\n", 0},
{"mixed endings not frontmatter", "---\r\ntitle: x\n---\n", 0},
```

To `TestSpliceTopPlacement`'s table (each case asserts the block lands AFTER the named prefix, not at byte 0):

```go
{"crlf frontmatter and h1", "---\r\ntitle: x\r\n---\r\n\r\n# T\r\n\r\nrules\r\n",
	"---\r\ntitle: x\r\n---\r\n\r\n# T\r\n\r\n"},
{"blank line between frontmatter and h1", "---\ntitle: x\n---\n\n# T\n\nrules\n",
	"---\ntitle: x\n---\n\n# T\n\n"},
{"blank line then h1, no frontmatter", "\n# T\n\nrules\n",
	"\n# T\n\n"},
{"blank line after frontmatter, no h1", "---\ntitle: x\n---\n\nrules\n",
	"---\ntitle: x\n---\n"},
```

(Adapt each case's fields to the table's actual column meanings — the second string is a prefix the output must keep before the block. If the table shape differs, add an equivalent standalone test asserting `bytes.HasPrefix(out, wantPrefix)` and that the begin marker appears exactly once, after that prefix. Note the fourth case: with no H1 the block goes directly after the fence; the blank line is only skipped when it leads to an H1.)

- [ ] **Step 2: Run to verify failure.** `go test ./internal/render/ -run 'TestFrontmatterEnd|TestSpliceTopPlacement'` — the CRLF cases FAIL (offset 0 / block at byte 0).

- [ ] **Step 3: Implement.** Replace `frontmatterEnd` and the H1 blank-line skip in `insertAt` in `internal/render/block.go`:

```go
func frontmatterEnd(s string) int {
	// The fence's line ending is taken from the opening line and required
	// throughout: a file that mixes endings inside its fence is not treated
	// as frontmatter, keeping the rule set closed (same philosophy as the
	// LF-only version, extended by exactly one ending).
	nl := "\n"
	if strings.HasPrefix(s, "---\r\n") {
		nl = "\r\n"
	} else if !strings.HasPrefix(s, "---\n") {
		return 0
	}
	open := 3 + len(nl)
	end := strings.Index(s[open:], nl+"---")
	if end < 0 {
		return 0
	}
	after := open + end + len(nl) + 3
	rest := s[after:]
	if rest != "" && !strings.HasPrefix(rest, nl) {
		return 0
	}
	if strings.HasPrefix(rest, nl) {
		after += len(nl)
	}
	return after
}
```

Replace `insertAt` wholesale (blank-line seam before the H1, CRLF-aware blank line after it):

```go
// insertAt returns the offset where a new block belongs: the top of the file,
// after leading YAML frontmatter and a leading H1 if either is present. One
// blank line between frontmatter and the H1 does not stop the H1 being the
// leading H1 (same single-blank-line rule as the skip after the H1). Only
// these constructs are skipped; the rule set is closed deliberately so
// output stays predictable.
func insertAt(s string) int {
	i := frontmatterEnd(s)
	h := i
	if strings.HasPrefix(s[h:], "\r\n") {
		h += 2
	} else if strings.HasPrefix(s[h:], "\n") {
		h++
	}
	if strings.HasPrefix(s[h:], "# ") {
		// Commit the blank-line skip only now: with no H1 behind it, the
		// block belongs directly after the fence, above the blank line.
		i = h
		if nl := strings.IndexByte(s[i:], '\n'); nl >= 0 {
			i += nl + 1
			if strings.HasPrefix(s[i:], "\r\n") {
				i += 2
			} else if strings.HasPrefix(s[i:], "\n") {
				i++
			}
		} else {
			i = len(s)
		}
	}
	return i
}
```

(Everywhere: check `"\r\n"` BEFORE `"\n"`; the reverse order never matches CRLF.)

- [ ] **Step 4: Run to verify pass.** `go test ./internal/render/` — all PASS, including the pre-existing LF cases (the LF path must be byte-for-byte equivalent to the old code; `TestGolden` guards rendered output).

- [ ] **Step 5: Full check.** `go test ./... && go vet ./... && gofmt -l .` (no gofmt output).

- [ ] **Step 6: Snapshot the diff** (message: `fix(render): recognize CRLF frontmatter and H1 spacing in top placement`).

---

### Task 3: Splice × shape matrix

**Files:**
- Create: `internal/render/matrix_test.go` (package `render_test` — external, so it may import shapetest freely)

**Interfaces:**
- Consumes: `shapetest.Shapes()`, oracles, `shapetest.RenderedBlock`; `render.Splice(existing []byte, body string, meta render.BlockMeta) ([]byte, error)`, `render.Extract(file []byte) (*render.Block, error)`, `render.BodyHash(body string) string`, `render.Placeholder`.
- Produces: the matrix pattern Tasks 4-6 copy.

- [ ] **Step 1: Write the matrix test:**

```go
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
```

- [ ] **Step 2: Add a placeholder-shape pass.** Same file: run every non-`HasBlock` shape again with the placeholder pre-inserted at top, asserting substitution keeps validity:

```go
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
```

(Note `FrontmatterPreserved` is intentionally called with `withMarker` as "before": a file whose frontmatter already sits below a marker has no byte-0 fence, so the oracle is vacuous there — that placement was the USER's explicit `<!-- escapement:block -->` override, which is allowed to sit anywhere.)

- [ ] **Step 3: Run.** `go test ./internal/render/ -run 'TestShapetest|TestSpliceShapeMatrix|TestSplicePlaceholder' -v`. Expected: PASS for every shape (Task 2 fixed CRLF). **Any failure is a NEW instance of the bug class: apply the Global Constraints finding rule and record it in the ledger either way.**

- [ ] **Step 4: Full check.** `go test ./... && go vet ./... && gofmt -l .`

- [ ] **Step 5: Snapshot the diff** (message: `test(render): shape matrix for Splice across awkward file shapes`).

---

### Task 4: applyPlacement × shape matrix

**Files:**
- Create: `internal/cli/placement_matrix_test.go` (package `cli` — in-package; `applyPlacement` is unexported. shapetest does not import cli, so no cycle.)

**Interfaces:**
- Consumes: `applyPlacement(root string, d Detected, p placement) error` (initoffer.go:154), `placeAbove`/`placeEnd`, `Detected{Path, Target string, ...}` (see initscan.go for exact fields; only Path and Target matter here), shapetest, `render.Splice`/`render.Extract`.
- Produces: nothing consumed later.

- [ ] **Step 1: Write the matrix test:**

```go
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
```

(If `Detected` field names differ, copy the construction used by `TestApplyPlacementEndPreservesNewlineSeparation` at initoffer_test.go:84 — equal rigor, real struct.)

- [ ] **Step 2: Run.** `go test ./internal/cli/ -run TestApplyPlacementShapeMatrix -v`. Expected: PASS everywhere. The CRLF/placeAbove cell passes because `applyPlacement` delegates to `render.FrontmatterEnd`, fixed in Task 2 — this cell is the regression guard proving the shared-parser design holds. **Any failure: finding rule.**

- [ ] **Step 3: Pin the CRLF prompt-answer non-bug.** The survey noted `readBoundedLine` keeps a trailing `\r`, and only the call sites' `strings.TrimSpace` saves a CRLF answer ("y\r\n") from silently declining the gate. That is one refactor away from a real bug; pin it in `internal/cli/initoffer_test.go`:

```go
// A CRLF-terminated answer ("y\r\n", routine from Windows terminals and
// piped scripts) must be accepted. readBoundedLine keeps the '\r'; the
// TrimSpace at the call sites is what strips it, and this test is what
// notices if that pairing is ever broken.
func TestAskPlacementAcceptsCRLFAnswers(t *testing.T) {
	var out bytes.Buffer
	br := bufio.NewReader(strings.NewReader("y\r\ne\r\n"))
	got := askPlacement(&out, br, []Detected{{Path: "CLAUDE.md", Target: "claude"}}, false)
	if got != placeEnd {
		t.Errorf("CRLF-terminated y/e answers gave %q, want placeEnd", got)
	}
}
```

- [ ] **Step 4: Full check.** `go test ./... && go vet ./... && gofmt -l .`

- [ ] **Step 5: Snapshot the diff** (message: `test(cli): shape matrix for applyPlacement, CRLF prompt answers pinned`).

---

### Task 5: MergeMCP matrix + whitespace-only tolerance

**Files:**
- Modify: `internal/render/mcp.go:19` (one line)
- Test: `internal/render/mcp_test.go` (extend)

**Interfaces:**
- Consumes: `render.MergeMCP(existing []byte, servers map[string]map[string]any, prevOwned []string) ([]byte, []string, error)`.
- Produces: `MergeMCP` treats a whitespace-only file as empty (matching `UnownedMCPServers`'s existing `bytes.TrimSpace` rule). Everything else unchanged.

- [ ] **Step 1: Write the failing + pinning tests** in `internal/render/mcp_test.go` (package matches the existing file):

```go
func TestMergeMCPShapeMatrix(t *testing.T) {
	servers := map[string]map[string]any{"esc-tools": {"command": "esc-mcp"}}
	cases := []struct {
		name     string
		existing string
		wantErr  bool
	}{
		{"nil", "", false},
		{"whitespace-only", "\n  \n", false}, // FAILS pre-fix: json parse error on whitespace
		{"empty-object", "{}\n", false},
		{"user-server-preserved", `{"mcpServers":{"team-db":{"command":"db"}}}`, false},
		{"crlf-json", "{\r\n  \"mcpServers\": {}\r\n}\r\n", false},
		{"no-trailing-newline", `{"mcpServers":{}}`, false},
		{"invalid-json", "not json", true},
		{"mcpservers-not-object", `{"mcpServers": 3}`, true},
		{"name-squat", `{"mcpServers":{"esc-tools":{"command":"theirs"}}}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, keys, err := MergeMCP([]byte(tc.existing), servers, nil)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("unsupported shape must fail closed, got:\n%s", out)
				}
				return
			}
			if err != nil {
				t.Fatalf("MergeMCP: %v", err)
			}
			var doc map[string]any
			if err := json.Unmarshal(out, &doc); err != nil {
				t.Fatalf("output is not valid JSON: %v\n%s", err, out)
			}
			ms, ok := doc["mcpServers"].(map[string]any)
			if !ok {
				t.Fatalf("output mcpServers is not an object:\n%s", out)
			}
			if _, ok := ms["esc-tools"]; !ok {
				t.Errorf("owned server missing")
			}
			if tc.name == "user-server-preserved" {
				if _, ok := ms["team-db"]; !ok {
					t.Errorf("user's own server entry was dropped")
				}
			}
			// The subsequent sync: merging again over our own output must
			// be a byte-for-byte fixed point.
			again, _, err := MergeMCP(out, servers, keys)
			if err != nil {
				t.Fatalf("second merge (sync) errored: %v", err)
			}
			if !bytes.Equal(again, out) {
				t.Errorf("MergeMCP not idempotent:\nfirst  %s\nsecond %s", out, again)
			}
		})
	}
}
```

(Add `"bytes"` and `"encoding/json"` to the test file's imports if absent.)

- [ ] **Step 2: Run to verify the one expected failure.** `go test ./internal/render/ -run TestMergeMCPShapeMatrix -v` — only `whitespace-only` FAILS (parse error). Everything else already passes; that is the finding report for this path: MergeMCP already fails closed on real hazards.

- [ ] **Step 3: Fix.** In `internal/render/mcp.go`, change the parse gate to match `UnownedMCPServers`:

```go
	if len(bytes.TrimSpace(existing)) > 0 {
		if err := json.Unmarshal(existing, &doc); err != nil {
			return nil, nil, fmt.Errorf("parsing existing .mcp.json: %w", err)
		}
	}
```

(`bytes` is already imported in mcp.go.)

- [ ] **Step 4: Run to verify pass.** `go test ./internal/render/` — PASS. Then full check.

- [ ] **Step 5: Snapshot the diff** (message: `fix(render): MergeMCP treats a whitespace-only .mcp.json as empty, matrix for the merge path`).

---

### Task 6: mergeDir verbatim-copy matrix

mergeDir writes whole files it does not parse, so "structural validity" here means byte-verbatim copies (no newline or encoding munging) for awkward file contents, nested trees preserved, and convergence. Hostile-entry behavior (symlinks, `..`, self-targeting) is already pinned by existing tests from the prior cycle: do not duplicate them.

**Files:**
- Test: `internal/engine/mergedir_test.go` (extend; package `engine`)

**Interfaces:**
- Consumes: `mergeDir(root, artPath, src, dst string, prevFiles []string) ([]string, error)` (apply.go:620), shapetest (content bytes only).
- Produces: nothing consumed later.

- [ ] **Step 1: Write the test:**

```go
// TestMergeDirCopiesShapesVerbatim runs awkward file contents through
// mergeDir twice (write, then the subsequent sync) and asserts every byte
// lands verbatim: mergeDir owns whole files, so its structural-validity
// obligation is to never reinterpret content, and to converge.
func TestMergeDirCopiesShapesVerbatim(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "pack-src")
	files := map[string]string{
		"SKILL.md":            "---\r\nname: s\r\n---\r\n\r\nCRLF body\r\n",
		"no-newline.md":       "last line without newline",
		"nested/deep/ref.md":  "# nested\n\ncontent\n",
		"whitespace-only.md":  "\n  \n",
	}
	for rel, content := range files {
		p := filepath.Join(src, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const artPath = ".claude/skills/esc-acme-org-s"
	dst := filepath.Join(root, filepath.FromSlash(artPath))

	written, err := mergeDir(root, artPath, src, dst, nil)
	if err != nil {
		t.Fatalf("mergeDir: %v", err)
	}
	if len(written) != len(files) {
		t.Fatalf("wrote %d files, want %d: %v", len(written), len(files), written)
	}
	for rel, content := range files {
		got, err := os.ReadFile(filepath.Join(dst, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if string(got) != content {
			t.Errorf("%s not copied verbatim:\nwant %q\ngot  %q", rel, content, got)
		}
	}

	// The subsequent sync: same pack state, prevFiles from the first pass.
	again, err := mergeDir(root, artPath, src, dst, written)
	if err != nil {
		t.Fatalf("second mergeDir (sync): %v", err)
	}
	if !slices.Equal(again, written) {
		t.Errorf("file list not stable across sync: %v vs %v", again, written)
	}
	for rel, content := range files {
		got, _ := os.ReadFile(filepath.Join(dst, filepath.FromSlash(rel)))
		if string(got) != content {
			t.Errorf("%s changed on the second sync:\nwant %q\ngot  %q", rel, content, got)
		}
	}
}
```

(Use the file's existing imports style; add `"slices"` if absent. If `mergeDir`'s written list ordering makes `slices.Equal` wrong, sort both — but the survey says the list is collected sorted.)

- [ ] **Step 2: Run.** `go test ./internal/engine/ -run TestMergeDirCopiesShapesVerbatim -v` — expected PASS (mergeDir copies bytes). **Any failure: finding rule.** This also closes the parked minor "no test covers a nested subdirectory in a pack skill tree."

- [ ] **Step 3: Full check, snapshot the diff** (message: `test(engine): mergeDir verbatim-copy matrix incl. nested dirs and CRLF`).

---

### Task 7: Contain the orphan-BLOCK status read

The parked ruling (2026-08-05 ledger, sweep minor 3) already adjudicated this as "the correct next fix": `status.go:147` does `os.ReadFile` on a lockfile-derived path with no `containedPath` and no `refuseSymlinks`, the same class as the Critical fixed on the dir path directly below it.

**Files:**
- Modify: `internal/engine/status.go:143-179` (orphan-block loop)
- Test: `internal/engine/` — extend the test file that covers the orphan-dir hostile-lockfile cases (grep `refuseSymlinks` and `validateDirEntries` in `internal/engine/*_test.go` to find it; mirror its fixture pattern).

**Interfaces:**
- Consumes: `containedPath(root, rel string) (string, error)` (apply.go:21), `refuseSymlinks(root, rel string) error` (apply.go:47).
- Produces: no signature changes.

- [ ] **Step 1: Write the failing tests** in `internal/cli` (the proven route for driving `Status` end to end: `setupGovernedRepo` gives a real governed repo, then mutate its lockfile by hand the way `orphandir_test.go` hand-builds one). Two cases:

```go
// TestStatusOrphanBlockRefusesEscapingLockPath: the orphan-block status
// pass read lockfile paths with no containment or symlink check, the same
// class as the Critical fixed on the dir pass. A hostile entry must not be
// read at all: the read-only surface skips silently, and Apply failing
// closed on the same entry is the signal that case gets (same ruling as
// the dir pass).
func TestStatusOrphanBlockRefusesEscapingLockPath(t *testing.T) {
	root := setupGovernedRepo(t)
	victim := filepath.Join(filepath.Dir(root), "victim.md")
	if err := os.WriteFile(victim, shapetest.RenderedBlock("outside content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lock, err := lockfile.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	lock.Artifacts = append(lock.Artifacts, lockfile.LockArtifact{
		Path: "../victim.md", Kind: engine.KindBlock, Hash: "irrelevant",
	})
	if err := lock.Save(root); err != nil {
		t.Fatal(err)
	}
	out, code := runEscOut(t, root, "status", "--json")
	if code != 0 && code != 1 {
		t.Fatalf("status exit %d:\n%s", code, out)
	}
	if strings.Contains(out, "victim.md") {
		t.Errorf("status followed a lockfile path out of the repo to build a report:\n%s", out)
	}
}

// TestStatusOrphanBlockRefusesSymlinkedPath: same rule through a symlink.
func TestStatusOrphanBlockRefusesSymlinkedPath(t *testing.T) {
	root := setupGovernedRepo(t)
	outside := filepath.Join(filepath.Dir(root), "outside.md")
	if err := os.WriteFile(outside, shapetest.RenderedBlock("outside content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "NOTES.md")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	lock, err := lockfile.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	lock.Artifacts = append(lock.Artifacts, lockfile.LockArtifact{
		Path: "NOTES.md", Kind: engine.KindBlock, Hash: "irrelevant",
	})
	if err := lock.Save(root); err != nil {
		t.Fatal(err)
	}
	out, code := runEscOut(t, root, "status", "--json")
	if code != 0 && code != 1 {
		t.Fatalf("status exit %d:\n%s", code, out)
	}
	if strings.Contains(out, "NOTES.md") {
		t.Errorf("status read through a symlink to build a report:\n%s", out)
	}
}
```

(Verify against the codebase before running: `engine.KindBlock`'s type vs `lockfile.LockArtifact.Kind`'s field type — if they differ, use the literal string an existing lockfile shows for a block artifact. If `setupGovernedRepo`'s repo already has a NOTES.md or the plan claims the path, pick an unclaimed name. `shapetest` is importable from `internal/cli` tests; no cycle, shapetest imports only esc and stdlib.)

- [ ] **Step 2: Run to verify both fail** (a finding IS produced pre-fix).

- [ ] **Step 3: Implement.** In the orphan-block loop, replace the bare read:

```go
			abs, cerr := containedPath(root, la.Path)
			if cerr != nil {
				continue
			}
			// Read-only surface: a lockfile path reaching out of the repo
			// through a symlink is refused rather than followed to build a
			// report. Apply fails closed and loudly on the same entry, which
			// is the signal that case gets. Same rule as the orphan-dir pass
			// below; the block pass predates it and was the last bare read.
			if refuseSymlinks(root, la.Path) != nil {
				continue
			}
			content, err := os.ReadFile(abs)
			if err != nil {
				continue // already gone
			}
```

(This also drops the `filepath.Join(root, filepath.FromSlash(la.Path))` — `containedPath` returns the absolute path. Check whether `filepath` is still used elsewhere in the file before touching imports; it is, in `classify`.)

- [ ] **Step 4: Verify non-vacuity.** Temporarily revert the guard, confirm both tests fail, restore. Run `go test ./internal/engine/`.

- [ ] **Step 5: Full check, snapshot the diff** (message: `fix(engine): contain and symlink-check the orphan-block status read`).

---

### Task 8: Orphan-dir status must not advise what Apply refuses

Parked ruling (2026-08-05 ledger, sweep minor 1): the orphan-dir status pass has no escapement-ownership check, so a lock entry at `.claude/skills/team-notes` makes status advise `esc sync --force` while Apply refuses with "not an escapement-owned directory" and exits 4.

**Files:**
- Modify: `internal/engine/status.go:198-242` (orphan-dir loop)
- Test: same engine test file as Task 7's neighbors.

**Interfaces:**
- Consumes: the ownership rule from apply.go:318-345: `!strings.HasPrefix(path.Base(prev.Path), "esc-")` on the REPO-RELATIVE path (never absolute; see the comment there for why). Imports: add `"path"` and `"strings"` to status.go if absent.
- Produces: no signature changes.

- [ ] **Step 1: Confirm Apply's check order.** `TestApplyOrphanDirOwnershipGuardUsesRelativePath` (orphandir_test.go:142) and the apply.go:318 excerpt show the ownership guard fires FIRST in the loop, before containment, symlinks, or any stat. So Apply hard-errors on a non-owned entry even when the directory is gone, and status must therefore report the entry BEFORE its own `os.Stat` "already gone" skip — place the new check accordingly.

- [ ] **Step 2: Write the failing test** in `internal/cli`, same fixture route as Task 7:

```go
// TestStatusOrphanDirNotOwnedMatchesApplyRefusal: status used to advise
// `esc sync --force` for a lockfile dir entry Apply refuses to touch as
// not escapement-owned (exit 4). Advice must describe what sync will
// actually do.
func TestStatusOrphanDirNotOwnedMatchesApplyRefusal(t *testing.T) {
	root := setupGovernedRepo(t)
	writeFiles(t, root, map[string]string{".claude/skills/team-notes/notes.md": "ours\n"})
	lock, err := lockfile.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	lock.Artifacts = append(lock.Artifacts, lockfile.LockArtifact{
		Path: ".claude/skills/team-notes", Kind: engine.KindDir,
		Hash: "irrelevant", Files: []string{"notes.md"},
	})
	if err := lock.Save(root); err != nil {
		t.Fatal(err)
	}
	out, _ := runEscOut(t, root, "status", "--json")
	var report struct {
		Findings []struct {
			Subject string `json:"subject"`
			Managed string `json:"managed"`
			Detail  string `json:"detail"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("status --json did not parse: %v\n%s", err, out)
	}
	var found bool
	for _, f := range report.Findings {
		if f.Subject != ".claude/skills/team-notes" {
			continue
		}
		found = true
		if f.Managed != "orphan" {
			t.Errorf("managed = %q, want orphan", f.Managed)
		}
		if strings.Contains(f.Detail, "--force") {
			t.Errorf("status advises --force where Apply refuses: %q", f.Detail)
		}
		if !strings.Contains(f.Detail, "refuses") {
			t.Errorf("detail must say sync refuses, got %q", f.Detail)
		}
	}
	if !found {
		t.Fatal("no finding for the non-owned lockfile dir entry; sync will exit 4 on it and status said nothing")
	}
}
```

(Adapt the JSON field names to the real contract if they differ — the goldens in `internal/cli/testdata/status-*.golden.json` are the source of truth.)

- [ ] **Step 3: Run to verify failure.** Pre-fix, the entry either produces a finding advising `--force` (via the edited/unverifiable branch) or one advising plain removal; either fails an assertion above.

- [ ] **Step 4: Implement.** At the top of the orphan-dir loop body, after the `la.Kind != KindDir || desiredDirs[la.Path]` filter:

```go
			// Ownership mirrors Apply's rule (apply.go, orphan-dir removal):
			// decided on the repo-relative path's final element, never the
			// absolute path. Status must never advise `esc sync --force` for
			// an entry Apply refuses to touch: that advice-versus-behavior
			// divergence sends a user to a command that exits 4.
			if !strings.HasPrefix(path.Base(la.Path), "esc-") {
				res.Findings = append(res.Findings, Finding{
					Subject: la.Path, Kind: la.Kind, State: Orphan, Local: LocalNone,
					Detail: "lockfile records a retired skill directory escapement does not own; `esc sync` refuses to remove it. Delete the stale lockfile entry by hand",
				})
				continue
			}
```

Adjust placement relative to the existence check per Step 1's reading of Apply.

- [ ] **Step 5: Run, verify pass, full check, snapshot the diff** (message: `fix(engine): status stops advising --force for dirs Apply refuses to own`).

---

### Task 9: The placement offer confirms what it wrote and says when it understood nothing

Parked residuals 1 and 2 from the 2026-08-08 ledger: a yes rewrites up to four user-owned files and prints nothing; an unrecognized position answer (after an explicit yes) writes nothing and says nothing.

**Files:**
- Modify: `internal/cli/initoffer.go` (`askPlacement` default branch; `offerPlacement` apply loop)
- Test: `internal/cli/initoffer_test.go`

**Interfaces:**
- Consumes: existing seam (`fakeStdin`, `run` in cli_test.go; direct `askPlacement`/`offerPlacement` tests in initoffer_test.go).
- Produces: two new stdout lines; tests asserting exact prior output may need equivalent-rigor updates (note each in the ledger).

- [ ] **Step 1: Write the failing tests:**

```go
func TestOfferConfirmsEachMarkerWrite(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"CLAUDE.md": "# P\n\nrules\n",
		"AGENTS.md": "# P\n\nrules\n",
	})
	fakeStdin(t, "y\ne\n")
	code, out := run(t, root, "init")
	if code != 0 {
		t.Fatalf("init exit %d:\n%s", code, out)
	}
	for _, want := range []string{"CLAUDE.md: wrote", "AGENTS.md: wrote"} {
		if !strings.Contains(out, want) {
			t.Errorf("a write into a user-owned file must be confirmed; missing %q in:\n%s", want, out)
		}
	}
}

func TestUnrecognizedPositionAnswerSaysSo(t *testing.T) {
	var out bytes.Buffer
	br := bufio.NewReader(strings.NewReader("y\nbanana\n"))
	got := askPlacement(&out, br, []Detected{{Path: "CLAUDE.md", Target: "claude"}}, false)
	if got != placeDefault {
		t.Fatalf("unrecognized answer must never guess, got %q", got)
	}
	if !strings.Contains(out.String(), "not one of the options") {
		t.Errorf("silence after an explicit yes is not an answer; output:\n%s", out.String())
	}
}
```

(Adapt the `TestOfferConfirms...` fixture to however sibling init tests set up multiple detected files; use the git-repo fixture if a bare TempDir changes the flow — mirror `TestInitWarnsThereIsNoUndoOutsideAGitRepo`'s setup, which proves a plain TempDir works.)

- [ ] **Step 2: Run to verify both fail.**

- [ ] **Step 3: Implement.** In `askPlacement`'s position-question default branch, before returning:

```go
	default:
		// Never guess; but after an explicit yes, silence reads as success.
		// Say what happened and what was (not) done.
		fmt.Fprintln(w, "That was not one of the options; leaving the files unchanged.")
		return placeDefault
```

In `offerPlacement`'s apply loop:

```go
	for _, d := range eligible {
		if err := applyPlacement(root, d, p); err != nil {
			errs = append(errs, fmt.Errorf("%s: could not write the placement marker: %w", d.Path, err))
			continue
		}
		// One answer just rewrote a user-owned file; a write with no
		// confirmation is indistinguishable from a decline in the output.
		fmt.Fprintf(stdout, "  %s: wrote %s\n", d.Path, render.Placeholder)
	}
```

- [ ] **Step 4: Run the full cli package**, fix any exact-output assertions the two new lines break (equivalent rigor: extend the expected strings, never delete an assertion; record each touched test in the ledger).

- [ ] **Step 5: Full check, snapshot the diff** (message: `fix(cli): confirm marker writes, name an unrecognized position answer`).

---

### Task 10: Two brittle tests report honestly

Parked residuals 3 and 4: `TestReadBoundedLineStopsAtTheBound` hangs (ten-minute panic dump) instead of failing if the bound is removed; `TestInitWarnsThereIsNoUndoOutsideAGitRepo` assumes TMPDIR is not inside a git worktree. `TestFileIsCleanNotAGitRepo` (named in the same ledger entry as carrying the identical assumption) gets the same one-line fix.

**Files:**
- Modify: `internal/cli/initoffer_test.go:329` and `:799`; the same env line in `TestFileIsCleanNotAGitRepo` (locate by name in `internal/cli`).

**Interfaces:** none.

- [ ] **Step 1: Rewrite the bound test:**

```go
// Without the byte bound this returns never; the select converts "never"
// into a red assertion instead of the package timeout's panic dump, which
// reads as infrastructure failure rather than a failed guard.
func TestReadBoundedLineStopsAtTheBound(t *testing.T) {
	done := make(chan string, 1)
	go func() { done <- readBoundedLine(bufio.NewReader(infiniteReader{'y'}), maxOfferAnswerBytes) }()
	select {
	case got := <-done:
		if len(got) != maxOfferAnswerBytes {
			t.Errorf("read %d bytes, want the bound of %d", len(got), maxOfferAnswerBytes)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("readBoundedLine did not return: the byte bound is not enforced")
	}
}
```

(Add `"time"` to imports. The goroutine leaks on timeout; the test is already failing at that point, so the leak is irrelevant.)

- [ ] **Step 2: Pin git out of the temp dirs.** In `TestInitWarnsThereIsNoUndoOutsideAGitRepo` and `TestFileIsCleanNotAGitRepo`, immediately after `root := t.TempDir()`:

```go
	// t.TempDir may itself sit inside a git worktree (TMPDIR under a repo);
	// a ceiling directly above root stops git's upward discovery so this
	// test means "not a git repo" everywhere, not just on machines with a
	// clean TMPDIR. EvalSymlinks because git compares resolved paths and
	// macOS TMPDIR lives behind /var -> /private/var.
	ceiling, err := filepath.EvalSymlinks(filepath.Dir(root))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", ceiling)
```

(`gitCommand` builds env from `os.Environ()`, so `t.Setenv` reaches the subprocess. If `root` itself must be symlink-resolved for the ceiling to bite — verify by running the test with TMPDIR pointed inside this very repo — resolve `root` the same way before use.)

- [ ] **Step 3: Verify honestly.** Run both tests normally (PASS). Then verify the bound test's select works: temporarily change `readBoundedLine`'s loop to `for {`, run, confirm a red assertion (not a hang) within 10s, revert. Then verify the ceiling: `TMPDIR=$(pwd)/.tmptest go test ./internal/cli/ -run 'TestInitWarnsThereIsNoUndoOutsideAGitRepo|TestFileIsCleanNotAGitRepo'` from the repo root (create `.tmptest/` first, delete after) — PASS proves the assumption is gone. Record the TMPDIR run's output in the ledger.

- [ ] **Step 4: Full check, snapshot the diff** (message: `test(cli): bound test fails instead of hanging; git-free tests pin a ceiling`).

---

### Task 11: Copy and docs sweep — the notice, the CHANGELOG label, the cycle's own entries

Parked residuals 5 and 8, plus the CHANGELOG entries for this cycle's user-visible changes. The `notice` const (render.go:29) contains an em-dash and is written into every managed block in every governed repo: the most outward-facing string the product has.

**Files:**
- Modify: `internal/render/render.go:29`; `CHANGELOG.md`; `README.md`; `examples/governed-service/{CLAUDE,AGENTS,GEMINI}.md`
- Regenerate: `internal/render/testdata/{claude,agents,gemini,copilot}.golden.md`; `internal/cli/testdata/status-mixed.golden.json`, `status-altered.golden.json` (and any other report golden the run rewrites)

**Interfaces:**
- Consumes: golden regeneration flags — `go test ./internal/render -run TestGolden -update`; `go test ./internal/cli -update-report-golden` (flag declared report_golden_test.go:16; run the whole package with the flag).
- Produces: new notice text, used verbatim below.

- [ ] **Step 1: Change the notice.** In `internal/render/render.go`:

```go
const notice = "> Managed by escapement. Do not edit. Run `esc diff` to see source. Team content goes outside this block."
```

- [ ] **Step 2: Regenerate goldens.** `go test ./internal/render -run TestGolden -update`, then `go test ./internal/cli -update-report-golden`. Inspect the diff: the ONLY changes must be the notice line and the hashes derived from block bodies containing it. Any other drift is a stop-and-report finding.

- [ ] **Step 3: Update the examples honestly.** The three `examples/governed-service/*.md` files carry real rendered blocks whose `hash=` covers the notice line. Do not hand-compute hashes. Write a TEMPORARY test in `internal/render` (e.g. `regen_examples_test.go`), run it once, then DELETE it:

```go
// Temporary regeneration helper. Not committed: delete after running.
func TestRegenExamples(t *testing.T) {
	for _, f := range []string{"CLAUDE.md", "AGENTS.md", "GEMINI.md"} {
		p := filepath.Join("..", "..", "examples", "governed-service", f)
		orig, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		blk, err := Extract(orig)
		if err != nil || blk == nil {
			t.Fatalf("%s: %v", f, err)
		}
		body := strings.Replace(blk.Body,
			"> Managed by escapement — do not edit.",
			"> Managed by escapement. Do not edit.", 1)
		out, err := Splice(orig, body, BlockMeta{Packs: blk.Packs})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, out, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
```

Verify each example file's diff is exactly the notice line plus the `hash=` field; then delete the helper file. Update the README's notice occurrence by hand (one line).

- [ ] **Step 4: Label the breaking change.** In `CHANGELOG.md`, the `esc status` orphan-dir bullet (line 32) becomes:

```markdown
- **BREAKING for CI gating on `esc status --check`.** `esc status` reports a
  retired esc skill directory as an `orphan` finding, the way it already did
  for an orphaned managed block. A retirement that `esc sync` declines is
  therefore visible to `esc status --check` instead of passing it at exit 0.
  Repos previously green on `--check` while parked in a declined retirement
  now exit 1 there.
```

- [ ] **Step 5: Add this cycle's entries.** Under `## [Unreleased]`:

In `### Fixed` (create the section if absent):

```markdown
- CRLF instruction files: leading YAML frontmatter with CRLF line endings is
  now recognized, so sync's top placement and the `esc init` marker offer
  insert below the fence instead of above it (which silently demoted the
  frontmatter to a setext heading). Escapement still renders LF; a managed
  block converted to CRLF on disk (e.g. by `core.autocrlf`) reports as
  altered, byte-truthfully.
- A whitespace-only `.mcp.json` is treated as empty instead of failing the
  sync with a JSON parse error.
- `esc status` no longer follows a hostile lockfile path out of the repo (or
  through a symlink) when reporting an orphaned managed block, matching the
  containment rules the skill-directory pass already enforced.
- `esc status` no longer advises `esc sync --force` for a lockfile directory
  entry that `esc sync` refuses to touch as not escapement-owned; it now
  reports what sync will actually do.
```

In `### Changed`:

```markdown
- The managed-block notice line reads "Managed by escapement. Do not edit."
  (previously an em-dash). Cosmetic for humans; a hash change for tooling:
  the next `esc sync` rewrites the block, and until then `esc status`
  reports `stale`, which is routine drift.
- `esc init` confirms each marker write ("CLAUDE.md: wrote <!-- escapement:block -->")
  and says so when a position answer is not recognized, instead of writing
  or declining silently.
```

- [ ] **Step 6: Full check.** `go test ./... && go vet ./... && gofmt -l .` — in particular the render and cli golden tests pass WITHOUT their update flags. Confirm `regen_examples_test.go` is deleted (`git status`).

- [ ] **Step 7: Snapshot the diff** (message: `docs: drop the notice em-dash, label the --check breaking change, changelog the cycle`).

---

## Self-Review Notes

- Spec coverage: handoff P1a (AGENTS.md invariant) was applied before this plan was written and is in the working tree; P1b/P1c are Tasks 1-6; every P2 residual maps to Tasks 7-11 (orphan-block read → 7; ownership divergence → 8; CHANGELOG label → 11; missing confirmation → 9; silent unrecognized answer → 9; hanging test → 10; TMPDIR assumption → 10; notice em-dash → 11).
- The CRLF hash papercut (autocrlf-converted blocks report altered and sync's skip gate then requires `--force`) is deliberately OUT of scope, documented in the Task 11 changelog entry, and goes to the owner at check-in.
- The catalog separator `" — "` in `composeCatalog` (render.go:127) also ships inside managed blocks; the handoff names only the notice const, so it is NOT changed here and is flagged for the owner at check-in.
- Type consistency: `shapetest.Shape` fields and function signatures are quoted identically in Tasks 1, 3, 4, 6; `MergeMCP`/`mergeDir` signatures match the survey's exact readings.
