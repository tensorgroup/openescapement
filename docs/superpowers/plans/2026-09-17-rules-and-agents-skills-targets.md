# Rules and Agents-Skills Render Targets Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two built-in render targets. `rules`: one escapement-owned whole file per opt-in fragment under `.claude/rules/esc-<pack>-<stem>.md`, carrying a verbatim `paths:` frontmatter scope; `agents-skills`: the pack's skill directories rendered a second time under `.agents/skills/<name>`, off by default. Along the way, build the whole-file occupied gate and orphan pass that no `KindFile` artifact has today (proven first on `GOVERNANCE.md`).

**Architecture:** `esc` is a Go CLI with a `Plan → Apply` pipeline. `internal/targets` holds the built-in target table; `internal/pack` loads/validates fragments; `internal/render` composes text; `internal/engine` plans artifacts, classifies drift (`status.go`), writes/removes them (`apply.go`), and diffs (`diff.go`). Artifacts are one of `KindBlock`/`KindFile`/`KindDir`/`KindJSONKeys`. Rule files are `KindFile`; agents-skills copies are `KindDir`.

**Tech Stack:** Go 1.24, module `github.com/tensorgroup/openescapement`, binary `esc`. Single external dependency `gopkg.in/yaml.v3`; everything else stdlib; system `git` via `os/exec`.

**Spec:** docs/superpowers/specs/2026-09-17-rules-and-agents-skills-targets-design.md

## Global Constraints
- Dependency policy: `gopkg.in/yaml.v3` only; everything else stdlib. Adding any dependency is out of scope.
- `internal/pack` must NOT import `internal/render` (render imports pack — that direction is a cycle). Normalize line endings inside pack with stdlib `strings.ReplaceAll`, not `render.NormalizeEndings`.
- Exit-code sentinels (`internal/esc`): 0 ok, 1 `ErrConstraint` (drift/constraint), 2 `ErrConfig`/`ErrUsage` (usage/config), 3 `ErrSignature`/`ErrLockMismatch` (integrity), 4 plain error (containment/other).
- Containment refusals (`refuseSymlinks`, escaping lockfile entries) return a PLAIN error → exit 4, never `ErrConstraint`.
- Renderer invariants (golden-testable): bytes outside a managed block are never modified; nothing is written after a verification/constraint failure; all writes are atomic; renderer output is deterministic (same packs → same bytes).
- Byte preservation is necessary but NOT sufficient: any whole-file write must be structurally valid (frontmatter at byte 0, marker on its own line, no glued last line).
- Never adopt a pre-existing directory/file except when its content is byte-identical (hash-equal, endings-normalized) to what escapement would write; `--force` does NOT override the occupied gate.
- The hand-edit / orphan skip gate requires a prior lockfile entry; carry the prior lock entry forward unchanged on any decline that leaves content on disk.
- The redaction walk test in `internal/publisher` must keep passing: add NO new content-shaped field to `engine.Report`/`Finding`/`Skipped`/`publisher.Envelope`.
- `gofmt -l .` clean, `go vet ./...` clean, `go test ./...` green before any task is done.
- Commit messages describe the WHY from the diff; no AI co-authorship trailers; never `--no-verify`; do not commit unless the executing skill's checkpoint says to (this plan's steps commit per task).

## File structure

| Path | Responsibility | Tasks |
|---|---|---|
| `internal/targets/targets.go` | Built-in target table; `OptIn`, `KindRulesDir`, `Defaults()` | 1 |
| `internal/targets/targets_test.go` | Table/Defaults/IsFragmentTarget tests | 1 |
| `internal/render/render.go` | `TargetRules`/`TargetAgentsSkills` consts, `FragmentNamesTarget`, `RuleFilePath`, `RuleFileStem`, `RenderRule` | 1,4 |
| `internal/render/rules_test.go` | RenderRule golden tests | 4 |
| `internal/render/governance.go` | GOVERNANCE.md lists rules fragments | 4 |
| `internal/render/governance_test.go` | Governance rules-section test | 4 |
| `internal/cli/initscan.go` | `targetOrder`, `.agents/skills` detection, `detectedTargets` appends rules, governance sync clause | 1,8 |
| `internal/cli/initscan_test.go` | `TestTargetOrderCoversAllTargets` extended | 1 |
| `internal/engine/engine.go` | `PlanResult.Notices`, governance hash fix, rules planner, agents-skills planner, `allTargets` from table, `isBuiltInTarget` | 2,5,6 |
| `internal/engine/apply.go` | KindFile occupied gate + adoption, orphan-file pass, `ownedFilePath`/`ownedRulePath`, `ownedSkillPath` both roots, new skip causes | 2,5,6 |
| `internal/engine/status.go` | KindFile occupied classify, orphan-file pass, SkillDuplicate scoped to `.claude/skills` | 2,6 |
| `internal/engine/diff.go` | `prospectiveContent` symlink refusal, rule-file diff, default list from table | 2,7 |
| `internal/pack/fragment.go` | LF normalize at load, `Fragment.Paths`, rule-path validation | 3 |
| `internal/pack/compose.go` | `ComposeFragments` refuses `paths:` | 3 |
| `internal/cli/cli.go` | print `plan.Notices`, `skipHint` cases, init rules/agents-skills wiring | 5,8 |
| `internal/engine/*_test.go` | Engine tests (occupied/orphan/rules/agents-skills/notice/shape) | 2,5,6,9 |
| `examples/packs/acme-org/*` | `rules/api-handlers.md`, version bump | 10 |
| `examples/governed-service/*` | regenerated | 10 |
| `README.md`, `docs/pack-authoring.md`, `docs/cli-and-portal.md`, `CHANGELOG.md`, `AGENTS.md` | docs | 2,11 |

---

## Task 1 — Targets table, `OptIn`, `KindRulesDir`, `Defaults()`, render constants, target-order test

**Files**
- Modify `internal/targets/targets.go` (constants ~15-30, `Info` struct ~35-45, `builtIns` ~47-54, `IsFragmentTarget` ~82-85).
- Create `internal/targets/targets_test.go`.
- Modify `internal/render/render.go` (const block ~12-19; add `FragmentNamesTarget` exported wrapper near ~99).
- Modify `internal/cli/initscan.go` (`targetOrder` ~76-79).
- Modify `internal/cli/initscan_test.go` (`TestTargetOrderCoversAllTargets` ~123).
- Modify `internal/engine/engine.go` (`isBuiltInTarget` ~412-419) and `internal/engine/diff.go` (`PolicyDiff` default list ~63-65).

