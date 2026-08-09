# Telemetry Surface Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The real CLI-to-portal event path from `docs/superpowers/specs/2026-08-05-telemetry-surface-design.md`: a redacting, non-fatal publisher in `esc sync`/`esc status`, an evolved ingest endpoint, the extended event schema, and portal views that distinguish unadulterated, augmented, altered, and withheld.

**Architecture:** A new stdlib-only `internal/publisher` package owns identity normalization, redaction, the outbox, and the HTTP client; `internal/cli` wires it after lockfile write and output. The portal's EXISTING `POST /api/v1/events` handler evolves to accept the versioned wire envelope, resolve repo identity against the registry, and retain unregistered remotes. `store.Event` gains artifact and collection records; `Drift` becomes derived; seed and views migrate together.

**Tech Stack:** Go 1.24, stdlib only (net/http both ends, per spec §5).

## Global Constraints

- Single external dependency policy: stdlib only. No new deps anywhere in this plan.
- **Publish is always non-fatal** (spec §4): it runs after the lockfile is written and after all normal output; it never changes an exit code, never suppresses output, has a short timeout and no in-process retries. This is invariant-grade; every publisher test asserts exit code and output unchanged on failure.
- **The redaction walk test is the privacy enforcement point** (spec §2): it walks the MARSHALLED payload and fails on any content-bearing field below level `content`. Never weaken it; extending the Report type later must extend it.
- Sentinel errors map to exit codes (0/1/2/3/4) as in AGENTS.md; nothing here adds a new exit path.
- Determinism: seed stays deterministic (fixed rand seed, epoch parameter). Golden tests in internal/cli must pass without update flags unless a task explicitly regenerates them.
- `go test ./... && go vet ./... && gofmt -l .` clean before any task is done. No em-dashes in new prose or rendered output.
- COMMITS: per-task, message drafted from the diff, repo style, no AI trailers.
- Adjudicated deviations from the spec text (planning-time, Fable 5, do not re-litigate; the docs task records both in the spec):
  1. The ingest route stays `POST /api/v1/events` (exists, tested, path-versioned) instead of the spec's `POST /ingest` sketch.
  2. The portal already gates that route with its session token via `withAuth`; the operator-set ingest token is `esc serve --token <value>` overriding the random default, not a new parallel auth system.
- Chosen constants (spec left them open; state once, use everywhere): publisher timeout 5s; outbox cap 200 entries; outbox max age 14 days; wire schema version 1.

## Wire format (used by Tasks 4, 5, 7; defined once)

```go
// internal/publisher/publisher.go
// Envelope is the versioned wire payload POSTed to each configured
// endpoint. The portal validates Schema before anything else.
type Envelope struct {
	Schema     int            `json:"schema"` // 1
	Remote     string         `json:"remote"` // normalized; "" when the repo has no remote
	ConfigPath string         `json:"config_path"`
	Report     *engine.Report `json:"report"`
}
```

---

### Task 1: Pack manifest gains `reporting.endpoint`

**Files:**
- Modify: `internal/pack/pack.go` (`Reporting` struct ~:41; validation near the `UpdateCheck.Endpoint` https check ~:189)
- Test: the pack package's existing manifest-validation test file (find the UpdateCheck.Endpoint cases and sit beside them)

**Interfaces:**
- Produces: `Reporting struct { Amendments string; Endpoint string \`yaml:"endpoint"\` }`. Validation mirrors UpdateCheck.Endpoint exactly: empty is fine; non-empty must be `https://` (reject `http://` and anything else) with the same error style. Task 5 reads `p.Manifest.Reporting.Endpoint` from every loaded pack.

- [ ] Write failing tests: valid https endpoint accepted; `http://` rejected; empty accepted; message style matches the UpdateCheck case.
- [ ] Run, observe failures; implement field + validation; run; full check; commit `feat(pack): reporting.endpoint declares where a repo publishes`.

---

### Task 2: `internal/publisher` identity

**Files:**
- Create: `internal/publisher/identity.go`, `internal/publisher/identity_test.go`

**Interfaces:**
- Produces:
  - `func NormalizeRemote(raw string) string` — pure. Rules (closed set): trim space; strip `ssh://`, `https://`, `http://`, `git://` schemes; strip userinfo (`user@`, `user:pass@`); convert scp-like `git@host:org/repo` to `host/org/repo`; lowercase the host segment only; strip one trailing `.git` and any trailing `/`. Result form `host/org/repo`; empty input returns "".
  - `func RepoRemote(ctx context.Context, root string) string` — `git -C root config --get remote.origin.url` via os/exec with `LC_ALL=C` (mirror internal/cli's gitCommand pattern; this package keeps its own tiny copy, unexported, since internal/cli's is unexported and the CLI package must not be imported from here). Any git failure returns "" (no-remote bucket), never an error: identity is best-effort by design.

