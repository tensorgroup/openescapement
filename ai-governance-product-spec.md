# Deterministic Governance for Your AI Usage — Product Specification

**Author:** Billy Zajac | Tensor Group
**Status:** Draft v2 — pre-brainstorm baseline (extend in Claude Code)
**Last updated:** July 2026
**Related:** `caltech-ai-governance-role-spec.md` (first design-partner candidate)
**Working tagline:** *Deterministic governance for your AI usage.*

---

## 1. Thesis

Every organization is adopting AI agents and assistants faster than it can govern them. The existing answers are either policy documents nobody reads or heavyweight enterprise compliance suites that fight how developers actually work.

**This product deliberately does not compete with "AI governance software."** That category (Credo AI, Holistic AI, watsonx.governance) is top-down, compliance-document-oriented, and sold through six-month procurement cycles. This product is the opposite on every axis: lightweight, self-service, developer-native, visual, and adoptable by a single team in an afternoon — then grown bottom-up across an organization.

The winning pattern — proven at NYSE (script consolidation), MP3.com (config management), and Disney (API governance at trillions of requests, <2ms, tiny team) — is the same every time:

> **Make compliance easier than non-compliance. Ship the rules where the work happens. Give leadership one dashboard.**

The AI-era twist: for the first time, the "workers" being governed (coding agents, orchestration agents) *natively read instruction files*. CLAUDE.md, GEMINI.md, CURSOR.md, AGENTS.md, `~/.claude/skills`, `~/.claude/agents`, MCP configs — these are policy distribution channels that enforce themselves. No prior generation of governance tooling had this. That's the wedge.

**Primary focus (decided):** policy-as-artifacts for agentic development + unified usage/SDLC dashboard. Everything else is secondary or later.

## 2. The Environment This Is Built For

The design target is a complex, federated organization — Caltech is the archetype:

- Dozens of departments/divisions, each with its own projects and teams
- A central IT organization *plus* substantial shadow IT
- A full spectrum of AI adoption in the same building: heavy agentic developers, casual vibe-coders shipping POCs, and developers not using AI at all
- Central services that already exist — authentication, authorization, API key issuance, secrets management, API gateways, docs — that many teams **don't even know about**, so they roll their own
- Lots of experimental/POC work with unclear ownership and unknown lifespans

The product must be genuinely useful to a single team on day one (no org-wide mandate required) and become more valuable as more teams join — bottom-up adoption with top-down visibility as the payoff.

## 3. Product Pillars

### Pillar A — Policy-as-Artifacts (the wedge)
Versioned, centrally-managed rule packs distributed as the files agents already consume.

- **Tool-tailored artifact generation:** one policy source renders to whatever the team actually uses — CLAUDE.md, GEMINI.md, CURSOR.md, AGENTS.md, `~/.claude/skills/*`, `~/.claude/agents/*`, MCP config. Teams pick their tools; policy follows them.
- **Policy domains** (initial set):
  - **Build rules:** what agents may and may not build — authn/authz patterns (approved libraries/flows only), key and secret handling (never inline, approved vaults only), approved cloud platforms and services, required visibility/observability integration.
  - **SDLC rules:** repository tracking requirements, CI/CD integration, testing requirements including security testing (SAST/DAST/dependency scanning gates).
  - **Agent behavior rules:** orchestration turn limits per goal (runaway-token protection), model routing policy (approved models per subagent role — fast models for coding subagents, reasoning models for design/review), escalation rules (e.g., *any newly opened port requires review by [designated role]*).
  - **Paved-path pointers (new, and possibly the killer feature for federated orgs):** rule packs actively *teach* — "your org already has an SSO service, here's the endpoint; API keys come from here; secrets go in this vault; the API gateway is at X; docs live at Y." The artifact doesn't just constrain the agent — it makes the agent an ambassador for central services people didn't know existed. Every vibe-coded POC gets steered onto the paved road automatically.
- **Mechanism:** a lightweight CLI/daemon that syncs rule packs into repos and home dirs, verifies presence and integrity (hash-checked), and reports drift. Dependabot-meets-dotfiles for AI policy.
- **Format:** whatever is simplest that works. Current lean: canonical policy in YAML (or even plain markdown with front-matter) → rendered per-tool. Open question; don't over-engineer. It's possible the "format" is just markdown fragments with composition metadata.
- **Key property:** *deterministic*. The rules are literal text the agent reads every session — not probabilistic guidance, not post-hoc detection.