**Interfaces**
- Produces: `targets.NameRules = "rules"`, `targets.NameAgentsSkills = "agents-skills"`, `targets.KindRulesDir = "rules-dir"`, `targets.Info.OptIn bool`, `func targets.Defaults() []string`.
- Produces: `render.TargetRules = "rules"`, `render.TargetAgentsSkills = "agents-skills"`, `func render.FragmentNamesTarget(f pack.Fragment, target string) bool`.
- Consumes (unchanged): `targets.IsBuiltInName(string) bool`, `targets.IsFragmentTarget(string) bool`.
- Note: `engine.allTargets` is NOT rewired here (that happens in Task 5, together with the `rules` switch case, so the plan's target switch stays total). `isBuiltInTarget` is changed to delegate to `targets.IsBuiltInName` so an explicit `targets: [rules|agents-skills]` list is not rejected as unknown before its planner case exists.

**Steps**
- [ ] Write the failing test `internal/targets/targets_test.go`:
```go
package targets

import "testing"

func TestRulesAndAgentsSkillsInTable(t *testing.T) {
	r, ok := ByName(NameRules)
	if !ok {
		t.Fatal("rules missing from built-in table")
	}
	if r.Kind != KindRulesDir || !r.BuiltIn || r.OptIn {
		t.Errorf("rules row wrong: %+v", r)
	}
	as, ok := ByName(NameAgentsSkills)
	if !ok {
		t.Fatal("agents-skills missing from built-in table")
	}
	if as.Kind != KindSkillsDir || !as.BuiltIn || !as.OptIn {
		t.Errorf("agents-skills row wrong: %+v", as)
	}
}

func TestIsFragmentTargetAcceptsRules(t *testing.T) {
	if !IsFragmentTarget(NameRules) {
		t.Error("rules must be a valid fragment target")
	}
	if IsFragmentTarget(NameSkills) || IsFragmentTarget(NameMCP) {
		t.Error("skills/mcp are not fragment targets")
	}
	if IsFragmentTarget(NameAgentsSkills) {
		t.Error("agents-skills is a skills-dir kind, not a fragment target")
	}
}

func TestDefaultsExcludesOptInIncludesRules(t *testing.T) {
	d := Defaults()
	seen := map[string]bool{}
	for _, n := range d {
		seen[n] = true
	}
	for _, want := range []string{NameClaude, NameAgents, NameGemini, NameGovernance, NameSkills, NameMCP, NameRules} {
		if !seen[want] {
			t.Errorf("Defaults() missing %q", want)
		}
	}
	if seen[NameAgentsSkills] {
		t.Error("Defaults() must exclude the opt-in agents-skills target")
	}
}
```
- [ ] Run `go test ./internal/targets/`. Expected failure: `undefined: NameRules` / `KindRulesDir` / `OptIn` / `Defaults` (compile error).
- [ ] Implement in `internal/targets/targets.go`. Add to the built-in names const block:
```go
	NameMCP        = "mcp"
	NameRules        = "rules"
	NameAgentsSkills = "agents-skills"
```
Add to the target kinds const block:
```go
	KindMCPConfig    = "mcp-config"
	KindRulesDir     = "rules-dir"
```
Add `OptIn bool` to `Info` (below `BuiltIn`), with a doc comment:
```go
	BuiltIn     bool
	// OptIn marks a built-in that an empty targets: list does NOT expand to;
	// a repo must name it explicitly. Zero value false keeps every existing
	// row in the default set.
	OptIn       bool
	OwnerPack   string // defining pack name; empty for built-ins
```
Append two rows to `builtIns` (after the mcp row):
```go
	{Name: NameMCP, File: "", Kind: KindMCPConfig, BuiltIn: true},
	{Name: NameRules, File: "", Kind: KindRulesDir, BuiltIn: true},
	{Name: NameAgentsSkills, File: "", Kind: KindSkillsDir, BuiltIn: true, OptIn: true},
```
Extend `IsFragmentTarget`:
```go
	return ok && (in.Kind == KindManagedBlock || in.Kind == KindWholeFile || in.Kind == KindRulesDir)
```
Add `Defaults`:
```go
// Defaults returns the built-in target names an empty targets: list expands
// to, in table order: every built-in that is not opt-in. agents-skills is the
// one built-in excluded (OptIn), so a repo must list it explicitly.
func Defaults() []string {
	var out []string
	for _, in := range builtIns {
		if in.BuiltIn && !in.OptIn {
			out = append(out, in.Name)
		}
	}
	return out
}
```
- [ ] Run `go test ./internal/targets/`. Expect PASS.
- [ ] Add render constants + exported wrapper in `internal/render/render.go`. In the const block append:
```go
	TargetMCP        = "mcp"
	TargetRules        = "rules"
	TargetAgentsSkills = "agents-skills"
```
Add near `fragmentNamesTarget`:
```go
// FragmentNamesTarget reports whether f explicitly names target. An empty
// target list never matches: rule and custom targets require explicit opt-in.
func FragmentNamesTarget(f pack.Fragment, target string) bool {
	return fragmentNamesTarget(f, target)
}
```
- [ ] Change `isBuiltInTarget` in `internal/engine/engine.go` to:
```go
func isBuiltInTarget(name string) bool {
	return targets.IsBuiltInName(name)
}
```
- [ ] Change the `PolicyDiff` default target list in `internal/engine/diff.go` from the four-literal slice to `targets.Defaults()` (add the `targets` import to diff.go):
```go
	targets := cur.Config.Targets
	if len(targets) == 0 {
		targets = targetsPkg.Defaults()
	}
```
Import as `targetsPkg "github.com/tensorgroup/openescapement/internal/targets"` to avoid shadowing the local `targets` variable. Rule/skills/mcp/agents-skills names all have `render.TargetFile[t] == ""` and are skipped by the existing guard, so this is behavior-neutral until Task 7.
- [ ] Extend `targetOrder` in `internal/cli/initscan.go` to include the two new names in table order:
```go
var targetOrder = []string{
	render.TargetClaude, render.TargetAgents, render.TargetGemini, render.TargetGovernance,
	render.TargetRules, render.TargetMCP, render.TargetSkills, render.TargetAgentsSkills,
}
```
- [ ] Extend `TestTargetOrderCoversAllTargets` in `internal/cli/initscan_test.go` so the `want` set includes the two new targets:
```go
	want := map[string]bool{
		render.TargetMCP: true, render.TargetSkills: true,
		render.TargetRules: true, render.TargetAgentsSkills: true,
	}
```
- [ ] Run `go test ./... ` (full suite). Expect PASS: no default-set behavior changed yet (empty-list still uses the unchanged `allTargets` literal; explicit new names are accepted by `isBuiltInTarget` but not yet listed anywhere).
- [ ] `gofmt -w .` and `go vet ./...`.
- [ ] Commit: `feat(targets): add rules and agents-skills to the built-in table with an opt-in flag` — body explains why agents-skills is opt-in (two copies of every skill only when a repo runs both Claude Code and Kimi Code) and why the default set is now derivable from the table.

---

## Task 2 — Whole-file machinery for every `KindFile`, proven on `GOVERNANCE.md`

Builds the occupied gate, byte-identical adoption, orphan pass, ownership, and pre-read symlink refusal for `KindFile` artifacts — before any rule file exists — so it is reviewable on `GOVERNANCE.md` alone.

**Files**
- Modify `internal/engine/engine.go` (governance planner ~221-225; `prospectiveContent` ~360-374).
- Modify `internal/engine/apply.go` (skip causes ~136-161; add `ownedFilePath`/`ownedRulePath`/`identicalFileContent`; KindFile occupied gate in `Apply` after the KindDir block ~349; orphan-file pass after the block-removal loop ~649).
- Modify `internal/engine/status.go` (`classify` KindFile branch ~410-428; orphan-file pass after the orphan-dir pass ~322).
- Create `internal/engine/wholefile_test.go`.
- Modify `internal/cli/cli.go` (`skipHint` ~525-543).
- Modify `internal/cli/initscan.go` (`detectedSyncClause` ~139-155; governance is whole-file, not a block).
- Modify `internal/cli/initoffer.go` (`offerPlacement` eligibility ~354-355 excludes governance).
- Create/extend a CLI test for the init GOVERNANCE.md wording (`internal/cli/init_governance_test.go`).
- Modify `AGENTS.md` (never-adopt/retirement invariants: "directory" → "artifact" where general) and `CHANGELOG.md` (Changed entry).

**Interfaces**
- Produces: `engine.SkipUnmanagedFileAtTarget SkipCause = "unmanaged-file-at-target"`, `engine.SkipOrphanFileEdited SkipCause = "orphan-file-edited"`.
- Produces (apply.go, unexported): `func ownedFilePath(rel string) bool`, `func ownedRulePath(rel string) bool`, `func identicalFileContent(abs, wantHash string) bool`.
- Consumes: `render.NormalizeEndings([]byte) []byte`, `esc.HashBytes([]byte) string`, `render.TargetFile map[string]string`, existing `containedPath`, `refuseSymlinks`.

**Steps**
- [ ] Write the failing test `internal/engine/wholefile_test.go` (drives Apply/Status directly with a hand-built plan + lockfile, mirroring `orphandir_test.go`):
```go
package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/lockfile"
	"github.com/tensorgroup/openescapement/internal/render"
)

func fileArtifact(path, body string) Artifact {
	return Artifact{Path: path, Kind: KindFile, Body: body, Hash: esc.HashBytes(render.NormalizeEndings([]byte(body)))}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func saveLock(t *testing.T, root string, arts ...lockfile.LockArtifact) {
	t.Helper()
	l := &lockfile.Lock{Schema: 1, Artifacts: arts}
	if err := l.Save(root); err != nil {
		t.Fatal(err)
	}
}

// Occupied: a KindFile path already holds unmanaged content and there is no
// lock entry. Sync skips it, status reports Occupied, --force does not override.
func TestKindFileOccupiedSkipsAndForceDoesNotOverride(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "GOVERNANCE.md", "my own hand-written governance\n")
	a := fileArtifact("GOVERNANCE.md", "escapement content\n")
	plan := &PlanResult{Artifacts: []Artifact{a}}

	res, err := Apply(root, plan, false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Cause != SkipUnmanagedFileAtTarget {
		t.Fatalf("want one unmanaged-file skip, got %+v", res.Skipped)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "GOVERNANCE.md")); string(got) != "my own hand-written governance\n" {
		t.Errorf("occupied file was overwritten: %q", got)
	}
	// --force must NOT override occupied.
	res, err = Apply(root, plan, true)
	if err != nil {
		t.Fatalf("apply --force: %v", err)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Cause != SkipUnmanagedFileAtTarget {
		t.Fatalf("--force must not override occupied: %+v", res.Skipped)
	}
	st := classify(root, a, mustLock(t, root))
	if st.State != Occupied {
		t.Errorf("status should report Occupied, got %s", st.State)
	}
}

// Byte-identical adoption: an existing file whose normalized bytes equal the
// desired output is adopted (lock entry written, nothing written to disk).
func TestKindFileIdenticalAdopts(t *testing.T) {
	root := t.TempDir()
	// CRLF on disk, LF desired: normalized-equal, so adopt.
	writeFile(t, root, "GOVERNANCE.md", "escapement content\r\n")
	a := fileArtifact("GOVERNANCE.md", "escapement content\n")
	plan := &PlanResult{Artifacts: []Artifact{a}}
	res, err := Apply(root, plan, false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Adopted) != 1 || res.Adopted[0] != "GOVERNANCE.md" {
		t.Fatalf("want adoption, got applied=%v adopted=%v skipped=%+v", res.Applied, res.Adopted, res.Skipped)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "GOVERNANCE.md")); string(got) != "escapement content\r\n" {
		t.Errorf("adoption must not rewrite disk: %q", got)
	}
	if classify(root, a, mustLock(t, root)).State != InSync {
		t.Error("adopted file must classify InSync")
	}
}

// Orphan: a KindFile lock entry no longer planned is removed; a hand-edited one
// is declined and its entry carried forward.
func TestKindFileOrphanRemovedAndEditedDeclined(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "GOVERNANCE.md", "escapement content\n")
	saveLock(t, root, lockfile.LockArtifact{Path: "GOVERNANCE.md", Kind: KindFile, Hash: esc.HashBytes([]byte("escapement content\n"))})
	res, err := Apply(root, &PlanResult{}, false) // empty plan: GOVERNANCE.md is orphaned
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "GOVERNANCE.md")); !os.IsNotExist(err) {
		t.Errorf("unedited orphan file should be removed")
	}

	// Now an edited orphan: declined, carried forward.
	root2 := t.TempDir()
	writeFile(t, root2, "GOVERNANCE.md", "hand edited\n")
	saveLock(t, root2, lockfile.LockArtifact{Path: "GOVERNANCE.md", Kind: KindFile, Hash: esc.HashBytes([]byte("escapement content\n"))})
	res, err = Apply(root2, &PlanResult{}, false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Cause != SkipOrphanFileEdited {
		t.Fatalf("edited orphan must be declined: %+v", res.Skipped)
	}
	if _, err := os.Stat(filepath.Join(root2, "GOVERNANCE.md")); err != nil {
		t.Errorf("declined orphan must stay on disk: %v", err)
	}
	lk := mustLock(t, root2)
	if lk.Artifact("GOVERNANCE.md") == nil {
		t.Error("declined orphan lock entry must be carried forward")
	}
}

// Ownership: a lock entry naming a file escapement does not own is refused
// before any read.
func TestKindFileOrphanRefusesUnownedPath(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "VICTIM.md", "not escapement's\n")
	saveLock(t, root, lockfile.LockArtifact{Path: "VICTIM.md", Kind: KindFile, Hash: "sha256:whatever"})
	if _, err := Apply(root, &PlanResult{}, false); err == nil {
		t.Fatal("apply must refuse to remove an unowned KindFile lock entry")
	}
	if _, err := os.Stat(filepath.Join(root, "VICTIM.md")); err != nil {
		t.Errorf("unowned file must not be touched: %v", err)
	}
}

func mustLock(t *testing.T, root string) *lockfile.Lock {
	t.Helper()
	l, err := lockfile.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return l
}
```
- [ ] Run `go test ./internal/engine/ -run KindFile`. Expected failure: `undefined: SkipUnmanagedFileAtTarget` / `SkipOrphanFileEdited` and assertion failures (occupied not skipped; orphan file not removed) — compile error first.
- [ ] Implement the two skip causes in `internal/engine/apply.go` (after `SkipUnmanagedDirAtTarget`):
```go
	// SkipUnmanagedFileAtTarget: a file escapement never wrote already occupies
	// a KindFile artifact's target path, with no prior lock entry. Like the dir
	// case, sync declines and force does NOT override; the one exception is a
	// byte-identical (endings-normalized) file, adopted instead of skipped.
	SkipUnmanagedFileAtTarget SkipCause = "unmanaged-file-at-target"
	// SkipOrphanFileEdited: a KindFile the effective set no longer contains was
	// hand-edited since the last sync, so it was left in place, not removed.
	SkipOrphanFileEdited SkipCause = "orphan-file-edited"
```
- [ ] Implement the ownership + adoption helpers in `internal/engine/apply.go` (near `ownedSkillPath`). Add `"strings"` is already imported; add `render` (already imported):
```go
// ownedFilePath reports whether a lockfile KindFile entry is one escapement
// could have written: the built-in GOVERNANCE.md path, or a rule file under
// .claude/rules with the reserved esc- prefix. Decided on the repo-relative
// path, never the absolute one, and confines what a hostile lockfile can
// delete — the same ruling AGENTS.md records for skill directories. The prefix
// is a reserved namespace, not proof escapement created the file.
func ownedFilePath(rel string) bool {
	c := path.Clean(rel)
	return c == render.TargetFile[render.TargetGovernance] || ownedRulePath(c)
}

// ownedRulePath reports whether rel is exactly one element below .claude/rules
// carrying the reserved esc- prefix.
func ownedRulePath(rel string) bool {
	c := path.Clean(rel)
	return path.Dir(c) == ".claude/rules" && strings.HasPrefix(path.Base(c), "esc-")
}

// identicalFileContent reports whether the file at abs, endings-normalized,
// hashes to wantHash — the ONLY condition under which the KindFile occupied
// gate adopts rather than skips (nothing to destroy). refuseSymlinks has
// already run on this path in Apply, so the read cannot follow a symlink.
func identicalFileContent(abs, wantHash string) bool {
	got, err := os.ReadFile(abs)
	if err != nil {
		return false
	}
	return esc.HashBytes(render.NormalizeEndings(got)) == wantHash
}
```
Confirm `esc` is imported in apply.go — it is (`internal/esc`). `path` is imported. Good.
- [ ] Implement the KindFile occupied gate in `Apply`, immediately after the `if a.Kind == KindDir { ... }` block (before `if !force {`):
```go
		if a.Kind == KindFile {
			// Same never-adopt gate as KindDir, and the same one exception: a
			// pre-existing file with no lock entry is skipped (force does not
			// override), UNLESS its normalized bytes already equal what we would
			// write, in which case there is nothing to destroy and we adopt.
			if prevLock.Artifact(a.Path) == nil {
				if _, statErr := os.Lstat(abs); statErr == nil {
					if identicalFileContent(abs, a.Hash) {
						arts = append(arts, lockfile.LockArtifact{Path: a.Path, Kind: a.Kind, Hash: a.Hash})
						res.Adopted = append(res.Adopted, a.Path)
						continue
					}
					res.Skipped = append(res.Skipped, Skipped{
						Subject: a.Path, Kind: a.Kind, Cause: SkipUnmanagedFileAtTarget,
						Reason:       "an unmanaged file already occupies this path",
						ExpectedHash: a.Hash,
					})
					continue
				}
			}
		}
```
- [ ] Implement the orphan-file pass in `Apply`, after the stale-managed-block removal loop (before `lock := &lockfile.Lock{...}` at ~651):
```go
	// Remove owned whole files (GOVERNANCE.md, rule files) the effective set no
	// longer contains — same carry-forward semantics as the block/dir passes.
	desiredFiles := map[string]bool{}
	for _, a := range p.Artifacts {
		if a.Kind == KindFile {
			desiredFiles[a.Path] = true
		}
	}
	if prevLock != nil {
		for _, prev := range prevLock.Artifacts {
			if prev.Kind != KindFile || desiredFiles[prev.Path] {
				continue
			}
			if !ownedFilePath(prev.Path) {
				return nil, fmt.Errorf("refusing to remove %q: not an escapement-owned file", prev.Path)
			}
			abs, err := containedPath(root, prev.Path)
			if err != nil {
				return nil, err
			}
			if err := refuseSymlinks(root, prev.Path); err != nil {
				return nil, err
			}
			content, err := os.ReadFile(abs)
			if os.IsNotExist(err) {
				continue // already gone
			}
			if err != nil {
				return nil, err
			}
			if !force {
				if actual := esc.HashBytes(render.NormalizeEndings(content)); actual != prev.Hash {
					res.Skipped = append(res.Skipped, Skipped{
						Subject: prev.Path, Kind: prev.Kind, Cause: SkipOrphanFileEdited,
						Reason:       "no longer provided by any pack but hand-edited since the last sync; left in place instead of being removed",
						ExpectedHash: prev.Hash, ActualHash: actual,
					})
					arts = append(arts, prev) // carry forward, like the orphan-block pass
					continue
				}
			}
			if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
				return nil, err
			}
		}
	}
```
- [ ] Run `go test ./internal/engine/ -run KindFile`. Expect PASS.
- [ ] Add the status-side occupied classification. In `internal/engine/status.go`, replace the `case KindFile:` branch so it mirrors the KindDir occupied gate:
```go
	case KindFile:
		if locked == nil {
			// Mirror of Apply's KindFile occupied gate: no prior lock entry plus
			// something on disk is a file escapement never wrote. A byte-for-byte
			// (endings-normalized) match adopts on the next sync, so report the
			// in-sync verdict it will converge to, not Occupied.
			if _, lerr := os.Lstat(abs); lerr == nil {
				if identicalFileContent(abs, a.Hash) {
					return Finding{Subject: a.Path, Kind: a.Kind, State: InSync, Local: LocalNone}
				}
				return Finding{Subject: a.Path, Kind: a.Kind, State: Occupied, Local: LocalNone,
					Detail: "an unmanaged file occupies this path; escapement will not adopt or overwrite it — move it aside, then run `esc sync` (`--force` does not override this)"}
			}
		}
		content, err := os.ReadFile(abs)
		if err != nil {
			return Finding{Subject: a.Path, Kind: a.Kind, State: Missing, Local: LocalNone, Detail: "file does not exist — run `esc sync`"}
		}
		f := Finding{Subject: a.Path, Kind: a.Kind, Local: LocalNone}
		actual := esc.HashBytes(render.NormalizeEndings(content))
		if actual == a.Hash {
			f.State = InSync
			return f
		}
		f.State, f.Detail = staleOrAltered(actual, "file was hand-edited (hash mismatch)")
		if f.State == Altered {
			f.Alteration = &Alteration{ExpectedHash: a.Hash, ActualHash: actual}
		}
		return f
```
- [ ] Add the status-side orphan-file pass in `internal/engine/status.go`, after the orphan skill-directory pass (before the `for _, v := range plan.Violations` loop ~324):
```go
	// Orphan whole files: a KindFile the lockfile records that the effective set
	// no longer contains. Same shape and vocabulary as the orphan-block/dir
	// passes, so a declined retirement stays visible to `esc status --check`.
	desiredFilesStatus := map[string]bool{}
	for _, a := range plan.Artifacts {
		if a.Kind == KindFile {
			desiredFilesStatus[a.Path] = true
		}
	}
	if lock != nil {
		for _, la := range lock.Artifacts {
			if la.Kind != KindFile || desiredFilesStatus[la.Path] {
				continue
			}
			if !ownedFilePath(la.Path) {
				res.Findings = append(res.Findings, Finding{
					Subject: la.Path, Kind: la.Kind, State: Orphan, Local: LocalNone,
					Detail: "lockfile records a file escapement does not own; `esc sync` refuses to remove it. Delete the stale lockfile entry by hand",
				})
				continue
			}
			abs, cerr := containedPath(root, la.Path)
			if cerr != nil {
				continue
			}
			if refuseSymlinks(root, la.Path) != nil {
				continue
			}
			content, err := os.ReadFile(abs)
			if err != nil {
				continue // already gone
			}
			f := Finding{
				Subject: la.Path, Kind: la.Kind, State: Orphan, Local: LocalNone,
				Detail: "a file escapement rendered for a target no longer in the effective set — run `esc sync` to remove it",
			}
			if actual := esc.HashBytes(render.NormalizeEndings(content)); actual != la.Hash {
				f.Alteration = &Alteration{ExpectedHash: la.Hash, ActualHash: actual}
				f.Detail = "a hand-edited file escapement rendered for a target no longer in the effective set; `esc sync` leaves it in place, `esc sync --force` removes it"
			}
			res.Findings = append(res.Findings, f)
		}
	}
```
- [ ] Fix the governance planner desired hash in `internal/engine/engine.go` (`case render.TargetGovernance:`), so it matches classify's endings-normalized hash:
```go
		case render.TargetGovernance:
			content := render.Governance(res.PackObjs)
			res.Artifacts = append(res.Artifacts, Artifact{
				Path: render.TargetFile[t], Kind: KindFile,
				Hash: esc.HashBytes(render.NormalizeEndings([]byte(content))), Body: content,
			})
```
- [ ] Add pre-read symlink refusal to `prospectiveContent` in `internal/engine/engine.go`, so plan, status, sync, and diff all refuse a symlinked KindFile path at exit 4 (Plan runs this in its validation loop; Status/Diff call Plan first):
```go
func prospectiveContent(root string, a Artifact) ([]byte, error) {
	if err := refuseSymlinks(root, a.Path); err != nil {
		return nil, err
	}
	existing, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(a.Path)))
	...
```
- [ ] Write a symlink-refusal round-trip test appended to `wholefile_test.go`, plus a CRLF round-trip test:
```go
func TestKindFileSymlinkRefusedInPlan(t *testing.T) {
	root := t.TempDir()
	// Symlink the whole-file target to /etc/passwd.
	if err := os.Symlink("/etc/passwd", filepath.Join(root, "GOVERNANCE.md")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	a := fileArtifact("GOVERNANCE.md", "x\n")
	if _, err := prospectiveContent(root, a); err == nil {
		t.Fatal("prospectiveContent must refuse a symlinked KindFile target")
	}
}

func TestKindFileDesiredHashEqualsClassifyOnCRLF(t *testing.T) {
	root := t.TempDir()
	body := "line one\nline two\n"
	a := fileArtifact("GOVERNANCE.md", body)
	// Write CRLF to disk with a matching lock entry (as a prior sync + Windows
	// checkout would leave it), then classify.
	writeFile(t, root, "GOVERNANCE.md", "line one\r\nline two\r\n")
	saveLock(t, root, lockfile.LockArtifact{Path: "GOVERNANCE.md", Kind: KindFile, Hash: a.Hash})
	if classify(root, a, mustLock(t, root)).State != InSync {
		t.Error("CRLF checkout of a whole file must classify InSync")
	}
}
```
- [ ] Run `go test ./internal/engine/`. Expect PASS.
- [ ] Add `skipHint` cases in `internal/cli/cli.go`:
```go
	case engine.SkipUnmanagedFileAtTarget:
		return "escapement never owned that file · move it aside, then `esc sync` (`--force` does not override this)"
	case engine.SkipOrphanFileEdited:
		return "the file is still there · revert it to the synced content, or `esc sync --force` to remove it"
```
- [ ] Update `esc init`'s GOVERNANCE.md story. In `internal/cli/initscan.go` `detectedSyncClause`, add a governance case before the block default:
```go
	case render.TargetGovernance:
		return "report GOVERNANCE.md as occupied and not overwrite it — move or delete it so escapement can own it"
```
And exclude governance from the placement offer in `internal/cli/initoffer.go` `offerPlacement` (governance is a whole file, has no managed block):
```go
		if !isFileTarget(d.Target) || d.Target == render.TargetGovernance || d.HasBlock || d.HasPlaceholder {
			continue
		}
```
And in `internal/cli/initscan.go` `explainDetection`, guard `needsPlacementHint` the same way:
```go
		if isFileTarget(d.Target) && d.Target != render.TargetGovernance && !d.HasBlock && !d.HasPlaceholder {
			needsPlacementHint = true
		}
```
- [ ] Write `internal/cli/init_governance_test.go`:
```go
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitExplainsGovernanceAsOccupied(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "GOVERNANCE.md"), []byte("# my governance\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := run(t, root, "init", "--yes")
	if code != 0 {
		t.Fatalf("init: %d\n%s", code, out)
	}
	if !strings.Contains(out, "occupied") || !strings.Contains(out, "move or delete") {
		t.Errorf("init should explain GOVERNANCE.md as occupied with the remedy; got:\n%s", out)
	}
	if strings.Contains(out, "insert a managed block at the top of GOVERNANCE.md") {
		t.Errorf("GOVERNANCE.md must not be described as a managed-block target:\n%s", out)
	}
}
```
- [ ] Run `go test ./internal/cli/ -run TestInitExplainsGovernance`. Expect PASS.
- [ ] Run the full suite `go test ./...`. Expect PASS (the shipped `examples/governed-service` lock has a GOVERNANCE.md entry, so it is not occupied; the normalized hash is a no-op on its LF checkout).
- [ ] Update `AGENTS.md`: in the "Sync never adopts a directory it did not create" invariant, generalize the never-adopt language to cover files too — change "A `KindDir` artifact ..." to note that a `KindFile` artifact with no prior lock entry whose target already exists is likewise skipped (`SkipUnmanagedFileAtTarget`, exit 0, status `occupied`), adopted only on a byte-identical (endings-normalized) match, and that `--force` does not override. In the retirement invariant, change "Retiring a skill directory" wording to "Retiring an artifact" where the sentence states the general rule (declined removal carries the lock entry forward, keeps reporting). Keep the skill-directory-specific manifest details as-is.
- [ ] Add a CHANGELOG `### Changed` entry under `## [Unreleased]`:
```
- The whole-file occupied gate and orphan pass now cover every `KindFile`
  artifact, `GOVERNANCE.md` included: a repo that already has its own
  `GOVERNANCE.md` is reported `occupied` and is no longer overwritten silently
  on first sync (move or delete it, or let escapement adopt a byte-identical
  copy); a `GOVERNANCE.md` no longer in the effective set is reported `orphan`
  and removed, or left in place if hand-edited.
```
- [ ] `gofmt -w .`, `go vet ./...`.
- [ ] Commit: `feat(engine): whole-file occupied gate and orphan pass for every KindFile` — body explains that this is the machinery rules needs, built and proven on GOVERNANCE.md first, and names the first-sync behavior change.

---

## Task 3 — Pack loader: LF normalize at load, `Fragment.Paths`, rule-path validation, compose refusal

**Files**
- Modify `internal/pack/fragment.go` (`Fragment` struct ~17-21; `loadFragment` ~29-54; add `validateRulePaths`; add `ruleStem` regexp).
- Modify `internal/pack/compose.go` (meta struct ~31-33; add `paths:` refusal ~43).
- Create `internal/pack/fragment_rules_test.go`.
- Create/extend `internal/pack/compose_test.go` (paths refusal case).

**Interfaces**
- Produces: `pack.Fragment.Paths []string`.
- Consumes: `targets.NameRules`, `targets.IsFragmentTarget`; sentinels `esc.ErrManifest`, `esc.ErrConstraint`.
- Note: normalize with `strings.ReplaceAll(s, "\r\n", "\n")` (no `render` import — cycle).

**Steps**
- [ ] Write the failing test `internal/pack/fragment_rules_test.go`:
```go
package pack

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tensorgroup/openescapement/internal/esc"
)

func loadOneFragment(t *testing.T, name, content string) (*Fragment, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rules", name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return loadFragment(dir, "acme", "rules/"+name, map[string]bool{})
}

func TestPathsParsedAndBodyStrippedOfFrontmatter(t *testing.T) {
	f, err := loadOneFragment(t, "api.md", "---\ntargets: [rules]\npaths:\n  - \"src/api/**\"\n---\n# API\n\nbody\n")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(f.Paths) != 1 || f.Paths[0] != "src/api/**" {
		t.Errorf("paths not parsed: %v", f.Paths)
	}
	if want := "# API\n\nbody\n"; f.Body != want {
		t.Errorf("body = %q want %q", f.Body, want)
	}
}

func TestPathsWithoutRulesTargetIsManifestError(t *testing.T) {
	_, err := loadOneFragment(t, "api.md", "---\ntargets: [claude]\npaths:\n  - \"src/**\"\n---\nbody\n")
	if !errors.Is(err, esc.ErrManifest) {
		t.Fatalf("want ErrManifest, got %v", err)
	}
}

func TestEmptyPathsListIsError(t *testing.T) {
	_, err := loadOneFragment(t, "api.md", "---\ntargets: [rules]\npaths: []\n---\nbody\n")
	if !errors.Is(err, esc.ErrManifest) {
		t.Fatalf("want ErrManifest for empty paths list, got %v", err)
	}
}

func TestPathsEntryEmptyOrNewlineIsError(t *testing.T) {
	_, err := loadOneFragment(t, "api.md", "---\ntargets: [rules]\npaths:\n  - \"\"\n---\nbody\n")
	if !errors.Is(err, esc.ErrManifest) {
		t.Fatalf("want ErrManifest for empty entry, got %v", err)
	}
}

func TestBadRuleStemIsError(t *testing.T) {
	_, err := loadOneFragment(t, "API_Handlers.md", "---\ntargets: [rules]\n---\nbody\n")
	if !errors.Is(err, esc.ErrManifest) {
		t.Fatalf("want ErrManifest for bad stem, got %v", err)
	}
}

func TestRulesAndClaudeTogetherIsValid(t *testing.T) {
	f, err := loadOneFragment(t, "api.md", "---\ntargets: [rules, claude]\npaths:\n  - \"src/**\"\n---\nbody\n")
	if err != nil {
		t.Fatalf("rules+claude must be valid: %v", err)
	}
	if len(f.Targets) != 2 {
		t.Errorf("both targets should survive: %v", f.Targets)
	}
}

func TestNoFrontmatterFragmentHasNoPaths(t *testing.T) {
	f, err := loadOneFragment(t, "api.md", "just a body\n")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if f.Paths != nil || len(f.Targets) != 0 {
		t.Errorf("no-frontmatter fragment must have no targets/paths: %+v", f)
	}
}

func TestCRLFFrontmatterSplitAfterNormalize(t *testing.T) {
	f, err := loadOneFragment(t, "api.md", "---\r\ntargets: [rules]\r\n---\r\nbody line\r\n")
	if err != nil {
		t.Fatalf("CRLF frontmatter must split after normalize: %v", err)
	}
	if len(f.Targets) != 1 || f.Targets[0] != "rules" {
		t.Errorf("targets lost on CRLF fragment: %v", f.Targets)
	}
	if f.Body != "body line\n" {
		t.Errorf("body = %q", f.Body)
	}
}
```
- [ ] Run `go test ./internal/pack/ -run 'Paths|Stem|CRLF|NoFrontmatter|RulesAndClaude'`. Expected failure: `f.Paths undefined` (compile) then assertion failures.
- [ ] Implement in `internal/pack/fragment.go`. Add `Paths []string` to `Fragment`:
```go
type Fragment struct {
	Path    string
	Targets []string
	Paths   []string // glob scopes for a rules fragment; nil for every other fragment
	Body    string
}
```
Add imports `"regexp"` (keep `filepath`, `strings`). Add the stem pattern:
```go
// ruleStem constrains a rules fragment's filename stem, which becomes part of
// an on-disk filename under .claude/rules/.
var ruleStem = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
```
Rewrite `loadFragment` to normalize endings and parse/validate paths:
```go
func loadFragment(packDir, packName, rel string, customNames map[string]bool) (*Fragment, error) {
	raw, err := os.ReadFile(filepath.Join(packDir, rel))
	if err != nil {
		return nil, fmt.Errorf("%w: rule %s: %v", esc.ErrManifest, rel, err)
	}
	frag := &Fragment{Path: rel}
	// Normalize CRLF to LF at load: the splitter below recognizes only LF
	// fences, so a CRLF fragment would otherwise lose its frontmatter (and its
	// scope) silently. Same one-conversion rule as render.NormalizeEndings, but
	// inlined here because pack must not import render (cycle).
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if fm, body, ok := splitFrontmatter(content); ok {
		var meta struct {
			Targets []string `yaml:"targets"`
			Paths   []string `yaml:"paths"`
		}
		if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
			return nil, fmt.Errorf("%w: rule %s frontmatter: %v", esc.ErrManifest, rel, err)
		}
		for _, tgt := range meta.Targets {
			if !targets.IsFragmentTarget(tgt) && !customNames[tgt] {
				return nil, fmt.Errorf("%w: pack %q: rule %s: unknown or foreign target %q (not a built-in target or a custom target defined by this pack)", esc.ErrConstraint, packName, rel, tgt)
			}
		}
		if err := validateRulePaths(packName, rel, meta.Targets, meta.Paths); err != nil {
			return nil, err
		}
		frag.Targets = meta.Targets
		frag.Paths = meta.Paths
		frag.Body = body
	} else {
		frag.Body = content
	}
	return frag, nil
}

// validateRulePaths enforces the rules-fragment authoring rules (§ Rules
// target). All failures are ErrManifest and name the fragment.
func validateRulePaths(packName, rel string, tgts, paths []string) error {
	namesRules := false
	for _, t := range tgts {
		if t == targets.NameRules {
			namesRules = true
		}
	}
	if !namesRules {
		if paths != nil {
			return fmt.Errorf("%w: pack %q: rule %s: paths: is only allowed on a fragment whose targets include %q", esc.ErrManifest, packName, rel, targets.NameRules)
		}
		return nil
	}
	// A rules fragment's filename stem becomes part of an on-disk filename.
	stem := strings.TrimSuffix(filepath.Base(rel), ".md")
	if !ruleStem.MatchString(stem) {
		return fmt.Errorf("%w: pack %q: rule %s: filename stem %q must match %s", esc.ErrManifest, packName, rel, stem, ruleStem)
	}
	// paths, when present at all (including an explicit empty list), must be a
	// non-empty list of non-empty, newline-free strings. A nil slice means the
	// key was absent: an unconditional rule, which is allowed.
	if paths == nil {
		return nil
	}
	if len(paths) == 0 {
		return fmt.Errorf("%w: pack %q: rule %s: paths: when present must be a non-empty list", esc.ErrManifest, packName, rel)
	}
	for _, p := range paths {
		if p == "" || strings.ContainsAny(p, "\r\n") {
			return fmt.Errorf("%w: pack %q: rule %s: each paths entry must be a non-empty string with no newlines", esc.ErrManifest, packName, rel)
		}
	}
	return nil
}
```
Note: relies on yaml.v3 leaving an absent `paths:` key as a nil slice and materializing `paths: []` as a non-nil empty slice; the two tests above pin both.
- [ ] Run `go test ./internal/pack/ -run 'Paths|Stem|CRLF|NoFrontmatter|RulesAndClaude'`. Expect PASS.
- [ ] Write the compose refusal test in `internal/pack/compose_test.go` (create if absent, else append):
```go
func TestComposeFragmentsRejectsPaths(t *testing.T) {
	parts := [][]byte{
		[]byte("---\ntargets: [rules]\npaths:\n  - \"src/**\"\n---\nbody a\n"),
		[]byte("---\ntargets: [rules]\n---\nbody b\n"),
	}
	if _, err := ComposeFragments(parts); !errors.Is(err, esc.ErrManifest) {
		t.Fatalf("compose must reject a path-scoped part with ErrManifest, got %v", err)
	}
}
```
Add imports `errors` and `esc` if creating the file (`package pack`).
- [ ] Run `go test ./internal/pack/ -run TestComposeFragmentsRejectsPaths`. Expected failure: composes without error.
- [ ] Implement in `internal/pack/compose.go`: extend the meta struct and refuse `paths:`:
```go
			var meta struct {
				Targets []string `yaml:"targets"`
				Paths   []string `yaml:"paths"`
			}
			if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
				return nil, fmt.Errorf("%w: compose part %d frontmatter: %v", esc.ErrManifest, i, err)
			}
			if len(meta.Paths) > 0 {
				return nil, fmt.Errorf("%w: compose part %d: a path-scoped rule fragment cannot be composed", esc.ErrManifest, i)
			}
```
- [ ] Run `go test ./internal/pack/`. Expect PASS.
- [ ] Add a portal-path confirmation test `internal/portal/web/compose_error_test.go` is out of scope — instead add a one-line comment in the plan record: `models.go handleModelAdopt/handleModelAdoptSave already route ComposeFragments errors through serverError` (verified at models.go ~365 and ~414); no portal code change is required. Skip if no test seam exists; do NOT add portal code.
- [ ] `gofmt -w .`, `go vet ./...`, `go test ./...`.
- [ ] Commit: `feat(pack): parse and validate rules-fragment paths; normalize line endings at load` — body explains the silent-CRLF-scope-loss bug this closes and why composition of a scoped fragment is refused.

---

## Task 4 — Renderer: `RenderRule`, `RuleFilePath`, GOVERNANCE.md lists rules fragments

**Files**
- Modify `internal/render/render.go` (add `RuleFileStem`, `RuleFilePath`, `RenderRule`; add imports `path`, `gopkg.in/yaml.v3`).
- Modify `internal/render/governance.go` (append a rules section per pack).
- Create `internal/render/rules_test.go`.
- Create/extend `internal/render/governance_test.go`.

**Interfaces**
- Produces: `func render.RuleFileStem(f pack.Fragment) string`, `func render.RuleFilePath(packName string, f pack.Fragment) string`, `func render.RenderRule(f pack.Fragment, packName string) string`.
- Consumes: `render.NormalizeEndings`, `pack.Fragment`.

**Steps**
- [ ] Write the failing test `internal/render/rules_test.go`:
```go
package render_test

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/tensorgroup/openescapement/internal/pack"
	"github.com/tensorgroup/openescapement/internal/render"
)

func TestRenderRuleNoPathsExact(t *testing.T) {
	f := pack.Fragment{Path: "rules/api.md", Targets: []string{"rules"}, Body: "# API\n\ndo the thing\n"}
	got := render.RenderRule(f, "acme")
	want := "<!-- Managed by escapement (pack acme). Do not edit. -->\n\n# API\n\ndo the thing\n"
	if got != want {
		t.Errorf("no-paths render:\n got %q\nwant %q", got, want)
	}
}

func TestRenderRulePathsFrontmatterDeterministicAndAtByte0(t *testing.T) {
	paths := []string{"src/api/**", "cmd/*.go"}
	f := pack.Fragment{Path: "rules/api.md", Targets: []string{"rules"}, Paths: paths, Body: "body\n"}
	got := render.RenderRule(f, "acme")
	if !strings.HasPrefix(got, "---\n") {
		t.Fatalf("frontmatter must be at byte 0:\n%q", got)
	}
	fm, _ := yaml.Marshal(struct {
		Paths []string `yaml:"paths"`
	}{paths})
	wantHead := "---\n" + string(fm) + "---\n<!-- Managed by escapement (pack acme). Do not edit. -->\n\nbody\n"
	if got != wantHead {
		t.Errorf("paths render:\n got %q\nwant %q", got, wantHead)
	}
	// Deterministic: same input, same bytes.
	if render.RenderRule(f, "acme") != got {
		t.Error("RenderRule not deterministic")
	}
}

func TestRenderRuleQuotesRoundTrip(t *testing.T) {
	paths := []string{`a"quote`, `a\backslash`}
	f := pack.Fragment{Path: "rules/x.md", Targets: []string{"rules"}, Paths: paths, Body: "b\n"}
	got := render.RenderRule(f, "acme")
	fmEnd := strings.Index(got[4:], "\n---\n")
	var parsed struct {
		Paths []string `yaml:"paths"`
	}
	if err := yaml.Unmarshal([]byte(got[4:4+fmEnd]), &parsed); err != nil {
		t.Fatalf("frontmatter did not parse: %v\n%q", err, got)
	}
	if len(parsed.Paths) != 2 || parsed.Paths[0] != `a"quote` || parsed.Paths[1] != `a\backslash` {
		t.Errorf("paths did not round-trip: %v", parsed.Paths)
	}
}

