# EOL-Normalized Managed Hashing + Catalog Separator Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A `core.autocrlf` checkout stops reporting pristine managed content as `altered` (blocks AND the whole-file GOVERNANCE.md target), and the catalog notes separator drops its em-dash for whichever of `" - "`/`": "` the conventions survey recommends.

**Architecture:** Line endings are presentation owned by git and editors, not policy content; the MANAGED-axis comparison normalizes `\r\n` to `\n` before hashing, while rendering stays LF-only and byte-exact everywhere else. One normalization function lives in `internal/render`; `render.BodyHash` applies it (marker-hash generation is unaffected since rendered bodies are already LF), `render.Extract` learns CRLF marker-line boundaries, and `classify`'s KindFile case normalizes before hashing. Owner-approved reversal of the previous cycle's "byte-exact hashing stays" ruling, decided 2026-08-09.

**Tech Stack:** Go 1.24, stdlib only.

## Global Constraints

- Single external dependency policy: stdlib only.
- `go test ./... && go vet ./... && gofmt -l .` clean before any task is claimed done.
- Rendering NEVER emits CRLF; normalization exists only on the comparison path. For all-LF repos every hash is byte-identical to today (normalization is an identity), so no lockfile or golden invalidation from Tasks A/B.
- Accepted trade, stated in the CHANGELOG: a hand edit that ONLY changes line endings is no longer reported. That is the point, not a regression.
- A genuine content edit combined with CRLF conversion must STILL report altered (test-pinned).
- Skill directories (KindDir) and `.mcp.json` are OUT of scope: `.mcp.json` is already immune (JSON parse tolerates `\r\n` before canonical hashing); skill-dir files can be arbitrary content where blind `\r\n` rewriting is wrong. Skill-dir exposure is noted for the owner, not fixed here.
- Byte-preservation-plus-structure invariant (AGENTS.md) binds as always; matrix failures are findings, never assertion adjustments.
- No em-dashes in new prose or rendered output.

---

### Task A: render — CRLF-tolerant Extract boundaries and normalized BodyHash

**Files:**
- Modify: `internal/render/block.go` (`BodyHash` :31, `Extract` bodyStart :157 and end :169)
- Test: `internal/render/block_test.go`; `internal/shapetest/shapetest.go` (one new catalog shape)

**Interfaces:**
- Produces: `func NormalizeEndings(b []byte) []byte` (exported from render, `bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))`; lone `\r` untouched, closed rule set). `BodyHash(body string) string` now `esc.HashBytes([]byte(strings.ReplaceAll(body, "\r\n", "\n")))` with a doc comment carrying the presentation-vs-content ruling. Task B consumes `NormalizeEndings`.

- [ ] **Step 1: Failing test.** In block_test.go:

```go
// A managed block converted wholesale to CRLF (core.autocrlf on a Windows
// checkout) must still extract, and its body must hash equal to the LF
// hash in its own begin marker: line endings are git's presentation, not
// policy content, so the managed axis must not report this as drift.
func TestExtractCRLFConvertedBlockHashesEqual(t *testing.T) {
	lf, err := Splice(nil, "policy line\n", BlockMeta{Packs: []string{"p@1"}})
	if err != nil {
		t.Fatal(err)
	}
	crlf := bytes.ReplaceAll(lf, []byte("\n"), []byte("\r\n"))
	blk, err := Extract(crlf)
	if err != nil || blk == nil {
		t.Fatalf("Extract on CRLF block: blk=%v err=%v", blk, err)
	}
	if BodyHash(blk.Body) != blk.Hash {
		t.Errorf("CRLF-converted body hashes %q, marker says %q; a pristine autocrlf checkout would report altered", BodyHash(blk.Body), blk.Hash)
	}
	// The normalization must not mask a real edit under CRLF endings.
	edited := bytes.ReplaceAll(crlf, []byte("policy"), []byte("edited"))
	eb, err := Extract(edited)
	if err != nil || eb == nil {
		t.Fatal("Extract on edited CRLF block failed")
	}
	if BodyHash(eb.Body) == eb.Hash {
		t.Error("a genuine edit under CRLF endings must still mismatch")
	}
}
```

