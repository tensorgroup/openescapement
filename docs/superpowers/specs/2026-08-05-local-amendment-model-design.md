# Local amendment model: augment-aware status and non-destructive sync

Status: approved design, 2026-08-05. First of two specs. This one covers the local data model, sync behavior, and JSON surfaces; a follow-on spec covers the CLI to portal telemetry path and portal rendering, built against the JSON contract frozen here.

Motivation: teams will routinely add their own rules alongside a synced pack, and will occasionally edit inside the managed block by accident. Today the first is preserved but invisible, the second is silently overwritten, and one case (skill directories) destroys added files outright. The product framing is enablement rather than enforcement, so the tool should observe and report these states rather than block a rollout.

## 1. Two orthogonal axes

Every artifact carries two independent classifications:

- `managed`: `in-sync` | `altered` | `stale` | `missing` | `orphan`. Applies to content escapement owns. This is the existing `engine.State`, unchanged in meaning; `altered` is the renamed `modified` case (see §7).
- `local`: `none` | `amended`. Applies to content escapement does not own but that shares the artifact.

They are never collapsed into a single enum. The common real case is a team that both appended its own rules and carries an inadvertent edit inside ours; one enum would force dropping one of those facts.

The portal's four repo-level states derive from the cross product without the portal knowing anything about markers or hashing: `in-sync`/`none` is unadulterated, `in-sync`/`amended` is augmented, `altered`/anything is altered, and an absent lockfile is ungoverned.

Per artifact kind:

| Kind | `managed` compares | `local: amended` means | Today |
|---|---|---|---|
| `block` (CLAUDE.md, AGENTS.md, GEMINI.md, custom targets) | block body hash against plan | any bytes outside the begin/end markers | preserved, unreported |
| `json-keys` (`.mcp.json`) | owned keys hash | non-owned server entries present | preserved, unreported |
| `file` (whole-file targets) | file hash | not applicable, escapement owns the whole file; always `none` | correct |
| `dir` (skill directories) | pack-provided files' hashes | files present that the pack did not provide | **deleted on every sync** |

Three of four kinds already preserve the delta. Reporting is what is missing. `dir` is a live data-loss defect: `stageDir` does `os.RemoveAll(dst)` followed by a rename (`internal/engine/apply.go:229`), so any file a team adds to an esc-owned skills directory is destroyed by the next sync.

## 2. Directory manifests

Fixing `dir` requires distinguishing "the team added this file" from "the pack used to ship this file and dropped it". The lockfile stores only a tree hash, which cannot tell those apart, and guessing wrong in the second direction is worse than the current defect: a rule the org deliberately removed would persist forever as a phantom amendment.

`lockfile.LockArtifact` gains `Files []string`, the pack-relative paths the last sync wrote. This is the same shape as the existing `Keys []string` used for the json-keys case, and takes one lockfile schema bump.

Sync semantics for a `dir` artifact become per file:

- Present in the current pack: written, as today.
- Present in the previous manifest but not in the current pack: removed. The pack dropped it.
- Present on disk, in neither the current pack nor the previous manifest: left untouched and counted as a local amendment.

`stageDir` is replaced by a merge that stages the pack tree and reconciles against the previous manifest, retaining the current atomicity property (no half-written or half-deleted destination on failure).

Pre-manifest lockfiles have no `Files` entry. On the first sync after upgrade, every on-disk file not provided by the current pack is treated as a local amendment and preserved. This is the conservative direction: it can leave one stale pack-dropped file behind, which status reports as an amendment, rather than deleting a team's work.

## 3. Sync: skip and report

`engine.Apply` gains a classification pass before any write. When `managed == altered`, the write is skipped and recorded. Everything else syncs normally, including `stale`, which is the no-human-edit case and the entire point of the tool.

Granularity is per artifact, and per file within a `dir` artifact, which the §2 manifest provides for free. An edited `esc-security/rules.md` does not block the other skills in that directory from updating.

**The lockfile entry for a skipped artifact is left unchanged** while pack pins advance normally. This is load-bearing. `classify` distinguishes stale from modified by comparing the on-disk hash against `locked.Hash` (`internal/engine/status.go:148`). Leaving the entry pointing at what was last actually written means the next `esc status` finds a body matching neither the new plan hash nor the lock hash, and reports `altered` indefinitely until someone resolves it. Writing the new hash would make the alteration disappear from the report on the next run.

`esc sync --force` overwrites altered artifacts and converges everything, including replacing pack-provided files in a skills directory. It never removes local amendments outside the managed region; `--force` is about our content, not theirs.

## 4. Exit codes and command semantics

`esc sync` exits 0 with skipped artifacts, printing one warning per skip. A repository that declines part of a policy update does not fail its own rollout.

The consequence, stated plainly because it changes an existing meaning: **exit 0 from `esc sync` no longer asserts that the repository matches policy.** It asserts that everything escapement was willing to apply was applied. Compliance gating belongs on `esc status --check`, which exits 1 on any non-in-sync artifact (`internal/cli/cli.go:288`; note that bare `esc status` exits 0 on ordinary drift and always has).

`local: amended` on its own is never drift. An augmented file with an in-sync block is reported as in sync and does not affect any exit code. Amendment is expected behavior, not a finding.

## 5. Reporting surface

`engine.Finding` grows:

```go
type Finding struct {
    Path       string
    Kind       string
    State      State       // managed axis
    Local      LocalState  // none | amended
    Detail     string
    Amendment  *Amendment  // non-nil when Local == amended
    Alteration *Alteration // non-nil when State == altered
}

type Amendment struct {
    Bytes, Lines int
    Hash         string
    Content      string `json:",omitempty"` // always set locally; publisher drops it below level "content"
}

type Alteration struct {
    ExpectedHash, ActualHash string
    Diff                     string `json:",omitempty"` // same treatment as Content
}
```

`Content` and `Diff` are discrete fields rather than text inlined into a rendered blob, so the payload can be reduced later without a schema migration.

**Local surfaces always show local truth in full.** `esc status`, `esc status --json`, and `esc sync --json` report every amendment completely, including content, regardless of the reporting level. It is the user's machine and their own file, and an open-source user running with no `reporting` block at all must still get a fully useful local report. The reporting level governs only what leaves the repository, and the redaction that applies it lives in the publisher (spec 2), which consumes this document and emits a reduced version of it. Spec 1 resolves and reports the effective level in the `collection` object but never acts on it.

Human output gains an amendment suffix and a collection notice:

```
  ✓ AGENTS.md          in sync  ·  +47 lines local
  ✗ CLAUDE.md          altered: managed block was hand-edited
                       `esc diff` to inspect · `esc sync --force` to overwrite
  ✓ .claude/skills/esc-security  in sync  ·  3 unmanaged files preserved
  ⚠ org-baseline       1.2.0 pinned, 1.3.0 available

Local amendments will be reported upstream once a publisher is configured, including content.
(pack policy; set report_amendments: metrics or off in .escapement.yaml to withhold)
```

The collection notice prints whenever the effective reporting level is above `off` and at least one amendment exists. It is unconditional and not behind a verbose flag. A team must be able to see that its own additions are being reported without reading the pack manifest.

`esc status --json` is the portal's contract: schema version, pack pins with `pinned`, `latest`, and `signed`, the artifact array above, and a `collection` object giving the effective level and its source (`pack` or `repo-override`).

`esc sync --json` emits the same document plus a `skipped` array of what sync declined to write and why.

Both surfaces are golden-tested and neither requires the portal to exist.

## 6. Reporting configuration

Pack manifest, mirroring the existing `UpdateCheck` in shape and stance:

```go
Reporting *Reporting `yaml:"reporting,omitempty"`

type Reporting struct {
    Amendments string `yaml:"amendments"` // "metrics" | "content"
}
```

Absent means nothing is sent upstream, ever. The unconnected open-source path stays silent by construction, the same posture §5.4 of the product spec takes for update checks. A pack must explicitly ask before anything leaves a repository. Local reporting is unaffected; a repo with no `reporting` block still gets complete `status` output per §5.

Repo config gains `report_amendments` (`"metrics"` | `"off"`), which can only clamp down: `content` to `metrics` to `off`. A repository can never raise the level above what the pack requested, so it cannot opt itself into sending content the org did not ask for.

With multiple packs, the highest level any pack requests wins (`content` over `metrics` over none), then the repo override clamps. This mirrors the update-check rule that the strictest cadence wins: a pack can make policy stricter but never weaker. Adding a second pack must not be able to silently reduce what an admin sees, which would look like the fleet got cleaner when it only got blinder.

The veto is visible rather than silent. `--json` reports the source of the effective level, so a clamped repository surfaces as amended with content withheld. An admin sees the withholding instead of seeing nothing.

## 7. Naming

`engine.Modified` is renamed `engine.Altered`, with the string value moving from `modified` to `altered`, to match the vocabulary used throughout status output and the portal. There are no users and no external consumers of these strings, so no alias is retained.

## 8. Testing

Integration tests over real temp git repos, per existing convention:

- Append prose to AGENTS.md, sync: amendment survives byte for byte, reported as `in-sync`/`amended` with correct line and byte counts, exit 0.
- Edit inside the managed block, sync: file untouched, artifact in the `skipped` array, lockfile entry unchanged, exit 0, subsequent `status` still reports `altered`.
- Same, then `sync --force`: block converges, surrounding amendment still intact.
- Add a file to an esc-owned skills directory, sync: file preserved, counted as an amendment.
- Pack drops a skill file it previously shipped, sync: file removed, not misreported as an amendment.
- Pre-manifest lockfile upgrade path: unknown on-disk files preserved on first sync.
- Add a non-owned server to `.mcp.json`, sync: entry preserved and reported as an amendment.
- Reporting resolution table, asserted against the `collection` object rather than against payload contents, since spec 1 never redacts: pack absent resolves to `off`, single pack resolves to its declared level, two packs at different levels resolve to the higher, repo clamp lowers the effective level and sets source `repo-override`, repo attempting to raise above the pack level is rejected at config load.
- Local completeness under every resolution above: amendment content is present in `status --json` even when the effective level is `off`.

Golden files for `esc status --json` and `esc sync --json` covering unadulterated, augmented, altered, and mixed repositories.

Renderer invariants from AGENTS.md continue to hold and stay golden-tested: bytes outside a managed block are never modified, nothing is written after a verification or constraint failure, all writes are atomic, output is deterministic.

## 9. Docs

- `AGENTS.md`: the two-axis model and the new invariant that local amendments are preserved across sync for every artifact kind.
- Product spec: §5 gains the reporting model and the pack-declares/repo-declines consent shape; the enablement-over-enforcement rationale for skip-and-report is recorded alongside open question 2 (enforcement vs observation), which this decides for v1.
- README: `esc sync` exit-code semantics, `esc status --check` as the compliance gate, and `report_amendments`.
- CHANGELOG bullet. No em-dashes.