### Pillar B — Visibility Dashboard (the retention product)
Single shared/central pane of glass — with radical emphasis on **easy**: easy signup, easy onboarding, easy visualization, easy steering.

**Self-service registry (new pillar-B centerpiece):**
- Teams self-register in minutes: who we are, what we're building, which AI tools/models we use, what central services we consume (or didn't know about), expected duration (*"2-week POC"*, *"production, indefinite"*), and current needs.
- POC/experiment-friendly by design: registering a throwaway project takes 90 seconds and expires automatically; expiration prompts a keep/kill/archive decision — this alone gives leadership the shadow-IT map nobody has.
- Needs surfacing: "we need an API key," "we don't know how to do auth" — routed to the owning central team. The registry becomes the discovery layer between shadow IT and central IT, in both directions.

**AI usage telemetry**
- Which models are in use (provider, version), by which users/teams, at what frequency
- Token consumption: cost attribution, anomaly detection, runaway-agent alerts
- Model usage vs. routing policy (right model for the right job?)

**Rules & compliance**
- Rule pack versions deployed per repo/team; drift detection
- Policy exceptions requested/granted/expired
- Turn-limit and escalation-rule trigger events

**SDLC & deployment surface**
- Commit and deploy progress per governed repo
- Deploy locations (cloud accounts, regions, environments)
- Open ports across deployed services, with review status
- Secret management posture (vault adoption, inline-secret incidents)
- Account and role management (user/role inventory, orphaned account detection)

**SIEM export (decided: required, likely at MVP):** Splunk/Datadog/generic webhook + syslog export of governance events. This is table stakes for CISO credibility and often the difference between "toy" and "tool" in security review.

### Pillar C — Review Workflows (the expansion, kept deliberately light)
- Human-in-the-loop gates defined in policy: port openings, new cloud services, new model adoption, exceptions
- AI vendor/tool intake review — directly reusable at Caltech
- Audit trail exportable for SOC 2 / ISO 42001 / NIST AI RMF evidence

