# Portal Frontend Modernization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Modernize the `esc` admin portal with platform-CSS polish (view transitions, responsive layout, segmented filters, focus rings), a strict security-header baseline, and a vendored htmx scoped to exactly three progressively-enhanced interactions, without adding a Go dependency or a runtime network fetch.

**Architecture:** The portal stays server-rendered Go `html/template` with all assets embedded via `go:embed`. htmx 2.0.x is vendored as a single embedded JS asset, hardened by a separate config file (CSP forbids inline script), and used only to swap named template regions that also render as complete pages when the `HX-Request` header is absent. Every response carries a strict CSP and `X-Content-Type-Options: nosniff`.

**Tech Stack:** Go 1.24 (`html/template`, `net/http`, stdlib only), platform CSS (`@view-transition`, `:focus-visible`, container/media queries), vendored htmx 2.0.9 (JS asset, not a Go dependency).

## Global Constraints

Every task's requirements implicitly include this section. Values are copied verbatim from the decided spec (`docs/superpowers/specs/2026-08-01-portal-frontend-stack-options.md`).

- **No CDN or external request, ever.** Assets ship inside the `esc` binary via `go:embed`. htmx is vendored and embedded, never fetched at runtime.
- **Single Go dependency policy holds:** `gopkg.in/yaml.v3` is the only Go dependency. htmx is a vendored JS *asset*, not a Go module. No new Go dependency may be added.
- **Strict CSP, exact string:** `Content-Security-Policy: default-src 'none'; style-src 'self'; script-src 'self'; img-src 'self'; connect-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'`. Also `X-Content-Type-Options: nosniff`. No inline `<script>`, no inline `style=` attributes, no `eval`.
- **Server stays source of truth:** HTML rendered by Go templates. No JSON API is introduced for the UI's sake. htmx swaps HTML fragments.
- **Graceful degradation is a test gate, not a rule:** every route renders a complete page; fragments are returned only when the `HX-Request` header is present.
- **htmx hardening (all required):** `htmx.config.allowEval = false`, `htmx.config.includeIndicatorStyles = false`, `htmx.config.historyEnabled = false`, `htmx.config.selfRequestsOnly = true`; no `hx-on` attributes anywhere; `hx-disable` wraps ALL server-rendered pack/markdown (`mdHTML`) content.
- **Cookie:** `HttpOnly`, `SameSite=Strict`, and `Secure` when served over TLS.
- **A11y clauses:** `prefers-reduced-motion: reduce` disables view-transition animations; every htmx-swapped region has `aria-live="polite"`; sortable headers are real buttons with `aria-sort`; charts get `role="img"` and `<title>`; tables get `scope="col"` headers and empty states.
- **Vendored version pinned with committed checksum;** every version bump gets a source diff review.
- **Repo gates:** `gofmt -w .`, `go vet ./...`, `go test ./...` must pass before any task is claimed done. Conventional-commit messages, no AI co-authorship trailers, no em-dashes in user-facing copy/docs.

---

### Task 1: Hardening headers and session-cookie posture

Independent, ships first. Applies the strict CSP and `nosniff` to every portal response (pages, static assets, redirects, 401/404), and upgrades the session cookie to `SameSite=Strict` + `Secure`-when-TLS.

**Files:**
- Modify: `internal/portal/web/server.go` (add `contentSecurityPolicy` const + `securityHeaders` middleware near the other middleware around lines 194-201; wrap in `Handler()` at line 191; change cookie at lines 221-227)
- Test: `internal/portal/web/server_test.go`

**Interfaces:**
- Produces: `func securityHeaders(h http.Handler) http.Handler`; `const contentSecurityPolicy string`. These are consumed by no later task directly but must remain wrapping the whole handler.

- [ ] **Step 1: Write the failing test**

Add to `internal/portal/web/server_test.go`:

```go
import "crypto/tls" // add to the existing import block

func TestSecurityHeadersOnEveryRoute(t *testing.T) {
	const wantCSP = "default-src 'none'; style-src 'self'; script-src 'self'; img-src 'self'; connect-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'"
	h := newTestServer(t, "").Handler()
	for _, p := range []string{"/", "/fleet", "/packs", "/usage", "/static/style.css", "/nope"} {
		rr := get(t, h, p, nil)
		if got := rr.Header().Get("Content-Security-Policy"); got != wantCSP {
			t.Fatalf("%s: CSP = %q", p, got)
		}
		if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Fatalf("%s: nosniff = %q", p, got)
		}
	}
	// Unauthorized responses still carry the headers.
	ha := newTestServer(t, "sekrit").Handler()
	if rr := get(t, ha, "/", nil); rr.Code != 401 || rr.Header().Get("Content-Security-Policy") != wantCSP {
		t.Fatalf("401 missing CSP: code=%d", rr.Code)
	}
}

func TestSessionCookieHardening(t *testing.T) {
	h := newTestServer(t, "sekrit").Handler()

	// Plain HTTP: Strict + HttpOnly, not Secure.
	rr := get(t, h, "/?token=sekrit", nil)
	c := findCookie(t, rr, "esc_session")
	if c.SameSite != http.SameSiteStrictMode || !c.HttpOnly || c.Secure {
		t.Fatalf("http cookie: samesite=%v httponly=%v secure=%v", c.SameSite, c.HttpOnly, c.Secure)
	}

	// TLS: Secure set.
	req := httptest.NewRequest("GET", "/?token=sekrit", nil)
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !findCookie(t, rec, "esc_session").Secure {
		t.Fatal("tls cookie must be Secure")
	}
}

func findCookie(t *testing.T, rr *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, c := range rr.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("cookie %q not set", name)
	return nil
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/portal/web/ -run 'TestSecurityHeaders|TestSessionCookie' -v`
Expected: FAIL (headers absent; cookie is `SameSiteLaxMode`).

- [ ] **Step 3: Add the CSP constant and middleware**

In `internal/portal/web/server.go`, add after the `noStore` function (around line 201):

```go
// contentSecurityPolicy is the strict CSP applied to every portal response.
// default-src 'none' denies everything not explicitly allowed; connect-src
// 'self' is required for htmx XHR (it does not fall back from default-src).
const contentSecurityPolicy = "default-src 'none'; style-src 'self'; script-src 'self'; img-src 'self'; connect-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'"

// securityHeaders sets the CSP and nosniff header on every response. It wraps
// the entire handler, so pages, static assets, redirects, and error responses
// all carry the same baseline.
func securityHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		h.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 4: Wrap the handler**

In `Handler()`, change the final return (line 191) from:

```go
	return s.withAuth(mux)
```

to:

```go
	return securityHeaders(s.withAuth(mux))
```

- [ ] **Step 5: Harden the cookie**

Replace the `http.SetCookie` block in `withAuth` (lines 221-227) with:

```go
			http.SetCookie(w, &http.Cookie{
				Name:     sessionCookie,
				Value:    s.Token,
				Path:     "/",
				HttpOnly: true,
				Secure:   r.TLS != nil,
				SameSite: http.SameSiteStrictMode,
			})
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `gofmt -w . && go vet ./... && go test ./internal/portal/web/ -v`
Expected: PASS (including the pre-existing `TestAuthRequired`, which only asserts `HttpOnly`).

- [ ] **Step 7: Commit**

