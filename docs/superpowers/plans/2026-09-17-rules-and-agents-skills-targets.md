# Rules and Agents-Skills Render Targets Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two built-in render targets. `rules`: one escapement-owned whole file per opt-in fragment under `.claude/rules/esc-<pack>-<stem>.md`, carrying a verbatim `paths:` frontmatter scope; `agents-skills`: the pack's skill directories rendered a second time under `.agents/skills/<name>`, off by default. Along the way, build the whole-file occupied gate and orphan pass that no `KindFile` artifact has today (proven first on `GOVERNANCE.md`).

**Architecture:** `esc` is a Go CLI with a `Plan → Apply` pipeline. `internal/targets` holds the built-in target table; `internal/pack` loads/validates fragments; `internal/render` composes text; `internal/engine` plans artifacts, classifies drift (`status.go`), writes/removes them (`apply.go`), and diffs (`diff.go`). Artifacts are one of `KindBlock`/`KindFile`/`KindDir`/`KindJSONKeys`. Rule files are `KindFile`; agents-skills copies are `KindDir`.

**Tech Stack:** Go 1.24, module `github.com/tensorgroup/openescapement`, binary `esc`. Single external dependency `gopkg.in/yaml.v3`; everything else stdlib; system `git` via `os/exec`.

**Spec:** docs/superpowers/specs/2026-09-17-rules-and-agents-skills-targets-design.md

## Global Constraints
- Dependency policy: `gopkg.in/yaml.v3` only; everything else stdlib. Adding any dependency is out of scope.
- `internal/pack` must NOT import `internal/render` (render imports pack — that direction is a cycle). Normalize line endings inside pack with stdlib `strings.ReplaceAll`, not `render.NormalizeEndings`. `internal/render` MAY import `internal/targets` (targets imports only stdlib), which is how the target-name strings are defined once.
- Exit-code sentinels (`internal/esc`): 0 ok, 1 `ErrConstraint` (drift/constraint), 2 `ErrConfig`/`ErrUsage` (usage/config), 3 `ErrSignature`/`ErrLockMismatch` (integrity), 4 plain error (containment/other).
- Containment refusals (`refuseSymlinks`, escaping lockfile entries, a non-canonical owned path) return a PLAIN error → exit 4, never `ErrConstraint`.
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
| `internal/targets/targets.go` | Built-in target table (mcp reordered before skills; rules, agents-skills appended); `OptIn`, `KindRulesDir`, `Defaults()` | 1 |
| `internal/targets/targets_test.go` | `TestBuiltIns` (existing) updated to 8 rows + Defaults test | 1 |
| `internal/render/render.go` | `Target*` consts defined as `targets.Name*`; `FragmentNamesTarget`, `RuleFilePath`, `RuleFileStem`, `RenderRule` | 1,4 |
| `internal/render/rules_test.go`, `governance_test.go` | RenderRule + governance golden tests | 4 |
| `internal/render/governance.go` | GOVERNANCE.md lists rules fragments | 4 |
| `internal/cli/initscan.go` | `targetOrder` derived from table; `.agents/skills` detection; `detectedTargets` appends rules; governance sync clause; `explainDetection` governance handling | 1,2,8 |
| `internal/cli/initscan_test.go` | `TestTargetOrderCoversAllTargets` → equality; six-target explanation updated | 1,2 |
| `internal/cli/initoffer.go`, `initoffer_test.go` | governance excluded from the placement offer | 2 |
| `internal/engine/engine.go` | `PlanResult.Notices`, governance hash fix, rules planner, agents-skills planner, `allTargets` from table, `isBuiltInTarget` | 2,5,6 |
| `internal/engine/apply.go` | KindFile occupied gate + adoption, orphan-file pass (+empty-dir cleanup), canonical `ownedFilePath`/`ownedRulePath`, `identicalFileContent`, `ownedSkillPath` both roots, new skip causes, `refuseSymlinks` message | 2,5,6 |
| `internal/engine/status.go` | KindFile occupied classify, orphan-file pass (errors propagate), SkillDuplicate scoped to `.claude/skills` | 2,6 |
| `internal/engine/diff.go` | `prospectiveContent` symlink refusal, rule-file diff, default list from table | 2,7 |
| `internal/pack/fragment.go` | LF normalize at load, `Fragment.Paths` (yaml.Node parse), rule-path validation | 3 |
| `internal/pack/pack.go` | in-pack rule-filename collision (`ErrManifest`) | 3 |
| `internal/pack/compose.go` | `ComposeFragments` normalizes endings + refuses `paths:` (incl. single-part path) | 3 |
| `internal/cli/cli.go` | print `plan.Notices`, `skipHint` cases, adopted-line wording, init rules/agents-skills wiring | 2,5,8 |
| `internal/cli/custom_targets_test.go`, `internal/engine/orphandir_test.go` | symlink-message assertions updated | 2 |
| `README.md` | target list; skip-cause enum | 2,11 |
| `examples/packs/acme-org/*`, `examples/governed-service/*` | rule fragment, version bump, regenerate | 10 |
| `docs/pack-authoring.md`, `docs/cli-and-portal.md`, `CHANGELOG.md`, `AGENTS.md` | docs/invariants | 2,11 |

---

## Task 1 — Targets table, `OptIn`, `KindRulesDir`, `Defaults()`, render constants single-sourced, derived target order

**Files**
- Modify `internal/targets/targets.go` (built-in name consts ~15-22, kind consts ~24-30, `Info` struct ~35-45, `builtIns` table ~47-54 — REORDER so `mcp` precedes `skills`, then append `rules` and `agents-skills` — `IsFragmentTarget` ~82-85; add `Defaults`).
- Modify `internal/targets/targets_test.go` (`TestBuiltIns` ~5-38 already exists; update its expected rows to the eight-row table and append the new test — do NOT create the file).
- Modify `internal/render/render.go` (const block ~12-19: redefine every `Target*` as the matching `targets.Name*` and add `TargetRules`/`TargetAgentsSkills`; import `internal/targets`; add `FragmentNamesTarget` near ~99).
- Modify `internal/cli/initscan.go` (`targetOrder` ~76-79 → derived from `targets.BuiltIns()`).
- Modify `internal/cli/initscan_test.go` (`TestTargetOrderCoversAllTargets` ~123 → equality against the table order).
- Modify `internal/engine/engine.go` (`isBuiltInTarget` ~412-419 → delegate to `targets.IsBuiltInName`) and `internal/engine/diff.go` (`PolicyDiff` default list ~63-65 → `targets.Defaults()`).

