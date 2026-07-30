# Admin Portal (`esc serve`) — Design

**Status:** Approved design, pre-plan.
**Parent:** `ai-governance-product-spec.md` §3 Pillar B, §6, §7; `docs/roadmap/v0.2-mcp-and-telemetry.md` Part B.
**Date:** 2026-07-30

## Purpose

The first version of the control-plane admin portal. Dual use, one binary:

1. **Pitch artifact.** A portal an exec (COO/CIO/CISO/CTO) who is not close to the technology can understand in minutes: what models are managed, what the rulebooks are and how they're administered, which teams and repos use them, and usage metrics. `esc serve --demo` seeds a realistic fictional org and is screen-shareable immediately.
2. **Product skeleton.** The real v0.2 control plane starts here: same binary, real ingest API matching the roadmap event schema, so the `esc` CLI reporter and later collectors wire in additively.

## Decisions

- **Hybrid, not mock.** Real Go server with a `--demo` seed mode, rejected pure-demo throwaway and real-data-only (too slow to look impressive).
- **Pure Go, one binary.** New `esc serve` subcommand. Stdlib `net/http` + `html/template`, assets via `embed`, charts as server-rendered SVG. **Zero new dependencies** — the single-external-dependency policy holds. Rejected React SPA (npm surface contradicts the security posture and one-binary self-hosting) and vendored JS blobs (unauditable third-party code, worse than a `go.mod` entry because invisible).
- **Single-org for v1.** Multi-tenancy is a SaaS-tier concern.
- **Observe-don't-distribute preserved.** The portal never pushes rules into repos (see Pack admin).

## Architecture

- `esc serve [--demo] [--addr] [--data-dir]` starts portal UI + ingest API on one port.
- **Storage:** append-only JSONL event log plus small JSON registry files (org, departments, teams, repos, pack sources) under `--data-dir` (default `~/.escapement/server`). Rollups computed in memory per request; at v1 fleet sizes this is milliseconds. A small internal store interface so Postgres can replace it for the SaaS without touching handlers.
- **Ingest:** `POST /api/v1/events`, bearer token, event schema per the v0.2 roadmap sketch (`kind: sync | status | update_check | provider_usage | ...`). Real from day one; unknown fields tolerated, invalid events rejected 4xx.
- **Demo seed:** deterministic (fixed seed, no wall-clock randomness at generation; timestamps derived from a fixed epoch passed at seed time). Roughly: 6 departments, 15 teams, 40 repos, mixed adoption (governed/ungoverned), some drift, some stale packs, 60 days of token/cost history across several models and agent tools.

## Pack admin (edit → publish)

The server holds a working clone of a pack-source git repo. Portal "publish" = validate the pack (existing lint/render paths), commit, tag a new version. Distribution is untouched: clients get rules exclusively via `esc sync` pull + signature verification. The portal is a pack editor, not a rule pusher.

Editing UI: markdown textarea, rendered preview, plain diff against the current version. No WYSIWYG; a diff is what a security reviewer trusts.

Demo mode seeds a local pack repo and one governed demo repo on disk so the full pitch loop runs live: edit rule in browser → publish new version → `esc sync` in the demo repo → managed block updates on screen.

## Pages

Every page has a one-line plain-English explainer strip under the title (e.g. "These are the AI rulebooks your teams' coding agents read on every session"). The portal teaches the concept as it shows the data.

1. **`/` Overview.** Five exec-parseable numbers, each linking to its drill-down: governed repos (and % of known), repos with drift, repos on stale rules, token spend this week, models in use. One trend chart: governed repos over time.
2. **`/fleet`.** Departments → teams → repos table: pack + version, last sync, status (in-sync / drifted / stale), agent tools seen. Filterable by status. Team-level identity by default (roadmap privacy stance); user-level only with future org opt-in.
3. **`/packs`.** Each pack: current version, signed badge, last published, consuming repos. Detail page: markdown, version history, edit → publish flow.
4. **`/usage`.** Tokens per model per day stacked by team; estimated cost; filters for team/model/time range. Attribution honesty per roadmap: aggregate numbers labeled as aggregate, never silently attributed to a tool.

## Auth

- `--demo`: open, binds localhost only.
- Otherwise: generated admin session token, login URL printed on start (Jupyter-style). OIDC/passkeys remain in the v0.2 roadmap design; not built here.

## Error handling

- Publish validates before any commit; nothing half-published; atomic-write discipline throughout.
- Ingest never crashes the portal; malformed events are rejected with 4xx and counted.
- Empty data renders honest empty states, not broken charts.

## Testing

- `httptest` handler tests for every route.
- Golden tests for rendered pages and SVG charts (deterministic output is house style).
- Seed-determinism test: same seed → byte-identical demo data.
- Pack publish round-trip against a real temp git repo, matching existing integration-test patterns (`ESC_CACHE_DIR` respected).

## Out of scope (v1)

Multi-tenancy, OIDC/passkeys/SAML, MCP server, provider-API collectors (usage data is seeded or ingested, not fetched), SIEM export, registry self-service/expiry flows, review workflows.