```bash
git add internal/portal/web/server.go internal/portal/web/server_test.go
git commit -m "feat(portal): strict CSP, nosniff, and Strict/Secure session cookie"
```

---

### Task 2: Modern CSS layer

Pure CSS plus small template edits for markup that CSS cannot restyle in place. Adds cross-document view transitions (with reduced-motion opt-out), a responsive sidebar that collapses below 700px, segmented-control filter rows, `:focus-visible` rings, a skip link, and the `.htmx-indicator` rules (config disables htmx's injected styles). No htmx behavior yet; the indicator rules are inert until Task 3.

**Files:**
- Modify: `internal/portal/web/static/style.css` (append new blocks)
- Modify: `internal/portal/web/templates/layout.html` (skip link + `id="main"`)
- Modify: `internal/portal/web/templates/fleet.html` (status filters -> segmented control)
- Modify: `internal/portal/web/templates/usage.html` (day filters -> segmented control)
- Test: `internal/portal/web/server_test.go`

**Interfaces:**
- Produces: CSS classes `.segmented`, `.seg`, `.seg.active`, `.th-sort`, `.skip`, `.htmx-indicator`. Consumed by Tasks 3, 4c.
- Note: `.seg.active` uses `<span class="seg active">` for the current filter and `<a class="seg">` for others, replacing the previous `<strong>`/`<a>` + `·` markup.

- [ ] **Step 1: Write the failing test**

Add to `internal/portal/web/server_test.go`:

```go
func TestModernCSSMarkup(t *testing.T) {
	h := newTestServer(t, "").Handler()

	layout := get(t, h, "/", nil).Body.String()
	if !strings.Contains(layout, `class="skip"`) || !strings.Contains(layout, `id="main"`) {
		t.Fatal("layout missing skip link / main landmark")
	}

	fleet := get(t, h, "/fleet", nil).Body.String()
	if !strings.Contains(fleet, `class="segmented"`) || !strings.Contains(fleet, `class="seg active"`) {
		t.Fatal("fleet filters not segmented")
	}

	usage := get(t, h, "/usage", nil).Body.String()
	if !strings.Contains(usage, `class="segmented"`) {
		t.Fatal("usage day filters not segmented")
	}

	css := get(t, h, "/static/style.css", nil).Body.String()
	for _, want := range []string{"@view-transition", "prefers-reduced-motion", ":focus-visible", "@media (max-width: 700px)", ".htmx-indicator"} {
		if !strings.Contains(css, want) {
			t.Fatalf("style.css missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/portal/web/ -run TestModernCSSMarkup -v`
Expected: FAIL.

- [ ] **Step 3: Append the CSS**

Append to `internal/portal/web/static/style.css`:

```css
/* ---- Cross-document view transitions (opt-in; silent no-op where unsupported) ---- */
@view-transition { navigation: auto; }
@media (prefers-reduced-motion: reduce) {
  ::view-transition-group(*),
  ::view-transition-old(*),
  ::view-transition-new(*) {
    animation: none !important;
  }
}

/* ---- Skip link ---- */
.skip {
  position: absolute;
  left: -9999px;
  top: 0;
  z-index: 10;
  background: var(--surface);
  color: var(--accent);
  padding: var(--s2) var(--s3);
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-sm);
}
.skip:focus { left: var(--s3); top: var(--s3); }

/* ---- Focus-visible rings ---- */
.sidebar nav a:focus-visible,
a.card:focus-visible,
.seg:focus-visible,
.th-sort:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

/* ---- Segmented controls (filter rows) ---- */
.segmented {
  display: inline-flex;
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-sm);
  overflow: hidden;
}
.segmented .seg {
  padding: var(--s2) var(--s3);
  font-size: 14px;
  color: var(--muted);
  background: var(--surface);
  border-right: 1px solid var(--border);
}
.segmented .seg:last-child { border-right: none; }
.segmented a.seg:hover { background: var(--page); color: var(--accent); text-decoration: none; }
.segmented .seg.active { background: var(--accent-weak); color: var(--accent); font-weight: 600; }

/* ---- Sortable table headers ---- */
th form { display: inline; margin: 0; }
.th-sort {
  background: none;
  border: none;
  margin: 0;
  padding: 0;
  font: inherit;
  font-size: 13px;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: .03em;
  color: var(--muted);
  cursor: pointer;
}
.th-sort:hover { color: var(--accent); }
th[aria-sort="ascending"] .th-sort::after { content: " \25B2"; }
th[aria-sort="descending"] .th-sort::after { content: " \25BC"; }

/* ---- htmx indicator (includeIndicatorStyles is disabled in config) ---- */
.htmx-indicator { opacity: 0; transition: opacity .2s ease; }
.htmx-request .htmx-indicator,
.htmx-request.htmx-indicator { opacity: 1; }

/* ---- Responsive: collapse the fixed sidebar into a top bar below 700px ---- */
@media (max-width: 700px) {
  .sidebar {
    position: static;
    width: auto;
    flex-direction: row;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s3);
    border-right: none;
    border-bottom: 1px solid var(--border);
    padding: var(--s3) var(--s4);
  }
  .sidebar .brand { margin-bottom: 0; }
  .sidebar nav { flex-direction: row; flex-wrap: wrap; gap: var(--s2); }
  .sidebar-foot { margin-top: 0; margin-left: auto; padding: 0; }
  .content { margin-left: 0; max-width: 100%; padding: var(--s5) var(--s4); }
}
```

- [ ] **Step 4: Add the skip link and main landmark in layout.html**

In `internal/portal/web/templates/layout.html`, immediately after `<body>` (line 9) add:

```html
<a class="skip" href="#main">Skip to content</a>
```

and change `<main class="content">` (line 23) to:

```html
<main class="content" id="main">
```

- [ ] **Step 5: Convert fleet filters to a segmented control**

Replace the `<p class="filters">…</p>` block (lines 4-10) in `internal/portal/web/templates/fleet.html` with:

```html
<div class="filters">
  <div class="segmented" role="group" aria-label="Filter by status">
    {{if eq .Status ""}}<span class="seg active" aria-current="true">All</span>{{else}}<a class="seg" href="/fleet">All</a>{{end}}
    {{if eq .Status "in-sync"}}<span class="seg active" aria-current="true">In sync</span>{{else}}<a class="seg" href="/fleet?status=in-sync">In sync</a>{{end}}
    {{if eq .Status "drifted"}}<span class="seg active" aria-current="true">Drifted</span>{{else}}<a class="seg" href="/fleet?status=drifted">Drifted</a>{{end}}
    {{if eq .Status "stale"}}<span class="seg active" aria-current="true">Stale</span>{{else}}<a class="seg" href="/fleet?status=stale">Stale</a>{{end}}
    {{if eq .Status "ungoverned"}}<span class="seg active" aria-current="true">Ungoverned</span>{{else}}<a class="seg" href="/fleet?status=ungoverned">Ungoverned</a>{{end}}
  </div>
</div>
```

- [ ] **Step 6: Convert usage day filters to a segmented control**

In `internal/portal/web/templates/usage.html`, replace the trailing part of the filter form (the `<button type="submit">Filter</button>` line through line 19's `</form>`, i.e. lines 14-19) with:

```html
  <button type="submit">Filter</button>
  <div class="segmented" role="group" aria-label="Time range">
    {{if eq .Days 7}}<span class="seg active" aria-current="true">7d</span>{{else}}<a class="seg" href="?team={{.Team}}&amp;model={{.Model}}&amp;days=7">7d</a>{{end}}
    {{if eq .Days 30}}<span class="seg active" aria-current="true">30d</span>{{else}}<a class="seg" href="?team={{.Team}}&amp;model={{.Model}}&amp;days=30">30d</a>{{end}}
    {{if eq .Days 60}}<span class="seg active" aria-current="true">60d</span>{{else}}<a class="seg" href="?team={{.Team}}&amp;model={{.Model}}&amp;days=60">60d</a>{{end}}
  </div>
</form>
```

(The `hx-get` attributes for these links are added in Task 4b.)

- [ ] **Step 7: Run tests to verify they pass**

Run: `gofmt -w . && go vet ./... && go test ./internal/portal/web/ -v`
Expected: PASS. `TestFleetFilters` still counts pills (unaffected), `TestUsagePage` still finds both charts.

- [ ] **Step 8: Commit**

```bash
git add internal/portal/web/static/style.css internal/portal/web/templates/layout.html internal/portal/web/templates/fleet.html internal/portal/web/templates/usage.html internal/portal/web/server_test.go
git commit -m "feat(portal): view transitions, responsive sidebar, segmented filters, focus rings"
```

---

### Task 3: Vendor and harden htmx

Vendors htmx 2.0.9 as an embedded static asset, ships a CSP-safe config file that runs before htmx's initial process pass, wires both `<script>` tags into the layout, and applies the load-bearing `hx-disable` mitigation around `mdHTML` output. No interaction wiring yet; this task proves the assets embed, serve, and load under CSP.

**Files:**
- Create: `internal/portal/web/static/htmx.min.js` (downloaded, pinned)
- Create: `internal/portal/web/static/htmx-config.js`
- Create: `internal/portal/web/HTMX-VENDOR.md` (version + sha256 + upgrade procedure; not embedded, not served)
- Modify: `internal/portal/web/templates/layout.html` (two `<script>` tags)
- Modify: `internal/portal/web/templates/pack.html` (`hx-disable` around fragments)
- Test: `internal/portal/web/server_test.go`

**Interfaces:**
- Produces: served assets `/static/htmx.min.js` and `/static/htmx-config.js` (both embedded by the existing `//go:embed static/*` at server.go:28). Consumed by Tasks 4a-4c (the loaded runtime) — those tasks do not depend on this task at the Go/test level, since fragments are exercised directly via the `HX-Request` header.

- [ ] **Step 1: Download and pin htmx**

Run (network is used at vendor time only; the asset is committed and thereafter embedded):

```bash
cd internal/portal/web/static
curl -fsSL -o htmx.min.js https://github.com/bigskysoftware/htmx/releases/download/v2.0.9/htmx.min.js
shasum -a 256 htmx.min.js
```

If `v2.0.9` is not published, use the highest `2.0.x` stable release tag from `https://github.com/bigskysoftware/htmx/releases` (do NOT use the 4.0 beta), and record the actual version below. Fallback mirror: `https://unpkg.com/htmx.org@2.0.9/dist/htmx.min.js`.

- [ ] **Step 2: Record the checksum**

Create `internal/portal/web/HTMX-VENDOR.md` (fill `<sha256 from step 1>` with the printed hash):

```markdown
# Vendored htmx

- File: `static/htmx.min.js` (embedded via `//go:embed static/*`, served at `/static/htmx.min.js`)
- Version: 2.0.9
- Source: https://github.com/bigskysoftware/htmx/releases/tag/v2.0.9
- SHA-256: <sha256 from step 1>

This is a vendored JavaScript asset, not a Go dependency. It is never fetched
at runtime. Hardening lives in `static/htmx-config.js`.

## Upgrading

1. Download the new `htmx.min.js` from the release tag above.
2. Diff-review the source against the current file.
3. Update Version and SHA-256 here.
4. Re-run `go test ./internal/portal/...`.
```

- [ ] **Step 3: Write the hardened config**

Create `internal/portal/web/static/htmx-config.js`:

```javascript
// Portal htmx hardening. Loaded immediately after htmx.min.js and before
// DOMContentLoaded, so these settings apply to htmx's initial process pass.
// A strict CSP forbids inline <script>, so this configuration lives in its
// own file rather than an inline tag or hx-config meta attribute.
htmx.config.allowEval = false;             // no eval-based features
htmx.config.includeIndicatorStyles = false; // do not inject an inline <style> (CSP blocks it); .htmx-indicator lives in style.css
htmx.config.historyEnabled = false;         // no localStorage page snapshots of sensitive usage/cost pages
htmx.config.selfRequestsOnly = true;        // default in 2.x; asserted explicitly
```

- [ ] **Step 4: Add the script tags in layout.html**

In `internal/portal/web/templates/layout.html`, after the stylesheet link (line 7) add:

```html
<script src="/static/htmx.min.js" defer></script>
<script src="/static/htmx-config.js" defer></script>
```

`defer` preserves document order and runs both before `DOMContentLoaded`: htmx registers its init listener, then the config mutates `htmx.config`, then htmx processes the DOM with the hardened config.

- [ ] **Step 5: Wrap pack markdown with hx-disable**

In `internal/portal/web/templates/pack.html`, change the fragments `<section>` (line 13) from `<section>` to:

```html
<section hx-disable>
```

This wraps every `{{.HTML}}` (`mdHTML` output, which is `template.HTML` and pack-author-controlled) so htmx never processes attacker-influenced markup. `hx-disable` on an ancestor covers all descendants.

- [ ] **Step 6: Write the failing test**

Add to `internal/portal/web/server_test.go`:

```go
func TestHtmxAssetsServed(t *testing.T) {
	h := newTestServer(t, "").Handler()

	js := get(t, h, "/static/htmx.min.js", nil)
	if js.Code != 200 || len(js.Body.String()) < 1000 {
		t.Fatalf("htmx.min.js: code=%d len=%d", js.Code, js.Body.Len())
	}

	cfg := get(t, h, "/static/htmx-config.js", nil).Body.String()
	for _, want := range []string{"allowEval = false", "includeIndicatorStyles = false", "historyEnabled = false", "selfRequestsOnly = true"} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("htmx-config.js missing %q", want)
		}
	}

	layout := get(t, h, "/", nil).Body.String()
	if !strings.Contains(layout, `src="/static/htmx.min.js"`) || !strings.Contains(layout, `src="/static/htmx-config.js"`) {
		t.Fatal("layout missing htmx script tags")
	}
}

