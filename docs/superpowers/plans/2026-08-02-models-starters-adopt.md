# Model Starters and Adopt Flow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Adopt the goldmark markdown library for the portal's display rendering (gaining safe clickable links), ship per-model starter rule-pack fragments for the full-depth vendors, and let a user adopt a starter into a writable rule pack through the existing publish pipeline.

**Architecture:** Three layers, all extending code merged to `main`. (1) `mdHTML` in `internal/portal/web/markdown.go` is reimplemented as a thin wrapper over goldmark (raw HTML disabled, Linkify scoped to http/https, a heading-offset transformer preserving the page's single `<h1>`). (2) `internal/guidance` gains an optional `Model.Starter` field validated against the embedded tree, plus ten authored starter fragments. (3) The portal renders starters on vendor pages and adds a `GET`/`POST /models/{vendor}/adopt` flow backed by a new `publish.Manager.AddFragment` method that adds a brand-new fragment to a pack's manifest, bumps the version, validates, and commits+tags.

**Tech Stack:** Go 1.24 stdlib, `gopkg.in/yaml.v3`, `github.com/yuin/goldmark` v1.8.5 (zero transitive dependencies, display-only), `html/template`, system `git` via `os/exec`, htmx (already vendored; the adopt flow is a plain full-page form).

## Global Constraints

- Go 1.24, module `github.com/tensorgroup/openescapement`, binary `esc`.
- Single external dependency policy: `gopkg.in/yaml.v3` for the engine/renderer, plus one display-only exception, `github.com/yuin/goldmark` (Task 1 amends AGENTS.md to justify it). Everything else is stdlib; system `git` via `os/exec`. No further dependencies are in scope.
- Sentinel errors in `internal/esc` map to exit codes (0 ok, 1 drift/constraint, 2 usage, 3 integrity/signature, 4 other); reuse `esc.ErrManifest` for pack validation failures — do not invent new ones.
- Renderer/publish invariants, all golden-testable: bytes outside a managed block are never modified; nothing is written after a verification or constraint failure; all writes are atomic (temp + rename for guidance; git restore-on-failure for packs); output is deterministic. goldmark runs with raw HTML disabled (never `html.WithUnsafe`) and only in the display path, never in the engine that writes instruction files.
- Guidance content is fail-soft: a broken hand-edited content file must never take the portal down; fall back to the embedded copy and surface a banner.
- The guidance editable file set is fixed by the registry; the portal never creates a path from unchecked user input.
- No em-dashes and no AI telltales in any authored prose (starters, notes, AGENTS.md line, CHANGELOG, code comments). Outward-facing text stays concise.
- No AI co-authorship trailers in commit messages. Commit messages are conventional-commit style, drafted from the diff.
- Privacy-respecting: no prompt-content collection. YAGNI: lightweight over complete.
- Verification gates before any task is "done": `gofmt -w .`, `go vet ./...`, `go test ./...` all clean. `ESC_CACHE_DIR` and `t.TempDir()` isolation as the existing tests use them.

---

## File Structure

| File | Responsibility | Tasks |
|------|----------------|-------|
| `AGENTS.md` | single-dependency policy gains a justified goldmark exception | 1 |
| `go.mod`, `go.sum` | pin `github.com/yuin/goldmark` v1.8.5 | 1 |
| `internal/portal/web/markdown.go` | `mdHTML` reimplemented over goldmark (wrapper + transformer + front-matter strip) | 1 |
| `internal/portal/web/markdown_test.go` | `TestMdHTML` (kept) + `TestMdHTMLLinks` (behavior) | 1 |
| `internal/guidance/models.go` | `Model.Starter` field + validation | 2 |
| `internal/guidance/models_test.go` | starter validation unit test | 2 |
| `internal/guidance/models/examples/{vendor}/starter-*.md` | 10 authored starter fragments | 3 |
| `internal/guidance/models/models.yaml` | `starter:` paths on current full-depth models | 3 |
| `internal/guidance/checklist_test.go` | every current full-depth model has a starter | 3 |
| `internal/portal/publish/publish.go` | `AddFragment`, `ErrFragmentExists`, `addRuleToManifest`, extracted `commitAndTag` | 4 |
| `internal/portal/publish/publish_adopt_test.go` | AddFragment tests | 4 |
| `internal/portal/web/models.go` | starter display on vendor page; adopt GET/POST handlers | 5, 6 |
| `internal/portal/web/templates/model_vendor.html` | starter block + adopt button | 5 |
| `internal/portal/web/templates/model_adopt.html` | adopt form + preview | 6 |
| `internal/portal/web/server.go` | routes + `pageNames` registration | 6 |
| `internal/portal/web/models_test.go` | vendor-page + adopt tests; combined fixture | 5, 6 |
| `internal/portal/web/degrade_test.go` | adopt GET full-page gate | 6 |
| `CHANGELOG.md` | Unreleased bullet | 7 |

---

## Task 1: Adopt goldmark for portal markdown rendering

**Files:**
- Modify: `AGENTS.md` (single-dependency policy: add a justified goldmark exception sub-bullet)
- Modify: `go.mod`, `go.sum` (pin `github.com/yuin/goldmark`)
- Modify: `internal/portal/web/markdown.go` (replace the hand-rolled converter with a goldmark wrapper + heading transformer + front-matter strip)
- Test: `internal/portal/web/markdown_test.go` (keep `TestMdHTML`, add `TestMdHTMLLinks`)

**Interfaces:**
- Consumes: goldmark packages `github.com/yuin/goldmark`, `.../extension`, `.../ast`, `.../parser`, `.../text`, `.../util`.
- Produces: `mdHTML(src []byte) template.HTML` — unchanged signature; now goldmark-backed. Removes `inlineMd`, `isMdBlockStart`, `mdCodeRE`, `mdBoldRE` (goldmark replaces them). All existing `mdHTML` callers (`models.go`, `server.go` pack detail) are unaffected.

**Key decisions (stated explicitly, per the coordinator's requirements):**
- **Safety / hostile schemes:** we use `goldmark.New(...)` with the default `renderer/html` (Unsafe=false — we never pass `html.WithUnsafe`). That renderer escapes all raw source HTML and filters dangerous link destinations via `html.IsDangerousURL` (the entity-encoded-scheme bypass was CVE-2026-5160 / GHSA-c97m-vxhj-p7j6, fixed in 1.7.17; v1.8.5 is past it). A `[x](javascript:...)` link renders with an empty `href` and no `javascript:` text.
- **Autolinking:** bare URLs are linked only with the Linkify extension; we scope it to http/https via `extension.WithLinkifyAllowedProtocols([][]byte{[]byte("http"), []byte("https")})`.
- **`rel="noreferrer"`:** goldmark does not emit `rel` on links or autolinks. To preserve the spec requirement we inject it by a single post-render string replace of `<a href=` with `<a rel="noreferrer" href=`. This is safe: goldmark's safe renderer always emits `<a href="` first and has already escaped/filtered the destination. No spec deviation.
- **Heading offset (output drift):** the old converter mapped `#`→h2, `##`→h3, `###`→h4 (the page `<h1>` comes from the layout title). goldmark maps `#`→h1. We add a small AST transformer that bumps every heading one level, reproducing the old mapping and preserving the single-`<h1>` accessibility property. Chosen over relaxing tests/CSS.
- **Front-matter:** pack and example/starter fragments begin with a `---\n...\n---\n` YAML block; goldmark would render that as a thematic break plus a setext heading. We strip a leading front-matter block before rendering so metadata is not shown as body. Vendor notes have no front-matter and pass through unchanged.

- [ ] **Step 1: Amend the dependency policy in AGENTS.md, then commit it**

In `AGENTS.md`, under "Gotchas and invariants", immediately after the existing bullet that begins `- **Single external dependency policy:**` (ending "Adding a dependency needs explicit justification."), add this indented sub-bullet (no em-dashes, keeps the policy's security-posture spirit):

```markdown
  - Exception (display-only): `github.com/yuin/goldmark` renders markdown for the admin portal's browser views. Scope is strictly display: it never runs in the renderer or engine that writes instruction files. It carries zero transitive dependencies (empty require graph), is pinned by `go.sum`, and runs with raw HTML disabled (no `html.WithUnsafe`). Any wider use, or any second display dependency, needs the same explicit justification.
```

Commit the policy amendment on its own (a pure-docs change, reviewable independently; the module manifest lands with the importing code in the next step so `go mod tidy` never sees an unused require):

```bash
! grep -n '—' AGENTS.md
git add AGENTS.md
git commit -m "docs: justify goldmark as the portal markdown renderer dependency"
```

- [ ] **Step 2: Add the dependency**

Run: `go get github.com/yuin/goldmark@v1.8.5`
Then verify it resolved at or above the XSS fix:

Run: `go list -m github.com/yuin/goldmark`
Expected: `github.com/yuin/goldmark v1.8.5` (or newer). If `go get @latest` resolves higher, that is fine as long as it is `>= v1.7.17`; record the resolved version. Confirm `go.mod`'s `require github.com/yuin/goldmark` is the only new line and no transitive requires were added.

- [ ] **Step 3: Write the failing behavior test**

Add to `internal/portal/web/markdown_test.go` (keep the existing `TestMdHTML` as-is):

```go
func TestMdHTMLLinks(t *testing.T) {
	cases := []struct {
		name, in       string
		want, notWant  []string
	}{
		{"markdown link", "See [docs](https://a.com/x) here.",
			[]string{`href="https://a.com/x"`, `>docs</a>`, `rel="noreferrer"`}, nil},
		{"bare url autolink", "Visit https://a.com/y now",
			[]string{`href="https://a.com/y"`}, nil},
		{"ampersand url", "Query https://a.com/s?x=1&y=2 end",
			[]string{`href="https://a.com/s?x=1&amp;y=2"`}, nil},
		{"hostile scheme not clickable", "Click [x](javascript:alert(1)) please",
			nil, []string{"javascript"}},
		{"url in code span not linkified", "Run `https://a.com/z` today",
			[]string{"<code>https://a.com/z</code>"}, []string{"<a "}},
		{"bold and link", "**note** https://a.com/w",
			[]string{"<strong>note</strong>", `href="https://a.com/w"`}, nil},
		{"link in list item", "- see [docs](https://a.com/l)",
			[]string{"<li>", `href="https://a.com/l"`}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := string(mdHTML([]byte(c.in)))
			for _, w := range c.want {
				if !strings.Contains(got, w) {
					t.Fatalf("missing %q in:\n%s", w, got)
				}
			}
			for _, nw := range c.notWant {
				if strings.Contains(got, nw) {
					t.Fatalf("unexpected %q in:\n%s", nw, got)
				}
			}
		})
	}
}
```

- [ ] **Step 4: Run to verify it fails**

Run: `go test ./internal/portal/web/ -run TestMdHTMLLinks -v`
Expected: FAIL (the hand-rolled `mdHTML` renders no anchors).

- [ ] **Step 5: Reimplement `markdown.go` over goldmark**

Replace the entire contents of `internal/portal/web/markdown.go` with:

```go
package web

