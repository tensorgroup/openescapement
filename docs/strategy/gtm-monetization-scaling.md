# GTM, Monetization, Onboarding & Scaling

**Status:** Working strategy (2026-07). Companion to `ai-governance-product-spec.md` §5, §8 and `licensing-and-open-core.md`.

## Monetization

**Model: open-core SaaS with a services on-ramp.** The CLI/format stays free forever; money comes from aggregation.

**Product tiers (strawman):**

| Tier | Who | What | Price anchor |
|---|---|---|---|
| **OSS** | Any team | CLI, packs, Action, base MCP server | Free |
| **Team** | Platform lead, 5–50 devs | Hosted dashboard: repos, pack versions, drift across the fleet; registry for their projects | ~$10–15/dev/mo (Snyk Team ~$25, Dependabot free — sit below security scanners, above linters) |
| **Org** | Central IT/CISO, 50–500 devs | Everything + org registry w/ expiry workflows, SIEM export, review workflows, audit evidence export, SSO/SCIM | ~$20–30/dev/mo or platform fee + per-dev; benchmark GitHub Advanced Security (~$49/committer) as ceiling |
| **Institution** | Universities, hospitals, gov labs | Self-hosted control plane, enterprise license + support | Annual contract, $30–100k depending on size |

**Pricing principles:** per-developer (not per-repo — repos are what we *want* proliferating); free tier generous enough that a single team never hits a wall (the bottom-up motion depends on it); the paid line is aggregation, never core governance capability.

**Services on-ramp:** governance discovery engagements (the Caltech Phase 1 shape) priced as consulting, delivering the product as the artifact. Early revenue, design-partner intimacy, reference stories. Guardrail: services must always end in product deployment, never bespoke PDFs — otherwise we become the consultancy we're replacing.

## Getting users

**Phase 1 — design partners (now → +6 mo):**
- Caltech as archetype federated institution (see product spec §10; independent IP, licensed in).
- 2–3 more design partners from opposite corners: one 50–200-dev startup heavy on Claude Code/Cursor, one mid-size company with a platform team. Free Org tier for feedback + logo rights.

**Phase 2 — community launch:**
- Launch narrative: **"Your agents already read CLAUDE.md. Your policy should too."** Show HN + a technical blog post on the managed-block/signing design (security depth is the differentiator; engineers share fail-closed design writeups).
- **Community packs are the growth engine:** seed 5–10 genuinely useful packs (the Django pack, the HIPAA-adjacent research-data pack, the university-lab pack, the "solo dev sane defaults" pack). Every pack is a landing page and a reason to install.
- GitHub Action marketplace listing; awesome-lists in the Claude Code/Cursor ecosystem; conference circuit where federated buyers gather (EDUCAUSE, Internet2, USENIX LISA/SREcon).
- The `GOVERNANCE.md` render is organic distribution: it names the tool in every governed repo a contractor or student opens.

**Phase 3 — land-and-expand:** bottom-up land (a team lead installs in an afternoon), top-down expand (platform lead standardizes; central IT buys the dashboard for the shadow-IT map). In federated institutions this may invert — central IT sponsors, departments adopt voluntarily because packs *help* (paved-path pointers), not because they're mandated. The product works in both directions by design.

## Onboarding (the 90-second story)

1. `go install …/esc@latest` (later: brew, signed binaries)
2. `esc init` in one repo
3. Point at a starter pack — either the community starter or `packs/your-org` scaffolded from our template
4. `esc sync` → agents are governed; `GOVERNANCE.md` appears; demo "ask your agent where API keys come from"
5. Add the Action → drift gates PRs
6. (Paid begins) Connect repo/org to the dashboard → fleet view, registry, expiry

Friction budget: steps 1–5 must need no account, no server, no sales conversation. The moment of magic is step 4 — the agent *answering with the org's paved-path knowledge*.

North-star metric: **governed repos** (weekly-synced). Secondary: packs published, orgs with ≥5 governed repos (expansion signal), drift-check CI adoption.

## Scaling

**Product sequencing** (matches spec §7): v0.1 CLI loop (done) → v0.2 MCP server + telemetry + SIEM → v0.3 registry/dashboard + collectors + review workflows. Each layer monetizes the layer below; none invalidates prior adoption.

**Technical scaling:** the deliberate architecture means the free tier costs us ~nothing (no server in the loop — git is the distribution network). The SaaS control plane is read-mostly aggregation (events, registry) — boring, cheap, multi-tenant Postgres-shaped. Self-hosted institution deployments stay viable because the control plane observes rather than distributes.

**Org scaling:** solo/Tensor Group → first hire is DevRel-shaped (packs, docs, launch) before a second engineer; services engagements fund headcount without dilution until SaaS revenue supports it.

**Risks & watchpoints:**
- **Fast-follow by a gateway or agent vendor** (LiteLLM, Cursor, Anthropic itself shipping org policy). Mitigation: be the *neutral, multi-tool* standard — vendors' own policy features are single-ecosystem by nature; speed + community packs + the federated-org niche they won't chase.
- **Instruction-file standards shift** (AGENTS.md consolidation, MCP evolutions). Mitigation: renderer architecture makes new targets cheap; track standards actively.
- **Free tier too good** (nobody needs the dashboard). Watch expansion signal; the registry/expiry workflow is the paid hook federated orgs can't self-assemble.
