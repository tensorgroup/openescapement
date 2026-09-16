# Pack Authoring Style Guide + Source Sweep Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the four documents the 2026-08-08 design committed to (pack authoring style guide, two-tier source sweep in the tracking doc, a `/sweep-sources` repo skill, a pack-lint idea file), audit all five example packs against the new checklist, and run the sweep once end to end.

**Architecture:** Docs and one Claude Code skill, no Go changes. The tracking doc stays the single source of truth for the source list and the log; the skill reads it and never duplicates it. Example pack edits ripple into `examples/governed-service/` rendered artifacts and its lockfile, which are regenerated together and guarded by an existing test.

**Tech Stack:** Markdown, a Claude Code skill (`SKILL.md` with frontmatter), `go run ./cmd/esc` for regeneration, `go test ./internal/cli` for the examples consistency guard.

**Spec:** `docs/superpowers/specs/2026-08-08-pack-style-and-source-sweep-design.md`

## Amendments to the spec, decided 2026-09-16

The spec is five weeks old. Three things changed underneath it; the plan absorbs them rather than reopening the design.

1. **Five example packs, not two.** The audit covers `acme-org`, `anthropic-models`, `openai-models`, `zai-models`, and `model-seats`.
2. **The model registry exists.** `internal/guidance/models/` (a `models.yaml` plus per-vendor notes), and the portal page that renders it (`internal/portal/web/models.go`), shipped after the spec. The sweep's impact step assesses the registry alongside packs and renderer targets, and the monthly deep check covers the vendors the registry and the model packs track. DeepSeek (in the registry) and Z.ai (has an example pack) join the source list as non-render-target rows.
3. **The deep check is overdue.** The last full vendor sweep was 2026-08-11. The spec's "run once end to end" verification step is a real month's work; it is its own task, run in the main session, not a smoke test.

Also: nothing under `.claude/` is tracked today and it is not gitignored. The skill becomes the first tracked file there, so `.claude/worktrees/` is ignored in the same change to keep an accidental `git add .claude` from committing a worktree.

## Global Constraints

- No `esc` CLI or renderer changes. No lint implementation. No CI job or GitHub Action.
- Single external dependency policy is untouched. The only Go edit is one string in `internal/cli/examples_test.go` (Task 2, Step 16): an existing test pins the acme-org pack version that the audit bumps.
- New docs follow the style guide they introduce: imperative voice, one instruction per sentence, no ALL-CAPS emphasis. New prose in this plan's docs uses no em-dashes; the tracking doc's existing `- **date** — ` entry separator is kept as is.
- The skill never duplicates the source list; it reads `docs/roadmap/vendor-guidance-tracking.md`.
- Community signal never triggers action without primary-source verification.
- Substantive changes the sweep finds still go through spec → design-doc; the skill never patches the renderer ad hoc.
- No repo-tracked file references the maintainer's private review tooling.
- `gofmt -w .` and `go vet ./...` are effectively no-ops here, but `go test ./...` must pass before the branch is done, because Task 2 changes rendered example artifacts and one test expectation.
- Never commit unless the step says commit. Never `--no-verify`. No AI co-authorship trailers.

## File structure

| Path | Responsibility | Task |
|---|---|---|
| `docs/pack-authoring.md` | The committed rule for writing pack rule files, with checklist | 1 |
| `AGENTS.md` | Deeper-context pointers to the style guide and the two-tier sweep | 1, 3 |
| `examples/README.md` | Pointer to the style guide for pack authors | 1 |
| `examples/packs/*/rules/*.md`, `examples/packs/acme-org/skills/acme-vault/SKILL.md`, `examples/packs/*/pack.yaml` | Audited rule text, version bumps | 2 |
| `examples/governed-service/**` | Regenerated rendered artifacts and lockfile | 2 |
| `internal/cli/examples_test.go` | The pinned acme-org version string | 2 |
| `docs/roadmap/vendor-guidance-tracking.md` | Two-tier practice, community sources, DeepSeek and Z.ai rows, log discipline | 3 |
| `.claude/skills/sweep-sources/SKILL.md` | The do-it-now button | 4 |
| `.gitignore` | Ignore `.claude/worktrees/` | 4 |
| `docs/ideas/pack-lint.md`, `docs/ideas/README.md` | Lint idea and its table row | 5 |
| `docs/roadmap/vendor-guidance-tracking.md` (log) | First tagged entries from the end-to-end run | 6 |
| `CHANGELOG.md` | Unreleased entries for the packs, the skill, and the docs | 7 |

---

### Task 1: The style guide and its cross-links

**Files:**
- Create: `docs/pack-authoring.md`
- Modify: `AGENTS.md` (the bullets under "Deeper context, read on demand", currently lines 36 to 42)
- Modify: `examples/README.md` (add one paragraph at the end of the file, after the "Maintenance note" paragraph)

**Interfaces:**
- Produces: the checklist at the end of `docs/pack-authoring.md`, which Task 2 applies verbatim and Task 4's skill names as an impact target.

- [ ] **Step 1: Write `docs/pack-authoring.md`**

Create the file with exactly this content:

````markdown
# Writing pack rules

A rule file is a markdown file under `rules/` in a pack. The renderer places it inside a managed block in every instruction file the pack targets (`CLAUDE.md`, `AGENTS.md`, `GEMINI.md`), in every repo that syncs the pack, and an agent loads that block into every session. One wasted line in a rule file is wasted in every session in every repo. Write accordingly.

This guide is the committed rule for every rule file, skill `SKILL.md`, and catalog note in a pack. It applies to this repository's own `AGENTS.md` as well; that file predates the guide and is brought into line in its own pass, not silently.

## Say it so a model acts on it

Four principles borrowed from Simplified Technical English (ASD-STE100). They transfer because they remove the ambiguity a model would otherwise resolve by guessing.

**Imperative, active voice.** Tell the agent what to do.

- Before: `It is recommended that secrets be fetched from the vault.`
- After: `Fetch secrets from the vault.`