- [ ] Failing table test: `git@host.edu:Org/Repo.git`, `https://user:tok@host.edu/org/repo/`, `ssh://git@host.edu/org/repo.git`, `http://host.edu/org/repo` all normalize to `host.edu/org/repo` (case: `Org/Repo` keeps its case, host lowercases); `""` stays `""`; garbage without host/path shape passes through stripped rather than erroring.
- [ ] Implement; verify; a small `RepoRemote` test with a real temp git repo (set and read a remote; and a repo with none returns ""). Full check; commit `feat(publisher): normalized repo identity`.

---

### Task 3: Redaction

**Files:**
- Create: `internal/publisher/redact.go`, `internal/publisher/redact_test.go`

**Interfaces:**
- Produces: `func Redact(rep *engine.Report, level string) *engine.Report` — returns a DEEP COPY (marshal/unmarshal round trip through engine.Report is the simplest honest deep copy and is acceptable); never mutates the input. `engine.ReportOff` (or no endpoint) is handled by the caller not publishing at all; Redact handles `ReportMetrics` (every `Amendment.Content` and every `Alteration.Diff` emptied, `omitempty` drops them from the wire) and `ReportContent` (returned whole).

- [ ] **The walk test, verbatim intent (spec §2):** marshal the redacted Envelope for a Report fixture that carries a populated `Amendment.Content` and `Alteration.Diff` in several findings (block, dir, json-keys kinds), then walk the decoded `map[string]any` recursively and fail on ANY key named `content` or `diff` with a non-empty value when level is `metrics`. At `content`, assert both survive verbatim. The walker is generic over the JSON tree precisely so a future Report field named `content` fails this test until someone consciously extends the redactor.
- [ ] Also assert: counts/hashes/states/pack pins survive at `metrics`; input Report unchanged after Redact (assert on a deep-equal snapshot).
- [ ] Implement; verify; full check; commit `feat(publisher): redaction is the privacy enforcement point`.

---

### Task 4: Outbox

**Files:**
- Create: `internal/publisher/outbox.go`, `internal/publisher/outbox_test.go`
- Modify: `internal/updatecheck/log.go` `ensureGitignore` (~:128) so `.escapement/.gitignore` covers `outbox.jsonl` as well — read how init/update-log use it first and extend in the same style.

**Interfaces:**
- Produces:
  - `const OutboxCap = 200`, `const OutboxMaxAge = 14 * 24 * time.Hour`
  - `func AppendOutbox(root, endpoint string, e Envelope, now time.Time) (dropped int, err error)` — appends `{"ts":..., "endpoint":..., "envelope":...}` as one JSON line to `.escapement/outbox.jsonl` (the endpoint URL rides with the entry so a flush knows where each goes), then enforces caps: count over `OutboxCap` or age over `OutboxMaxAge` drops OLDEST entries; every drop is counted and returned so the caller reports it on stderr.
  - `func FlushOutbox(root string, send func(endpoint string, e Envelope) error, now time.Time) (sent, remaining int, err error)` — oldest-first; stops at the first send failure (leaving the rest); a corrupt line is skipped and counted, never fatal.

