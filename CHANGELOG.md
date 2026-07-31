# Changelog

All notable changes to OpenEscapement are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims
to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html) once
tagged releases begin.

## [Unreleased]

### Added
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