**Interfaces**
- Produces: `targets.NameRules = "rules"`, `targets.NameAgentsSkills = "agents-skills"`, `targets.KindRulesDir = "rules-dir"`, `targets.Info.OptIn bool`, `func targets.Defaults() []string`.
- Produces: `render.TargetRules` / `render.TargetAgentsSkills` (defined as `targets.NameRules` / `targets.NameAgentsSkills`), `func render.FragmentNamesTarget(f pack.Fragment, target string) bool`.
- Consumes (unchanged): `targets.IsBuiltInName`, `targets.IsFragmentTarget`, `targets.BuiltIns()`.
- Note: `engine.allTargets` is NOT rewired here — that happens in Task 5, together with the `rules` switch case, so the plan's target switch stays total. Reordering `mcp` before `skills` in the table makes the derived `targetOrder` match `esc init`'s long-standing mcp-before-skills presentation (so no pinned init-output test reorders in this task); it also changes `engine.allTargets` iteration order once Task 5 sets `allTargets = targets.Defaults()` (verified: no test pins sync's artifact ordering, and the lockfile sorts by path).

**Steps**
- [ ] Modify `internal/targets/targets_test.go` — rewrite `TestBuiltIns` to the eight-row table (add an `optIn` column) and append the Defaults test. New `TestBuiltIns`:
```go
func TestBuiltIns(t *testing.T) {
	want := map[string]struct {
		file  string
		kind  string
		optIn bool
	}{
		"claude":        {"CLAUDE.md", KindManagedBlock, false},
		"agents":        {"AGENTS.md", KindManagedBlock, false},
		"gemini":        {"GEMINI.md", KindManagedBlock, false},
		"governance":    {"GOVERNANCE.md", KindWholeFile, false},
		"mcp":           {"", KindMCPConfig, false},
		"skills":        {"", KindSkillsDir, false},
		"rules":         {"", KindRulesDir, false},
		"agents-skills": {"", KindSkillsDir, true},
	}
	got := BuiltIns()
	if len(got) != len(want) {
		t.Fatalf("BuiltIns count: got %d want %d", len(got), len(want))
	}
	for _, in := range got {
		w, ok := want[in.Name]
		if !ok {
			t.Errorf("unexpected built-in %q", in.Name)
			continue
		}
		if in.File != w.file || in.Kind != w.kind || in.OptIn != w.optIn || !in.BuiltIn || in.OwnerPack != "" {
			t.Errorf("built-in %q wrong: %+v", in.Name, in)
		}
	}
	if !IsFragmentTarget("claude") || !IsFragmentTarget("governance") || !IsFragmentTarget("rules") {
		t.Error("claude/governance/rules must be fragment targets")
	}
	if IsFragmentTarget("skills") || IsFragmentTarget("mcp") || IsFragmentTarget("agents-skills") || IsFragmentTarget("copilot") {
		t.Error("skills/mcp/agents-skills/custom must not be fragment targets")
	}
}
```
Append:
```go
func TestDefaultsExcludesOptInIncludesRules(t *testing.T) {
	seen := map[string]bool{}
	for _, n := range Defaults() {
		seen[n] = true
	}
	for _, w := range []string{NameClaude, NameAgents, NameGemini, NameGovernance, NameSkills, NameMCP, NameRules} {
		if !seen[w] {
			t.Errorf("Defaults() missing %q", w)
		}
	}
	if seen[NameAgentsSkills] {
		t.Error("Defaults() must exclude the opt-in agents-skills target")
	}
}
```
- [ ] Run `go test ./internal/targets/`. Expected failure: compile error (`undefined: KindRulesDir`, `in.OptIn` undefined, `undefined: Defaults`). Once those exist but before the rows are added, `TestBuiltIns` fails with `BuiltIns count: got 6 want 8`.
- [ ] Implement in `internal/targets/targets.go`. Add to the built-in names const block:
```go
	NameMCP          = "mcp"
	NameRules        = "rules"
	NameAgentsSkills = "agents-skills"
```
Add to the target kinds const block:
```go
	KindMCPConfig    = "mcp-config"
	KindRulesDir     = "rules-dir"
```
Add `OptIn bool` to `Info` (below `BuiltIn`):
```go
	BuiltIn     bool
	// OptIn marks a built-in that an empty targets: list does NOT expand to;
	// a repo must name it explicitly. Zero value false keeps every existing
	// row in the default set.
	OptIn       bool
	OwnerPack   string // defining pack name; empty for built-ins
```
Rewrite the `builtIns` table — reorder so `mcp` precedes `skills` (so the derived target order matches `esc init`'s presentation), then append the two new rows:
```go
var builtIns = []Info{
	{Name: NameClaude, File: "CLAUDE.md", Kind: KindManagedBlock, BuiltIn: true},
	{Name: NameAgents, File: "AGENTS.md", Kind: KindManagedBlock, BuiltIn: true},
	{Name: NameGemini, File: "GEMINI.md", Kind: KindManagedBlock, BuiltIn: true},
	{Name: NameGovernance, File: "GOVERNANCE.md", Kind: KindWholeFile, BuiltIn: true},
	{Name: NameMCP, File: "", Kind: KindMCPConfig, BuiltIn: true},
	{Name: NameSkills, File: "", Kind: KindSkillsDir, BuiltIn: true},
	{Name: NameRules, File: "", Kind: KindRulesDir, BuiltIn: true},
	{Name: NameAgentsSkills, File: "", Kind: KindSkillsDir, BuiltIn: true, OptIn: true},
}
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
- [ ] Single-source the render target-name strings in `internal/render/render.go`. Import `internal/targets` and redefine the const block against it:
```go
import (
	"fmt"
	"strings"

	"github.com/tensorgroup/openescapement/internal/pack"
	"github.com/tensorgroup/openescapement/internal/targets"
)

const (
	TargetClaude       = targets.NameClaude
	TargetAgents       = targets.NameAgents
	TargetGemini       = targets.NameGemini
	TargetGovernance   = targets.NameGovernance
	TargetSkills       = targets.NameSkills
	TargetMCP          = targets.NameMCP
	TargetRules        = targets.NameRules
	TargetAgentsSkills = targets.NameAgentsSkills
)
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
- [ ] Change the `PolicyDiff` default target list in `internal/engine/diff.go`. Add import `targetsPkg "github.com/tensorgroup/openescapement/internal/targets"` (aliased to avoid shadowing the local `targets` variable), then:
```go
	targets := cur.Config.Targets
	if len(targets) == 0 {
		targets = targetsPkg.Defaults()
	}
```
Rule/skills/mcp/agents-skills names all have `render.TargetFile[t] == ""` and are skipped by the existing guard, so this is behavior-neutral until Task 7.
- [ ] Derive `targetOrder` from the table in `internal/cli/initscan.go` (replace the literal slice):
```go
// targetOrder is the fixed presentation order for detected targets: exactly
// the built-in table order (see targets.BuiltIns), so there is one source of
// truth and a newly added built-in can never silently drop out of the
// generated targets: list or the explanation — TestTargetOrderCoversAllTargets
// pins the equality.
var targetOrder = builtInTargetOrder()

func builtInTargetOrder() []string {
	ins := targets.BuiltIns()
	out := make([]string, len(ins))
	for i, in := range ins {
		out[i] = in.Name
	}
	return out
}
```
Add the `internal/targets` import to initscan.go if not already present.
- [ ] Replace `TestTargetOrderCoversAllTargets` in `internal/cli/initscan_test.go` with an equality assertion against the table order:
```go
// TestTargetOrderCoversAllTargets pins targetOrder to exactly the built-in
// table order (targets.BuiltIns): one source of truth, so a target added to
// the table appears here without a second edit, and a stray or duplicate entry
// fails.
func TestTargetOrderCoversAllTargets(t *testing.T) {
	want := builtInTargetOrder()
	if len(targetOrder) != len(want) {
		t.Fatalf("targetOrder length %d, want %d (%v vs %v)", len(targetOrder), len(want), targetOrder, want)
	}
	for i := range want {
		if targetOrder[i] != want[i] {
			t.Errorf("targetOrder[%d] = %q, want %q", i, targetOrder[i], want[i])
		}
	}
}
```
- [ ] Run `go test ./...`. Expect PASS: the default set is unchanged (empty-list still uses the unchanged `allTargets` literal); `targetOrder` now lists mcp before skills (matching the pinned six-target init test) plus the two new names appended, which are never emitted as `Detected` yet; explicit new names are accepted by `isBuiltInTarget` but not listed anywhere.
- [ ] `gofmt -w .` and `go vet ./...`.
- [ ] Commit: `feat(targets): add rules and agents-skills to the built-in table with an opt-in flag` — body explains why agents-skills is opt-in (two copies of every skill only when a repo runs both Claude Code and Kimi Code), why the target-name strings and the init target order now derive from one table, and the mcp/skills table reorder.

---

## Task 2 — Whole-file machinery for every `KindFile`, proven on `GOVERNANCE.md`

Builds the occupied gate, byte-identical adoption, orphan pass, ownership, and pre-read symlink refusal for `KindFile` artifacts — before any rule file exists — so it is reviewable on `GOVERNANCE.md` alone. Every implementation change lands before the first "expect PASS".

**Files**
- Modify `internal/engine/engine.go` (governance planner ~221-225; `prospectiveContent` ~360-374).
- Modify `internal/engine/apply.go` (skip causes ~136-161; add `canonicalRel`/`ownedFilePath`/`ownedRulePath`/`identicalFileContent`; KindFile occupied gate after the KindDir block ~349; orphan-file pass after the block-removal loop ~649; `refuseSymlinks` message ~59).
- Modify `internal/engine/status.go` (`classify` KindFile branch ~410-428; orphan-file pass after the orphan-dir pass ~322).
- Create `internal/engine/wholefile_test.go`.
- Modify `internal/engine/orphandir_test.go` (~505) and `internal/cli/custom_targets_test.go` (~274): symlink-message assertion → new substring.
- Modify `internal/cli/cli.go` (`skipHint` ~525-543; adopted line ~478).
- Modify `internal/cli/skip_hint_test.go` (`TestSkipHintPerCause` ~21).
- Modify `internal/cli/initscan.go` (`detectedSyncClause` ~139-155; `explainDetection` ~173-203) and `internal/cli/initoffer.go` (`offerPlacement` eligibility ~354-355).
- Modify `internal/cli/initscan_test.go` (`TestInitExplainsAllSixDetectedTargets`) and `internal/cli/initoffer_test.go` (`TestInitOfferFullFlowForSixTargets`); create `internal/cli/init_governance_test.go`.
- Modify `README.md` (skip-cause enum ~331), `AGENTS.md` (never-adopt/retirement/lockfile-deletion invariants), `CHANGELOG.md` (Changed entries).

**Interfaces**
- Produces: `engine.SkipUnmanagedFileAtTarget SkipCause = "unmanaged-file-at-target"`, `engine.SkipOrphanFileEdited SkipCause = "orphan-file-edited"`.
- Produces (apply.go, unexported): `func canonicalRel(rel string) bool`, `func ownedFilePath(rel string) bool`, `func ownedRulePath(rel string) bool`, `func identicalFileContent(abs, wantHash string) bool`.
- Consumes: `render.NormalizeEndings`, `esc.HashBytes`, `render.TargetFile`, `containedPath`, `refuseSymlinks`.

**Steps**
- [ ] Write the failing test `internal/engine/wholefile_test.go`:
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
	if err := (&lockfile.Lock{Schema: 1, Artifacts: arts}).Save(root); err != nil {
		t.Fatal(err)
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

// Occupied: no lock entry + unmanaged content on disk → skip; --force must not
// override; status reports Occupied.
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
	res, err = Apply(root, plan, true)
	if err != nil {
		t.Fatalf("apply --force: %v", err)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Cause != SkipUnmanagedFileAtTarget {
		t.Fatalf("--force must not override occupied: %+v", res.Skipped)
	}
	if classify(root, a, mustLock(t, root)).State != Occupied {
		t.Errorf("status should report Occupied")
	}
}

// Byte-identical adoption: CRLF on disk, LF desired → normalized-equal → adopt.
func TestKindFileIdenticalAdopts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "GOVERNANCE.md", "escapement content\r\n")
	a := fileArtifact("GOVERNANCE.md", "escapement content\n")
	res, err := Apply(root, &PlanResult{Artifacts: []Artifact{a}}, false)
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

// Orphan removed when unedited; declined and carried forward when edited.
func TestKindFileOrphanRemovedAndEditedDeclined(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "GOVERNANCE.md", "escapement content\n")
	saveLock(t, root, lockfile.LockArtifact{Path: "GOVERNANCE.md", Kind: KindFile, Hash: esc.HashBytes([]byte("escapement content\n"))})
	if _, err := Apply(root, &PlanResult{}, false); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "GOVERNANCE.md")); !os.IsNotExist(err) {
		t.Errorf("unedited orphan file should be removed")
	}

	root2 := t.TempDir()
	writeFile(t, root2, "GOVERNANCE.md", "hand edited\n")
	saveLock(t, root2, lockfile.LockArtifact{Path: "GOVERNANCE.md", Kind: KindFile, Hash: esc.HashBytes([]byte("escapement content\n"))})
	res, err := Apply(root2, &PlanResult{}, false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Cause != SkipOrphanFileEdited {
		t.Fatalf("edited orphan must be declined: %+v", res.Skipped)
	}
	if _, err := os.Stat(filepath.Join(root2, "GOVERNANCE.md")); err != nil {
		t.Errorf("declined orphan must stay on disk: %v", err)
	}
	if mustLock(t, root2).Artifact("GOVERNANCE.md") == nil {
		t.Error("declined orphan lock entry must be carried forward")
	}
}

// A removed rule file leaves the .claude/rules directory empty; Apply cleans it.
func TestKindFileOrphanRuleRemovesEmptyDir(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".claude/rules/esc-acme-x.md", "body\n")
	saveLock(t, root, lockfile.LockArtifact{Path: ".claude/rules/esc-acme-x.md", Kind: KindFile, Hash: esc.HashBytes([]byte("body\n"))})
	if _, err := Apply(root, &PlanResult{}, false); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "rules")); !os.IsNotExist(err) {
		t.Errorf(".claude/rules should be removed once empty")
	}
}

// Ownership canonicality: every one of these lock paths must be refused before
// any read, leaving the file untouched.
func TestKindFileOrphanRefusesUncanonicalOrUnownedPaths(t *testing.T) {
	cases := []string{
		`.claude/rules/esc-x\..\..\VICTIM.md`, // backslash
		".claude/rules/./esc-x.md",            // ./ alias
		".claude/rules/team.md",               // unprefixed
		".claude/rules/sub/esc-x.md",          // nested
		"governance.md",                       // wrong case
		"docs/GOVERNANCE.md",                  // wrong dir
	}
	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "VICTIM.md", "not escapement's\n")
			saveLock(t, root, lockfile.LockArtifact{Path: p, Kind: KindFile, Hash: "sha256:whatever"})
			if _, err := Apply(root, &PlanResult{}, false); err == nil {
				t.Fatalf("apply must refuse to act on lock path %q", p)
			}
			if _, err := os.Stat(filepath.Join(root, "VICTIM.md")); err != nil {
				t.Errorf("nothing must be removed for %q: %v", p, err)
			}
		})
	}
}

func TestOwnedFilePathExactGovernanceOnly(t *testing.T) {
	if !ownedFilePath("GOVERNANCE.md") {
		t.Error("GOVERNANCE.md must be owned")
	}
	for _, p := range []string{"governance.md", "docs/GOVERNANCE.md", "./GOVERNANCE.md"} {
		if ownedFilePath(p) {
			t.Errorf("%q must not be owned", p)
		}
	}
	if !ownedRulePath(".claude/rules/esc-x.md") {
		t.Error("prefixed single-element rule path must be owned")
	}
	for _, p := range []string{".claude/rules/team.md", ".claude/rules/sub/esc-x.md", ".claude/rules/./esc-x.md", `.claude/rules/esc-x\y.md`} {
		if ownedRulePath(p) {
			t.Errorf("%q must not be owned", p)
		}
	}
}

