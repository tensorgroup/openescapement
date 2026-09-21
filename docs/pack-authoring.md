# Writing pack rules

A rule file is a markdown file under `rules/` in a pack. The renderer places it inside a managed block in every instruction file the pack targets (`CLAUDE.md`, `AGENTS.md`, `GEMINI.md`), in every repo that syncs the pack, and an agent loads that block into every session. One wasted line in a rule file is wasted in every session in every repo. Write accordingly.

This guide is the committed rule for every rule file, skill `SKILL.md`, and catalog note in a pack. It applies to this repository's own `AGENTS.md` as well.

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

## Path-scoped rule files

A rule fragment normally joins the shared managed block in `CLAUDE.md`, `AGENTS.md`, and `GEMINI.md`. Add `targets: [rules]` to its frontmatter to render it as its own file under `.claude/rules/` instead.

Add `paths:` to scope the rule to part of the tree. Give it a non-empty list of glob strings, for example `paths: ["src/api/**"]`. Omit `paths:` for a rule that applies unconditionally.

Keep a path-scoped rule file small. An agent loads it the same way it loads `.claude/CLAUDE.md`, in full, whenever the rule's scope is in play.

Do not write your own notice line into the file body. Escapement writes that line for you, the same way it does for every other managed target.

The `esc-` prefix is not reserved in your rule's *filename stem* — a stem like `esc-x` is legal and simply composes into `.claude/rules/esc-acme-esc-x.md`. What is reserved is the *destination* namespace: any path matching `.claude/rules/esc-*` is treated as escapement's, whether or not escapement actually wrote it. Do not place a team-owned file there yourself. Unlike other team-added content, a file in that namespace gets none of the protection escapement otherwise gives a hostile or stale lockfile entry, because the prefix alone is what escapement checks before touching a path there.

The filename stem must match `^[a-z0-9][a-z0-9-]*$`. Pick one that reads clearly next to the pack name it gets composed with.

The portal's starter-set adoption cannot compose a path-scoped fragment. Adoption merges fragment bodies into a repo's own draft pack, and that merge has no slot for a `paths:` scope: composing a set that includes even one path-scoped fragment fails outright. Ship a path-scoped rule directly in a pack instead.

## What this guide rejects

Full ASD-STE100 is rejected, and recorded here so it is not relitigated. STE's controlled dictionary (about 900 approved words, one approved meaning each) and its sentence-length caps solve a problem models do not have: non-native human readers and translation. The dictionary costs precision on terms of art, and STE's procedural style strips rationale, which is among the highest-value content in an instruction file. Take the four principles above. Leave the rest.

## Checklist

Run this over every rule file, skill, and catalog note before publishing a pack version.

- [ ] Every rule is imperative and active.
- [ ] One term per concept, used consistently across the whole pack.
- [ ] One instruction per sentence.
- [ ] Every `it`, `this`, `these`, `they` has one obvious referent.
- [ ] Every rule an agent might work around states its reason.
- [ ] Every rewrite keeps the rule's meaning: a permission stays a permission, a list stays a list.
- [ ] Every line would change agent behavior if kept and would not if cut.
- [ ] Detail lives in a linked doc, not inline.
- [ ] No ALL-CAPS emphasis, no repeated emphasis, no bold sentences.
- [ ] Nothing restates what the repo itself shows.

Four items are checkable by machine, at least in part: one term per concept, one instruction per sentence, the line budget (as a size cap, since whether a line changes behavior is a judgment), and no emphasis. They are candidates for a future `esc pack lint` (`docs/ideas/pack-lint.md`). The rest are review.