**One term per concept.** A pack picks its names and keeps them, because a model treats two names as two things.

- Before: `Request access in #platform-team.` then later `requires review by the platform team`
- After: `#platform-team` in both places.

**One instruction per sentence.** A sentence that carries two instructions gets half-followed.

- Before: `Authorization checks belong at the API layer; use the central policy service where available.`
- After: `Put authorization checks at the API layer. Use the central policy service where it is available.`

**Obvious referents.** When `it` or `this` could point at two things, repeat the noun.

- Before: `Runs are limited to 50 turns; if a task exceeds this, stop.`
- After: `Stop an orchestration run at 50 turns per goal.`

## Say why

State the reason for any rule an agent might be tempted to work around. A model generalizes a rule far better with its rationale, and a rule with no reason reads as a preference to be traded off.

- Before: `Do not use --no-verify.`
- After: `Do not use --no-verify. A red gate is a finding to fix, not an obstacle to route around.`

The reason belongs in the same bullet as the rule, in one sentence. A rule that needs a paragraph of reasoning needs a linked doc instead.

## Earn every line

**Line budget.** Every line changes agent behavior, or it is cut. Ask of each line: if this were deleted, what would an agent do differently? If the answer is nothing, delete it.

**Progressive disclosure.** A rule says what to do and links where the detail lives. The agent reads the detail when the task needs it, not in every session.

**No emphasis.** No ALL-CAPS words, no repeated emphasis, no bold on whole sentences. Current vendor guidance (see `docs/roadmap/vendor-guidance-tracking.md`, 2026-07-28 entry) is explicit that the current model generation wants no repeated or ALL-CAPS emphasis. Bold is for a defined term on first use or a short lead-in label on a bullet or paragraph, nothing else.

**Nothing the repo already shows.** An agent can read the build file, the test layout, and the directory tree. A rule file records what the agent cannot discover: constraints, policy, the reason behind an unusual choice.

## What this guide rejects

Full ASD-STE100 is rejected, and recorded here so it is not relitigated. STE's controlled dictionary (about 900 approved words, one approved meaning each) and its sentence-length caps solve a problem models do not have: non-native human readers and translation. The dictionary costs precision on terms of art, and STE's procedural style strips rationale, which is among the highest-value content in an instruction file. Take the four principles above; leave the rest.

## Checklist

Run this over every rule file, skill, and catalog note before publishing a pack version.

- [ ] Every rule is imperative and active.
- [ ] One term per concept, used consistently across the whole pack.
- [ ] One instruction per sentence.
- [ ] Every `it`, `this`, `these`, `they` has one obvious referent.
- [ ] Every rule an agent might work around states its reason.
- [ ] Every line would change agent behavior if kept and would not if cut.
- [ ] Detail lives in a linked doc, not inline.
- [ ] No ALL-CAPS emphasis, no repeated emphasis, no bold sentences.
- [ ] Nothing restates what the repo itself shows.

The first, third, and eighth items are mechanically checkable and are candidates for a future `esc pack lint` (`docs/ideas/pack-lint.md`). The rest are review.
````

- [ ] **Step 2: Cross-link from `AGENTS.md`**

In the "Deeper context, read on demand" list, add this bullet directly after the `docs/cli-and-portal.md` bullet:

```markdown
- `docs/pack-authoring.md` — the committed style rule for pack rule files, skills, and catalog notes; it applies to this file too, which is brought into line in its own pass
```

- [ ] **Step 3: Cross-link from `examples/README.md`**

Add this line after the "Maintenance note" paragraph at the end of the file:

```markdown
Writing your own pack: the rule files in these examples follow `docs/pack-authoring.md`. Run its checklist before you publish a version.
```

- [ ] **Step 4: Verify the links resolve**

Run:

```sh
test -f docs/pack-authoring.md && grep -c 'docs/pack-authoring.md' AGENTS.md examples/README.md && grep -c 'docs/ideas/pack-lint.md' docs/pack-authoring.md && ! grep -n '—' docs/pack-authoring.md
```

Expected: `docs/pack-authoring.md` exists; `AGENTS.md:1`, `examples/README.md:1`, the lint pointer count is `1`, and the em-dash grep prints nothing (the `!` makes the whole line exit 0 only when no em-dash is present). `docs/ideas/pack-lint.md` does not exist yet; Task 5 creates it.

- [ ] **Step 5: Commit**

```sh
git add docs/pack-authoring.md AGENTS.md examples/README.md
git commit -m "docs: committed style rule for writing pack rules

Four STE principles that transfer to instruction files (imperative, one
term per concept, one instruction per sentence, no ambiguous pronouns),
plus state-the-why, a line budget, and no emphasis. Full ASD-STE100 is
rejected on the record so it is not relitigated."
```

---

### Task 2: Audit the example packs against the checklist

**Files:**
- Modify: `examples/packs/acme-org/rules/secrets.md`
- Modify: `examples/packs/acme-org/rules/authn.md`
- Modify: `examples/packs/acme-org/rules/hosting.md`
- Modify: `examples/packs/acme-org/rules/sdlc.md`
- Modify: `examples/packs/acme-org/rules/agent-behavior.md`
- Modify: `examples/packs/acme-org/rules/paved-path.md`
- Modify: `examples/packs/acme-org/skills/acme-vault/SKILL.md`
- Modify: `examples/packs/anthropic-models/rules/model-selection.md`
- Modify: `examples/packs/openai-models/rules/model-selection.md`
- Modify: `examples/packs/zai-models/rules/model-selection.md`
- Modify: `examples/packs/model-seats/rules/seat-policy.md`
- Modify: `examples/packs/model-seats/rules/panel-protocol.md`
- Modify: `examples/packs/*/pack.yaml` (version bump, all five)
- Regenerate: `examples/governed-service/**` (via `esc sync`)
- Modify: `internal/cli/examples_test.go:33` (the pinned `acme-org@0.1.0` string)
- Test: `internal/cli/examples_test.go` (`TestExamplesSync`) and `internal/cli/examples_consistency_test.go` (`TestExamplesLockMatchesShippedFiles`), both existing

