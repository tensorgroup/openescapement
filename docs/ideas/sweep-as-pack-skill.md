# Ship the source sweep as a pack skill

**Status:** Raw idea, unscoped. No commitment to build.
**Touches:** spec §3 (Pillar A, packs and skills distribution), §6 (the seeded demo pack and `esc pack add-skill`).
**Boundary (§4):** a skill an agent runs on a human's say-so inside a governed repo. It reads public vendor pages and writes one log file in that repo. No server, no telemetry, no automation on `esc`'s side; the cadence is whoever runs it.

## The problem

This repo keeps its render-target assumptions current with `.claude/skills/sweep-sources`: a skill that reads the vendor source list and log from `docs/roadmap/vendor-guidance-tracking.md`, runs whichever tier is due (weekly community signal or monthly deep vendor check), verifies every candidate against a primary source, and appends a tagged log entry with a verdict. It is a maintainer tool today.

A small shop governing its own repos has the same problem one level up: the instruction-file conventions and model lineups its packs encode go stale, and nobody is paid to watch vendor changelogs. Most will not run a cloud routine for it; the maintainer of this repo does not either. What they will do is run a skill by hand when they think of it, and let it tell them what is due.

## What it would be

- The skill vendored into a pack (the seeded demo pack, or `examples/packs/acme-org`) with `esc pack add-skill`, so any governed repo that syncs the pack gets `/sweep-sources` in `.claude/skills/`.
- The source list and log move from a repo-specific doc to a file the skill owns inside the governed repo, seeded on first run, so the skill reads its own state rather than this repo's roadmap doc.
- The verdict line stays: `packs need updating: yes|no`. For a consumer it means "tell your pack author", not "edit the pack".

## What it would not be

- Not a scheduler. `esc` never runs anything on its own; the skill runs when a person invokes it, and its due-date arithmetic is what makes an irregular habit still cover both tiers.
- Not a portal feature. The portal's guidance-freshness view (spec §3, Pillar B) is the fleet-level answer; this is the per-repo, no-server one.

## Open questions

- Whether the source list should be pack-authored (the org decides which vendors matter) or seeded from this repo's list.
- Whether the skill's log belongs in the governed repo or in the pack repo, where the author acts on verdicts.