func TestPackMarkdownHasHxDisable(t *testing.T) {
	h := newTestServerWithPacks(t).Handler()
	detail := get(t, h, "/packs/org-baseline", nil).Body.String()
	if !strings.Contains(detail, "hx-disable") {
		t.Fatal("pack fragments must be wrapped with hx-disable")
	}
}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `gofmt -w . && go vet ./... && go test ./internal/portal/web/ -v`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/portal/web/static/htmx.min.js internal/portal/web/static/htmx-config.js internal/portal/web/HTMX-VENDOR.md internal/portal/web/templates/layout.html internal/portal/web/templates/pack.html internal/portal/web/server_test.go
git commit -m "feat(portal): vendor htmx 2.0.9 with hardened config and hx-disable on pack markdown"
```

---

### Task 4a: htmx diff preview (pack-edit) with full-page fallback

Introduces the shared `isHX` / `renderFragment` helpers and the first interaction: the "Preview diff" button swaps only a `#diff-region`. Without the `HX-Request` header the same POST renders the full `pack_edit` page (existing behavior preserved).

**Files:**
- Modify: `internal/portal/web/server.go` (`isHX`, `renderFragment`; `handlePackPublish` diff branch)
- Modify: `internal/portal/web/templates/pack_edit.html` (region + hx attributes + `define "diff-region"`)
- Test: `internal/portal/web/packs_test.go`