**Interfaces:**
- Consumes: the checklist in `docs/pack-authoring.md` (Task 1).
- Produces: nothing downstream; Task 6's sweep reads the packs as they stand after this task.

Each edit below is one checklist finding. Replace the "before" text with the "after" text exactly. Where a file is not listed, the audit found nothing to change. Tables, URLs, prices, and model IDs are not touched: this is a style audit, not a content refresh. Catalog notes in the five `pack.yaml` files were audited while planning: none carries ALL-CAPS or repeated emphasis, and the sentence fragments in acme-org's notes are catalog labels, not rules, so no note changes.

One term is settled for acme-org: the reviewer is `#platform-team`, and a rule that needs its review says `review by #platform-team`. The catalog's `Request review in #platform-team` is the same instruction from the requester's side and stays.

- [ ] **Step 1: `acme-org/rules/secrets.md`**

Before (line 4):
```
- Secrets live in the org vault: https://vault.acme.example — request access in #platform-team.
```
After:
```
- Keep secrets in the org vault: https://vault.acme.example. Request access in #platform-team.
```

Before (line 5):
```
- API keys for AI providers are issued through https://keys.acme.example (do not create personal provider accounts for org work).
```
After:
```
- Get API keys for AI providers from https://keys.acme.example. Do not create personal provider accounts for org work: a centrally issued key can be rotated and revoked, a personal one cannot.
```

Before (line 6):
```
- If you (an agent) encounter a hardcoded secret, stop and flag it for the developer instead of copying or moving it.
```
After:
```
- If you find a hardcoded secret, stop and flag it to the developer. Do not copy or move it: moving a leaked secret spreads it without rotating it.
```

- [ ] **Step 2: `acme-org/rules/authn.md`**

Before (line 4):
```
- New services must use the approved OIDC flow with the org identity provider; libraries: `acme-auth-go`, `acme-auth-ts`.
```
After:
```
- Use the approved OIDC flow with the org identity provider for every new service, through `acme-auth-go` or `acme-auth-ts`.
```

Before (line 5):
```
- Authorization checks belong at the API layer; use the central policy service where available.
```
After:
```
- Put authorization checks at the API layer. Use the central policy service where it is available.
```

- [ ] **Step 3: `acme-org/rules/hosting.md`**

Before (line 3):
```
- Production workloads run on the paved-road platform (https://platform.acme.example). POCs may use the allowed hosted platforms in the catalog below.
```
After:
```
- Run production workloads on the paved-road platform (https://platform.acme.example). For a POC, use one of the allowed hosted platforms in the catalog below.
```

Before (line 4):
```
- Sharing a locally-hosted service: use the org tailnet (Tailscale — preferred) or Headscale for lab clusters. Cloudflare Tunnel requires review by #platform-team.
```
After:
```
- To share a locally hosted service, use the org tailnet (Tailscale). Use Headscale only for lab clusters. Cloudflare Tunnel requires review by #platform-team.
```

Before (line 6):
```
- Any newly opened port on a deployed service requires review by the platform team before it ships.
```
After:
```
- Any newly opened port on a deployed service requires review by #platform-team before it ships.
```

- [ ] **Step 4: `acme-org/rules/sdlc.md`**

Before (line 3):
```
- All code — including POCs and vibe-coded experiments — lives in a tracked repository under the org's GitHub organization.
```
After:
```
- Keep all code, including POCs and vibe-coded experiments, in a tracked repository under the org's GitHub organization.
```

Before (line 4):
```
- CI runs on every PR and must include: tests, dependency scanning, and SAST. Use the shared workflow templates at https://github.com/acme/workflows.
```
After:
```
- Run CI on every PR with tests, dependency scanning, and SAST. Use the shared workflow templates at https://github.com/acme/workflows.
```

Before (line 5):
```
- Do not disable, skip, or `--no-verify` past commit hooks or CI gates.
```
After:
```
- Do not disable, skip, or `--no-verify` past commit hooks or CI gates. A red gate is a finding to fix, not an obstacle to route around.
```

- [ ] **Step 5: `acme-org/rules/agent-behavior.md`**

Before (line 3):
```
- Orchestration runs are limited to 50 turns per goal; if a task exceeds this, stop and summarize progress for a human instead of continuing.
```
After:
```
- Stop an orchestration run at 50 turns per goal. Leave a progress summary for a human, because the cap exists so a run that is not converging is caught by a person, not by a budget alarm.
```

Before (line 4):
```
- Model routing: use fast models for mechanical edits and code generation subagents; reserve reasoning models for design, review, and security-sensitive decisions.
```
After:
```
- Use fast models for mechanical edits and code-generation subagents. Reserve reasoning models for design, review, and security-sensitive decisions.
```

- [ ] **Step 6: `acme-org/rules/paved-path.md`**

Before (line 3):
```
Before building infrastructure, check these. They exist, they're maintained, and using them is always acceptable:
```
After:
```
Before building infrastructure, check this table. These services exist, are maintained, and are always acceptable to use:
```

- [ ] **Step 7: `acme-org/skills/acme-vault/SKILL.md`**

Replace the numbered list (lines 8 to 12) with:

```
1. Reference secrets by path, for example `acme/team-name/service-name/API_KEY`.
2. In code, use the vault SDK (`acme-vault-go`, `acme-vault-ts`). Never fetch secrets over raw HTTP.
3. For local development, run `acme-vault login` once. The SDK picks up the session.
4. In CI, the shared workflow templates inject `VAULT_ADDR` and `VAULT_ROLE` automatically.
5. Never write a fetched secret to a file, log, or agent transcript.
```

- [ ] **Step 8: `anthropic-models/rules/model-selection.md`**

Before (line 3):
```
Pick the model by the task, not by habit. Start at the top for anything that needs judgment, step down for volume and speed, and treat every departure from the vendor's own default as a decision with a reason and a reversal condition.
```
After:
```
Pick the model by the task, not by habit. Start at the top for anything that needs judgment. Step down for volume and speed. Treat every departure from the vendor's own default as a decision with a reason and a reversal condition.
```