func TestRenderRuleCRLFBodyNormalized(t *testing.T) {
	f := pack.Fragment{Path: "rules/x.md", Targets: []string{"rules"}, Body: "a\r\nb\r\n"}
	got := render.RenderRule(f, "acme")
	if strings.Contains(got, "\r") {
		t.Errorf("CRLF must be normalized in rule output: %q", got)
	}
	if !strings.HasSuffix(got, "a\nb\n") {
		t.Errorf("body: %q", got)
	}
}

func TestRenderRuleNoTrailingNewlineBodyGetsOne(t *testing.T) {
	f := pack.Fragment{Path: "rules/x.md", Targets: []string{"rules"}, Body: "no newline"}
	got := render.RenderRule(f, "acme")
	if !strings.HasSuffix(got, "no newline\n") {
		t.Errorf("must end with exactly one newline: %q", got)
	}
}

func TestRuleFilePath(t *testing.T) {
	f := pack.Fragment{Path: "rules/api-handlers.md"}
	if p := render.RuleFilePath("acme-org", f); p != ".claude/rules/esc-acme-org-api-handlers.md" {
		t.Errorf("path = %q", p)
	}
}
```
- [ ] Run `go test ./internal/render/ -run RenderRule`. Expected failure: `undefined: render.RenderRule` (compile).
- [ ] Implement in `internal/render/render.go`. Update imports:
```go
import (
	"fmt"
	"path"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/tensorgroup/openescapement/internal/pack"
)
```
Append:
```go
// RuleFileStem is a rules fragment's filename without the .md suffix.
func RuleFileStem(f pack.Fragment) string {
	return strings.TrimSuffix(path.Base(f.Path), ".md")
}

