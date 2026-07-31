# Portal Visual Refresh and Multi-Agent Demo Seed — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `esc serve` a polished sidebar-shell admin console (refined enterprise-light styling, restyled server-rendered SVG charts) and a demo seed that lands managed blocks in CLAUDE.md, AGENTS.md, and GEMINI.md.

**Architecture:** Pure presentation and seed-data change. No new routes, handlers, dependencies, or client JavaScript. Templates stay server-rendered `html/template`; charts stay deterministic server-rendered SVG (`strings.Builder`, golden-tested); the demo seed writes three pre-existing instruction files and flips the demo pack fragment to a three-target front-matter. Active-nav highlighting is driven by a new `Page` field on the shared template data; the org name in the sidebar comes from the registry the store already loads.

**Tech Stack:** Go 1.24, stdlib + `gopkg.in/yaml.v3` only, `html/template`, `os/exec` to system `git`, CSS with `:root` custom properties, `system-ui` font stack.

## Global Constraints

- **Single external dependency:** `gopkg.in/yaml.v3` only. Everything else is stdlib; system `git` via `os/exec`. Adding any dependency is forbidden here.
- **No client-side JavaScript.** No webfonts. `system-ui` stack only. No dark mode / `prefers-color-scheme`.
- **No new routes, handlers, auth changes, pages, or behavior changes.** Markup, CSS, chart styling, seed data, and docs only.
- **Determinism:** chart SVG output must be byte-deterministic (no map iteration into output, no timestamps). Golden files are regenerated deliberately and reviewed by eye, never by reflex.
- **Renderer invariants unchanged:** bytes outside a managed block are never modified; writes are atomic; nothing writes after a verification/constraint failure. This plan makes **no** `internal/render` changes — all three targets (`claude`, `agents`, `gemini`) already exist there.
- **Gates before claiming any task done:** `gofmt -w .`, `go vet ./...`, `go test ./...` all clean.
- **Commit style:** conventional commits, describe the *why*, NO AI co-authorship trailers. Commit at the end of every task.

---

## Task 1: Design tokens, sidebar shell, active-nav

Foundation: replace the stylesheet with a token system + sidebar layout, rebuild `layout.html` as a sidebar shell, and thread the active page + org name through the shared template data. After this task the app renders fully styled with working active-nav, and every existing test passes.

**Files:**
- Modify: `internal/portal/web/static/style.css` (full rewrite)
- Modify: `internal/portal/web/templates/layout.html` (full rewrite)
- Modify: `internal/portal/web/server.go` (`layoutData`, `baseData`, 8 call sites)
- Modify: `internal/portal/web/templates/overview.html:1`, `fleet.html:1`, `packs.html:1`, `usage.html:1`, `pack.html:1`, `pack_edit.html:1` (title blocks only)
- Test: `internal/portal/web/server_test.go` (add `TestActiveNav`)

**Interfaces:**
- Consumes: `s.Store.Registry().Org.Name` (existing `store.Registry`/`store.Org`, `store.go:27-35`).
- Produces: `layoutData{Version, Page, Org string}`; `func (s *Server) baseData(page string) layoutData`. Later tasks that construct page data embed `layoutData` and call `s.baseData("<navkey>")` where navkey ∈ `{"overview","fleet","packs","usage"}`.

- [ ] **Step 1: Add the active-nav test (failing)**

Add to `internal/portal/web/server_test.go`:

```go
func TestActiveNav(t *testing.T) {
	h := newTestServer(t, "").Handler()
	cases := map[string]string{
		"/":      `href="/" class="active"`,
		"/fleet": `href="/fleet" class="active"`,
		"/packs": `href="/packs" class="active"`,
		"/usage": `href="/usage" class="active"`,
	}
	for path, want := range cases {
		body := get(t, h, path, nil).Body.String()
		if !strings.Contains(body, want) {
			t.Fatalf("%s: missing active nav %q", path, want)
		}
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/portal/web/ -run TestActiveNav`
Expected: FAIL (layout has no `class="active"` yet).

- [ ] **Step 3: Add `Page`/`Org` to `layoutData` and rework `baseData`**

In `internal/portal/web/server.go`, replace:

```go
type layoutData struct {
	Version string
}

func (s *Server) baseData() layoutData {
	return layoutData{Version: s.Version}
}
```

with:

```go
type layoutData struct {
	Version string
	Page    string // active nav key: "overview", "fleet", "packs", "usage"
	Org     string // organization name, pinned in the sidebar footer
}

func (s *Server) baseData(page string) layoutData {
	org := ""
	if s.Store != nil {
		org = s.Store.Registry().Org.Name
	}
	return layoutData{Version: s.Version, Page: page, Org: org}
}
```

- [ ] **Step 4: Pass the nav key at all 8 `baseData` call sites**