**Interfaces:**
- Produces: `func isHX(r *http.Request) bool` (true when header `HX-Request == "true"`); `func (s *Server) renderFragment(w http.ResponseWriter, page, block string, data any)` — executes a single named template block on `s.pages[page]` with `Content-Type: text/html; charset=utf-8` and 200 status. Both consumed by Tasks 4b and 4c.
- Template block produced: `{{define "diff-region"}}` in `pack_edit.html`.

- [ ] **Step 1: Write the failing test**

Add to `internal/portal/web/packs_test.go`:

```go
func TestDiffPreviewFragment(t *testing.T) {
	s := newTestServerWithPacks(t)
	form := url.Values{
		"frag":    {"rules/security.md"},
		"content": {"---\ntargets: [claude, agents]\n---\n# Security\n\n- Added rule.\n"},
		"version": {"1.3.0"},
		"action":  {"diff"},
	}
	newReq := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/packs/org-baseline/publish", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, req)
		return rr
	}
	frag := newReq().Body.String()
	if strings.Contains(frag, "<html") {
		t.Fatal("HX diff response must be a fragment, not a full page")
	}
	if !strings.Contains(frag, `class="line add"`) {
		t.Fatalf("fragment missing diff: %s", frag)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/portal/web/ -run TestDiffPreviewFragment -v`
Expected: FAIL (full page returned; `renderFragment` undefined).

- [ ] **Step 3: Add the shared helpers**

In `internal/portal/web/server.go`, add after `renderStatus` (around line 167):

```go
// isHX reports whether the request came from htmx, which sets HX-Request: true
// on every AJAX request. Handlers use it to return a fragment instead of a
// full page.
func isHX(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }

// renderFragment executes a single named template block (a swappable region)
// from the given page's template set, for htmx partial swaps. Unlike render it
// emits no <html> shell.
func (s *Server) renderFragment(w http.ResponseWriter, page, block string, data any) {
	t, ok := s.pages[page]
	if !ok {
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, block, data); err != nil {
		log.Printf("portal: internal error rendering fragment %s/%s: %v", page, block, err)
	}
}
```

- [ ] **Step 4: Branch the diff handler**

In `handlePackPublish`, replace the `case "diff":` block (lines 460-474) with:

```go
	case "diff":
		diff, err := s.Packs.Diff(r.Context(), name, frag, []byte(content))
		if err != nil {
			serverError(w, err)
			return
		}
		data := packEditData{
			layoutData:       s.baseData("packs"),
			Name:             name,
			Frag:             frag,
			Content:          content,
			SuggestedVersion: version,
			Diff:             diff,
		}
		if isHX(r) {
			s.renderFragment(w, "pack_edit", "diff-region", data)
			return
		}
		s.render(w, "pack_edit", data)
```

- [ ] **Step 5: Restructure pack_edit.html**

Replace the entire `{{define "content"}}…{{end}}` block (lines 3-20) of `internal/portal/web/templates/pack_edit.html` with:

```html
{{define "content"}}
<h2>{{.Name}} <span class="muted">/ {{.Frag}}</span></h2>
<div id="diff-region" aria-live="polite">{{template "diff-region" .}}</div>
<form method="post" action="/packs/{{.Name}}/publish">
  <input type="hidden" name="frag" value="{{.Frag}}">
  <p><textarea name="content" rows="24" cols="90">{{.Content}}</textarea></p>
  <p>
    <label>Version
      <input name="version" value="{{.SuggestedVersion}}">
    </label>
  </p>
  <p>
    <button type="submit" name="action" value="diff"
      hx-post="/packs/{{.Name}}/publish" hx-target="#diff-region" hx-swap="innerHTML">Preview diff</button>
    <button type="submit" name="action" value="publish">Publish</button>
    <span class="htmx-indicator muted">Rendering diff&hellip;</span>
  </p>
</form>
{{end}}

{{define "diff-region"}}
{{if .Error}}<div class="error">{{.Error}}</div>{{end}}
{{if .Diff}}<pre class="diff">{{range diffLines .Diff}}<span class="line {{.Class}}">{{.Text}}</span>{{end}}</pre>{{end}}
{{end}}
```