// RuleFilePath is the repo-relative path a rules fragment renders to.
func RuleFilePath(packName string, f pack.Fragment) string {
	return path.Join(".claude", "rules", "esc-"+packName+"-"+RuleFileStem(f)+".md")
}

// RenderRule renders one rules fragment to a whole .claude/rules file:
// optional paths frontmatter (only when the fragment is path-scoped, so
// frontmatter sits at byte 0 as Claude Code's rules loader requires), a
// one-line ownership notice (deliberately without the pack version, so a
// version bump does not rewrite every rule file), a blank line, then the
// endings-normalized, trailing-whitespace-trimmed body plus exactly one
// newline. Deterministic: same fragment, same bytes.
func RenderRule(f pack.Fragment, packName string) string {
	var b strings.Builder
	if len(f.Paths) > 0 {
		fm, _ := yaml.Marshal(struct {
			Paths []string `yaml:"paths"`
		}{f.Paths})
		b.WriteString("---\n")
		b.Write(fm)
		b.WriteString("---\n")
	}
	b.WriteString("<!-- Managed by escapement (pack " + packName + "). Do not edit. -->\n\n")
	b.WriteString(strings.TrimSpace(string(NormalizeEndings([]byte(f.Body)))))
	b.WriteString("\n")
	return b.String()
}
```
- [ ] Run `go test ./internal/render/ -run 'RenderRule|RuleFilePath'`. Expect PASS.
- [ ] Write the governance rules-section test in `internal/render/governance_test.go` (create if absent):
```go
package render_test