import (
	"bytes"
	"html/template"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// md is the shared, safe markdown renderer for portal display. It uses
// goldmark's default renderer/html with Unsafe left off, so raw source HTML is
// escaped and dangerous link schemes (javascript:, data:, ...) are filtered by
// html.IsDangerousURL. Linkify autolinks bare URLs, restricted to http and
// https. A heading offset maps '#' to <h2> so the layout's <h1> stays the
// page's only top-level heading.
var md = goldmark.New(
	goldmark.WithExtensions(
		extension.NewLinkify(
			extension.WithLinkifyAllowedProtocols([][]byte{[]byte("http"), []byte("https")}),
		),
	),
	goldmark.WithParserOptions(
		parser.WithASTTransformers(util.Prioritized(headingOffset{}, 100)),
	),
)

// headingOffset bumps every heading one level deeper (# -> h2, ## -> h3, ...)
// so rendered guidance and fragment content never emits a competing <h1>.
type headingOffset struct{}

func (headingOffset) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if h, ok := n.(*ast.Heading); ok && h.Level < 6 {
				h.Level++
			}
		}
		return ast.WalkContinue, nil
	})
}

// mdHTML renders a safe subset of markdown to HTML for portal display. Raw
// source HTML is escaped; bare http/https URLs and [text](url) links become
// anchors carrying rel="noreferrer"; dangerous-scheme link targets are
// filtered by goldmark's default (safe) renderer. A leading YAML front-matter
// block is stripped so fragment metadata is not rendered as body.
func mdHTML(src []byte) template.HTML {
	var buf bytes.Buffer
	if err := md.Convert(stripFrontmatter(src), &buf); err != nil {
		// Convert only errors on a writer failure; fall back to escaped source.
		return template.HTML(template.HTMLEscapeString(string(src)))
	}
	// goldmark does not emit rel on anchors; add it uniformly. The safe
	// renderer always writes `<a href="` first and has already escaped and
	// scheme-filtered the destination, so this replace cannot inject markup.
	out := strings.ReplaceAll(buf.String(), `<a href=`, `<a rel="noreferrer" href=`)
	return template.HTML(out)
}

// stripFrontmatter removes a leading "---\n...\n---\n" YAML block, matching how
// pack fragments and guidance examples carry their targets metadata. Content
// without a leading front-matter block is returned unchanged.
func stripFrontmatter(src []byte) []byte {
	s := string(src)
	if !strings.HasPrefix(s, "---\n") {
		return src
	}
	rest := s[len("---\n"):]
	if i := strings.Index(rest, "\n---\n"); i >= 0 {
		return []byte(rest[i+len("\n---\n"):])
	}
	return src
}
```

- [ ] **Step 6: Confirm no dead references remain**

Run: `grep -rn 'inlineMd\|isMdBlockStart\|mdCodeRE\|mdBoldRE' internal/`
Expected: no matches (all four were internal to the old `markdown.go` and are now removed). If any match appears, it is a stale reference to fix.

- [ ] **Step 7: Run the affected suites**

Run: `go test ./internal/portal/web/ -run 'TestMdHTML|TestMdHTMLLinks|TestPacksListAndDetail|TestModelVendorPageRendersNoteModelsExamplesAnchors' -v`
Expected: PASS.

Enumeration of existing assertions the executor must verify (re-checked against current test bodies; NONE require edits, because the heading offset reproduces the old h2/h3/h4 mapping and front-matter stripping keeps `# Security`/`# Anthropic` mapping intact):
- `internal/portal/web/markdown_test.go` `TestMdHTML`: `<h2>Title</h2>` (from `#` offset), `<strong>bold</strong>`, `<code>code</code>`, `<li>a</li>`, `<pre><code>x &lt; y`, `&lt;script&gt;`, and NOT `<script>` — all still hold under goldmark (raw HTML escaped, no `WithUnsafe`). No change.
- `internal/portal/web/packs_test.go` `TestPacksListAndDetail`: `<h2>Security</h2>` — `rules/security.md`'s `# Security` after front-matter strip and offset renders `<h2>Security</h2>`. No change.
- `internal/portal/web/models_test.go` `TestModelVendorPageRendersNoteModelsExamplesAnchors`: assertions are on template output (`id="claude-opus-5"`, edit links, doc host, `<pre>` raw) and `hx-disable`, not on rendered heading tags. No change.
- `internal/portal/web/packs_test.go` `TestDiffColoring` / `TestDiffPreview*`: unaffected (diff rendering does not use `mdHTML`). No change.
The only test delta is the new `TestMdHTMLLinks` added in Step 3.

- [ ] **Step 8: Full gate + commit (code + module manifest together)**

```bash
gofmt -w internal/portal/web/markdown.go internal/portal/web/markdown_test.go
go mod tidy
go vet ./... && go test ./...
git add go.mod go.sum internal/portal/web/markdown.go internal/portal/web/markdown_test.go
git commit -m "feat(portal): adopt goldmark for markdown rendering"
```

(Commit choice: two commits. Commit one is the AGENTS.md policy amendment alone, so the dependency decision is reviewable on its own. Commit two carries `go.mod`/`go.sum` together with the code that imports goldmark, so `go mod tidy` never sees an unused require.)

---

## Task 2: Optional per-model `Starter` field with embedded-file validation

**Files:**
- Modify: `internal/guidance/models.go` (add `Starter` to `Model`; validate in `ParseRegistry`)
- Test: `internal/guidance/models_test.go` (add `TestParseRegistryValidatesStarter`)

**Interfaces:**
- Consumes: `embeddedModels embed.FS` (already declared in `models.go`).
- Produces: `Model.Starter string` (yaml key `starter`, omitempty). `ParseRegistry` now errors if a set `starter` names a file absent from the embedded tree (`models/<starter>`).

**Ordering note:** This task adds the field and validation plus a negative unit test only. It does NOT add the "every current model has a starter" checklist — that lands in Task 3 together with the starter content, so the suite stays green (no starters exist yet).

- [ ] **Step 1: Write the failing test**

Add to `internal/guidance/models_test.go`:

```go
func TestParseRegistryValidatesStarter(t *testing.T) {
	missing := "vendors:\n  - key: anthropic\n    name: A\n    models: [{id: x, name: X, tier: mid, roles: [coding], status: current, docs: [], verified: 2026-08-02, starter: examples/anthropic/does-not-exist.md}]\n"
	if _, err := ParseRegistry([]byte(missing)); err == nil {
		t.Fatal("starter naming a missing embedded file must be rejected")
	}
	unset := "vendors:\n  - key: anthropic\n    name: A\n    models: [{id: x, name: X, tier: mid, roles: [coding], status: current, docs: [], verified: 2026-08-02}]\n"
	if _, err := ParseRegistry([]byte(unset)); err != nil {
		t.Fatalf("unset starter must be fine: %v", err)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/guidance/ -run TestParseRegistryValidatesStarter -v`
Expected: FAIL (unknown field `starter` under `KnownFields(true)`, so `missing` errors but `unset` also errors — the test's second half fails, confirming the field is unrecognized).

- [ ] **Step 3: Add the field and validation**

In `internal/guidance/models.go`, add the field to `Model` (after `Verified`):

```go
	Verified string   `yaml:"verified"`
	Starter  string   `yaml:"starter,omitempty"`
```

In `ParseRegistry`, inside the per-model loop, after the `time.Parse(... m.Verified ...)` block and before the loop closes, add:

```go
			if m.Starter != "" {
				if _, err := embeddedModels.ReadFile("models/" + m.Starter); err != nil {
					return nil, fmt.Errorf("guidance: model %s: starter %q not found in embedded tree", m.ID, m.Starter)
				}
			}
```

- [ ] **Step 4: Run it to verify it passes**

Run: `go test ./internal/guidance/ -run TestParseRegistryValidatesStarter -v`
Expected: PASS.

- [ ] **Step 5: Full gate + commit**

```bash
gofmt -w internal/guidance/models.go internal/guidance/models_test.go
go vet ./... && go test ./internal/guidance/
git add internal/guidance/models.go internal/guidance/models_test.go
git commit -m "feat(guidance): optional per-model starter field with embedded-file validation"
```

---

## Task 3: Per-model starter rule-pack fragments

**Files:**
- Create: 10 files under `internal/guidance/models/examples/{vendor}/starter-<id>.md`
- Modify: `internal/guidance/models/models.yaml` (add `starter:` to each current full-depth model)
- Test: `internal/guidance/checklist_test.go` (add `TestEveryCurrentFullDepthModelHasStarter`)

**Interfaces:**
- Consumes: `Model.Starter` (Task 2), `Set.ReadFile` (disk-first, embedded fallback), `VendorOrder`.
- Produces: seedable/editable starter fragments; registry entries pointing at them.

**Content contract (quality bar for every starter):**
1. A valid rule-pack fragment: front-matter `targets:` matching that vendor's existing `model-routing.md` example — Anthropic `[claude, agents]`, OpenAI `[agents]`, Google `[gemini, agents]` — then a `# ...` heading, then governance bullets.
2. Prose derives only from the already-verified vendor notes (`anthropic.md`, `openai.md`, `google.md`) — no new live research. Each bullet is grounded in a fact from the note (routing position, price/context, review-gate for frontier models, cost caution). Model ids in backticks.
3. Ends with exactly one pointer line to its vendor guidance page for sources.
4. No em-dashes, no AI telltales.

**Coverage (verified from `models.yaml`): the ten `status: current` models of the three full-depth vendors.** Legacy (`gpt-5.2`, `gemini-3-pro`) and sparse vendors (Kimi/Deepseek/Grok) get none.

- [ ] **Step 1: Write the failing checklist test**

Add to `internal/guidance/checklist_test.go`:

```go
func TestEveryCurrentFullDepthModelHasStarter(t *testing.T) {
	s := Load("") // embedded
	fullDepth := map[string]bool{"anthropic": true, "openai": true, "google": true}
	for _, v := range s.Registry.Vendors {
		if !fullDepth[v.Key] {
			continue
		}
		for _, m := range v.Models {
			if m.Status != "current" {
				continue
			}
			if m.Starter == "" {
				t.Errorf("current model %s (%s) has no starter", m.ID, v.Key)
				continue
			}
			if _, _, err := s.ReadFile(m.Starter); err != nil {
				t.Errorf("starter %q for %s not readable: %v", m.Starter, m.ID, err)
			}
		}
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/guidance/ -run TestEveryCurrentFullDepthModelHasStarter -v`
Expected: FAIL (no starters set).

- [ ] **Step 3: Author the ten starter files**

Fully-worked reference example — `internal/guidance/models/examples/anthropic/starter-claude-sonnet-5.md`:

```markdown
---
targets: [claude, agents]
---
# Claude Sonnet 5 governance (claude-sonnet-5)

- Route day-to-day coding to `claude-sonnet-5`: near-frontier coding quality at a fraction of Claude Opus 5's cost (1M-token context, $3 / $15 per million input/output tokens).
- Also use it for review on lower-stakes diffs where a full frontier pass is not warranted; escalate risky or multi-file diffs to `claude-opus-5`.
- Keep planning and plan-checking on the frontier tier (`claude-opus-5`), not on Sonnet 5.
- Apply the org's standard review policy to Sonnet 5 diffs; reserve the mandatory human review gate for high-autonomy frontier-model changes.

See the Anthropic model guidance page for sources and full governance notes.
```

Create the remaining nine to the same bar. Exact content:

`internal/guidance/models/examples/anthropic/starter-claude-opus-5.md`:
```markdown
---
targets: [claude, agents]
---
# Claude Opus 5 governance (claude-opus-5)

- Use `claude-opus-5` as the default frontier model for planning, plan-checking, and reviewing multi-file changes (1M-token context, $5 / $25 per million input/output tokens).
- Do not route routine day-to-day coding here; send that to `claude-sonnet-5` and reserve Opus 5 for work that needs its reasoning depth.
- Escalate to `claude-fable-5` only when Opus 5 genuinely stalls: Fable 5 costs double per token and runs slower.
- Require a human review gate before merging any change Opus 5 produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the Anthropic model guidance page for sources and full governance notes.
```

`internal/guidance/models/examples/anthropic/starter-claude-fable-5.md`:
```markdown
---
targets: [claude, agents]
---
# Claude Fable 5 governance (claude-fable-5)

- Reserve `claude-fable-5` for the hardest problems `claude-opus-5` cannot close: it is priced at $10 / $50 per million tokens, double Opus 5, and slower in practice.
- Do not make Fable 5 the default frontier choice for routine planning or review; that burns budget with no quality gain over Opus 5.
- Keep day-to-day coding on `claude-sonnet-5` and bulk work on `claude-haiku-4-5`.
- Require a human review gate before merging any change Fable 5 produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the Anthropic model guidance page for sources and full governance notes.
```

`internal/guidance/models/examples/anthropic/starter-claude-haiku-4-5.md`:
```markdown
---
targets: [claude, agents]
---
# Claude Haiku 4.5 governance (claude-haiku-4-5)

- Route bulk, mechanical, and high-volume work to `claude-haiku-4-5`: classification, extraction, mechanical edits, and simple agentic subtasks run at scale ($1 / $5 per million tokens, 200K-token context).
- Do not route multi-step reasoning to it; it is the only current Claude model without adaptive thinking.
- Send day-to-day coding to `claude-sonnet-5` and planning or review to `claude-opus-5`.
- It is roughly 5x cheaper than Sonnet 5 on output tokens, so prefer it for repetitive work to hold spend down.

See the Anthropic model guidance page for sources and full governance notes.
```

`internal/guidance/models/examples/openai/starter-gpt-5.6-sol.md`:
```markdown
---
targets: [agents]
---
# GPT-5.6 Sol governance (gpt-5.6-sol)

- Use `gpt-5.6-sol` for planning, plan-checking, and reviewing multi-file changes (1.05M-token context, $5 / $30 per million input/output tokens).
- Reserve it for problems that need its full reasoning depth rather than routine coding; send day-to-day coding to `gpt-5.6-terra`.
- Run bulk and mechanical work on `gpt-5.6-luna`.
- Require a human review gate before merging any change Sol produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the OpenAI model guidance page for sources and full governance notes.
```

`internal/guidance/models/examples/openai/starter-gpt-5.6-terra.md`:
```markdown
---
targets: [agents]
---
# GPT-5.6 Terra governance (gpt-5.6-terra)

- Route day-to-day coding to `gpt-5.6-terra`: competitive with the prior GPT-5.5 flagship at well under half the cost ($2 / $12 per million tokens, 1.05M-token context).
- Also use it for review on lower-stakes diffs where a full `gpt-5.6-sol` pass is not warranted.
- Keep planning and plan-checking on `gpt-5.6-sol`, and run bulk work on `gpt-5.6-luna`.
- Migrate any configuration still pointing at the legacy `gpt-5.2` to Terra or Luna.

See the OpenAI model guidance page for sources and full governance notes.
```

`internal/guidance/models/examples/openai/starter-gpt-5.6-luna.md`:
```markdown
---
targets: [agents]
---
# GPT-5.6 Luna governance (gpt-5.6-luna)

- Route bulk, mechanical, and high-volume work to `gpt-5.6-luna`: classification, extraction, mechanical edits, and simple agentic subtasks run at scale ($0.20 / $1.20 per million tokens, 1.05M-token context).
- It is the cheapest tier in the GPT-5.6 family, roughly 25x cheaper than Sol and 10x cheaper than Terra on output tokens.
- Send day-to-day coding to `gpt-5.6-terra` and planning or review to `gpt-5.6-sol`.
- Migrate any configuration still pointing at the legacy `gpt-5.2` to Luna or Terra.

See the OpenAI model guidance page for sources and full governance notes.
```

`internal/guidance/models/examples/google/starter-gemini-3.1-pro-preview.md`:
```markdown
---
targets: [gemini, agents]
---
# Gemini 3.1 Pro Preview governance (gemini-3.1-pro-preview)

- Use `gemini-3.1-pro-preview` for planning, plan-checking, and reviewing multi-file changes (1,048,576-token input context, $2 / $12 per million input/output tokens for prompts up to 200K tokens).
- Treat its preview label as a production caveat, not a reason to avoid it; it is Google's current frontier model.
- Send day-to-day coding to `gemini-3.6-flash` and bulk work to `gemini-3.5-flash-lite`.
- Require a human review gate before merging any change it produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the Google model guidance page for sources and full governance notes.
```

`internal/guidance/models/examples/google/starter-gemini-3.6-flash.md`:
```markdown
---
targets: [gemini, agents]
---
# Gemini 3.6 Flash governance (gemini-3.6-flash)

- Route day-to-day coding to `gemini-3.6-flash`: sustained frontier-level intelligence at Flash speed and cost, tuned for rapid iterate-test-fix cycles ($1.50 / $7.50 per million tokens, 1,048,576-token input context).
- Also use it for review on lower-stakes diffs where a full `gemini-3.1-pro-preview` pass is not warranted.
- Keep planning and plan-checking on `gemini-3.1-pro-preview`, and run bulk work on `gemini-3.5-flash-lite`.
- It supersedes the older `gemini-3.5-flash` as the coding default in this registry.

See the Google model guidance page for sources and full governance notes.
```

`internal/guidance/models/examples/google/starter-gemini-3.5-flash-lite.md`:
```markdown
---
targets: [gemini, agents]
---
# Gemini 3.5 Flash-Lite governance (gemini-3.5-flash-lite)

- Route bulk, mechanical, and high-volume work to `gemini-3.5-flash-lite`: sub-agent tasks, document parsing, and simple extraction where latency and API cost bind ($0.30 / $2.50 per million tokens, 1,048,576-token input context).
- On output tokens it is roughly 3x cheaper than `gemini-3.6-flash` and 5x cheaper than `gemini-3.1-pro-preview`.
- Send day-to-day coding to `gemini-3.6-flash` and planning or review to `gemini-3.1-pro-preview`.
- Use it for classification, extraction, and high-volume agentic subtasks run at scale.

See the Google model guidance page for sources and full governance notes.
```

- [ ] **Step 4: Wire the starter paths into `models.yaml`**

Add a `starter:` line to each of the ten current full-depth models. Insert it directly under the `verified:` line of each. The exact additions:

- `claude-opus-5`: `        starter: examples/anthropic/starter-claude-opus-5.md`
- `claude-fable-5`: `        starter: examples/anthropic/starter-claude-fable-5.md`
- `claude-sonnet-5`: `        starter: examples/anthropic/starter-claude-sonnet-5.md`
- `claude-haiku-4-5`: `        starter: examples/anthropic/starter-claude-haiku-4-5.md`
- `gpt-5.6-sol`: `        starter: examples/openai/starter-gpt-5.6-sol.md`
- `gpt-5.6-terra`: `        starter: examples/openai/starter-gpt-5.6-terra.md`
- `gpt-5.6-luna`: `        starter: examples/openai/starter-gpt-5.6-luna.md`
- `gemini-3.1-pro-preview`: `        starter: examples/google/starter-gemini-3.1-pro-preview.md`
- `gemini-3.6-flash`: `        starter: examples/google/starter-gemini-3.6-flash.md`
- `gemini-3.5-flash-lite`: `        starter: examples/google/starter-gemini-3.5-flash-lite.md`

Do NOT add `starter:` to `gpt-5.2` or `gemini-3-pro` (legacy) or any sparse-vendor model.

- [ ] **Step 5: Run the guidance suite**

Run: `go test ./internal/guidance/ -v`
Expected: PASS — `TestEveryCurrentFullDepthModelHasStarter`, `TestParseEmbeddedRegistry` (now exercises the positive `starter`-exists path), and `TestParseRegistryValidatesStarter` all green.

- [ ] **Step 6: Guard against em-dashes and commit**

```bash
! grep -rl '—' internal/guidance/models/examples/*/starter-*.md
gofmt -w internal/guidance/checklist_test.go
go vet ./... && go test ./internal/guidance/
git add internal/guidance/models/ internal/guidance/checklist_test.go
git commit -m "feat(guidance): per-model starter rule-pack fragments for full-depth vendors"
```

---

## Task 4: `publish.Manager.AddFragment` for adopting a new fragment

**Files:**
- Modify: `internal/portal/publish/publish.go` (add `ErrFragmentExists`, `addRuleToManifest`, `AddFragment`; extract `commitAndTag` from `Publish`)
- Test: `internal/portal/publish/publish_adopt_test.go` (new)

**Why this is not just `Publish`:** `Manifest.validate` requires every `rules:` entry to exist on disk, but an unlisted file on disk is ignored — it would be committed yet never appear in `PackInfo.Fragments` and never sync. Adopting a brand-new fragment must therefore ALSO add its path to `pack.yaml`'s `rules:` list. `Publish` only rewrites the version and overwrites an existing fragment, so a new method is required.

**Interfaces:**
- Consumes: existing `safeFragPath`, `restore`, `restoreAndErr`, `rewriteVersion`, `versionRE`, `gitRun`, `m.lockFor`, `pack.Load`, `esc.ErrManifest`.
- Produces:
  - `var ErrFragmentExists = errors.New("fragment already exists in pack")`
  - `func (m *Manager) AddFragment(ctx context.Context, name, frag string, content []byte, newVersion string) error`
  - `func commitAndTag(ctx context.Context, dir, name, newVersion string) error` (extracted; also used by `Publish`)
  - `func addRuleToManifest(path, rel string) error`

- [ ] **Step 1: Write the failing tests**

Create `internal/portal/publish/publish_adopt_test.go`:

```go
package publish

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/esc"
)

const testStarter = "---\ntargets: [claude, agents]\n---\n# Model x\n\n- Route work to x.\n"

func TestAddFragmentAdoptsNewFragment(t *testing.T) {
	mgr := NewManager(newPackClone(t))
	ctx := context.Background()
	dest := "rules/model-claude-sonnet-5.md"
	if err := mgr.AddFragment(ctx, "org-baseline", dest, []byte(testStarter), "1.3.0"); err != nil {
		t.Fatalf("AddFragment: %v", err)
	}
	info, err := mgr.Get(ctx, "org-baseline")
	if err != nil {
		t.Fatal(err)
	}
	if info.Version != "1.3.0" {
		t.Fatalf("version = %s, want 1.3.0", info.Version)
	}
	found := false
	for _, f := range info.Fragments {
		if f == dest {
			found = true
		}
	}
	if !found {
		t.Fatalf("adopted fragment not in manifest: %v", info.Fragments)
	}
	if len(info.Tags) == 0 || info.Tags[0].Name != "v1.3.0" {
		t.Fatalf("tags = %v, want newest v1.3.0", info.Tags)
	}
	got, err := mgr.ReadFragment("org-baseline", dest)
	if err != nil || string(got) != testStarter {
		t.Fatalf("ReadFragment = %q, %v", got, err)
	}
}

func TestAddFragmentCollisionRefuses(t *testing.T) {
	mgr := NewManager(newPackClone(t))
	ctx := context.Background()
	dest := "rules/model-claude-sonnet-5.md"
	if err := mgr.AddFragment(ctx, "org-baseline", dest, []byte(testStarter), "1.3.0"); err != nil {
		t.Fatal(err)
	}
	// Second adopt of the same path (new version so it clears the tag guard).
	err := mgr.AddFragment(ctx, "org-baseline", dest, []byte(testStarter), "1.4.0")
	if !errors.Is(err, ErrFragmentExists) {
		t.Fatalf("second adopt err = %v, want ErrFragmentExists", err)
	}
}

func TestAddFragmentInvalidContentRestores(t *testing.T) {
	mgr := NewManager(newPackClone(t))
	ctx := context.Background()
	bad := "---\ntargets: [nonsense-target]\n---\nbody\n"
	err := mgr.AddFragment(ctx, "org-baseline", "rules/model-bad.md", []byte(bad), "1.3.0")
	if !errors.Is(err, esc.ErrManifest) {
		t.Fatalf("err = %v, want esc.ErrManifest", err)
	}
	info, err := mgr.Get(ctx, "org-baseline")
	if err != nil {
		t.Fatal(err)
	}
	if info.Version != "1.2.0" {
		t.Fatalf("version = %s, want restored 1.2.0", info.Version)
	}
	if _, statErr := os.Stat(filepath.Join(info.Dir, "rules/model-bad.md")); statErr == nil {
		t.Fatal("failed adopt left the fragment on disk")
	}
	raw, _ := os.ReadFile(filepath.Join(info.Dir, "pack.yaml"))
	if strings.Contains(string(raw), "model-bad.md") {
		t.Fatal("failed adopt left the rule in the manifest")
	}
}
```

Note: `newPackClone` is the existing unexported helper in the `publish` package's test files — reuse it, do not duplicate.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/portal/publish/ -run TestAddFragment -v`
Expected: FAIL (undefined `AddFragment`, `ErrFragmentExists`).

- [ ] **Step 3: Extract `commitAndTag` from `Publish`**

In `internal/portal/publish/publish.go`, replace the entirety of `Publish`'s Step 4 block (from the `// Step 4: commit + tag.` comment through the final `return nil` of `Publish`) with:

```go
	// Step 4: commit + tag.
	return commitAndTag(ctx, dir, name, newVersion)
}

// commitAndTag stages, commits, and annotated-tags the pack clone at dir as
// v<newVersion>. It captures the pre-commit SHA so a failed tag step unwinds
// exactly that commit, and only while it is still HEAD; any failure restores
// the clone before returning. Shared by Publish (edit an existing fragment)
// and AddFragment (adopt a new one).
func commitAndTag(ctx context.Context, dir, name, newVersion string) error {
	tagName := "v" + newVersion
	preCommitHead, err := gitRun(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return restoreAndErr(ctx, dir, "HEAD", err)
	}
	preCommitHead = strings.TrimSpace(preCommitHead)

	if _, err := gitRun(ctx, dir, "add", "-A"); err != nil {
		return restoreAndErr(ctx, dir, "HEAD", err)
	}
	commitMsg := fmt.Sprintf("portal: publish %s v%s", name, newVersion)
	if _, err := gitRun(ctx, dir,
		"-c", "user.name=esc portal", "-c", "user.email=portal@escapement.local", "-c", "commit.gpgsign=false",
		"commit", "-m", commitMsg,
	); err != nil {
		return restoreAndErr(ctx, dir, "HEAD", err)
	}

	newHead, err := gitRun(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return restoreAndErr(ctx, dir, preCommitHead, err)
	}
	newHead = strings.TrimSpace(newHead)

	tagMsg := fmt.Sprintf("publish v%s", newVersion)
	if _, err := gitRun(ctx, dir,
		"-c", "user.name=esc portal", "-c", "user.email=portal@escapement.local", "-c", "tag.gpgsign=false",
		"tag", "-a", tagName, "-m", tagMsg,
	); err != nil {
		if curHead, herr := gitRun(ctx, dir, "rev-parse", "HEAD"); herr == nil && strings.TrimSpace(curHead) == newHead {
			return restoreAndErr(ctx, dir, preCommitHead, err)
		}
		return fmt.Errorf("tag failed and the commit could not be safely unwound (HEAD moved): %v", err)
	}
	return nil
}
```

(The `preCommitHead`/`newHead`/`tagName` locals that were inline in `Publish` now live only in `commitAndTag`; ensure no leftover references remain in `Publish`.)

- [ ] **Step 4: Add `ErrFragmentExists`, `addRuleToManifest`, and `AddFragment`**

Confirm `"errors"` is imported (it is). Add near `versionRE`:

```go
// ErrFragmentExists reports that AddFragment's destination path is already
// present in the pack on disk. The portal maps it to a 422 directing the user
// to edit that fragment instead; v1 never overwrites.
var ErrFragmentExists = errors.New("fragment already exists in pack")
```

Add these functions (place after `Publish`/`commitAndTag`):

```go
// addRuleToManifest appends rel to the top-level "rules" sequence of the
// pack.yaml at path, creating the key if absent. It round-trips through
// yaml.Node so comments and key order survive, mirroring rewriteVersion.
func addRuleToManifest(path, rel string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("pack.yaml: expected a top-level mapping")
	}
	mapping := doc.Content[0]
	var rules *yaml.Node
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == "rules" {
			rules = mapping.Content[i+1]
			break
		}
	}
	if rules == nil {
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "rules"},
			&yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"})
		rules = mapping.Content[len(mapping.Content)-1]
	}
	if rules.Kind != yaml.SequenceNode {
		rules.Kind = yaml.SequenceNode
		rules.Tag = "!!seq"
		rules.Value = ""
	}
	rules.Content = append(rules.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: rel})
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

// AddFragment adopts a brand-new fragment into the named pack: it writes
// content to frag, adds frag to pack.yaml's rules list, bumps the version to
// newVersion, validates, and commits + tags v<newVersion>. It refuses to
// overwrite: if frag already exists on disk it returns ErrFragmentExists and
// touches nothing. Like Publish, any failure after the working tree is
// modified restores the clone before returning, and calls for the same pack
// serialize.
func (m *Manager) AddFragment(ctx context.Context, name, frag string, content []byte, newVersion string) error {
	lock := m.lockFor(name)
	lock.Lock()
	defer lock.Unlock()

	dir := filepath.Join(m.Dir, name)

	if !versionRE.MatchString(newVersion) {
		return fmt.Errorf("invalid version %q: must match %s", newVersion, versionRE.String())
	}
	fragPath, err := safeFragPath(dir, frag)
	if err != nil {
		return err
	}
	tagName := "v" + newVersion
	existing, err := gitRun(ctx, dir, "tag", "-l", tagName)
	if err != nil {
		return err
	}
	if strings.TrimSpace(existing) != "" {
		return fmt.Errorf("tag %s already exists", tagName)
	}
	// Collision guard: never overwrite an existing fragment. This runs before
	// anything is written, so no restore is needed on this path.
	if _, err := os.Stat(fragPath); err == nil {
		return ErrFragmentExists
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(fragPath), 0o755); err != nil {
		return restoreAndErr(ctx, dir, "HEAD", err)
	}
	if err := os.WriteFile(fragPath, content, 0o644); err != nil {
		return restoreAndErr(ctx, dir, "HEAD", err)
	}
	manifestPath := filepath.Join(dir, "pack.yaml")
	if err := addRuleToManifest(manifestPath, frag); err != nil {
		return restoreAndErr(ctx, dir, "HEAD", err)
	}
	if err := rewriteVersion(manifestPath, newVersion); err != nil {
		return restoreAndErr(ctx, dir, "HEAD", err)
	}

	if _, err := pack.Load(dir); err != nil {
		if rerr := restore(ctx, dir, "HEAD"); rerr != nil {
			return fmt.Errorf("%w: pack validation failed: %v (restore also failed: %v)", esc.ErrManifest, err, rerr)
		}
		return fmt.Errorf("%w: pack validation failed: %v", esc.ErrManifest, err)
	}

	return commitAndTag(ctx, dir, name, newVersion)
}
```

- [ ] **Step 5: Run the publish suite**

Run: `go test ./internal/portal/publish/ -v`
Expected: PASS — new `TestAddFragment*` plus all existing `Publish` tests (guarding the `commitAndTag` extraction).

- [ ] **Step 6: Full gate + commit**

```bash
gofmt -w internal/portal/publish/publish.go internal/portal/publish/publish_adopt_test.go
go vet ./... && go test ./internal/portal/publish/
git add internal/portal/publish/publish.go internal/portal/publish/publish_adopt_test.go
git commit -m "feat(portal): publish manager AddFragment for adopting new pack fragments"
```

---

## Task 5: Render per-model starters on the vendor guidance page

**Files:**
- Modify: `internal/portal/web/models.go` (`modelView`, updated `handleModelVendor`, `writablePacks`, `modelVendorData`)
- Modify: `internal/portal/web/templates/model_vendor.html`
- Test: `internal/portal/web/models_test.go` (combined fixture + two tests)

**Interfaces:**
- Consumes: `guidance.Set.ReadFile`, `s.Packs.List`, `mdHTML`, `nextPatchVersion` (from `server.go`), `publish.NewManager`, `newPackClone` (packs_test.go).
- Produces:
  - `type modelView struct { guidance.Model; StarterFile string; StarterHTML template.HTML; StarterRaw string; HasStarter bool }`
  - `func (s *Server) writablePacks(ctx context.Context) ([]publish.PackInfo, error)`
  - `modelVendorData.Models` becomes `[]modelView`; new field `PacksAvailable bool`.
  - Test helper `newTestServerWithPacksAndGuidance(t *testing.T) *Server`.

- [ ] **Step 1: Write the failing tests**

Add to `internal/portal/web/models_test.go` (add `"github.com/tensorgroup/openescapement/internal/portal/publish"` to this file's import block):

```go
func newTestServerWithPacksAndGuidance(t *testing.T) *Server {
	t.Helper()
	dataDir := t.TempDir()
	if err := guidance.Seed(dataDir); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	packs := publish.NewManager(newPackClone(t))
	s := New(st, packs, "", "test")
	s.GuidanceDir = dataDir + "/guidance"
	return s
}

func TestVendorPageRendersStarterAndAdoptButton(t *testing.T) {
	h := newTestServerWithPacksAndGuidance(t).Handler()
	body := get(t, h, "/models/anthropic", nil).Body.String()
	for _, want := range []string{
		"Starter rule pack",
		`/models/anthropic/edit?file=examples%2fanthropic%2fstarter-claude-sonnet-5.md`,
		`/models/anthropic/adopt?model=claude-sonnet-5`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("/models/anthropic missing %q", want)
		}
	}
}

func TestVendorPageAdoptDisabledWithoutPacks(t *testing.T) {
	h := newTestServerWithGuidance(t).Handler() // Packs nil
	body := get(t, h, "/models/anthropic", nil).Body.String()
	if !strings.Contains(body, "Configure a rule pack repo") {
		t.Fatal("expected disabled-adopt hint")
	}
	if strings.Contains(body, `/models/anthropic/adopt?model=claude-sonnet-5`) {
		t.Fatal("adopt link must not render when no packs configured")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/portal/web/ -run 'TestVendorPageRendersStarter|TestVendorPageAdoptDisabled' -v`
Expected: FAIL.

- [ ] **Step 3: Add `modelView`, `writablePacks`, and update the handler**

In `internal/portal/web/models.go`, add to the import block: `"context"`, and `"github.com/tensorgroup/openescapement/internal/portal/publish"`.

Add above `modelVendorData`:

```go
// writablePacks lists the configured pack clones the adopt flow can publish
// into. Nil manager (no repos configured) yields an empty slice, not an error.
func (s *Server) writablePacks(ctx context.Context) ([]publish.PackInfo, error) {
	if s.Packs == nil {
		return nil, nil
	}
	return s.Packs.List(ctx)
}

// modelView pairs a registry Model with its rendered starter fragment (when
// one is set) for the vendor page's per-model starter block.
type modelView struct {
	guidance.Model
	StarterFile string
	StarterHTML template.HTML
	StarterRaw  string
	HasStarter  bool
}
```

Change `modelVendorData`'s `Models` field type and add `PacksAvailable`:

```go
type modelVendorData struct {
	layoutData
	VendorKey      string
	VendorName     string
	NoteHTML       template.HTML
	NoteFile       string
	Models         []modelView
	Examples       []exampleView
	PacksAvailable bool
	Degraded       []string
}
```

In `handleModelVendor`, replace the `Models: v.Models,` construction. After the examples loop and before `s.render`, build the model views and packs flag:

```go
	models := make([]modelView, 0, len(v.Models))
	for _, m := range v.Models {
		mv := modelView{Model: m}
		if m.Starter != "" {
			if content, deg, rerr := g.ReadFile(m.Starter); rerr == nil {
				mv.HasStarter = true
				mv.StarterFile = m.Starter
				mv.StarterHTML = mdHTML(content)
				mv.StarterRaw = string(content)
				if deg {
					degraded = append(degraded, m.Starter)
				}
			}
		}
		models = append(models, mv)
	}
	packsAvailable := false
	if infos, perr := s.writablePacks(r.Context()); perr == nil && len(infos) > 0 {
		packsAvailable = true
	}
```

Then update the `s.render` call to pass `Models: models,` and `PacksAvailable: packsAvailable,`.

- [ ] **Step 4: Update the vendor template**

In `internal/portal/web/templates/model_vendor.html`, replace the `{{range .Models}} ... {{end}}` article block (the models section) with:

```html
  {{range .Models}}
  <article class="fragment" id="{{.ID}}">
    <h4>{{.Name}} <span class="muted">{{.ID}}</span></h4>
    <ul class="chips"><li>{{.Tier}}</li>{{range .Roles}}<li>{{.}}</li>{{end}}<li>{{.Status}}</li></ul>
    <p class="muted">Verified {{.Verified}}</p>
    {{if .Docs}}<ul>{{range .Docs}}<li><a href="{{.}}" rel="noreferrer">{{host .}}</a></li>{{end}}</ul>{{end}}
    {{if .HasStarter}}
    <div class="starter" hx-disable>
      <h5>Starter rule pack <a href="/models/{{$.VendorKey}}/edit?file={{.StarterFile}}">Edit</a></h5>
      {{.StarterHTML}}
      <pre>{{.StarterRaw}}</pre>
    </div>
    {{if $.PacksAvailable}}
    <p><a class="button" href="/models/{{$.VendorKey}}/adopt?model={{.ID}}">Add to rule pack</a></p>
    {{else}}
    <p><button type="button" disabled>Add to rule pack</button> <span class="muted">Configure a rule pack repo to enable adoption.</span></p>
    {{end}}
    {{end}}
  </article>
  {{else}}
  <p class="muted">No models catalogued yet.</p>
  {{end}}
```

- [ ] **Step 5: Run the web suite**

Run: `go test ./internal/portal/web/ -v`
Expected: PASS — new tests plus existing `TestModelVendorPageRendersNoteModelsExamplesAnchors` (Models still expose `.ID`, `.Name`, `.Tier`, `.Roles`, `.Status`, `.Verified`, `.Docs` via the embedded `guidance.Model`).

- [ ] **Step 6: Full gate + commit**

```bash
gofmt -w internal/portal/web/models.go internal/portal/web/models_test.go
go vet ./... && go test ./internal/portal/web/
git add internal/portal/web/models.go internal/portal/web/templates/model_vendor.html internal/portal/web/models_test.go
git commit -m "feat(portal): render per-model starters on vendor guidance pages"
```

---

## Task 6: Adopt flow — add a starter to a rule pack

**Files:**
- Modify: `internal/portal/web/models.go` (`packOption`, `modelAdoptData`, `starterModel`, `handleModelAdopt`, `handleModelAdoptSave`)
- Create: `internal/portal/web/templates/model_adopt.html`
- Modify: `internal/portal/web/server.go` (register routes + `pageNames`)
- Test: `internal/portal/web/models_test.go` (adopt tests), `internal/portal/web/degrade_test.go` (full-page gate)

**Interfaces:**
- Consumes: `writablePacks`, `starterModel`, `s.Packs.AddFragment`, `publish.ErrFragmentExists`, `esc.ErrManifest`, `nextPatchVersion`, `mdHTML`, `findVendor`, `g.ReadFile`.
- Produces:
  - `func (s *Server) handleModelAdopt(w, r)` (GET), `func (s *Server) handleModelAdoptSave(w, r)` (POST)
  - `type packOption struct { Name, SuggestedVersion string }`
  - `type modelAdoptData struct { ... }`
  - `func starterModel(v guidance.Vendor, id string) (guidance.Model, bool)`
  - route `GET /models/{vendor}/adopt`, `POST /models/{vendor}/adopt`; `"model_adopt"` in `pageNames`.

**Behavior:** GET renders a full-page form: pack `<select>`, read-only destination `rules/model-<id>.md`, a version field defaulting to the first pack's next version, and a rendered+raw starter preview. POST calls `AddFragment` and redirects to `/packs/{pack}?published=v{version}` on success. Collision (`ErrFragmentExists`) and validation (`esc.ErrManifest`) both re-render at 422 with preserved state, different copy. Nil manager / no writable packs / unknown vendor / unknown-or-starterless model / unknown pack all 404.

- [ ] **Step 1: Write the failing tests**

Add to `internal/portal/web/models_test.go`:

```go
func TestAdoptGETRendersForm(t *testing.T) {
	h := newTestServerWithPacksAndGuidance(t).Handler()
	rr := get(t, h, "/models/anthropic/adopt?model=claude-sonnet-5", nil)
	if rr.Code != 200 {
		t.Fatalf("adopt GET code=%d", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{`<select name="pack"`, "org-baseline", "rules/model-claude-sonnet-5.md", "Starter preview", "Route day-to-day coding"} {
		if !strings.Contains(body, want) {
			t.Fatalf("adopt form missing %q", want)
		}
	}
}

func TestAdoptGET404s(t *testing.T) {
	withPacks := newTestServerWithPacksAndGuidance(t).Handler()
	if get(t, withPacks, "/models/anthropic/adopt?model=nope", nil).Code != 404 {
		t.Fatal("unknown model should 404")
	}
	if get(t, withPacks, "/models/nope/adopt?model=claude-sonnet-5", nil).Code != 404 {
		t.Fatal("unknown vendor should 404")
	}
	noPacks := newTestServerWithGuidance(t).Handler() // Packs nil
	if get(t, noPacks, "/models/anthropic/adopt?model=claude-sonnet-5", nil).Code != 404 {
		t.Fatal("no packs should 404")
	}
}

func adoptPost(t *testing.T, h http.Handler, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/models/anthropic/adopt", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestAdoptPOSTPublishesAndRedirects(t *testing.T) {
	h := newTestServerWithPacksAndGuidance(t).Handler()
	rr := adoptPost(t, h, url.Values{
		"model": {"claude-sonnet-5"}, "pack": {"org-baseline"}, "version": {"1.3.0"},
	})
	if rr.Code != 303 || rr.Header().Get("Location") != "/packs/org-baseline?published=v1.3.0" {
		t.Fatalf("adopt POST: code=%d loc=%s body=%s", rr.Code, rr.Header().Get("Location"), rr.Body.String())
	}
	detail := get(t, h, "/packs/org-baseline", nil).Body.String()
	if !strings.Contains(detail, "rules/model-claude-sonnet-5.md") {
		t.Fatal("adopted fragment not visible on pack detail")
	}
}

func TestAdoptPOSTCollision422(t *testing.T) {
	h := newTestServerWithPacksAndGuidance(t).Handler()
	if rr := adoptPost(t, h, url.Values{"model": {"claude-sonnet-5"}, "pack": {"org-baseline"}, "version": {"1.3.0"}}); rr.Code != 303 {
		t.Fatalf("first adopt: %d", rr.Code)
	}
	rr := adoptPost(t, h, url.Values{"model": {"claude-sonnet-5"}, "pack": {"org-baseline"}, "version": {"1.4.0"}})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("collision code=%d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Edit that fragment") || !strings.Contains(body, `<select name="pack"`) {
		t.Fatalf("collision page missing message or preserved form: %s", body)
	}
}
```

Add to `internal/portal/web/degrade_test.go`:

```go
func TestModelAdoptRendersFullPageWithoutHX(t *testing.T) {
	h := newTestServerWithPacksAndGuidance(t).Handler()
	body := get(t, h, "/models/anthropic/adopt?model=claude-sonnet-5", nil).Body.String()
	if !strings.Contains(body, "<html") {
		t.Fatal("adopt route must be a full page")
	}
	if !strings.Contains(body, "esc <strong>portal</strong>") {
		t.Fatal("adopt route missing sidebar")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/portal/web/ -run 'Adopt' -v`
Expected: FAIL (routes not registered; handlers/template absent).

- [ ] **Step 3: Add the adopt handlers and data types**

In `internal/portal/web/models.go`, add `"errors"`, `"fmt"`, and `"github.com/tensorgroup/openescapement/internal/esc"` to imports. Add:

```go
// starterModel returns the vendor's model with the given id when it has a
// starter set.
func starterModel(v guidance.Vendor, id string) (guidance.Model, bool) {
	for _, m := range v.Models {
		if m.ID == id && m.Starter != "" {
			return m, true
		}
	}
	return guidance.Model{}, false
}

// packOption is one writable pack in the adopt selector.
type packOption struct {
	Name             string
	SuggestedVersion string
}

// modelAdoptData is the /models/{vendor}/adopt page's data.
type modelAdoptData struct {
	layoutData
	VendorKey   string
	VendorName  string
	ModelID     string
	ModelName   string
	Dest        string
	StarterHTML template.HTML
	StarterRaw  string
	Packs       []packOption
	PackName    string
	Version     string
	Error       string
}

// packOptions maps writable packs to selector options with a suggested next
// version each.
func packOptions(infos []publish.PackInfo) []packOption {
	opts := make([]packOption, len(infos))
	for i, p := range infos {
		opts[i] = packOption{Name: p.Name, SuggestedVersion: nextPatchVersion(p.Version)}
	}
	return opts
}

func (s *Server) handleModelAdopt(w http.ResponseWriter, r *http.Request) {
	vendor := r.PathValue("vendor")
	g := s.loadGuidance()
	v, ok := findVendor(g.Registry, vendor)
	if !ok {
		http.NotFound(w, r)
		return
	}
	m, ok := starterModel(v, r.URL.Query().Get("model"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	content, _, err := g.ReadFile(m.Starter)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	infos, err := s.writablePacks(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	if len(infos) == 0 {
		http.NotFound(w, r)
		return
	}
	opts := packOptions(infos)
	s.render(w, "model_adopt", modelAdoptData{
		layoutData:  s.baseData("models"),
		VendorKey:   v.Key,
		VendorName:  v.Name,
		ModelID:     m.ID,
		ModelName:   m.Name,
		Dest:        "rules/model-" + m.ID + ".md",
		StarterHTML: mdHTML(content),
		StarterRaw:  string(content),
		Packs:       opts,
		PackName:    opts[0].Name,
		Version:     opts[0].SuggestedVersion,
	})
}

func (s *Server) handleModelAdoptSave(w http.ResponseWriter, r *http.Request) {
	vendor := r.PathValue("vendor")
	g := s.loadGuidance()
	v, ok := findVendor(g.Registry, vendor)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	m, ok := starterModel(v, r.FormValue("model"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	content, _, err := g.ReadFile(m.Starter)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	infos, err := s.writablePacks(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	if len(infos) == 0 {
		http.NotFound(w, r)
		return
	}
	packName := r.FormValue("pack")
	version := r.FormValue("version")
	valid := false
	for _, p := range infos {
		if p.Name == packName {
			valid = true
		}
	}
	if !valid {
		http.NotFound(w, r)
		return
	}
	dest := "rules/model-" + m.ID + ".md"
	if err := s.Packs.AddFragment(r.Context(), packName, dest, content, version); err != nil {
		if errors.Is(err, publish.ErrFragmentExists) || errors.Is(err, esc.ErrManifest) {
			msg := err.Error()
			if errors.Is(err, publish.ErrFragmentExists) {
				msg = "This pack already has " + dest + ". Edit that fragment in the pack instead; adopting never overwrites."
			}
			s.renderStatus(w, http.StatusUnprocessableEntity, "model_adopt", modelAdoptData{
				layoutData:  s.baseData("models"),
				VendorKey:   v.Key,
				VendorName:  v.Name,
				ModelID:     m.ID,
				ModelName:   m.Name,
				Dest:        dest,
				StarterHTML: mdHTML(content),
				StarterRaw:  string(content),
				Packs:       packOptions(infos),
				PackName:    packName,
				Version:     version,
				Error:       msg,
			})
			return
		}
		serverError(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/packs/%s?published=v%s", packName, version), http.StatusSeeOther)
}
```

- [ ] **Step 4: Create the adopt template**

Create `internal/portal/web/templates/model_adopt.html`:

```html
{{define "title"}}Adopt starter: {{.ModelName}}{{end}}
{{define "explainer"}}Add the {{.ModelName}} starter to a rule pack.{{end}}
{{define "content"}}
{{if .Error}}<div class="error">{{.Error}}</div>{{end}}
<form method="post" action="/models/{{.VendorKey}}/adopt">
  <input type="hidden" name="model" value="{{.ModelID}}">
  <p>
    <label>Rule pack
      <select name="pack">
        {{range .Packs}}<option value="{{.Name}}"{{if eq .Name $.PackName}} selected{{end}}>{{.Name}} (next {{.SuggestedVersion}})</option>{{end}}
      </select>
    </label>
  </p>
  <p>Destination fragment: <code>{{.Dest}}</code></p>
  <p>
    <label>Version
      <input name="version" value="{{.Version}}">
    </label>
  </p>
  <p><button type="submit">Add to rule pack</button></p>
</form>
<section hx-disable>
  <h3>Starter preview</h3>
  <article class="fragment">
    {{.StarterHTML}}
    <pre>{{.StarterRaw}}</pre>
  </article>
</section>
{{end}}
```

- [ ] **Step 5: Register routes and the page template**

In `internal/portal/web/server.go`, add `"model_adopt"` to `pageNames`:

```go
var pageNames = []string{"overview", "fleet", "packs", "pack", "pack_edit", "usage", "models", "model_vendor", "model_edit", "model_adopt"}
```

In `Handler`, add after the `model_edit` routes:

```go
	mux.HandleFunc("GET /models/{vendor}/adopt", s.handleModelAdopt)
	mux.HandleFunc("POST /models/{vendor}/adopt", s.handleModelAdoptSave)
```

(`New()` eagerly parses every `pageNames` template, so `model_adopt.html` from Step 4 must already exist — it does within this task.)

- [ ] **Step 6: Run the web suite**

Run: `go test ./internal/portal/web/ -v`
Expected: PASS — all adopt tests, the degrade gate, and every prior test.

- [ ] **Step 7: Full gate + commit**

```bash
gofmt -w internal/portal/web/models.go internal/portal/web/models_test.go internal/portal/web/degrade_test.go
go vet ./... && go test ./internal/portal/web/
git add internal/portal/web/models.go internal/portal/web/templates/model_adopt.html internal/portal/web/server.go internal/portal/web/models_test.go internal/portal/web/degrade_test.go
git commit -m "feat(portal): adopt flow to add a model starter to a rule pack"
```

---

## Task 7: Documentation

**Files:**
- Modify: `CHANGELOG.md` (add to the `## [Unreleased]` / `### Added` list)

**Interfaces:** none.

- [ ] **Step 1: Add the CHANGELOG bullet**

Insert as a new bullet at the top of the `### Added` list under `## [Unreleased]` in `CHANGELOG.md` (no em-dashes):

```markdown
- Model starters and adopt flow: every current full-depth-vendor model
  (Anthropic, OpenAI, Google) ships a starter rule-pack fragment encoding its
  routing position, review-gate, and cost guidance, seeded and portal-editable
  like the other guidance files. Each vendor page renders the starter with an
  Add to rule pack button; the adopt form writes the starter into a chosen
  writable pack through the publish pipeline (validate, add to the manifest,
  bump version, commit, tag) and never overwrites an existing fragment.
  Portal markdown is now rendered by goldmark (display-only, raw HTML
  disabled), turning Markdown links and bare http/https URLs into anchors.
```

- [ ] **Step 2: Guard em-dashes and commit**

```bash
! grep -n '—' CHANGELOG.md
git add CHANGELOG.md
git commit -m "docs: changelog for model starters and adopt flow"
```

---

## Task 8: Final verification

**Files:** none (verification only).

- [ ] **Step 1: Format and static gates**

Run: `gofmt -l . && go vet ./...`
Expected: `gofmt -l .` prints nothing; `go vet` clean.

- [ ] **Step 2: Module hygiene**

Run: `go mod tidy && git diff --exit-code go.mod go.sum`
Expected: no diff (goldmark is the only added require, already tidy, zero transitive deps).

- [ ] **Step 3: Full test suite**

Run: `go test ./...`
Expected: all packages PASS, including the integration tests that build real temp git repos.

- [ ] **Step 4: Build**

Run: `go build ./...`
Expected: clean build.

- [ ] **Step 5: Manual walkthrough (demo mode)**

Run `esc serve --demo` (seeds an org, a writable pack repo, and the guidance tree) and confirm each, capturing a screenshot at each point:
1. `/models/anthropic` — every current model shows a rendered "Starter rule pack" block with an Edit link and an enabled "Add to rule pack" button; the vendor note's Sources links and doc-host links are clickable anchors (goldmark), and no raw `---` front-matter shows in the rendered starter.
2. Click "Add to rule pack" on `claude-sonnet-5` — the adopt form shows the pack selector, read-only destination `rules/model-claude-sonnet-5.md`, a suggested version, and the starter preview (rendered + raw).
3. Submit — redirected to `/packs/{pack}?published=v<version>` with the published banner; the new `rules/model-claude-sonnet-5.md` fragment appears in the pack detail, and a new `v<version>` tag is listed.
4. Re-open the adopt form for the same model and submit again with a new version — a 422 page appears with the "Edit that fragment in the pack instead" message and the form state preserved.
5. On a server with no pack repos (or `/models/kimi`, which has no starters), confirm the adopt button is absent/disabled with the "Configure a rule pack repo" hint and `GET /models/anthropic/adopt?model=...` returns 404.

- [ ] **Step 6: Hand off for code review**

Proceed to `superpowers:requesting-code-review` (dispatch `code-reviewer`), then `superpowers:finishing-a-development-branch`.

---

## Self-Review

**Spec coverage (against `2026-08-02-models-starters-adopt-design.md`):**
- §1 clickable links → Task 1, now via goldmark: Markdown links + Linkify autolinks scoped to http/https, `rel="noreferrer"` injected post-render, dangerous schemes filtered by goldmark's safe renderer, raw HTML escaped, code spans not linkified, list-item context. Heading offset + front-matter strip preserve prior output shape so no other tests regress. ✓
- §2 per-model starter field + embedded-file validation + starter content + seeded/editable → Tasks 2, 3 (starters live under `examples/`, already in `KnownFiles` and the `Seed` walk). ✓
- §3 adopt flow: starter render + Add button → Task 5; adopt GET/POST + redirect → Task 6; collision 422 → Tasks 4+6; no-packs disabled/404, unknown model/starter 404 → Tasks 5, 6; disk-first starter read → Task 6. ✓
- Error handling (422 with preserved state; 404s) → Task 6. ✓
- Testing bullets → Tasks 1 (link table), 2, 3, 4, 5, 6. ✓
- Sequencing → Tasks 1–8. ✓
- Parent carried-over constraints (embedded+disk fail-soft, fixed file set, no em-dashes) → Global Constraints + guards. ✓

**Dependency-policy amendment:** Task 1 Step 1 adds the justified goldmark exception to AGENTS.md (display-only, zero transitive deps, `go.sum`-pinned, raw HTML disabled) and commits it separately from the code, keeping the policy's security-posture language intact. Verification (Task 8 Step 2) asserts `go mod tidy` produces no diff, proving the zero-transitive-dependency claim.

**Design gap surfaced and closed (unchanged from prior revision):** adopting a NEW fragment requires adding it to `pack.yaml`'s `rules:` list, or the file is committed but orphaned; Task 4 adds `AddFragment` rather than reusing `Publish`.

**goldmark-specific risks flagged for the executor:**
- Hostile-scheme filtering relies on goldmark's default `renderer/html` (Unsafe off) plus the 1.7.17 fix; Task 1 Step 2 verifies the resolved version is `>= v1.7.17`. If a future resolution regressed this, `TestMdHTMLLinks`'s hostile-scheme case fails loudly.
- `rel="noreferrer"` is added by string replace because goldmark emits no `rel`; documented as the deliberate mechanism, not a deviation.
- Front-matter stripping is required because goldmark would otherwise render fragment YAML as a thematic break plus setext heading; vendor notes (no front-matter) are unaffected.

**Placeholder scan:** every code/template/test/content block is complete; the ten starters are fully authored.

**Type consistency:** `mdHTML(src []byte) template.HTML` signature preserved (all callers unchanged); `AddFragment`, `ErrFragmentExists`, `commitAndTag`, `addRuleToManifest`, `writablePacks`, `starterModel`, `packOption`/`packOptions`, `modelView`, `modelVendorData.PacksAvailable`, `modelAdoptData`, and `Model.Starter` are referenced identically across tasks. Redirect target and the `%2f` edit-link assertion match existing handler behavior.

**Assumptions flagged:** the adopt version field defaults to the first writable pack's next patch version (no client JS under CSP); "writable packs" = all clones from `Manager.List`.

---