// identicalFileContent must refuse a symlink itself, so a symlink whose target
// hashes equal is never treated as an adoptable match.
func TestIdenticalFileContentRefusesSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "real.md")
	if err := os.WriteFile(target, []byte("escapement content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.md")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if identicalFileContent(link, esc.HashBytes([]byte("escapement content\n"))) {
		t.Error("a symlink must never be reported as identical content, even when its target matches")
	}
}

// prospectiveContent refuses a symlinked whole-file target at plan time, which
// is what makes plan/status/diff/sync exit 4 on it.
func TestKindFileSymlinkRefusedInPlan(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink("/etc/passwd", filepath.Join(root, "GOVERNANCE.md")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := prospectiveContent(root, fileArtifact("GOVERNANCE.md", "x\n")); err == nil {
		t.Fatal("prospectiveContent must refuse a symlinked KindFile target")
	}
}

func TestKindFileDesiredHashEqualsClassifyOnCRLF(t *testing.T) {
	root := t.TempDir()
	a := fileArtifact("GOVERNANCE.md", "line one\nline two\n")
	writeFile(t, root, "GOVERNANCE.md", "line one\r\nline two\r\n")
	saveLock(t, root, lockfile.LockArtifact{Path: "GOVERNANCE.md", Kind: KindFile, Hash: a.Hash})
	if classify(root, a, mustLock(t, root)).State != InSync {
		t.Error("CRLF checkout of a whole file must classify InSync")
	}
}
```
- [ ] Run `go test ./internal/engine/ -run 'KindFile|OwnedFilePath|IdenticalFileContent'`. Expected failure: compile error (`undefined: SkipUnmanagedFileAtTarget`, `ownedFilePath`, `ownedRulePath`, `identicalFileContent`).
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
- [ ] Implement the canonical ownership + adoption helpers in `internal/engine/apply.go` (near `ownedSkillPath`; `strings`, `path`, `render`, `os`, `esc` are all imported):
```go
// canonicalRel reports whether rel is a plain, canonical, slash-separated
// repo-relative path safe to compare against desired artifact paths and to
// feed to the filesystem: no backslash, no leading slash, and path.Clean is a
// no-op (no ".", "..", "//", or trailing slash). A lockfile is
// attacker-controlled input, so a non-canonical entry is refused before it is
// ever compared or read, not silently normalized into an owned path.
func canonicalRel(rel string) bool {
	if rel == "" || strings.ContainsRune(rel, '\\') || strings.HasPrefix(rel, "/") {
		return false
	}
	return path.Clean(rel) == rel
}

// ownedFilePath reports whether a lockfile KindFile entry is one escapement
// could have written: the built-in GOVERNANCE.md path exactly, or a rule file
// under .claude/rules with the reserved esc- prefix. Non-canonical paths are
// refused outright. Decided on the repo-relative path, never the absolute one,
// and confines what a hostile lockfile can delete — the same ruling AGENTS.md
// records for skill directories. The prefix is a reserved namespace, not proof
// escapement created the file.
func ownedFilePath(rel string) bool {
	if !canonicalRel(rel) {
		return false
	}
	return rel == render.TargetFile[render.TargetGovernance] || ownedRulePath(rel)
}

// ownedRulePath reports whether rel is exactly one canonical element below
// .claude/rules carrying the reserved esc- prefix.
func ownedRulePath(rel string) bool {
	if !canonicalRel(rel) {
		return false
	}
	return path.Dir(rel) == ".claude/rules" && strings.HasPrefix(path.Base(rel), "esc-")
}

// identicalFileContent reports whether the file at abs, endings-normalized,
// hashes to wantHash — the ONLY condition under which the KindFile occupied
// gate adopts rather than skips (nothing to destroy). It Lstats first and
// refuses a symlink itself: classify's occupied branch does not run
// refuseSymlinks, so a symlink whose target hashes equal must never read as an
// adoptable match.
func identicalFileContent(abs, wantHash string) bool {
	fi, err := os.Lstat(abs)
	if err != nil || fi.Mode()&os.ModeSymlink != 0 {
		return false
	}
	got, err := os.ReadFile(abs)
	if err != nil {
		return false
	}
	return esc.HashBytes(render.NormalizeEndings(got)) == wantHash
}
```
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
- [ ] Implement the orphan-file pass in `Apply`, after the stale-managed-block removal loop (before `lock := &lockfile.Lock{...}`):
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
			// ownedFilePath already rejects any non-canonical or unowned path
			// before it is compared or read.
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
			// Best-effort: drop a now-empty .claude/rules directory, as the
			// skill-dir pass drops an emptied skill directory. os.Remove only
			// succeeds on an empty dir, and GOVERNANCE.md's parent is the repo
			// root (never a rule path), so this is gated on ownedRulePath.
			if ownedRulePath(prev.Path) {
				_ = os.Remove(filepath.Join(root, ".claude", "rules"))
			}
		}
	}
```
- [ ] Add the status-side occupied classification. In `internal/engine/status.go`, replace the `case KindFile:` branch:
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
- [ ] Add the status-side orphan-file pass in `internal/engine/status.go`, after the orphan skill-directory pass (before the `for _, v := range plan.Violations` loop). Unlike the block/dir passes, a containment or symlink failure here PROPAGATES (Status returns the error → exit 4), so a retired rule file replaced by a symlink cannot silently vanish from status while sync would refuse it:
```go
	// Orphan whole files: a KindFile the lockfile records that the effective set
	// no longer contains. A containment/symlink failure propagates (exit 4) to
	// match sync's orphan-file pass; only os.IsNotExist reads as already gone.
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
				return nil, cerr
			}
			if serr := refuseSymlinks(root, la.Path); serr != nil {
				return nil, serr
			}
			content, err := os.ReadFile(abs)
			if os.IsNotExist(err) {
				continue // already gone
			}
			if err != nil {
				return nil, err
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
- [ ] Fix the governance planner desired hash in `internal/engine/engine.go` (`case render.TargetGovernance:`):
```go
		case render.TargetGovernance:
			content := render.Governance(res.PackObjs)
			res.Artifacts = append(res.Artifacts, Artifact{
				Path: render.TargetFile[t], Kind: KindFile,
				Hash: esc.HashBytes(render.NormalizeEndings([]byte(content))), Body: content,
			})
```
- [ ] Add pre-read symlink refusal to `prospectiveContent` in `internal/engine/engine.go` (Plan runs this in its validation loop; Status/Diff/sync call Plan first, so all four exit 4 on a symlinked target):
```go
func prospectiveContent(root string, a Artifact) ([]byte, error) {
	if err := refuseSymlinks(root, a.Path); err != nil {
		return nil, err
	}
	existing, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(a.Path)))
	...
```
- [ ] Update the `refuseSymlinks` message in `internal/engine/apply.go` (it now guards reads as well as writes), keeping the `rel:` prefix + symlink-location structure:
```go
			return fmt.Errorf("%s: refusing to read or write through a symlink at %s; replace the symlink with a regular file or directory, or drop the target", rel, cur)
```
Update the two engine/CLI assertions that pin the old substring to the new one (`refusing to read or write through a symlink`): `internal/engine/orphandir_test.go` ~505 and `internal/cli/custom_targets_test.go` ~274. (Leave `internal/cli/skillvendor.go` and `internal/cli/packupdate_symlink_test.go` alone — those use the `esc pack` author-command's own copy of the message, out of scope here.)
- [ ] Run `go test ./internal/engine/`. Expect PASS (all engine-side machinery now exists).
- [ ] Add `skipHint` cases in `internal/cli/cli.go`:
```go
	case engine.SkipUnmanagedFileAtTarget:
		return "escapement never owned that file · move it aside, then `esc sync` (`--force` does not override this)"
	case engine.SkipOrphanFileEdited:
		return "the file is still there · revert it to the synced content, or `esc sync --force` to remove it"
```
- [ ] Extend `TestSkipHintPerCause` in `internal/cli/skip_hint_test.go` with the two new causes:
```go
		{engine.SkipUnmanagedFileAtTarget, "escapement never owned that file · move it aside, then `esc sync` (`--force` does not override this)"},
		{engine.SkipOrphanFileEdited, "the file is still there · revert it to the synced content, or `esc sync --force` to remove it"},
```
- [ ] Generalize the adopted-artifact line in `internal/cli/cli.go` (~478) so it reads for files too:
```go
	for _, s := range res.Adopted {
		fmt.Fprintf(stderr, "  adopted %s: existing content matches pack output exactly\n", s)
	}
```
- [ ] Update the skip-cause enum in `README.md` (~331): add `unmanaged-file-at-target` and `orphan-file-edited` to the `cause` value list (alongside the existing `hand-edited`, `orphan-dir-unmanaged`, `orphan-dir-edited`, `orphan-block-edited`).
- [ ] Update `esc init`'s GOVERNANCE.md story. In `internal/cli/initscan.go` `detectedSyncClause`, add a governance case before the block default:
```go
	case render.TargetGovernance:
		return "report GOVERNANCE.md as occupied and not overwrite it — move or delete it so escapement can own it"
```
Rewrite `explainDetection` in `internal/cli/initscan.go` so the amendment-preservation claim never covers GOVERNANCE.md (a whole file escapement owns, not an amendment-preserving block), and governance is excluded from the placement hint:
```go
func explainDetection(w io.Writer, detected []Detected) {
	ordered := orderDetected(detected)
	labels := make([]string, len(ordered))
	clauses := make([]string, len(ordered))
	needsPlacementHint := false
	governanceDetected := false
	for i, d := range ordered {
		labels[i] = detectedItemLabel(d)
		clauses[i] = detectedSyncClause(d)
		if d.Target == render.TargetGovernance {
			governanceDetected = true
			continue
		}
		if isFileTarget(d.Target) && !d.HasBlock && !d.HasPlaceholder {
			needsPlacementHint = true
		}
	}
	fmt.Fprintf(w, "Found %s.\n\n", joinList(labels))
	const preserved = "Your current content is preserved byte for byte and reported as a local amendment."
	const governanceNote = "A detected GOVERNANCE.md is a whole file escapement owns: it is reported occupied and left untouched until you move or delete it."
	if len(clauses) == 1 {
		if governanceDetected {
			fmt.Fprintf(w, "`esc sync` will %s.\n", clauses[0])
		} else {
			fmt.Fprintf(w, "`esc sync` will %s. %s\n", clauses[0], preserved)
		}
	} else {
		fmt.Fprintln(w, "`esc sync` will:")
		for _, c := range clauses {
			fmt.Fprintf(w, "  - %s\n", c)
		}
		fmt.Fprintf(w, "\n%s\n", preserved)
		if governanceDetected {
			fmt.Fprintf(w, "%s\n", governanceNote)
		}
	}
	if needsPlacementHint {
		fmt.Fprintf(w, "\nTo place the block somewhere else, put %s where you want it before syncing.\n", render.Placeholder)
	}
}
```
Exclude governance from the placement offer in `internal/cli/initoffer.go` `offerPlacement`:
```go
		if !isFileTarget(d.Target) || d.Target == render.TargetGovernance || d.HasBlock || d.HasPlaceholder {
			continue
		}