In `internal/portal/web/server.go`, change each `s.baseData()` call:
- `handleOverview` → `s.baseData("overview")`
- `handleFleet` → `s.baseData("fleet")`
- `handlePacks` → `s.baseData("packs")`
- `handlePackDetail` → `s.baseData("packs")`
- `handlePackEdit` → `s.baseData("packs")`
- `handlePackPublish` `case "diff"` → `s.baseData("packs")`
- `handlePackPublish` `case "publish"` error branch → `s.baseData("packs")`
- `handleUsage` → `s.baseData("usage")`

- [ ] **Step 5: Rewrite `layout.html` as the sidebar shell**

Replace the entire contents of `internal/portal/web/templates/layout.html` with:

```html
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{block "title" .}}esc portal{{end}}</title>
<link rel="stylesheet" href="/static/style.css">
</head>
<body>
<aside class="sidebar">
  <div class="brand">esc <strong>portal</strong></div>
  <nav>
    <a href="/"{{if eq .Page "overview"}} class="active"{{end}}>Overview</a>
    <a href="/fleet"{{if eq .Page "fleet"}} class="active"{{end}}>Fleet</a>
    <a href="/packs"{{if eq .Page "packs"}} class="active"{{end}}>Rule Packs</a>
    <a href="/usage"{{if eq .Page "usage"}} class="active"{{end}}>Usage</a>
  </nav>
  <div class="sidebar-foot">
    <div class="org">{{.Org}}</div>
    <div class="ver">esc {{.Version}}</div>
  </div>
</aside>
<main class="content">
  <header class="page-head">
    <h1>{{template "title" .}}</h1>
    <p class="explainer">{{block "explainer" .}}{{end}}</p>
  </header>
  <div class="page-body">{{block "content" .}}{{end}}</div>
</main>
</body>
</html>
```

Note: `{{block "title" .}}` defines + executes for the `<title>` tag; `{{template "title" .}}` re-invokes the same block for the `<h1>`. Keeping `esc <strong>portal</strong>` verbatim satisfies the existing `TestPagesRenderWithoutAuth` assertion.

- [ ] **Step 6: Trim the page title blocks to plain section names**

So the tab title and `<h1>` read cleanly (the sidebar carries the brand). Change line 1 of each:
- `overview.html`: `{{define "title"}}Overview{{end}}`
- `fleet.html`: `{{define "title"}}Fleet{{end}}`
- `packs.html`: `{{define "title"}}Rule Packs{{end}}`
- `usage.html`: `{{define "title"}}Usage{{end}}`
- `pack.html`: `{{define "title"}}{{.Pack.Name}}{{end}}`
- `pack_edit.html`: `{{define "title"}}Edit {{.Frag}}{{end}}`

- [ ] **Step 7: Rewrite `style.css` with the token system and shell**

Replace the entire contents of `internal/portal/web/static/style.css` with:

