# Models guidance addendum: clickable links, per-model starters, adopt flow

Status: approved design, 2026-08-02. Extends `2026-08-02-models-guidance-design.md`; that spec's constraints (embedded plus disk-backed content, fail-soft, fixed file set, no em-dashes in authored text) carry over unchanged.

## 1. Clickable links in rendered markdown

`inlineMd` (internal/portal/web/markdown.go) gains link support:

- Markdown links: `[text](url)` renders as an anchor.
- Autolinking: bare `http://` and `https://` URLs in text render as anchors whose visible text is the URL.
- Safety: hrefs are restricted to the http and https schemes; anything else (including `javascript:`) renders as plain text. Anchors carry `rel="noreferrer"`. Escaping order is preserved: input is HTML-escaped before link markup is applied, and the tests cover URLs containing ampersands plus a hostile-scheme case.
- Applies to all mdHTML consumers: guidance notes (Sources sections become clickable), example fragments, and pack fragment rendering.

## 2. Per-model starter rule packs

- `models.yaml` models gain an optional `starter` field: a rel path like `examples/<vendor>/starter-<model-id>.md`. `ParseRegistry` validates that a set `starter` names a file present in the embedded tree.
- Starter files are self-contained rule-pack fragments (targets front-matter plus markdown) encoding that model's governance: routing position, review-gate requirements, cost cautions. Content derives from the already-verified vendor notes; each starter ends with a one-line pointer to its vendor page for sources.
- v1 coverage: every `status: current` model of the full-depth vendors (Anthropic, OpenAI, Google). Legacy models and sparse vendors get no starter in v1.
- Starters are part of the guidance tree: seeded to disk, hand-editable, portal-editable via the existing editor (they are inside `examples/`, already in the editable file set).

## 3. Adopt flow: add a starter to a rule pack

- On `/models/{vendor}`, each model entry with a starter renders the starter (rendered plus raw copyable, same as examples) with an **Add to rule pack** button.
- The button opens `GET /models/{vendor}/adopt?model=<id>`: a plain full-page form showing a pack selector (writable packs from the publish manager), the destination fragment path prefilled and read-only (`rules/model-<id>.md`), the suggested next version, and the starter content preview.
- `POST /models/{vendor}/adopt` runs the existing publish pipeline (validate, commit, tag) via the publish manager, writing the starter content to the chosen pack at the destination path, then redirects to the pack detail page with the existing published banner.
- Collision: if the destination fragment already exists in the chosen pack, respond 422 with an error directing the user to edit that fragment in the pack instead. No overwrite in v1.
- No packs configured (publish manager nil or no writable packs): the button renders disabled with a hint, and the adopt routes 404.
- Model or starter unknown: 404. The starter content is read through the guidance Set (disk first, embedded fallback), so a hand-edited starter is what gets adopted.
- No portal-only state. The pack repo is the record; rules reach repos only via `esc sync`, unchanged.

## Error handling

Existing sentinels and portal patterns: publish validation failures render 422 with the error and preserved form state; unknown vendor, model, or starter 404; the guidance Degraded banner logic is unchanged.

## Testing

- inlineMd table tests: markdown link, bare-URL autolink, ampersand URL, hostile scheme rejected, code and bold unaffected, link inside a list item.
- Registry: starter validation (set but missing file rejects; unset fine); checklist extension: every current full-depth-vendor model has a starter.
- Portal: vendor page renders starter blocks and the adopt button only where a starter exists; adopt GET form renders with pack selector and preview; adopt POST publishes and redirects (against the packs fixture); collision 422; no-packs 404; degradation gate covers the adopt GET.
- Content: each starter parses as a valid fragment against the pack loader.

## Sequencing

One implementation plan, executed before the Targets page cycle: renderer links; registry starter field plus checklist; starter content authoring; portal starter display plus adopt flow; docs (CHANGELOG bullet); final verification.