```
- [ ] Update `internal/cli/initscan_test.go` `TestInitExplainsAllSixDetectedTargets` to the new expected output (governance now an occupied clause; the governanceNote line after `preserved`; mcp still before skills because the table reorder matches init's presentation):
```go
	want := fmt.Sprintf(
		"Initialized %s\n\n"+
			"Found CLAUDE.md (3 lines), AGENTS.md (3 lines), GEMINI.md (3 lines), GOVERNANCE.md (3 lines), .mcp.json and .claude/skills.\n\n"+
			"`esc sync` will:\n"+
			"  - insert a managed block at the top of CLAUDE.md\n"+
			"  - insert a managed block at the top of AGENTS.md\n"+
			"  - insert a managed block at the top of GEMINI.md\n"+
			"  - report GOVERNANCE.md as occupied and not overwrite it — move or delete it so escapement can own it\n"+
			"  - add pack MCP servers alongside your existing ones\n"+
			"  - add pack skills alongside yours in .claude/skills\n\n"+
			"Your current content is preserved byte for byte and reported as a local amendment.\n"+
			"A detected GOVERNANCE.md is a whole file escapement owns: it is reported occupied and left untouched until you move or delete it.\n\n"+
			"To place the block somewhere else, put %s where you want it before syncing.\n",
		config.Path(root), render.Placeholder)
```
- [ ] Update `internal/cli/initoffer_test.go` `TestInitOfferFullFlowForSixTargets`: GOVERNANCE.md is no longer eligible for the placement offer, so the offer now covers 3 files. Change the doc comment ("four of them eligible" → "three of them eligible"), the `wantOffer` string, and the post-write content-check loop:
```go
	wantOffer := "\nOptional: write " + render.Placeholder + " into all 3 files now, to choose where the block lands.\n" +
		"  CLAUDE.md\n  AGENTS.md\n  GEMINI.md\n" +
		"Place the marker now? [y/N] " +
		"Where should it go in all 3 files?\n" +
		"  [a] above the title, the first thing in the file (below any frontmatter)\n" +
		"  [e] end of the file\n> " +
		"  CLAUDE.md: wrote " + render.Placeholder + "\n" +
		"  AGENTS.md: wrote " + render.Placeholder + "\n" +
		"  GEMINI.md: wrote " + render.Placeholder + "\n"
	if !strings.HasSuffix(out, wantOffer) {
		t.Errorf("offer output =\n%q\nwant suffix\n%q", out, wantOffer)
	}
	for _, name := range []string{"CLAUDE.md", "AGENTS.md", "GEMINI.md"} {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != render.Placeholder+"\n# P\n\nrules\n" {
			t.Errorf("%s: one answer must apply to every eligible file, got:\n%q", name, content)
		}
	}
```
(GOVERNANCE.md stays in the test's `writeFiles` map — it is detected but not offered — and keeps its original `# P\n\nrules\n` content, which the loop above no longer checks.)
- [ ] Create `internal/cli/init_governance_test.go`:
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
	if strings.Contains(out, "preserved byte for byte") {
		t.Errorf("a governance-only detection must not claim amendment preservation:\n%s", out)
	}
}
```
- [ ] Run `go test ./internal/cli/`. Expect PASS.
- [ ] Run `go test ./...`. Expect PASS (the shipped `examples/governed-service` lock has a GOVERNANCE.md entry, so it is not occupied; the normalized hash is a no-op on its LF checkout).
- [ ] Update `AGENTS.md`:
  - Never-adopt invariant: generalize so a `KindFile` artifact with no prior lock entry whose target already exists is likewise skipped (`SkipUnmanagedFileAtTarget`, exit 0, status `occupied`), adopted only on a byte-identical (endings-normalized) match, and `--force` does not override.
  - Retirement invariant: change "Retiring a skill directory" to "Retiring an artifact" where the sentence states the general rule (a declined removal carries the lock entry forward and keeps reporting); keep the skill-directory manifest specifics.
  - Lockfile-deletion invariant: add that after deleting the lockfile a whole-file artifact (`GOVERNANCE.md`, a rule file) becomes `occupied` rather than overwritten — its content is no longer silently replaced, but managed blocks still are, so the "do not delete the lockfile" rule stands.
- [ ] Add CHANGELOG `### Changed` entries under `## [Unreleased]`:
```
- The whole-file occupied gate and orphan pass now cover every `KindFile`
  artifact, `GOVERNANCE.md` included: a repo that already has its own
  `GOVERNANCE.md` is reported `occupied` and is no longer overwritten silently
  on first sync (move or delete it, or let escapement adopt a byte-identical
  copy); a `GOVERNANCE.md` no longer in the effective set is reported `orphan`
  and removed, or left in place if hand-edited.
- Containment now runs before reads as well as writes: `esc status`, `esc diff`,
  `esc render`, and `esc sync` refuse a symlinked instruction-file target (exit
  4) instead of reading through it.
```
- [ ] `gofmt -w .`, `go vet ./...`.
- [ ] Commit: `feat(engine): whole-file occupied gate and orphan pass for every KindFile` — body explains that this is the machinery rules needs, built and proven on GOVERNANCE.md first, and names the first-sync behavior change and the read-side containment.

---

## Task 3 — Pack loader: LF normalize at load, `Fragment.Paths` (strict), in-pack collision, compose refusal

**Files**
- Modify `internal/pack/fragment.go` (`Fragment` struct ~17-21; `loadFragment` ~29-54; add `parsePaths` (yaml.Node), `validateRulePaths`, `ruleStem`).
- Modify `internal/pack/pack.go` (`Load` ~140-147: add an in-pack rule-filename collision check; add `namesRulesTarget`).
- Modify `internal/pack/compose.go` (`ComposeFragments` ~18-54: normalize endings per part, refuse `paths:` including on the single-part early return).
- Create `internal/pack/fragment_rules_test.go`; extend `internal/pack/compose_test.go`.

**Interfaces**
- Produces: `pack.Fragment.Paths []string`.
- Consumes: `targets.NameRules`, `targets.IsFragmentTarget`; sentinels `esc.ErrManifest`, `esc.ErrConstraint`.
- Note: normalize with `strings.ReplaceAll(s, "\r\n", "\n")` (no `render` import — cycle). `paths:` is parsed via `yaml.Node`, not `[]string` decode, so absent / null / `~` / coerced scalars / non-string elements are all distinguishable.

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
	if f.Body != "# API\n\nbody\n" {
		t.Errorf("body = %q", f.Body)
	}
}

func mustFail(t *testing.T, name, content string) {
	t.Helper()
	if _, err := loadOneFragment(t, name, content); !errors.Is(err, esc.ErrManifest) {
		t.Fatalf("want ErrManifest, got %v", err)
	}
}

func TestPathsWithoutRulesTargetIsError(t *testing.T) {
	mustFail(t, "api.md", "---\ntargets: [claude]\npaths:\n  - \"src/**\"\n---\nbody\n")
}
func TestEmptyPathsListIsError(t *testing.T)  { mustFail(t, "api.md", "---\ntargets: [rules]\npaths: []\n---\nbody\n") }
func TestNullPathsIsError(t *testing.T)       { mustFail(t, "api.md", "---\ntargets: [rules]\npaths:\n---\nbody\n") }
func TestTildeNullPathsIsError(t *testing.T)  { mustFail(t, "api.md", "---\ntargets: [rules]\npaths: ~\n---\nbody\n") }
func TestScalarPathsIsError(t *testing.T)     { mustFail(t, "api.md", "---\ntargets: [rules]\npaths: foo\n---\nbody\n") }
func TestNonStringPathsElemIsError(t *testing.T) {
	mustFail(t, "api.md", "---\ntargets: [rules]\npaths:\n  - 123\n---\nbody\n")
}
func TestEmptyPathsEntryIsError(t *testing.T) {
	mustFail(t, "api.md", "---\ntargets: [rules]\npaths:\n  - \"\"\n---\nbody\n")
}
func TestBadRuleStemIsError(t *testing.T) { mustFail(t, "API_Handlers.md", "---\ntargets: [rules]\n---\nbody\n") }

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
	if len(f.Targets) != 1 || f.Targets[0] != "rules" || f.Body != "body line\n" {
		t.Errorf("targets/body lost on CRLF fragment: %v / %q", f.Targets, f.Body)
	}
}
```
- [ ] Run `go test ./internal/pack/ -run 'Paths|Stem|CRLF|NoFrontmatter|RulesAndClaude'`. Expected failure: `f.Paths undefined` (compile) then assertion failures.
- [ ] Implement in `internal/pack/fragment.go`. Add `Paths []string` to `Fragment`, add imports `"regexp"` (keep `filepath`, `strings`, `yaml`), and add the stem pattern:
```go
type Fragment struct {
	Path    string
	Targets []string
	Paths   []string // glob scopes for a rules fragment; nil for every other fragment
	Body    string
}

// ruleStem constrains a rules fragment's filename stem, which becomes part of
// an on-disk filename under .claude/rules/.
var ruleStem = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
```
Rewrite `loadFragment` and add `parsePaths` / `validateRulePaths`:
```go
func loadFragment(packDir, packName, rel string, customNames map[string]bool) (*Fragment, error) {
	raw, err := os.ReadFile(filepath.Join(packDir, rel))
	if err != nil {
		return nil, fmt.Errorf("%w: rule %s: %v", esc.ErrManifest, rel, err)
	}
	frag := &Fragment{Path: rel}
	// Normalize CRLF to LF at load: the splitter recognizes only LF fences, so
	// a CRLF fragment would otherwise lose its frontmatter (and its scope)
	// silently. Same one-conversion rule as render.NormalizeEndings, inlined
	// because pack must not import render (cycle).
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if fm, body, ok := splitFrontmatter(content); ok {
		var meta struct {
			Targets []string `yaml:"targets"`
		}
		if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
			return nil, fmt.Errorf("%w: rule %s frontmatter: %v", esc.ErrManifest, rel, err)
		}
		for _, tgt := range meta.Targets {
			if !targets.IsFragmentTarget(tgt) && !customNames[tgt] {
				return nil, fmt.Errorf("%w: pack %q: rule %s: unknown or foreign target %q (not a built-in target or a custom target defined by this pack)", esc.ErrConstraint, packName, rel, tgt)
			}
		}
		paths, hasPaths, perr := parsePaths([]byte(fm), packName, rel)
		if perr != nil {
			return nil, perr
		}
		if err := validateRulePaths(packName, rel, meta.Targets, hasPaths); err != nil {
			return nil, err
		}
		frag.Targets = meta.Targets
		frag.Paths = paths
		frag.Body = body
	} else {
		frag.Body = content
	}
	return frag, nil
}

