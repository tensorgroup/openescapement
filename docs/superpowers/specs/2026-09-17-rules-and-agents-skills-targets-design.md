# Path-scoped rule files and a cross-tool skills directory: two render targets

Date: 2026-09-17
Status: approved for implementation

## Goal

Add two built-in render targets so a pack can reach two instruction channels the 2026-09-16 source sweep found agents reading and `esc` not writing:

1. `rules`: path-scoped rule files under `.claude/rules/`, which Claude Code loads with the same priority as `.claude/CLAUDE.md` and scopes to matching files through a `paths:` frontmatter field, and which Grok Build reads for compatibility.
2. `agents-skills`: the pack's skill directories rendered under `.agents/skills/`, the cross-tool skills location Kimi Code scans alongside its own.

A third candidate from the same sweep, the managed-policy `CLAUDE.md` Claude Code reads from an OS path (`/Library/Application Support/ClaudeCode/CLAUDE.md`, `/etc/claude-code/CLAUDE.md`, `C:\Program Files\ClaudeCode\CLAUDE.md`), is decided here and not built: see Decisions.

Decisions made during brainstorming: one rendered file per rules fragment, escapement-owned; opt-in through fragment frontmatter with `paths:` carried through verbatim; `agents-skills` renders byte-identical copies and is off by default; the managed-policy file stays outside `esc`'s write scope.

## Rules target

### Pack authoring

A fragment under `rules/` opts in by naming `rules` in its frontmatter `targets:` list. It may also carry `paths:`, a non-empty list of glob strings, which is the only new frontmatter field:

```markdown
---
targets: [rules]
paths:
  - "src/api/**/*.ts"
---
## API handlers

- Validate every request body at the boundary. ...
```

Rules the manifest loader enforces (`internal/pack`, `esc.ErrManifest`):

- `paths:` is allowed only on a fragment whose `targets:` includes `rules`. A managed block cannot be path-scoped, so `paths:` on a block-only fragment is an error naming the fragment.
- `paths:` entries are non-empty strings. The renderer does not interpret them; Claude Code does. No glob validation beyond that.
- A fragment may target `rules` and block targets in the same list. It then contributes to those blocks as today and also renders as a rule file. `paths:` applies to the rule file only.
- A fragment with `targets: [rules]` and no `paths:` renders as an unconditional rule file (loaded in every session), which is Claude Code's documented meaning for a rules file without `paths:`.

### Rendered file

Each rules fragment renders to `.claude/rules/esc-<pack>-<stem>.md`, where `<pack>` is the pack name and `<stem>` is the fragment's filename without `.md`. Two packs, or two fragments in one pack, producing the same path is a config error (`esc.ErrConfig`) naming both, the same rule the skills planner applies to colliding skill directories.

File content, in order:

1. The frontmatter block `---\npaths:\n  - "..."\n---\n`, present only when the fragment has `paths:`; entries are emitted as double-quoted YAML strings in the fragment's order.
2. One line: `<!-- Managed by escapement (pack <name>@<version>). Do not edit. -->`, followed by a blank line. The pack version is in the notice so a version bump changes the file, as it changes a managed block's header.
3. The fragment body, byte for byte, ending with exactly one newline.

Output is deterministic: same packs, same bytes.

### Engine and lifecycle

Each rendered rule file is a whole-file artifact (`engine.KindFile`), the kind `GOVERNANCE.md` already uses: the hash covers the whole file, one lockfile entry per file, atomic write. The target's `targets.Info` kind is a new constant, `KindRulesDir`, because one target produces many files; the plan-time expansion is per fragment.

The existing gates apply without modification, and the plan's tests assert each one on the new path:

- **Hand-edit skip.** A rule file whose content hash differs from its lock entry is skipped with the stderr warning and a `hand-edited` skip cause; the lock entry is left unchanged so `esc status` keeps reporting it; `esc sync --force` overwrites.
- **Occupied.** A file already at the path with no lock entry is never adopted. It is skipped and reported by status as `occupied`, gating `--check`, unless its bytes exactly equal what escapement would write, in which case sync adopts it (writes the lock entry, writes nothing to disk, prints the notice). This is the `KindDir` carve-out extended to `KindFile`; the plan checks whether `GOVERNANCE.md` already behaves this way and, if it does not, the behavior is added for both.
- **Retirement.** A fragment that stops targeting `rules`, a pack that drops the fragment, or a repo that removes `rules` from `targets` leaves the file orphaned. `esc status` reports it as `orphan` before sync is asked to delete it. Sync removes it on the next run, unless its content no longer matches the lock entry, in which case removal is declined (`orphan-block-edited` semantics, with a `KindFile` cause name if the existing one reads wrong). The plan checks whether the orphan pass already handles `KindFile` and adds it if not.
- **Ownership.** The removal pass takes only lock-listed paths, and `ownedRulePath` requires exactly one element below `.claude/rules` with the `esc-` prefix, so a lock entry can never name a team's own rule file. Containment and symlink refusal run before anything reads or writes, as for every other artifact.

