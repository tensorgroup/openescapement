# Telemetry surface: publishing repo state to the admin portal

Status: approved design, 2026-08-05. Second of three specs in this cycle. Depends on the local amendment model (2026-08-05), whose `esc status --json` document is the input this spec transports. Build after it.

**As built (2026-08-09):** implementation deviated from this spec's sketch in five places, all adjudicated during the implementation cycle (see `.superpowers/sdd/2026-08-09-telemetry-surface/progress.md`):

- The ingest route is `POST /api/v1/events`, not `POST /ingest` as sketched in §5. The path-versioned route already existed, was already tested, and already carried the legacy raw-`store.Event` producers; the envelope path was added alongside it rather than opening a second endpoint.
- The operator sets the ingest token with `esc serve --token <token>`, overriding the default random per-start token, through the existing `withAuth` bearer/cookie/`?token=` gate. There is no parallel auth system for envelope ingest.
- Redaction at `metrics` also strips `Finding.Detail` and `Skipped.Reason`, not just `Amendment.Content` and `Alteration.Diff` as §2 originally scoped. Both are human prose populated in places directly from `err.Error()`, an unbounded channel for local error text; their structural content already ships machine-readable (`managed`/`local` states, `skipped[].cause`), so tightening the redaction boundary costs nothing at `metrics` and closes a leak.
- `Event.Drift` derivation (§6) is three-valued, not the boolean-shaped sketch implied there: `altered`/`missing`/`orphan` on any artifact maps to `drifted`; else `stale` on any artifact maps to `stale`; else `in-sync`. The existing rollup distinguishes `stale` from `drifted` and the migration promise was that rollups keep working, so collapsing `stale` into `drifted` would have silently broken that distinction.
- The ingest body cap is 1MB, not the 64KB the endpoint carried before this spec. The two body shapes (raw `store.Event` and a content-level `publisher.Envelope`, which can carry full amendment text and diffs) cannot be capped separately before the handler has read enough of the body to tell them apart, so one shared cap sized for the larger shape replaced the smaller one.

Motivation: the portal's event pipeline does not exist. `store.Event` is produced only by `internal/portal/seed/seed.go`, which is synthetic demo data. Nothing in `internal/cli` emits events and there is no ingest endpoint. Every dashboard in the portal today renders invented numbers. This spec builds the real path, end to end: a publisher in the CLI, an ingest endpoint on the portal, an extended event schema, and the views that show augmented versus unadulterated versus altered repos.

## 1. Endpoint declared by the pack

```yaml
reporting:
  amendments: content
  endpoint: https://esc.example.edu/ingest
```

`Reporting` (added in the amendment model spec) gains `Endpoint string`, mirroring `UpdateCheck.Endpoint` in both shape and stance. No endpoint means nothing is sent, whatever the level says. Policy about what leaves a repo travels the same versioned, signed channel as every other rule, and the unconnected open-source path stays silent by construction.

With multiple packs declaring endpoints, each configured endpoint receives a payload. Packs come from different sources and an org running two packs may run two portals; silently picking one would drop data with no diagnostic.

## 2. Redaction lives in the publisher

The amendment model spec establishes that local surfaces always report complete local truth, and that the effective reporting level governs only what leaves the repository. This spec is where that level is finally applied.

The publisher takes the `esc status --json` document and produces the wire payload:

- `off` or no endpoint: nothing sent.
- `metrics`: `Amendment.Content` and `Alteration.Diff` are dropped. Counts, hashes, states, and pack pins remain.
- `content`: sent whole.

Redaction is a single function over the document, unit-testable without a network, and asserted by a test that walks the marshalled payload and fails on any content field present below level `content`. That test is the enforcement point for the entire privacy posture, so it is written to fail loudly rather than to pass quietly.

## 3. Identity and auth

**Repo identity** is the normalized git remote URL plus the repo-relative path of the escapement config. Normalization strips credentials, trailing `.git`, and protocol differences, so `git@host:org/repo.git` and `https://host/org/repo` resolve identically. A repo with no remote sends its identity as the empty remote plus a local path and is bucketed as unregistered.

The portal registry maps remote to `repo_id`. **An unmatched remote is retained in an unregistered bucket, never dropped.** That bucket is the shadow-IT view the product spec asks for in §4: repos running org policy that nobody registered are exactly the population the portal exists to reveal. Dropping them would make the fleet look tidier than it is, which is the opposite of the product.

