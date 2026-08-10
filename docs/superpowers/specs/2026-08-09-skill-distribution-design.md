# Skill distribution: vendoring external skills into packs, agents, and user-level installs

Status: approved design, 2026-08-09.

Motivation: skills are becoming the highest-leverage artifact a pack can carry. An org will often want most of its people running a skill that saves tokens or encodes a working method, and today the only way a pack ships one is as a directory the pack author wrote by hand inside the pack repo. The skills people actually want usually live in someone else's GitHub repo, come in suites, get updated upstream, and may already be installed on a developer's machine at the user level. This spec gives packs a way to adopt external skills with provenance and a maintenance path, gives individuals a managed way to install skills for themselves, adds subagent files as a pack artifact, and settles naming and conflicts across all of it.

Two workflows, one rule for where things land. Pack-mandated skills are policy: they go in the project at `.claude/skills/`, versioned with the repo, identical for the whole team. Personal skills are the individual's business: they go in `~/.claude/skills/`, tracked by escapement but mandated by nobody. Who mandates a skill decides where it lives.

## 1. Vendoring, not referencing

When a pack author adds a skill from an external repo, the files are copied into the pack repo at add time. The alternative, recording a URL and ref in `pack.yaml` and letting every consuming machine fetch the external repo at sync, was rejected: it multiplies network fetches by the number of skill sources, couples every sync to the availability of third-party repos, and moves skill bytes outside what the org's pack signature covers. Skills are instructions agents execute. The author reviewing the exact bytes before shipping them, and the org's existing signature covering them, is the point.

Vendoring keeps the consumer sync path unchanged. A vendored skill is an ordinary skill-dir artifact; consumers still fetch one repo and verify one signature.

## 2. Provenance in `sources.yaml`

A `sources.yaml` file at the pack root records, per vendored skill: source URL, subdir, requested ref, resolved commit, and the dir hash of the vendored copy. It is authoring metadata. Consumers never read it, so the consumer-facing schema is untouched, but it travels inside the pack repo, so the signed tag and `LockPack.Hash` cover it automatically.

Provenance deliberately does not live inside the skill directory. The vendored directory stays byte-identical to the upstream subdir, which keeps its `DirHash` verifiable against upstream, keeps update diffs clean, and avoids inventing a file that every walk of the directory would have to filter. The hardening cycle documented what happens when two walks filter differently; the way to not have that bug is to have nothing to filter.

## 3. Naming: upstream names survive

Today the engine hardcodes the on-disk name of every pack skill to `esc-<pack>-<dir>`. That prefix is collision-proof but renames the skill: a suite like superpowers, whose skills reference each other by canonical name, would break, and agents see unfamiliar names.

`skills:` entries in `pack.yaml` become a union type. A plain string keeps today's behavior exactly, prefix included, so existing packs and lockfiles see no change. An object entry `{path: skills/brainstorming, name: brainstorming}` sets the on-disk directory name explicitly, validated against the same name pattern as everything else. `add-skill` always writes the object form with the canonical upstream name.

Conflicts are first-class and fail closed:

- Plan fails with a clear error if two entries across all configured packs resolve to the same `.claude/skills/` path.
- `add-skill` refuses a name already present in the pack.
- If an unmanaged directory already occupies a target path on a consuming machine, sync skips it with a stderr warning and `esc status` reports it. Never silent adoption, with one carve-out: if the occupying directory's content hash exactly equals what escapement would have written, there is nothing to destroy, so sync records the lock entry and writes nothing to disk (a stderr notice, not a warning), which also lets a sync interrupted between the directory write and the lock save converge on retry instead of dead-ending forever. Anything short of an exact match is not adopted. `--force` does not override the skip case: overwriting a directory escapement never owned is destruction, not convergence. Adoption requires the user to move the directory aside.

## 4. Authoring commands

All run in the pack repo and reuse `internal/source` (cache, detached checkout, resolved commit) and the `DirFiles` walk (symlinks refused, `.git` skipped).