// parsePaths extracts the paths: field strictly from frontmatter via a
// yaml.Node, so absent (nil, present=false) is distinguishable from null, an
// empty sequence, a scalar, and non-string elements — all of which are
// errors. present reports whether the key appeared at all.
func parsePaths(fm []byte, packName, rel string) (paths []string, present bool, err error) {
	var doc yaml.Node
	if uerr := yaml.Unmarshal(fm, &doc); uerr != nil {
		return nil, false, fmt.Errorf("%w: rule %s frontmatter: %v", esc.ErrManifest, rel, uerr)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, false, nil
	}
	m := doc.Content[0]
	fail := func(msg string) error {
		return fmt.Errorf("%w: pack %q: rule %s: %s", esc.ErrManifest, packName, rel, msg)
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value != "paths" {
			continue
		}
		v := m.Content[i+1]
		if v.Tag == "!!null" {
			return nil, true, fail("paths: is present but null; give a non-empty list of glob strings or remove the key")
		}
		if v.Kind != yaml.SequenceNode {
			return nil, true, fail("paths: must be a list of glob strings")
		}
		if len(v.Content) == 0 {
			return nil, true, fail("paths: when present must be a non-empty list")
		}
		for _, e := range v.Content {
			if e.Kind != yaml.ScalarNode || e.Tag != "!!str" {
				return nil, true, fail("paths: every entry must be a string")
			}
			if e.Value == "" || strings.ContainsAny(e.Value, "\r\n") {
				return nil, true, fail("paths: every entry must be a non-empty string with no newlines")
			}
			paths = append(paths, e.Value)
		}
		return paths, true, nil
	}
	return nil, false, nil
}

// validateRulePaths enforces the targets-relationship and stem rules; paths
// content itself was already validated by parsePaths.
func validateRulePaths(packName, rel string, tgts []string, hasPaths bool) error {
	namesRules := false
	for _, t := range tgts {
		if t == targets.NameRules {
			namesRules = true
		}
	}
	if !namesRules {
		if hasPaths {
			return fmt.Errorf("%w: pack %q: rule %s: paths: is only allowed on a fragment whose targets include %q", esc.ErrManifest, packName, rel, targets.NameRules)
		}
		return nil
	}
	stem := strings.TrimSuffix(filepath.Base(rel), ".md")
	if !ruleStem.MatchString(stem) {
		return fmt.Errorf("%w: pack %q: rule %s: filename stem %q must match %s", esc.ErrManifest, packName, rel, stem, ruleStem)
	}
	return nil
}
```
- [ ] Run `go test ./internal/pack/ -run 'Paths|Stem|CRLF|NoFrontmatter|RulesAndClaude'`. Expect PASS.
- [ ] Write the in-pack collision test (append to `fragment_rules_test.go`), driven through `Load` so `esc pack` author commands see it:
```go
func TestInPackRuleStemCollisionIsError(t *testing.T) {
	dir := t.TempDir()
	man := "schema: 1\nname: acme\nversion: 1.0.0\nrules:\n  - rules/api.md\n  - sub/api.md\n"
	if err := os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(man), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"rules/api.md", "sub/api.md"} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("---\ntargets: [rules]\n---\nx\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Load(dir); !errors.Is(err, esc.ErrManifest) {
		t.Fatalf("two rules fragments with the same stem must be ErrManifest, got %v", err)
	}
}
```
- [ ] Run `go test ./internal/pack/ -run TestInPackRuleStemCollisionIsError`. Expected failure: Load succeeds.
- [ ] Implement the in-pack collision check in `internal/pack/pack.go` `Load`, after the fragment-loading loop (mirrors the `seenSkillDir` precedent in `validate`):
```go
	seenRule := map[string]string{} // lowercased esc-<pack>-<stem>.md -> rel
	for _, f := range p.Fragments {
		if !namesRulesTarget(f) {
			continue
		}
		key := strings.ToLower("esc-" + m.Name + "-" + strings.TrimSuffix(filepath.Base(f.Path), ".md") + ".md")
		if prev, ok := seenRule[key]; ok {
			return nil, fmt.Errorf("%w: rules %q and %q both resolve to rule file %q", esc.ErrManifest, prev, f.Path, key)
		}
		seenRule[key] = f.Path
	}
	return p, nil
```
Add the helper (pack already imports `targets`):
```go
func namesRulesTarget(f Fragment) bool {
	for _, t := range f.Targets {
		if t == targets.NameRules {
			return true
		}
	}
	return false
}
```
- [ ] Run `go test ./internal/pack/ -run TestInPackRuleStemCollisionIsError`. Expect PASS.
- [ ] Write the compose refusal tests in `internal/pack/compose_test.go`. Add imports `errors` and `github.com/tensorgroup/openescapement/internal/esc`:
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

func TestComposeFragmentsRejectsSinglePathScopedPart(t *testing.T) {
	// The portal's resolveAdoptSelection can pass a single part, which used to
	// return byte-identical before any parse.
	one := [][]byte{[]byte("---\ntargets: [rules]\npaths:\n  - \"src/**\"\n---\nbody\n")}
	if _, err := ComposeFragments(one); !errors.Is(err, esc.ErrManifest) {
		t.Fatalf("single path-scoped part must be rejected, got %v", err)
	}
}
```
- [ ] Run `go test ./internal/pack/ -run TestComposeFragments`. Expected failure: the single-part case returns nil error.
- [ ] Rewrite `ComposeFragments` in `internal/pack/compose.go` to normalize endings per part and refuse `paths:` on every part (single-part path included):
```go
func ComposeFragments(parts [][]byte) ([]byte, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("%w: compose: no fragments selected", esc.ErrManifest)
	}
	// Normalize endings up front so the LF-only splitter never lets a CRLF
	// scoped part slip through unparsed.
	norm := make([][]byte, len(parts))
	for i, p := range parts {
		norm[i] = []byte(strings.ReplaceAll(string(p), "\r\n", "\n"))
	}
	for i, p := range norm {
		has, err := partHasPaths(p, i)
		if err != nil {
			return nil, err
		}
		if has {
			return nil, fmt.Errorf("%w: compose part %d: a path-scoped rule fragment cannot be composed", esc.ErrManifest, i)
		}
	}
	if len(norm) == 1 {
		return norm[0], nil
	}
	var union []string
	seen := map[string]bool{}
	bodies := make([]string, 0, len(norm))
	for _, p := range norm {
		content := string(p)
		if fm, body, ok := splitFrontmatter(content); ok {
			var meta struct {
				Targets []string `yaml:"targets"`
			}
			if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
				return nil, fmt.Errorf("%w: compose frontmatter: %v", esc.ErrManifest, err)
			}
			for _, t := range meta.Targets {
				if !seen[t] {
					seen[t] = true
					union = append(union, t)
				}
			}
			content = body
		}
		bodies = append(bodies, strings.TrimSpace(content))
	}
	var b strings.Builder
	if len(union) > 0 {
		b.WriteString("---\ntargets: [" + strings.Join(union, ", ") + "]\n---\n\n")
	}
	b.WriteString(strings.Join(bodies, "\n\n"))
	b.WriteString("\n")
	return []byte(b.String()), nil
}

// partHasPaths reports whether a compose part carries a non-empty paths: list.
func partHasPaths(part []byte, i int) (bool, error) {
	fm, _, ok := splitFrontmatter(string(part))
	if !ok {
		return false, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(fm), &doc); err != nil {
		return false, fmt.Errorf("%w: compose part %d frontmatter: %v", esc.ErrManifest, i, err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return false, nil
	}
	m := doc.Content[0]
	for j := 0; j+1 < len(m.Content); j += 2 {
		if m.Content[j].Value == "paths" {
			v := m.Content[j+1]
			return v.Kind == yaml.SequenceNode && len(v.Content) > 0, nil
		}
	}
	return false, nil
}
```
- [ ] Run `go test ./internal/pack/`. Expect PASS. (The portal already routes `ComposeFragments` errors through `serverError` — verified in `internal/portal/web/models.go` `handleModelAdopt`/`handleModelAdoptSave` — so no portal code change is needed.)
- [ ] `gofmt -w .`, `go vet ./...`, `go test ./...`.
- [ ] Commit: `feat(pack): parse and validate rules-fragment paths; normalize line endings at load` — body explains the silent-CRLF-scope-loss bug, the strict yaml.Node paths parse, the in-pack collision check, and why a scoped fragment cannot be composed.

---

## Task 4 — Renderer: `RenderRule`, `RuleFilePath`, GOVERNANCE.md lists rules fragments

**Files**
- Modify `internal/render/render.go` (add `RuleFileStem`, `RuleFilePath`, `RenderRule`; add imports `path`, `gopkg.in/yaml.v3`).
- Modify `internal/render/governance.go` (append a rules section per pack).
- Create `internal/render/rules_test.go`; create/extend `internal/render/governance_test.go`.

**Interfaces**
- Produces: `render.RuleFileStem`, `render.RuleFilePath`, `render.RenderRule`.
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
	want := "---\n" + string(fm) + "---\n<!-- Managed by escapement (pack acme). Do not edit. -->\n\nbody\n"
	if got != want {
		t.Errorf("paths render:\n got %q\nwant %q", got, want)
	}
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
	if strings.Contains(got, "\r") || !strings.HasSuffix(got, "a\nb\n") {
		t.Errorf("CRLF body must normalize: %q", got)
	}
}

func TestRenderRuleNoTrailingNewlineBodyGetsOne(t *testing.T) {
	f := pack.Fragment{Path: "rules/x.md", Targets: []string{"rules"}, Body: "no newline"}
	if got := render.RenderRule(f, "acme"); !strings.HasSuffix(got, "no newline\n") {
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
- [ ] Implement in `internal/render/render.go`. Update imports to add `"path"` and `"gopkg.in/yaml.v3"` (keeping `fmt`, `strings`, `pack`, and the `targets` import from Task 1). Append:
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
// optional paths frontmatter (only when path-scoped, so frontmatter sits at
// byte 0 as Claude Code's rules loader requires), a one-line ownership notice
// (deliberately without the pack version, so a version bump does not rewrite
// every rule file), a blank line, then the endings-normalized,
// trailing-whitespace-trimmed body plus exactly one newline. Deterministic.
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
		t.Errorf("governance must include rule bodies:\n%s", g)
	}
}

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
- [ ] Run `go test ./internal/render/`. Expect PASS.
- [ ] `gofmt -w .`, `go vet ./...`, `go test ./...`.
- [ ] Commit: `feat(render): render rules fragments to whole files and list them in GOVERNANCE.md`.

---

## Task 5 — Engine planner for `rules`

**Files**
- Modify `internal/engine/engine.go` (`allTargets` ~66-69 → `targets.Defaults()`; add `PlanResult.Notices`; add `case render.TargetRules:`; add the filter-excludes-rules notice after the empty-list expansion).
- Modify `internal/cli/cli.go` (`syncOnce` prints `plan.Notices` to stderr).
- Create `internal/engine/rules_planner_test.go`; create `internal/cli/rules_notice_test.go`.

**Interfaces**
- Produces: `engine.PlanResult.Notices []string`.
- Consumes: `render.FragmentNamesTarget`, `render.RuleFilePath`, `render.RenderRule`, `esc.HashBytes`, `render.NormalizeEndings`, `esc.ErrConfig`.
- Note: in-pack rule collisions are already caught in `pack.Load` (Task 3). This planner's collision map is CROSS-pack only, keyed by pack INDEX (two sources may share a manifest name), case-folded, `ErrConfig` naming both packs.

**Steps**
- [ ] Write the failing test `internal/engine/rules_planner_test.go`:
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