```css
/* Design tokens: refined enterprise light. No dark mode, no webfonts. */
:root {
  --page: #f8f9fb;
  --surface: #ffffff;
  --ink: #1a2230;
  --ink-2: #384252;
  --muted: #5b6672;
  --accent: #2563eb;
  --accent-hover: #1d4fd7;
  --accent-weak: #eef2fd;
  --border: #e6e9ef;
  --border-strong: #d7dce4;

  /* Status pill pairs (background / text), each ≥ WCAG AA on its background. */
  --sync-bg: #e3f4ec;  --sync-ink: #1a7a4f;
  --drift-bg: #fdf1dd; --drift-ink: #9a560f;
  --stale-bg: #fce8e8; --stale-ink: #b02a2a;
  --ungov-bg: #eceef1; --ungov-ink: #566070;

  /* Spacing on a 4px scale. */
  --s1: 4px; --s2: 8px; --s3: 12px; --s4: 16px; --s5: 24px; --s6: 32px; --s7: 48px;

  --radius: 8px;
  --radius-sm: 6px;
  --shadow-1: 0 1px 2px rgba(16, 24, 40, .05);
  --shadow-2: 0 4px 12px rgba(16, 24, 40, .08);

  --sidebar-w: 220px;
  --content-max: 1080px;

  --font: system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
  --mono: ui-monospace, SFMono-Regular, Menlo, monospace;
}

* { box-sizing: border-box; }

html, body {
  margin: 0;
  padding: 0;
  background: var(--page);
  color: var(--ink);
  font-family: var(--font);
  font-size: 15px;
  line-height: 1.5;
}

a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }
.muted { color: var(--muted); }

/* ---- Sidebar shell ---- */
.sidebar {
  position: fixed;
  top: 0; left: 0; bottom: 0;
  width: var(--sidebar-w);
  display: flex;
  flex-direction: column;
  background: var(--surface);
  border-right: 1px solid var(--border);
  padding: var(--s5) var(--s4);
}
.sidebar .brand {
  font-size: 18px;
  color: var(--muted);
  margin-bottom: var(--s6);
  padding: 0 var(--s2);
}
.sidebar .brand strong { color: var(--ink); font-weight: 600; }
.sidebar nav { display: flex; flex-direction: column; gap: var(--s1); }
.sidebar nav a {
  color: var(--ink-2);
  font-size: 14px;
  padding: var(--s2);
  border-radius: var(--radius-sm);
}
.sidebar nav a:hover { background: var(--page); text-decoration: none; }
.sidebar nav a.active { background: var(--accent-weak); color: var(--accent); font-weight: 600; }
.sidebar-foot { margin-top: auto; padding: var(--s2); }
.sidebar-foot .org { font-size: 14px; font-weight: 600; color: var(--ink); }
.sidebar-foot .ver { font-size: 13px; color: var(--muted); }

/* ---- Content column ---- */
.content {
  margin-left: var(--sidebar-w);
  padding: var(--s6) var(--s7);
  max-width: calc(var(--sidebar-w) + var(--content-max));
}
.page-head { margin-bottom: var(--s5); }
.page-head h1 { font-size: 24px; font-weight: 600; margin: 0 0 var(--s2); }
.page-head .explainer { margin: 0; color: var(--muted); font-size: 15px; max-width: 70ch; }
section { margin: var(--s6) 0; }
h2 { font-size: 18px; font-weight: 600; margin: 0 0 var(--s3); }
h3 { font-size: 15px; font-weight: 600; margin: 0 0 var(--s3); }

/* ---- Stat cards ---- */
.cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: var(--s3);
  margin: var(--s4) 0;
}
.card {
  display: block;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  box-shadow: var(--shadow-1);
  padding: var(--s4);
  color: inherit;
}
a.card:hover { border-color: var(--border-strong); box-shadow: var(--shadow-2); text-decoration: none; }
.card .num {
  display: block;
  font-size: 24px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--ink);
}
.card .label { display: block; font-size: 14px; color: var(--muted); margin-top: var(--s1); }
.card .muted { font-size: 13px; }
.card.warn { border-color: var(--border-strong); }
.card.warn.amber .num { color: var(--drift-ink); }
.card.warn.red .num { color: var(--stale-ink); }

/* ---- Tables ---- */
table {
  width: 100%;
  border-collapse: collapse;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  box-shadow: var(--shadow-1);
  overflow: hidden;
}
th, td {
  text-align: left;
  padding: var(--s3) var(--s4);
  border-bottom: 1px solid var(--border);
  font-size: 14px;
  font-variant-numeric: tabular-nums;
}
th {
  color: var(--muted);
  font-weight: 500;
  font-size: 13px;
  text-transform: uppercase;
  letter-spacing: .03em;
  background: var(--page);
}
tbody tr:hover { background: var(--page); }
tbody tr:last-child td { border-bottom: none; }
tfoot td { font-weight: 600; border-top: 1px solid var(--border-strong); background: var(--page); }
.mono, td .mono { font-family: var(--mono); font-size: 13px; }

/* ---- Status pills ---- */
.pill {
  display: inline-block;
  padding: 2px var(--s2);
  border-radius: 999px;
  font-size: 13px;
  font-weight: 500;
  line-height: 1.4;
}
.pill.in-sync { background: var(--sync-bg); color: var(--sync-ink); }
.pill.drifted { background: var(--drift-bg); color: var(--drift-ink); }
.pill.stale { background: var(--stale-bg); color: var(--stale-ink); }
.pill.ungoverned { background: var(--ungov-bg); color: var(--ungov-ink); }

/* ---- Chips ---- */
.chips { list-style: none; display: flex; flex-wrap: wrap; gap: var(--s2); padding: 0; margin: 0; }
.chips li {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 999px;
  padding: var(--s1) var(--s3);
  font-size: 13px;
  font-family: var(--mono);
  color: var(--ink-2);
}

/* ---- Filter rows ---- */
.filters {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--s2);
  margin: 0 0 var(--s4);
  color: var(--muted);
  font-size: 14px;
}
.filters strong { color: var(--ink); }
.filters a { color: var(--muted); }
.filters a:hover { color: var(--accent); }

/* ---- Form controls ---- */
select, input:not([type="hidden"]), button {
  font-family: var(--font);
  font-size: 14px;
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-sm);
  padding: var(--s2) var(--s3);
  background: var(--surface);
  color: var(--ink);
}
select:focus, input:focus, textarea:focus, button:focus-visible {
  outline: 2px solid var(--accent-weak);
  border-color: var(--accent);
}
button {
  cursor: pointer;
  font-weight: 500;
  background: var(--accent);
  color: #fff;
  border-color: var(--accent);
}
button:hover { background: var(--accent-hover); }
button[value="diff"] {
  background: var(--surface);
  color: var(--ink);
  border-color: var(--border-strong);
}
button[value="diff"]:hover { background: var(--page); }

/* ---- Charts ---- */
svg { max-width: 100%; height: auto; display: block; }

/* ---- Pack fragments ---- */
.fragment {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  box-shadow: var(--shadow-1);
  padding: var(--s4) var(--s5);
  margin: var(--s3) 0;
}
.fragment h4 { margin-top: 0; font-size: 15px; }
.fragment h4 a { margin-left: var(--s3); font-size: 13px; font-weight: 500; }

/* ---- Code / diff ---- */
pre.diff, .fragment pre {
  background: var(--page);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  padding: var(--s3) var(--s4);
  overflow-x: auto;
  font-family: var(--mono);
  font-size: 13px;
  line-height: 1.6;
}
pre.diff { margin: 0; }
.diff .line { display: block; }
.diff .add { background: var(--sync-bg); color: var(--sync-ink); }
.diff .del { background: var(--stale-bg); color: var(--stale-ink); }
.diff .meta { color: var(--muted); }

/* ---- Editor textarea ---- */
textarea {
  width: 100%;
  max-width: 100%;
  font-family: var(--mono);
  font-size: 13px;
  line-height: 1.6;
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-sm);
  padding: var(--s3);
  background: var(--surface);
  color: var(--ink);
  tab-size: 2;
}

/* ---- Banners ---- */
.error, .success {
  border-radius: var(--radius-sm);
  padding: var(--s3) var(--s4);
  margin: var(--s3) 0;
  font-size: 14px;
}
.error { background: var(--stale-bg); border: 1px solid var(--stale-ink); color: var(--stale-ink); }
.success { background: var(--sync-bg); border: 1px solid var(--sync-ink); color: var(--sync-ink); }
```

