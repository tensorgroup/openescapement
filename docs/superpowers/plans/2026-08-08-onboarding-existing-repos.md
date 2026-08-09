# Onboarding Into Existing Repos Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `esc init` aware of instruction files a repo already has: detect them, pre-fill config from what is actually there, explain what the first sync will do, and offer to place the managed-block marker.

**Architecture:** `esc init` gains a detection pass over the files in `render.TargetFile` plus `.mcp.json` and existing skill directories. Detection drives three things: the `targets` list written into the generated config, an explanation printed in the amendment model's vocabulary, and an optional per-file offer to write `<!-- escapement:block -->` at a chosen position. Reconciling a team's existing rules against a pack's is semantic work and does NOT go in the CLI; it ships as a skill inside a pack.

**Tech Stack:** Go 1.24, stdlib plus `gopkg.in/yaml.v3`. No new dependencies.

**Source spec:** `docs/superpowers/specs/2026-08-05-onboarding-existing-repos-design.md`, sections 2, 3, 4 (init portion), and 6. **Section 1 (top placement) already shipped** in the local-amendment-model plan, commit `fc70378`. Do not re-implement it.

## Global Constraints

- Single external dependency policy: `gopkg.in/yaml.v3` only. Everything else stdlib, system `git` via `os/exec`. No new dependencies in this plan.
- Renderer invariants: bytes outside a managed block are never modified; nothing is written after a verification or constraint failure; all writes are atomic; renderer output is deterministic.
- Sentinel errors in `internal/esc` map to exit codes: 0 ok, 1 drift/constraint, 2 usage, 3 integrity/signature, 4 other. `esc.ErrConfig` maps to 2.
- `go test ./...`, `go vet ./...`, and `gofmt -w .` must pass before any task is considered done.
- Integration tests build real temp git repos, no mocks. `ESC_CACHE_DIR` overrides the pack cache.
- No em-dashes in any NEW user-facing copy or docs. The repo's existing docs contain some; do not do a repo-wide rewrite.
- No test may block on real stdin. Every interactive path needs an injectable seam, following the `maybeUpdates` pattern in `internal/cli/cli.go`.

## Existing Code This Plan Builds On

Reference these by symbol, not line number: a concurrent residual sweep is editing `internal/cli/cli.go` and `internal/engine/apply.go`, so line numbers will have shifted.

| Symbol | Where | Relevance |
|---|---|---|
| `cmdInit(root string, stdout io.Writer) error` | `internal/cli/cli.go` | The function being extended. Currently writes config, `allowed_signers`, `.escapement/.gitignore`, and errors if config exists. |
| `configTemplate`, `signersTemplate` | `internal/cli/cli.go` | Const strings. `configTemplate` becomes a function in Task 1. |
| `maybeUpdates` | `internal/cli/cli.go` | The precedent for an injectable interactive seam. Task 2 mirrors its shape. |
| `updatecheck.MaybeIO` | `internal/updatecheck/throttle.go` | Shows how interactivity and the prompt reader are made explicit for tests. |
| `render.TargetFile` | `internal/render/render.go` | `map[string]string` with exactly four entries: claude→CLAUDE.md, agents→AGENTS.md, gemini→GEMINI.md, governance→GOVERNANCE.md. Skills and MCP are separate target kinds and are NOT in this map. |
| `render.Placeholder` | `internal/render/block.go` | `"<!-- escapement:block -->"`. |
| `render.Extract` | `internal/render/block.go` | Returns `(nil, nil)` when no block, error on corrupt structure. |
| `render.TargetSkills`, `render.TargetMCP` | `internal/render/render.go` | Target name constants for the two non-file kinds. |
| `config.Config` | `internal/config/config.go` | Has `Targets []string` with `omitempty`; empty means all targets. |
| `source.git` | `internal/source/source.go` | Unexported. Task 2 needs its own small runner in `internal/cli`; do not export this one. |

## File Structure