func writeRulePack(t *testing.T, name, version string, rules map[string]string) string {
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

func planLocal(t *testing.T, cfg *config.Config) (*PlanResult, error) {
	t.Helper()
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	return planFromConfig(context.Background(), t.TempDir(), cfg)
}

func ruleCfg(targetsList []string, packs ...string) *config.Config {
	c := &config.Config{Targets: targetsList}
	for _, pd := range packs {
		c.Packs = append(c.Packs, config.PackRef{Source: pd, Ref: "", Trust: "unsigned"})
	}
	return c
}

func TestRulesPlannerEmitsRuleFile(t *testing.T) {
	pd := writeRulePack(t, "acme", "1.0.0", map[string]string{
		"rules/api.md": "---\ntargets: [rules]\npaths:\n  - \"src/**\"\n---\nbody\n",
	})
	res, err := planLocal(t, ruleCfg(nil, pd)) // empty targets = defaults, includes rules
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
		t.Fatalf("rule file artifact missing")
	}
	if found.Kind != KindFile || !strings.Contains(found.Body, "src/**") {
		t.Errorf("rule artifact wrong: %+v", found)
	}
}

func TestRulesCrossPackCollisionIsConfigError(t *testing.T) {
	// Two DIFFERENT packs, same name, same stem → same rule path → ErrConfig.
	a := writeRulePack(t, "shared", "1.0.0", map[string]string{"rules/api.md": "---\ntargets: [rules]\n---\na\n"})
	b := writeRulePack(t, "shared", "2.0.0", map[string]string{"rules/api.md": "---\ntargets: [rules]\n---\nb\n"})
	if _, err := planLocal(t, ruleCfg(nil, a, b)); !errors.Is(err, esc.ErrConfig) {
		t.Fatalf("cross-pack rule collision must be ErrConfig, got %v", err)
	}
}

func TestRulesCrossPackCollisionCaseFolded(t *testing.T) {
	a := writeRulePack(t, "Acme", "1.0.0", map[string]string{"rules/api.md": "---\ntargets: [rules]\n---\na\n"})
	b := writeRulePack(t, "acme", "1.0.0", map[string]string{"rules/api.md": "---\ntargets: [rules]\n---\nb\n"})
	if _, err := planLocal(t, ruleCfg(nil, a, b)); !errors.Is(err, esc.ErrConfig) {
		t.Fatalf("case-folded cross-pack collision must be ErrConfig, got %v", err)
	}
}

func TestRulesFilterExcludedNotice(t *testing.T) {
	pd := writeRulePack(t, "acme", "1.0.0", map[string]string{"rules/api.md": "---\ntargets: [rules]\n---\na\n"})
	res, err := planLocal(t, ruleCfg([]string{"claude"}, pd))
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	for i := range res.Artifacts {
		if strings.HasPrefix(res.Artifacts[i].Path, ".claude/rules/") {
			t.Fatal("no rule file when rules is filtered out")
		}
	}
	if len(res.Notices) == 0 || !strings.Contains(res.Notices[0], "targets rules") {
		t.Errorf("want a filter-excludes-rules notice, got %v", res.Notices)
	}
}
```
- [ ] Run `go test ./internal/engine/ -run Rules`. Expected failure: `res.Notices undefined` (compile), and once compiling, the default-set case errors `unknown target rules` until the switch case exists.
- [ ] Implement in `internal/engine/engine.go`. Rewire `allTargets`:
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
Add the `rules` planner case after `case render.TargetSkills:` (cross-pack only, keyed by pack index):
```go
		case render.TargetRules:
			// In-pack collisions are caught in pack.Load. Here the map is
			// cross-pack, keyed by pack index (two sources may share a manifest
			// name), case-folded: a repo-path produced by two different packs is
			// ErrConfig naming both.
			ruleOwner := map[string]int{} // lowercased repo path -> pack index
			for i, p := range res.PackObjs {
				for _, f := range p.Fragments {
					if !render.FragmentNamesTarget(f, render.TargetRules) {
						continue
					}
					relPath := render.RuleFilePath(p.Manifest.Name, f)
					lc := strings.ToLower(relPath)
					if owner, ok := ruleOwner[lc]; ok && owner != i {
						return nil, fmt.Errorf("%w: rule file %s is produced by packs %s and %s; rename one fragment", esc.ErrConfig, relPath, res.PackObjs[owner].Manifest.Name, p.Manifest.Name)
					}
					ruleOwner[lc] = i
					content := render.RenderRule(f, p.Manifest.Name)
					res.Artifacts = append(res.Artifacts, Artifact{
						Path: relPath, Kind: KindFile,
						Hash: esc.HashBytes(render.NormalizeEndings([]byte(content))), Body: content,
					})
				}
			}
```
Add the filter-excludes-rules notice right after the empty-list expansion block:
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
- [ ] Print notices in `internal/cli/cli.go` `syncOnce` (stderr, human path only — `syncJSON` stays silent). After the applied/skipped stdout summary, before `publishSyncResult`:
```go
	for _, n := range plan.Notices {
		fmt.Fprintf(stderr, "  notice: %s\n", n)
	}
```
- [ ] Write the CLI notice + hand-edit-skip + CRLF-round-trip + symlink + orphan tests in `internal/cli/rules_notice_test.go`:
```go
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// governRulePack writes a temp repo governed by a local pack with one rules
// fragment, and returns the repo root. targetsLine, if non-empty, is appended
// verbatim to config.yaml.
func governRulePack(t *testing.T, fragBody, targetsLine string) string {
	t.Helper()
	pd := t.TempDir()
	man := "schema: 1\nname: acme\nversion: 1.0.0\nrules:\n  - rules/api.md\n"
	if err := os.WriteFile(filepath.Join(pd, "pack.yaml"), []byte(man), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(pd, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pd, "rules", "api.md"), []byte(fragBody), 0o644); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".escapement"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "schema: 1\npacks:\n  - source: " + pd + "\n    ref: \"\"\n    trust: unsigned\n" + targetsLine
	if err := os.WriteFile(filepath.Join(root, ".escapement", "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestSyncPrintsRulesFilterNotice(t *testing.T) {
	root := governRulePack(t, "---\ntargets: [rules]\n---\nbody\n", "targets:\n  - claude\n")
	code, out := run(t, root, "sync")
	if code != 0 {
		t.Fatalf("sync: %d\n%s", code, out)
	}
	if !strings.Contains(out, "targets rules") {
		t.Errorf("sync must warn that the filter excludes rules:\n%s", out)
	}
}

func TestRuleFileHandEditSkipAndForce(t *testing.T) {
	root := governRulePack(t, "---\ntargets: [rules]\n---\nbody\n", "")
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("sync: %d\n%s", code, out)
	}
	rule := filepath.Join(root, ".claude", "rules", "esc-acme-api.md")
	if err := os.WriteFile(rule, []byte("hand edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := run(t, root, "sync"); code != 0 || !strings.Contains(out, "skipped") {
		t.Fatalf("hand-edited rule file must be skipped (code=%d):\n%s", code, out)
	}
	if got, _ := os.ReadFile(rule); string(got) != "hand edited\n" {
		t.Errorf("skip must not overwrite: %q", got)
	}
	if code, out := run(t, root, "sync", "--force"); code != 0 {
		t.Fatalf("sync --force: %d\n%s", code, out)
	}
	if got, _ := os.ReadFile(rule); string(got) == "hand edited\n" {
		t.Error("--force must overwrite the rule file")
	}
}

func TestRuleFileCRLFRoundTrip(t *testing.T) {
	root := governRulePack(t, "---\r\ntargets: [rules]\r\n---\r\nbody line\r\n", "")
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("sync: %d\n%s", code, out)
	}
	if code, out := run(t, root, "status", "--check"); code != 0 {
		t.Fatalf("status --check must be clean after a CRLF fragment sync: %d\n%s", code, out)
	}
	if code, out := run(t, root, "sync"); code != 0 || !strings.Contains(out, "0 skipped") {
		t.Fatalf("second sync should be a clean no-op: %d\n%s", code, out)
	}
}

func TestSymlinkedRulesDirRefusedExit4(t *testing.T) {
	root := governRulePack(t, "---\ntargets: [rules]\n---\nbody\n", "")
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, ".claude", "rules")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	for _, cmd := range [][]string{{"sync"}, {"status"}, {"diff"}} {
		if code, out := run(t, root, cmd...); code != 4 {
			t.Errorf("esc %v on a symlinked .claude/rules must exit 4, got %d:\n%s", cmd, code, out)
		}
	}
}