- [ ] **Step 8: Run the web tests**

Run: `go test ./internal/portal/web/`
Expected: PASS (including `TestActiveNav` and the untouched `TestPagesRenderWithoutAuth`, which still finds `esc <strong>portal</strong>`).

- [ ] **Step 9: gofmt / vet**

Run: `gofmt -w . && go vet ./internal/portal/web/`
Expected: no output.

- [ ] **Step 10: Commit**

```bash
git add internal/portal/web/static/style.css internal/portal/web/templates internal/portal/web/server.go internal/portal/web/server_test.go
git commit -m "feat(portal): sidebar shell and enterprise-light design tokens

Replace the flat top-nav prototype with a fixed sidebar (brand, active-page
nav, org name pinned at the bottom) and a token-based stylesheet. Active-nav
state comes from a new Page field on the shared template data; the org name
comes from the registry the store already loads."
```

---

## Task 2: Page markup refinements — hero tint and diff coloring

Two markup changes the token stylesheet cannot do alone: status-tinting the overview hero numbers, and coloring add/remove lines in the publish diff preview. The diff coloring needs a template helper (no new route, no behavior change).

**Files:**
- Modify: `internal/portal/web/server.go` (add `diffLine` type, `diffLines` func, register in `New`)
- Modify: `internal/portal/web/templates/overview.html:7,9` (hero tint classes)
- Modify: `internal/portal/web/templates/pack_edit.html:6` (diff rendering)
- Test: `internal/portal/web/packs_test.go` (add `TestDiffColoring`)

**Interfaces:**
- Consumes: `packEditData.Diff` (the `git diff --no-index` unified-diff string from `publish.Manager.Diff`, `publish.go:242`).
- Produces: template func `diffLines(string) []diffLine` where `diffLine{Class, Text string}` and `Class ∈ {"add","del","meta",""}`.

- [ ] **Step 1: Add the diff-coloring test (failing)**

Add to `internal/portal/web/packs_test.go`:

```go
func TestDiffColoring(t *testing.T) {
	s := newTestServerWithPacks(t)
	form := url.Values{
		"frag":    {"rules/security.md"},
		"content": {"---\ntargets: [claude, agents]\n---\n# Security\n\n- Never commit secrets.\n- Added rule.\n"},
		"version": {"1.3.0"},
		"action":  {"diff"},
	}
	req := httptest.NewRequest("POST", "/packs/org-baseline/publish", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("diff render: %d %s", rr.Code, rr.Body.String())
	}
	if body := rr.Body.String(); !strings.Contains(body, `class="line add"`) {
		t.Fatalf("added line not colored:\n%s", body)
	}
}
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `go test ./internal/portal/web/ -run TestDiffColoring`
Expected: FAIL (`diffLines` not defined / no `class="line add"`).

- [ ] **Step 3: Add the `diffLines` helper**

In `internal/portal/web/server.go`, add near `fmtLastSync`:

```go
// diffLine is one line of a unified diff, classed for template coloring.
type diffLine struct {
	Class string // "add", "del", "meta", or "" (context)
	Text  string
}

// diffLines splits a `git diff --no-index` unified diff into classed lines:
// header lines ("diff ", "index ", "---", "+++", "@@") as meta, "+" adds and
// "-" removals colored, everything else unclassed context. Text is rendered
// through html/template, so it is auto-escaped.
func diffLines(s string) []diffLine {
	if s == "" {
		return nil
	}
	raw := strings.Split(strings.TrimRight(s, "\n"), "\n")
	out := make([]diffLine, 0, len(raw))
	for _, ln := range raw {
		class := ""
		switch {
		case strings.HasPrefix(ln, "+++"), strings.HasPrefix(ln, "---"),
			strings.HasPrefix(ln, "@@"), strings.HasPrefix(ln, "diff "),
			strings.HasPrefix(ln, "index "):
			class = "meta"
		case strings.HasPrefix(ln, "+"):
			class = "add"
		case strings.HasPrefix(ln, "-"):
			class = "del"
		}
		out = append(out, diffLine{Class: class, Text: ln})
	}
	return out
}
```

- [ ] **Step 4: Register `diffLines` in the template FuncMap**

In `internal/portal/web/server.go`, in `New`, extend the FuncMap:

```go
	s.layout = template.Must(template.New("layout.html").Funcs(template.FuncMap{
		"abbrev":    abbrevTokens,
		"fmtTime":   fmtLastSync,
		"diffLines": diffLines,
	}).ParseFS(templateFS, "templates/layout.html"))
