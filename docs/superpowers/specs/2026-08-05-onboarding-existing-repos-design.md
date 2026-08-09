# Onboarding into repos that already have instruction files

Status: approved design, 2026-08-05. Third of three specs in this cycle. Depends on the local amendment model (2026-08-05) for the vocabulary it reports; independent of the telemetry surface spec.

Motivation: the common first contact is a repo that already has a hand-written CLAUDE.md or AGENTS.md. That path already works mechanically, because `Splice` preserves every byte outside the managed block and the amendment model reports the surrounding content as `local: amended`. What it lacks is sensible placement, a config bootstrap that reflects what is actually in the repo, and any help reconciling rules the team already wrote against the rules the pack brings.

The scope split here is deliberate: placement and detection are deterministic and belong in the CLI; reconciling the semantic overlap between a team's rules and a pack's rules is judgment work and belongs in an agent, driven by a skill the pack ships.

## 1. Top placement for new blocks

`render.Splice` currently appends a new block at the end of a non-empty file (`internal/render/block.go:64`). For a governance tool this is the wrong default: the org baseline lands after whatever the team already wrote, and on conflict the resolution a reader reaches is ambiguous. Org policy should anchor the file.

`Splice` keeps its current branch structure and changes only its final branch. Branches are evaluated in this order:

1. Block already present: replaced in place, exactly as today. **Existing repos never see their block move.** Top placement applies only to insertions into files that do not yet carry one.
2. `<!-- escapement:block -->` placeholder present: substituted in place, as today. This remains the explicit override for teams that want the block elsewhere.
3. Empty file: block only, as today.
4. Otherwise (the changed branch): inserted at the top of the file, after the two leading constructs below, instead of appended at the end.

Two leading constructs are skipped when computing "the top":

- **YAML frontmatter.** A `---` fence at byte 0 through its closing `---` line. This is correctness, not taste: content inserted above frontmatter stops the frontmatter from being frontmatter, silently breaking any tool that reads it.
- **A leading H1.** A single `# Heading` line, plus one immediately following blank line if present. Placing a policy block above a document's title reads as damage rather than as governance.

No other constructs are skipped. The rule set is closed at two, deliberately; each additional heuristic makes the output harder to predict and the golden tests less meaningful.

Insertion leaves exactly one blank line between the block and the content that follows it, and preserves the original bytes of everything it moved past. The renderer invariants from AGENTS.md continue to hold: bytes outside the managed block are never modified, writes are atomic, output is deterministic.

## 2. `esc init` detects, explains, then offers

`cmdInit` (`internal/cli/cli.go:119`) writes config scaffolding and never looks at the repo. It gains a detection pass.

**Detection.** Scan for the files in `render.TargetFile`, plus `.mcp.json` and any existing skill directories. Report what was found with sizes.

**Config bootstrap.** Pre-fill `targets` in the generated config with the detected set. When nothing is detected, leave `targets` empty, which continues to mean all targets. A repo that already has AGENTS.md and no GEMINI.md should not have GEMINI.md conjured into existence by its first sync.

**Explanation.** Print what the first sync will do, in the amendment model's vocabulary, so the state the portal will show is the state the user was told about:

```
Found CLAUDE.md (183 lines) and .mcp.json (4 servers).

`esc sync` will insert a managed block at the top of CLAUDE.md and add pack
MCP servers alongside your existing ones. Your current content is preserved
byte for byte and reported as a local amendment.

To place the block somewhere else, put <!-- escapement:block --> where you
want it before syncing.
```

**The offer.** After explaining, on a TTY, init offers to place the block marker for each detected file:

> **Amended as shipped (final review of this cycle).** Two things below did not survive implementation, both recorded here rather than silently rewritten. (1) The per-file, four-option prompt became one gate, `Place the marker now? [y/N]`, asked once, plus one position question (`above the title` or `end of file`) whose answer applies to every eligible file. Six code paths already converged on writing nothing, so a repo with four instruction files got four consecutive prompts that each recommended doing nothing; and the "top (recommended)" and "after the heading" options were the same position as §1 top placement, so they would have written a placeholder that changed nothing. (2) The undo guard rail below is stated too broadly: it holds inside a git repo, where a dirty file is skipped, but in a plain directory there is no undo at all and init would still write. As shipped, the gate says so and the write still requires an explicit yes. `--yes` no longer accepts a placement; it declines the gate, so no non-interactive run modifies a detected file.

```
Place the managed block in CLAUDE.md?
  [t] top (recommended)   [h] after the heading   [e] end   [s] skip
```

Accepting writes `<!-- escapement:block -->` at the chosen position and nothing else. Init still never renders policy content; it only records where the block goes, and the next `esc sync` fills it. That keeps a getting-started command from writing rules a user has not seen, and keeps `Splice` the single code path that ever emits managed content.

