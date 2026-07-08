# OpenEscapement

Deterministic governance for AI usage — policy-as-artifacts for agentic development, plus a unified usage/SDLC dashboard and self-service registry for federated organizations.

## Current State

**Pre-code.** The repo currently holds the product spec; we are actively speccing what to build. No stack, build system, or code structure exists yet — do not assume one.

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