Before (the end of the "Opus 5 is review-required" bullet):
```
Meet it, record it, and flip the status.
```
After:
```
When the condition is met, record the evidence and flip the status.
```

- [ ] **Step 9: `openai-models/rules/model-selection.md`**

Before (the "Sol is not retired" bullet):
```
- Sol is not retired. Keep it where it already reviews well, and move planning and design work to Astra as sessions roll over.
```
After:
```
- Sol is not retired. Keep it where it already reviews well. Move planning and design work to Astra as sessions roll over.
```

- [ ] **Step 10: `zai-models/rules/model-selection.md`**

Before (the "Never name a preview id" bullet, first sentence only; keep the rest of the bullet as is):
```
- Never name a preview id in a wrapper, a schema, or an interface; pin it in one line of config.
```
After:
```
- Never name a preview id in a wrapper, a schema, or an interface. Pin it in one line of config.
```

- [ ] **Step 11: `model-seats/rules/seat-policy.md`**

Before (line 6, the italics):
```
Seats are granted and revoked from *logged evidence*, dispute win shares and
```
After:
```
Seats are granted and revoked from logged evidence, dispute win shares and
```

- [ ] **Step 12: `model-seats/rules/panel-protocol.md`**

Before (item 2, first line):
```
2. **Independence first.** Seats produce round-1 output in parallel WITHOUT seeing
```
After:
```
2. **Independence first.** Seats produce round-1 output in parallel without seeing
```

Before (item 3, first line):
```
3. **Targeted cross-examination.** Relay only the *specific disputed claim* to the
```
After:
```
3. **Targeted cross-examination.** Relay only the specific disputed claim to the
```

Replace item 5 in full (from `5. **Mandatory outcome logging.**` to the end of the file) with:

```
5. **Mandatory outcome logging.** Append a structured record per panel: seats, rounds,
   immediate-agreement flag, and each dispute (one-line summary, challenger, each seat's
   proposal, winner, reason). Record the findings themselves per seat (claim, reference,
   the moderator's confirmed/refuted/partial verdict, evidence checked, action taken),
   grouped across seats, so that a finding only one seat raised is derivable as that
   seat's unique catch rather than a count. An unlogged panel does not count toward any
   seat's record.
```

- [ ] **Step 13: Bump every pack version**

Rule text changed in all five packs, and a pack version is what tells a consumer's `esc status` that policy moved.

| File | Before | After |
|---|---|---|
| `examples/packs/acme-org/pack.yaml` | `version: 0.1.0` | `version: 0.1.1` |
| `examples/packs/anthropic-models/pack.yaml` | `version: 0.2.0` | `version: 0.2.1` |
| `examples/packs/openai-models/pack.yaml` | `version: 0.1.0` | `version: 0.1.1` |
| `examples/packs/zai-models/pack.yaml` | `version: 0.1.0` | `version: 0.1.1` |
| `examples/packs/model-seats/pack.yaml` | `version: 0.1.0` | `version: 0.1.1` |

- [ ] **Step 14: Run the mechanical checks**

```sh
grep -rnoE '\b[A-Z]{4,}\b' examples/packs/*/rules/*.md examples/packs/*/skills/*/SKILL.md | grep -vE ':(HTTP|SDLC|OIDC|SAST|AGENTS)$'
grep -rn '—' examples/packs/*/rules/*.md examples/packs/*/skills/*/SKILL.md
```

Expected: the first grep prints nothing. Before the edits its only non-acronym hit is `WITHOUT` in `panel-protocol.md`, which Step 12 removes; the allowlist names exactly the acronyms that remain. The second grep may still print em-dashes in tables, headings, and frontmatter that the audit left alone; that is acceptable, the guide bans emphasis, not punctuation. The audit edits above remove every em-dash that joined two instructions.

- [ ] **Step 15: Regenerate `examples/governed-service/`**

The acme-org rule and skill changes alter the rendered artifacts and every hash in the lockfile. Regenerate them together:

```sh
(cd examples/governed-service && go run ../../cmd/esc sync)
(cd examples/governed-service && go run ../../cmd/esc status --check); echo "status exit=$?"
git status --short examples/governed-service
```

Expected: `sync` exits 0. A stderr line about a missing `ESC_PORTAL_TOKEN` is normal (telemetry is unconfigured, not failing). `status --check` prints `status exit=0`: that is the proof the regeneration converged, since the tests in Step 17 compare the lock to the shipped files and would also pass if this step were skipped. `git status` lists `CLAUDE.md`, `AGENTS.md`, `GEMINI.md`, `GOVERNANCE.md`, `.claude/skills/esc-acme-org-acme-vault/SKILL.md`, and `.escapement/escapement.lock` as modified, nothing else (`.mcp.json` and `.escapement/config.yaml` are untouched by rule edits).

- [ ] **Step 16: Update the pinned version in `TestExamplesSync`**

`internal/cli/examples_test.go:33` asserts the rendered example `CLAUDE.md` contains `escapement:begin packs=acme-org@0.1.0`. Change that one string:

Before:
```go
	for _, want := range []string{"pnpm install", "vault.acme.example", "Tailscale", "escapement:begin packs=acme-org@0.1.0"} {
```
After:
```go
	for _, want := range []string{"pnpm install", "vault.acme.example", "Tailscale", "escapement:begin packs=acme-org@0.1.1"} {
```

The other three strings still appear in the audited rules (`Tailscale` in hosting.md, `vault.acme.example` in secrets.md, `pnpm install` in the catalog), so nothing else in the test moves.

- [ ] **Step 17: Run the two example tests and the full suite**

```sh
go test ./internal/cli -run 'TestExamplesSync|TestExamplesLockMatchesShippedFiles'
go test ./...
```