Team-authored files under `.claude/rules/` are untouched by every sync. Nothing in this target reads or reports them; the local axis has nothing to say about a directory of separate files.

### Defaults and `esc init`

`rules` is a built-in and part of the set `targets: []` means. It is inert until a pack fragment targets it, so no existing repo changes on its next sync. `esc init` does not detect `.claude/rules/`; there is nothing to offer, since escapement never writes into a file it did not create there.

## Agents-skills target

`agents-skills` renders the same skill directories the `skills` target renders, at `.agents/skills/<name>` instead of `.claude/skills/<name>`, each a `KindDir` artifact with its own lock entry and file manifest. The planner is the existing skills planner with a second root; the ownership check (`ownedSkillPath`) accepts either root. Skill name collisions are checked per root.

Two copies of every skill is the deliberate outcome: Claude Code reads `.claude/skills/` and Kimi Code reads `.agents/skills/`, and a repo using both needs both. Byte-identical copies are what a deterministic renderer can promise; a symlink would violate the containment rules (`esc` neither writes nor follows symlinks), and a single configurable root cannot serve both agents.

`agents-skills` is **not** part of the set `targets: []` means. A repo enables it by listing it in `targets`. This is the one built-in that is opt-in, and `internal/targets` records that on its `Info` (`DefaultOn bool`), so the config resolver reads it from the table rather than special-casing the name. `esc init` pre-fills `agents-skills` in `targets` when it finds a non-empty `.agents/skills/`, matching what it does for `.claude/skills/`.

The `SkillDuplicate` finding (a managed skill name also present under `~/.claude/skills`) is not extended to `~/.agents/skills`. It can be added when a user asks.

## Decisions

- **The managed-policy CLAUDE.md is not a render target.** `esc` writes agent-readable files only inside the governed repo, and keeps only its pack cache and `esc serve` data under the home directory (`docs/cli-and-portal.md`). The OS-path file is deployed by IT through device management; it is a fleet channel, and the fleet is the portal's concern, not the CLI's. An org that wants pack content there can produce it with `esc render` and ship it by its own means. Revisit only if a customer asks for the portal to publish it, which would be a portal feature, not a CLI target.
- **Cursor's `.cursor/rules/*.mdc` is out of scope.** It is a different file format with its own frontmatter; it gets its own cycle when there is demand.
- **Custom targets keep refusing `.claude/` and `.agents/`.** The built-in targets are the only writers under control directories; that rule is unchanged.

## Out of scope

- No changes to the portal.
- No changes to the pack cache, sources, signing, or `esc pack` author commands.
- No `SkillDuplicate` extension to `~/.agents/skills`.

## Testing

- Golden tests in `internal/render` for a rules fragment with `paths:`, without `paths:`, and with a body lacking a trailing newline; the notice line and version bump behavior.
- Manifest tests in `internal/pack`: `paths:` without `rules` is an error naming the fragment; an empty `paths:` entry is an error; a fragment targeting both `rules` and `claude` composes into the block and renders the file.
- Engine tests in `internal/engine`: filename collision across packs is `ErrConfig`; hand-edit skip and `--force`; occupied (identical and non-identical); orphan removal and declined removal when edited; ownership (a lock entry naming `.claude/rules/team.md` or a nested path is refused); `agents-skills` absent from the default set and present when listed; two roots with their own manifests.
- The write-path × file-shape matrix (`internal/shapetest`) gains the rule-file write path: no trailing newline in the fragment, CRLF fragment, empty body.
- `internal/cli`: `esc init` pre-fills `agents-skills` when `.agents/skills/` exists; `TestEveryExamplePackSyncs` covers the new example fragment.
- `examples/packs/acme-org` gains `rules/api-handlers.md` with `targets: [rules]` and `paths: ["src/api/**"]`; `examples/governed-service` is regenerated and `TestExamplesLockMatchesShippedFiles` guards the pair.

## Docs

- README: the target list gains `rules` and `agents-skills`, with the opt-in note.
- `docs/pack-authoring.md`: a short section on writing a rules fragment (when to scope, keep the file small, the notice line is not yours).
- `docs/cli-and-portal.md`: one sentence recording the managed-policy decision.
- CHANGELOG: Added entries for both targets.

## Verification

- `go test ./...`, `go vet ./...`, `gofmt -l .` clean.
- `examples/governed-service` after sync contains `.claude/rules/esc-acme-org-api-handlers.md` with the `paths:` frontmatter, and `esc status --check` exits 0.
- A repo with `targets: [claude, agents]` and a pack that targets `rules` still renders the rule file (rules is default-on); the same repo does not gain `.agents/skills/` until it lists `agents-skills`.