```

- [ ] **Step 5: Render the diff as classed lines**

In `internal/portal/web/templates/pack_edit.html`, replace line 6:

```html
{{if .Diff}}<pre class="diff">{{.Diff}}</pre>{{end}}
```

with:

```html
{{if .Diff}}<pre class="diff">{{range diffLines .Diff}}<span class="line {{.Class}}">{{.Text}}</span>{{end}}</pre>{{end}}
```

- [ ] **Step 6: Tint the overview hero numbers**

In `internal/portal/web/templates/overview.html`, change the drift card (line 7) and stale card (line 9) so the number tints only when nonzero. Line 7:

```html
  <a class="card{{if .Stats.DriftedRepos}} warn amber{{end}}" href="/fleet?status=drifted"><span class="num">{{.Stats.DriftedRepos}}</span>
```

Line 9:

```html
  <a class="card{{if .Stats.StaleRepos}} warn red{{end}}" href="/fleet?status=stale"><span class="num">{{.Stats.StaleRepos}}</span>
```

- [ ] **Step 7: Run the web tests**

Run: `go test ./internal/portal/web/`
Expected: PASS (`TestDiffColoring`, `TestEditAndPublishFlow`, `TestPublishValidationErrorKeepsContent`, and the rest).

- [ ] **Step 8: gofmt / vet**

Run: `gofmt -w . && go vet ./internal/portal/web/`
Expected: no output.

- [ ] **Step 9: Commit**

```bash
git add internal/portal/web/server.go internal/portal/web/packs_test.go internal/portal/web/templates/overview.html internal/portal/web/templates/pack_edit.html
git commit -m "feat(portal): status-tinted overview hero and colored diff preview

Tint the drift/stale hero numbers only when nonzero, and split the publish
diff preview into add/remove/meta lines so a reviewer can read the change at
a glance. Coloring is a pure template helper; no route or handler changes."
```

---

## Task 3: Restyle charts and regenerate goldens

Align the SVG chart palette and axis text with the new tokens, mute the axis labels, add gridlines, and soften the gridline stroke. Golden files are regenerated deliberately and reviewed by eye.

**Files:**
- Modify: `internal/portal/charts/charts.go` (palette, `textAttrs`, gridline levels + stroke)
- Modify (regenerated): `internal/portal/charts/testdata/line.golden.svg`, `internal/portal/charts/testdata/bars.golden.svg`

**Interfaces:**
- Consumes: nothing new. `charts.Line`, `charts.StackedBars`, `charts.Abbrev` signatures are unchanged.
- Produces: unchanged public API; only rendered bytes change.

- [ ] **Step 1: Update the palette and axis text style**

In `internal/portal/charts/charts.go`, replace:

```go
var palette = []string{"#2563a8", "#4d9078", "#b0713f", "#7a5aa0", "#a84b57", "#5b7a99"}

const (
	mLeft, mRight, mTop, mBottom = 40.0, 20.0, 10.0, 24.0
	textAttrs                    = `font-family="system-ui,sans-serif" font-size="11" fill="#5f6b76"`
)
```

with:

```go
var palette = []string{"#2563eb", "#1a7a4f", "#9a560f", "#7c3aed", "#b02a2a", "#3b82a0"}

const (
	mLeft, mRight, mTop, mBottom = 40.0, 20.0, 10.0, 24.0
	textAttrs                    = `font-family="system-ui,sans-serif" font-size="11" fill="#5b6672"`
	gridStroke                   = "#e9ecf1"
)

// gridLevels returns the y-values to draw gridlines/labels at: quarters from
// 0 to maxY (five lines), for a calmer, easier-to-read grid than 0/mid/max.
func gridLevels(maxY float64) []float64 {
	return []float64{0, maxY / 4, maxY / 2, 3 * maxY / 4, maxY}
}
```

- [ ] **Step 2: Use the softer gridlines in `Line`**

In `Line`, replace the gridline loop:

```go
	// gridlines + y labels at 0, mid, max
	for _, v := range []float64{0, maxY / 2, maxY} {
		y := mTop + ph - ph*v/maxY
		fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="#e3e7ea"/>`,
			f(mLeft), f(y), f(mLeft+pw), f(y))
		fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="end" %s>%s</text>`,
			f(mLeft-6), f(y+4), textAttrs, esc(abbrev(v)))
	}
