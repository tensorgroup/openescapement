# Who does what: the CLI, the pack repo, and the portal

OpenEscapement has one binary, `esc`, and it wears three hats. This page says which hat does what, where it runs, and what a server adds. The short version:

- **Rules flow through git.** A pack is a versioned, signed tag in a pack repo. A governed repo pulls it with `esc sync`. No server sits in that path.
- **The CLI is the enforcement point.** Everything a developer needs, including status, sync, diff, and updating a pin, works offline in a single repo.
- **The portal is the aggregation point.** It receives reports that repos choose to send and turns many repos into one picture. It never pushes anything into a repo.

## How the pieces connect

```
  pack repo (git)                       governed repo N                    admin portal
  ───────────────                       ─────────────────                  ────────────
  pack.yaml, rules/, skills/            .escapement/config.yaml            esc serve
  esc pack add-skill / update-skill     esc sync   ──► CLAUDE.md,          fleet · packs
  git tag v1.2.0 (signed)   ───────►    esc status     AGENTS.md,          models · usage
                                        esc diff       .mcp.json,
                                        esc update     .claude/skills/
                                             │
                                             └── optional, redacted report ──►  POST /api/v1/events
```

Two arrows, two directions. Policy goes left to right through git. Telemetry goes right to bottom-right over HTTP, and only if the pack declares a reporting endpoint and the repo has a token.

## By role

| | Developer in a governed repo | Pack author in the pack repo | Admin across the fleet |
|---|---|---|---|
| **Runs** | `esc init`, `esc sync`, `esc status`, `esc diff`, `esc update`, `esc render` | `esc pack add-skill`, `esc pack update-skill`, `esc pack outdated`, then a signed git tag | `esc serve` and a browser |
| **Can** | Apply the org's rules to this repo. See drift between the lockfile and what is on disk. Keep local amendments alongside managed content. Bump a pack pin. Gate CI on `esc status --check`. | Vendor an external skill with provenance so the org signature covers exact bytes. Keep vendored skills current. Fail CI on the pack repo when anything is behind. Ship a new pack version by tagging. | See every registered repo, its pack pins, and its drift state in one table. Open one repo and see which artifacts are altered, stale, missing, or amended. Publish a pack version from the browser (validate, commit, tag). Read curated per-vendor model guidance and adopt rule fragments from it. Register a repo or a project by hand. |
| **Cannot** | Change the org's rules. Make the portal see this repo without a token and a declared endpoint. | Push rules into a repo. A tag reaches a repo only when someone runs `esc sync` there. | Push rules into a repo. See amendment text or diffs from a repo reporting at `metrics`. See anything at all from a repo reporting at `off`. |
| **Needs a server?** | No | No | Yes, this is the server |
| **Needs a network?** | Only to fetch the pack repo | Only to fetch upstream skills | Only for repos publishing to it |

## What only the server can do

A repo knows its own state and nothing else. These questions have no answer without something that sees across repos:

- Which repos are still on pack v1.2 after v1.3 shipped?
- Which repos have an altered managed block that `esc sync` is refusing to overwrite?
- Which teams have amended the org rules, and with what, when they report at `content`?
- Is drift growing or shrinking week over week?
- Which repos have gone quiet and stopped reporting?

That is the value of running `esc serve`: one table where each row is a repo and each column is a fact the CLI already computes locally. The portal adds no new enforcement. It adds visibility, and visibility is what a platform or security team is missing today.

## What the server never does

- **Never pushes rules.** Publishing from the portal produces a git tag. The tag reaches a repo the same way any other pack source does, when `esc sync` runs there.
- **Never sees prompt content.** Reports carry pack pins, artifact states, counts, hashes, and item names. Amendment text and diffs leave a repo only at the `content` level, which a pack must declare and a repo can always lower.
- **Never blocks a sync.** Publishing runs after the lockfile is written and exit codes are set. A portal outage queues to a local outbox and prints one line to stderr.
- **Never required.** Every CLI command works with no portal configured. A repo with no `reporting` block sends nothing, ever.

## Where the portal's features actually live

Two of the portal's three jobs do not need a server at all. It is worth being honest about this because it shapes what an admin CLI should look like.

| Portal feature | Needs the server? | Why |
|---|---|---|
| Fleet view and per-repo detail | Yes | The data arrives from many machines and lives in the portal's store |
| Pack publishing (validate, commit, tag) | No | It is a git operation against a local checkout of the pack repo |
| Model guidance and rule fragments | No | It is embedded markdown seeded to the data dir |

## Planned: admin from the CLI

Not built yet. Listed here so the boundary is drawn before the code is.

- `esc pack validate` and `esc pack publish`, run in the pack repo. The same validate, commit, tag sequence the portal performs, with no server involved. This is the natural home for the pack author's last step and makes the portal's publish page a convenience rather than the only path.
- `esc fleet [--json]`, run anywhere with `ESC_PORTAL_TOKEN` set. Asks the portal for the fleet table and prints it. This one genuinely needs the server, because that is where the data is.

The rule for adding an admin command: if the work is a git or filesystem operation, it belongs in the CLI first and the portal wraps it. If the work is a question about many repos, it belongs in the portal and the CLI queries it.