func TestStatusReportsOrphanRuleFile(t *testing.T) {
	root := governRulePack(t, "---\ntargets: [rules]\n---\nbody\n", "")
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("sync: %d\n%s", code, out)
	}
	cfgPath := filepath.Join(root, ".escapement", "config.yaml")
	cfg, _ := os.ReadFile(cfgPath)
	if err := os.WriteFile(cfgPath, append(cfg, []byte("targets:\n  - claude\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := run(t, root, "status", "--check"); code == 0 {
		t.Errorf("an orphaned rule file must fail status --check:\n%s", out)
	}
}
```
- [ ] Run `go test ./internal/cli/ -run 'Rules|SymlinkedRulesDir|StatusReportsOrphanRuleFile|RuleFile'`. Expect PASS.
- [ ] Run `go test ./...`. Expect PASS: default-set repos (examples) now include `rules`, but no example pack ships a rules fragment yet (Task 10 adds acme-org's), so no rule files render and every example stays clean.
- [ ] `gofmt -w .`, `go vet ./...`.
- [ ] Commit: `feat(engine): plan per-fragment rule files and warn when a filter excludes rules` — body explains the cross-pack-only collision (in-pack lives in pack.Load), the index keying, and the excluded-filter notice.

---

## Task 6 — Engine planner for `agents-skills`

**Files**
- Modify `internal/engine/engine.go` (extract the skills planner into a root-parameterized helper; add `case render.TargetSkills:` / `case render.TargetAgentsSkills:`).
- Modify `internal/engine/apply.go` (`ownedSkillPath` accepts both roots ~291-293).
- Modify `internal/engine/status.go` (`SkillDuplicate` hashByPath scoped to `.claude/skills` ~164-171).
- Create `internal/engine/agents_skills_test.go`; create `internal/cli/agents_skills_test.go`.

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

func skillCfg(pd string, targetsList ...string) *config.Config {
	return &config.Config{
		Packs:   []config.PackRef{{Source: pd, Ref: "", Trust: "unsigned"}},
		Targets: targetsList,
	}
}

func TestAgentsSkillsRendersSecondRoot(t *testing.T) {
	pd := writeSkillPack(t, "acme", map[string]string{"SKILL.md": "hi\n"})
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	res, err := planFromConfig(context.Background(), t.TempDir(), skillCfg(pd, "skills", "agents-skills"))
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	want := map[string]bool{".claude/skills/esc-acme-demo": false, ".agents/skills/esc-acme-demo": false}
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
	res, err := planFromConfig(context.Background(), t.TempDir(), skillCfg(pd)) // empty = defaults
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	for _, a := range res.Artifacts {
		if a.Path == ".agents/skills/esc-acme-demo" {
			t.Fatal("agents-skills must not render under the default target set")
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

func TestSkillDuplicateNeverAttachesToAgentsSkills(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".claude", "skills", "esc-acme-demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pd := writeSkillPack(t, "acme", map[string]string{"SKILL.md": "hi\n"})
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".escapement"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := skillCfg(pd, "skills", "agents-skills").Save(root); err != nil {
		t.Fatal(err)
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
- [ ] Run `go test ./internal/engine/ -run 'AgentsSkills|OwnedSkillPathBoth|SkillDuplicateNever'`. Expected failure: no `.agents/skills` artifact (switch default errors on `agents-skills`); `ownedSkillPath(".agents/...")` false.
- [ ] Implement in `internal/engine/engine.go`: replace the inline `case render.TargetSkills:` body with two calls to a shared helper:
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
Add the helper (mirrors the current inline code, parameterized on `rootDir`, per-root lowercased collision map):
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
			target := path.Join(rootDir, "skills", e.DirName(p.Manifest.Name))
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
Delete the now-duplicated inline skills body.
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
- [ ] Run `go test ./internal/engine/`. Expect PASS.
- [ ] Write the CLI end-to-end test `internal/cli/agents_skills_test.go` (both roots, independent retirement, amendment preservation). `governSkillPack` returns both the repo root and the pack dir so the retire step can rewrite the config without re-parsing it:
```go
package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func governSkillPack(t *testing.T, targetsLine string) (root, packDir string) {
	t.Helper()
	packDir = t.TempDir()
	man := "schema: 1\nname: acme\nversion: 1.0.0\nskills:\n  - skills/demo\n"
	if err := os.WriteFile(filepath.Join(packDir, "pack.yaml"), []byte(man), 0o644); err != nil {
		t.Fatal(err)
	}
	sk := filepath.Join(packDir, "skills", "demo")
	if err := os.MkdirAll(sk, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sk, "SKILL.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".escapement"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeSkillConfig(t, root, packDir, targetsLine)
	return root, packDir
}

func writeSkillConfig(t *testing.T, root, packDir, targetsLine string) {
	t.Helper()
	cfg := "schema: 1\npacks:\n  - source: " + packDir + "\n    ref: \"\"\n    trust: unsigned\n" + targetsLine
	if err := os.WriteFile(filepath.Join(root, ".escapement", "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAgentsSkillsBothRootsAndIndependentRetirement(t *testing.T) {
	root, packDir := governSkillPack(t, "targets:\n  - skills\n  - agents-skills\n")
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("sync: %d\n%s", code, out)
	}
	claude := filepath.Join(root, ".claude", "skills", "esc-acme-demo")
	agents := filepath.Join(root, ".agents", "skills", "esc-acme-demo")
	for _, p := range []string{claude, agents} {
		if _, err := os.Stat(filepath.Join(p, "SKILL.md")); err != nil {
			t.Fatalf("missing %s: %v", p, err)
		}
		if err := os.WriteFile(filepath.Join(p, "TEAM.md"), []byte("mine\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Drop agents-skills; the .agents root retires, .claude root stays.
	writeSkillConfig(t, root, packDir, "targets:\n  - skills\n")
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("resync: %d\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(agents, "SKILL.md")); !os.IsNotExist(err) {
		t.Error(".agents SKILL.md should be retired")
	}
	if _, err := os.Stat(filepath.Join(agents, "TEAM.md")); err != nil {
		t.Error(".agents TEAM.md (team-added) must survive retirement")
	}
	if _, err := os.Stat(agents); err != nil {
		t.Error(".agents skill directory must remain because a team file is left in it")
	}
	if _, err := os.Stat(filepath.Join(claude, "SKILL.md")); err != nil {
		t.Error(".claude SKILL.md must remain")
	}
	if _, err := os.Stat(filepath.Join(claude, "TEAM.md")); err != nil {
		t.Error(".claude TEAM.md must remain")
	}
}
```
- [ ] Run `go test ./internal/cli/ -run AgentsSkills`. Expect PASS.
- [ ] Run `go test ./...`. Expect PASS.
- [ ] `gofmt -w .`, `go vet ./...`.
- [ ] Commit: `feat(engine): render skills a second time under .agents/skills as an opt-in target` — body explains two byte-identical copies (Claude Code vs Kimi Code) over a symlink or a single configurable root, and the duplicate-scan scoping.

---

## Task 7 — `esc diff --against` diffs rule files by path

**Files**
- Modify `internal/engine/diff.go` (`PolicyDiff` add a rule-file diff; add `ruleArtifactBodies`, `rulePolicyDiff`).
- Create `internal/engine/diff_rules_test.go`.

**Interfaces**
- Produces (diff.go, unexported): `func ruleArtifactBodies([]Artifact) map[string]string`, `func rulePolicyDiff(ctx, cur, next *PlanResult) (string, error)`.
- Consumes: existing `gitDiff`.

**Steps**
- [ ] Write the failing test `internal/engine/diff_rules_test.go`:
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
		t.Errorf("unchanged rule file must not appear:\n%s", out)
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
		t.Errorf("a paths change must appear:\n%s", out)
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
// ruleArtifactBodies returns the body of every rule-file artifact (KindFile
// under .claude/rules), keyed by path.
func ruleArtifactBodies(artifacts []Artifact) map[string]string {
	bodies := map[string]string{}
	for _, a := range artifacts {
		if a.Kind == KindFile && strings.HasPrefix(a.Path, ".claude/rules/") {
			bodies[a.Path] = a.Body
		}
	}
	return bodies
}

// rulePolicyDiff diffs rule files present in either plan, matched by path:
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
- [ ] Run `go test ./internal/engine/ -run PolicyDiffRule`. Expect PASS.
- [ ] `gofmt -w .`, `go vet ./...`, `go test ./...`.
- [ ] Commit: `feat(diff): esc diff --against now covers rule files by path`.

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
	if !strings.Contains(readConfig(t, root), "- rules") {
		t.Errorf("init must add rules to an explicit targets list:\n%s", readConfig(t, root))
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
- [ ] Implement `.agents/skills` detection in `internal/cli/initscan.go` `detectExisting`, after the `.claude/skills` block:
```go
	if info, err := os.Lstat(filepath.Join(root, ".agents", "skills")); err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		// Lstat + IsDir + non-symlink: a `.agents/skills -> ../.claude/skills`
		// layout must not be pre-filled, or the first sync would fail sync's
		// symlink refusal at exit 4.
		if entries, err := os.ReadDir(filepath.Join(root, ".agents", "skills")); err == nil && len(entries) > 0 {
			out = append(out, Detected{Target: render.TargetAgentsSkills, Path: ".agents/skills"})
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
	// A newly governed repo with an explicit list should still receive rule
	// files by default; agents-skills only when it was actually detected.
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
- [ ] Run `go test ./...`. Expect PASS. Note: `TestConfigTemplateTargets` passes an explicit list to `configTemplate` directly (not through `detectedTargets`), so it is unaffected; `TestInitExplainsDetection`'s "clean files" case (CLAUDE.md + .mcp.json) now writes `targets: [claude, mcp, rules]` — that test asserts the explanation prose, not the config body, and `rules` is not a `Detected` item so it never appears in the "Found ..." text.
- [ ] `gofmt -w .`, `go vet ./...`.
- [ ] Commit: `feat(cli): esc init manages rules by default and pre-fills agents-skills for a real .agents/skills` — body explains the Lstat symlink guard.

---

## Task 9 — Shapetest matrix for the rule-file write, removal, and adoption compare

This matrix is a regression guard added AFTER the machinery landed in Tasks 2/5, so it has no fail-first step: it pins that a fully escapement-owned whole-file write survives awkward body shapes (no trailing newline, real CRLF checkout, empty body), that a path-scoped file keeps its frontmatter fence at byte 0 after every write, and that adoption and removal both compare on the endings-normalized hash. It lives in `internal/engine` (it needs `render.RenderRule`, `esc.HashBytes`, `classify`/`Apply` together), matching how `internal/render` and `internal/cli` own their own matrix tests while `internal/shapetest` stays import-light.

**Files**
- Create `internal/engine/rulefile_shape_test.go`.

**Steps**
- [ ] Write the test `internal/engine/rulefile_shape_test.go` (regression guard: no fail-first run):
```go
package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/pack"
	"github.com/tensorgroup/openescapement/internal/render"
)

// ruleBodyShapes are awkward fragment bodies a rule-file write must survive.
// The "paths" case also proves the frontmatter fence lands at byte 0.
func ruleBodyShapes() map[string]pack.Fragment {
	return map[string]pack.Fragment{
		"no-trailing-newline": {Path: "rules/api.md", Targets: []string{"rules"}, Body: "line one\nline two"},
		"crlf":                {Path: "rules/api.md", Targets: []string{"rules"}, Body: "line one\r\nline two\r\n"},
		"empty":               {Path: "rules/api.md", Targets: []string{"rules"}, Body: ""},
		"paths":               {Path: "rules/api.md", Targets: []string{"rules"}, Paths: []string{"src/**"}, Body: "body\n"},
	}
}

func TestRuleFileWriteAdoptRemoveShapeMatrix(t *testing.T) {
	for name, frag := range ruleBodyShapes() {
		t.Run(name, func(t *testing.T) {
			content := render.RenderRule(frag, "acme")
			relPath := render.RuleFilePath("acme", frag)
			a := Artifact{Path: relPath, Kind: KindFile, Body: content, Hash: esc.HashBytes(render.NormalizeEndings([]byte(content)))}

			// (1) write path: apply writes it; classify reports InSync; the file
			// ends in a newline; a paths-scoped file has its fence at byte 0.
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
			if len(frag.Paths) > 0 && !bytes.HasPrefix(got, []byte("---\n")) {
				t.Errorf("paths frontmatter must be at byte 0 after write: %q", got)
			}

			// (2) adoption: a REAL CRLF copy on disk with no lock entry is
			// adopted because the compare normalizes endings.
			root2 := t.TempDir()
			p := filepath.Join(root2, filepath.FromSlash(relPath))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(strings.ReplaceAll(content, "\n", "\r\n")), 0o644); err != nil {
				t.Fatal(err)
			}
			res, err := Apply(root2, &PlanResult{Artifacts: []Artifact{a}}, false)
			if err != nil {
				t.Fatalf("apply adopt: %v", err)
			}
			if len(res.Adopted) != 1 {
				t.Errorf("a CRLF copy of identical content must be adopted, got adopted=%v skipped=%+v", res.Adopted, res.Skipped)
			}

			// (3) removal: an orphaned rule file (no plan) whose on-disk copy is
			// CRLF still hashes equal, so it is removed, not declined.
			root3 := t.TempDir()
			if _, err := Apply(root3, &PlanResult{Artifacts: []Artifact{a}}, false); err != nil {
				t.Fatal(err)
			}
			dp := filepath.Join(root3, filepath.FromSlash(relPath))
			onDisk, _ := os.ReadFile(dp)
			if err := os.WriteFile(dp, bytes.ReplaceAll(onDisk, []byte("\n"), []byte("\r\n")), 0o644); err != nil {
				t.Fatal(err)
			}
			resR, err := Apply(root3, &PlanResult{}, false)
			if err != nil {
				t.Fatalf("apply remove: %v", err)
			}
			if len(resR.Skipped) != 0 {
				t.Errorf("unedited orphan rule file must not be declined: %+v", resR.Skipped)
			}
			if _, err := os.Stat(dp); !os.IsNotExist(err) {
				t.Errorf("orphan rule file must be removed")
			}
		})
	}
}
```
- [ ] Run `go test ./internal/engine/ -run RuleFileWriteAdoptRemoveShapeMatrix`. Expect PASS (regression guard; the machinery landed in Tasks 2/5).
- [ ] `gofmt -w .`, `go vet ./...`, `go test ./...`.
- [ ] Commit: `test(engine): rule-file write/adopt/remove matrix across body shapes` — body notes the matrix lives in engine because it needs render+apply, keeping shapetest import-light.

---

## Task 10 — Examples: acme-org rule fragment, version bump, regenerate governed-service

**Files**
- Create `examples/packs/acme-org/rules/api-handlers.md`.
- Modify `examples/packs/acme-org/pack.yaml` (add the rule; bump `0.1.1` → `0.1.2`).
- Regenerate `examples/governed-service/*`.
- Modify `internal/cli/examples_test.go` (pinned block string; acme-org rule assertions).
- Create `internal/cli/examples_agents_skills_test.go`.

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
- [ ] Edit `examples/packs/acme-org/pack.yaml`: bump `version: 0.1.1` → `version: 0.1.2`; add `  - rules/api-handlers.md` to the `rules:` list.
- [ ] Regenerate the shipped governed-service tree:
```
cd examples/governed-service
ESC_CACHE_DIR=$(mktemp -d) go run ../../cmd/esc sync
ESC_CACHE_DIR=$(mktemp -d) go run ../../cmd/esc status --check ; echo "status --check exit: $?"
```
Confirm the run creates `examples/governed-service/.claude/rules/esc-acme-org-api-handlers.md` with `paths:` frontmatter, updates block headers to `packs=acme-org@0.1.2`, refreshes GOVERNANCE.md (now listing the rule file) and `.escapement/escapement.lock`, and `status --check` exits 0. Stage every changed/created file under `examples/governed-service/`.
- [ ] Update `internal/cli/examples_test.go`: change the pinned substring `"escapement:begin packs=acme-org@0.1.1"` → `"...@0.1.2"`. Add imports `strings` and `gopkg.in/yaml.v3` if absent. Add to `TestEveryExamplePackSyncs`, inside the `t.Run(name, ...)` closure after the GOVERNANCE.md check:
```go
			if name == "acme-org" {
				content, err := os.ReadFile(filepath.Join(root, ".claude", "rules", "esc-acme-org-api-handlers.md"))
				if err != nil {
					t.Fatalf("acme-org rule file missing: %v", err)
				}
				s := string(content)
				if !strings.HasPrefix(s, "---\n") {
					t.Fatalf("rule file has no frontmatter:\n%s", s)
				}
				end := strings.Index(s[4:], "\n---\n")
				if end < 0 {
					t.Fatalf("rule frontmatter unterminated:\n%s", s)
				}
				var meta struct {
					Paths []string `yaml:"paths"`
				}
				if err := yaml.Unmarshal([]byte(s[4:4+end]), &meta); err != nil {
					t.Fatalf("rule frontmatter parse: %v", err)
				}
				if len(meta.Paths) != 1 || meta.Paths[0] != "src/api/**" {
					t.Errorf("rule paths = %v, want [src/api/**]", meta.Paths)
				}
			}
```
- [ ] Create `internal/cli/examples_agents_skills_test.go`. Copy the WHOLE examples tree (as `TestExamplesSync` does, since governed-service's config points at `../packs/acme-org`), then add `agents-skills` to the governed-service targets and sync:
```go
package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExamplesAgentsSkillsVariant(t *testing.T) {
	src, err := filepath.Abs("../../examples")
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := copyTree(src, tmp); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	root := filepath.Join(tmp, "governed-service")
	cfgPath := filepath.Join(root, ".escapement", "config.yaml")
	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg = append(cfg, []byte("targets:\n  - claude\n  - agents\n  - gemini\n  - governance\n  - mcp\n  - skills\n  - rules\n  - agents-skills\n")...)
	if err := os.WriteFile(cfgPath, cfg, 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("sync: %d\n%s", code, out)
	}
	for _, p := range []string{
		".claude/skills/esc-acme-org-acme-vault/SKILL.md",
		".agents/skills/esc-acme-org-acme-vault/SKILL.md",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
	if code, out := run(t, root, "status", "--check"); code != 0 {
		t.Fatalf("status --check must be clean with agents-skills: %d\n%s", code, out)
	}
}
```
- [ ] Run `go test ./internal/cli/`. Expect PASS, including `TestExamplesSync`, `TestEveryExamplePackSyncs`, `TestExamplesLockMatchesShippedFiles`.
- [ ] Run `go test ./...`. Expect PASS.
- [ ] `gofmt -w .`, `go vet ./...`.
- [ ] Commit: `docs(examples): acme-org ships a path-scoped rule; regenerate governed-service at 0.1.2`.

---

## Task 11 — Docs: README, pack-authoring, cli-and-portal, CHANGELOG

**Files**
- Modify `README.md` (target list ~7 and the pack-authoring reference ~223).
- Modify `docs/pack-authoring.md` (rules section).
- Modify `docs/cli-and-portal.md` (managed-policy decision sentence).
- Modify `CHANGELOG.md` (`### Added` entries; the `### Changed` entries landed in Task 2).

**Steps**
- [ ] README: extend the channels list (line ~7) to mention `.claude/rules/` and `.agents/skills/`. Add a short "Targets" note: `rules` and `agents-skills` are built-ins; an empty `targets:` list renders every built-in EXCEPT `agents-skills` (opt-in); an explicit `targets:` list is an exhaustive filter, so a repo with one must add `rules` for rule files and `agents-skills` for the second skills root; a pack shipping a `targets: [rules]` fragment needs an `esc` new enough to know the target (an older binary rejects it as an unknown target, exit 1).
- [ ] `docs/pack-authoring.md`: add a `## Path-scoped rule files` section: opt in with `targets: [rules]`; add `paths:` (a non-empty list of glob strings) to scope, or omit it for an unconditional rule; keep the file small (loaded like `.claude/CLAUDE.md`); the notice line is escapement's, not yours; the `esc-` prefix under `.claude/rules/` is a reserved namespace escapement owns; the filename stem must match `^[a-z0-9][a-z0-9-]*$`; a path-scoped fragment cannot be composed by the portal's starter-set adoption.
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
- Fragment line endings are normalized to LF at load, so a CRLF-authored
  fragment keeps its frontmatter and scope. Packs whose fragments were
  previously CRLF re-render once as a one-time `stale` on the next sync.
```
- [ ] Run `go test ./...`. Expect PASS.
- [ ] Commit: `docs: document the rules and agents-skills targets and the managed-policy decision`.

---

## Self-review (performed against every spec section)

Spec → task coverage:
- Rules pack authoring rules (paths only with rules; strict yaml.Node parse rejecting null/`~`/empty-seq/scalar/non-string/empty/newline; both rules+block; unconditional; explicit-name test; stem pattern; LF-normalize at load; compose refuses paths incl. single part) → Task 3; the explicit-name test drives the planner in Task 5 via `render.FragmentNamesTarget`.
- Rendered file (path, frontmatter via yaml.v3, notice without version, TrimSpace + one newline, byte-0 frontmatter, GOVERNANCE.md lists rules) → Task 4. Collisions: in-pack `ErrManifest` in `pack.Load` (Task 3), cross-pack `ErrConfig` keyed by pack index, case-folded (Task 5).
- Whole-file machinery for every KindFile (desired-hash normalization; occupied gate + byte-identical adoption; `--force` no override; orphan pass with empty-dir cleanup; canonical `ownedFilePath`/`ownedRulePath`; `identicalFileContent` symlink refusal; pre-read symlink refusal in plan/status/diff/sync with the updated message; status orphan-file errors propagate; init GOVERNANCE.md occupied wording; skipHint + `TestSkipHintPerCause`; README enum; CHANGELOG; AGENTS.md never-adopt/retirement/lockfile-deletion) → Task 2.
- Targets/defaults/init (`OptIn`, `Defaults()`, `allTargets` [rewired in Task 5], `targetOrder` derived from the table with an equality test, `render.Target*` single-sourced from `targets.Name*`, `PolicyDiff` default, init writes rules, pre-fills agents-skills via Lstat, older-binary note) → Tasks 1, 5, 8.
- Agents-skills (second root, `ownedSkillPath` both roots, per-root lowercased collision, SkillDuplicate scoped to `.claude/skills`, two-root retirement/amendment) → Task 6.
- `esc diff --against` rule files → Task 7. Shapetest matrix → Task 9. Examples → Task 10. Docs → Tasks 2 + 11.

Deviations and accepted behavior, recorded:
1. **`pack` cannot import `render`** (cycle). LF normalization is inlined in `pack`/`compose` with `strings.ReplaceAll`; semantics match `render.NormalizeEndings` (one conversion, CRLF→LF).
2. **`render` imports `targets`** to define the `Target*` strings once as `targets.Name*` (no cycle: targets imports only stdlib).
3. **`allTargets` rewire is in Task 5**, paired with the `rules` switch case, so the target switch stays total. Task 1 rewires only `targetOrder` (safe: presentation/detection only), `render.Target*`, `isBuiltInTarget`, and `PolicyDiff`'s default.
4. **builtIns table reordered** so `mcp` precedes `skills`, making the derived `targetOrder` match `esc init`'s existing mcp-before-skills presentation (so no pinned init-output test reorders in Task 1) and flipping only `engine.allTargets` iteration order in Task 5 — verified no test pins sync's artifact ordering, and the lockfile sorts by path.
5. **`prospectiveContent` symlink refusal is unconditional** (KindBlock and KindFile). Apply already refuses symlinks before writing a block, so plan-time read refusal is consistent; it changes `esc status`/`esc diff`/`esc render` from "read through a symlinked target" to exit 4, recorded in the CHANGELOG Changed entry and covered by tests.
6. **Status orphan-file pass propagates** containment/symlink errors (exit 4), deliberately unlike the older block/dir orphan passes that `continue`; the spec requires a retired rule file behind a symlink to be refused, not silently dropped.
7. **`TrimSpace` on a rule body** follows `render.Compose`'s block precedent; it also strips leading whitespace from a body that begins with indentation, which the spec accepts.
8. **LF normalization at load also affects block rendering of CRLF fragments**: a pack whose fragments were CRLF re-renders once as a one-time `stale` on the next sync — noted in the CHANGELOG Added entry.
9. **Shapetest matrix placement**: the rule-file matrix lives in `internal/engine` (needs render+apply+classify); `internal/shapetest` stays import-light, matching how render/cli own their own matrix tests. It is a regression guard, so it has no fail-first step.
10. **Portal**: `models.go` already routes `ComposeFragments` errors through `serverError` (verified in `handleModelAdopt`/`handleModelAdoptSave`), so the compose refusal surfaces with no portal code change.

No placeholders remain; every identifier used across tasks (`SkipUnmanagedFileAtTarget`, `SkipOrphanFileEdited`, `canonicalRel`, `ownedFilePath`, `ownedRulePath`, `identicalFileContent`, `PlanResult.Notices`, `planSkillDirs`, `parsePaths`, `partHasPaths`, `namesRulesTarget`, `render.RenderRule`, `render.RuleFilePath`, `render.FragmentNamesTarget`, `targets.Defaults`, `targets.KindRulesDir`, `Info.OptIn`, `builtInTargetOrder`) is defined in the task that first uses it.