- [ ] **Step 2: Observe failure** (`go test ./internal/render/ -run TestExtractCRLF`): pre-fix, body carries a leading `\r\n` (bodyStart's `\n` check misses) and per-line `\r`s, so the first assertion fails.

- [ ] **Step 3: Implement.** In Extract, where bodyStart consumes the newline after the begin marker and where b.end consumes the newline after the end marker, accept `\r\n` before `\n` (check `"\r\n"` FIRST). Add `NormalizeEndings`, make `BodyHash` normalize. Doc comments carry the ruling, not the mechanics.

- [ ] **Step 4: Matrix shape.** In shapetest's catalog add `{Name: "only-managed-block-crlf", Content: <RenderedBlock("old body\n") with \n→\r\n>, HasBlock: true}` (build via `bytes.ReplaceAll` on `RenderedBlock`, no new helper). The existing Splice matrix picks it up automatically: replacement in place, LF re-render, idempotence. Run the render and cli matrices; any failure is a finding.

- [ ] **Step 5: Full check.** Goldens must pass WITHOUT update flags (LF identity). Then snapshot/commit per session rules: `fix(render): line endings are presentation, not managed drift`.

---

### Task B: engine — KindFile normalization, classification tests, CHANGELOG truth

**Files:**
- Modify: `internal/engine/status.go` (KindFile case, `actual := esc.HashBytes(content)`)
- Test: `internal/engine/classify_alteration_test.go` (or the file whose fixtures fit; follow its existing pattern)
- Modify: `CHANGELOG.md` (the CRLF bullet added by the previous cycle states the OLD behavior and must be rewritten)

**Interfaces:**
- Consumes: `render.NormalizeEndings` from Task A.

- [ ] **Step 1: Failing tests.** Following classify_alteration_test.go's on-disk fixture pattern: (1) a synced governed repo whose CLAUDE.md is converted wholesale to CRLF classifies the block artifact `in-sync`; (2) GOVERNANCE.md converted to CRLF classifies `in-sync`; (3) GOVERNANCE.md converted AND content-edited classifies `altered`. If the existing fixtures drive `classify` directly, construct content via `render` calls and conversion, never hand-typed hashes.

- [ ] **Step 2: Observe** (1) and (2) fail pre-fix; (3) passes both sides.

- [ ] **Step 3: Implement.** KindFile: `actual := esc.HashBytes(render.NormalizeEndings(content))`. One comment line: same ruling as `render.BodyHash`, whole-file targets included. KindBlock needs no change (BodyHash carries it) — verify, don't duplicate normalization.

- [ ] **Step 4: Check Apply's write behavior for in-sync artifacts** and REPORT it in your task report (does sync skip writes when classify says in-sync, or write unconditionally?). Do not change Apply either way; the goal of this task is classification truth. If Apply rewrites unconditionally, note it as a residual for the ledger.

- [ ] **Step 5: CHANGELOG.** Rewrite the clause "a managed block converted to CRLF on disk (e.g. by `core.autocrlf`) reports as altered, byte-truthfully" to state the new behavior: CRLF-converted managed content (blocks and `GOVERNANCE.md`) classifies in sync because line endings are presentation; a hand edit that only changes line endings is therefore no longer reported; a real edit under CRLF still is. No em-dashes.

- [ ] **Step 6: Full check, snapshot/commit:** `fix(engine): GOVERNANCE.md joins the endings-normalized managed axis`.

---

### Task C: catalog separator (BLOCKED on the conventions survey verdict)

**Files:**
- Modify: `internal/render/render.go:127` (`composeCatalog`, the `" — " + e.Notes` join)
- Regenerate: `internal/render/testdata/*.golden.md` that carry catalog notes; `internal/cli/testdata/status-*.golden.json` if affected; `examples/governed-service/*.md` IF the demo pack renders catalog notes (check; use the Task 11 Extract+Splice regen approach from the previous plan if so)
- Modify: `CHANGELOG.md` (one bullet, hash-change note same as the notice change)

- [ ] **Step 1:** Fill in the separator from the survey verdict (owner accepts `" - "` or `": "`).
- [ ] **Step 2:** Change the join, regenerate goldens, inspect the diff: separator and derived hashes ONLY.
- [ ] **Step 3:** Check examples for catalog notes; regenerate the same way as the notice change if present.
- [ ] **Step 4:** CHANGELOG bullet; full check; snapshot/commit: `docs(render): catalog notes drop the em-dash separator`.

## Self-Review Notes

- The previous cycle's "byte-exact hashing stays" was a planning-time ruling, reversed here explicitly by the owner (2026-08-09, "lets get GOVERNANCE.md too"). Recorded so no reviewer flags the contradiction.
- KindDir skill files share the autocrlf exposure and are deliberately excluded (arbitrary content); surfaced to the owner in the final summary.
- Orphan-block status pass and sync's skip gate both reach hashes through `render.BodyHash`/`classify`, so Task A/B cover them without extra edits — reviewers should verify rather than trust this claim.
