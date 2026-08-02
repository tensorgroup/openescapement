# Models guidance: vendor and model pages with governance advice

Status: approved design, 2026-08-02

Adds a top-level Models section to the portal: curated, sourced guidance on each vendor's models, what they excel at, how to govern them, and example rule-pack fragments to adopt. The content is authored as a guidance pack so it can later be published and forked like any pack.

## Goals

- The tool provides guidance, not just enforcement: which models suit planning, plan-checking, and review versus coding versus cheap bulk work, and how to govern each class.
- Content is browsable by vendor (Anthropic, OpenAI, Google, Kimi, Deepseek, Grok), with models and versions under each.
- Every guidance claim carries a source: vendor docs and blogs, plus this repo's vendor-tracking notes (for example the Claude 5 context-engineering entry, claude.com/blog 2026-07-24).
- Guidance files are plain markdown, hand-editable in any editor and editable from the portal, displayed rendered.
- The content format is pack-shaped from day one. Publishing it as a standalone guidance pack later is a hosting decision, not a rewrite. Others may then publish or fork their own guidance packs.

## Non-goals (v1)

- Publishing the guidance pack to a public repo (format-ready, deferred until the registry pillar).
- Org-side guidance editing workflows beyond direct file edits (forking the future pack covers that).
- Per-model telemetry-driven recommendations, automatic staleness detection, model benchmarking. The product-spec boundary stands: curated, sourced guidance is paved-path pointing, not a model evaluation platform. The product spec gains one line making that distinction explicit.

## 1. Content: the guidance tree

New `guidance/models/` tree in this repo, embedded via go:embed:

- `models.yaml` - the registry. Vendors in fixed display order; under each, models with: `id` (matches usage telemetry ids like `claude-opus-5`), `name`, `tier` (`frontier` | `mid` | `fast`), `roles` (subset of: `planning`, `plan-check`, `review`, `coding`, `bulk`), `status` (`current` | `legacy`), `docs` (list of URLs), `verified` (date the entry was last checked, maintained on the vendor-tracking cadence).
- One markdown note per vendor (`anthropic.md`, `openai.md`, `google.md`, `kimi.md`, `deepseek.md`, `grok.md`): prose guidance covering what each model excels at, role recommendations, governance advice (review gates for high-autonomy frontier models, cost controls for expensive ones, routing policy), and sourced links.
- `examples/<vendor>/*.md` - example rule-pack fragments in the real fragment format (front-matter plus markdown), copyable into any org pack. At minimum one model-routing fragment per full-depth vendor encoding roles (plan and review on frontier tier, code on mid tier, bulk on fast tier).

Launch depth is tiered: Anthropic, OpenAI, Google get full guidance and example fragments; Kimi, Deepseek, Grok get complete model lists, doc links, and a short guidance note each. All six appear in the UI.

Accuracy rule: model lineups and claims are live-verified against primary sources during implementation, not written from memory. Each vendor note ends with a Sources list.

## 2. Disk-backed, editable content

`esc serve` seeds `guidance/` into `<data-dir>/guidance/` on first run (create-if-missing, same pattern and caveat as the demo repo seed: delete the directory to re-seed after an upgrade). The portal reads from disk on every request, so:

- Hand edits in any editor appear on the next page load.
- The portal's Edit buttons write to the same files.
- The embedded copy is only the seed; when guidance becomes a real pack repo later, `<data-dir>/guidance/` becomes a checkout of it.

Portal edits are atomic file writes (temp plus rename). No git versioning of the guidance dir in v1; history arrives when it becomes a pack. `models.yaml` is not editable from the portal in v1 (structure changes are repo work); the markdown notes and example fragments are.

## 3. Portal pages

New top-level sidebar entry **Models** between Rule Packs and Usage (one entry; vendor nesting happens on the page).

- `/models`: vendor sections in fixed order, each a heading with its models as cards: name, tier and role chips, status, verified date, link to the vendor page. Sparse vendors render the same structure with fewer models.
- `/models/{vendor}`: the guidance page. Rendered markdown of the vendor note (via the existing mdHTML path, wrapped in hx-disable like all rendered markdown), an Edit button per note opening the same textarea editor flow the pack fragment editor uses (Preview is not needed for v1; Save writes the file and redirects back to the rendered view), the model registry entries for that vendor, and the example fragments each shown rendered with a copyable raw view and its own Edit button.
- Model chips on the overview page and model names in the usage table link to `/models/{vendor}#<model-id>` anchors.
- Doc links show their host, per the portal convention. Every page shows the verified dates.

Editing follows the existing security posture: rendered guidance markdown is pack-author-class content and stays inside hx-disable; the editor is a plain form POST (full-page flow, no htmx needed in v1); CSP and atomic-write rules apply.

## 4. Layout and navigation

Vendor pages reuse the existing shell, tokens, and card styles. The sidebar gains one item; no sub-navigation in the sidebar. Vendor order is fixed (Anthropic, OpenAI, Google, Kimi, Deepseek, Grok) to keep output deterministic and the ICP-relevant vendors first.

## 5. Error handling

- Missing or unparseable `models.yaml` on disk: the portal falls back to the embedded copy for rendering and shows a banner naming the broken file (fail-soft for a content directory a human hand-edits; sync-style fail-closed is wrong here because nothing downstream executes this content).
- A vendor note file missing on disk: rendered from the embedded copy with the same banner.
- Editor saves validate nothing beyond non-empty path safety (the file set is fixed by the registry; the portal never creates new paths from user input).

## 6. Testing

- Registry parsing: table-driven tests for `models.yaml` (unknown fields rejected, role and tier enums validated, verified dates parsed).
- Seeding: create-if-missing behavior, existing directory untouched.
- Pages: `/models` renders all six vendors and every registry model; vendor pages render notes, examples, anchors; edit round-trip writes the file and the next GET shows the change; fallback banner appears when a disk file is removed or corrupted.
- Degradation gate extends to the new GET routes (full pages, no fragments in v1).
- Content acceptance: a checklist test that every registry model id appearing in seeded usage telemetry has a registry entry, and every vendor note ends with a Sources section.

## 7. Phasing

One implementation plan, ordered: content format and registry parsing; seeding and disk-backed loading; portal pages and editing; content authoring per vendor (research tasks with live source verification, tiered depth); product-spec line and docs. Content authoring tasks are independent per vendor and can be reviewed as prose, not just code.

## 8. Review notes

Design discussion resolved: content home is pack-shaped in-repo now, publishable later (user); tiered launch depth (user); disk-backed seeding to satisfy hand-editing plus portal Edit buttons on rendered markdown (user requirement reconciled with the embedded-asset posture).
