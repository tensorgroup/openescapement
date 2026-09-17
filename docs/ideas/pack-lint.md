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