## 4. What This Is Not
- **Not enterprise AI governance software** — different altitude, different buyer motion, different soul; if a deal requires a six-month compliance-suite bake-off, it's the wrong deal
- Not an AI firewall/proxy (gateways like LiteLLM are integration partners, not competitors)
- Not a code scanner (we verify gates exist; we don't run scans)
- Not a model evaluation platform
- Not policy consulting delivered as PDFs (that's the services on-ramp)

## 5. Users & Buyers

| Persona | Pain | What they get |
|---|---|---|
| **Individual team lead** (entry point) | Wants agents to behave; doesn't know what central services exist | Rule packs + paved-path pointers, 90-second registration |
| **Platform/DevEx lead** (champion) | Agents doing chaotic things across repos | Composition, drift detection, sane defaults |
| **Central IT / CIO** (buyer in federated orgs) | Shadow IT invisible; central services underused | The registry + dashboard *is* the shadow-IT map |
| **CISO / security** | Unknown AI attack surface | Telemetry, review gates, SIEM export, audit trail |
| **Finance/ops** | Runaway token spend | Cost attribution, turn limits, anomaly alerts |

Initial ICP: 50–500 person engineering orgs heavy on Claude Code / Cursor, **plus federated institutions (universities, hospitals, government labs) where the self-service registry + paved-path discovery is uniquely valuable**. Caltech as archetype and design partner.

## 6. Architecture Sketch (v0 thinking — extend in Claude Code)

- **Control plane (SaaS):** policy source of truth, rule-pack composer/renderer, registry, dashboard, review workflows, org/team RBAC. Multi-tenant. Self-hostable option for institutions.
- **MCP server as product surface (promoted from open question — strong candidate for core architecture):** the SaaS wraps/ships an MCP server that agents connect to. This gives:
  - Live policy queries ("am I allowed to open a port?") instead of only static artifacts
  - Telemetry as a natural side effect of agent connections (which agent, which model, which session — no separate collector needed for a big slice of the data)
  - Registration and paved-path discovery *available to the agent itself* ("register this project," "where do I get an API key here?")
  - Static artifacts remain for determinism and offline/air-gapped cases; MCP adds the live layer. The two reinforce each other.
- **Collectors:**
  - CLI/daemon in repos and dev machines (rule sync, drift, local events)
  - CI/CD integrations (GitHub Actions first)
  - Cloud collectors, read-only (deploy locations, ports, IAM) — AWS/GCP first
  - Model-usage: provider admin APIs (Anthropic, OpenAI, Google) + optional gateway integration
- **Data model spine:** Org → Departments → Teams → Projects (with lifecycle/expiry) → Repos/Services → Policies → Deployments → Events
- **Design values:** privacy-respecting by default (aggregate telemetry, no prompt-content collection in v1); lightweight over complete; self-hostable; boring and fast

### Update propagation & freshness (DECIDED)

No cron, no daemon, no server push. Clients pull on their own cadence, and the cadence is itself policy:

- **Cadence rule lives in the pack.** A pack manifest may declare `update_check: { every: 7d, endpoint: <optional URL> }`. If no installed pack declares one, no automatic checks occur — the unconnected OSS path stays silent by construction (same stance as the telemetry consent model). With multiple packs, the strictest (shortest) cadence wins.
- **Trigger: piggyback on invocation.** Any `esc` command consults the local check log first; if the last successful check is older than the cadence, it performs one before proceeding. `esc sync` counts as a check by nature. A check never blocks or fails the invoking command; a failed check costs one short stderr line.
- **Check location.** Default: a lightweight ref comparison against each pack's existing git source (`git ls-remote`; no content fetched). If `endpoint` is set — a full URL carrying scheme/host/port, HTTPS required — the client instead makes a single request to that endpoint (typically the control plane), reporting pinned pack versions and receiving latest-version info. Either way the check only *informs*: rules still arrive exclusively through the normal `esc sync` fetch + signature-verification path. The observe-don't-distribute line is untouched.
- **Local log.** `.escapement/update-log.jsonl` — append-only, trimmed to the last N entries (default 50), gitignored per-clone state (`esc init` scaffolds the ignore entry). Each entry: timestamp, outcome (`ok_current` | `ok_updates` | `error`), per-pack pinned vs. latest versions, and whether a prompt was shown/accepted/declined.
- **On updates found.** Stderr notice always. Interactive TTY: prompt — Enter runs `esc sync` now, N/Esc skips (skip is logged). Non-TTY (CI, agent-invoked): notice only, never a hanging prompt.
- **Governance teeth via status.** Two new finding kinds: `pack-stale` (updates known available, not applied) and `check-overdue` (no successful check within cadence and the status-time attempt also failed). Both count as findings, so `esc status --check` exits 1 in CI while interactive use stays a polite prompt.
- **Server side (control plane, v0.2+).** The admin portal edits packs, so setting the cadence there publishes a new pack version — one policy channel, versioned and audited like any other rule change; clients pick it up on their next check. Every check request against a control-plane endpoint is logged (org, repo, outcome, versions, timestamp): a passive fleet heartbeat that powers the freshness dashboards, per-repo last-seen, and the staleness view of the shadow-IT map with no separate heartbeat infrastructure. When connected, check results also flow as `update_check` telemetry events under the decided consent model (connected = opt-out, unconnected = silent).

## 7. MVP Cut (strawman — challenge this in Claude Code)

**v0.1 (weeks, not months):**
1. Rule-pack composer: policy source → rendered CLAUDE.md/GEMINI.md/CURSOR.md/AGENTS.md/skills fragments
2. Self-service team/project registry with expiry
3. CLI sync + drift detection for GitHub repos
4. Minimal dashboard: registry view, repos, rule versions, drift, token usage via provider admin APIs

**v0.2:**
5. MCP server: live policy queries + connection telemetry + agent-initiated registration
6. Turn-limit + model-routing policy; violation events
7. SIEM export (webhook + syslog)

**v0.3:**
8. Open-port/deploy-location collectors (AWS read-only); port review workflow
9. Exceptions + audit trail export
10. Paved-path pointer packs (org service catalog → rule-pack fragments)

## 8. Business Model — Open Source + SaaS (current lean)

**The copyability concern, addressed head-on:** yes, this is easy to copy once seen. That's true of most great developer tools (GitHub, Vercel, Tailscale were all "copyable"). The defenses are not secrecy:

1. **Open source *is* the moat, not the vulnerability.** If the rule-pack format, renderer, and CLI are open and become the *standard* way orgs express AI policy-as-artifacts, copying the format helps us — every community rule pack, every integration, every blog post compounds our position. Being copied by closed-source competitors then means fighting our free, standard, community-backed core.
2. **The SaaS value is aggregation, not code:** hosted control plane, multi-org dashboard, SIEM export, review workflows, SSO, retention, support. Boring to self-host at scale, cheap to buy.
3. **Speed + distribution + trust** beat feature parity. First-mover with the Caltech reference story ("governs AI at a top research institution") and community rule packs is hard to displace.
4. **Community rule packs as network effect:** "the Django pack," "the HIPAA pack," "the university research-data pack" — contributed, shared, versioned. Copycats get the code, not the library.

**Proposed split (for brainstorm):**
- **Open source:** rule-pack format/spec, renderer, CLI/daemon, base MCP server, reference dashboards
- **SaaS/paid:** hosted multi-tenant control plane, org registry at scale, SIEM export, review workflows, audit/evidence export, SSO/SCIM, support; self-hosted enterprise licenses for institutions
- **Services on-ramp:** governance discovery engagements (Caltech Phase 1 shape) that deploy the product as the deliverable

## 9. Competitive Frame (validate with research)

- Enterprise AI governance suites: different altitude — explicitly not our market or our motion
- AI gateways (LiteLLM, Portkey, Kong AI): traffic-level; integration partners
- AI-SPM (Wiz et al.): cloud-side AI resource discovery; partial collector overlap
- **Open lane:** nobody owns "policy-as-artifacts for agentic development + unified usage/SDLC dashboard + self-service registry for federated orgs." Verify in research pass — the fast-follow risk (Section 8) makes the open-source-standard play more urgent, not less.

## 10. Caltech Relationship (see role spec §8)

- Build independently on Tensor Group time/infrastructure
- License/deploy into any Caltech engagement — never work-for-hire; explicit IP language, attorney-reviewed
- Caltech as design partner → the federated-institution segment + the reference story
- Caltech's environment (departments, shadow IT, unknown central services, POC sprawl, mixed AI adoption) is literally Section 2 — discovery there doubles as product research

## 11. Open Questions for Claude Code Brainstorm

*(All prior questions remain open; new thoughts noted inline.)*

1. **Name.** ("Paved road" / "charter" / mechanical "governor" directions — a governor is literally a deterministic control device for a powerful engine.)
2. **Enforcement vs. observation in v1** — drift *detection* vs. drift *blocking* via CI check?
3. **Rule-pack format** — current lean: simplest thing that works; YAML-or-markdown canonical source rendered per-tool (CLAUDE.md, GEMINI.md, CURSOR.md, AGENTS.md, skills, agents dirs). Possibly no schema at all in v0.1, just composed markdown fragments. Decide by building.
4. **MCP server as core product surface** — promoted to a strong architectural lean (§6); remaining questions: authn model for agent connections, offline behavior, how much policy lives server-side vs. artifact-side.
5. **Turn-limit enforcement mechanics** — artifact instruction only, CLI wrapping agent invocation, MCP-server-side session accounting, or gateway integration?
6. **Multi-agent orchestration standards** (A2A, MCP evolutions) — what to attach to?
7. **Open-core split** — proposed split in §8; pressure-test what must stay open for the standard play to work vs. what's safely paid.
8. **Pricing research** — comparable DevEx/security tools per-seat/per-repo benchmarks.
9. **SIEM export** — DECIDED: yes, required; likely v0.2. Remaining: which format first (CEF? OCSF? plain JSON webhook?), Splunk vs. Datadog priority.
10. **Regulatory tailwinds** — EU AI Act, ISO/IEC 42001, NIST AI RMF: which actually drive urgency for the ICP?
11. **NEW — Registry lifecycle design:** what does the expire → keep/kill/archive flow look like? Who gets nagged? What's the default TTL for a "POC"?
12. **NEW — Paved-path catalog ingestion:** how does an org's existing service catalog (auth endpoints, key issuance, vaults, gateways, docs) get into rule packs — manual YAML, CMDB import, or crawl-and-suggest?
13. **NEW — Adoption sequencing in federated orgs:** single team → department → central IT, or land with central IT and push down? (Lean: bottom-up land, top-down expand — but Caltech may invert this.)
14. **NEW — How copy-resistant is the community rule-pack library really?** What makes packs sticky (versioning, signing, provenance, ratings)?
