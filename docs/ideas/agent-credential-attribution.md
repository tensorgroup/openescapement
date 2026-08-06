# Agent credential use, attribution & audit

**Status:** Raw idea, unscoped. No commitment to build any tier of this.
**Touches:** spec §3 (Pillar A rules, Pillar B telemetry, Pillar C review), §4 (boundaries — this idea pushes hard on them), §11 Q4 (MCP authn), Q6 (which standards to attach to), Q9 (SIEM export).
**Companion:** `docs/roadmap/v0.2-mcp-and-telemetry.md` (the auth and consent models there constrain anything here).

## The problem

Coding agents run with their human operator's ambient credentials. The user is logged into `gh`, `aws`, `gcloud`, `kubectl`, a browser session, an internal API with a token in `~/.netrc` or an env var, or an MCP server holding an OAuth token the user granted once. The agent inherits all of it, because inheriting it is what makes the agent useful.

Three consequences, in increasing order of how much they should bother a security team:

1. **No scoping.** The agent has exactly the human's privileges, never less. A person who can drop a production table has an agent that can drop a production table. Least privilege has no expression here at all.
2. **No attribution.** The target system's audit log records the human. GitHub says alice pushed. CloudTrail says alice deleted the bucket. Nothing distinguishes "alice decided this" from "alice's agent decided this while alice was at lunch". Post-incident reconstruction is guesswork, and the human carries accountability for decisions they did not make.
3. **Confused deputy.** An agent holding the human's credentials and also reading untrusted input (an issue body, a fetched page, a dependency's README) is a prompt-injection target with real privileges attached. Attribution does not fix this. Separate, downscoped identity partly does.

This is a governance problem with no owner. Identity providers model humans and services, not "a human's agent acting for twenty minutes". The agent vendors ship sandboxing and permission prompts, which are per-tool and per-session and invisible to the org. Nobody is rendering an org's answer to "how may an agent use credentials here?" into a place the agent will actually read it.

That last sentence is our wedge. It is the same shape as the problem OpenEscapement already solves.

## Why it plausibly belongs in this product

- **Pillar A already renders exactly this kind of rule.** `examples/packs/acme-org/rules/agent-behavior.md` says escalate before "granting any credential access", and the `acme-vault` skill already points the agent at the org's secret store instead of letting it invent one. This idea is that seed grown into a real surface: a credential-class taxonomy plus a paved path to the org's agent-credential issuance.
- **`paved_path(need)` in the v0.2 MCP server is the delivery mechanism.** "Where do I get an API key here?" is already a designed tool call. "Where do I get an *agent-specific* key here?" is the same call with a better answer.
- **The audit tier is the telemetry pillar with a different event type.** No new architecture, no prompt or code content, and it feeds the SIEM export already committed in §11 Q9.
- **It sells to two of the five personas in §5.** The CISO row already reads "unknown AI attack surface → audit trail". Finance/ops care about the same trail for a different reason.

## Four tiers, weakest to strongest

Deliberately laddered. Each tier is independently shippable, and each is honest about what it does not do. Tiers 0 and 3 need nothing from anyone outside this repo. Tier 1 needs convention. Tier 2 needs the org to already have identity infrastructure, and our role in it is small on purpose.

### Tier 0 — Instruction-level policy (ships on v0.1 mechanics, no new infra)

A rule-pack fragment that classifies credential surfaces and tells the agent what it may do with each. Something like three classes:

- **Agent-usable** — the agent may use the human's ambient credential unsupervised (read-only APIs, local dev, scratch namespaces).
- **Confirm-per-action** — the agent must stop and get an explicit human confirmation for each use (anything writing to a shared system).
- **Agent-forbidden** — the agent must never use the human's credential, and must instead request an agent-specific one via the paved path (production, anything money-moving, anything touching customer data).

Plus the standing instruction: when you need a credential you do not have, do not reach for the human's session. Stop, state what you need and why, and point them at `<org issuance path>`.

**What it does:** makes the org's answer explicit, versioned, signed, and present in the file every agent reads. Turns an unasked question into a reviewed one.

**What it does not do:** enforce anything. An agent that ignores its instructions ignores these too. This tier is worth building anyway, because most agent credential misuse is drift and accident, not defiance, and because it is the artifact a security reviewer can point at.

### Tier 1 — Attribution markers (cheap, needs convention not cooperation)

Ask the agent to tag actions it originates so the marker lands in the *target system's* audit log, which is the only log a security team already trusts. Candidate carriers, roughly in order of how easy they are to adopt:

- **Git commit trailers.** Needs no server-side cooperation at all, and PRs are already a review surface. Note the tension: `Co-Authored-By: Claude` is the current de facto marker and it is contested (this repo's own agent instructions ban it). A purpose-built trailer naming session and pack version, rather than co-authorship, may be the more defensible design.
- **HTTP request headers.** A structured actor header on agent-originated requests. W3C Trace Context `tracestate` is an interesting carrier because it is designed to survive proxies. RFC 9421 (HTTP Message Signatures) is the route if the marker ever needs to be non-repudiable rather than advisory.
- **Cloud-native session identity.** These are the strongest option in this tier because the platform enforces the marker rather than trusting it: AWS STS `SourceIdentity` propagates through role chaining into CloudTrail and cannot be altered by the assuming principal; GCP service-account impersonation records the delegation chain; Kubernetes `Impersonate-User` / `Impersonate-Extra-*` headers are recorded in the API server audit log.

**What it does:** makes agent-originated actions separable from human-originated ones in logs that already exist.

**What it does not do:** resist a determined agent. Self-reported attribution is spoofable by construction; the cloud-native variants are the exception, and they are really Tier 2 in disguise. Treat this tier as defence against accident and ambiguity.

### Tier 2 — Separate agent identity (the actual answer, mostly not ours to build)

Stop piggybacking. Mint the agent a short-lived, downscoped principal derived from the human's identity, so the credential itself carries both the delegation and the scope reduction.

The standards that already model this, which matters because inventing here would be a mistake:

- **OAuth 2.0 Token Exchange (RFC 8693)** is close to purpose-built for this. The `act` claim expresses "actor A acting on behalf of subject B", and `may_act` expresses who is permitted to. A delegation token saying "claude-code session X acting for alice, scoped to these three operations, expiring in 30 minutes" is a standards-compliant artifact today, not a research project.
- **SPIFFE/SPIRE** for agents running in CI or containers, where workload identity is the right frame.
- **Cloud primitives:** AssumeRole with a narrowing session policy plus `SourceIdentity`; GCP short-lived impersonated tokens; Azure managed identity.

**Our role, stated narrowly on purpose:** we do not become an identity provider, a secrets broker, or a token minting service. Spec §4 rules out being an AI firewall or proxy, and this is where that boundary earns its keep. What we do is what we already do: the rule pack says *which class of credential an action requires*, and the paved path says *where this org issues agent credentials*. The org's IdP issues them. If that infrastructure does not exist, the pack says so and escalates, which is itself useful information for the dashboard (an org-wide "no agent identity path exists" finding is a genuine Pillar B insight).

The risk to watch: this tier is where scope creep would kill us. Every conversation about it will drift toward "we could just issue the token ourselves". The answer is no.

### Tier 3 — Local action log with optional upload (universal fallback)

When we can neither prevent nor attribute, record. An append-only local log of credential-touching actions: which surface, which credential reference, which agent tool and session, which pack version was in force, timestamp, and the outcome. Never the secret. Never prompt or code content.

This inherits the v0.2 consent model exactly, which is what makes it safe to propose: **unconnected OSS CLI collects nothing** (no control plane to send to, so the pure open-source path stays silent by construction), and connected orgs get admin-controlled opt-out. It feeds the SIEM export in §11 Q9 without a new pipeline.

Weakest tier, and the only one that works regardless of vendor cooperation, standards adoption, or whether the org has an IdP. That combination usually means it ships first.

## What this cannot be

Worth writing down before anyone gets excited, because the gap between tiers 0/1 and real enforcement is where this idea could quietly turn into a lie:

- **Instructions are not controls.** Anything genuinely binding lives at the credential issuer, the gateway, or the OS sandbox. We can render a rule that says "never use production credentials"; we cannot stop it. Every artifact and every piece of marketing has to say this plainly, or the first incident makes us the product that promised enforcement and delivered prose.
- **Self-reported attribution is spoofable.** Only Tier 2, where the credential is genuinely different, gives attribution that survives an adversary.
- **This does not solve prompt injection.** Downscoping shrinks the blast radius. It does not close the hole.
- **The audit log is not proof of authorization.** It reconstructs what happened. It does not mean anyone approved it.

## Open questions

1. **Is there already a cross-vendor agent-attribution convention forming?** This must be checked against primary sources before designing anything, not answered from memory. It is the same failure mode `docs/roadmap/vendor-guidance-tracking.md` exists to prevent, and it may belong in that doc's quarterly sweep. Ties to §11 Q6.
2. **Which carrier first?** Git trailers are the cheapest real win (no server cooperation, PR review surface already exists). Cloud session tags are the strongest but only cover cloud actions.
3. **Does the credential-class taxonomy belong in the pack schema or stay prose?** Schema makes it queryable via MCP `check_policy(action)` and lintable; prose ships this week. YAGNI says prose until a second consumer exists.
4. **CLI or MCP server for the Tier 3 log?** The CLI is the v0.1 surface with no server dependency, but it does not observe the agent's actions. The MCP server observes but only sees MCP-mediated actions. Neither sees a raw `curl`. This gap may be fundamental.
5. **Where does this sit against the gateway partners?** LiteLLM and similar are named integration partners, not competitors. If a gateway is in the path it can enforce what we can only request, which argues for us emitting policy the gateway consumes rather than duplicating it.
6. **Does an org without any agent-identity path get a finding?** Proposed yes: a Pillar B dashboard row. Needs a check on whether that reads as useful or as nagging.
7. **How much of this is v0.2 versus much later?** Tier 0 is nearly free today. Tier 3 rides the telemetry work. Tiers 1 and 2 depend on ecosystem answers we do not have.