| File | Responsibility | Change |
|---|---|---|
| `internal/cli/initscan.go` | Detect existing instruction files and report what was found | Create |
| `internal/cli/initoffer.go` | The interactive placement offer and its guards | Create |
| `internal/cli/cli.go` | `cmdInit` wiring, flags, config template | Modify |
| `internal/portal/seed/repos.go` | Seeded demo pack gains the reconcile skill | Modify |

`initscan.go` and `initoffer.go` are separate files because detection is pure and offer is interactive plus filesystem-mutating. Keeping them apart is what makes detection testable without a seam.

---

### Task 1: Detect existing instruction files, bootstrap config, explain

**Files:**
- Create: `internal/cli/initscan.go`
- Modify: `internal/cli/cli.go` (`cmdInit`, `configTemplate`)
- Test: `internal/cli/initscan_test.go`

**Interfaces:**
- Produces:
  - `type Detected struct { Target, Path string; Bytes, Lines int; HasBlock, HasPlaceholder bool }`
  - `func detectExisting(root string) ([]Detected, error)` returns entries sorted by `Path`, empty slice (never nil) when nothing is found
  - `func configTemplate(targets []string) string` replaces the const of the same name

- [ ] **Step 1: Write the failing test**

Create `internal/cli/initscan_test.go`:

```go
package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectExisting(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"CLAUDE.md":  "# Project\n\nour rules\nline three\n",
		".mcp.json":  `{"mcpServers":{"ours":{"command":"a"}}}`,
	})
	if err := os.MkdirAll(filepath.Join(root, ".claude", "skills", "team-thing"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := detectExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]Detected{}
	for _, d := range got {
		byPath[d.Path] = d
	}

	claude, ok := byPath["CLAUDE.md"]
	if !ok {
		t.Fatal("CLAUDE.md not detected")
	}
	if claude.Target != "claude" || claude.Lines != 4 {
		t.Errorf("claude = %+v, want target=claude lines=4", claude)
	}
	if claude.HasBlock || claude.HasPlaceholder {
		t.Errorf("plain file must report no block and no placeholder: %+v", claude)
	}
	if _, ok := byPath[".mcp.json"]; !ok {
		t.Error(".mcp.json not detected")
	}
	if _, ok := byPath["AGENTS.md"]; ok {
		t.Error("absent file must not be detected")
	}
}

func TestDetectExistingEmptyRepo(t *testing.T) {
	got, err := detectExisting(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Error("must return an empty slice, never nil")
	}
	if len(got) != 0 {
		t.Errorf("got %+v, want none", got)
	}
}

func TestDetectExistingFindsBlockAndPlaceholder(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"AGENTS.md": "before\n" + render.Placeholder + "\nafter\n",
	})
	got, err := detectExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].HasPlaceholder || got[0].HasBlock {
		t.Errorf("got %+v, want one entry with HasPlaceholder", got)
	}
}

func TestConfigTemplateTargets(t *testing.T) {
	with := configTemplate([]string{"claude", "mcp"})
	if !strings.Contains(with, "targets:\n  - claude\n  - mcp\n") {
		t.Errorf("targets not rendered:\n%s", with)
	}
	without := configTemplate(nil)
	if strings.Contains(without, "targets:") {
		t.Errorf("empty targets must be omitted entirely:\n%s", without)
	}
	if !strings.Contains(without, "schema: 1") {
		t.Errorf("base template lost:\n%s", without)
	}
}
```

Add the `render` and `strings` imports the test needs.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run "TestDetect|TestConfigTemplate" -v`
Expected: FAIL, undefined: `detectExisting`, `Detected`; `configTemplate` is a const not a function.

- [ ] **Step 3: Implement detection**

Create `internal/cli/initscan.go`:

```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"

	"github.com/tensorgroup/openescapement/internal/render"
)

// Detected is one instruction file or directory already present in a repo
// before escapement was introduced.
type Detected struct {
	Target         string // a render.Target* constant
	Path           string // repo-relative, slash-separated
	Bytes, Lines   int
	HasBlock       bool // already carries a managed block
	HasPlaceholder bool // already carries the placement marker
}

