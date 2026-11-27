# OpenEscapement — reference

*Written 2026-11-24: these sections moved from the README, word for word, so the README can stay a short
getting-started guide. The README links here. Headings are one level up from the README's. Three
facts were corrected in the move after a review against the code: the portal has five pages, `esc status`
prints `+N lines local` (the portal says augmented), and `skipped[].cause` has a fifth value.*

## Contents

- [Adding `esc` to a repo that already has instruction files](#adding-esc-to-a-repo-that-already-has-instruction-files)
- [`esc serve` — the admin portal](#esc-serve--the-admin-portal)
- [Local amendments](#local-amendments)
- [Publishing to the admin portal](#publishing-to-the-admin-portal)
- [JSON output (`--json`)](#json-output---json)
- [The GitHub Action](#the-github-action)
- [Targets](#targets)
- [Custom targets](#custom-targets)

## Adding `esc` to a repo that already has instruction files

Most first contact isn't a blank repo, it's one with a hand-written `CLAUDE.md` or
`AGENTS.md` already in place. `esc init` handles that case explicitly:

- **Detects** what's already there (`CLAUDE.md`, `AGENTS.md`, `GEMINI.md`,
  `GOVERNANCE.md`, `.mcp.json`, `.claude/skills/`, `.agents/skills/`) and pre-fills
  `targets` in the generated config with exactly what it found, plus `rules`: a
  newly governed repo should still receive rule files by default, so an explicit
  targets list always includes it even when nothing on disk implied it. A repo
  with `AGENTS.md` and no `GEMINI.md` doesn't get `GEMINI.md` conjured into
  existence by its first sync. Nothing detected leaves `targets` empty, which
  still means all targets.
- **Explains**, in the same vocabulary `esc status` uses, what the first `esc sync`
  will do to each detected file: your content is preserved byte for byte and
  reported as a local amendment. A file that already carries a managed block or the
  placeholder marker is told that instead of promised an insert it won't get.
- **Offers**, once, to place the managed-block marker (`<!-- escapement:block -->`),
  on a TTY. One question, defaulting to no: say no and nothing is written, and the
  block lands at the top of each file, below any frontmatter and title, at first
  sync. Say yes and you pick a position once, above the title (but still below any
  frontmatter) or at the end of the file, and it applies to every eligible file.
  Declining, an empty answer, Ctrl-D, and anything unrecognized all leave every file
  untouched; the placeholder is the only way to override where the block lands.

`esc init` never writes rendered policy, only the marker, and only when you ask it
to. A getting-started command shouldn't put rules into your instruction files before
you've seen them; `esc sync` is what writes policy, later, once you've reviewed the
pack.

Guard rails: init never writes over a file with uncommitted changes, because git is
what would restore it; that file is reported and left out of the offer, and the rest
are still offered. In a directory that is not a git repository there is no undo at
all, so the prompt says so and still requires an explicit yes rather than treating
"no version control" as the safest case. A non-interactive run, including
`esc init --yes`, never asks and never writes anything beyond the config scaffold.

Once a pack is synced, run its `esc-reconcile` skill, if it ships one (the seeded
demo pack does), to compare your existing rules against the pack's for duplicates or
contradictions. That comparison is judgment work for an agent: a CLI heuristic would
be wrong often enough to erode trust, and a model call would break `esc`'s
no-network, single-dependency design, so the guidance ships as a skill instead of
code in `esc` itself.

## `esc serve` — the admin portal

For what the CLI, the pack repo, and the portal each do, and what a server adds,
see [docs/cli-and-portal.md](cli-and-portal.md).

```sh
esc serve --demo               # one binary, seeded fictional org, no setup
```

Five pages: an overview of fleet posture, a fleet list of registered repos and their
drift status, rule-pack browsing with portal-side publishing (validate → commit → tag),
Models (vendor guidance and fragments to adopt; see below), and usage. The portal never pushes rules into repos — packs it publishes still reach a
repo only when someone runs `esc sync` there, same as any other pack source.

`esc serve --demo` resets its example data to pristine on every startup and prints `esc: demo data reset`, so the demo always shows current content. Anything you change during a demo (published pack versions, adopted starters, guidance edits) is discarded when you restart. Non-demo servers keep your data and only refresh guidance files you have not edited.

Outside `--demo`, `esc serve` prints a fresh random bearer token on every start unless you pass `esc serve --token <token>` to set it explicitly (the same token repos need in `ESC_PORTAL_TOKEN` to publish to this server's ingest endpoint). `--demo` always disables auth, regardless of `--token`.

The portal is server-rendered with a strict CSP; its one third-party asset, htmx, is
vendored and embedded (pinned by SHA-256 in `internal/portal/web/HTMX-VENDOR.md`), never
fetched at runtime, and every interaction still works with JavaScript disabled.

The portal's **Models** section is curated, sourced guidance on each vendor's models (Anthropic, OpenAI, Google, Kimi, Deepseek, Grok): what each model class is good at, which roles it suits (planning, review, coding, bulk), how to govern it, and example rule-pack fragments to adopt. Every claim carries a source. The guidance ships embedded and is seeded to `<data-dir>/guidance/` on first run. Files you have not edited are refreshed to the latest embedded content on later runs, so upgrades reach you automatically; any file you edit by hand or from the portal is never overwritten. To restore a shipped file, delete it and restart. Raw HTML in guidance or fragment markdown is dropped by the safe renderer; write markdown.

## Local amendments

Content escapement does not own (text you add outside a managed block, extra
`.mcp.json` server entries, files you add to a skill directory) is never
touched by sync, and is reported on a second, independent axis: `local: none`
or `local: amended`. An amendment alone is never drift and never changes an
exit code. `esc status` shows your own additions next to `in sync` (for example
`+11 lines local`); the admin portal calls that state **augmented**. That's expected
use, not a problem to fix.

`esc sync` leaves an artifact alone when its managed region was hand-edited,
instead of overwriting it: it warns on stderr, records the skip, and still
exits 0. **Exit 0 from `esc sync` no longer asserts the repo matches
policy**: it asserts that everything escapement was willing to apply was
applied. Gate compliance on `esc status --check`, which exits 1 on any
artifact that is not in sync. Run `esc sync --force` to overwrite a
hand-edited managed region and converge; `--force` only touches content
escapement owns and never overwrites a local amendment.

If a pack manifest declares `reporting: { amendments: metrics, endpoint:
https://... }` (or `amendments: content`), a repo publishes local amendments
to that endpoint at that level on every `esc sync` and `esc status`; a
repo's own `.escapement/config.yaml` can lower the level via
`report_amendments: metrics` or `off`, but never raise it above what the
pack declared. With no `reporting` block, or a `reporting` block with no
`endpoint`, nothing is ever sent anywhere. This governs only what leaves the
repo: your own `esc status` always shows your amendments in full, including
content, on your own machine, regardless of the resolved level; see JSON
output below.

### Publishing to the admin portal

Set `ESC_PORTAL_TOKEN` in the environment (never in config, so it can never
be committed) to authenticate publishes. Publishing is always non-fatal: it
runs after the lockfile is written and all normal output is printed, so a
telemetry outage never changes a command's exit code or suppresses its
output. An unset token with an endpoint configured is one stderr warning and
a skipped publish, not a failure. Anything that could not be sent queues to
the gitignored `.escapement/outbox.jsonl` (capped at 200 entries or 14 days,
oldest evicted first, every drop reported on stderr) and flushes oldest-first
on the next successful publish.

What leaves the repo depends on the resolved `reporting.amendments` level:

| Level | What leaves the repo |
|---|---|
| `off` (default, or no `reporting` block) | Nothing. Nothing is sent, ever. |
| `metrics` | Repo identity, pack pins, and every artifact's `managed`/`local` state, counts, hashes, and item names (e.g. which `.mcp.json` keys or skill-directory files were added). Never amendment content, alteration diffs, or free-text detail/reason prose. |
| `content` | Everything `metrics` sends, plus full amendment text and alteration diffs. |

## JSON output (`--json`)

`esc status --json` and `esc sync --json` print one JSON document (schema 1)
to stdout instead of human-readable output: the contract a separate
consumer (a portal, a dashboard, a script) can parse without reading Go
structs:

| Field | Type | Meaning |
|---|---|---|
| `schema` | int | Document schema version |
| `command` | string | `"status"` or `"sync"` |
| `packs[]` | array | One entry per pinned pack: `source`, `ref`, `pinned` (resolved content hash), `latest` (omitted if unknown), `signed` |
| `findings[]` | array | Every classified artifact, pack pin, or subsystem signal (see below) |
| `collection` | object | `amendments` (`off` / `metrics` / `content`) and `source` (`default` / `pack` / `repo-override`): the resolved reporting level and where it came from |
| `skipped[]` | array | `sync` only, always present (possibly `[]`); omitted entirely for `status`. What sync declined to write or remove: `subject`, `kind`, `cause`, `reason`, `expected_hash`, `actual_hash`. `cause` is the machine-readable discriminator (`hand-edited`, `orphan-dir-unmanaged`, `orphan-dir-edited`, `orphan-block-edited`, `unmanaged-dir-at-target`, `unmanaged-file-at-target`, `orphan-file-edited`); `reason` is prose for a human and nothing should branch on it |

Each `findings[]` entry carries `subject` (an artifact path, a pack source
URL, or the literal `"update-check"`), `kind` (`block` / `file` / `dir` /
`json-keys` for artifacts, or `pack` / `update-check` / `constraint`),
`managed` (the drift state), `local` (`none` / `amended`), `detail`, and
optional `amendment` / `alteration` objects.

Two things worth knowing before you parse this:

- **`subject` is not unique within `findings[]`.** A constraint-violation
  finding reuses an artifact's path, so key on `(subject, kind)`, not
  `subject` alone.
- **`--json` skips the interactive update check** on both commands, which
  for `status` also skips the update-log append: a consumer polling only
  `esc status --json` will never see a pack's `latest` field advance.
  `esc sync --json` still records a check entry every time, since sync
  always calls the recorder regardless of `--json`.

`amendment.content` and `alteration.diff` are both omitted for `.mcp.json`
(kind `json-keys`) and for skill directories (kind `dir`): a whole-file
`.mcp.json` diff would ship a user's own *unowned* MCP server entries into a
document a publisher forwards upstream, and a directory's alteration hashes
are a whole-tree pair with no single-file diff to show. Local surfaces
(`esc status`, `esc status --json`, `esc sync --json`) always report
complete local truth, amendment content included, no matter what the
resolved reporting level is. The level governs only what a publisher sends
onward, never what your own machine shows you.

See `docs/superpowers/specs/2026-08-05-local-amendment-model-design.md` §5
for the full field-level rationale.


## The GitHub Action


There's also a GitHub Action:

```yaml
- uses: tensorgroup/openescapement@main
  # runs `esc status --check` — policy drift fails the build
```



## Targets

`rules` (path-scoped files under `.claude/rules/`) and `agents-skills` (a second skills root under `.agents/skills/`) are built-in targets. `rules` is part of the default set: an empty `targets:` list renders every built-in except `agents-skills`, which is opt-in only. An explicit `targets:` list is an exhaustive filter, not an addition to the defaults, so a repo that sets one must list `rules` to keep receiving rule files and `agents-skills` to render the second skills root. A pack that ships a `targets: [rules]` fragment needs a repo running an `esc` new enough to know that target; an older binary rejects the fragment as an unknown target and exits 1.

A rule fragment can opt into `targets: [rules]` to render as its own file under `.claude/rules/` instead of joining the shared managed block, and scope itself to part of the tree with `paths:`. See `docs/pack-authoring.md` for the full authoring reference.

## Custom targets

A pack may define extra managed-block markdown targets in `pack.yaml` under
`custom_targets`, so an org can govern files like
`.github/copilot-instructions.md` or `QWEN.md` without waiting for an esc
release:

```yaml
custom_targets:
  - name: copilot
    file: .github/copilot-instructions.md
    doc: https://docs.github.com/en/copilot/customizing-copilot
    description: Repository custom instructions for GitHub Copilot.
```

`name` matches `^[a-z][a-z0-9-]{0,31}$` and is used in fragment `targets:`;
`file` is a clean relative `.md` path of at most two segments, ASCII only, not
in a control directory. A fragment reaches a custom target only when it names
it explicitly; a fragment with no `targets:` goes to built-in files only.

**Single owner:** a custom target belongs to the one pack that defines it.
Only that pack's fragments render into it. Two packs declaring the same name
or file is an error and sync refuses. If an org and a team need to co-write
one file, put the fragments in the same pack.

A custom target renders only if its file is listed in the repo's
`.escapement/config.yaml`:

```yaml
allow_custom_target_files:
  - .github/copilot-instructions.md
```

Pinning a pack grants it write access to a known, fixed set of files. Custom
targets would let a pack choose new paths, so the acknowledgment list keeps a
repo's write surface enumerable from the repo's own config. Filename rules
alone are not enough because markdown transcludes (one instruction file can
pull in another), so a file the pack does not name directly could still change
what an agent reads. Without the acknowledgment, sync fails closed and names
the file to add.

Recommended layout: an org base pack plus team packs, each able to contribute
its own custom targets, with the repo acknowledging every file it wants
written:

```yaml
packs:
  - {source: git@github.com:acme/esc-org-base, ref: v2.1.0}      # org floor
  - {source: git@github.com:acme-payments/esc-team, ref: v0.3.0} # team additions
targets: []                                                       # optional filter
allow_custom_target_files:
  - .github/copilot-instructions.md                               # org base's target
  - PAYMENTS-AGENTS.md                                            # team's target
```