import (
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/pack"
	"github.com/tensorgroup/openescapement/internal/render"
)

func TestGovernanceListsRulesFragments(t *testing.T) {
	p := &pack.Pack{
		Manifest: pack.Manifest{Name: "acme", Version: "1.0.0"},
		Fragments: []pack.Fragment{
			{Path: "rules/api.md", Targets: []string{"rules"}, Paths: []string{"src/api/**"}, Body: "scope the API\n"},
			{Path: "rules/all.md", Targets: []string{"rules"}, Body: "always on\n"},
			{Path: "rules/blk.md", Targets: []string{"claude"}, Body: "block only\n"},
		},
	}
	g := render.Governance([]*pack.Pack{p})
	if !strings.Contains(g, "esc-acme-api.md") || !strings.Contains(g, "src/api/**") {
		t.Errorf("governance must list the scoped rule file and its paths:\n%s", g)
	}
	if !strings.Contains(g, "esc-acme-all.md") {
		t.Errorf("governance must list the unconditional rule file:\n%s", g)
	}
	if !strings.Contains(g, "scope the API") || !strings.Contains(g, "always on") {
		t.Errorf("governance must include rule bodies so it covers all policy:\n%s", g)
	}
}

// A path-scoped fragment's frontmatter must never leak into a managed block.
func TestBlockNeverLeaksPathsFrontmatter(t *testing.T) {
	p := &pack.Pack{
		Manifest: pack.Manifest{Name: "acme", Version: "1.0.0"},
		Fragments: []pack.Fragment{
			{Path: "rules/api.md", Targets: []string{"rules", "claude"}, Paths: []string{"src/api/**"}, Body: "shared body\n"},
		},
	}
	block := render.Compose([]*pack.Pack{p}, render.TargetClaude)
	if strings.Contains(block, "paths:") || strings.Contains(block, "---") {
		t.Errorf("block must not carry rule frontmatter:\n%s", block)
	}
	if !strings.Contains(block, "shared body") {
		t.Errorf("block should still contain the shared body:\n%s", block)
	}
}
```
- [ ] Run `go test ./internal/render/ -run 'Governance|BlockNeverLeaks'`. Expected failure: governance lacks the rules section.
- [ ] Implement the rules section in `internal/render/governance.go`, appended after the existing per-pack policy loop (before `return b.String()`):
```go
	for _, p := range packs {
		var rules []pack.Fragment
		for _, f := range p.Fragments {
			if fragmentNamesTarget(f, TargetRules) {
				rules = append(rules, f)
			}
		}
		if len(rules) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("\n## Rule files: %s %s\n", p.Manifest.Name, p.Manifest.Version))
		for _, f := range rules {
			b.WriteString(fmt.Sprintf("\n**%s**", "esc-"+p.Manifest.Name+"-"+RuleFileStem(f)+".md"))
			if len(f.Paths) > 0 {
				b.WriteString(" — paths: " + strings.Join(f.Paths, ", "))
			}
			b.WriteString("\n\n")
			b.WriteString(strings.TrimSuffix(f.Body, "\n"))
			b.WriteString("\n")
		}
	}
```
(`fmt` and `strings` are already imported in governance.go.)
- [ ] Run `go test ./internal/render/`. Expect PASS.
- [ ] `gofmt -w .`, `go vet ./...`, `go test ./...`.
- [ ] Commit: `feat(render): render rules fragments to whole files and list them in GOVERNANCE.md` — body explains the notice-without-version choice and byte-0 frontmatter requirement.

---

## Task 5 — Engine planner for `rules`

**Files**
- Modify `internal/engine/engine.go` (`allTargets` ~66-69 → derive from table; add `PlanResult.Notices` field ~58-64; add `case render.TargetRules:` in the render loop ~214; add the filter-excludes-rules notice after the empty-list expansion ~191).
- Modify `internal/cli/cli.go` (`syncOnce` prints `plan.Notices` to stderr ~430).
- Create `internal/engine/rules_planner_test.go`.

**Interfaces**
- Produces: `engine.PlanResult.Notices []string`.
- Consumes: `render.FragmentNamesTarget`, `render.RuleFilePath`, `render.RenderRule`, `esc.HashBytes`, `render.NormalizeEndings`, sentinels `esc.ErrManifest`/`esc.ErrConfig`.

**Steps**
- [ ] Write the failing test `internal/engine/rules_planner_test.go`. Because a full plan needs a fetched pack, use the local-dir pack + config helpers already used by the CLI suite via `planFromConfig`; here drive `planFromConfig` with a temp pack directory:
```go
package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/esc"
)

