# Model guidance refresh and access routes — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring the portal's model guidance and the example packs up to the 2026-10-24 lineup (Gemini 4 Argon, Kimi K3.1 Pro, DeepSeek V4.1 Flash through OpenRouter), add an "Access routes" section to every vendor note (plan plus vendor CLI, versus direct API, versus OpenRouter, with prices and when a plan pays for itself), add example packs for Google, Kimi, and DeepSeek, update the `model-seats` pack for profiles and lead rotation, and add a tools page.

**Architecture:** Content changes inside the existing structure: `internal/guidance/models/*.md` (vendor notes), `models.yaml` (registry), `examples/<vendor>/` (starters), `examples/packs/<name>/` (packs). One Go test is added (every vendor note has an Access routes section). No other code changes.

**Tech Stack:** Go 1.24 tests (`go test ./...`), markdown, YAML.

**Spec:** [Balancewheel](https://github.com/tensorgroup/balancewheel), `docs/superpowers/specs/2026-10-24-profiles-lead-rotation-cockpit-design.md` (build-order step 1; sections "Seats added in this round", "The sandbox", "Lead rotation", "Jev", "Third-party tools"). Also read this repo's `docs/pack-authoring.md` (the committed style rule for every rule file and catalog note) and `AGENTS.md`.

## Global Constraints

- Every factual claim in a vendor note carries a source in that note's `## Sources` list; a price carries its "as of" date. A number that cannot be sourced is left out, not estimated.
- Example fragments (`internal/guidance/models/examples/**`) carry **no absolute prices** (`TestExampleFragmentsCarryNoAbsolutePricing`).
- Every `current` model of a full-depth vendor (anthropic, openai, google) has a starter (`TestEveryCurrentFullDepthModelHasStarter`).
- Model ids live in catalogs and notes, never in rule text as an interface; seats are named by role (existing `model-seats` convention).
- Pack rule files follow `docs/pack-authoring.md`: imperative, one instruction per sentence, one term per concept.
- Every pack version bump and every guidance change gets a `CHANGELOG.md` `[Unreleased]` entry in the same commit.
- Commit messages: lowercase, `feat(guidance):` / `feat(packs):` / `docs:` / `test:` prefixes, as in the existing log. No AI co-author trailers.
- Before claiming done: `go test ./...`, `go vet ./...`, `gofmt -l .` prints nothing.

## Review Focus

- **A stale price or plan tier shown as current.** Vendor plans change monthly. Every price line names its as-of date and source; Task 1's facts sheet is the only place numbers come from.
- **A "plan pays for itself" claim with no arithmetic.** Each Access routes section that recommends a plan shows the break-even in tokens or sessions per month, computed from the sourced prices, so a reader can redo it.
- **OpenRouter promo prices presented as list prices.** OpenRouter showed `deepseek/deepseek-v4.1-flash` at $0.02 / $1.00 per million on 2026-10-24, far under DeepSeek's own list; state it as a provider price that can end without notice (the `zai-models` pack's wording).
- **Kimi Code's permission rules described as enforcement.** Verified 2026-10-24: Kimi Code CLI 2.3.1 ignores `[[permission.rules]]` deny rules in `-p` mode. Any governance line about Kimi Code must say OS-level sandboxing is the enforcement.
- **Stale routing after a demotion.** Starters and governance bullets that route to a model this plan moves to legacy stay green in every test; Task 3 step 3b and Task 4 step 2 reconcile them by grep.
- **A new pack that parses but renders badly.** `TestEveryExamplePackSyncs` covers parse and render; read the rendered `AGENTS.md` block of each new pack once by eye (Task 6 step).

---

### Task 1: Facts sheet (research, not committed)