```

with:

```go
	// gridlines + y labels at quarter intervals
	for _, v := range gridLevels(maxY) {
		y := mTop + ph - ph*v/maxY
		fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s"/>`,
			f(mLeft), f(y), f(mLeft+pw), f(y), gridStroke)
		fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="end" %s>%s</text>`,
			f(mLeft-6), f(y+4), textAttrs, esc(abbrev(v)))
	}
```

- [ ] **Step 3: Use the softer gridlines in `StackedBars`**

In `StackedBars`, replace the gridline loop:

```go
	// gridlines + y labels at 0, mid, max
	for _, v := range []float64{0, maxY / 2, maxY} {
		y := top + ph - ph*v/maxY
		fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="#e3e7ea"/>`,
			f(mLeft), f(y), f(mLeft+pw), f(y))
		fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="end" %s>%s</text>`,
			f(mLeft-6), f(y+4), textAttrs, esc(abbrev(v)))
	}
```

with:

```go
	// gridlines + y labels at quarter intervals
	for _, v := range gridLevels(maxY) {
		y := top + ph - ph*v/maxY
		fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s"/>`,
			f(mLeft), f(y), f(mLeft+pw), f(y), gridStroke)
		fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="end" %s>%s</text>`,
			f(mLeft-6), f(y+4), textAttrs, esc(abbrev(v)))
	}
```

- [ ] **Step 4: Confirm the goldens now mismatch (expected)**

Run: `go test ./internal/portal/charts/ -run Golden`
Expected: FAIL with "golden mismatch" for `line.golden.svg` and `bars.golden.svg`. `TestEmptyStates` still passes ("No data yet" is unchanged).

- [ ] **Step 5: Regenerate the goldens (deliberate)**

The test harness (`internal/portal/charts/charts_test.go`) writes goldens when run with the `-update` flag. Run:

Run: `go test ./internal/portal/charts/ -run Golden -update`
Expected: PASS (files rewritten).

- [ ] **Step 6: Review the regenerated SVG by eye**

Run: `git diff --stat internal/portal/charts/testdata/` then inspect the two files.
Confirm by eye: five gridlines, `fill="#5b6672"` axis text, `stroke="#e9ecf1"` gridlines, `#2563eb`/`#1a7a4f`/... series colors, no unexpected coordinate drift. Optionally open one in a browser to sanity-check rendering.

- [ ] **Step 7: Confirm the goldens pass without `-update`**

Run: `go test ./internal/portal/charts/`
Expected: PASS.

- [ ] **Step 8: gofmt / vet**

Run: `gofmt -w . && go vet ./internal/portal/charts/`
Expected: no output.

- [ ] **Step 9: Commit**

```bash
git add internal/portal/charts/charts.go internal/portal/charts/testdata/line.golden.svg internal/portal/charts/testdata/bars.golden.svg
git commit -m "feat(portal): restyle charts to match new design tokens

Repoint the series palette and axis text at the token colors, mute the axis
labels, draw quarter-interval gridlines, and soften the gridline stroke.
Golden SVG regenerated with -update and reviewed by eye."
```

---

## Task 4: Demo seed — three instruction files, gemini target

Seed realistic pre-existing `CLAUDE.md`, `AGENTS.md`, and `GEMINI.md` in the demo repo, and flip the demo pack fragment to target all three so a publish→sync lands a managed block in each while leaving the seeded content untouched (the renderer invariant on show).

**Files:**
- Modify: `internal/portal/seed/repos.go` (`securityFragHeader`, `buildDemoRepo`)
- Test: `internal/portal/seed/repos_test.go` (extend `TestReposIdempotent`)
- Test: `internal/cli/serve_e2e_test.go` (extend `TestDemoPublishSyncLoop`)

**Interfaces:**
- Consumes: existing `seed.Repos(dataDir) (packDir, demoRepo string, err error)` and renderer targets `claude`/`agents`/`gemini` (`render.go:20-24`).
- Produces: demo repo containing `CLAUDE.md`, `AGENTS.md`, `GEMINI.md` with pre-existing content; demo pack fragment front-matter `targets: [claude, agents, gemini]`.

- [ ] **Step 1: Extend the seed test to require three files (failing)**

In `internal/portal/seed/repos_test.go`, in `TestReposIdempotent`, after the `config.yaml` assertion (line 23), add:

```go
	for _, name := range []string{"CLAUDE.md", "AGENTS.md", "GEMINI.md"} {
		if _, err := os.Stat(filepath.Join(r1, name)); err != nil {
			t.Fatalf("missing seeded %s: %v", name, err)
		}
	}
```

- [ ] **Step 2: Extend the e2e test to check all three targets (failing)**

In `internal/cli/serve_e2e_test.go`, in `TestDemoPublishSyncLoop`, replace the initial CLAUDE.md check (currently lines 28-31):

```go
	claude, _ := os.ReadFile(filepath.Join(demoRepo, "CLAUDE.md"))
	if !strings.Contains(string(claude), "org-baseline@1.2.0") {
		t.Fatalf("initial block: %s", claude)
	}
```

