# Path-scoped rule files and a cross-tool skills directory: two render targets

Date: 2026-09-17
Status: approved for implementation (revised the same day after design review)

## Goal

Add two built-in render targets so a pack can reach two instruction channels the 2026-09-16 source sweep found agents reading and `esc` not writing:

1. `rules`: path-scoped rule files under `.claude/rules/`, which Claude Code loads with the same priority as `.claude/CLAUDE.md` and scopes to matching files through a `paths:` frontmatter field, and which Grok Build reads for compatibility.
2. `agents-skills`: the pack's skill directories rendered under `.agents/skills/`, the cross-tool skills location Kimi Code scans alongside its own.

A third candidate from the same sweep, the managed-policy `CLAUDE.md` Claude Code reads from an OS path (`/Library/Application Support/ClaudeCode/CLAUDE.md`, `/etc/claude-code/CLAUDE.md`, `C:\Program Files\ClaudeCode\CLAUDE.md`), is decided here and not built: see Decisions.

Decisions made during brainstorming: one rendered file per rules fragment, escapement-owned; opt-in through fragment frontmatter with `paths:` carried through verbatim; `agents-skills` renders byte-identical copies and is off by default; the managed-policy file stays outside `esc`'s write scope.

Decisions made during design review, recorded so they are not relitigated: an explicit `targets:` list stays an exhaustive filter (no default-on exception for `rules`); the whole-file occupied gate and orphan pass that `rules` needs do not exist today for any `KindFile` and are built for all of them, `GOVERNANCE.md` included; fragment line endings are normalized at load; path-scoped fragments cannot be composed.

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

Rules the manifest loader enforces (`internal/pack`, `esc.ErrManifest`, each error naming the fragment):

- `paths:` is allowed only on a fragment whose `targets:` includes `rules`. A managed block cannot be path-scoped.
- `paths:` when present is a non-empty list of non-empty strings with no newline characters. The renderer does not interpret globs; Claude Code does.
- A fragment may target `rules` and block targets in the same list. It then contributes to those blocks as today and also renders as a rule file. `paths:` applies to the rule file only.
- A fragment with `targets: [rules]` and no `paths:` renders as an unconditional rule file, which is Claude Code's documented meaning for a rules file without `paths:`.
- Only a fragment that names `rules` explicitly renders as a rule file. A fragment with no frontmatter or an empty `targets:` list applies to every block target today (`render.fragmentApplies`) and must not become a rule file, or every existing pack would sprout unconditional rule files on its next sync. The rules planner uses the explicit-name test (`render.fragmentNamesTarget`), never the applies-everywhere test.
- The fragment's filename stem (its name without `.md`) must match `^[a-z0-9][a-z0-9-]*$`, because it becomes part of an on-disk filename. Today `rules:` entries are arbitrary safe relative paths; this rule applies only to fragments that target `rules`.
- Fragment content is normalized to LF line endings at load (`render.NormalizeEndings`) before the frontmatter is split. Today the splitter recognizes only LF fences, so a CRLF fragment would lose its frontmatter and its scope silently; normalizing at load fixes that for every target, not just `rules`.
- `pack.ComposeFragments`, which the portal's starter-set adoption uses to merge several fragments into one, returns `esc.ErrManifest` when any part carries `paths:`. Composition would union the parts' targets and drop the scope, turning a path-scoped rule into an unconditional one; refusing is the only safe answer. The portal surfaces the error as it does other manifest errors.

### Rendered file

Each rules fragment renders to `.claude/rules/esc-<pack>-<stem>.md`. Collision checks compare the lowercased path, because macOS and Windows filesystems fold case and a `pack.ValidName` name may carry uppercase: two fragments in one pack colliding is `esc.ErrManifest`; two packs colliding is `esc.ErrConfig` naming both packs, the rule the skills planner applies to colliding skill directories.

File content, in order:

1. The frontmatter block, present only when the fragment has `paths:`: `---\n` then `paths:` serialized with `gopkg.in/yaml.v3` from a `struct{ Paths []string }` (deterministic, correctly quoted), then `---\n`.
2. One line: `<!-- Managed by escapement (pack <name>). Do not edit. -->` followed by a blank line. The pack version is deliberately not in the notice: the lockfile carries provenance, and putting the version in every rule file would rewrite every rule file on every pack version bump. The notice is one line of context per matched rule file; that cost is accepted so a reader of the file knows who owns it, the same reason `GOVERNANCE.md` carries its notice.
3. The fragment body with trailing whitespace trimmed (`strings.TrimSpace`, as `render.Compose` does), followed by exactly one newline.

Output is deterministic: same packs, same bytes. Frontmatter is at byte 0, which is what Claude Code's rules loader requires.

`GOVERNANCE.md` lists rules fragments too, in a section headed by the pack and, for scoped fragments, the `paths:` list, so the human-readable summary keeps covering all policy.

### Engine and lifecycle

Each rendered rule file is a whole-file artifact (`engine.KindFile`), the kind `GOVERNANCE.md` uses: one lockfile entry per file, atomic write. The desired hash is computed over endings-normalized bytes, because `classify` in `internal/engine/status.go` already compares whole-file targets that way while the governance planner hashes raw bytes; the plan fixes the governance planner to match, and a load, sync, status, sync round trip on a CRLF checkout is a test.

The target's `targets.Info` kind is a new constant, `KindRulesDir`, because one target produces many files. The artifacts it produces are plain `KindFile`. `targets.IsFragmentTarget` accepts it.

Three pieces of engine machinery this target needs do not exist today for any `KindFile`, and are built for all `KindFile` artifacts, which today means rule files and `GOVERNANCE.md`:

- **Occupied gate.** Today the never-adopt gate and the byte-identical adoption carve-out are `KindDir`-only; a `KindFile` with no prior lock entry is overwritten, and `classify` has no `occupied` state for it. After this change, a file already at a `KindFile` path with no lock entry is skipped by sync and reported by status as `occupied` (gating `--check`), unless its normalized bytes equal what escapement would write, in which case sync adopts it: writes the lock entry, writes nothing to disk, prints the notice. `--force` does not override occupied, as for directories. This changes first-sync behavior for a repo that already has its own `GOVERNANCE.md`: it is no longer overwritten silently. `esc init`'s explanation for a detected `GOVERNANCE.md` changes to say so and to name the remedy (move or delete the file so escapement can own it). The CHANGELOG marks this as a behavior change.
- **Orphan pass.** Today the retirement passes in apply and the orphan detection in status handle `KindDir` and `KindBlock` only; a retired `KindFile` lock entry is dropped and the file is stranded with no finding. After this change, a `KindFile` lock entry whose artifact is no longer planned (a fragment stopped targeting `rules`, a pack dropped the fragment, the repo removed the target from `targets`) is reported by status as `orphan` and removed by sync, unless its normalized content hash no longer matches the lock entry, in which case removal is declined with a `KindFile` skip cause, the entry is carried forward, and status keeps reporting it. Both passes are new code with the same carry-forward semantics as the block and dir passes.
- **Ownership.** The removal pass takes only lock-listed paths, and each path must pass an ownership test before anything reads the disk: `ownedRulePath` requires exactly one element below `.claude/rules` with the `esc-` prefix; `GOVERNANCE.md` is owned by its exact built-in path. The prefix is a reserved namespace, load-bearing because escapement always writes it and validates it on write; it confines what a hostile lockfile can delete to that namespace. It does not prove escapement created a given file, and the hash gate is not containment either, since a hostile entry can carry the correct hash: this is the same ruling AGENTS.md records for skill directories. A team's own `esc-` prefixed file under `.claude/rules/` is therefore not protected by name; the docs say the prefix is reserved.

Containment runs before every read, not only before writes. Today plan-time `prospectiveContent`, status, and `esc diff` read a `KindFile` destination without symlink refusal, so a symlinked `.claude/rules/` or a symlinked rule file would be read through. The plan adds `refuseSymlinks` on the rules directory and on each rule path before any read in plan, status, and diff, exiting 4 with the existing containment message, with regression tests for each command.

`esc diff --against REF`, the policy review `esc update` points users at, iterates a hard-coded target list and diffs only single-file targets, so rule files would be invisible to it. The plan extends it: rule-file artifacts are diffed by path (added, removed, body changed, `paths:` changed), and the default target list is derived from the targets table rather than a second literal.

