# Changelog

All notable changes to OpenEscapement are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims
to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html) once
tagged releases begin.

## [Unreleased]

Planned — see `docs/roadmap/`:
- Release automation: signed binaries via goreleaser + cosign, SLSA provenance
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