with a loop over all three files that also proves pre-existing content survives:

```go
	for _, name := range []string{"CLAUDE.md", "AGENTS.md", "GEMINI.md"} {
		b, _ := os.ReadFile(filepath.Join(demoRepo, name))
		if !strings.Contains(string(b), "org-baseline@1.2.0") {
			t.Fatalf("%s missing managed block:\n%s", name, b)
		}
		if !strings.Contains(string(b), "Payments service") {
			t.Fatalf("%s lost pre-existing content:\n%s", name, b)
		}
	}
```

Then replace the final CLAUDE.md-only assertion (currently lines 53-56):

```go
	claude, _ = os.ReadFile(filepath.Join(demoRepo, "CLAUDE.md"))
	if !strings.Contains(string(claude), "org-baseline@1.3.0") ||
		!strings.Contains(string(claude), "Model routing") {
		t.Fatalf("published rule did not arrive:\n%s", claude)
	}
```

with:

```go
	for _, name := range []string{"CLAUDE.md", "AGENTS.md", "GEMINI.md"} {
		b, _ := os.ReadFile(filepath.Join(demoRepo, name))
		if !strings.Contains(string(b), "org-baseline@1.3.0") ||
			!strings.Contains(string(b), "Model routing") {
			t.Fatalf("%s did not receive published rule:\n%s", name, b)
		}
	}
```

- [ ] **Step 3: Run the tests to confirm they fail**

Run: `go test ./internal/portal/seed/ ./internal/cli/ -run 'TestReposIdempotent|TestDemoPublishSyncLoop'`
Expected: FAIL (only CLAUDE.md is seeded; AGENTS.md/GEMINI.md are missing and the fragment does not target gemini).

- [ ] **Step 4: Add `gemini` to the demo fragment targets**

In `internal/portal/seed/repos.go`, change:

```go
const securityFragHeader = "---\ntargets: [claude, agents]\n---\n# Security\n\n"
```

to:

```go
const securityFragHeader = "---\ntargets: [claude, agents, gemini]\n---\n# Security\n\n"
```

- [ ] **Step 5: Seed three realistic instruction files**

In `internal/portal/seed/repos.go`, replace the tail of `buildDemoRepo` (currently lines 152-154):

```go
	claude := "# esc demo repo\n\nThis repo is governed by the esc admin portal demo. " +
		"Run `esc sync` here after publishing a rule change from the portal.\n"
	return os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(claude), 0o644)
```

with:

```go
	// Pre-existing user content, one file per target. On sync, esc appends its
	// managed block below this content and never touches these bytes — the
	// core renderer invariant the publish demo is meant to show.
	files := map[string]string{
		"CLAUDE.md": "# Payments service\n\n" +
			"Go 1.24 monorepo; run `make test` before pushing.\n" +
			"Ask in #payments-eng before changing the ledger schema.\n",
		"AGENTS.md": "# Payments service\n\n" +
			"Primary language is Go. Keep handlers thin and push logic into `internal/`.\n" +
			"Integration tests need a local Postgres; see `docs/dev-setup.md`.\n",
		"GEMINI.md": "# Payments service\n\n" +
			"This repo settles real money. Prefer boring, well-tested changes.\n" +
			"Never log full card numbers or auth tokens.\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
```

- [ ] **Step 6: Run the tests to confirm they pass**

Run: `go test ./internal/portal/seed/ ./internal/cli/ -run 'TestReposIdempotent|TestDemoPublishSyncLoop'`
Expected: PASS.

- [ ] **Step 7: gofmt / vet**

Run: `gofmt -w . && go vet ./internal/portal/seed/ ./internal/cli/`
Expected: no output.

- [ ] **Step 8: Commit**

```bash
git add internal/portal/seed/repos.go internal/portal/seed/repos_test.go internal/cli/serve_e2e_test.go
git commit -m "feat(portal): demo seed exercises all three instruction files

Seed pre-existing CLAUDE.md, AGENTS.md, and GEMINI.md in the demo repo and
target the demo pack fragment at all three, so the publish->sync walkthrough
lands a managed block in every file the major agents read while proving the
renderer leaves surrounding user content untouched. No renderer change: all
three targets already exist."
```

---

## Task 5: Docs — Antigravity vendor entry and changelog

Record the Google Antigravity check in the vendor-guidance log (its instruction files are already covered) and note the visual refresh plus the create-if-missing seed caveat in the changelog.

**Files:**
- Modify: `docs/roadmap/vendor-guidance-tracking.md` (append a log entry)
- Modify: `CHANGELOG.md` (two `[Unreleased]` bullets)

**Interfaces:** none (documentation only).

- [ ] **Step 1: Append the 2026-07-31 Antigravity log entry**

In `docs/roadmap/vendor-guidance-tracking.md`, after the last log entry (the `2026-07-30 xAI / Grok Build` line), add:

