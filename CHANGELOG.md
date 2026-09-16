# Changelog

All notable changes to OpenEscapement are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims
to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html) once
tagged releases begin.

## [Unreleased]

### Added
- `examples/packs/openai-models` (GPT-6 Astra preferred for planning and review;
  the GPT-5.6 tiers for day-to-day and bulk work; GPT-5.2 banned as retired) and
  `examples/packs/zai-models` (GLM 5.3 Flash via OpenRouter as the near-free seat,
  with its retired stealth preview id banned by name). Every catalog status carries
  its reason and, where it departs from the vendor's default, its reversal condition.
- README: "Independently, or with Balancewheel" states the boundary with the
  per-user layer (esc never writes agent configuration; its home-directory state is
  the pack cache and `esc serve` data) and the three install paths.

### Changed
- `examples/packs/anthropic-models` 0.2.0: Fable 5.1 preferred; Opus 4.8 the
  Opus-tier choice; Sonnet 5 at its now-permanent $2/$10; Opus 5 moved to
  review-required with the reason and a measurable reversal condition; Fable 5 to
  review-required as superseded.
- **BREAKING for CI gating on `esc sync`.** Exit 0 from `esc sync` no longer
  asserts that the repo matches policy. It now asserts only that everything
  escapement was willing to apply was applied: an artifact whose managed
  region was hand-edited, and a retired skill directory still holding files
  the team added or a pack-provided file the team edited, are all reported
  and skipped, and sync still exits 0. Any
  pipeline that treated `esc sync` exit 0 as a compliance check must move that
  gate to `esc status --check`, which exits 1 on any artifact not in sync.
  Skipped artifacts are listed on stderr and in the `skipped` array of
  `esc sync --json`.
- `esc sync --json` and `esc status --json` name an artifact with `subject` in
  both `findings[]` and `skipped[]`. `skipped[].path` was renamed to
  `skipped[].subject` for that consistency.
- `skipped[]` entries carry a machine-readable `cause` alongside the prose
  `reason`: `hand-edited`, `orphan-dir-unmanaged`, `orphan-dir-edited`, or
  `orphan-block-edited`.
- `esc sync` prints a remedy line under each declined artifact instead of one
  blanket line covering all of them. The blanket line pointed at `esc diff`,
  which only ever covers a hand-edited managed region, so a declined skill
  directory or an undeleted orphaned block sent the reader to a command that
  shows nothing about it.
- **BREAKING for CI gating on `esc status --check`.** `esc status` reports a
  retired esc skill directory as an `orphan` finding, the way it already did
  for an orphaned managed block. A retirement that `esc sync` declines is
  therefore visible to `esc status --check` instead of passing it at exit 0.
  Repos previously green on `--check` while parked in a declined retirement
  now exit 1 there.
- A symlink standing where escapement is about to write now exits 4, not 1.
  Exit 1 is the drift-and-constraint class a CI gate reads as routine and
  self-healing; a containment refusal is neither, and the other containment
  refusals already exit 4.
- Guidance seeding now refreshes unedited files to the latest embedded content
  instead of create-if-missing only, tracked by a
  `<data-dir>/guidance/.seeded.json` hash manifest; hand-edited and
  portal-edited files are still never overwritten, and pre-manifest dirs are
  migrated by recording only files that still match the shipped content.
- `esc serve --demo` now resets its demo-owned data (org store, pack repos,
  demo repo, guidance) to pristine on every startup and prints
  `esc: demo data reset`. Non-demo servers are unaffected.
- The managed-block notice line reads "Managed by escapement. Do not edit."
  (previously an em-dash), and the catalog notes separator is now a spaced
  hyphen, `name (category) - notes` (previously an em-dash there too, and
  the awesome-list convention this catalog line follows machine-enforces
  the hyphen form). Cosmetic for humans; a hash change for tooling: the
  next `esc sync` rewrites the block, and until then `esc status` reports
  `stale`, which is routine drift.
- `esc init` confirms each marker write ("CLAUDE.md: wrote <!-- escapement:block -->")
  and says so when a position answer is not recognized, instead of writing
  or declining silently.

### Added
- `esc init` detects instruction files a repo already has, pre-fills `targets` from them, explains what the first sync will do, and offers once, on a TTY, to place the managed-block marker. The offer defaults to no; one answer covers every detected file. It never writes rendered policy, never writes over a file with uncommitted changes, and outside a git repository says there is no undo before asking. `esc init` now rejects positional arguments instead of silently scaffolding the current directory.
- The seeded demo pack ships an `esc-reconcile` skill: guidance for an agent comparing a team's existing rules against the pack's.
- Local amendments: content escapement does not own is preserved everywhere, reported on a new `local` axis, and surfaced in `esc status`. Files added to skill directories are no longer deleted by sync.
- `esc sync` skips artifacts whose managed region was hand-edited instead of overwriting them, warns, and exits 0. `esc sync --force` converges.
- New managed blocks are inserted at the top of a file rather than appended.
- `esc status --json` and `esc sync --json` emit a machine-readable report.
- `reporting.amendments` in a pack manifest and `report_amendments` in repo
  config resolve the level at which a repo reports local amendments
  upstream; see the telemetry publisher bullets below for the transport
  that now sends them.
