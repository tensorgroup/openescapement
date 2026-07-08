# OpenEscapement

Deterministic governance for AI usage — policy-as-artifacts for agentic development, plus a unified usage/SDLC dashboard and self-service registry for federated organizations.

## Current State

v0.1 of the `esc` CLI is implemented (see design doc + plan under `docs/superpowers/`). The registry, dashboard, MCP server, and telemetry pillars are future work.

## Stack & Commands

- Go 1.24, module `github.com/tensorgroup/openescapement`, binary `esc` (`cmd/esc`)
- **Single external dependency policy:** `gopkg.in/yaml.v3` only — everything else stdlib; system `git` via `os/exec`. This is a security posture, not a preference; adding a dep needs explicit justification.
- Test: `go test ./...` (integration tests build real temp git repos — no mocks). Lint: `go vet ./...`; format: `gofmt -w .`
- Layout: `internal/pack` (manifest/fragments), `internal/source` (git fetch + signature verify), `internal/config`, `internal/lockfile`, `internal/render` (compose, managed blocks, governance, mcp merge, constraints), `internal/engine` (plan/apply/status/diff), `internal/cli`
- Errors: sentinel errors in `internal/esc` map to exit codes — 0 ok, 1 drift/constraint, 2 usage, 3 integrity/signature, 4 other
- Invariants to preserve: bytes outside a managed block are never modified; nothing is written after a verification or constraint failure; all writes atomic; renderer output deterministic (golden-testable)
- `ESC_CACHE_DIR` overrides the pack cache (tests rely on this)

## Key Documents

- `ai-governance-product-spec.md` — canonical product spec (Draft v2). Read this before any product or design discussion. Decided items and open questions are marked inline (§11 lists open questions).
- `docs/superpowers/specs/` — design docs produced from brainstorming sessions (spec → plan → implementation cycle).

## Core Concepts (from the spec)

- **Rule packs:** versioned policy source rendered into the instruction files agents natively read (CLAUDE.md, GEMINI.md, CURSOR.md, AGENTS.md, skills/agents dirs, MCP config). Deterministic — literal text the agent reads every session.
- **Paved-path pointers:** rule packs that teach agents about existing central services (SSO, vaults, gateways) so shadow-IT projects get steered onto the paved road.
- **Self-service registry:** 90-second project registration with expiry (keep/kill/archive), giving leadership the shadow-IT map.
- **MCP server as product surface:** live policy queries + telemetry as a side effect of agent connections; static artifacts remain for determinism.

## Product Boundaries (what this is NOT)

- Not enterprise AI governance software (Credo AI, Holistic AI altitude)
- Not an AI firewall/proxy — gateways are integration partners
- Not a code scanner — we verify gates exist, we don't run scans
- Not a model evaluation platform

## Working Conventions

- Spec-driven: substantive product/architecture decisions get captured back into the spec or a design doc, not left in conversation.
- Open-core mindset: rule-pack format, renderer, CLI, and base MCP server are intended to be open source — design them to stand alone without the SaaS control plane.
- Design values: lightweight over complete, privacy-respecting by default (no prompt-content collection), self-hostable, boring and fast.
- YAGNI ruthlessly — the spec repeatedly warns against over-engineering (e.g., the rule-pack format may just be markdown fragments with composition metadata).