```markdown
- **2026-07-31** Google / Antigravity (Antigravity docs, GEMINI.md + AGENTS.md conventions). Google Antigravity reads a project-level `GEMINI.md` as its tool-specific, higher-priority instruction file, plus the cross-tool `AGENTS.md` family, and honors global copies under `~/.gemini/`. Additional workspace rules live in `.agent/rules/`. There is no `ANTIGRAVITY.md` target: covering CLAUDE.md, AGENTS.md, and GEMINI.md already covers Antigravity, so the demo seed now publishes to all three. No renderer changes; all three targets already ship in `internal/render/render.go`.
```

- [ ] **Step 2: Add the changelog bullets**

In `CHANGELOG.md`, replace:

```markdown
  `--demo` seeds a deterministic fictional org plus a local pack repo and governed repo
  for an end-to-end publish→sync walkthrough. Portal-published tags are unsigned in v1.

Planned — see `docs/roadmap/`:
```

with:

```markdown
  `--demo` seeds a deterministic fictional org plus a local pack repo and governed repo
  for an end-to-end publish→sync walkthrough. Portal-published tags are unsigned in v1.
- Portal visual refresh: fixed sidebar shell with active-page nav, refined
  enterprise-light design tokens (4px spacing scale, tabular-nums metrics,
  WCAG-AA status pills), and restyled deterministic SVG charts.
- `esc serve --demo` now seeds pre-existing `CLAUDE.md`, `AGENTS.md`, and
  `GEMINI.md` in the demo repo, and the demo pack publishes to all three, so a
  publish→sync lands a managed block in every file while leaving the seeded
  content untouched. Seeding stays create-if-missing: delete
  `~/.escapement/server/demo-repo` to regenerate it.

Planned — see `docs/roadmap/`:
```

- [ ] **Step 3: Commit**

```bash
git add docs/roadmap/vendor-guidance-tracking.md CHANGELOG.md
git commit -m "docs: Antigravity vendor entry and portal refresh changelog

Record the 2026-07-31 Google Antigravity check (its GEMINI.md/AGENTS.md
targets are already covered) and note the visual refresh plus the
create-if-missing demo-seed caveat in the changelog."
```

---

## Task 6: Full verification and manual smoke

Final gate. No code changes; run every gate and a manual walkthrough of `esc serve --demo`.

**Files:** none.

- [ ] **Step 1: Format check**

Run: `gofmt -l .`
Expected: no output (no unformatted files).

- [ ] **Step 2: Vet**

Run: `go vet ./...`
Expected: no output.

- [ ] **Step 3: Full test suite**

Run: `go test ./...`
Expected: PASS across all packages.

- [ ] **Step 4: Build and launch the demo**

Run: `go build -o /tmp/esc ./cmd/esc` and launch `esc serve --demo` against a fresh, throwaway data dir so the seed runs (delete or point away from any existing `~/.escapement/server`). Note the printed local URL.

- [ ] **Step 5: Manual visual walkthrough**

In a browser, confirm each item, then stop the server:
- Sidebar renders on every page; the active nav item is highlighted and matches the current page.
- Org name shows in the sidebar footer; `esc <version>` shows beneath it.
- Overview: stat hero row; drift/stale numbers tinted when nonzero; adoption chart uses the new palette and gridlines; model chips render.
- Fleet: filter row styled; table rows hover; status pills use the four AA color pairs; repo paths render.
- Rule Packs: version-badge cards; pack view shows fragments; the editor textarea is monospace; "Preview diff" shows add lines green and remove lines red.
- Usage: filter row + selects/buttons styled; both charts restyled; summary table with tabular-num figures.

- [ ] **Step 6: Confirm the demo landed all three files**

List the demo repo dir and confirm `CLAUDE.md`, `AGENTS.md`, `GEMINI.md` exist; after publishing a rule from the portal and running `esc sync` in the demo repo, confirm each file gained an `org-baseline` managed block below its seeded "Payments service" content.

- [ ] **Step 7: No commit needed** — verification only. The plan is complete when Steps 1-6 pass.

---

## Assumptions and Open Questions

- **Overview "recent-activity" section (spec §2):** not implemented — there is no recent-activity data in `overviewData`, and this plan adds no handler/store changes (Global Constraint: no behavior changes). The overview's existing adoption chart + models-in-use sections serve as the posture view. If a real activity feed is wanted, it needs a separate spec adding data to the store and handler.
- **Sidebar org name:** sourced from `s.Store.Registry().Org.Name`. On a store with no `registry.json` (e.g. `newTestServer`) this is empty and the footer shows only the version — acceptable, no crash.
- **Page `<title>`/`<h1>` reuse:** page title blocks are trimmed to plain section names and reused for both the tab title and the page `<h1>`; the brand moves to the sidebar. `pack.html` keeps its content `<h2>` (pack name + version) beneath the new `<h1>` — a minor, harmless heading duplication left as-is to avoid touching the tested detail markup.
- **Diff format:** coloring assumes `git diff --no-index` unified-diff prefixes (`+`, `-`, `+++`, `---`, `@@`, `diff `, `index `), confirmed against `publish.go:261`. `TestDiffColoring` guards it.