**Auth** is a bearer token from `ESC_PORTAL_TOKEN`. Environment only, never config, so it cannot be committed. A missing token with an endpoint configured is a warning on stderr and a skipped publish, never a failed sync.

## 4. Publishing is always non-fatal

Publish runs after the lockfile is written and after all output is printed. A telemetry outage must never be able to break a policy sync, so:

- Short timeout, no retries in-process.
- Failure never changes the exit code and never suppresses normal output.
- Unsent payloads append to `.escapement/outbox.jsonl`, which flushes oldest-first on the next successful publish.
- The outbox is capped by count and by age. Beyond the cap the oldest entries are dropped and the drop is reported on stderr, because a silently truncating queue reads as healthy telemetry while losing data.
- `.escapement/.gitignore` gains `outbox.jsonl`, alongside the `update-log.jsonl` entry `esc init` already writes.

`esc sync` and `esc status` both publish. Status publishing is what keeps a repo's state fresh in the portal without requiring a policy change to trigger it.

## 5. Ingest

`POST /ingest` on the portal server. Validates the bearer token, validates the payload against the schema version, resolves repo identity against the registry, and appends to the existing `events.jsonl` through the current store. Rejects with 401 on a bad token and 400 on an unparseable or wrong-version payload, and returns 202 otherwise.

Token configuration is server-side, a value the operator sets when starting `esc serve`. This is self-hosted infrastructure inside an org's own trust boundary; per-repo credentials and rotation are out of scope for this pass and noted in the product spec as follow-on work.

Transport is stdlib `net/http` on both ends. No dependency change.

## 6. Event schema

`store.Event` gains `Artifacts []EventArtifact`, carrying per-artifact `path`, `kind`, `managed`, `local`, and the amendment and alteration payloads at whatever level survived redaction. It also gains a `collection` record of the effective reporting level and its source, so a view can distinguish "no amendments" from "amendments withheld by repo override". Those are very different facts and must never render identically.

`Event.Drift` becomes a value derived from the artifact array rather than a field the sender chooses. Existing rollups keep reading it, so `rollup.go` and the charts continue to work through the migration.

`seed.go` must produce the new shape. The demo is the portal's only development surface, and a seed that emits pre-migration events would leave every new view untestable and the existing views rendering blanks. Seed data gains repos across all four states, including one with a repo-level override withholding content, so the withheld case has a rendering to develop against.

## 7. Portal views

- **Fleet table** gains a state column driven by the cross product: unadulterated, augmented, altered, ungoverned. Augmented is not styled as a problem, because it is not one. Altered is the state that draws attention.
- **Repo detail** lists artifacts with their two axes, amendment size, and, at level `content`, the amendment text and the alteration diff. When content is withheld, the view says so and names the source of the withholding.
- **Unregistered bucket** lists repos reporting from unrecognized remotes, with a path to register them into the tree.

## 8. Testing

- Redaction table across all three levels, asserting on the marshalled payload rather than on struct fields, including the walk-and-fail test from §2.
- Identity normalization: SSH and HTTPS forms of the same remote resolve identically; credentials stripped; no-remote repos bucketed as unregistered.
- Publisher: success, connection refused, timeout, 401, and 500, each asserting exit code unchanged and normal output intact.
- Outbox: failed publish enqueues; next success flushes oldest-first; cap eviction drops oldest and reports it; a corrupt outbox line is skipped rather than fatal.
- Ingest: valid payload appended and readable through the store; bad token 401; wrong schema version 400; unregistered remote retained in the bucket.
- End to end over a real temp repo and a real portal on a loopback port: amend a file, sync, assert the event lands with the expected states, then repeat with a repo-level clamp and assert content absent.
- Multi-pack: two endpoints each receive a payload.

## 9. Docs

- `AGENTS.md`: the publish path, its non-fatal invariant, and the redaction test as the enforcement point for the privacy posture.
- Product spec: §5 gains the transport model; §7 records that content collection is self-hosted-only for now and that per-repo credentials are follow-on work.
- README: `ESC_PORTAL_TOKEN`, the outbox, and what a repo sends at each reporting level.
- CHANGELOG bullet. No em-dashes.