Team-authored files under `.claude/rules/` are untouched by every sync. Nothing in this target reads or reports them; the local axis has nothing to say about a directory of separate files.

### Targets, defaults, and `esc init`

`rules` is a built-in and part of the set an empty `targets:` list means. An explicit `targets:` list remains an exhaustive filter, as it is today for every target: a repo with `targets: [claude, agents]` does not render rule files until it adds `rules`. That is the org's opt-out, and it is the same rule custom targets already follow. To keep the silent case visible, `esc sync` prints one stderr line when a pack fragment targets `rules` and the repo's filter excludes it ("pack X targets rules; this repo's targets list excludes it"), with no finding and no exit-code change.

`agents-skills` is the one built-in that is **not** part of the empty-list set. `targets.Info` gains `OptIn bool` (zero value false, so the six existing rows need no edit), and the default set is derived from the table wherever a target list is enumerated today: `engine.allTargets`, `initscan.targetOrder`, `PolicyDiff`'s default list, and `render`'s constants are reconciled to one source of truth, with `TestTargetOrderCoversAllTargets` extended to the new rows. `esc init` includes `rules` in any explicit `targets:` list it writes, so a newly governed repo gets rule files by default, and pre-fills `agents-skills` when it finds a non-empty `.agents/skills/` directory that is a real directory: detection uses `Lstat` and skips a symlink, because a `.agents/skills -> ../.claude/skills` layout would otherwise pass init and then fail sync's symlink refusal at exit 4. The same applies to a symlinked `.claude/rules/`: containment refuses it with a message naming the fix (replace the symlink with a directory, or drop the target).

Packs that ship a `targets: [rules]` fragment require an `esc` that knows the target; an older binary rejects the fragment as an unknown target (`ErrConstraint`, exit 1). The README and CHANGELOG say so; there is no minimum-version field in the manifest and this change does not add one.

## Agents-skills target

`agents-skills` renders the same skill directories the `skills` target renders, at `.agents/skills/<name>` instead of `.claude/skills/<name>`, each a `KindDir` artifact with its own lock entry and file manifest. The planner is the existing skills planner with a second root; the ownership check (`ownedSkillPath`) accepts either root. Skill name collisions are checked per root, lowercased.

Two copies of every skill is the deliberate outcome: Claude Code reads `.claude/skills/` and Kimi Code reads `.agents/skills/`, and a repo using both needs both. Byte-identical copies are what a deterministic renderer can promise; a symlink would violate the containment rules (`esc` neither writes nor follows symlinks), and a single configurable root cannot serve both agents.

The `SkillDuplicate` finding today attaches to every `KindDir` finding and compares the directory's base name against `~/.claude/skills/<name>`. The loop is scoped to artifacts under `.claude/skills/` so an `.agents/skills/` copy never reports a duplicate against the user's Claude skills. Extending the finding to `~/.agents/skills` is not done; it can be added when a user asks.

## Decisions

- **The managed-policy CLAUDE.md is not a render target.** `esc` writes agent-readable files only inside the governed repo, and keeps only its pack cache and `esc serve` data under the home directory (`docs/cli-and-portal.md`). The OS-path file is deployed by IT through device management; it is a fleet channel, and the fleet is the portal's concern, not the CLI's. An org that wants pack content there can take the rendered block text from a governed checkout (`esc render` prints it) and ship it by its own means. Revisit only if a customer asks for the portal to publish it, which would be a portal feature, not a CLI target.
- **Cursor's `.cursor/rules/*.mdc` is out of scope.** It is a different file format with its own frontmatter; it gets its own cycle when there is demand.
- **Custom targets keep refusing `.claude/` and `.agents/`.** The built-in targets are the only writers under control directories; that rule is unchanged.
- **Explicit `targets:` lists stay exhaustive.** No built-in is added behind a repo's back. The cost is that existing repos with explicit lists must add `rules` to receive rule files; the stderr notice above is how they learn a pack wanted to.

## Out of scope