The `Preview diff` button posts the whole form (htmx includes the clicked button's `name=action value=diff`). With JS off it submits normally and the server renders the full page. The `Publish` button is a plain submit (full navigation / redirect), unchanged.

- [ ] **Step 6: Run tests to verify they pass**

Run: `gofmt -w . && go vet ./... && go test ./internal/portal/web/ -v`
Expected: PASS. `TestDiffColoring` (no `HX-Request`) still gets a full page with the three diff classes; `TestPublishValidationErrorKeepsContent` still gets a 422 full page whose `#diff-region` shows the error.

- [ ] **Step 7: Commit**

```bash
git add internal/portal/web/server.go internal/portal/web/templates/pack_edit.html internal/portal/web/packs_test.go
git commit -m "feat(portal): htmx diff preview with full-page fallback"
```

---

### Task 4b: htmx usage filters with full-page fallback

The usage filter form and day-range links swap only a `#usage-results` region (charts + summary table). Without the `HX-Request` header, `/usage` renders the full page (existing behavior).

**Files:**
- Modify: `internal/portal/web/server.go` (`handleUsage` final render)
- Modify: `internal/portal/web/templates/usage.html` (region wrapper + `define "usage-results"` + hx attributes)
- Test: `internal/portal/web/usage_test.go`

**Interfaces:**
- Consumes: `isHX`, `renderFragment` (Task 4a).
- Produces: template block `{{define "usage-results"}}` in `usage.html`.

- [ ] **Step 1: Write the failing test**

Add to `internal/portal/web/usage_test.go`:

```go
func TestUsageFragment(t *testing.T) {
	dir := t.TempDir()
	epoch := time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
	if err := seed.Demo(dir, epoch); err != nil {
		t.Fatal(err)
	}
	st, _ := store.Open(dir)
	s := New(st, nil, "", "test")
	s.Now = func() time.Time { return epoch }
	h := s.Handler()

	frag := get(t, h, "/usage", map[string]string{"HX-Request": "true"}).Body.String()
	if strings.Contains(frag, "<html") {
		t.Fatal("HX usage response must be a fragment")
	}
	if strings.Count(frag, "<svg") != 2 {
		t.Fatalf("fragment should carry both charts: %d", strings.Count(frag, "<svg"))
	}
	full := get(t, h, "/usage", nil).Body.String()
	if !strings.Contains(full, "<html") {
		t.Fatal("no-HX usage response must be a full page")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/portal/web/ -run TestUsageFragment -v`
Expected: FAIL (full page returned for HX).

- [ ] **Step 3: Branch the usage handler**

In `handleUsage`, replace the final `s.render(w, "usage", data)` (line 676) with:

```go
	if isHX(r) {
		s.renderFragment(w, "usage", "usage-results", data)
		return
	}
	s.render(w, "usage", data)
```

- [ ] **Step 4: Restructure usage.html**

Replace the entire `{{define "content"}}…{{end}}` block of `internal/portal/web/templates/usage.html` with (note the form gains `hx-get`, day links gain `hx-get`, and the results move into a `define`):

```html
{{define "content"}}
<form class="filters" method="get" action="/usage"
  hx-get="/usage" hx-target="#usage-results" hx-swap="innerHTML">
  <input type="hidden" name="days" value="{{.Days}}">
  <select name="team" aria-label="Team">
    <option value="">All teams</option>
    {{range .Teams}}<option value="{{.ID}}"{{if eq .ID $.Team}} selected{{end}}>{{.Name}}</option>{{end}}
  </select>
  <select name="model" aria-label="Model">
    <option value="">All models</option>
    {{range .Models}}<option value="{{.}}"{{if eq . $.Model}} selected{{end}}>{{.}}</option>{{end}}
  </select>
  <button type="submit">Filter</button>
  <span class="htmx-indicator muted">Updating&hellip;</span>
  <div class="segmented" role="group" aria-label="Time range">
    {{if eq .Days 7}}<span class="seg active" aria-current="true">7d</span>{{else}}<a class="seg" href="?team={{.Team}}&amp;model={{.Model}}&amp;days=7" hx-get="/usage?team={{.Team}}&amp;model={{.Model}}&amp;days=7" hx-target="#usage-results" hx-swap="innerHTML">7d</a>{{end}}
    {{if eq .Days 30}}<span class="seg active" aria-current="true">30d</span>{{else}}<a class="seg" href="?team={{.Team}}&amp;model={{.Model}}&amp;days=30" hx-get="/usage?team={{.Team}}&amp;model={{.Model}}&amp;days=30" hx-target="#usage-results" hx-swap="innerHTML">30d</a>{{end}}
    {{if eq .Days 60}}<span class="seg active" aria-current="true">60d</span>{{else}}<a class="seg" href="?team={{.Team}}&amp;model={{.Model}}&amp;days=60" hx-get="/usage?team={{.Team}}&amp;model={{.Model}}&amp;days=60" hx-target="#usage-results" hx-swap="innerHTML">60d</a>{{end}}
  </div>
</form>
<div id="usage-results" aria-live="polite">{{template "usage-results" .}}</div>
{{end}}

{{define "usage-results"}}
<section>
  <h2>Tokens per day, by model</h2>
  {{.TokensChart}}
</section>
<section>
  <h2>Cost per day, by team</h2>
  {{.CostChart}}
</section>
<p class="muted">Aggregate usage by API workspace. Per-tool attribution arrives with MCP connection telemetry.</p>
<table>
  <thead>
    <tr>
      <th scope="col">Model</th>
      <th scope="col">Total tokens</th>
      <th scope="col">Est. cost</th>
      <th scope="col">Share</th>
    </tr>
  </thead>
  <tbody>
    {{range .Rows}}
    <tr>
      <td>{{.Model}}</td>
      <td>{{abbrev .Tokens}}</td>
      <td>{{printf "$%.2f" .Cost}}</td>
      <td>{{printf "%.0f%%" .Share}}</td>
    </tr>
    {{else}}
    <tr><td colspan="4" class="muted">No usage in this range.</td></tr>
    {{end}}
  </tbody>
  <tfoot>
    <tr>
      <td>Total</td>
      <td>{{abbrev .TotalTokens}}</td>
      <td>{{printf "$%.2f" .TotalCost}}</td>
      <td>100%</td>
    </tr>
  </tfoot>
</table>
{{end}}
```

(`scope="col"` and the empty-state row are added here; Task 6 verifies them.)

- [ ] **Step 5: Run tests to verify they pass**

Run: `gofmt -w . && go vet ./... && go test ./internal/portal/web/ -v`
Expected: PASS. `TestUsagePage` still finds both `<svg>` and the model text in the full page.

- [ ] **Step 6: Commit**

```bash
git add internal/portal/web/server.go internal/portal/web/templates/usage.html internal/portal/web/usage_test.go
git commit -m "feat(portal): htmx usage filters with full-page fallback"
```

---

### Task 4c: Server-side fleet column sorting, htmx-enhanced

Adds a new server-side sort (columns Repository, Last sync, Status) with a stable sort, real sortable-header `<button>`s carrying `aria-sort`, degrading to a GET form submit and enhanced by an htmx region swap of `#fleet-table`.

**Files:**
- Modify: `internal/portal/web/server.go` (`net/url` import; `colSort` type; `fleetSorts` map; `fleetHeader` helper; `fleetData` fields; `handleFleet`)
- Modify: `internal/portal/web/templates/fleet.html` (region wrapper + `define "fleet-table"` with sortable headers)
- Test: `internal/portal/web/fleet_test.go`

**Interfaces:**
- Consumes: `isHX`, `renderFragment` (Task 4a); `store.FleetRow` fields `RepoName string`, `Status string`, `LastSync time.Time`.
- Produces:
  - `type colSort struct { Href, Aria, NextDir string }` — `Href` is the toggle URL (status preserved), `Aria` is `"none"|"ascending"|"descending"`, `NextDir` is `"asc"|"desc"`.
  - `func fleetHeader(col, status, activeCol, dir string) colSort`.
  - `fleetData` gains `SortCol string`, `RepoSort, LastSyncSort, StatusSort colSort`.
  - Template block `{{define "fleet-table"}}` in `fleet.html`.
  - Sort query contract: `?sort=repo|last-sync|status&dir=asc|desc`; unknown `sort` = registry order, `dir` defaults to `asc`.

- [ ] **Step 1: Write the failing test**

Add to `internal/portal/web/fleet_test.go`:

```go
func TestFleetSort(t *testing.T) {
	dir := t.TempDir()
	epoch := time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
	if err := seed.Demo(dir, epoch); err != nil {
		t.Fatal(err)
	}
	st, _ := store.Open(dir)
	h := New(st, nil, "", "test").Handler()

	// aria-sort is set on the active column, ascending by default.
	asc := get(t, h, "/fleet?sort=repo", nil).Body.String()
	if !strings.Contains(asc, `aria-sort="ascending"`) {
		t.Fatal("active column should carry aria-sort=ascending")
	}
	// Toggling the same column flips direction, reflected in the header link.
	desc := get(t, h, "/fleet?sort=repo&dir=desc", nil).Body.String()
	if !strings.Contains(desc, `aria-sort="descending"`) {
		t.Fatal("dir=desc should render aria-sort=descending")
	}
	// HX request returns only the table fragment.
	frag := get(t, h, "/fleet?sort=status", map[string]string{"HX-Request": "true"}).Body.String()
	if strings.Contains(frag, "<html") || !strings.Contains(frag, "<table") {
		t.Fatal("HX sort must return the table fragment only")
	}
}

func TestFleetEmptyState(t *testing.T) {
	h := newTestServer(t, "").Handler() // empty store: no repos
	body := get(t, h, "/fleet", nil).Body.String()
	if !strings.Contains(body, "No repos match") {
		t.Fatal("empty fleet table needs an empty state")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/portal/web/ -run 'TestFleetSort|TestFleetEmptyState' -v`
Expected: FAIL.

- [ ] **Step 3: Add the sort import**

In `internal/portal/web/server.go`, add `"net/url"` to the import block (with the other stdlib imports).

- [ ] **Step 4: Add the sort machinery**

In `internal/portal/web/server.go`, replace the `fleetData` struct (lines 281-285) with:

```go
// colSort is one sortable header's rendered state: the toggle URL (current
// status preserved), the aria-sort value, and the direction a click will
// request next.
type colSort struct {
	Href    string // "/fleet?status=..&sort=..&dir=.."
	Aria    string // "none" | "ascending" | "descending"
	NextDir string // "asc" | "desc"
}

// fleetSorts maps a sort key to its less-than comparator over FleetRow.
var fleetSorts = map[string]func(a, b store.FleetRow) bool{
	"repo":      func(a, b store.FleetRow) bool { return a.RepoName < b.RepoName },
	"status":    func(a, b store.FleetRow) bool { return a.Status < b.Status },
	"last-sync": func(a, b store.FleetRow) bool { return a.LastSync.Before(b.LastSync) },
}

// fleetHeader computes a sortable column's rendered state. When col is the
// active sort, aria-sort reflects dir and a click toggles direction;
// otherwise a click sorts ascending.
func fleetHeader(col, status, activeCol, dir string) colSort {
	h := colSort{Aria: "none", NextDir: "asc"}
	if col == activeCol {
		if dir == "desc" {
			h.Aria, h.NextDir = "descending", "asc"
		} else {
			h.Aria, h.NextDir = "ascending", "desc"
		}
	}
	h.Href = fmt.Sprintf("/fleet?status=%s&sort=%s&dir=%s", url.QueryEscape(status), col, h.NextDir)
	return h
}

// fleetData extends layoutData with the (possibly filtered, possibly sorted)
// fleet rows, the active status filter, and per-column sort header state.
type fleetData struct {
	layoutData
	Rows         []store.FleetRow
	Status       string // "" = all
	SortCol      string // "", "repo", "status", "last-sync"
	RepoSort     colSort
	LastSyncSort colSort
	StatusSort   colSort
}
```

- [ ] **Step 5: Sort in the handler**

In `handleFleet`, replace the `data := fleetData{...}` construction and the preceding filter tail (lines 304-313) with:

```go
	} else {
		status = ""
	}

	sortCol := r.URL.Query().Get("sort")
	dir := r.URL.Query().Get("dir")
	if dir != "desc" {
		dir = "asc"
	}
	if less, ok := fleetSorts[sortCol]; ok {
		sort.SliceStable(rows, func(i, j int) bool {
			if dir == "desc" {
				return less(rows[j], rows[i])
			}
			return less(rows[i], rows[j])
		})
	} else {
		sortCol = ""
	}

	data := fleetData{
		layoutData:   s.baseData("fleet"),
		Rows:         rows,
		Status:       status,
		SortCol:      sortCol,
		RepoSort:     fleetHeader("repo", status, sortCol, dir),
		LastSyncSort: fleetHeader("last-sync", status, sortCol, dir),
		StatusSort:   fleetHeader("status", status, sortCol, dir),
	}
	if isHX(r) {
		s.renderFragment(w, "fleet", "fleet-table", data)
		return
	}
	s.render(w, "fleet", data)
```

- [ ] **Step 6: Add the sortable table to fleet.html**

In `internal/portal/web/templates/fleet.html`, replace the `<div id="fleet-table" ...>` placeholder from Task 2's content block with a wrapper that renders the region, then define the region. The content block's `<div class="filters">…</div>` from Task 2 stays; below it, ensure:

```html
<div id="fleet-table" aria-live="polite">{{template "fleet-table" .}}</div>
{{end}}

{{define "fleet-table"}}
<table>
  <thead>
    <tr>
      <th scope="col">Department</th>
      <th scope="col">Team</th>
      <th scope="col" aria-sort="{{.RepoSort.Aria}}">
        <form method="get" action="/fleet">
          <input type="hidden" name="status" value="{{.Status}}">
          <input type="hidden" name="dir" value="{{.RepoSort.NextDir}}">
          <button class="th-sort" type="submit" name="sort" value="repo"
            hx-get="{{.RepoSort.Href}}" hx-target="#fleet-table" hx-swap="innerHTML">Repository</button>
        </form>
      </th>
      <th scope="col">Rule packs</th>
      <th scope="col" aria-sort="{{.LastSyncSort.Aria}}">
        <form method="get" action="/fleet">
          <input type="hidden" name="status" value="{{.Status}}">
          <input type="hidden" name="dir" value="{{.LastSyncSort.NextDir}}">
          <button class="th-sort" type="submit" name="sort" value="last-sync"
            hx-get="{{.LastSyncSort.Href}}" hx-target="#fleet-table" hx-swap="innerHTML">Last sync</button>
        </form>
      </th>
      <th scope="col" aria-sort="{{.StatusSort.Aria}}">
        <form method="get" action="/fleet">
          <input type="hidden" name="status" value="{{.Status}}">
          <input type="hidden" name="dir" value="{{.StatusSort.NextDir}}">
          <button class="th-sort" type="submit" name="sort" value="status"
            hx-get="{{.StatusSort.Href}}" hx-target="#fleet-table" hx-swap="innerHTML">Status</button>
        </form>
      </th>
      <th scope="col">Agent tools</th>
    </tr>
  </thead>
  <tbody>
    {{range .Rows}}
    <tr>
      <td>{{.DeptName}}</td>
      <td>{{.TeamName}}</td>
      <td>{{.RepoName}}</td>
      <td>{{range $i, $p := .Packs}}{{if $i}}, {{end}}{{$p}}{{end}}</td>
      <td>{{fmtTime .LastSync}}</td>
      <td><span class="pill {{.Status}}">{{.Status}}</span></td>
      <td>{{range $i, $t := .Tools}}{{if $i}}, {{end}}{{$t}}{{end}}</td>
    </tr>
    {{else}}
    <tr><td colspan="7" class="muted">No repos match this filter.</td></tr>
    {{end}}
  </tbody>
</table>
{{end}}
```

The header is a real `<button>` (with `aria-sort` on its `<th>`) inside a GET `<form>`: JS off, it submits and navigates a full page (graceful degradation); with htmx, `hx-get` swaps only `#fleet-table`.

- [ ] **Step 7: Run tests to verify they pass**

Run: `gofmt -w . && go vet ./... && go test ./internal/portal/web/ -v`
Expected: PASS. `TestFleetFilters` still counts 40/4 pills (empty-state row appears only at zero rows).

- [ ] **Step 8: Commit**

```bash
git add internal/portal/web/server.go internal/portal/web/templates/fleet.html internal/portal/web/fleet_test.go
git commit -m "feat(portal): server-side fleet column sorting, htmx-enhanced"
```

---

### Task 5: Graceful-degradation test gate

A single integration test file asserting the degradation contract: every registered page route returns a complete page without `HX-Request`, and the three enhanced endpoints return fragments only when the header is present.

**Files:**
- Create: `internal/portal/web/degrade_test.go`

**Interfaces:**
- Consumes: test helpers `newTestServer`, `newTestServerWithPacks`, `get` (existing).

- [ ] **Step 1: Write the test**

Create `internal/portal/web/degrade_test.go`:

```go
package web

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// pageRoutes are the full-page GET routes the degradation contract covers.
var pageRoutes = []string{"/", "/fleet", "/usage", "/packs"}

func TestEveryRouteRendersFullPageWithoutHX(t *testing.T) {
	h := newTestServer(t, "").Handler()
	for _, p := range pageRoutes {
		body := get(t, h, p, nil).Body.String()
		if !strings.Contains(body, "<html") {
			t.Fatalf("%s: missing <html", p)
		}
		if !strings.Contains(body, "esc <strong>portal</strong>") {
			t.Fatalf("%s: missing sidebar", p)
		}
	}
}

func TestFleetAndUsageFragmentsRequireHXHeader(t *testing.T) {
	h := newTestServer(t, "").Handler()
	for _, p := range []string{"/fleet?sort=repo", "/usage"} {
		if full := get(t, h, p, nil).Body.String(); !strings.Contains(full, "<html") {
			t.Fatalf("%s without HX must be a full page", p)
		}
		frag := get(t, h, p, map[string]string{"HX-Request": "true"}).Body.String()
		if strings.Contains(frag, "<html") {
			t.Fatalf("%s with HX must be a fragment", p)
		}
	}
}

func TestDiffEndpointFragmentRequiresHXHeader(t *testing.T) {
	s := newTestServerWithPacks(t)
	form := url.Values{
		"frag":    {"rules/security.md"},
		"content": {"---\ntargets: [claude, agents]\n---\n# Security\n\n- Added rule.\n"},
		"version": {"1.3.0"},
		"action":  {"diff"},
	}
	post := func(hx bool) string {
		req := httptest.NewRequest("POST", "/packs/org-baseline/publish", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if hx {
			req.Header.Set("HX-Request", "true")
		}
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, req)
		return rr.Body.String()
	}
	if full := post(false); !strings.Contains(full, "<html") {
		t.Fatal("diff without HX must be a full page")
	}
	if frag := post(true); strings.Contains(frag, "<html") {
		t.Fatal("diff with HX must be a fragment")
	}
}
```

- [ ] **Step 2: Run the test**

Run: `go test ./internal/portal/web/ -run 'Degrad|FullPage|RequireHX' -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/portal/web/degrade_test.go
git commit -m "test(portal): graceful-degradation gate for htmx fragments"
```

---

### Task 6: Accessibility sweep

Chart SVGs get a `<title>` (they already carry `role="img"`); table headers get `scope="col"`; tables get empty states; the skip link is already in place from Task 2. `scope` and empty states for the usage and fleet tables were added in Tasks 4b/4c; this task completes the remaining piece — chart titles — and verifies the whole set.

**Files:**
- Modify: `internal/portal/charts/charts.go` (`Line`, `StackedBars`, `empty` gain a `title` param and emit `<title>`)
- Modify: `internal/portal/web/server.go` (3 chart call sites pass titles)
- Modify: `internal/portal/charts/charts_test.go` (updated call signatures)
- Modify: `internal/portal/charts/testdata/line.golden.svg`, `bars.golden.svg` (regenerated)
- Test: `internal/portal/web/overview_test.go`

**Interfaces:**
- Produces: `func Line(title string, pts []Point, w, h int) template.HTML`; `func StackedBars(title string, labels []string, series []Series, w, h int) template.HTML`. Callers: `handleOverview`, `handleUsage`.

- [ ] **Step 1: Write the failing test**

Add to `internal/portal/web/overview_test.go`:

```go
func TestChartsHaveAccessibleTitles(t *testing.T) {
	dir := t.TempDir()
	epoch := time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
	if err := seed.Demo(dir, epoch); err != nil {
		t.Fatal(err)
	}
	st, _ := store.Open(dir)
	s := New(st, nil, "", "test")
	s.Now = func() time.Time { return epoch }
	h := s.Handler()

	overview := get(t, h, "/", nil).Body.String()
	if !strings.Contains(overview, `role="img"`) || !strings.Contains(overview, "<title>") {
		t.Fatal("overview chart needs role=img and a <title>")
	}
	usage := get(t, h, "/usage", nil).Body.String()
	if strings.Count(usage, "<title>") < 2 {
		t.Fatal("both usage charts need <title>")
	}
}
```

(Add any missing imports: `strings`, `testing`, `time`, `seed`, `store` — mirror `usage_test.go`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/portal/web/ -run TestChartsHaveAccessibleTitles -v`
Expected: FAIL (no `<title>` in chart SVGs).

- [ ] **Step 3: Add the title parameter in charts.go**

In `internal/portal/charts/charts.go`:

Change `empty` (line 70) to accept and emit a title:

```go
func empty(title string, w, h int) template.HTML {
	return template.HTML(fmt.Sprintf(
		`<svg viewBox="0 0 %d %d" width="%d" height="%d" role="img"><title>%s</title><text x="%s" y="%s" text-anchor="middle" %s>No data yet</text></svg>`,
		w, h, w, h, esc(title), f(float64(w)/2), f(float64(h)/2), textAttrs))
}
```

Change `Line`'s signature (line 78) and its opening tag / empty call:

```go
func Line(title string, pts []Point, w, h int) template.HTML {
	if len(pts) == 0 {
		return empty(title, w, h)
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 %d %d" width="%d" height="%d" role="img"><title>%s</title>`, w, h, w, h, esc(title))
```

Change `StackedBars`'s signature (line 124) and its opening tag / empty call:

```go
func StackedBars(title string, labels []string, series []Series, w, h int) template.HTML {
	if len(labels) == 0 || len(series) == 0 {
		return empty(title, w, h)
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 %d %d" width="%d" height="%d" role="img"><title>%s</title>`, w, h, w, h, esc(title))
```

- [ ] **Step 4: Update the server call sites**

In `internal/portal/web/server.go`:
- Line 270: `AdoptionChart: charts.Line("Governed repos over time", pts, 640, 220),`
- Line 670: `TokensChart: charts.StackedBars("Tokens per day by model", labels, tokenChartSeries, 640, 220),`
- Line 671: `CostChart:   charts.StackedBars("Cost per day by team", labels, costChartSeries, 640, 220),`

- [ ] **Step 5: Update charts_test.go call sites**

In `internal/portal/charts/charts_test.go`:
- Line 41: `golden(t, "line.golden.svg", string(Line("Adoption", pts, 480, 160)))`
- Line 50: `golden(t, "bars.golden.svg", string(StackedBars("Usage", labels, series, 480, 200)))`
- Line 54: `if got := string(Line("Adoption", nil, 480, 160)); !contains(got, "No data yet") {`
- Line 57: `if got := string(StackedBars("Usage", nil, nil, 480, 200)); !contains(got, "No data yet") {`

- [ ] **Step 6: Regenerate the golden SVGs**

Run: `go test ./internal/portal/charts/ -run 'Golden' -update`
Then verify: `go test ./internal/portal/charts/ -v`
Expected: PASS. Inspect that `line.golden.svg` and `bars.golden.svg` now contain `<title>Adoption</title>` / `<title>Usage</title>`.

- [ ] **Step 7: Run the full portal suite**

Run: `gofmt -w . && go vet ./... && go test ./internal/portal/... -v`
Expected: PASS (including `TestFleetEmptyState`, `scope="col"` present via Tasks 4b/4c).

- [ ] **Step 8: Commit**

```bash
git add internal/portal/charts/charts.go internal/portal/charts/charts_test.go internal/portal/charts/testdata/line.golden.svg internal/portal/charts/testdata/bars.golden.svg internal/portal/web/server.go internal/portal/web/overview_test.go
git commit -m "feat(portal): a11y sweep — chart titles, scoped headers, empty states"
```

---

### Task 7: Final verification and docs

Runs the whole gate suite, a manual `esc serve --demo` walkthrough (the controller screenshots), and records the user-facing changes plus the vendored-asset policy.

**Files:**
- Modify: `CHANGELOG.md` (Unreleased → Added)
- Modify: `README.md` (one line under the `esc serve` section, ~line 74-76)

- [ ] **Step 1: Full verification gate**

Run:

```bash
gofmt -l . && go vet ./... && go test ./... && go build ./cmd/esc
```

Expected: `gofmt -l .` prints nothing; vet clean; all tests pass; binary builds.

- [ ] **Step 2: Manual demo walkthrough (controller screenshots)**

Run: `go run ./cmd/esc serve --demo` and confirm, in a browser:
1. Overview loads; navigating between pages animates (Chromium/Safari) with no flash.
2. Fleet: click a column header — the table re-sorts without a full reload; `aria-sort` toggles; with JS disabled the same click still sorts via full navigation.
3. Fleet/Usage segmented filter rows render as connected segments; the active segment is highlighted.
4. Usage: change team/model/day filters — only the charts + table region updates.
5. Pack edit: "Preview diff" updates the diff region inline; "Publish" still does a full redirect.
6. Narrow the window below ~700px — the sidebar collapses to a top bar and content stays usable.
7. DevTools: no CSP violations in the console; response headers show the exact CSP and `X-Content-Type-Options: nosniff`; the session cookie shows `SameSite=Strict`.

- [ ] **Step 3: Update the CHANGELOG**

Add under `## [Unreleased]` → `### Added` in `CHANGELOG.md`:

```markdown
- Portal frontend modernization: cross-document view transitions (disabled under
  prefers-reduced-motion), a responsive sidebar that collapses below 700px,
  segmented filter controls, and :focus-visible rings. A strict
  Content-Security-Policy (default-src 'none') and X-Content-Type-Options:
  nosniff now ship on every portal response; the session cookie is SameSite=Strict
  and Secure over TLS.
- Portal partial updates via vendored htmx 2.0.9 (an embedded JS asset pinned by
  SHA-256 in internal/portal/web/HTMX-VENDOR.md, never fetched at runtime, not a
  Go dependency), scoped to three interactions: pack-edit diff preview, usage
  filters, and fleet column sorting. Each degrades to a full-page render when
  JavaScript is off; fragments are returned only for HX-Request requests. htmx
  runs under the strict CSP with eval, history, and injected indicator styles
  disabled, and hx-disable wrapping all pack-authored markdown.
```

- [ ] **Step 4: Update the README**

In `README.md`, under the `### esc serve — the admin portal` section, add one line after the feature description (around line 76):

```markdown
The portal is server-rendered with a strict CSP; its one third-party asset, htmx, is vendored and embedded (pinned by SHA-256 in `internal/portal/web/HTMX-VENDOR.md`), never fetched at runtime, and every interaction still works with JavaScript disabled.
```

- [ ] **Step 5: Re-run the gate and commit**

```bash
gofmt -w . && go vet ./... && go test ./...
git add CHANGELOG.md README.md
git commit -m "docs(portal): changelog and README for view transitions and vendored htmx"
```

---

## Self-Review

**1. Spec coverage** — every spec requirement maps to a task:

- Hardening headers (exact CSP, nosniff, cookie Strict+Secure, applied to all responses, tested per route) → Task 1.
- View transitions + reduced-motion opt-out → Task 2. Responsive sidebar from the fixed-220px layout (media query at 700px, concrete static/top-bar mechanism) → Task 2. Segmented filter controls (fleet + usage) → Task 2. `:focus-visible` rings on nav and card links → Task 2.
- Vendor htmx 2.0.x (pinned 2.0.9, sha256 in committed `HTMX-VENDOR.md`, embedded via existing `go:embed`) → Task 3. `htmx-config.js` with allowEval/includeIndicatorStyles/historyEnabled/selfRequestsOnly → Task 3. `.htmx-indicator` in style.css → Task 2. Script tags in layout → Task 3. `hx-disable` around `mdHTML` output (pack.html, the only `mdHTML` render site) → Task 3.
- Three interactions with full-page fallback: diff preview (4a), usage filters (4b), fleet sorting with NEW server-side stable sort + `aria-sort` buttons (4c). `aria-live="polite"` on all three swapped regions → 4a/4b/4c. Fragments only when `HX-Request` present via `isHX` helper → 4a.
- Degradation test gate iterating routes + three fragment endpoints → Task 5.
- A11y: charts `role="img"` + `<title>` → Task 6; `th scope="col"` → 4b/4c/6; empty states (fleet/usage) → 4c/4b; skip link → Task 2.
- Final verification: gofmt/vet/test/build + demo walkthrough + CHANGELOG + README asset policy → Task 7.

**2. Placeholder scan** — the only value not literally in the plan is the htmx sha256, which is genuinely unknowable until the file is downloaded; Task 3 Step 1-2 compute and record it as a real action, not a deferred TODO. All CSS, template, and Go bodies are complete.

**3. Type consistency** — `isHX`/`renderFragment` defined in 4a, reused verbatim in 4b/4c/5. `colSort{Href,Aria,NextDir}` and `fleetHeader` names match between server.go and fleet.html. `Line`/`StackedBars` new signatures (title first) match across charts.go, server.go call sites, and charts_test.go. Template block names (`diff-region`, `usage-results`, `fleet-table`) are consistent between each `define` and its `renderFragment` call. `.htmx-indicator`, `.segmented`, `.seg`, `.th-sort` class names match between style.css and the templates.

---

Plan complete. It is ready to be saved to `docs/superpowers/plans/2026-08-01-portal-frontend-modernization.md`.

Key source files the executor will touch (all absolute):
- `/Users/billyz/code/openescapement/internal/portal/web/server.go`
- `/Users/billyz/code/openescapement/internal/portal/web/templates/{layout,fleet,usage,pack,pack_edit}.html`
- `/Users/billyz/code/openescapement/internal/portal/web/static/style.css`
- `/Users/billyz/code/openescapement/internal/portal/web/static/htmx.min.js` and `htmx-config.js` (new)
- `/Users/billyz/code/openescapement/internal/portal/web/HTMX-VENDOR.md` (new)
- `/Users/billyz/code/openescapement/internal/portal/charts/charts.go` and its golden testdata
- Tests: `server_test.go`, `fleet_test.go`, `usage_test.go`, `packs_test.go`, `overview_test.go`, and new `degrade_test.go`
- `/Users/billyz/code/openescapement/CHANGELOG.md`, `/Users/billyz/code/openescapement/README.md`