// detectExisting reports the instruction files a repo already has. It never
// modifies anything. The result is sorted by Path and is empty, not nil, when
// nothing is found.
func detectExisting(root string) ([]Detected, error) {
	out := []Detected{}
	for target, name := range render.TargetFile {
		content, err := os.ReadFile(filepath.Join(root, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		d := Detected{
			Target: target, Path: name,
			Bytes: len(content),
			Lines: bytes.Count(content, []byte("\n")),
			HasPlaceholder: bytes.Contains(content, []byte(render.Placeholder)),
		}
		if len(content) > 0 && !bytes.HasSuffix(content, []byte("\n")) {
			d.Lines++
		}
		// A corrupt block still counts as present: the point is to warn the
		// user, and `esc sync` will report the corruption properly.
		if b, err := render.Extract(content); err != nil || b != nil {
			d.HasBlock = true
		}
		out = append(out, d)
	}
	if info, err := os.Stat(filepath.Join(root, ".mcp.json")); err == nil && !info.IsDir() {
		out = append(out, Detected{Target: render.TargetMCP, Path: ".mcp.json", Bytes: int(info.Size())})
	}
	if entries, err := os.ReadDir(filepath.Join(root, ".claude", "skills")); err == nil && len(entries) > 0 {
		out = append(out, Detected{Target: render.TargetSkills, Path: ".claude/skills"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
```

- [ ] **Step 4: Convert `configTemplate` to a function**

In `internal/cli/cli.go`, replace the `configTemplate` const with:

```go
func configTemplate(targets []string) string {
	var b strings.Builder
	b.WriteString(`schema: 1
packs: []
# Example:
#   packs:
#     - source: github.com/acme/policy-packs//org
#       ref: v1.0.0
#     - source: ../local-packs/team   # local dirs need explicit trust
#       ref: ""
#       trust: unsigned
allowed_signers_file: .escapement/allowed_signers
`)
	if len(targets) > 0 {
		b.WriteString("# Detected in this repo at `esc init`. Remove a line to stop managing that target.\ntargets:\n")
		for _, t := range targets {
			b.WriteString("  - " + t + "\n")
		}
	}
	return b.String()
}
```

Preserve the existing comment block byte-for-byte; only the `targets` section is new.

- [ ] **Step 5: Wire detection into `cmdInit` and print the explanation**

`cmdInit` runs `detectExisting` BEFORE writing anything, so a detection error aborts before the repo is touched. Build the target list from the detected entries in `render.TargetFile` order plus mcp and skills, deduplicated, and pass it to `configTemplate`.

After writing config, print what was found and what the first sync will do, in the amendment model's vocabulary:

```
Found CLAUDE.md (183 lines) and .mcp.json.

`esc sync` will insert a managed block at the top of CLAUDE.md and add pack
MCP servers alongside your existing ones. Your current content is preserved
byte for byte and reported as a local amendment.

To place the block somewhere else, put <!-- escapement:block --> where you
want it before syncing.
```

When nothing is detected, print the existing "Initialized ... Add pack sources" message unchanged. When a detected file already has a block or a placeholder, say so instead of promising to insert one.

- [ ] **Step 6: Run to verify pass**

Run: `go test ./internal/cli/ -v && go vet ./... && gofmt -l .`
Expected: PASS, `gofmt -l` prints nothing. Existing `TestInitAndRenderStdout` must still pass; if it asserts on the old output, update it deliberately and say so in your report.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/
git commit -m "feat(cli): esc init detects existing instruction files

Pre-fills targets from what the repo actually has, so a first sync does
not conjure a GEMINI.md into a repo that only uses CLAUDE.md, and states
in the amendment model's vocabulary what will happen to content already
there."
```

---

### Task 2: The placement offer

**Files:**
- Create: `internal/cli/initoffer.go`
- Modify: `internal/cli/cli.go` (`cmdInit` signature and flags)
- Test: `internal/cli/initoffer_test.go`

**Interfaces:**
- Consumes: `Detected`, `detectExisting` (Task 1)
- Produces:
  - `type placement string` with `placeDefault`, `placeAbove`, `placeEnd`
  - `var initOffer = func(w io.Writer, in io.Reader, interactive bool, d Detected) placement` the injectable seam
  - `func applyPlacement(root string, d Detected, p placement) error` writes the placeholder atomically
  - `func fileIsClean(ctx context.Context, root, path string) (bool, error)` git worktree check
  - `cmdInit` gains a `--yes` flag

**The option set, and why it is three and not four.** Spec §1's top placement already inserts a new block AFTER leading YAML frontmatter and a leading H1. So "top" and "after the heading" are the same position, and offering both would mean the recommended choice writes a placeholder that changes nothing. Each option must do something distinct:

- `placeDefault` (`k`, recommended) keeps the shipped behavior: the block lands at the top, after any frontmatter and title, at first sync. **Writes nothing.** This is also what declining does.
- `placeAbove` (`a`) writes the placeholder at byte 0, above the title, for a team that wants policy to be the literal first thing in the file.
- `placeEnd` (`e`) writes the placeholder at the end.

There is no separate skip: `placeDefault` and a declined or empty answer are the same outcome, so collapsing them removes a distinction a user would have to think about for no gain.

- [ ] **Step 1: Write the failing tests**

Create `internal/cli/initoffer_test.go`:

```go
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/render"
)

func TestApplyPlacement(t *testing.T) {
	cases := []struct {
		name, existing string
		p              placement
		wantPrefix     string
	}{
		{"above", "# P\n\nrules\n", placeAbove, render.Placeholder + "\n# P\n"},
		{"end", "# P\n\nrules\n", placeEnd, "# P\n\nrules\n" + render.Placeholder + "\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, map[string]string{"CLAUDE.md": tc.existing})
			d := Detected{Target: "claude", Path: "CLAUDE.md"}
			if err := applyPlacement(root, d, tc.p); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(got), tc.wantPrefix) {
				t.Errorf("got:\n%q\nwant prefix:\n%q", got, tc.wantPrefix)
			}
			if !strings.Contains(string(got), "rules\n") {
				t.Errorf("original content lost:\n%s", got)
			}
			if strings.Count(string(got), render.Placeholder) != 1 {
				t.Errorf("placeholder must appear exactly once:\n%s", got)
			}
		})
	}
}

func TestApplyPlacementDefaultWritesNothing(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"CLAUDE.md": "rules\n"})
	before, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err := applyPlacement(root, Detected{Target: "claude", Path: "CLAUDE.md"}, placeDefault); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if string(before) != string(after) {
		t.Error("the default placement must not touch the file")
	}
}

func TestApplyPlacementDefaultMatchesSyncBehavior(t *testing.T) {
	// placeDefault writing nothing is only correct if sync then puts the
	// block where the offer promised. Prove the promise rather than assuming
	// it: a file left untouched must still get its block after the title.
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"CLAUDE.md": "# P\n\nrules\n"})
	out, err := render.Splice([]byte("# P\n\nrules\n"), "body\n", render.BlockMeta{Packs: []string{"p@1"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(out), "# P\n\n<!-- escapement:begin ") {
		t.Errorf("offer copy promises the block lands after the title, got:\n%s", out)
	}
	_ = root
}
```

`placeAbove` writes at byte 0 and needs no parsing. `placeDefault` writes nothing. So this task needs NO copy of the frontmatter-and-H1 logic, which is the point of collapsing the option set: there is now exactly one implementation of "where does the top of this file begin," and it stays in `render.Splice`.

Also write CLI integration tests, using the `newGoverned`/`run` helpers in `internal/cli/cli_test.go`:

- Non-TTY `esc init` in a repo with a CLAUDE.md: detection printed, file byte-identical afterwards, exit 0.
- Injected answers `a` and `e` each write the placeholder at the expected offset.
- Injected `k`, an empty answer, and an unrecognized answer all write nothing.
- `--yes` with no TTY writes nothing, because the recommended choice is `placeDefault`. Assert this explicitly: `--yes` accepting a no-op is correct here and is the kind of thing a later reader will "fix" into a file write if no test pins it.
- A file with uncommitted changes is skipped with a stated reason even when the answer selects `a` or `e`, and the other detected files are still processed.
- A file already carrying a placeholder or a managed block gets no offer and is not written.
- After an `a` or `e` answer, `esc sync` fills that placeholder rather than applying top placement.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run "TestApplyPlacement|TestInitOffer" -v`
Expected: FAIL, undefined: `applyPlacement`, `placement`, `placeTop`.

- [ ] **Step 3: Implement the offer**

Create `internal/cli/initoffer.go`. The seam mirrors `maybeUpdates`:

```go
// initOffer is the placement-prompt seam: production reads a real TTY, tests
// inject a deterministic reader so no test can ever block on real stdin. The
// same pattern as maybeUpdates.
var initOffer = func(w io.Writer, in io.Reader, interactive bool, d Detected) placement {
	if !interactive {
		return placeDefault
	}
	fmt.Fprintf(w, "Where should the managed block go in %s?\n", d.Path)
	fmt.Fprintln(w, "  [k] keep default: top of the file, below any title (recommended)")
	fmt.Fprintln(w, "  [a] above the title, the very first thing in the file")
	fmt.Fprintln(w, "  [e] end of the file")
	// read one line; empty or unrecognized input returns placeDefault
}
```

`applyPlacement` writes `render.Placeholder` at the chosen offset using the existing atomic-write discipline, preserving every original byte. It writes nothing for `placeDefault`.

`fileIsClean` runs `git status --porcelain -- <path>` with a small unexported runner in `internal/cli`. Do NOT export `source.git`. A file with output is dirty. A repo that is not a git repo at all should be treated as clean rather than erroring, since `esc init` must work before the first commit; state that choice in a comment.

- [ ] **Step 4: Wire into `cmdInit`**

Order matters and must be deliberate: detect, write config scaffolding, print the explanation, THEN offer. The offer comes last so a user who declines everything still has a working config.

Guards, all of which the tests above assert:
- Non-interactive runs never modify an existing file. Note `--yes` selects `placeDefault`, which writes nothing, so `--yes` cannot modify a file either; it exists so a scripted run does not stall waiting for input.
- Empty or unrecognized input returns `placeDefault`.
- A file with uncommitted changes is reported and skipped, with the reason given, because git is the undo mechanism for anything init writes.
- A file that already has a block or a placeholder is never offered.

- [ ] **Step 5: Run to verify pass**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/cli/
git commit -m "feat(cli): offer to place the managed-block marker at init

Init writes only the placeholder, never rendered policy, so a
getting-started command cannot put rules a user has not seen into their
instruction files. Declining leaves the repo exactly as detection found
it and top placement still applies at first sync. Skips any file with
uncommitted changes, since git is the undo for anything init writes."
```

---

### Task 3: Reconciliation ships as a pack skill

Reconciling a team's existing rules against a pack's is semantic work. In the CLI it would mean either a heuristic wrong often enough to erode trust in a tool whose value is being trustworthy, or a model call, which breaks the single external dependency policy, the no-network posture, and "boring and fast" at once. It ships as a skill an agent executes, delivered through the same versioned, signed channel as every other rule.

**Files:**
- Modify: `internal/portal/seed/repos.go`
- Test: `internal/portal/seed/repos_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks
- Produces: the seeded demo pack gains `skills/esc-reconcile/SKILL.md` and a `skills:` manifest entry

- [ ] **Step 1: Write the failing test**

```go
func TestSeededPackShipsReconcileSkill(t *testing.T) {
	dir := t.TempDir()
	// follow the existing seed test's setup for producing a pack repo
	raw, err := os.ReadFile(filepath.Join(dir, "skills", "esc-reconcile", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, want := range []string{"escapement:begin", "escapement:end", "never edit"} {
		if !strings.Contains(body, want) {
			t.Errorf("skill must mention %q:\n%s", want, body)
		}
	}
	manifest, err := os.ReadFile(filepath.Join(dir, "pack.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifest), "skills/esc-reconcile") {
		t.Errorf("manifest must declare the skill:\n%s", manifest)
	}
}
```

Read `internal/portal/seed/repos_test.go` first and match how it already builds and inspects the seeded pack.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/portal/seed/ -run TestSeededPackShipsReconcileSkill -v`
Expected: FAIL, no such file

- [ ] **Step 3: Write the skill**

Add to `packYAML`'s `skills:` list and add the file content. The skill instructs an agent to:

- Read the managed block and the surrounding content in the same file.
- Report surrounding rules that duplicate a pack rule, and surrounding rules that contradict one.
- Propose edits to the human-authored sections only.
- **Never edit inside the `escapement:begin` / `escapement:end` markers.** Edits there produce the `altered` state, which stops that artifact from receiving policy updates until a human resolves it.

That last constraint is the useful interlock: the amendment model gives the skill a precise, mechanically checked boundary, and an agent that ignores it produces a state the portal surfaces rather than a silent corruption. Say that in the skill text, because an agent reading it benefits from knowing why.

Keep the skill under roughly 40 lines. No em-dashes.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: PASS. The demo portal's seeded repos now sync one extra skill; if any seed or portal test asserts a skill count, update it deliberately.

- [ ] **Step 5: Commit**

```bash
git add internal/portal/seed/
git commit -m "feat(seed): ship a reconcile skill in the demo pack

Comparing a team's existing rules against a pack's is judgment work and
belongs in an agent, not in a deterministic CLI. Delivering it as a pack
skill means the org controls it by publishing a pack version, the same
versioned channel as every other rule."
```

---

### Task 4: Docs

**Files:**
- Modify: `README.md`, `AGENTS.md`, `ai-governance-product-spec.md`, `CHANGELOG.md`

- [ ] **Step 1: README**

Add the "adding esc to a repo that already has instruction files" path: what `esc init` detects, what it pre-fills, what it offers, that declining changes nothing, and that the placeholder is the override. Note that init never writes rendered policy, only the marker.

- [ ] **Step 2: AGENTS.md**

One bullet under "Gotchas and invariants": `esc init` detects existing instruction files and may write the placeholder marker, but never rendered policy and never to a file with uncommitted changes.

- [ ] **Step 3: Product spec**

Record that reconciliation is agent-executed and pack-delivered rather than a CLI feature, with the determinism rationale. Match the surrounding section's formatting; check the current numbering rather than trusting any number quoted in the spec doc, since it has drifted before.

- [ ] **Step 4: CHANGELOG**

```markdown
- `esc init` detects instruction files a repo already has, pre-fills `targets` from them, explains what the first sync will do, and offers to place the managed-block marker. It never writes rendered policy and never touches a file with uncommitted changes.
- The seeded demo pack ships an `esc-reconcile` skill: guidance for an agent comparing a team's existing rules against the pack's.
```

- [ ] **Step 5: Verify and commit**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: PASS

Run: `git diff --cached | grep "^+" | grep "—"`
Expected: no output

```bash
git add README.md AGENTS.md ai-governance-product-spec.md CHANGELOG.md
git commit -m "docs: onboarding into repos with existing instruction files"
```

---

## Verification Before Completion

```bash
go test ./... && go vet ./... && gofmt -l .
```

| Spec section | Task |
|---|---|
| §1 top placement | Already shipped in `fc70378`, not in this plan |
| §2 detect, explain, offer | 1, 2 |
| §3 reconcile as a pack skill | 3 |
| §4 testing | 1, 2, 3 |
| §6 docs | 4 |
