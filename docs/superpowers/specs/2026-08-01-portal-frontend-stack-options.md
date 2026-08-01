# Portal frontend stack: options for a more modern, dynamic UI

Status: draft for review, 2026-08-01. Decision pending.

Context: the portal is server-rendered Go html/template with zero JavaScript, all assets embedded in the binary, and no external requests. That posture is part of the product: a governance tool whose own supply chain is auditable. The question is how to get a more modern, more dynamic feel without giving that up.

## Constraints any option must satisfy

- No CDN or external request, ever. Assets ship inside the esc binary via go:embed.
- Any third-party code is vendored at a pinned version with its checksum committed, upgraded only by deliberate diff-reviewed bumps.
- Strict Content-Security-Policy must be addable: no eval, no inline script, script-src 'self' at most.
- The server stays the source of truth: HTML rendered by Go templates, no JSON API surface introduced for the UI's sake.
- Degrades gracefully: every page remains fully functional with JS disabled.

## Option A: modern CSS only (zero new supply chain)

Use current platform CSS, no JS at all:
- Cross-document View Transitions: animated navigation between pages (supported Chromium and Safari; silent no-op elsewhere). Makes an MPA feel like an SPA for free.
- `:has()`, container queries, `popover`, `<details>`, scroll-driven animations for disclosure, menus, and micro-interaction.
- Cost: CSS only. Keeps the zero-JS story fully intact.
- Limit: no partial updates; filters and diff preview remain full-page reloads (the server renders in single-digit milliseconds, so reloads are fast, but the reload flash is visible).

## Option B: Option A plus vendored htmx (~14 KB gzipped, zero dependencies)

htmx adds attribute-driven partial page updates while the server keeps rendering HTML fragments:
- Usage filters swap the chart region in place; publish diff preview updates inline; fleet drift status can poll.
- Security profile: one auditable file, no dependencies of its own, no eval in its core; runs under CSP `script-src 'self'`. Config hardening: `htmx.config.allowEval = false`, `htmx.config.selfRequestsOnly = true`, inline `hx-on` attributes unused.
- Vendored and embedded, never fetched. Version pinned, checksum committed.
- Risk: third-party code inside a security product. Mitigations above, plus the graceful-degradation rule keeps htmx an enhancement, not a dependency.

## Option C: Alpine.js or similar client-state library. Rejected.

Alpine evaluates expressions from DOM attributes, which is eval-equivalent; its CSP build removes most of its value. The portal has almost no client-side state to manage.

## Option D: Tailwind or a CSS framework. Rejected.

Changes authoring ergonomics, not user-perceived quality; the portal already has a coherent token system; adds a Node build step to a pure `go build` project.

## Recommendation (revised after review, 2026-08-01)

Adopt A everywhere, and ship B in the same release, scoped to the three interactions the UX review ranked highest: pack-edit diff preview, usage filters, and fleet column sorting. Drift polling waits. The security review approved A outright and approved B conditional on the mitigations below; live verification found no direct CVEs against htmx 2.x core (pin the current 2.0.x stable; do not adopt the 4.0 beta).

### Required mitigations for B (from security review)

- `hx-disable` wraps ALL server-rendered pack or markdown content, so htmx never processes pack-author-controlled HTML. This is the load-bearing mitigation: `mdHTML` output is `template.HTML` and bypasses auto-escaping.
- CSP gains `connect-src 'self'` (htmx XHR is otherwise blocked; connect-src does not fall back usefully from `default-src 'none'`) and `base-uri 'none'`.
- `htmx.config.includeIndicatorStyles = false` (its default injects an inline style element that strict CSP blocks); ship `.htmx-indicator` rules in style.css.
- `htmx.config.historyEnabled = false`: htmx's back-button cache snapshots pages into localStorage; usage/cost pages are sensitive.
- `allowEval = false`, no `hx-on` attributes anywhere, `selfRequestsOnly` left at its default (true in 2.x).
- Vendored at a pinned version with committed checksum, embedded via go:embed, never fetched; every version bump gets a source diff review.
- Cookie gains `Secure` when served over TLS; SameSite upgraded Lax to Strict (load-bearing CSRF hardening for POST /publish, which has no CSRF token).
- Degradation is a test gate, not a rule: every route renders a full page; fragments are returned only when the `HX-Request` header is present; an integration test asserts every route's no-HX-Request response is a complete page.

### UX review requirements folded in

- Day-one htmx scope: diff preview and usage filters (the two worst reload offenders and the core authoring loop), plus fleet column sorting (currently absent, the one table interaction users expect).
- Pure-CSS work that outranks any JS for perceived modernity: responsive layout (the fixed sidebar currently breaks below ~700px; collapse it via container queries), filter rows restyled as segmented controls, `:focus-visible` rings on nav and card links (existing a11y bug).
- A11y clauses binding the eventual spec: `prefers-reduced-motion: reduce` disables view transitions; every htmx-swapped region has `aria-live="polite"` and explicit focus management; sortable headers are real buttons with `aria-sort`; charts get `role="img"` and titles; tables get scoped headers and empty states.

### Hardening headers (ship regardless of A or B)

`Content-Security-Policy: default-src 'none'; style-src 'self'; script-src 'self'; img-src 'self'; connect-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'`, `X-Content-Type-Options: nosniff`, cookie `Secure` + `SameSite=Strict`.