**Files:**
- Create: `$SCRATCH/facts-2026-10-24.md` (the executor's scratch directory; never committed)

**Interfaces:**
- Produces: one table per item below with `value | source URL | retrieved date`. Tasks 2 to 7 take every number and every model id from this sheet and copy its source URL into the note's Sources list.

- [ ] **Step 1: Collect, from vendor pages only (OpenRouter's model pages count for OpenRouter prices):**
  - Gemini 4 Argon: API model id(s), context window, API price per million (in/out, and any tier breakpoints), release status (preview or GA), whether it supersedes `gemini-3.1-pro-preview`; agy model ids (`gemini-4-argon-low|medium|high`, observed in `agy models` on agy 1.4.7); OpenRouter id `google/gemini-4-argon` price (observed $2 / $10 on 2026-10-24) and the `:batch` variant.
  - Kimi K3.1 Pro and K3.1 Pro High-Speed: API ids and prices; Kimi Code CLI model ids `kimi-code/k3.1-pro` and `kimi-code/k3.1-pro-highspeed` (observed in a Kimi Code 2.3.1 config); whether K3 is superseded.
  - DeepSeek V4.1 Flash: DeepSeek list price (the note already says $0.30 / $1.20 peak, $0.15 / $0.60 off-peak — re-verify), OpenRouter ids `deepseek/deepseek-v4.1-flash` and `deepseek/deepseek-v4.1-flash-1017` with prices.
  - Subscription plans that include a vendor coding CLI, with monthly price and what the vendor says about usage limits: Anthropic (Pro, Max tiers; Claude Code), OpenAI (Plus, Pro; Codex CLI), Google (the AI plan tiers that include Antigravity CLI `agy`, and the fact that agy 1.4.7 lists Claude Opus 5.5 and Sonnet 5.5 as selectable models), Moonshot (Kimi Code plan tiers, including the $30/month tier), xAI and DeepSeek if they have one.
  - Kimi Code's documented permission semantics: what its docs say about `[[permission.rules]]` matching and about the permission mode `-p` runs in (the docs say non-interactive mode uses the `auto` permission mode), and its hooks' fail-open table. Task 4 cites these next to the 2026-10-24 test observation.
- [ ] **Step 2: Check the sheet.** Every row has a URL and a date; rows with no vendor source are deleted.

### Task 2: Every vendor note gets an Access routes section

**Files:**
- Test: `internal/guidance/checklist_test.go`
- Modify: `internal/guidance/models/{anthropic,openai,google,kimi,deepseek,grok}.md`

**Interfaces:**
- Consumes: Task 1's sheet.
- Produces: a `## Access routes` section in every vendor note, placed after `## Governance` and before `## Sources`.

- [ ] **Step 1: Write the failing test** `TestEveryVendorNoteHasAccessRoutes` next to `TestEveryVendorNoteHasSources`: for each key in `VendorOrder`, the note contains `## Access routes`, and that heading's index is lower than the index of `## Sources`.
- [ ] **Step 2: Run** `go test ./internal/guidance -run TestEveryVendorNoteHasAccessRoutes -v` — expect FAIL for all six vendors.
- [ ] **Step 3: Write the six sections.** Each is a short table, one row per route that exists for that vendor: **plan + vendor CLI** (plan name, monthly price, CLI name, what the vendor says about limits), **direct API** (per-million prices), **OpenRouter** (id, price, "provider prices can change without notice"). Below the table, two to four bullets: when the plan pays for itself (break-even arithmetic from the table, e.g. "at $X/M output, the $Y plan breaks even at Z M output tokens a month"); that repo-aware agent work through the vendor's own CLI on a plan is usually the cheaper route for daily use; that metered routes suit fast, small, occasional calls. Google's section names that agy also serves Claude Opus 5.5 and Sonnet 5.5 on the Google plan. Kimi's section names the Kimi Code plan tiers. Add each new source URL to that note's `## Sources`.
- [ ] **Step 4: Run** `go test ./internal/guidance -v` — expect PASS.
- [ ] **Step 5: Commit** `feat(guidance): access routes — plan and cli versus api versus openrouter, per vendor` (with a CHANGELOG `[Unreleased]` line).

### Task 3: Google — Gemini 4 Argon

**Files:**
- Modify: `internal/guidance/models/models.yaml` (google vendor), `internal/guidance/models/google.md`, `internal/guidance/models/examples/google/model-routing.md`, and every Google starter that routes planning or review to `gemini-3.1-pro-preview`: `starter-gemini-3.8-flash.md`, `starter-gemini-3.7-flash.md` (deleted in the review fix pass, since 3.7 Flash is legacy), `starter-gemini-3.5-flash-lite.md`, `starter-gemini-3.1-pro-preview.md`
- Create: `internal/guidance/models/examples/google/starter-gemini-4-argon.md`

**Interfaces:**
- Consumes: Task 1's Gemini 4 Argon rows.
- Produces: registry entry `id: <API id from the sheet>`, `tier: frontier`, `roles: [planning, plan-check, review]`, `status: current`, `verified: 2026-10-24`, `starter: examples/google/starter-gemini-4-argon.md`.

- [ ] **Step 1: Add the registry entry first in the google list** and run `go test ./internal/guidance -v` — expect FAIL in `ParseRegistry` (starter not found).
- [ ] **Step 2: Write the starter** in the shape of `starter-gemini-3.8-flash.md` (title `Gemini 4 Argon governance (<id>)`, five bullets, no prices): route planning, plan-check, and review to it; name the agy effort-suffixed ids as the CLI route; say which model it supersedes, per the sheet.
- [ ] **Step 3: Update `google.md`:** a `### Gemini 4 Argon (<id>) - frontier` section at the top of `## Models` (what it is, context, price with date, roles); move `gemini-3.1-pro-preview` to `legacy` in both the note and the registry **only if** the sheet says Argon supersedes it — and if so, remove its `starter:` field from the registry (a legacy model is not offered as a starter; the file stays embedded and editable) and rewrite its starter's first bullet to point at Argon; update `## Governance` routing bullets; update `model-routing.md` and the three Flash starters so none routes planning, plan-check, or review to a legacy model.
- [ ] **Step 3b: Grep check:** `grep -rn "gemini-3.1-pro-preview" internal/guidance/models` — every remaining hit is either the legacy section of `google.md`, its own starter, or a "supersedes" line.
- [ ] **Step 4: Run** `go test ./... ` — expect PASS.
- [ ] **Step 5: Commit** `feat(guidance): gemini 4 argon becomes google's frontier choice`.

### Task 4: Kimi — K3.1 Pro, and Kimi Code's enforcement gap

**Files:**
- Modify: `internal/guidance/models/models.yaml` (kimi vendor), `internal/guidance/models/kimi.md`

**Interfaces:**
- Produces: registry entries for K3.1 Pro (`tier: frontier`, `roles: [planning, plan-check, review]`) and K3.1 Pro High-Speed (`tier: fast`, `roles: [coding, bulk]`), `status: current`, `verified: 2026-10-24`, ids from the sheet. Kimi is not a full-depth vendor, so no starter is required.

- [ ] **Step 1: Add both entries;** set K3 to `legacy` only if the sheet says K3.1 Pro supersedes it.
- [ ] **Step 2: Update `kimi.md`:** model sections in the existing shape; reconcile `## Governance` routing (planning, plan-check, review to K3.1 Pro if it supersedes K3; fast reads to K3.1 Pro High-Speed) so no bullet still routes to a superseded model; put the K2.5 sunset lines in past tense ("was sunset on 2026-08-31"); add one bullet with this content: "Do not rely on Kimi Code's permission rules to stop writes in non-interactive (`-p`) runs. Non-interactive runs use the `auto` permission mode, deny rules did not stop writes in a 2026-10-24 test of Kimi Code 2.3.1, and hooks fail open by Kimi Code's own documentation. Run an unattended Kimi Code session under an OS sandbox that denies writes outside its working directory." Sources: the Kimi Code docs pages for permissions, `-p` mode, and hooks (Task 1 sheet); the test is labelled as a first-party observation with its date.
- [ ] **Step 3: Run** `go test ./...` — expect PASS.
- [ ] **Step 4: Commit** `feat(guidance): kimi k3.1 pro; kimi code permission rules are not enforcement in -p`.

### Task 5: DeepSeek — the OpenRouter route for V4.1 Flash

**Files:**
- Modify: `internal/guidance/models/deepseek.md` (and `models.yaml` only if the sheet changes a status)

- [ ] **Step 1: Update the V4.1 Flash section:** re-verified list prices; the OpenRouter ids `deepseek/deepseek-v4.1-flash` (the alias to use) and `deepseek/deepseek-v4.1-flash-1017` (dated, priced higher on 2026-10-24; superseded: -1017 is the 2026-10-17 release with weekday peak pricing, not a dated snapshot of the alias) with their prices as provider prices; recommended roles gain "fast second opinions and triage" if consistent with the sheet.
- [ ] **Step 2: Run** `go test ./...` — expect PASS.
- [ ] **Step 3: Commit** `feat(guidance): deepseek v4.1 flash through openrouter`.

### Task 6: Example packs — google-models, kimi-models, deepseek-models

**Files:**
- Create: `examples/packs/google-models/{pack.yaml,rules/model-selection.md}`, `examples/packs/kimi-models/{pack.yaml,rules/model-selection.md}`, `examples/packs/deepseek-models/{pack.yaml,rules/model-selection.md}`
- Modify: `examples/README.md` (list the three), `README.md` (the `esc`-only paragraph that names the model packs, ~line 175), `CHANGELOG.md`

**Interfaces:**
- Consumes: Tasks 1, 3, 4, 5.
- Produces: three packs at `version: 0.1.0`, `schema: 1`, shaped like `examples/packs/zai-models` (a model table with id, price with date, use-for; "Rules of thumb" bullets; a catalog with `preferred` / `allowed` / `banned` entries and notes).

- [ ] **Step 1: Write the three packs.** Google: Gemini 4 Argon `preferred` for planning and review, the Flash tiers by role, retired previews `banned` by name if the sheet lists any; a rule line that agy is the plan route. Kimi: K3.1 Pro `preferred` for planning and review, High-Speed for fast reads, K2.5 `banned` (sunset 2026-08-31); the Kimi Code `-p` sandbox rule from Task 4 as a rule line. DeepSeek: V4.1 Flash `preferred` for fast, cheap second opinions and triage, named by both ids — `deepseek-flash` on DeepSeek's own API and `deepseek/deepseek-v4.1-flash` on OpenRouter — so direct-API users get the right one; the dated `-1017` snapshot `allowed` (superseded: -1017 is the 2026-10-17 release with weekday peak pricing, not a snapshot); "set a spend cap on the OpenRouter key".
- [ ] **Step 2: Run** `go test ./... -run TestEveryExamplePackSyncs -v` — expect PASS for all packs, new ones included.
- [ ] **Step 3: Render each new pack once into a temp repo** (`esc init` / `esc sync` against the pack path, per `examples/README.md`) and read the managed block in `AGENTS.md` by eye: no broken tables, no leftover template text.
- [ ] **Step 4: Commit** `feat(packs): google, kimi, and deepseek model packs`.

### Task 7: model-seats pack 0.3.0 — profiles, lead rotation, enforcement, the dated roster

**Files:**
- Modify: `examples/packs/model-seats/pack.yaml`, `examples/packs/model-seats/rules/seat-policy.md`, `examples/packs/model-seats/rules/decision-model-routing.md`
- Create: `examples/packs/model-seats/rules/lead-rotation.md`

**Interfaces:**
- Consumes: the spec sections "Who leads, who moderates", "The sandbox", "Lead rotation", "Jev: from advice to acting", "Lead scoring".

- [ ] **Step 1: `seat-policy.md`:** add one defining sentence before the seat table: "The lead is the agent the user works in for a project; the lead is the moderator of every panel it convenes." Then use `lead` and `moderator` consistently in the new rules. Add to the conduct rules — "Enforce read-only at the OS level. A seat's own permission settings are not enforcement until an attempted write fails with an OS error on a fresh and a resumed session." Add the moderator-is-recorded rule: "Record which lead moderated each panel, by resolved model id." In `pack.yaml`, reconcile the banned writable-seat catalog note (~line 51: "its vendor's seat is excluded from the panel"): a lead's own vendor may still sit as a fresh blind session with none of the lead's conversation; only a seat that shares the lead's session is excluded.
- [ ] **Step 2: `lead-rotation.md`** (new): leads are named per profile in an order; a lead runs only under containment verified both ways by attack; switch away on an observed plan cap; return to the preferred lead at the next task boundary after its recorded reset; never two writable leads in one worktree; log every switch with from, to, reason; score leads from git, review findings on their commits, and the user's rating, never from the lead's own report; no composite lead score. Imperative, one instruction per sentence.
- [ ] **Step 3: `decision-model-routing.md`:** add the acting mode — the router may choose seats only within a profile's required seats and minimum; promotion requires shadow evidence with a coverage-based miss count (a confirmed major finding group no retained seat raised); acting mode runs a full-roster audit on a fixed share of panels and returns to shadow on one audit miss.
- [ ] **Step 4: `pack.yaml`:** `version: 0.3.0`; add `rules/lead-rotation.md`; catalog adds three trial seats dated 2026-10 (Google peer seat on `gemini-4-argon-high` via agy; Kimi peer seat on `kimi-code/k3.1-pro` via Kimi Code under an OS sandbox; fast metered seat on `deepseek/deepseek-v4.1-flash` via pi on OpenRouter), each `status: review-required` with the note "on trial from 2026-10-24 with a fresh scoreboard"; and lead entries for a non-Anthropic-plan Claude lead (Opus 5.5 via agy on the Google plan, `review-required`, "lead quality not yet scored").
- [ ] **Step 5: Run** `go test ./...` — expect PASS; render the pack once as in Task 6 step 3.
- [ ] **Step 6: Commit** `feat(packs): model-seats 0.3.0 — lead rotation, os-level enforcement, trial seats`.

### Task 8: Tools page and README pointer

**Files:**
- Create: `docs/agent-tools.md`
- Modify: `README.md` (one line in the model-guidance section linking it), `CHANGELOG.md`

- [ ] **Step 1: Write `docs/agent-tools.md`:** dated 2026-10-24, scoped to what an org governs — what each tool is, what it costs, and the setting that matters for governance — and pointing to Balancewheel's README for per-user setup (Balancewheel owns per-user agent configuration; this page does not give install recipes for it). The user asked for these tools to be documented in both repos; this is esc's half. Sections: the vendor CLIs (`claude`, `codex`, `agy`, `kimi`, `pi`); plans versus metered keys (link each vendor note's Access routes); OpenRouter (one key, a spend cap); nodeterm (live canvas; BUSL license); cc-switch (usage and quota view; keep routing, failover, and aggregation off, keep **provider management off for Codex** because it migrates Codex history and writes Codex profiles, and keep prompt, MCP, and skills sync off for any file esc or Balancewheel manages); nono and `sandbox-exec` (OS-level containment; the three verified runs from the spec); ACP, described per runtime — `kimi acp` is native, Codex and pi reach ACP through adapters (`codex-acp`, `pi-acp`), Claude Code through its adapter, agy as listed in the ACP registry — so a reader knows an adapter is an extra install. No personal paths.
- [ ] **Step 2: Run** `go test ./...` and `gofmt -l .` — expect PASS and no output.
- [ ] **Step 3: Commit** `docs: agent tools page — clis, plans versus metered, canvas, usage, sandboxes`.