- **Telemetry publisher.** `esc sync` and `esc status` publish the resolved,
  redacted `esc status --json` document to every distinct `reporting.endpoint`
  a pack declares, as a versioned envelope (schema 1) over HTTPS,
  bearer-authenticated with `ESC_PORTAL_TOKEN` from the environment.
  Publishing runs after the lockfile is written and all normal output is
  printed, and is strictly non-fatal: no publish failure ever changes a
  command's exit code or suppresses its output. Below `content`, `Redact`
  strips amendment content, alteration diffs, and free-text finding/skip
  detail before anything leaves the repo; counts, hashes, states, item
  names, and pack pins still ship at `metrics`. Unsent envelopes queue to
  the gitignored `.escapement/outbox.jsonl` (200-entry / 14-day cap, oldest
  dropped first, every drop reported on stderr) and flush oldest-first on
  the next successful publish; a permanently rejected payload (400/404/405/
  413/415/422) is dropped rather than retried so it cannot stall the queue
  behind it.
- The portal's `POST /api/v1/events` now also accepts a versioned
  `publisher.Envelope` alongside the legacy raw-event body, derives the
  event's drift state from its artifacts, and matches it against the
  registry by normalized remote (an unmatched remote lands in the
  unregistered bucket instead of being dropped). Ingest body cap raised
  from 64KB to 1MB to fit content-level payloads. `esc serve --token
  <token>` sets the server's bearer token explicitly (default: a random
  token printed on start; `--demo` still disables auth).
- The fleet table gains a state column (unadulterated / augmented / altered
  / ungoverned); repo detail lists each artifact's managed/local axes,
  amendment size, and, at `content`, the amendment text and alteration
  diff, naming the source when content is withheld; an unregistered bucket
  lists repos reporting from unrecognized remotes with a register
  affordance.
- The seeded demo now covers all four fleet states, including a repo-level
  override that withholds content, so every new fleet view has real data to
  render against.
- Vendor starter sets: each full-depth vendor page grows a starter-set panel
  that adopts any selection of model starters plus the model-routing overview
  as one composed fragment (`rules/models-<vendor>.md`). Single-model adoption
  and its `rules/model-<id>.md` naming are unchanged. Starter and example
  fragments no longer carry absolute prices — relative cost guidance stays in
  the fragments; dollar figures live in the vendor guidance pages.
- Model starters and adopt flow: every current full-depth-vendor model
  (Anthropic, OpenAI, Google) ships a starter rule-pack fragment encoding its
  routing position, review-gate, and cost guidance, seeded and portal-editable
  like the other guidance files. Each vendor page renders the starter with an
  Add to rule pack button; the adopt form writes the starter into a chosen
  writable pack through the publish pipeline (validate, add to the manifest,
  bump version, commit, tag) and never overwrites an existing fragment.
  Portal markdown is now rendered by goldmark (display-only, raw HTML
  disabled), turning Markdown links and bare http/https URLs into anchors.
- Models guidance: a new portal **Models** section with curated, sourced,
  editable guidance per vendor (Anthropic, OpenAI, Google, Kimi, Deepseek,
  Grok). A strict `models.yaml` registry (fixed vendor order), per-vendor
  markdown notes, and example rule-pack fragments ship embedded and are seeded
  to `<data-dir>/guidance/` create-if-missing. The portal reads from disk on
  every request with per-file fallback to the embedded copy and a fail-soft
  banner. Overview chips and usage-table model names link to their vendor page
  anchors. Editor writes are atomic and restricted to the fixed guidance file
  set (`models.yaml` is repo-managed, not portal-editable).
- Custom targets: a pack's `pack.yaml` may define managed-block markdown targets
  under `custom_targets` (`name`, `file`, optional `doc`/`description`), usable in
  fragment `targets:`. Each target is owned by its defining pack (no cross-pack
  co-writing; duplicate name or file across packs fails), renders only the owning
  pack's explicitly-targeting fragments with no catalog section, and lands only
  after the repo acknowledges its file in `allow_custom_target_files` (fail-closed,
  reported by `esc status`). Path rules reject traversal, control directories,
  non-ASCII, and built-in collisions; writes refuse symlinked parents; targets
  that leave the effective set have their managed block removed on the next sync.
- `esc status` (without `--check`) now exits 1 when a fail-closed constraint
  violation exists, for example an unacknowledged custom target file. Ordinary
  drift and orphaned managed blocks still exit 0 without `--check` and 1 with it;
  orphans are self-healing, since the next sync removes the stale block.
- Release automation: goreleaser builds for linux/darwin/windows, cosign keyless
  signing, SLSA v1 build-level-3 provenance, CycloneDX SBOMs, Homebrew cask,
  verified `install.sh`, and `VERIFYING.md`.