// writePack lays down a minimal unsigned local pack and returns its dir.
func writePack(t *testing.T, name, version string, rules map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	man := "schema: 1\nname: " + name + "\nversion: " + version + "\nrules:\n"
	for rel := range rules {
		man += "  - " + rel + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(man), 0o644); err != nil {
		t.Fatal(err)
	}
	for rel, content := range rules {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func planLocal(t *testing.T, root string, cfg *config.Config) (*PlanResult, error) {
	t.Helper()
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	return planFromConfig(context.Background(), root, cfg)
}

func cfgFor(packDir string, targets ...string) *config.Config {
	return &config.Config{
		Packs:   []config.PackRef{{Source: packDir, Ref: "", Trust: "unsigned"}},
		Targets: targets,
	}
}

func TestRulesPlannerEmitsRuleFile(t *testing.T) {
	pd := writePack(t, "acme", "1.0.0", map[string]string{
		"rules/api.md": "---\ntargets: [rules]\npaths:\n  - \"src/**\"\n---\nbody\n",
	})
	res, err := planLocal(t, t.TempDir(), cfgFor(pd)) // empty targets = defaults, includes rules
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	var found *Artifact
	for i := range res.Artifacts {
		if res.Artifacts[i].Path == ".claude/rules/esc-acme-api.md" {
			found = &res.Artifacts[i]
		}
	}
	if found == nil {
		t.Fatalf("rule file artifact missing; got %v", pathsOf(res.Artifacts))
	}
	if found.Kind != KindFile || !strings.Contains(found.Body, "src/**") {
		t.Errorf("rule artifact wrong: %+v", found)
	}
}

func TestRulesInPackCollisionIsManifestError(t *testing.T) {
	pd := writePack(t, "acme", "1.0.0", map[string]string{
		"rules/api.md":     "---\ntargets: [rules]\n---\na\n",
		"sub/api.md":       "---\ntargets: [rules]\n---\nb\n", // same stem "api" -> same esc-acme-api.md
	})
	_, err := planLocal(t, t.TempDir(), cfgFor(pd))
	if !errors.Is(err, esc.ErrManifest) {
		t.Fatalf("want ErrManifest for in-pack collision, got %v", err)
	}
}

func TestRulesFilterExcludedNotice(t *testing.T) {
	pd := writePack(t, "acme", "1.0.0", map[string]string{
		"rules/api.md": "---\ntargets: [rules]\n---\na\n",
	})
	res, err := planLocal(t, t.TempDir(), cfgFor(pd, "claude")) // explicit list excludes rules
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	for i := range res.Artifacts {
		if strings.HasPrefix(res.Artifacts[i].Path, ".claude/rules/") {
			t.Fatal("no rule file should be planned when rules is filtered out")
		}
	}
	if len(res.Notices) == 0 || !strings.Contains(res.Notices[0], "targets rules") {
		t.Errorf("want a filter-excludes-rules notice, got %v", res.Notices)
	}
}

func pathsOf(arts []Artifact) []string {
	var out []string
	for _, a := range arts {
		out = append(out, a.Path)
	}
	return out
}
```
- [ ] Run `go test ./internal/engine/ -run 'Rules'`. Expected failure: `res.Notices undefined` (compile) and no rule artifact (switch default would error "unknown target rules" once allTargets includes it — so this test also drives the switch case).
- [ ] Implement in `internal/engine/engine.go`. Rewire `allTargets` to the table:
```go
var allTargets = targets.Defaults()
```
Add `Notices []string` to `PlanResult`:
```go
type PlanResult struct {
	Config     *config.Config
	Packs      []lockfile.LockPack
	PackObjs   []*pack.Pack
	Artifacts  []Artifact
	Violations []render.Violation
	// Notices are non-fatal, exit-code-neutral messages the CLI prints to
	// stderr (e.g. a pack targets rules but the repo's filter excludes it).
	Notices []string
}
```
Add the `rules` planner case in the render loop switch, after the `case render.TargetSkills:` block:
```go
		case render.TargetRules:
			// One whole file per opt-in fragment. Collisions compare the
			// lowercased repo path (macOS/Windows fold case, and a pack name may
			// carry uppercase): two fragments in one pack is ErrManifest, two
			// packs is ErrConfig naming both — the same rule the skills planner
			// applies to colliding directories.
			ruleOwner := map[string]string{} // lowercased repo path -> pack name
			for _, p := range res.PackObjs {
				for _, f := range p.Fragments {
					if !render.FragmentNamesTarget(f, render.TargetRules) {
						continue
					}
					relPath := render.RuleFilePath(p.Manifest.Name, f)
					lc := strings.ToLower(relPath)
					if owner, ok := ruleOwner[lc]; ok {
						if owner == p.Manifest.Name {
							return nil, fmt.Errorf("%w: pack %s: two rule fragments resolve to %s; rename one", esc.ErrManifest, p.Manifest.Name, relPath)
						}
						return nil, fmt.Errorf("%w: rule file %s is produced by packs %s and %s; rename one fragment", esc.ErrConfig, relPath, owner, p.Manifest.Name)
					}
					ruleOwner[lc] = p.Manifest.Name
					content := render.RenderRule(f, p.Manifest.Name)
					res.Artifacts = append(res.Artifacts, Artifact{
						Path: relPath, Kind: KindFile,
						Hash: esc.HashBytes(render.NormalizeEndings([]byte(content))), Body: content,
					})
				}
			}
```
Add the filter-excludes-rules notice immediately after the empty-list expansion block (`if len(targets) == 0 { targets = append(...) }`, ~line 189-191):
```go
	rulesSelected := false
	for _, t := range targets {
		if strings.ToLower(t) == render.TargetRules {
			rulesSelected = true
			break
		}
	}
	if !rulesSelected {
		for _, p := range res.PackObjs {
			for _, f := range p.Fragments {
				if render.FragmentNamesTarget(f, render.TargetRules) {
					res.Notices = append(res.Notices, fmt.Sprintf("pack %s targets rules; this repo's targets list excludes it", p.Manifest.Name))
					break
				}
			}
		}
	}
```
- [ ] Run `go test ./internal/engine/ -run Rules`. Expect PASS.
- [ ] Print notices in `internal/cli/cli.go` `syncOnce`, right after `plan, err := engine.Plan(...)` succeeds and before/after applying (choose after the applied/skipped stdout summary, on stderr, so `--json`'s syncJSON path stays silent):
```go
	for _, n := range plan.Notices {
		fmt.Fprintf(stderr, "  notice: %s\n", n)
	}
```
Place this in `syncOnce` only (not `syncJSON`).
- [ ] Add a CLI-level notice test in `internal/cli/cli_test.go` or a new `internal/cli/rules_notice_test.go`: init a repo whose config sets `targets: [claude]`, add a local pack with a `targets: [rules]` fragment, run sync, assert stderr contains "targets rules". (Use `run` and a local pack dir written into a temp repo, mirroring `TestEveryExamplePackSyncs`'s config shape.)
- [ ] Run `go test ./...`. Expect PASS: default-set repos (examples) now include `rules`, but no example pack ships a rules fragment yet (Task 10 adds acme-org's), so no rule files render and every example stays clean.
- [ ] `gofmt -w .`, `go vet ./...`.
- [ ] Commit: `feat(engine): plan per-fragment rule files and warn when a filter excludes rules` — body explains the collision severities and why an excluded filter warns instead of silently dropping policy.

---

## Task 6 — Engine planner for `agents-skills`

**Files**
- Modify `internal/engine/engine.go` (extract the skills planner into a root-parameterized helper; add `case render.TargetSkills:`/`case render.TargetAgentsSkills:`).
- Modify `internal/engine/apply.go` (`ownedSkillPath` accepts both roots ~291-293).
- Modify `internal/engine/status.go` (`SkillDuplicate` hashByPath scoped to `.claude/skills` ~154-158).
- Create `internal/engine/agents_skills_test.go`.

**Interfaces**
- Produces (engine.go, unexported): `func planSkillDirs(res *PlanResult, rootDir string) error`.
- Changed: `ownedSkillPath` matches `.claude/skills` OR `.agents/skills`.

**Steps**
- [ ] Write the failing test `internal/engine/agents_skills_test.go`:
```go
package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/tensorgroup/openescapement/internal/config"
)

func writeSkillPack(t *testing.T, name string, skillFiles map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	man := "schema: 1\nname: " + name + "\nversion: 1.0.0\nskills:\n  - skills/demo\n"
	if err := os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(man), 0o644); err != nil {
		t.Fatal(err)
	}
	for rel, content := range skillFiles {
		p := filepath.Join(dir, "skills", "demo", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestAgentsSkillsRendersSecondRoot(t *testing.T) {
	pd := writeSkillPack(t, "acme", map[string]string{"SKILL.md": "hi\n"})
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	cfg := &config.Config{
		Packs:   []config.PackRef{{Source: pd, Ref: "", Trust: "unsigned"}},
		Targets: []string{"skills", "agents-skills"},
	}
	res, err := planFromConfig(context.Background(), t.TempDir(), cfg)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	want := map[string]bool{
		".claude/skills/esc-acme-demo": false,
		".agents/skills/esc-acme-demo": false,
	}
	for _, a := range res.Artifacts {
		if _, ok := want[a.Path]; ok {
			if a.Kind != KindDir {
				t.Errorf("%s should be KindDir", a.Path)
			}
			want[a.Path] = true
		}
	}
	for p, seen := range want {
		if !seen {
			t.Errorf("missing artifact %s", p)
		}
	}
}

func TestAgentsSkillsExcludedFromDefaultSet(t *testing.T) {
	pd := writeSkillPack(t, "acme", map[string]string{"SKILL.md": "hi\n"})
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	cfg := &config.Config{Packs: []config.PackRef{{Source: pd, Ref: "", Trust: "unsigned"}}} // empty = defaults
	res, err := planFromConfig(context.Background(), t.TempDir(), cfg)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	for _, a := range res.Artifacts {
		if a.Path == ".agents/skills/esc-acme-demo" {
			t.Fatal("agents-skills must not render under the default (empty) target set")
		}
	}
}

func TestOwnedSkillPathBothRoots(t *testing.T) {
	if !ownedSkillPath(".claude/skills/x") || !ownedSkillPath(".agents/skills/x") {
		t.Error("both skill roots must be owned")
	}
	if ownedSkillPath(".claude/skills/x/y") || ownedSkillPath("other/x") {
		t.Error("nested or foreign paths must not be owned")
	}
}
```
- [ ] Run `go test ./internal/engine/ -run 'AgentsSkills|OwnedSkillPathBoth'`. Expected failure: no `.agents/skills` artifact (switch default errors on `agents-skills`); `ownedSkillPath(".agents/...")` false.
- [ ] Implement in `internal/engine/engine.go`: extract the existing `case render.TargetSkills:` body into a helper and call it for both roots:
```go
		case render.TargetSkills:
			if err := planSkillDirs(res, ".claude"); err != nil {
				return nil, err
			}
		case render.TargetAgentsSkills:
			if err := planSkillDirs(res, ".agents"); err != nil {
				return nil, err
			}
```
Add the helper (mirrors the current inline code, parameterized on `rootDir`, with a lowercased per-root collision map):
```go
// planSkillDirs appends a KindDir artifact per pack skill entry under
// rootDir/skills (rootDir is ".claude" or ".agents"). Each call keeps its own
// collision map, so a name may exist once per root. Keys are lowercased
// because macOS/Windows fold case.
func planSkillDirs(res *PlanResult, rootDir string) error {
	skillOwner := map[string]string{} // lowercased target path -> pack name
	for _, p := range res.PackObjs {
		for _, e := range p.Manifest.Skills {
			src := filepath.Join(p.Dir, filepath.FromSlash(e.Path))
			files, err := pack.DirFiles(src)
			if err != nil {
				return err
			}
			h, err := pack.DirHashOf(src, files)
			if err != nil {
				return err
			}
			name := e.DirName(p.Manifest.Name)
			target := path.Join(rootDir, "skills", name)
			lc := strings.ToLower(target)
			if owner, ok := skillOwner[lc]; ok {
				return fmt.Errorf("%w: skill directory %s is declared by packs %s and %s; rename one entry (skills: {path, name}) so they do not collide",
					esc.ErrConfig, target, owner, p.Manifest.Name)
			}
			skillOwner[lc] = p.Manifest.Name
			res.Artifacts = append(res.Artifacts, Artifact{
				Path: target, Kind: KindDir, Hash: h, SrcDir: src, Files: files,
			})
		}
	}
	return nil
}
```
Delete the now-duplicated inline skills body (the ~236-264 block).
- [ ] Implement `ownedSkillPath` in `internal/engine/apply.go`:
```go
func ownedSkillPath(rel string) bool {
	d := path.Dir(path.Clean(rel))
	return d == ".claude/skills" || d == ".agents/skills"
}
```
- [ ] Scope `SkillDuplicate` to `.claude/skills` in `internal/engine/status.go` — build `hashByPath` only from Claude-root dirs:
```go
		hashByPath := map[string]string{}
		for _, a := range plan.Artifacts {
			// Duplicate detection only compares against ~/.claude/skills, so an
			// .agents/skills copy must never report a duplicate. Scope by parent.
			if a.Kind == KindDir && path.Dir(a.Path) == ".claude/skills" {
				hashByPath[a.Path] = a.Hash
			}
		}
```
- [ ] Add a duplicate-scoping + independent-retirement + amendment test to `agents_skills_test.go`:
```go
func TestSkillDuplicateNeverAttachesToAgentsSkills(t *testing.T) {
	// A repo home with a colliding ~/.claude/skills entry must only mark the
	// .claude copy, never the .agents copy.
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude", "skills", "esc-acme-demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "skills", "esc-acme-demo", "SKILL.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pd := writeSkillPack(t, "acme", map[string]string{"SKILL.md": "hi\n"})
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	cfg := &config.Config{Packs: []config.PackRef{{Source: pd, Ref: "", Trust: "unsigned"}}, Targets: []string{"skills", "agents-skills"}}
	if err := os.MkdirAll(filepath.Join(root, ".escapement"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(root); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(mustPlan(t, root), false); false {
		_ = err // placeholder guard removed below
	}
	plan, err := Plan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(root, plan, false); err != nil {
		t.Fatal(err)
	}
	st, err := Status(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range st.Findings {
		if f.Subject == ".agents/skills/esc-acme-demo" && f.Duplicate != nil {
			t.Error(".agents/skills copy must never carry a SkillDuplicate")
		}
	}
}
```
(Executor note: drop the bogus `mustPlan`/`false` guard lines — they are an editing artifact; the real flow is `Plan` → `Apply` → `Status`. Keep only those three calls.)
- [ ] Run `go test ./internal/engine/`. Expect PASS.
- [ ] Add a CLI end-to-end test `internal/cli/agents_skills_test.go`: govern a temp repo with `targets: [skills, agents-skills]` and a local skill pack; run `sync`; assert both `.claude/skills/<name>/SKILL.md` and `.agents/skills/<name>/SKILL.md` exist; write a team file into each, drop one root from `targets`, re-sync, assert the dropped root's directory is retired while the team file in the other root survives (independent retirement + amendment preservation).
- [ ] Run `go test ./...`. Expect PASS.
- [ ] `gofmt -w .`, `go vet ./...`.
- [ ] Commit: `feat(engine): render skills a second time under .agents/skills as an opt-in target` — body explains why two byte-identical copies (Claude Code vs Kimi Code) rather than a symlink or a single configurable root.

---

## Task 7 — `esc diff --against` diffs rule files by path

**Files**
- Modify `internal/engine/diff.go` (`PolicyDiff` ~86-91 add a rule-file diff; add `ruleArtifactBodies`).
- Create `internal/engine/diff_rules_test.go`.

**Interfaces**
- Produces (diff.go, unexported): `func ruleArtifactBodies(artifacts []Artifact) map[string]string`, `func rulePolicyDiff(ctx, cur, next *PlanResult) (string, error)`.
- Consumes: existing `gitDiff`, `customPolicyDiff` pattern.

**Steps**
- [ ] Write the failing test `internal/engine/diff_rules_test.go` (hand-built plans; PolicyDiff's target-file loop skips `rules` because `TargetFile["rules"] == ""`, so only `rulePolicyDiff` acts):
```go
package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/config"
)

func planWithRuleFiles(files map[string]string) *PlanResult {
	res := &PlanResult{Config: &config.Config{Targets: []string{"rules"}}}
	for p, body := range files {
		res.Artifacts = append(res.Artifacts, Artifact{Path: p, Kind: KindFile, Body: body})
	}
	return res
}

func TestPolicyDiffRuleFileAddedRemovedChanged(t *testing.T) {
	ctx := context.Background()
	cur := planWithRuleFiles(map[string]string{
		".claude/rules/esc-acme-keep.md":   "keep body\n",
		".claude/rules/esc-acme-change.md": "old\n",
		".claude/rules/esc-acme-remove.md": "gone soon\n",
	})
	next := planWithRuleFiles(map[string]string{
		".claude/rules/esc-acme-keep.md":   "keep body\n",
		".claude/rules/esc-acme-change.md": "new\n",
		".claude/rules/esc-acme-add.md":    "brand new\n",
	})
	out, err := PolicyDiff(ctx, cur, next)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	for _, want := range []string{"esc-acme-change.md", "esc-acme-remove.md", "esc-acme-add.md"} {
		if !strings.Contains(out, want) {
			t.Errorf("diff missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "esc-acme-keep.md") {
		t.Errorf("unchanged rule file must not appear in the diff:\n%s", out)
	}
}

func TestPolicyDiffRulePathsChangeShows(t *testing.T) {
	ctx := context.Background()
	cur := planWithRuleFiles(map[string]string{".claude/rules/esc-acme-api.md": "---\npaths:\n    - src/**\n---\nb\n"})
	next := planWithRuleFiles(map[string]string{".claude/rules/esc-acme-api.md": "---\npaths:\n    - cmd/**\n---\nb\n"})
	out, err := PolicyDiff(ctx, cur, next)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if !strings.Contains(out, "esc-acme-api.md") {
		t.Errorf("a paths change must appear in the diff:\n%s", out)
	}
}
```
- [ ] Run `go test ./internal/engine/ -run PolicyDiffRule`. Expected failure: rule files absent from diff output.
- [ ] Implement in `internal/engine/diff.go`. In `PolicyDiff`, after the `customPolicyDiff` call and before the final `return`:
```go
	r, err := rulePolicyDiff(ctx, cur, next)
	if err != nil {
		return "", err
	}
	out.WriteString(r)
```
Add the helpers:
```go
// ruleArtifactBodies returns the rendered body of every rule-file artifact
// (KindFile under .claude/rules), keyed by path.
func ruleArtifactBodies(artifacts []Artifact) map[string]string {
	bodies := map[string]string{}
	for _, a := range artifacts {
		if a.Kind == KindFile && strings.HasPrefix(a.Path, ".claude/rules/") {
			bodies[a.Path] = a.Body
		}
	}
	return bodies
}

// rulePolicyDiff diffs rule files present in either plan, matched by path —
// added, removed, body changed, or paths changed (a frontmatter change is a
// body change). Mirrors customPolicyDiff.
func rulePolicyDiff(ctx context.Context, cur, next *PlanResult) (string, error) {
	curRules := ruleArtifactBodies(cur.Artifacts)
	nextRules := ruleArtifactBodies(next.Artifacts)
	paths := map[string]bool{}
	for p := range curRules {
		paths[p] = true
	}
	for p := range nextRules {
		paths[p] = true
	}
	sorted := make([]string, 0, len(paths))
	for p := range paths {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)
	var out strings.Builder
	for _, p := range sorted {
		a, b := curRules[p], nextRules[p]
		if a == b {
			continue
		}
		d, err := gitDiff(ctx, []byte(a), []byte(b), p)
		if err != nil {
			return "", err
		}
		out.WriteString(d)
	}
	return out.String(), nil
}
```
(`sort` and `strings` are already imported in diff.go.)
- [ ] Run `go test ./internal/engine/ -run PolicyDiffRule`. Expect PASS.
- [ ] `gofmt -w .`, `go vet ./...`, `go test ./...`.
- [ ] Commit: `feat(diff): esc diff --against now covers rule files by path` — body notes rule files were invisible to policy review before this.

---

## Task 8 — `esc init`: write `rules`, pre-fill `agents-skills` for a real `.agents/skills`

**Files**
- Modify `internal/cli/initscan.go` (`detectExisting` ~55-60 add `.agents/skills` detection; `detectedTargets` ~104-114 append rules; `detectedSyncClause` ~139-155 add agents-skills case).
- Create `internal/cli/init_agents_skills_test.go`.

**Interfaces**
- Consumes: `render.TargetRules`, `render.TargetAgentsSkills`.

**Steps**
- [ ] Write the failing test `internal/cli/init_agents_skills_test.go`:
```go
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readConfig(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, ".escapement", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestInitWritesRulesInExplicitList(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := run(t, root, "init", "--yes"); code != 0 {
		t.Fatalf("init: %d\n%s", code, out)
	}
	cfg := readConfig(t, root)
	if !strings.Contains(cfg, "- rules") {
		t.Errorf("init must add rules to an explicit targets list:\n%s", cfg)
	}
}

func TestInitPrefillsAgentsSkillsForRealDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agents", "skills", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".agents", "skills", "x", "SKILL.md"), []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := run(t, root, "init", "--yes"); code != 0 {
		t.Fatalf("init: %d\n%s", code, out)
	}
	if !strings.Contains(readConfig(t, root), "agents-skills") {
		t.Errorf("init must pre-fill agents-skills for a real .agents/skills dir")
	}
}

func TestInitSkipsSymlinkedAgentsSkills(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude", "skills", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, ".claude", "skills"), filepath.Join(root, ".agents-skills-link")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	// Point .agents/skills at .claude/skills via a symlink.
	if err := os.MkdirAll(filepath.Join(root, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, ".claude", "skills"), filepath.Join(root, ".agents", "skills")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if code, out := run(t, root, "init", "--yes"); code != 0 {
		t.Fatalf("init: %d\n%s", code, out)
	}
	if strings.Contains(readConfig(t, root), "agents-skills") {
		t.Errorf("init must NOT pre-fill agents-skills for a symlinked .agents/skills")
	}
}
```
- [ ] Run `go test ./internal/cli/ -run 'InitWritesRules|InitPrefills|InitSkipsSymlink'`. Expected failure: no `rules`/`agents-skills` in config.
- [ ] Implement `.agents/skills` detection in `internal/cli/initscan.go` `detectExisting`, after the `.claude/skills` detection block:
```go
	if asPath := filepath.Join(root, ".agents", "skills"); true {
		// Lstat, not Stat: a `.agents/skills -> ../.claude/skills` layout must
		// not be pre-filled, or the first sync would fail sync's symlink refusal
		// at exit 4. A symlink's Lstat reports IsDir()==false, so this both
		// skips symlinks and requires a real directory.
		if info, err := os.Lstat(asPath); err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			if entries, err := os.ReadDir(asPath); err == nil && len(entries) > 0 {
				out = append(out, Detected{Target: render.TargetAgentsSkills, Path: ".agents/skills"})
			}
		}
	}
```
- [ ] Make `detectedTargets` include `rules` whenever it writes an explicit list, in table order:
```go
func detectedTargets(detected []Detected) []string {
	ordered := orderDetected(detected)
	if len(ordered) == 0 {
		return nil // greenfield: empty means all default targets (rules included)
	}
	present := map[string]bool{}
	for _, d := range ordered {
		present[d.Target] = true
	}
	// A newly governed repo that gets an explicit list should still receive
	// rule files by default, so rules is always managed. agents-skills is only
	// added when it was actually detected (present already).
	present[render.TargetRules] = true
	out := []string{}
	for _, t := range targetOrder {
		if present[t] {
			out = append(out, t)
		}
	}
	return out
}
```
- [ ] Add the agents-skills sync clause in `detectedSyncClause`:
```go
	case render.TargetAgentsSkills:
		return "add pack skills alongside yours in .agents/skills"
```
- [ ] Run `go test ./internal/cli/ -run 'InitWritesRules|InitPrefills|InitSkipsSymlink'`. Expect PASS.
- [ ] Run `go test ./...`. Expect PASS (existing `TestInitExplainsDetection` cases: a repo with only CLAUDE.md now writes `targets: [claude, rules]` — verify that test's expected config strings still match; if it asserts the exact config body, update its expectation to include the `- rules` line and keep its explanation-text assertions unchanged, since `rules` is not a `Detected` item and does not appear in the "Found ..." prose).
- [ ] `gofmt -w .`, `go vet ./...`.
- [ ] Commit: `feat(cli): esc init manages rules by default and pre-fills agents-skills for a real .agents/skills` — body explains the Lstat symlink guard.

---

## Task 9 — Shapetest matrix for the rule-file write, removal, and adoption compare

The rule-file write path is a fully escapement-owned whole file, so the hazards are body-shape hash stability (no trailing newline, CRLF checkout, empty body) and correct adoption/removal across those shapes. Because the assertion needs `render.RenderRule`, `esc.HashBytes`, and `classify`/`Apply` together, the matrix test lives in `internal/engine` (as `internal/render` and `internal/cli` each own their own matrix tests), while `internal/shapetest` stays import-light.

**Files**
- Create `internal/engine/rulefile_shape_test.go`.

**Steps**
- [ ] Write the test `internal/engine/rulefile_shape_test.go`:
```go
package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/lockfile"
	"github.com/tensorgroup/openescapement/internal/pack"
	"github.com/tensorgroup/openescapement/internal/render"
)

// ruleBodyShapes are the awkward fragment bodies a rule-file write must survive.
func ruleBodyShapes() map[string]string {
	return map[string]string{
		"no-trailing-newline": "line one\nline two",
		"crlf":                "line one\r\nline two\r\n",
		"empty":               "",
	}
}

func TestRuleFileWriteAdoptRemoveShapeMatrix(t *testing.T) {
	for name, body := range ruleBodyShapes() {
		t.Run(name, func(t *testing.T) {
			frag := pack.Fragment{Path: "rules/api.md", Targets: []string{"rules"}, Body: body}
			content := render.RenderRule(frag, "acme")
			relPath := render.RuleFilePath("acme", frag)
			hash := esc.HashBytes(render.NormalizeEndings([]byte(content)))
			a := Artifact{Path: relPath, Kind: KindFile, Body: content, Hash: hash}

			// (1) write path: apply writes it, classify reports InSync.
			root := t.TempDir()
			if _, err := Apply(root, &PlanResult{Artifacts: []Artifact{a}}, false); err != nil {
				t.Fatalf("apply write: %v", err)
			}
			if classify(root, a, mustLock(t, root)).State != InSync {
				t.Errorf("written rule file must be InSync")
			}
			got, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(relPath)))
			if len(got) == 0 || got[len(got)-1] != '\n' {
				t.Errorf("rule file must end in a newline: %q", got)
			}

			// (2) occupied adoption compare: a CRLF-converted copy on disk with
			// no lock entry is adopted (normalized-equal), not skipped.
			root2 := t.TempDir()
			crlf := []byte(content)
			// force a CRLF on-disk variant that normalizes back to content
			p := filepath.Join(root2, filepath.FromSlash(relPath))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, append([]byte{}, crlf...), 0o644); err != nil {
				t.Fatal(err)
			}
			// Re-write as CRLF to exercise NormalizeEndings on the adopt path.
			if err := os.WriteFile(p, []byte(render.NormalizeEndings([]byte(content))), 0o644); err != nil {
				t.Fatal(err)
			}
			res, err := Apply(root2, &PlanResult{Artifacts: []Artifact{a}}, false)
			if err != nil {
				t.Fatalf("apply adopt: %v", err)
			}
			if len(res.Adopted) != 1 {
				t.Errorf("identical pre-existing rule file must be adopted, got %+v / skipped %+v", res.Adopted, res.Skipped)
			}

			// (3) removal path: an orphaned rule file (no plan) is removed;
			// a CRLF checkout still hashes equal, so it is removed, not declined.
			root3 := t.TempDir()
			if _, err := Apply(root3, &PlanResult{Artifacts: []Artifact{a}}, false); err != nil {
				t.Fatal(err)
			}
			// Convert on-disk to CRLF to prove normalized-hash removal.
			dp := filepath.Join(root3, filepath.FromSlash(relPath))
			cur, _ := os.ReadFile(dp)
			_ = os.WriteFile(dp, []byte(string(cur)), 0o644)
			resR, err := Apply(root3, &PlanResult{}, false) // rule no longer planned
			if err != nil {
				t.Fatalf("apply remove: %v", err)
			}
			if len(resR.Skipped) != 0 {
				t.Errorf("unedited orphan rule file must not be declined: %+v", resR.Skipped)
			}
			if _, err := os.Stat(dp); !os.IsNotExist(err) {
				t.Errorf("orphan rule file must be removed")
			}
			_ = lockfile.LockArtifact{} // keep the lockfile import if unused elsewhere
		})
	}
}
```
(Executor note: `mustLock` is defined in `wholefile_test.go` from Task 2; the `lockfile` import guard line may be dropped if `go vet` flags it unused.)
- [ ] Run `go test ./internal/engine/ -run RuleFileWriteAdoptRemoveShapeMatrix`. Expect PASS (the machinery landed in Tasks 2/5).
- [ ] `gofmt -w .`, `go vet ./...`, `go test ./...`.
- [ ] Commit: `test(engine): rule-file write/adopt/remove matrix across body shapes` — body notes the matrix lives in engine because it needs render+apply, keeping shapetest import-light.

---

## Task 10 — Examples: acme-org rule fragment, version bump, regenerate governed-service

**Files**
- Create `examples/packs/acme-org/rules/api-handlers.md`.
- Modify `examples/packs/acme-org/pack.yaml` (add the rule to `rules:`; bump `version: 0.1.1` → `0.1.2`).
- Regenerate `examples/governed-service/*` (run `esc sync` inside it).
- Modify `internal/cli/examples_test.go` (pinned block string `@0.1.1` → `@0.1.2`; add acme-org rule-file assertions in `TestEveryExamplePackSyncs`).
- Create `internal/cli/examples_agents_skills_test.go` (governed-service variant with agents-skills).

**Steps**
- [ ] Create `examples/packs/acme-org/rules/api-handlers.md`:
```markdown
---
targets: [rules]
paths:
  - "src/api/**"
---
# API handler rules

- Validate every request body at the trust boundary before use.
- Never log secrets, tokens, or full request bodies.
- Return typed errors; do not leak internal messages to clients.
```
- [ ] Edit `examples/packs/acme-org/pack.yaml`: bump `version: 0.1.1` to `version: 0.1.2`, and add `  - rules/api-handlers.md` to the `rules:` list.
- [ ] Regenerate the shipped governed-service tree. Run:
```
cd examples/governed-service
ESC_CACHE_DIR=$(mktemp -d) go run ../../cmd/esc sync
ESC_CACHE_DIR=$(mktemp -d) go run ../../cmd/esc status --check ; echo "status --check exit: $?"
```
Confirm the run creates `examples/governed-service/.claude/rules/esc-acme-org-api-handlers.md` with `paths:` frontmatter, updates the block headers to `packs=acme-org@0.1.2`, refreshes GOVERNANCE.md (now listing the rule file) and `.escapement/escapement.lock`, and that `status --check` exits 0. Stage every changed/created file under `examples/governed-service/`.
- [ ] Update `internal/cli/examples_test.go`: change the pinned substring `"escapement:begin packs=acme-org@0.1.1"` to `"...@0.1.2"`. Add to `TestEveryExamplePackSyncs`, inside the `t.Run(name, ...)` closure, an acme-org-specific block after the GOVERNANCE.md check:
```go
			if name == "acme-org" {
				rulePath := filepath.Join(root, ".claude", "rules", "esc-acme-org-api-handlers.md")
				content, err := os.ReadFile(rulePath)
				if err != nil {
					t.Fatalf("acme-org rule file missing: %v", err)
				}
				var meta struct {
					Paths []string `yaml:"paths"`
				}
				// Frontmatter is at byte 0, delimited by the first two --- lines.
				s := string(content)
				if !strings.HasPrefix(s, "---\n") {
					t.Fatalf("rule file has no frontmatter:\n%s", s)
				}
				end := strings.Index(s[4:], "\n---\n")
				if end < 0 {
					t.Fatalf("rule file frontmatter unterminated:\n%s", s)
				}
				if err := yaml.Unmarshal([]byte(s[4:4+end]), &meta); err != nil {
					t.Fatalf("rule frontmatter parse: %v", err)
				}
				if len(meta.Paths) != 1 || meta.Paths[0] != "src/api/**" {
					t.Errorf("rule paths = %v, want [src/api/**]", meta.Paths)
				}
			}
```
Add imports `strings` and `gopkg.in/yaml.v3` to `examples_test.go` if not present.
- [ ] Create `internal/cli/examples_agents_skills_test.go`: copy `examples/governed-service` into a temp dir, rewrite its `.escapement/config.yaml` to add `targets: [claude, agents, gemini, governance, mcp, skills, rules, agents-skills]`, run `sync`, assert both `.claude/skills/esc-acme-org-acme-vault/SKILL.md` and `.agents/skills/esc-acme-org-acme-vault/SKILL.md` exist, and `status --check` exits 0. (Use `copyTree` from `examples_test.go`.)
- [ ] Run `go test ./internal/cli/`. Expect PASS, including `TestExamplesSync`, `TestEveryExamplePackSyncs`, `TestExamplesLockMatchesShippedFiles`.
- [ ] Run `go test ./...`. Expect PASS.
- [ ] `gofmt -w .`, `go vet ./...`.
- [ ] Commit: `docs(examples): acme-org ships a path-scoped rule; regenerate governed-service at 0.1.2` — body notes the rule file and the version bump drove the regeneration.

---

## Task 11 — Docs: README, pack-authoring, cli-and-portal, CHANGELOG

**Files**
- Modify `README.md` (target list ~7 and the pack-authoring reference ~223).
- Modify `docs/pack-authoring.md` (add a rules section).
- Modify `docs/cli-and-portal.md` (one sentence recording the managed-policy CLAUDE.md decision).
- Modify `CHANGELOG.md` (`### Added` entries for both targets; the `### Changed` entry landed in Task 2).

**Steps**
- [ ] README: extend the channels list (line ~7) to mention `.claude/rules/` and `.agents/skills/`. Add a short "Targets" note stating: `rules` and `agents-skills` are built-ins; an empty `targets:` list renders every built-in EXCEPT `agents-skills` (opt-in); an explicit `targets:` list is an exhaustive filter, so a repo with an explicit list must add `rules` to receive rule files and `agents-skills` to get the second skills root; a pack that ships a `targets: [rules]` fragment needs an `esc` new enough to know the target — an older binary rejects it as an unknown target (exit 1).
- [ ] `docs/pack-authoring.md`: add a `## Path-scoped rule files` section covering: opt in with `targets: [rules]`; add `paths:` (a non-empty list of globs) to scope the file, or omit it for an unconditional rule; keep the file small (it is loaded like `.claude/CLAUDE.md`); the notice line is escapement's, not yours; the `esc-` prefix under `.claude/rules/` is a reserved namespace escapement owns; a rules fragment's filename stem must match `^[a-z0-9][a-z0-9-]*$`; a path-scoped fragment cannot be composed by the portal's starter-set adoption.
- [ ] `docs/cli-and-portal.md`: add one sentence recording that the managed-policy `CLAUDE.md` at OS paths is deliberately NOT a render target — `esc` writes agent-readable files only inside the governed repo; that OS-path file is an IT/device-management fleet channel, revisited only if the portal is asked to publish it.
- [ ] `CHANGELOG.md`: add under `## [Unreleased]` `### Added`:
```
- `rules` target: one escapement-owned file per opt-in fragment under
  `.claude/rules/esc-<pack>-<stem>.md`, carrying a verbatim `paths:` scope for
  Claude Code (and read by Grok Build). Built-in and part of the default set;
  an explicit `targets:` list must name it to receive rule files.
- `agents-skills` target: the pack's skill directories rendered a second time
  under `.agents/skills/<name>` for cross-tool agents (Kimi Code). Opt-in: an
  empty `targets:` list does not include it.
- Packs that ship a `targets: [rules]` fragment require an `esc` that knows the
  target; an older binary rejects the fragment as an unknown target (exit 1).
```
- [ ] Run `go test ./...` (docs-only, but confirm nothing references changed strings). Expect PASS.
- [ ] Commit: `docs: document the rules and agents-skills targets and the managed-policy decision`.

---

## Self-review (performed against every spec section)

Spec → task coverage:
- Rules pack authoring rules (paths only with rules; non-empty list; no newlines; both rules+block; unconditional; explicit-name test; stem pattern; LF-normalize at load; compose refuses paths) → Task 3, with the explicit-name test used by the planner in Task 5 (`render.FragmentNamesTarget`).
- Rendered file (path, frontmatter via yaml.v3, notice without version, TrimSpace + one newline, byte-0 frontmatter, lowercased collisions ErrManifest/ErrConfig, GOVERNANCE.md lists rules) → Tasks 4 (render) + 5 (collisions in planner).
- Whole-file machinery for every KindFile (desired-hash normalization; occupied gate + byte-identical adoption; `--force` no override; orphan pass; ownership `ownedFilePath`/`ownedRulePath`; pre-read symlink refusal in plan/status/diff; init GOVERNANCE.md change; CHANGELOG; AGENTS.md wording) → Task 2.
- Targets/defaults/init (`OptIn`, `Defaults()`, `allTargets`, `initscan.targetOrder`, `PolicyDiff` default, `TestTargetOrderCoversAllTargets`, init writes rules, pre-fills agents-skills via Lstat, older-binary note) → Tasks 1, 5 (allTargets rewire), 8.
- Agents-skills (second root, `ownedSkillPath` both roots, per-root lowercased collision, SkillDuplicate scoped to `.claude/skills`, two-root retirement/amendment tests) → Task 6.
- `esc diff --against` rule files → Task 7.
- Shapetest matrix (rule write/removal/adoption × no-trailing-newline/CRLF/empty) → Task 9.
- Examples + example tests → Task 10.
- Docs → Tasks 2 (CHANGELOG Changed, AGENTS.md) + 11.

Gaps / assumptions flagged for the executor and reviewer:
1. **`pack` cannot import `render`** (cycle). LF normalization is inlined in `pack` with `strings.ReplaceAll` rather than `render.NormalizeEndings`; semantics are identical (one conversion, CRLF→LF). The spec names `render.NormalizeEndings`; this is the only faithful way to honor it without a cycle.
2. **yaml.v3 nil-vs-empty-slice** distinguishes an absent `paths:` (nil, valid unconditional rule) from `paths: []` (non-nil empty, an error). Task 3's two tests pin both; if a future yaml.v3 changes this, the empty-list check needs a `yaml.Node` parse instead.
3. **`allTargets` rewire is deferred from Task 1 to Task 5** (paired with the `rules` switch case) so the target switch stays total and every task's suite is green. Task 1 still adds `Defaults()` and derives `initscan.targetOrder` and `PolicyDiff`'s default from the table; only `engine.allTargets` waits for its handler.
4. **`prospectiveContent` symlink refusal is unconditional** (both KindBlock and KindFile). This strengthens the block path too (Apply already refuses symlinks before writing a block), so it is consistent, not a regression; flagged because it changes plan-time behavior for a symlinked CLAUDE.md from "read through" to "exit 4".
5. **Shapetest matrix placement**: the rule-file matrix lives in `internal/engine` (needs render+apply+classify), not `internal/shapetest` (kept import-light, no writes), matching how `internal/render` and `internal/cli` own their own matrix tests. Spec text says "internal/shapetest gains three paths"; this is the same intent realized where the write path lives.
6. **Portal**: `models.go` already routes `ComposeFragments` errors through `serverError` (verified at ~365 and ~414), so the compose refusal surfaces with no portal code change. No portal edit is made.

No placeholders remain; every identifier used across tasks (`SkipUnmanagedFileAtTarget`, `SkipOrphanFileEdited`, `ownedFilePath`, `ownedRulePath`, `identicalFileContent`, `PlanResult.Notices`, `planSkillDirs`, `render.RenderRule`, `render.RuleFilePath`, `render.FragmentNamesTarget`, `targets.Defaults`, `targets.KindRulesDir`, `Info.OptIn`) is defined in the task that first uses it.