Declining every offer leaves the repo exactly as detection found it, and §1 top placement still applies at first sync. The offer is a convenience for choosing position, never a prerequisite.

Guard rails, all of which the tests below assert:

- **TTY only.** Non-interactive runs detect, explain, modify nothing, and exit 0. This reuses the existing update-check seam pattern (`maybeUpdates`, `internal/cli/cli.go:149`) so no test can block on real stdin.
- **`--yes` accepts the recommended placement** for every detected file, for scripted onboarding, and is the only way to modify files without a TTY.
- **Default is skip.** An empty answer changes nothing.
- **Clean worktree required per file.** A file with uncommitted changes is reported and skipped, with the reason given. Git is the undo mechanism for everything init writes, so init does not write where git cannot undo it.
- Writes are atomic and preserve every existing byte, the same invariants the renderer holds.

**Merging the team's rules against the pack's is still not init's job.** The offer above is placement only. Semantic reconciliation is §3, executed by an agent, for the reasons given there. Init's closing output names the reconcile skill as the next step once a pack is synced, which is the walkthrough completing rather than the CLI taking on judgment work.

Running `init` in an already-initialized repo continues to error, unchanged.

## 3. Reconciliation ships as a pack skill

A team's existing rules will duplicate and sometimes contradict the pack's. Detecting that is semantic work. Doing it in `esc` means either a heuristic wrong often enough to erode trust in a tool whose value is being trustworthy, or a model call, which breaks the single external dependency policy, the no-network posture, and "boring and fast" simultaneously. Neither is acceptable.

The reconciliation guidance ships as a skill in the pack, executed by whichever agent the team already uses. The org controls it by publishing a pack version, so it travels the same versioned, signed channel as every other rule.

The seeded demo pack (`internal/portal/seed/repos.go`) gains `skills/esc-reconcile/SKILL.md` and a `skills:` entry in its manifest, serving as the reference implementation. The skill instructs an agent to:

- Read the managed block and the surrounding content in the same file.
- Report surrounding rules that duplicate a pack rule, and surrounding rules that contradict one.
- Propose edits to the human-authored sections only.
- **Never edit inside the begin/end markers.** Edits there produce the `altered` state, which stops that artifact from receiving policy updates until a human resolves it.

That last constraint is the useful interlock: the amendment model gives the reconcile skill a precise, mechanically checked boundary, and an agent that ignores it produces a state the portal surfaces rather than a silent corruption.

## 4. Testing

Placement, as golden tests over `Splice`:

- Non-empty file with no block: block lands at top, original content follows after one blank line, bytes preserved.
- File with YAML frontmatter: block lands after the closing fence, frontmatter still parses.
- File with a leading H1: block lands after the heading and its blank line.
- File with frontmatter and an H1: both skipped, in order.
- File whose first line is `#hashtag` or `## Sub`, neither of which is a leading H1: block lands at byte 0.
- Unterminated frontmatter fence: treated as ordinary content, block lands at byte 0, no hang.
- File already carrying a block at the bottom: block stays at the bottom on resync.
- Placeholder present anywhere: block substitutes there, top placement not applied.
- Empty and whitespace-only files.

Init, as CLI integration tests over real temp repos:

- Repo with CLAUDE.md and AGENTS.md: both detected, `targets` pre-filled with exactly those, explanation names both.
- Repo with no instruction files: `targets` left empty, explanation reflects a clean start.
- Non-TTY run: detection output printed, no existing file modified, exit 0.
- Injected TTY answers, one case per option: `t`, `h`, `e` each write the placeholder at the expected offset; `s` and an empty answer write nothing.
- `--yes` without a TTY: recommended placement written to every detected file.
- File with uncommitted changes: skipped with a stated reason even when the answer accepts, and the rest of the detected files still processed.
- Placeholder already present in a detected file: no offer made for it, nothing written.
- After an accepted offer, `esc sync` fills that placeholder rather than applying §1 top placement.
- End to end: `init`, then `sync`, then `status` on a repo with a pre-existing 200-line CLAUDE.md reports `in-sync`/`amended` with the original content intact and the block at the top.

## 5. Implementation ordering

The placement change touches the renderer and rewrites its golden files, which the amendment model spec also does. Nothing from either spec is implemented yet, so §1 should be folded into that spec's renderer work rather than churning the same goldens twice. §2 and §3 are independent and can land in any order after it.

## 6. Docs

- `AGENTS.md`: top placement as a renderer invariant, and the placeholder as the documented override.
- README: the "adding esc to a repo that already has instruction files" path, covering placement, what init reports, and the reconcile skill.
- Product spec: §3 records that reconciliation is agent-executed and pack-delivered, not a CLI feature, with the determinism rationale.
- CHANGELOG bullet. No em-dashes.