- No changes to the portal beyond surfacing the compose error.
- No changes to the pack cache, sources, signing, or `esc pack` author commands.
- No `SkillDuplicate` extension to `~/.agents/skills`.
- No manifest minimum-version field.

## Testing

- Golden tests in `internal/render` for a rules fragment with `paths:`, without `paths:`, with a body lacking a trailing newline, and with a CRLF body; the notice line; a `paths:` entry containing a quote or backslash serializes correctly; a `paths:`-carrying fragment's frontmatter never leaks into a managed block.
- Manifest tests in `internal/pack`: `paths:` without `rules` is an error naming the fragment; an empty `paths:` list and an empty or newline-carrying entry are errors; a bad stem is an error; a fragment targeting both `rules` and `claude` composes into the block and renders the file; a fragment with no frontmatter does not render as a rule file; CRLF frontmatter is split correctly after normalization; `ComposeFragments` rejects a part with `paths:`.
- Targets tests in `internal/targets`: `rules` and `agents-skills` are in the table, `IsFragmentTarget` accepts `rules`, the default set excludes `agents-skills`, `TestTargetOrderCoversAllTargets` covers both.
- Engine tests in `internal/engine`, for rule files and for `GOVERNANCE.md` alike where the behavior is shared: filename collision within a pack (`ErrManifest`) and across packs (`ErrConfig`), case-folded; hand-edit skip and `--force`; occupied (identical adopts, non-identical skips, `--force` does not override); orphan reporting and removal; declined removal when edited, with the lock entry carried forward and status still reporting; ownership (a lock entry naming `.claude/rules/team.md`, an unprefixed file, or a nested path is refused before any read); symlinked `.claude/rules/` refused at exit 4 in plan, status, diff, and sync; desired hash equals classify's hash on a CRLF checkout (load, sync, status, sync round trip); `agents-skills` absent from the default set and present when listed; two roots with their own manifests, independent retirement, and amendment preservation in each; `SkillDuplicate` never attaches to an `.agents/skills/` artifact; the stderr notice when a filter excludes `rules`.
- `esc diff --against` tests: a rule file added, removed, body changed, `paths:` changed each appear in the diff.
- The write-path × file-shape matrix (`internal/shapetest`) gains three paths: the rule-file write, the rule-file removal, and the occupied adoption compare, each crossed with no trailing newline, CRLF, and empty body.
- `internal/cli`: `esc init` writes `rules` into an explicit list, pre-fills `agents-skills` for a real `.agents/skills/` and not for a symlinked one, and explains `GOVERNANCE.md` as occupied; `TestEveryExamplePackSyncs` asserts the acme-org rule file exists after sync and that its frontmatter parses to the expected `paths:`; a governed-service variant with `agents-skills` listed renders both roots.
- `examples/packs/acme-org` gains `rules/api-handlers.md` with `targets: [rules]` and `paths: ["src/api/**"]`; `examples/governed-service` is regenerated and `TestExamplesLockMatchesShippedFiles` guards the pair.

## Docs

- README: the target list gains `rules` and `agents-skills`, with the opt-in note, the explicit-list rule, and the older-binary note.
- `docs/pack-authoring.md`: a short section on writing a rules fragment (when to scope, keep the file small, the notice line is not yours, the `esc-` prefix is reserved).
- `docs/cli-and-portal.md`: one sentence recording the managed-policy decision.
- CHANGELOG: Added entries for both targets; a Changed entry for the whole-file occupied gate and orphan pass, naming `GOVERNANCE.md`.
- AGENTS.md: the never-adopt and retirement invariants now say "artifact" rather than "directory" where the behavior became general.

## Verification

- `go test ./...`, `go vet ./...`, `gofmt -l .` clean.
- `examples/governed-service` after sync contains `.claude/rules/esc-acme-org-api-handlers.md` with the `paths:` frontmatter, and `esc status --check` exits 0.
- A repo with `targets: []` and a pack that targets `rules` renders the rule file; the same repo with `targets: [claude, agents]` does not, and sync prints the notice; adding `rules` renders it. Neither repo gains `.agents/skills/` until it lists `agents-skills`.
- A repo that already has its own `GOVERNANCE.md` reports `occupied` on first status and is not overwritten by sync.