- [ ] Failing tests: append grows the file; cap eviction drops oldest and reports the count; age eviction; flush oldest-first order (send-func records order); flush stops on failure leaving the tail; corrupt line skipped; empty/missing file is a no-op.
- [ ] Implement (O_APPEND for the hot path; rewrite via temp+rename only when evicting or flushing, matching the repo's atomic-write discipline); verify; extend `ensureGitignore` + its existing test; full check; commit `feat(publisher): outbox holds unsent payloads, capped and loud about drops`.

---

### Task 5: HTTP client and CLI wiring

**Files:**
- Create: `internal/publisher/publisher.go` (Envelope from the header above, client, orchestration), `internal/publisher/publisher_test.go`
- Modify: `internal/cli/cli.go` — three call sites: `syncJSON` (~:334), `syncOnce` (~:397), `cmdStatus` (~:476), each AFTER lockfile write, RecordSync, and all output

**Interfaces:**
- Produces:
  - `func Publish(ctx context.Context, root string, rep *engine.Report, packs []*pack.Pack, coll engine.Collection, stderr io.Writer, now time.Time)` — no error return AT ALL; the signature is the non-fatality invariant. Behavior: collect every distinct `Reporting.Endpoint` across packs (each configured endpoint receives the payload, spec §1); if none, return; if `coll.Amendments == engine.ReportOff`, return; token from `os.Getenv("ESC_PORTAL_TOKEN")`, missing token with endpoints configured prints ONE stderr warning and returns; build Envelope (identity from Task 2, report through `Redact` at `coll.Amendments`); for each endpoint POST JSON with `Authorization: Bearer <token>`, `Content-Type: application/json`, 5s timeout client; on success flush the outbox to that endpoint; on failure append to the outbox and print one stderr line; eviction drops reported on stderr.
  - HTTP acceptance: 2xx is success (the portal returns 202); anything else is failure.
- [ ] Failing tests with `httptest.Server`: success posts the envelope (assert body shape and bearer header); connection refused, timeout (server sleeps), 401, 500 each: exit-path unaffected (Publish returns normally), payload lands in the outbox, stderr carries one line; next success flushes oldest-first (assert server receives outbox entries before the fresh one, in order); two packs with two endpoints both receive the payload; `off` sends nothing; missing token warns and skips.
- [ ] Wire the three CLI call sites; an integration test through `runEscOut` with a governed repo whose pack declares an endpoint at a loopback httptest server: `esc sync` exit code and stdout identical with the server up, down, and erroring (assert against a no-endpoint control run).
- [ ] Full check; commit `feat(cli): sync and status publish to configured endpoints, never fatally`.

---

### Task 6: Event schema and derived drift

**Files:**
- Modify: `internal/portal/store/store.go` (Event ~:61), `internal/portal/store/rollup.go`
- Test: store package tests beside the existing ones

**Interfaces:**
- Produces:
  - `type EventArtifact struct { Path, Kind, Managed, Local string; Amendment *engine.Amendment \`json:",omitempty"\`; Alteration *engine.Alteration \`json:",omitempty"\` }` (importing engine's types keeps one definition of the amendment payload; store already lives server-side where engine is importable).
  - `Event` gains `Artifacts []EventArtifact \`json:"artifacts,omitempty"\`` and `Collection *engine.Collection \`json:"collection,omitempty"\``.
  - `func DeriveDrift(arts []EventArtifact) bool` (match `Event.Drift`'s current type; read it first) — true iff any artifact's Managed is not `in-sync`. Ingest and seed SET Event.Drift from this; rollups keep reading the field unchanged (spec §6's migration promise).
  - `func FleetState(arts []EventArtifact) string` — cross product per spec §7: `altered` if any Managed == altered; else `augmented` if any Local == amended; else `unadulterated`. (`ungoverned` is the no-events case decided in rollup, not here.)
- [ ] Failing tests for DeriveDrift and FleetState across the four states incl. the both-axes repo (amended AND altered => altered wins the state, amendment still visible in detail). Old-shape events (no artifacts) must still unmarshal and roll up exactly as before: a JSONL fixture line from the current seed format decodes with nil Artifacts and untouched Drift.
- [ ] Implement; verify rollup.go output unchanged for artifact-less events (existing rollup tests must not change); full check; commit `feat(store): events carry artifacts and collection; drift is derived`.

---

### Task 7: Ingest evolves

**Files:**
- Modify: `internal/portal/web/ingest.go`, `internal/portal/web/server.go` (route stays `POST /api/v1/events`), `internal/portal/store/store.go` (registry lookup), `internal/cli/serve.go` (`--token` flag)
- Test: `internal/portal/web/ingest_test.go` (extend)

**Interfaces:**
- Consumes: `publisher.Envelope` (portal imports internal/publisher for the type and `NormalizeRemote`; publisher must not import portal — check the direction compiles), Task 6's types.
- Produces:
  - The handler accepts BOTH shapes: a raw `store.Event` (existing behavior, existing tests keep passing) and an Envelope, distinguished by the presence of a top-level `"schema"` key. Envelope path: `Schema != 1` => 400 with a body naming the expected version; resolve `Remote` via a registry lookup (`Registry` gains whatever remote-to-repo mapping it lacks; read `store.Registry`'s real shape first and extend minimally, persisting via the existing atomic `SaveRegistry`); a matched remote stamps the registered `RepoID`/`TeamID`; **an unmatched remote is retained**: `RepoID` empty, the normalized remote stored on the event (add `Event.Remote string \`json:"remote,omitempty"\``), never dropped (spec §3).
  - Event built from the envelope: `Kind` = `report.Command` (already valid kinds), `Packs` from `report.Packs`, `Artifacts` from `report.Findings` (artifact-kind findings only; pack/update-check/constraint findings are not artifacts), `Collection` from `report.Collection`, `Drift` = `DeriveDrift`, TS stamped server-side as today.
  - `esc serve --token <value>`: overrides the random token (non-demo); empty keeps today's behavior. Usage line updated.
- [ ] Failing tests: valid envelope => 202, event readable through the store with expected states; schema 2 => 400; bad token => 401 (already covered, extend for envelope); unregistered remote retained with remote set and empty RepoID; registered remote resolves; legacy raw-Event body still 202 (regression); `--token` gates the route.
- [ ] Implement; verify; full check; commit `feat(portal): ingest speaks the versioned envelope and keeps the unregistered`.

---

### Task 8: Seed produces the new shape

**Files:**
- Modify: `internal/portal/seed/seed.go`
- Test: seed's existing tests plus assertions on the new fields

**Interfaces:** consumes Task 6's types; produces demo data every later view task develops against.

- [ ] Extend the deterministic generator so governed repos emit Artifacts + Collection: distribute the 28 governed repos across unadulterated, augmented, altered (both-axes included), and give at least one repo `Collection{Amendments: "metrics", Source: "repo-override"}` with amendments whose Content is absent (the withheld case, spec §6); ungoverned stays event-less. `Drift` set via `DeriveDrift`. Same rand seed; existing counts (6 depts / 15 teams / 40 repos) unchanged.
- [ ] Assert determinism (two runs identical), the four states all present, the withheld repo present; existing seed tests keep passing (extend, never delete).
- [ ] Full check; commit `feat(seed): demo repos cover all four states and the withheld case`.

---

### Task 9: Portal views

**Files:**
- Modify: `internal/portal/store/rollup.go` (`FleetRow` gains the state), `internal/portal/web/server.go` + `templates/` (fleet column, repo detail page or section, unregistered bucket), following the existing layout/pageNames/renderFragment conventions and the degrade rule (every route full-renders without HX; fragments require HX-Request; `degrade_test.go` enforces this, keep it passing)
- Test: web package tests in the established `newTestServer*` style

**Interfaces:** consumes FleetState, seed data.

- [ ] Fleet table: state column with values unadulterated | augmented | altered | ungoverned; augmented styled neutrally, altered attention-styled (class names only; no new CSS framework). Assert the column renders each state from seeded events.
- [ ] Repo detail: per-artifact rows showing both axes, amendment size (Bytes/Lines/Items), and at level content the amendment text and alteration diff; when withheld, the exact copy pattern `content withheld by <source>` naming `repo-override` or `pack`. Assert both the content case and the withheld case render, and that "no amendments" and "amendments withheld" NEVER render identically (spec §6's must).
- [ ] Unregistered bucket: a section or page listing events with empty RepoID grouped by remote, with a register affordance into the existing registry (follow whatever registry-editing surface exists; if none exists, listing plus documented manual registry edit is the v1, noted in the report).
- [ ] Full check incl. degrade test; commit `feat(portal): fleet states, artifact detail, and the unregistered bucket`.

---

### Task 10: End to end

**Files:**
- Create: `internal/cli/telemetry_e2e_test.go`

- [ ] The spec §8 scenario over real parts: temp governed repo (existing fixtures) whose pack declares `reporting: {amendments: content, endpoint: <loopback>}`; real portal server on loopback (the `serve` wiring or `web.New` + `httptest`); amend a file; `esc sync`; assert the event lands with augmented state and amendment content present. Then repo-level clamp to `metrics` and assert content absent and Collection says repo-override. Then two packs, two endpoints, both receive.
- [ ] Full check; commit `test(cli): the telemetry path end to end`.

---

### Task 11: Docs

**Files:**
- Modify: `AGENTS.md`, `ai-governance-product-spec.md` (§5 transport, §7 self-hosted-only + per-repo credentials follow-on), `README.md`, `CHANGELOG.md`, `docs/superpowers/specs/2026-08-05-telemetry-surface-design.md` (record the two adjudicated deviations: route name, serve --token)

- [ ] AGENTS.md gotchas: the publish path is always non-fatal (exit codes and output never change on telemetry failure); the redaction walk test is the enforcement point for the privacy posture; the outbox caps and reports drops.
- [ ] README: `ESC_PORTAL_TOKEN`, the outbox file, what leaves the repo at each level (table: off/metrics/content).
- [ ] CHANGELOG bullet(s), no em-dashes. Product spec edits per spec §9.
- [ ] Full check; commit `docs: the telemetry surface, its privacy posture, and its knobs`.

## Self-Review Notes

- Spec coverage: §1→T1, §2→T3, §3→T2+T7 (+serve --token), §4→T4+T5, §5→T7, §6→T6+T8, §7→T9, §8→each task's tests + T10, §9→T11. Both spec deviations are adjudicated in Global Constraints and documented by T11.
- Import direction risk (T7): portal importing internal/publisher for Envelope/NormalizeRemote — publisher imports engine+pack only, so no cycle; the T7 implementer verifies before building.
- The legacy raw-Event ingest path is kept deliberately (existing tests, demo tooling); retiring it is future work, not scope.
- Order matters: T6 before T7/T8/T9; T1-T5 are CLI-side and independent of T6-T9 except T5's integration test needs only httptest, not the portal.