Expected: both pass. If `TestExamplesSync` fails on the version string, Step 16 was skipped. If `TestExamplesLockMatchesShippedFiles` fails, a pack or governed-service file was edited after the sync in Step 15; rerun Step 15.

- [ ] **Step 18: Commit**

```sh
git add examples/packs examples/governed-service internal/cli/examples_test.go
git commit -m "examples: audit every pack against the pack-authoring checklist

One instruction per sentence, one name per concept (#platform-team),
a stated reason on the rules an agent would route around (--no-verify,
the turn cap, personal API keys), and no ALL-CAPS or italic emphasis.
Every pack version bumps; governed-service is regenerated to match, and
TestExamplesSync's pinned acme-org version moves with it."
```

---

### Task 3: Two-tier practice in the tracking doc

**Files:**
- Modify: `docs/roadmap/vendor-guidance-tracking.md` (the `## Practice` section, lines 10 to 28, and the `## Sources` section, lines 30 to 40)
- Modify: `AGENTS.md` (the `docs/roadmap/vendor-guidance-tracking.md` bullet in "Deeper context")

**Interfaces:**
- Produces: the log-entry tag format `[weekly]` / `[deep]` and the Sources subsections `Primary sources per vendor` and `Community signal (weekly tier)`, which Task 4's skill parses by heading text. Do not rename either heading without updating the skill.

- [ ] **Step 1: Replace the first paragraph of `## Practice`**

Before (line 12):
```
Roughly quarterly, and at every major model-generation release, check for updated context/prompting guidance and new or changed instruction-file conventions.
```
After:
```
Two tiers, both run by the `/sweep-sources` skill in this repo (`.claude/skills/sweep-sources/SKILL.md`). With no argument the skill runs whatever is due; `weekly`, `deep`, or `all` forces a tier.

- **Weekly, community signal.** Scan the community sources listed under Sources for instruction-file convention changes (new files, renames, hierarchy or size-cap changes), context and prompting guidance changes, new agent targets worth rendering to, and major model-generation releases. Ignore hype, benchmarks, and product drama. Community sources are signal only: every candidate claim is verified against the vendor's primary source before it enters this log or drives any change. Secondary coverage has already been wrong twice (a stale Grok size cap and a wrong AAIF date, both caught in the 2026-07-30 sweep).
- **Monthly, deep vendor check.** Read the primary sources for every vendor below, and again at every major model-generation release. Quarterly was judged too slow for the current pace of change (decision 2026-08-08).
```

- [ ] **Step 2: Add the non-render-target vendors next to Copilot, not in the core set**

The core vendor set promises a top-level row in the portal's guidance-freshness view, so it is not the place for vendors that are not render targets. The existing "Also watched, not a render target" paragraph (about Copilot) is. Directly after that paragraph, add a new paragraph:

```
**Also tracked for the model registry, not a render target:** DeepSeek and Z.ai. The model registry (`internal/guidance/models/`) and the example model packs (`examples/packs/*-models/`) carry their lineups and prices, so the deep check reads their primary sources for model releases, retirements, and price changes only.
```

- [ ] **Step 2b: Put the log in newest-first order**

The `## Log` section is oldest-first from 2026-07-28 through 2026-07-31, then the 2026-09-16 entry sits above the 2026-08-11 entry. Reorder the entries (each is one `- **date** — ...` bullet plus, for 2026-08-11, its three indented sub-bullets) so the section reads 2026-09-16, 2026-08-11, 2026-07-31, 2026-07-30 (four entries, keep their existing relative order), 2026-07-28. Move whole entries; change no text. Verify:

```sh
grep -oE '^- \*\*2026-[0-9-]+' docs/roadmap/vendor-guidance-tracking.md | tr '\n' ' '
```

Expected: `- **2026-09-16 - **2026-08-11 - **2026-07-31 - **2026-07-30 - **2026-07-30 - **2026-07-30 - **2026-07-30 - **2026-07-28`.

- [ ] **Step 3: Replace the log-discipline paragraph**

Before (the paragraph starting `Record each check in the log below`):
```
Record each check in the log below (date, source, what changed, action taken). Check the primary sources in the Sources section first; secondary coverage has already been wrong twice (a stale Grok size cap, a wrong AAIF date — both caught in the 2026-07-30 sweep). If a change affects render targets or renderer defaults, it goes through the normal spec → design-doc cycle rather than being patched ad hoc.
```
After:
```
Record each sweep in the log below, newest first, tagged with its tier: `- **YYYY-MM-DD** — [weekly] ...` or `- **YYYY-MM-DD** — [deep] ...`, then source, what changed, and action taken. A sweep that finds nothing still gets a one-line `no signal` entry, because the skill computes what is due from the newest tagged entry per tier (a `[deep]` entry also satisfies the weekly tier). Entries without a tag predate the tiers and do not count toward a due date. Every sweep ends with a verdict line, `packs need updating: yes` or `no`, with reasons. If a change affects render targets, renderer defaults, or the model registry, it goes through the normal spec → design-doc cycle rather than being patched ad hoc.
```

- [ ] **Step 4: Restructure `## Sources`**

Change the line `Primary sources per vendor:` to a subsection heading `### Primary sources per vendor` and leave the existing bullets under it. Add these two bullets at the end of that list:

```
- **DeepSeek** — API news and model releases: <https://api-docs.deepseek.com/news>
- **Z.ai / GLM** — developer docs and model pages: <https://docs.z.ai/>
```

Then add a second subsection after the primary-source bullets:

```
### Community signal (weekly tier)

Signal only; nothing enters the log from these without primary-source verification. Subreddit names are themselves a moving target: when one renames or dies, the sweep corrects this list.

- Anthropic: r/Anthropic, r/ClaudeAI, r/ClaudeCode
- OpenAI: r/OpenAI, r/ChatGPTCoding
- Google: r/Bard, r/GeminiAI
- xAI: r/grok
- DeepSeek: r/DeepSeek
- Cross-vendor: r/LocalLLaMA (also covers Moonshot/Kimi and Z.ai, neither of which has an active English subreddit), Hacker News via the Algolia search API (<https://hn.algolia.com/api/v1/search_by_date>)
```

- [ ] **Step 5: Update the `AGENTS.md` pointer**

Before:
```
- `docs/roadmap/vendor-guidance-tracking.md` — cadence for checking model vendors' instruction-file and context guidance (the format of this very file is a moving target)
```
After:
```
- `docs/roadmap/vendor-guidance-tracking.md` — the two-tier sweep (weekly community signal, monthly deep vendor check) over model vendors' instruction-file and context guidance, run by `/sweep-sources`; the format of this very file is a moving target
```

- [ ] **Step 6: Verify structure**

```sh
grep -c '^### Primary sources per vendor$' docs/roadmap/vendor-guidance-tracking.md
grep -c '^### Community signal (weekly tier)$' docs/roadmap/vendor-guidance-tracking.md
grep -ci 'quarterly' docs/roadmap/vendor-guidance-tracking.md
grep -c 'sweep-sources' AGENTS.md
```

Expected: `1`, `1`, `1` (the single remaining mention, case-insensitive, is the decision record "Quarterly was judged too slow"), `1`.

- [ ] **Step 7: Commit**

```sh
git add docs/roadmap/vendor-guidance-tracking.md AGENTS.md
git commit -m "docs: two-tier source sweep, weekly community signal and monthly deep check

Quarterly was too slow for the current pace of change. Community sources
are signal only and never enter the log unverified. Log entries carry a
tier tag so the skill can compute what is due. DeepSeek and Z.ai join the
source list for the model registry and model packs, not as render targets."
```

---

### Task 4: The `/sweep-sources` skill

**Files:**
- Create: `.claude/skills/sweep-sources/SKILL.md`
- Modify: `.gitignore` (add `.claude/worktrees/`)

**Interfaces:**
- Consumes: the tag format and Sources headings from Task 3, the checklist from Task 1.
- Produces: log entries in the format Task 3 defined; the verdict line Task 6 reports.

- [ ] **Step 1: Ignore the worktree directory**

Append to `.gitignore`:

```
# Claude Code worktrees (local isolation, never committed)
.claude/worktrees/
```

- [ ] **Step 2: Write the skill**

Create `.claude/skills/sweep-sources/SKILL.md` with exactly this content:

````markdown
---
name: sweep-sources
description: Sweep the sources that shape instruction-file conventions and model guidance, verify every candidate against a primary source, log the result in docs/roadmap/vendor-guidance-tracking.md, and end with a packs-need-updating verdict. Use when asked to sweep sources, check vendor guidance, or when a weekly or monthly sweep is due. Takes weekly, deep, or all as its argument; with none it runs what is due.
---

# Sweep sources

The tracking doc `docs/roadmap/vendor-guidance-tracking.md` owns the source list, the cadence, and the log. This skill reads that file and never repeats its contents. When the two disagree, the tracking doc wins: follow it, and report the mismatch in your output so the skill gets fixed in its own change.

## 1. Decide which tier to run

Argument `$ARGUMENTS` is one of `weekly`, `deep`, `all`, or empty.

- `weekly`: the community-signal scan.
- `deep`: the primary-source vendor check. A deep run includes the weekly scan.
- `all`: same as `deep`.
- empty: compute what is due from the log.

Due-date rule. Take today's date from `date +%F`. Read the `## Log` section. Find the newest entry tagged `[weekly]` or `[deep]`; if it is 7 or more days old, or there is none, the weekly tier is due. Find the newest entry tagged `[deep]`; if it is 30 or more days old, or there is none, the deep tier is due. Entries with no tag are historical and do not count. If the deep tier is due, run `deep`. Else if the weekly tier is due, run `weekly`. Else say which dates were found, state that nothing is due, and stop.

Print one line before sweeping: the tier, today's date, and the last-run date per tier.

## 2. Sweep

Read the source lists from the tracking doc's `## Sources` section: `### Primary sources per vendor` and `### Community signal (weekly tier)`.

Filter every source to what this product cares about, and nothing else:

- instruction-file conventions: new files, renames, hierarchy changes, size-cap changes, import syntax
- context and prompting guidance changes from a vendor
- a new coding agent worth rendering to
- a major model-generation release, or a model retirement, that changes the model registry or a model pack

Ignore hype, benchmarks, pricing rumors, and product drama.

Weekly scan, per community source:

- Subreddits: fetch `https://old.reddit.com/r/<name>/top/?t=week`. If the fetch is blocked, use web search restricted to `site:reddit.com/r/<name>` for the past week. A subreddit that returns "not found" or "banned" is a source-list correction, not a finding.
- Hacker News: query `https://hn.algolia.com/api/v1/search_by_date?tags=story&numericFilters=created_at_i>EPOCH&query=TERM`, where `EPOCH` is the Unix time seven days ago (`date -v-7d +%s` on macOS, `date -d '7 days ago' +%s` on Linux), once per term: `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `context engineering`, `Claude Code`, `Codex CLI`, `Gemini CLI`, `Kimi Code`, `Grok Build`.

Deep check, per primary source: fetch each URL listed under `### Primary sources per vendor` and read for changes since the newest `[deep]` entry (or since the newest entry for that vendor when there is no `[deep]` entry yet). For vendors whose URL is a docs index, follow the instruction-file or customization page one level down.

Collect candidates as: source, claim, URL, date seen.

## 3. Verify

For every candidate from a community source, find the primary source that supports it and fetch it. Discard any candidate the primary source does not support, and say so in the log entry when the community claim was wrong (a wrong claim is itself useful signal about a source). Never log a claim on community evidence alone.

Note corrections to the source list: a dead or renamed subreddit, a moved docs URL. Apply the correction to the tracking doc's Sources section in the same edit as the log entry.

## 4. Log and assess

Append one entry at the top of `## Log` in this shape, newest first:

```
- **YYYY-MM-DD** — [weekly|deep] <source(s)>. <What changed, with the primary-source URL.> Action: <what was done, or "none">.
```

With nothing to report, tag the tier that actually ran, so its last-run date still advances:

```
- **YYYY-MM-DD** — [weekly] no signal.
- **YYYY-MM-DD** — [deep] no signal.
```

Then assess impact against, in order:

1. the example packs under `examples/packs/` (rule text, catalogs, prices, model IDs)
2. `docs/pack-authoring.md` (does new vendor guidance change the style rule?)
3. renderer targets and defaults (`internal/targets/`, `internal/render/`)
4. the model registry (`internal/guidance/models/models.yaml` and the per-vendor notes beside it)

End the entry, and the session's output, with the verdict line:

```
packs need updating: yes|no. <reasons>
```

## Boundaries

- This skill edits exactly one file: the tracking doc (log entry, source-list corrections). It never patches a pack, the renderer, or the registry. A `yes` verdict is a request for the normal spec → design-doc cycle.
- A finding that changes render targets, renderer defaults, or the model registry is out of scope for this skill's edit and is flagged in the verdict for that cycle.
- Commit nothing. Leave the tracking-doc change in the working tree for review.
````

- [ ] **Step 3: Verify the skill is discoverable and well-formed**

```sh
head -3 .claude/skills/sweep-sources/SKILL.md
git check-ignore -q .claude/worktrees/x && echo ignored
git check-ignore -q .claude/skills/sweep-sources/SKILL.md || echo tracked
grep -c '### Primary sources per vendor' .claude/skills/sweep-sources/SKILL.md
grep -c '### Community signal (weekly tier)' .claude/skills/sweep-sources/SKILL.md
sed -n '2,3p' .claude/skills/sweep-sources/SKILL.md | ruby -ryaml -e 'YAML.safe_load(STDIN.read); puts "frontmatter ok"'
```

Expected: the frontmatter opens with `---` and `name: sweep-sources`; `ignored`; `tracked`; both heading greps print a count of 1 or more (the skill names the exact headings it parses, matching Task 3); `frontmatter ok`. The YAML parse matters: an unquoted `: ` inside the description makes the frontmatter invalid, and Claude Code then loses the description that drives skill discovery.

- [ ] **Step 4: Commit**

```sh
git add .gitignore .claude/skills/sweep-sources/SKILL.md
git commit -m "skills: /sweep-sources, the do-it-now button for the source sweep

Reads the tier cadence, source list, and last-run dates from the tracking
doc, runs whatever is due, verifies community signal against primary
sources, logs a tagged entry, and ends with a packs-need-updating verdict.
Edits only the tracking doc; anything substantive goes through a spec.
Ignore .claude/worktrees so tracking .claude/skills cannot sweep one in."
```

---

### Task 5: The pack-lint idea file

**Files:**
- Create: `docs/ideas/pack-lint.md`
- Modify: `docs/ideas/README.md` (the `## Open ideas` table)

**Interfaces:**
- Consumes: the checklist item numbering in `docs/pack-authoring.md` (Task 1).

- [ ] **Step 1: Write the idea file**

Create `docs/ideas/pack-lint.md` with exactly this content:

````markdown
# `esc pack lint`

**Status:** Raw idea, unscoped. No commitment to build.
**Touches:** spec §3 (Pillar A, the pack format and CLI); the `esc pack` author commands defined in `docs/superpowers/specs/2026-08-09-skill-distribution-design.md`.
**Boundary (§4):** an author-side check that runs in the pack repo, like `esc pack outdated`. It never runs during `sync` or `status`, never reads a governed repo, and never sends content anywhere. Style is the pack author's problem; a consumer cannot fix it and must not be nagged about it.

## The problem

`docs/pack-authoring.md` is the committed style rule for rule files. Three of its nine checklist items are mechanically checkable, and nothing checks them. A pack author who skips the checklist ships emphasis and run-on rules to every repo that syncs the pack, and the first review that catches it is a model misfollowing the rule.

## Mechanically checkable

- ALL-CAPS emphasis: a run of four or more capitals that is not in an allowlist of acronyms and identifiers (the pack could carry the allowlist in `pack.yaml`).
- Repeated emphasis: bold or italic density above a threshold per file, bold spanning a whole sentence.
- Line and byte budgets: per rule file, and for the rendered managed block per target, against the renderer's size limits.
- One instruction per sentence, by heuristic: a bullet containing a semicolon, or a bullet with two or more imperative verbs joined by `and` or `, then`.
- Term consistency: a per-pack glossary in `pack.yaml` (canonical term plus forbidden variants) checked across every rule file, skill, and catalog note.
- Dead links: every URL in a rule file resolves, or is a `.example` domain.

## Not mechanically checkable

These stay a review, and the lint's output must say so, so nobody mistakes a clean lint for a clean pack:

- an ambiguous pronoun
- a rule with no stated reason
- whether a line changes agent behavior at all
- whether a rule restates what the repo already shows

## Shape if built

`esc pack lint [--check]` next to `add-skill` and `outdated`, run in the pack repo. Findings print as `file:line: rule: message`. `--check` exits 1 on any finding, for CI on the pack repo, matching `outdated --check`. No new dependency: the checks above are string scanning and the existing pack loader.
````

- [ ] **Step 2: Add the table row**

In `docs/ideas/README.md`, after the existing agent-credential row, add:

```markdown
| [`esc pack lint`](pack-lint.md) | Raw idea, unscoped | §3 Pillar A; `esc pack` author commands |
```

- [ ] **Step 3: Verify**

```sh
test -f docs/ideas/pack-lint.md && grep -c 'pack-lint.md' docs/ideas/README.md docs/pack-authoring.md
```

Expected: `docs/ideas/README.md:1` and `docs/pack-authoring.md:1`. Every cross-link from the spec's verification section now resolves.

- [ ] **Step 4: Commit**

```sh
git add docs/ideas/pack-lint.md docs/ideas/README.md
git commit -m "docs(ideas): esc pack lint, the mechanically checkable third of the style rule

Author-side only, never in sync or status. Names what it could check and
what it never could, so a clean lint is not mistaken for a clean pack."
```

---

### Task 6: Run the sweep end to end, both tiers

This task runs in the main session, not in a subagent: it needs web search and fetch, and it makes judgment calls about what is signal. The deep tier is overdue (last full vendor sweep 2026-08-11), so expect real findings, not a smoke test.

**Files:**
- Modify: `docs/roadmap/vendor-guidance-tracking.md` (log, and Sources if a correction is found)

**Interfaces:**
- Consumes: the skill from Task 4, the impact targets from Tasks 1 and 2.
- Produces: the first `[deep]` log entry and a verdict, which decides whether a follow-up cycle opens.

- [ ] **Step 1: Invoke the skill**

Run `/sweep-sources all` in the session. `.claude/skills/` did not exist when this session started, and Claude Code discovers a new skills directory at startup, so if `/sweep-sources` is not offered, restart the session in this worktree and run it again. Confirm the first printed line names the tier `deep`, today's date, and "none" for both last-run dates (no tagged entries exist yet).

- [ ] **Step 2: Review the appended entry**

Check, before accepting it:

- The entry is at the top of `## Log`, tagged `[deep]`, dated today.
- Every claim in it cites a primary-source URL, none cites only a subreddit or HN thread.
- Any source-list correction was applied in the Sources section, not only mentioned.
- The entry ends with `packs need updating: yes` or `no` and reasons.
- The skill edited nothing outside the tracking doc: `git status --short` lists only that file.

- [ ] **Step 3: Commit**

```sh
git add docs/roadmap/vendor-guidance-tracking.md
git commit -m "docs: first two-tier source sweep

Deep check across every primary source and the community signal set; the
first tagged entry, which is what the skill's due-date arithmetic keys on."
```

Adjust the body to name the headline finding when the sweep found one.

- [ ] **Step 4: Route the verdict**

If the verdict is `yes`, do not patch anything in this branch. Record the reasons in the handoff message so a follow-up spec can open; the model packs and registry have their own refresh path (see the 2026-09-16 log entry as the pattern).

---

### Task 7: Changelog

**Files:**
- Modify: `CHANGELOG.md` (the `## [Unreleased]` section, which already has `### Added` and pack entries)

- [ ] **Step 1: Add the entries**

Under `## [Unreleased]`, add to `### Added` (after its existing bullets):

```markdown
- `docs/pack-authoring.md`: the committed style rule for pack rule files, skills,
  and catalog notes, with an author checklist. `docs/ideas/pack-lint.md` files the
  mechanically checkable third of it as a future `esc pack lint`.
- `.claude/skills/sweep-sources`: a repo skill that runs the two-tier source sweep
  (weekly community signal, monthly deep vendor check) and logs a tagged entry in
  `docs/roadmap/vendor-guidance-tracking.md`.
```

Add a `### Changed` heading if the section has none, and under it:

```markdown
- Example packs audited against `docs/pack-authoring.md`: one instruction per
  sentence, one name per concept, a stated reason on rules an agent would route
  around, no ALL-CAPS or italic emphasis. acme-org 0.1.1, anthropic-models 0.2.1,
  openai-models 0.1.1, zai-models 0.1.1, model-seats 0.1.1.
- Vendor guidance sweep cadence: weekly community scan plus a monthly deep check,
  replacing the quarterly check. DeepSeek and Z.ai join the source list for the
  model registry and model packs, not as render targets.
```

- [ ] **Step 2: Commit**

```sh
git add CHANGELOG.md
git commit -m "docs(changelog): pack style guide, audited packs, two-tier sweep skill"
```

---

## Handoff after Task 7

Three items for the maintainer, none executed by this plan:

1. **Optional weekly cron.** The spec offers a scheduled invocation so the cadence does not depend on memory. The natural shape is a scheduled cloud routine that runs `/sweep-sources` weekly and opens a PR with the tracking-doc change (the skill already leaves the change uncommitted for review). Set it up only on an explicit yes.
2. **Delete `CONTINUE.md`** from the main checkout once this branch merges. It is the pre-crash resume note for this cycle and is untracked.
3. **Bring `AGENTS.md` into line with the style guide** in its own change. The guide says it applies to that file, and today it does not comply (ALL-CAPS emphasis, bold sentences, multi-instruction sentences). That is a content edit to the repo's canonical instructions and deserves its own review, not a ride-along here.

Then `superpowers:finishing-a-development-branch`.

## Self-review against the spec

- Deliverable 1 (`docs/pack-authoring.md`, cross-links, audit with fixes applied): Tasks 1 and 2. Both cross-links named in the spec are present; the audit covers all five packs.
- Deliverable 2 (two tiers, community source set, verification rule promoted, monthly cadence, log discipline with no-signal entries): Task 3. The tier-tag convention is an addition the spec implied but did not spell out; it is what makes "due-date arithmetic per tier" computable.
- Deliverable 3 (skill, optional arg, reads the doc, sweep, verify, log, verdict, cron offer): Task 4 and the handoff.
- Deliverable 4 (idea file, what is and is not checkable, README row): Task 5.
- Verification (all four files exist and links resolve; packs pass the checklist; skill runs once end to end): Tasks 1, 2, 5, 6.
- Out of scope respected: no CLI or renderer change (the single Go edit is a test's pinned example version), no CI job, no lint.
- Panel review 2026-09-16 (three seats) folded in: the pinned `acme-org@0.1.0` in `TestExamplesSync`, a colon that made the skill frontmatter invalid YAML, the `[weekly]`-only no-signal template, three more multi-instruction bullets in acme-org, the "review by" wording, the style guide's own bold sentences, DeepSeek and Z.ai placed outside the core vendor set, the log's mixed ordering, the AGENTS.md compliance claim, the changelog, and the `status --check` proof of regeneration.
- Names used across tasks: `[weekly]`/`[deep]` tags, `### Primary sources per vendor`, `### Community signal (weekly tier)`, and the verdict line `packs need updating:` are identical in Tasks 3, 4, and 6.