- `esc pack add-skill <url>[#subdir] [--ref vX.Y.Z] [--only name,...]` fetches the repo, discovers skill directories by the presence of `SKILL.md`, copies them under `skills/<name>/`, appends object entries to `pack.yaml`, and records provenance. A multi-skill repo vendors as a suite by default; `--only` narrows. The command prints what was vendored at which commit so the author reviews the diff before committing.
- `esc pack update-skill [name...|--all] [--ref <ref>]` re-fetches, defaulting to the highest semver tag, shows a diffstat, and rewrites the copy and provenance. When upstream has no tags at all, both `add-skill` and `update-skill` fall back to the default branch head and say so; the resolved commit is recorded either way, so the pin is exact even when the ref was loose. Divergence gate, mirroring sync's hand-edit rule: if the vendored copy no longer matches its recorded hash, the author edited it after vendoring, so the command warns and skips that skill unless `--force`. An edit made on purpose keeps reporting until someone resolves it, exactly as a hand-edited managed block does.
- `esc pack outdated [--check]` compares each pinned commit against upstream tags via the existing `LsRemoteTags`; `--check` exits 1 when anything is behind, for CI on the pack repo.

Consumers get updates through the normal pack version bump. Nothing about consumer-side trust changes.

## 5. Cross-level duplicates

A developer may already have a skill at the user level that the pack now mandates at the project level. The agent then sees two copies, possibly at different versions. Sync runs per machine, so escapement can only ever see the home directory of whoever runs it, and what it finds there is information, not drift.

`esc status` gains an informational finding when a managed project skill's name also exists in `~/.claude/skills/`, reporting whether the content matches. It never affects the exit code, the same stance as the `local` axis. Sync writes the project copy regardless and never touches `~/.claude` uninvited. The personal command surface (§7) warns in the other direction and offers `esc skill remove` as the dedupe path. The finding's payload is the skill name and a same/differs bit, both metrics-grade; no skill content is ever attached to it.

## 6. Agents directory

`pack.yaml` gains an `agents:` list of pack-relative `.md` files, synced as whole-file artifacts to `.claude/agents/<name>.md`. This reuses the existing `file` kind wholesale, including the hand-edit skip gate and lockfile handling. Pack-native only in this cycle; vendoring an agent from a URL is a natural later extension of the `add-skill` machinery. The path is Claude-specific; other tools' agent formats are out of scope until a real target needs one.

## 7. Personal user-level skills

`esc skill add <url>[#subdir] [--only name,...]`, `esc skill list`, `esc skill update [name|--all]`, `esc skill remove <name>`. These vendor directly into `~/.claude/skills/<name>` with state in `~/.escapement/skills.lock`, the existing lockfile format rooted at the home directory.

The engine's dir-sync and per-file manifest machinery is reused, so the invariants carry over: an existing unmanaged directory is never overwritten except the exact-hash-match adoption carve-out in §3, `remove` declines if the user added files inside the skill directory, `update` skips a hand-edited skill. There is no git undo at the user level, so `remove` states that and requires an explicit yes on a TTY or `--yes`, the same posture as `esc init`. Home-dir writes get the same containment rules as everything else: symlink refusal before any read, paths confined under `~/.claude/skills/`.

## 8. MCP version discipline

MCP curation already exists: `mcp.servers` in `pack.yaml` syncs to owned keys in `.mcp.json` and unowned entries survive. Vendoring does not map onto a few lines of JSON. What does map is version discipline: pack validation gains a warning when a server definition floats, an `@latest` tag or an npx package with no version pin. A warning, not an error; some servers are legitimately unversioned. Registry-mediated MCP discovery is deferred.

## 9. Build order

One spec, four phases, each with its own implementation plan:

1. Pack vendoring core: schema union, `sources.yaml`, `add-skill`/`update-skill`/`outdated`, conflict rules, cross-level duplicate finding.
2. Agents directory.
3. Personal user-level skills.
4. MCP float warning (small enough to ride along with phase 1 if convenient).

## 10. Future work: the non-admin portal

Recorded here as direction, to be designed in its own cycle. A non-admin portal view should let a team member see, and where permitted maintain, the pack and skill details for their projects. The strongest path makes the forge the identity and authorization system and git the write path:

- Authn is GitHub/Bitbucket OAuth. No portal password store.
- Authz is repo permissions. Read access to the pack repo grants the view; write access grants edit affordances. No escapement-side role system.
- A portal edit never mutates portal state or pushes to consumer repos. It becomes a branch, commit, and pull request on the pack repo under the user's own forge identity, reviewed like any change, reaching repos only via `esc sync`. This preserves the standing constraint that the control plane observes and does not distribute, and it means a compromised portal can at worst open pull requests, not silently change policy that agents execute.
- Refresh is pull-type: the view re-reads the repo on demand or on webhook, so viewing needs only read scope and no standing write credentials.

Open questions for that cycle: GitHub App versus OAuth app, Bitbucket and self-hosted GitLab parity, and the hosting story for a multi-user portal.