- `esc version` now prints commit and build date alongside the version.
- CI workflow: test matrix (linux + macos), race detector on engine/cli,
  gofmt/vet gates, goreleaser snapshot check.
- Update-check (client): packs may declare `update_check: { every: <7d|24h|90m>, endpoint: <https URL> }`.
  Overdue `esc status`/`diff`/`update`/`render` invocations run a lightweight
  `git ls-remote` staleness probe (10s timeout, never blocks the command), record it to
  the gitignored per-clone `.escapement/update-log.jsonl` (last 50 entries), and — on an
  interactive TTY — offer to re-pin and sync; `esc sync` records a check entry itself
  rather than probing. `esc status --check` reports `pack-stale`
  and `check-overdue` findings (exit 1). Inert unless a pack opts in. The `endpoint` field
  is parsed for forward-compatibility but not contacted in v0.1.
- `AGENTS.md`: canonical agent-facing repo instructions (CLAUDE.md now imports it),
  restructured per Anthropic's Claude 5 context-engineering guidance; vendor-guidance
  tracking practice documented in `docs/roadmap/vendor-guidance-tracking.md`.
- **`esc serve`** — the admin portal: a one-binary control plane with overview, fleet,
  rule-pack, and usage pages, a bearer-token events ingest API, and portal-side pack
  publishing (validate → commit → tag; distribution still happens only via `esc sync`).
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

### Fixed
- CRLF instruction files: leading YAML frontmatter with CRLF line endings is
  now recognized, so sync's top placement and the `esc init` marker offer
  insert below the fence instead of above it (which silently demoted the
  frontmatter to a setext heading), and a single blank line between
  frontmatter and a leading H1 no longer stops the block from landing below
  the title. Escapement still renders LF; line endings are now treated as
  presentation, not policy content, on the whole managed axis: a managed
  block or a whole-file target (`GOVERNANCE.md`) converted to CRLF on disk
  (e.g. by `core.autocrlf`) classifies in sync. A hand edit that only
  changes line endings is therefore no longer reported; a genuine content
  edit arriving under CRLF endings still is.
- A whitespace-only `.mcp.json` is treated as empty instead of failing the
  sync with a JSON parse error.
- `esc status` no longer follows a hostile lockfile path out of the repo (or
  through a symlink) when reporting an orphaned managed block, matching the
  containment rules the skill-directory pass already enforced.
- `esc status` no longer advises `esc sync --force` for a lockfile directory
  entry that `esc sync` refuses to touch as not escapement-owned; it now
  reports what sync will actually do. The entry is reported even when the
  directory itself is already gone, because sync refuses on ownership before
  it ever looks at the disk.

Planned — see `docs/roadmap/`:
- v0.2: MCP server surface (live policy queries, connection telemetry, agent-initiated registration)
- Dashboards & metrics: tool/LLM usage, token consumption per model, pack customization vs. defaults, sync freshness

## [0.1.0] — 2026-07-07

First release. A Go CLI, `esc`, that syncs signed, version-pinned policy
**rule packs** from git repos into the instruction files AI agents read.

### Added
- **Rule-pack format** — YAML manifest (`pack.yaml`) + markdown rule fragments, with a structured tool/service catalog (preferred/allowed/review-required/banned) and merged-file constraints.
- **Renderer** — six targets: managed blocks in `CLAUDE.md`/`AGENTS.md`/`GEMINI.md`, whole-file `GOVERNANCE.md`, `.claude/skills/` directories, and merged `.mcp.json` entries. Deterministic output, golden-file tested.
- **CLI** — `init`, `sync`, `status [--check]`, `diff [--against REF]`, `update`, `render --stdout`, `version`.
- **Git-native distribution** — packs sourced from git repos at pinned refs; system `git` used so existing auth applies. Hash-pinned lockfile (`.escapement/escapement.lock`).
- **Drift detection** — classifies each artifact as in-sync / modified / missing / stale / constraint-violated; `--check` exits non-zero for CI.
- **GitHub Action** — composite action running `esc status --check`.
- **Example pack + governed repo** under `examples/`.

### Security
- **Signature verification** (SSH `allowed_signers`) before pack content is used; unsigned sources require explicit `trust: unsigned`.
- **Fail-closed integrity** — moved tags, tampered fetches, and hand-edited artifacts fail loudly before any write.
- **Path containment** — pack-controlled names and file entries cannot escape the pack directory or governed repo; symlinks in packs rejected.
- **Argument-injection hardening** — git refs/URLs charset-validated and passed after `--`.
- **Constraint gate** covers all rendered content, including skill files and `.mcp.json` entries.
- Adversarial "hostile pack" test fixtures; `SECURITY.md` with threat model and known limitations.

### Documentation
- `README.md`, product spec, design doc, and implementation plan.
- Apache-2.0 `LICENSE` and open-core split rationale.
- GTM, monetization, onboarding, and scaling strategy.

[Unreleased]: https://github.com/tensorgroup/openescapement/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/tensorgroup/openescapement/releases/tag/v0.1.0
