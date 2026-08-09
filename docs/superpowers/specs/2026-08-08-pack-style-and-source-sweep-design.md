# Pack authoring style guide + source sweep process — design

Date: 2026-08-08
Status: approved for implementation

## Goal

Two related governance-hygiene pieces:

1. A committed style rule for writing pack rule files, built on the subset of Simplified Technical English (ASD-STE100) principles that actually transfer to agent instruction files, plus the things instruction files need that STE lacks.
2. A two-tier source sweep process (weekly community signal, monthly deep vendor check) over the sources that shape instruction-file conventions, with an on-demand "do it now" trigger, so pack and renderer assumptions never go quietly stale.

Decisions made during brainstorming: the trigger is a repo Claude Code skill (no product code, no CI job); cadence is weekly community + monthly deep (quarterly judged too slow for the current pace of change); the style rule ships as a doc now with mechanical linting filed as an idea, not built.

## Deliverable 1: `docs/pack-authoring.md`

The committed rule for writing pack rule files. Applies equally to this repo's own instruction files. Contents:

**Principles adopted from STE:**

- Imperative, active voice ("Run X before Y", not "it is recommended that").
- One term per concept, used consistently. A pack picks its names ("managed block", not sometimes "escapement region") and sticks to them.
- One instruction per sentence.
- No pronouns with unclear antecedents. "It" and "this" must have an obvious referent; when in doubt, repeat the noun.

**Principles beyond STE, specific to instruction files:**

- State the *why* for any rule an agent might be tempted to work around. Models generalize a rule far better with its rationale ("this is a security posture, not a preference").
- Every line must change agent behavior or be cut. Length is the real budget: these files load into every session.
- Progressive disclosure: point to deeper docs instead of inlining them.
- No ALL-CAPS or repeated emphasis; no restating what an agent can discover from the repo itself. (Both per the Claude 5 context-engineering rules already logged in `docs/roadmap/vendor-guidance-tracking.md`.)

**Explicit rejection, recorded so it is not relitigated:** full ASD-STE100 — the ~900-word controlled dictionary, one-approved-meaning-per-word, sentence-length caps. STE solves a human non-native-reader and translation problem that LLMs do not have; the dictionary costs precision on terms of art and its procedural style strips rationale, which is one of the highest-value elements of an instruction file.

Ends with a short author checklist. Cross-linked from `AGENTS.md`'s "Deeper context" list and from `examples/README.md`. The existing example packs (`examples/packs/acme-org`, `examples/packs/anthropic-models`) get audited against the checklist as part of implementation; fixes are applied, not just noted.

## Deliverable 2: two-tier structure in `docs/roadmap/vendor-guidance-tracking.md`

The tracking doc stays the single source of truth for the source list and the log. Changes:

**Tier 1 — weekly community signal scan.** New source set:

- Anthropic: r/Anthropic, r/ClaudeAI, r/ClaudeCode
- OpenAI: r/OpenAI, r/ChatGPTCoding
- Google: r/Bard, r/GeminiAI
- xAI: r/grok
- Cross-vendor: r/LocalLLaMA (also covers Moonshot/Kimi, which has no active English subreddit), Hacker News via Algolia search
- Subreddit names are themselves a moving target; the sweep corrects the list when one renames or dies.

Community sources are signal-scanning only. The doc's existing lesson is promoted to a stated rule: **community signal never triggers action directly; every candidate claim is verified against the primary source before it enters the log or drives any change.** (Secondary coverage has already been wrong twice: the stale Grok size cap and the wrong AAIF date, both caught 2026-07-30.)

**Tier 2 — monthly deep check.** The existing primary-source vendor list, cadence changed from quarterly to monthly, still additionally triggered by every major model-generation release.

**Log discipline:** format unchanged. A weekly sweep that finds nothing appends a one-line "no signal" entry, so the due-date arithmetic in the skill always has a real last-run date per tier.

## Deliverable 3: `.claude/skills/sweep-sources/SKILL.md`

The "do it now" button, checked into the repo so anyone working here can run `/sweep-sources`. Optional argument: `weekly`, `deep`, or `all`. With no argument, the skill reads the log dates in the tracking doc and runs whatever is due — a habitual weekly invocation therefore escalates to the deep check automatically once a month has passed.

Behavior, in order:

1. Read the source list and last-run dates from `docs/roadmap/vendor-guidance-tracking.md`. The skill never duplicates the source list.
2. Sweep due sources via web search/fetch. Filter aggressively to what matters to this product: instruction-file conventions (new files, renames, hierarchy or size-cap changes), context/prompting guidance changes, new agent targets worth rendering to, major model-generation releases. Ignore hype, benchmarks, and product drama.
3. Verify every candidate finding against the relevant primary source. Discard what does not hold; note corrections to the source list itself (dead or renamed subreddits).
4. Append a dated log entry (including "no signal" when empty), then assess impact against the example packs, `docs/pack-authoring.md`, and renderer targets/defaults, and end with an explicit verdict: **packs need updating: yes/no**, with reasons. Substantive changes still go through the normal spec → design-doc cycle per the tracking doc's existing rule; the skill never patches the renderer ad hoc.

After the skill lands, a Claude Code cron job invoking it weekly is offered as an optional final step so the cadence does not depend on anyone remembering.

## Deliverable 4: `docs/ideas/pack-lint.md`

Idea file for a future `esc pack lint`: the mechanically checkable subset of the style guide — ALL-CAPS emphasis detection, line/file budgets, term-consistency against a per-pack glossary. Status: raw idea, unscoped. Notes what is *not* mechanically checkable (ambiguous pronouns, missing rationale) so the lint is never mistaken for the whole rule. Gets a row in the `docs/ideas/README.md` table.

## Out of scope

- No `esc` CLI or renderer changes.
- No GitHub Action or unattended CI sweeps; the portal guidance-freshness view (spec §3, Pillar B) remains the productized future of this process.
- No lint implementation.

## Verification

- All four files exist, cross-links resolve (AGENTS.md → pack-authoring, examples/README → pack-authoring, ideas README table row).
- Example packs pass the pack-authoring checklist after audit.
- `/sweep-sources` runs end to end once ("do it now") and appends a well-formed log entry.
