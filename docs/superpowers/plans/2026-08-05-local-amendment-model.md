# Local Amendment Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `esc` aware of content it does not own: preserve it everywhere, report it as a local amendment, and stop overwriting hand-edits inside managed regions.

**Architecture:** Every artifact gains two orthogonal classifications, `managed` (content we own) and `local` (content we do not). `render` gains byte-level primitives that isolate unmanaged content; `engine` classifies and reports it. `Apply` skips artifacts whose managed region was hand-edited rather than overwriting them, and skill directories gain a per-file manifest in the lockfile so added files survive and dropped files are still removed.

**Tech Stack:** Go 1.24, stdlib plus `gopkg.in/yaml.v3`. No new dependencies.

**Source specs:** `docs/superpowers/specs/2026-08-05-local-amendment-model-design.md` (all sections) and `docs/superpowers/specs/2026-08-05-onboarding-existing-repos-design.md` §1 only, folded in here per that spec's §5 so the renderer goldens are rewritten once.

## Global Constraints

- Single external dependency policy: `gopkg.in/yaml.v3` only. Everything else stdlib, system `git` via `os/exec`. No new dependencies in this plan.
- Renderer invariants, all golden-testable: bytes outside a managed block are never modified; nothing is written after a verification or constraint failure; all writes are atomic; renderer output is deterministic.
- Sentinel errors in `internal/esc` map to exit codes: 0 ok, 1 drift/constraint, 2 usage, 3 integrity/signature, 4 other. New failure modes go through them.
- `go test ./...`, `go vet ./...`, and `gofmt -w .` must pass before any task is considered done.
- Integration tests build real temp git repos, no mocks. `ESC_CACHE_DIR` overrides the pack cache.
- No em-dashes in any user-facing copy or docs.
- There are no existing users. No backward-compatibility aliases are kept for renamed identifiers.

## File Structure

| File | Responsibility | Change |
|---|---|---|
| `internal/render/block.go` | Managed block markers, splice, extract | Modify: top placement, `BlockSurround` |
| `internal/render/mcp.go` | Owned MCP key merge and hash | Modify: `UnownedMCPServers` |
| `internal/engine/status.go` | Classification and findings | Modify: two axes, amendment population |
| `internal/engine/amendment.go` | Amendment and Alteration types, construction helpers | Create |
| `internal/engine/reporting.go` | Reporting level resolution | Create |
| `internal/engine/report.go` | JSON report document assembly | Create |
| `internal/engine/apply.go` | Writes | Modify: skip pass, dir merge, `SyncResult` |
| `internal/lockfile/lockfile.go` | Lock schema | Modify: `Files` field |
| `internal/pack/pack.go` | Manifest schema | Modify: `Reporting` |
| `internal/config/config.go` | Repo config schema | Modify: `ReportAmendments` |
| `internal/cli/cli.go` | Command surface and output | Modify: `--json`, output copy |

`amendment.go`, `reporting.go`, and `report.go` are new files rather than additions to `status.go` (209 lines) or `engine.go` (397 lines), keeping each focused on one responsibility.

## Existing test helpers

`internal/cli/cli_test.go` already provides these. Use them; do not write parallel versions.

| Helper | Signature |
|---|---|
| `gitIn` | `(t *testing.T, dir string, args ...string) string` |
| `writeFiles` | `(t *testing.T, dir string, files map[string]string)` |
| `packRepoFiles` | `(version string) map[string]string` |
| `newPackRepo` | `(t *testing.T, version string) string` |
| `newGoverned` | `(t *testing.T, packRepo, ref string) string` |
| `run` | `(t *testing.T, root string, args ...string) (int, string)` — note: exit code first |

Task 0 adds the thin fixture wrappers the later tasks' tests call.

---

### Task 0: Test fixtures

The tests in Tasks 4 through 10 need governed repos with skills and with a `reporting` block, plus a call wrapper with a friendlier return order. Building them once here keeps every later task's test code short and identical in shape.

**Files:**
- Create: `internal/cli/fixtures_test.go`

**Interfaces:**
- Produces:
  - `setupGovernedRepo(t *testing.T) string`
  - `setupGovernedRepoWithSkills(t *testing.T) string`
  - `setupGovernedRepoWithReporting(t *testing.T, level string) string`
  - `runEsc(t *testing.T, root string, args ...string)` — fails the test on a non-zero exit
  - `runEscOut(t *testing.T, root string, args ...string) (string, int)`
  - `dropSkillFileFromPack(t *testing.T, packRepo, rel string)`

- [ ] **Step 1: Write the fixtures**

Create `internal/cli/fixtures_test.go`:

```go
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runEsc runs a command and fails the test unless it exits 0.
func runEsc(t *testing.T, root string, args ...string) {
	t.Helper()
	code, out := run(t, root, args...)
	if code != 0 {
		t.Fatalf("esc %s exited %d:\n%s", strings.Join(args, " "), code, out)
	}
}

// runEscOut returns output first, matching how the assertions below read.
func runEscOut(t *testing.T, root string, args ...string) (string, int) {
	t.Helper()
	code, out := run(t, root, args...)
	return out, code
}

func setupGovernedRepo(t *testing.T) string {
	t.Helper()
	return newGoverned(t, newPackRepo(t, "1.0.0"), "v1.0.0")
}

// skillFiles is the pack-side fixture for a skill directory. Kept separate so
// dropSkillFileFromPack can publish a version without one of these entries.
func skillFiles() map[string]string {
	return map[string]string{
		"skills/esc-security/SKILL.md":  "---\nname: esc-security\ndescription: demo\n---\n\nRules.\n",
		"skills/esc-security/EXTRA.md":  "Extra pack content.\n",
	}
}

func setupGovernedRepoWithSkills(t *testing.T) string {
	t.Helper()
	packRepo := newPackRepo(t, "1.0.0")
	files := skillFiles()
	files["pack.yaml"] = withManifestLines(t, packRepo, "skills:\n  - skills/esc-security\n")
	writeFiles(t, packRepo, files)
	gitIn(t, packRepo, "add", ".")
	gitIn(t, packRepo, "commit", "-m", "add skills")
	gitIn(t, packRepo, "tag", "-f", "v1.0.0")
	return newGoverned(t, packRepo, "v1.0.0")
}

func setupGovernedRepoWithReporting(t *testing.T, level string) string {
	t.Helper()
	packRepo := newPackRepo(t, "1.0.0")
	writeFiles(t, packRepo, map[string]string{
		"pack.yaml": withManifestLines(t, packRepo, "reporting:\n  amendments: "+level+"\n"),
	})
	gitIn(t, packRepo, "add", ".")
	gitIn(t, packRepo, "commit", "-m", "declare reporting")
	gitIn(t, packRepo, "tag", "-f", "v1.0.0")
	return newGoverned(t, packRepo, "v1.0.0")
}

// withManifestLines appends lines to the pack manifest already on disk.
func withManifestLines(t *testing.T, packRepo, extra string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(packRepo, "pack.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s + extra
}

// dropSkillFileFromPack publishes a new pack version without rel, retagging
// so the governed repo picks it up on the next sync.
func dropSkillFileFromPack(t *testing.T, packRepo, rel string) {
	t.Helper()
	if err := os.Remove(filepath.Join(packRepo, filepath.FromSlash(rel))); err != nil {
		t.Fatal(err)
	}
	gitIn(t, packRepo, "add", "-A")
	gitIn(t, packRepo, "commit", "-m", "drop "+rel)
	gitIn(t, packRepo, "tag", "-f", "v1.0.0")
}
```

- [ ] **Step 2: Verify the fixtures build and produce working repos**

Add a smoke test asserting `setupGovernedRepoWithSkills` yields a repo where `esc sync` creates `.claude/skills/esc-security/SKILL.md`:

```go
func TestFixtureSkillsRepoSyncs(t *testing.T) {
	repo := setupGovernedRepoWithSkills(t)
	runEsc(t, repo, "sync")
	if _, err := os.Stat(filepath.Join(repo, ".claude", "skills", "esc-security", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}
```

Run: `go test ./internal/cli/ -run TestFixture -v`
Expected: PASS. If the skills path differs from the above, correct the fixture to match what `render.TargetSkills` actually produces rather than changing the renderer.

Note: `dropSkillFileFromPack` takes the **pack** repo path and a **pack-relative** path, not the governed repo. Task 5's second test builds its repo inline for exactly this reason, since `setupGovernedRepoWithSkills` returns only the governed repo.

- [ ] **Step 3: Commit**

```bash
git add internal/cli/fixtures_test.go
git commit -m "test(cli): fixtures for skills and reporting repos"
```

---

### Task 1: Rename `Modified` to `Altered`

Mechanical, and first so every later task uses final names.

**Files:**
- Modify: `internal/engine/status.go:23`, and all references across `internal/engine` and `internal/cli`

**Interfaces:**
- Produces: `engine.Altered State` with string value `"altered"`. `engine.Modified` no longer exists.

- [ ] **Step 1: Rename the constant and its string value**

In `internal/engine/status.go`:

```go
const (
	InSync             State = "in-sync"
	Altered            State = "altered"
	Missing            State = "missing"
	Stale              State = "stale"
	ConstraintViolated State = "constraint-violated"
	PackStale          State = "pack-stale"
	CheckOverdue       State = "check-overdue"
	Orphan             State = "orphan"
)
```

- [ ] **Step 2: Update every reference**

Run: `grep -rn "engine.Modified\|Modified\b" internal/ --include=*.go`

Replace each with `Altered`. Expect hits in `status.go` (the `staleOrModified` helper and each `classify` branch) and in tests. Rename the helper `staleOrModified` to `staleOrAltered`.

- [ ] **Step 3: Run the full suite**

Run: `go build ./... && go test ./...`
Expected: PASS. Any test asserting the literal string `"modified"` is updated to `"altered"` as part of this step.

- [ ] **Step 4: Commit**

```bash
git add internal/
git commit -m "refactor(engine): rename Modified state to Altered

Matches the vocabulary used in status output and the portal. No users
exist, so no alias is retained."
```

---

### Task 2: Top placement for new managed blocks

Implements onboarding spec §1.

**Files:**
- Modify: `internal/render/block.go:44-65`
- Test: `internal/render/block_test.go`, `internal/render/testdata/*.golden.md`

**Interfaces:**
- Produces: `render.Splice` unchanged in signature; new blocks insert at top instead of appending. Unexported `insertAt(s string) int`.

- [ ] **Step 1: Write the failing tests**

Add to `internal/render/block_test.go`:

```go
func TestSpliceTopPlacement(t *testing.T) {
	meta := BlockMeta{Packs: []string{"p@1"}}
	cases := []struct {
		name, existing, wantPrefix string
	}{
		{"plain", "team rules\n", "<!-- escapement:begin "},
		{"h1", "# Project\n\nteam rules\n", "# Project\n\n<!-- escapement:begin "},
		{"h1 no blank", "# Project\nteam rules\n", "# Project\n<!-- escapement:begin "},
		{"frontmatter", "---\ntitle: x\n---\nteam rules\n", "---\ntitle: x\n---\n<!-- escapement:begin "},
		{"frontmatter and h1", "---\ntitle: x\n---\n# P\n\nteam\n", "---\ntitle: x\n---\n# P\n\n<!-- escapement:begin "},
		{"hashtag not h1", "#hashtag\n", "<!-- escapement:begin "},
		{"subheading not h1", "## Sub\n", "<!-- escapement:begin "},
		{"unterminated frontmatter", "---\ntitle: x\n", "<!-- escapement:begin "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Splice([]byte(tc.existing), "body\n", meta)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(out), tc.wantPrefix) {
				t.Errorf("got:\n%s\nwant prefix:\n%s", out, tc.wantPrefix)
			}
			// Every original byte survives, in order.
			if !strings.Contains(string(out), strings.TrimPrefix(tc.existing, tc.wantPrefix)) &&
				!strings.Contains(string(out), "team") && tc.existing != "" {
				t.Errorf("original content lost:\n%s", out)
			}
		})
	}
}

func TestSpliceExistingBlockDoesNotMove(t *testing.T) {
	meta := BlockMeta{Packs: []string{"p@1"}}
	first, err := Splice([]byte("# P\n\nteam rules\n"), "body\n", meta)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a block that lives at the bottom already.
	bottom := "# P\n\nteam rules\n\n" + string(first[strings.Index(string(first), "<!-- escapement:begin "):])
	out, err := Splice([]byte(bottom), "body2\n", meta)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(out), "# P\n\nteam rules\n") {
		t.Errorf("existing block moved:\n%s", out)
	}
}

func TestSplicePlaceholderWinsOverTopPlacement(t *testing.T) {
	meta := BlockMeta{Packs: []string{"p@1"}}
	out, err := Splice([]byte("# P\n\nteam\n"+Placeholder+"\n"), "body\n", meta)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(out), "# P\n\nteam\n") {
		t.Errorf("placeholder ignored:\n%s", out)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/render/ -run TestSplice -v`
Expected: FAIL. `TestSpliceTopPlacement` fails on every case because the block is currently appended at the end.

- [ ] **Step 3: Implement**

Add to `internal/render/block.go`:

```go
// insertAt returns the offset where a new block belongs: the top of the file,
// after leading YAML frontmatter and a leading H1 if either is present. Only
// those two constructs are skipped; the rule set is closed deliberately so
// output stays predictable.
func insertAt(s string) int {
	i := 0
	if strings.HasPrefix(s, "---\n") {
		if end := strings.Index(s[4:], "\n---"); end >= 0 {
			after := 4 + end + len("\n---")
			rest := s[after:]
			if rest == "" || strings.HasPrefix(rest, "\n") {
				i = after
				if strings.HasPrefix(s[i:], "\n") {
					i++
				}
			}
		}
	}
	if strings.HasPrefix(s[i:], "# ") {
		if nl := strings.IndexByte(s[i:], '\n'); nl >= 0 {
			i += nl + 1
			if strings.HasPrefix(s[i:], "\n") {
				i++
			}
		} else {
			i = len(s)
		}
	}
	return i
}
```

Replace the final branch of `Splice` (currently `internal/render/block.go:60-64`):

```go
	at := insertAt(s)
	if at >= len(s) {
		sep := "\n"
		if strings.HasSuffix(s, "\n") || s == "" {
			sep = ""
		}
		return []byte(s + sep + block), nil
	}
	return []byte(s[:at] + block + "\n" + s[at:]), nil
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/render/ -run TestSplice -v`
Expected: PASS

- [ ] **Step 5: Regenerate renderer goldens**

Run: `go test ./internal/render/ ./internal/cli/ ./internal/engine/`
Expected: FAIL on golden comparisons. Inspect each diff and confirm the only change is block position. Regenerate per the repo's existing golden convention (check `internal/render/golden_test.go` for an update flag; if none exists, edit `internal/render/testdata/*.golden.md` by hand).

Run again: `go test ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/render/
git commit -m "feat(render): insert new managed blocks at the top of a file

Org policy should anchor an instruction file rather than trail whatever
the team already wrote. Frontmatter and a leading H1 are skipped: the
first for correctness, since content above a --- fence stops it being
frontmatter, the second because a policy block above a document title
reads as damage. Existing blocks are still replaced in place and never
move; the placeholder remains the explicit override."
```

---

### Task 3: Unmanaged content primitives in `render`

**Files:**
- Modify: `internal/render/block.go`, `internal/render/mcp.go`
- Test: `internal/render/block_test.go`, `internal/render/mcp_test.go`

**Interfaces:**
- Produces:
  - `render.BlockSurround(file []byte) (string, error)` returns the bytes outside the managed block, before and after concatenated. Returns `""` when there is no block or nothing outside it. Errors only on a corrupt block, matching `Extract`.
  - `render.UnownedMCPServers(file []byte, owned []string) ([]string, error)` returns sorted names of server entries not in `owned`.

- [ ] **Step 1: Write the failing tests**

Add to `internal/render/block_test.go`:

```go
func TestBlockSurround(t *testing.T) {
	meta := BlockMeta{Packs: []string{"p@1"}}
	withBlock, err := Splice([]byte("# P\n\nteam rules\n"), "body\n", meta)
	if err != nil {
		t.Fatal(err)
	}
	got, err := BlockSurround(withBlock)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "team rules") || strings.Contains(got, "escapement:begin") {
		t.Errorf("surround = %q", got)
	}

	only, err := Splice(nil, "body\n", meta)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := BlockSurround(only); err != nil || got != "" {
		t.Errorf("block-only file: got %q, err %v; want empty", got, err)
	}

	if got, err := BlockSurround([]byte("no block here\n")); err != nil || got != "" {
		t.Errorf("no block: got %q, err %v; want empty", got, err)
	}
}
```

Add to `internal/render/mcp_test.go`:

```go
func TestUnownedMCPServers(t *testing.T) {
	raw := []byte(`{"mcpServers":{"ours":{"command":"a"},"theirs":{"command":"b"},"also":{"command":"c"}}}`)
	got, err := UnownedMCPServers(raw, []string{"ours"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"also", "theirs"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
	if got, err := UnownedMCPServers(nil, nil); err != nil || len(got) != 0 {
		t.Errorf("empty file: got %v err %v", got, err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/render/ -run "TestBlockSurround|TestUnownedMCPServers" -v`
Expected: FAIL, undefined: `BlockSurround`, `UnownedMCPServers`

- [ ] **Step 3: Implement**

Add to `internal/render/block.go`:

```go
// BlockSurround returns the content of file outside its managed block. It is
// empty when the file has no block, or when nothing but whitespace surrounds
// one. Errors match Extract: only a corrupt block structure fails.
func BlockSurround(file []byte) (string, error) {
	b, err := Extract(file)
	if err != nil {
		return "", err
	}
	if b == nil {
		return "", nil
	}
	s := string(file)
	out := s[:b.start] + s[b.end:]
	if strings.TrimSpace(out) == "" {
		return "", nil
	}
	return out, nil
}
```

Add to `internal/render/mcp.go` (reusing whatever parse helper `OwnedMCPHash` already uses; if it unmarshals into a local type, mirror that here rather than introducing a second shape):

```go
// UnownedMCPServers returns the sorted names of server entries in file that
// are not in owned. A missing or empty file has none.
func UnownedMCPServers(file []byte, owned []string) ([]string, error) {
	if len(bytes.TrimSpace(file)) == 0 {
		return nil, nil
	}
	var doc struct {
		Servers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(file, &doc); err != nil {
		return nil, err
	}
	ownedSet := make(map[string]bool, len(owned))
	for _, k := range owned {
		ownedSet[k] = true
	}
	var out []string
	for name := range doc.Servers {
		if !ownedSet[name] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, nil
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/render/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/render/
git commit -m "feat(render): expose unmanaged content in blocks and mcp config

Byte-level primitives only; classification lives in engine."
```

---

### Task 4: Amendment types and two-axis findings for block and json-keys

**Files:**
- Create: `internal/engine/amendment.go`
- Modify: `internal/engine/status.go:31-36` (`Finding`), `internal/engine/status.go:144-209` (`classify`)
- Test: `internal/engine/amendment_test.go`, `internal/cli/` integration tests

**Interfaces:**
- Consumes: `render.BlockSurround`, `render.UnownedMCPServers` (Task 3); `engine.Altered` (Task 1)
- Produces:
  - `engine.LocalState string` with `LocalNone = "none"`, `LocalAmended = "amended"`
  - `engine.Amendment{Bytes, Lines int; Hash string; Items []string; Content string}`
  - `engine.Alteration{ExpectedHash, ActualHash, Diff string}`
  - `engine.newAmendment(content string, items []string) *Amendment` returns nil when there is nothing unmanaged
  - `Finding` gains `Kind string`, `Local LocalState`, `Amendment *Amendment`, `Alteration *Alteration`

- [ ] **Step 1: Write the failing test**

Create `internal/engine/amendment_test.go`:

```go
package engine

import "testing"

func TestNewAmendment(t *testing.T) {
	if got := newAmendment("", nil); got != nil {
		t.Errorf("empty content must yield nil, got %+v", got)
	}
	if got := newAmendment("   \n\n", nil); got != nil {
		t.Errorf("whitespace-only content must yield nil, got %+v", got)
	}
	got := newAmendment("one\ntwo\n", nil)
	if got == nil || got.Lines != 2 || got.Bytes != 8 {
		t.Fatalf("got %+v, want 2 lines / 8 bytes", got)
	}
	if got.Hash == "" {
		t.Error("hash must be set")
	}
	if got.Content != "one\ntwo\n" {
		t.Errorf("content = %q", got.Content)
	}
	items := newAmendment("", []string{"theirs"})
	if items == nil || len(items.Items) != 1 {
		t.Fatalf("items-only amendment must be non-nil, got %+v", items)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/engine/ -run TestNewAmendment -v`
Expected: FAIL, undefined: `newAmendment`

- [ ] **Step 3: Implement the types**

Create `internal/engine/amendment.go`:

```go
package engine

import (
	"strings"

	"github.com/tensorgroup/openescapement/internal/esc"
)

// LocalState classifies content escapement does not own that shares an
// artifact with content it does. It is orthogonal to State: a file can carry
// both a local amendment and an altered managed region, and collapsing the
// two would force dropping one of those facts.
type LocalState string

const (
	LocalNone    LocalState = "none"
	LocalAmended LocalState = "amended"
)

// Amendment describes unmanaged content sharing an artifact with managed
// content. Content carries surrounding text (kind=block); Items carries
// discrete unmanaged names: file paths for kind=dir, server names for
// kind=json-keys.
type Amendment struct {
	Bytes   int      `json:"bytes"`
	Lines   int      `json:"lines"`
	Hash    string   `json:"hash"`
	Items   []string `json:"items,omitempty"`
	Content string   `json:"content,omitempty"`
}

// Alteration describes a hand-edited managed region. Diff is populated only
// by the JSON report surfaces, which can afford the git shell-out; human
// output points at `esc diff` instead.
type Alteration struct {
	ExpectedHash string `json:"expected_hash"`
	ActualHash   string `json:"actual_hash"`
	Diff         string `json:"diff,omitempty"`
}

// newAmendment returns nil when there is nothing unmanaged to report.
func newAmendment(content string, items []string) *Amendment {
	if strings.TrimSpace(content) == "" && len(items) == 0 {
		return nil
	}
	a := &Amendment{
		Bytes: len(content),
		Lines: strings.Count(content, "\n"),
		Items: items,
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		a.Lines++
	}
	a.Hash = esc.HashBytes([]byte(content))
	a.Content = content
	return a
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/engine/ -run TestNewAmendment -v`
Expected: PASS

- [ ] **Step 5: Extend `Finding` and populate it in `classify`**

In `internal/engine/status.go`, replace the `Finding` struct:

```go
// Finding is one classified artifact (or pack pin) in a status report.
type Finding struct {
	Path       string      `json:"path"`
	Kind       string      `json:"kind,omitempty"`
	State      State       `json:"managed"`
	Local      LocalState  `json:"local"`
	Detail     string      `json:"detail,omitempty"`
	Amendment  *Amendment  `json:"amendment,omitempty"`
	Alteration *Alteration `json:"alteration,omitempty"`
}
```

Every existing positional literal `Finding{a.Path, Stale, "..."}` becomes a keyed literal. Update each one in `status.go` and anywhere else `grep -rn "Finding{" internal/` reports.

In `classify`, set `Kind: a.Kind` and `Local: LocalNone` as the default, then populate per kind:

```go
	case KindBlock:
		// ... existing read and Extract ...
		f := Finding{Path: a.Path, Kind: a.Kind, Local: LocalNone}
		if surround, serr := render.BlockSurround(content); serr == nil {
			if am := newAmendment(surround, nil); am != nil {
				f.Local, f.Amendment = LocalAmended, am
			}
		}
		actual := render.BodyHash(block.Body)
		if actual == a.Hash {
			f.State = InSync
			return f
		}
		f.State, f.Detail = staleOrAltered(actual, "managed block was hand-edited (hash mismatch)")
		if f.State == Altered {
			f.Alteration = &Alteration{ExpectedHash: a.Hash, ActualHash: actual}
		}
		return f
```

Refactor `staleOrAltered` to return `(State, string)` rather than a whole `Finding`, so each branch can attach its own axes.

For `KindJSONKeys`, populate the same way using `render.UnownedMCPServers(content, a.Keys)` and `newAmendment("", names)`.

For `KindFile`, `Local` is always `LocalNone`; escapement owns the whole file.

- [ ] **Step 6: Write the integration test**

Add to the CLI integration tests (follow the existing temp-repo helper in `internal/cli/cli_test.go`):

```go
func TestAmendedBlockReportsInSync(t *testing.T) {
	repo := setupGovernedRepo(t) // Task 0
	runEsc(t, repo, "sync")

	path := filepath.Join(repo, "AGENTS.md")
	existing, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(existing, []byte("\n## Team rules\n\nBe kind.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := engine.Status(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, f := range st.Findings {
		if f.Path != "AGENTS.md" {
			continue
		}
		found = true
		if f.State != engine.InSync {
			t.Errorf("managed axis = %q, want in-sync", f.State)
		}
		if f.Local != engine.LocalAmended {
			t.Errorf("local axis = %q, want amended", f.Local)
		}
		if f.Amendment == nil || !strings.Contains(f.Amendment.Content, "Be kind.") {
			t.Errorf("amendment = %+v", f.Amendment)
		}
	}
	if !found {
		t.Fatal("no finding for AGENTS.md")
	}
	if !st.Clean() {
		t.Error("an amendment alone must not make status unclean")
	}
}
```

Note `Clean()` must keep returning true: it iterates `f.State`, and `Local` is deliberately not consulted. Amendment is expected behavior, not drift.

- [ ] **Step 7: Run to verify pass**

Run: `go test ./internal/engine/ ./internal/cli/ -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/engine/ internal/cli/
git commit -m "feat(engine): report local amendments alongside managed state

Two orthogonal axes per artifact. A team that both appended its own
rules and carries an inadvertent edit inside ours produces both facts;
one enum would force dropping one. An amendment alone is never drift."
```

---

### Task 5: Directory manifests preserve added files

Fixes the data-loss defect: `stageDir` currently removes the destination tree wholesale (`internal/engine/apply.go:229`).

**Files:**
- Modify: `internal/lockfile/lockfile.go:25-30`, `internal/engine/apply.go:92-96` and `internal/engine/apply.go:213-233`
- Test: `internal/cli/` integration tests

**Interfaces:**
- Consumes: nothing from prior tasks
- Produces:
  - `lockfile.LockArtifact.Files []string` — pack-relative paths written by the last sync
  - `engine.mergeDir(src, dst string, prevFiles []string) (written []string, err error)` replaces `stageDir`

- [ ] **Step 1: Write the failing test**

```go
func TestSkillDirPreservesAddedFiles(t *testing.T) {
	repo := setupGovernedRepoWithSkills(t) // Task 0
	runEsc(t, repo, "sync")

	dir := filepath.Join(repo, ".claude", "skills", "esc-security")
	added := filepath.Join(dir, "team-notes.md")
	if err := os.WriteFile(added, []byte("our own notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	runEsc(t, repo, "sync")

	got, err := os.ReadFile(added)
	if err != nil {
		t.Fatalf("added file destroyed by sync: %v", err)
	}
	if string(got) != "our own notes\n" {
		t.Errorf("added file mutated: %q", got)
	}
}

func TestSkillDirRemovesPackDroppedFiles(t *testing.T) {
	// Built inline rather than via setupGovernedRepoWithSkills, because this
	// test needs the pack repo path in order to publish a version without
	// EXTRA.md. Mirrors that fixture exactly otherwise.
	packRepo := newPackRepo(t, "1.0.0")
	files := skillFiles()
	files["pack.yaml"] = withManifestLines(t, packRepo, "skills:\n  - skills/esc-security\n")
	writeFiles(t, packRepo, files)
	gitIn(t, packRepo, "add", ".")
	gitIn(t, packRepo, "commit", "-m", "add skills")
	gitIn(t, packRepo, "tag", "-f", "v1.0.0")
	repo := newGoverned(t, packRepo, "v1.0.0")

	runEsc(t, repo, "sync")
	dropped := filepath.Join(repo, ".claude", "skills", "esc-security", "EXTRA.md")
	if _, err := os.Stat(dropped); err != nil {
		t.Fatalf("precondition: %v", err)
	}

	dropSkillFileFromPack(t, packRepo, "skills/esc-security/EXTRA.md")
	runEsc(t, repo, "sync")

	if _, err := os.Stat(dropped); !os.IsNotExist(err) {
		t.Error("a file the pack dropped must be removed, not kept as an amendment")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run TestSkillDir -v`
Expected: FAIL. `TestSkillDirPreservesAddedFiles` fails with a missing-file error, because `stageDir` deleted it.

- [ ] **Step 3: Add the lockfile field**

In `internal/lockfile/lockfile.go`:

```go
type LockArtifact struct {
	Path  string   `json:"path"`
	Kind  string   `json:"kind"` // block | file | dir | json-keys
	Hash  string   `json:"hash"`
	Keys  []string `json:"keys,omitempty"`  // owned keys for json-keys artifacts
	Files []string `json:"files,omitempty"` // pack-relative paths written for dir artifacts
}
```

`Files` is sorted before save so the lockfile stays deterministic; add the sort alongside the existing artifact sort in `Save`.

- [ ] **Step 4: Replace `stageDir` with `mergeDir`**

In `internal/engine/apply.go`, replace `stageDir` (lines 213-233):

```go
// mergeDir reconciles dst against the pack tree at src. Files the pack
// provides are written; files the previous manifest recorded but the pack no
// longer provides are removed; anything else on disk is left alone as a local
// amendment. Returns the pack-relative paths written, for the manifest.
//
// A pre-manifest lockfile has no prevFiles, so nothing unknown is removed on
// the first sync after upgrade. That direction can leave one pack-dropped
// file behind, reported as an amendment, rather than deleting a team's work.
func mergeDir(src, dst string, prevFiles []string) ([]string, error) {
	staged, err := os.MkdirTemp(filepath.Dir(dst), ".esc-stage-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(staged)
	tree := filepath.Join(staged, filepath.Base(dst))
	if err := copyDir(src, tree); err != nil {
		return nil, err
	}

	var written []string
	if err := filepath.WalkDir(tree, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(tree, p)
		if err != nil {
			return err
		}
		written = append(written, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(written)

	if err := os.MkdirAll(dst, 0o755); err != nil {
		return nil, err
	}
	// Remove what the pack dropped, before writing what it provides.
	nowProvided := make(map[string]bool, len(written))
	for _, f := range written {
		nowProvided[f] = true
	}
	for _, prev := range prevFiles {
		if nowProvided[prev] {
			continue
		}
		if err := os.Remove(filepath.Join(dst, filepath.FromSlash(prev))); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	for _, f := range written {
		from := filepath.Join(tree, filepath.FromSlash(f))
		to := filepath.Join(dst, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return nil, err
		}
		content, err := os.ReadFile(from)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(from)
		if err != nil {
			return nil, err
		}
		if err := atomicWrite(to, content); err != nil {
			return nil, err
		}
		if err := os.Chmod(to, info.Mode().Perm()); err != nil {
			return nil, err
		}
	}
	return written, nil
}
```

Note the atomicity property changes shape: the whole tree is no longer swapped in one rename, because a wholesale swap is exactly what destroys unmanaged files. Each file is written atomically instead, and the pack tree is fully staged and validated before any destination write begins, so a mid-copy pack failure still cannot touch `dst`.

Update the call site (`apply.go:92-96`):

```go
		case KindDir:
			desiredDirs[a.Path] = true
			var prevFiles []string
			if prev := prevLock.Artifact(a.Path); prev != nil {
				prevFiles = prev.Files
			}
			files, err := mergeDir(a.SrcDir, abs, prevFiles)
			if err != nil {
				return err
			}
			a.Files = files
```

Add `Files []string` to `engine.Artifact` and carry it into the `LockArtifact` literal at `apply.go:115`:

```go
		arts = append(arts, lockfile.LockArtifact{
			Path: a.Path, Kind: a.Kind, Hash: a.Hash, Keys: a.Keys, Files: a.Files,
		})
```

- [ ] **Step 5: Run to verify pass**

Run: `go test ./internal/cli/ -run TestSkillDir -v && go test ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/lockfile/ internal/engine/
git commit -m "fix(engine): stop deleting files teams add to skill directories

stageDir removed the destination tree wholesale, so any file added to
an esc-owned skill dir was destroyed by the next sync. The lockfile now
records the pack-relative paths each sync wrote, which is what lets us
tell a file the team added from a file the pack dropped. Pre-manifest
locks preserve everything unknown on first sync."
```

---

### Task 6: Report directory amendments

**Files:**
- Modify: `internal/engine/status.go` (`classify`, `KindDir` branch)
- Test: `internal/cli/` integration tests

**Interfaces:**
- Consumes: `lockfile.LockArtifact.Files` (Task 5), `newAmendment` (Task 4)

- [ ] **Step 1: Write the failing test**

```go
func TestSkillDirAmendmentReported(t *testing.T) {
	repo := setupGovernedRepoWithSkills(t)
	runEsc(t, repo, "sync")
	dir := filepath.Join(repo, ".claude", "skills", "esc-security")
	if err := os.WriteFile(filepath.Join(dir, "team-notes.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := engine.Status(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range st.Findings {
		if f.Kind != engine.KindDir {
			continue
		}
		if f.State != engine.InSync {
			t.Errorf("managed axis = %q, want in-sync", f.State)
		}
		if f.Local != engine.LocalAmended {
			t.Errorf("local axis = %q, want amended", f.Local)
		}
		if f.Amendment == nil || len(f.Amendment.Items) != 1 || f.Amendment.Items[0] != "team-notes.md" {
			t.Errorf("amendment = %+v", f.Amendment)
		}
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run TestSkillDirAmendment -v`
Expected: FAIL. `f.Local` is `none` and `f.Amendment` is nil.

- [ ] **Step 3: Implement**

The `managed` hash for a dir must now cover only pack-provided files, or an added file would flip the artifact to `altered`. Replace the `KindDir` branch of `classify`:

```go
	case KindDir:
		if info, err := os.Stat(abs); err != nil || !info.IsDir() {
			return Finding{Path: a.Path, Kind: a.Kind, State: Missing, Local: LocalNone,
				Detail: "skill directory missing — run `esc sync`"}
		}
		f := Finding{Path: a.Path, Kind: a.Kind, Local: LocalNone}
		unmanaged, err := unmanagedDirFiles(abs, a.Files)
		if err != nil {
			return Finding{Path: a.Path, Kind: a.Kind, State: Altered, Local: LocalNone, Detail: err.Error()}
		}
		if am := newAmendment("", unmanaged); am != nil {
			f.Local, f.Amendment = LocalAmended, am
		}
		actual, err := pack.DirHashOf(abs, a.Files)
		if err != nil {
			f.State, f.Detail = Altered, err.Error()
			return f
		}
		if actual == a.Hash {
			f.State = InSync
			return f
		}
		f.State, f.Detail = staleOrAltered(actual, "skill directory was modified")
		if f.State == Altered {
			f.Alteration = &Alteration{ExpectedHash: a.Hash, ActualHash: actual}
		}
		return f
```

Add `pack.DirHashOf(dir string, rel []string) (string, error)`, hashing only the listed relative paths in sorted order using the same canonical scheme as the existing `pack.DirHash`. Keep `DirHash` for plan-time hashing of a pack's own tree, where every file is pack-provided by definition. Add `unmanagedDirFiles(dir string, provided []string) ([]string, error)` in `status.go`, walking `dir` and returning sorted slash-separated relative paths not in `provided`.

Plan-time `a.Files` for a dir artifact comes from walking `a.SrcDir` in `engine.go` where the artifact is built (`engine.go:229`), so status has the expected file list without reading the lockfile.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/engine/ ./internal/cli/ ./internal/pack/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/engine/ internal/pack/
git commit -m "feat(engine): report unmanaged files in skill directories

The managed hash now covers only pack-provided files, so a file a team
adds reports as an amendment rather than flipping the directory to
altered."
```

---

### Task 7: Sync skips altered artifacts

**Files:**
- Modify: `internal/engine/apply.go:52` (`Apply` signature and the write loop), `internal/cli/cli.go` (sync command)
- Test: `internal/cli/` integration tests

**Interfaces:**
- Consumes: `classify` (Tasks 4, 6)
- Produces:
  - `engine.Skipped{Path, Kind, Reason, ExpectedHash, ActualHash string}`
  - `engine.SyncResult{Applied []string; Skipped []Skipped}`
  - `engine.Apply(root string, p *PlanResult, force bool) (*SyncResult, error)`

- [ ] **Step 1: Write the failing test**

```go
func TestSyncSkipsAlteredBlock(t *testing.T) {
	repo := setupGovernedRepo(t)
	runEsc(t, repo, "sync")

	path := filepath.Join(repo, "AGENTS.md")
	orig, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := bytes.Replace(orig, []byte("Be kind"), []byte("Be unkind"), 1)
	if bytes.Equal(orig, edited) {
		t.Fatal("precondition: fixture text not found")
	}
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	lockBefore, err := os.ReadFile(filepath.Join(repo, ".escapement", "escapement.lock"))
	if err != nil {
		t.Fatal(err)
	}

	out, code := runEscOut(t, repo, "sync")
	if code != 0 {
		t.Errorf("exit = %d, want 0; a declined artifact must not fail a rollout\n%s", code, out)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, edited) {
		t.Error("sync overwrote a hand-edited block")
	}
	lockAfter, err := os.ReadFile(filepath.Join(repo, ".escapement", "escapement.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(lockBefore, lockAfter) {
		t.Error("lock entry for a skipped artifact must not advance")
	}

	st, err := engine.Status(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range st.Findings {
		if f.Path == "AGENTS.md" && f.State != engine.Altered {
			t.Errorf("state = %q, want altered after a skip", f.State)
		}
	}
}

func TestSyncForceOverwritesAlteredBlockKeepingAmendment(t *testing.T) {
	repo := setupGovernedRepo(t)
	runEsc(t, repo, "sync")
	path := filepath.Join(repo, "AGENTS.md")
	orig, _ := os.ReadFile(path)
	edited := append(bytes.Replace(orig, []byte("Be kind"), []byte("Be unkind"), 1),
		[]byte("\n## Team rules\n\nours\n")...)
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatal(err)
	}

	runEsc(t, repo, "sync", "--force")

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(after, []byte("Be unkind")) {
		t.Error("--force must converge the managed block")
	}
	if !bytes.Contains(after, []byte("## Team rules")) {
		t.Error("--force must not remove local amendments")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run TestSync -v`
Expected: FAIL. Sync currently overwrites, so the edited content is gone and the lock advanced.

- [ ] **Step 3: Implement**

In `internal/engine/apply.go`:

```go
// Skipped is one artifact Apply declined to write.
type Skipped struct {
	Path         string `json:"path"`
	Kind         string `json:"kind"`
	Reason       string `json:"reason"`
	ExpectedHash string `json:"expected_hash"`
	ActualHash   string `json:"actual_hash"`
}

// SyncResult reports what a sync wrote and what it declined to write.
type SyncResult struct {
	Applied []string  `json:"applied"`
	Skipped []Skipped `json:"skipped,omitempty"`
}
```

Change the signature to `func Apply(root string, p *PlanResult, force bool) (*SyncResult, error)`.

Inside the artifact loop, before the write switch:

```go
		if !force {
			if f := classify(root, a, prevLock); f.State == Altered {
				res.Skipped = append(res.Skipped, Skipped{
					Path: a.Path, Kind: a.Kind, Reason: f.Detail,
					ExpectedHash: a.Hash, ActualHash: alterationActual(f),
				})
				// Carry the previous lock entry forward unchanged. classify
				// distinguishes stale from altered by comparing against
				// locked.Hash, so an entry that advanced past what we last
				// actually wrote would make the alteration vanish from the
				// next status run.
				if prev := prevLock.Artifact(a.Path); prev != nil {
					arts = append(arts, *prev)
				}
				continue
			}
		}
```

`alterationActual` returns `f.Alteration.ActualHash` when non-nil and `""` otherwise. Append `a.Path` to `res.Applied` after a successful write.

In `internal/cli/cli.go`, add a `--force` bool flag to the sync command, pass it through, and print one warning line per skip:

```go
	for _, s := range res.Skipped {
		fmt.Fprintf(stderr, "  skipped %s: %s\n", s.Path, s.Reason)
	}
	if len(res.Skipped) > 0 {
		fmt.Fprintln(stderr, "  `esc diff` to inspect, `esc sync --force` to overwrite")
	}
```

Sync still returns 0. Update every other `Apply` call site reported by `grep -rn "engine.Apply(" internal/`.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/engine/ internal/cli/
git commit -m "feat(engine): skip altered artifacts instead of overwriting them

Sync leaves a hand-edited managed region alone, syncs everything else,
warns, and exits 0. A repo that declines part of a policy update does
not fail its own rollout; the state is reported instead. The lock entry
for a skipped artifact stays put, which is what keeps the alteration
visible on later status runs. --force converges.

Exit 0 from sync no longer asserts the repo matches policy. Compliance
gating belongs on `esc status --check`."
```

---

### Task 8: Reporting level resolution

**Files:**
- Create: `internal/engine/reporting.go`
- Modify: `internal/pack/pack.go:32`, `internal/config/config.go:23-29`
- Test: `internal/engine/reporting_test.go`

**Interfaces:**
- Produces:
  - `pack.Reporting{Amendments string}`, `pack.Manifest.Reporting *Reporting`
  - `config.Config.ReportAmendments string`
  - `engine.Collection{Amendments, Source string}`
  - `engine.ResolveReporting(packs []*pack.Pack, cfg *config.Config) (Collection, error)`

Levels: `off` < `metrics` < `content`. Sources: `default`, `pack`, `repo-override`.

- [ ] **Step 1: Write the failing test**

Create `internal/engine/reporting_test.go`:

```go
package engine

import (
	"testing"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/pack"
)

func pk(level string) *pack.Pack {
	p := &pack.Pack{}
	if level != "" {
		p.Manifest.Reporting = &pack.Reporting{Amendments: level}
	}
	return p
}

func TestResolveReporting(t *testing.T) {
	cases := []struct {
		name       string
		packs      []*pack.Pack
		override   string
		wantLevel  string
		wantSource string
		wantErr    bool
	}{
		{"no packs declare", []*pack.Pack{pk("")}, "", "off", "default", false},
		{"single pack metrics", []*pack.Pack{pk("metrics")}, "", "metrics", "pack", false},
		{"single pack content", []*pack.Pack{pk("content")}, "", "content", "pack", false},
		{"highest wins", []*pack.Pack{pk("metrics"), pk("content")}, "", "content", "pack", false},
		{"highest wins reversed", []*pack.Pack{pk("content"), pk("metrics")}, "", "content", "pack", false},
		{"repo clamps down", []*pack.Pack{pk("content")}, "metrics", "metrics", "repo-override", false},
		{"repo clamps off", []*pack.Pack{pk("content")}, "off", "off", "repo-override", false},
		{"repo cannot raise", []*pack.Pack{pk("metrics")}, "content", "", "", true},
		{"invalid pack level", []*pack.Pack{pk("everything")}, "", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveReporting(tc.packs, &config.Config{ReportAmendments: tc.override})
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Amendments != tc.wantLevel || got.Source != tc.wantSource {
				t.Errorf("got %+v, want %s/%s", got, tc.wantLevel, tc.wantSource)
			}
		})
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/engine/ -run TestResolveReporting -v`
Expected: FAIL, undefined: `ResolveReporting`, `pack.Reporting`

- [ ] **Step 3: Add the schema fields**

In `internal/pack/pack.go`, add to `Manifest`:

```go
	Reporting *Reporting `yaml:"reporting,omitempty"`
```

and the type:

```go
// Reporting declares what a repo consuming this pack may send upstream.
// Absent means nothing is sent, ever: the unconnected path stays silent by
// construction, the same stance UpdateCheck takes.
type Reporting struct {
	Amendments string `yaml:"amendments"` // "metrics" | "content"
}
```

In `internal/config/config.go`, add to `Config`:

```go
	ReportAmendments string `yaml:"report_amendments,omitempty"` // "metrics" | "off"; may only clamp down
```

`config.Load` uses `dec.KnownFields(true)`, so the field must be declared before any config can set it.

- [ ] **Step 4: Implement resolution**

Create `internal/engine/reporting.go`:

```go
package engine

import (
	"fmt"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/pack"
)

// Reporting levels, ordered.
const (
	ReportOff     = "off"
	ReportMetrics = "metrics"
	ReportContent = "content"
)

var reportRank = map[string]int{ReportOff: 0, ReportMetrics: 1, ReportContent: 2}

// Collection is the resolved reporting level and where it came from. Spec 1
// resolves and reports it but never acts on it: local surfaces always show
// complete local truth, and redaction lives in the publisher.
type Collection struct {
	Amendments string `json:"amendments"`
	Source     string `json:"source"` // default | pack | repo-override
}

// ResolveReporting returns the effective level. The highest level any pack
// requests wins, mirroring the update-check rule that the strictest cadence
// wins: a pack can make policy stricter but never weaker, so adding a second
// pack can never silently reduce what an admin sees. The repo may then clamp
// down, never up.
func ResolveReporting(packs []*pack.Pack, cfg *config.Config) (Collection, error) {
	level, source := ReportOff, "default"
	for _, p := range packs {
		if p == nil || p.Manifest.Reporting == nil {
			continue
		}
		want := p.Manifest.Reporting.Amendments
		rank, ok := reportRank[want]
		if !ok || want == ReportOff {
			return Collection{}, fmt.Errorf("%w: pack %s declares reporting.amendments %q (want metrics or content)",
				esc.ErrManifest, p.Manifest.Name, want)
		}
		if rank > reportRank[level] {
			level, source = want, "pack"
		}
	}
	if cfg == nil || cfg.ReportAmendments == "" {
		return Collection{Amendments: level, Source: source}, nil
	}
	want := cfg.ReportAmendments
	rank, ok := reportRank[want]
	if !ok || want == ReportContent {
		return Collection{}, fmt.Errorf("%w: report_amendments %q (want metrics or off)", esc.ErrConfig, want)
	}
	if rank >= reportRank[level] {
		return Collection{}, fmt.Errorf("%w: report_amendments %q cannot raise the level above the pack-declared %q",
			esc.ErrConfig, want, level)
	}
	return Collection{Amendments: want, Source: "repo-override"}, nil
}
```

`esc.ErrConfig` does not exist yet and there is no exit-2 case in `exitCode`. Add both, since AGENTS.md requires new failure modes to go through the sentinels.

In `internal/esc/errs.go`, alongside the existing sentinels:

```go
	ErrConfig = errors.New("invalid repo config")
```

In `internal/cli/cli.go:92-99`, add the case:

```go
	case errors.Is(err, esc.ErrConfig):
		return 2
```

A malformed `report_amendments`, or one attempting to raise the level, is a usage error: the operator wrote something the tool cannot honor, and exit 2 is the documented usage code.

- [ ] **Step 5: Run to verify pass**

Run: `go test ./internal/engine/ -run TestResolveReporting -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/pack/ internal/config/ internal/engine/
git commit -m "feat(engine): resolve the reporting level from pack and repo

Pack declares, repo may decline. Absent means nothing is ever sent. The
highest level any pack requests wins so a second pack cannot silently
reduce visibility; the repo clamp is the veto, and it is visible in the
resolved source rather than silent."
```

---

### Task 9: JSON report surfaces

**Files:**
- Create: `internal/engine/report.go`
- Modify: `internal/cli/cli.go` (`cmdStatus`, `cmdSync`)
- Test: `internal/engine/report_test.go`, `internal/cli/testdata/*.golden.json`

**Interfaces:**
- Consumes: `Finding` (Task 4), `SyncResult` (Task 7), `Collection` (Task 8)
- Produces:
  - `engine.Report{Schema int; Packs []ReportPack; Artifacts []Finding; Collection Collection; Skipped []Skipped}`
  - `engine.NewReport(st *StatusResult, coll Collection, sync *SyncResult) *Report`

- [ ] **Step 1: Write the failing test**

Create `internal/engine/report_test.go`:

```go
package engine

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReportIncludesContentRegardlessOfLevel(t *testing.T) {
	st := &StatusResult{Findings: []Finding{{
		Path: "AGENTS.md", Kind: KindBlock, State: InSync, Local: LocalAmended,
		Amendment: newAmendment("our own rules\n", nil),
	}}}
	// Local surfaces always report complete local truth. The level is
	// reported, never applied; redaction is the publisher's job.
	rep := NewReport(st, Collection{Amendments: ReportOff, Source: "default"}, nil)
	out, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "our own rules") {
		t.Errorf("local report must include content even at level off:\n%s", out)
	}
	if !strings.Contains(string(out), `"amendments":"off"`) {
		t.Errorf("collection must be reported:\n%s", out)
	}
}

func TestReportSchemaVersion(t *testing.T) {
	rep := NewReport(&StatusResult{}, Collection{}, nil)
	if rep.Schema != 1 {
		t.Errorf("schema = %d, want 1", rep.Schema)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/engine/ -run TestReport -v`
Expected: FAIL, undefined: `NewReport`, `Report`

- [ ] **Step 3: Implement**

Create `internal/engine/report.go`:

```go
package engine

// ReportPack is one pack pin as reported to a consumer.
type ReportPack struct {
	Source string `json:"source"`
	Ref    string `json:"ref"`
	Pinned string `json:"pinned"`
	Latest string `json:"latest,omitempty"`
	Signed bool   `json:"signed"`
}

// Report is the machine-readable status document and the contract the
// telemetry publisher consumes. It always carries complete local truth; the
// reporting level is reported here but applied downstream.
type Report struct {
	Schema     int          `json:"schema"`
	Packs      []ReportPack `json:"packs"`
	Artifacts  []Finding    `json:"artifacts"`
	Collection Collection   `json:"collection"`
	Skipped    []Skipped    `json:"skipped,omitempty"`
}

// NewReport assembles the document. sync may be nil for status runs.
func NewReport(st *StatusResult, coll Collection, sync *SyncResult) *Report {
	r := &Report{Schema: 1, Collection: coll, Artifacts: st.Findings}
	if st.Plan != nil {
		for _, lp := range st.Plan.Packs {
			r.Packs = append(r.Packs, ReportPack{
				Source: lp.Source, Ref: lp.Ref, Pinned: lp.Hash,
				Signed: packSigned(st.Plan, lp.Source),
			})
		}
	}
	if sync != nil {
		r.Skipped = sync.Skipped
	}
	return r
}
```

`packSigned` reads the trust setting already resolved during planning; reuse whatever field `PlanResult` carries rather than recomputing. Populate `Latest` from the existing `updatecheck.LastSuccess` data that `Status` already loads, matching by source.

Populate `Alteration.Diff` here, for altered artifacts only, using the existing `gitDiff` helper in `internal/engine/diff.go`. Human output does not need it and does not pay for the shell-out.

Add `--json` to both commands in `internal/cli/cli.go`. When set, marshal the report with `json.MarshalIndent(rep, "", "  ")` for stable goldens, write it to stdout, and suppress the human output. Exit-code behavior is unchanged by the flag.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/engine/ -run TestReport -v`
Expected: PASS

- [ ] **Step 5: Add golden coverage**

Add CLI integration tests writing `esc status --json` output for four repos: unadulterated, augmented, altered, and mixed (augmented plus altered plus a skill-dir amendment). Compare against `internal/cli/testdata/status-*.golden.json`. Hashes vary per run, so normalize them through a helper that replaces any 64-hex run with `<hash>` before comparing.

- [ ] **Step 6: Run and commit**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: PASS, `gofmt -l` prints nothing

```bash
git add internal/engine/ internal/cli/
git commit -m "feat(cli): add --json to status and sync

The machine-readable document is the contract the telemetry publisher
will consume. It always carries complete local truth: it is the user's
own machine and their own files, and an open-source user with no
reporting block still gets a fully useful report."
```

---

### Task 10: Human output

**Files:**
- Modify: `internal/cli/cli.go:255-292` (`cmdStatus`)
- Test: `internal/cli/` integration tests

**Interfaces:**
- Consumes: `Finding` (Task 4), `Collection` (Task 8)

- [ ] **Step 1: Write the failing test**

```go
func TestStatusOutputShowsAmendmentAndNotice(t *testing.T) {
	repo := setupGovernedRepoWithReporting(t, "content") // Task 0; pack declares reporting
	runEsc(t, repo, "sync")
	path := filepath.Join(repo, "AGENTS.md")
	existing, _ := os.ReadFile(path)
	if err := os.WriteFile(path, append(existing, []byte("\n## Team\n\nours\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := runEscOut(t, repo, "status")
	if code != 0 {
		t.Errorf("exit = %d, want 0; an amendment alone is not drift", code)
	}
	if !strings.Contains(out, "lines local") {
		t.Errorf("missing amendment suffix:\n%s", out)
	}
	if !strings.Contains(out, "reported upstream") {
		t.Errorf("missing collection notice:\n%s", out)
	}
	if strings.Contains(out, "—") {
		t.Errorf("no em-dashes in user-facing copy:\n%s", out)
	}
}

func TestStatusNoticeAbsentWhenReportingOff(t *testing.T) {
	repo := setupGovernedRepo(t) // pack declares no reporting
	runEsc(t, repo, "sync")
	path := filepath.Join(repo, "AGENTS.md")
	existing, _ := os.ReadFile(path)
	os.WriteFile(path, append(existing, []byte("\n## Team\n\nours\n")...), 0o644)

	out, _ := runEscOut(t, repo, "status")
	if strings.Contains(out, "reported upstream") {
		t.Errorf("notice must not print when nothing is sent:\n%s", out)
	}
	if !strings.Contains(out, "lines local") {
		t.Errorf("local reporting is unaffected by the level:\n%s", out)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run TestStatus -v`
Expected: FAIL, output lacks both strings.

- [ ] **Step 3: Implement**

Replace the finding loop in `cmdStatus`:

```go
	for _, f := range st.Findings {
		suffix := ""
		if f.Amendment != nil {
			switch {
			case len(f.Amendment.Items) > 0:
				suffix = fmt.Sprintf("  ·  %d unmanaged %s preserved",
					len(f.Amendment.Items), plural(len(f.Amendment.Items), "file", "files"))
			default:
				suffix = fmt.Sprintf("  ·  +%d lines local", f.Amendment.Lines)
			}
		}
		if f.State == engine.InSync {
			fmt.Fprintf(stdout, "  ✓ %-20s in sync%s\n", f.Path, suffix)
			continue
		}
		fmt.Fprintf(stdout, "  ✗ %-20s %s: %s%s\n", f.Path, f.State, f.Detail, suffix)
		if f.State == engine.Altered {
			fmt.Fprintf(stdout, "  %-22s `esc diff` to inspect · `esc sync --force` to overwrite\n", "")
		}
		if f.State == engine.ConstraintViolated {
			hasViolation = true
		}
	}
```

After the loop, print the collection notice when the resolved level is above `off` and at least one finding carries an amendment:

```go
	if coll.Amendments != engine.ReportOff && anyAmendment(st.Findings) {
		what := "counts and hashes only"
		if coll.Amendments == engine.ReportContent {
			what = "including content"
		}
		fmt.Fprintf(stdout, "\nLocal amendments are reported upstream, %s.\n", what)
		fmt.Fprintln(stdout, "(pack policy; set report_amendments: metrics or off in .escapement.yaml to withhold)")
	}
```

The notice is unconditional rather than behind a verbose flag: a team must be able to see that its own additions are reported without reading the pack manifest.

Three supporting pieces, none of which exist yet.

`coll` comes from resolving the level against the plan `cmdStatus` already computed. Immediately after the `engine.Status` call:

```go
	coll, err := engine.ResolveReporting(st.Plan.PackObjs, st.Plan.Config)
	if err != nil {
		return exitCode(err, stderr)
	}
```

Add both helpers to `internal/cli/cli.go`:

```go
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func anyAmendment(fs []engine.Finding) bool {
	for _, f := range fs {
		if f.Amendment != nil {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/
git commit -m "feat(cli): show local amendments and the collection notice

Amendment size renders alongside in-sync state, since an amendment is
expected behavior rather than a finding. The notice prints whenever
anything would be sent upstream."
```

---

### Task 11: Docs

**Files:**
- Modify: `AGENTS.md`, `README.md`, `ai-governance-product-spec.md`, `CHANGELOG.md`

- [ ] **Step 1: Update `AGENTS.md`**

Under "Gotchas and invariants", add:

```markdown
- **Local amendments are preserved for every artifact kind.** Content escapement does not own (bytes outside a managed block, non-owned MCP server entries, files added to a skill directory) survives every sync and is reported on the `local` axis. Skill directories carry a per-file manifest in the lockfile, which is what distinguishes a file the team added from one the pack dropped.
- **Two orthogonal axes per artifact:** `managed` (in-sync, altered, stale, missing, orphan) and `local` (none, amended). Never collapse them; an artifact can be both amended and altered.
- **Sync skips altered artifacts.** A hand-edited managed region is left alone, warned about, and reported. `esc sync` exit 0 means everything escapement was willing to apply was applied, not that the repo matches policy. Gate compliance on `esc status --check`.
- **New managed blocks are inserted at the top of a file**, after YAML frontmatter and a leading H1. Existing blocks are replaced in place and never move. `<!-- escapement:block -->` is the explicit override.
```

- [ ] **Step 2: Update `README.md`**

Cover: `esc sync` exit-code semantics and `esc status --check` as the compliance gate; `--force`; `report_amendments`; what "augmented" means and that it is not a problem state. No em-dashes.

- [ ] **Step 3: Update the product spec**

In `ai-governance-product-spec.md` §5, add the reporting model and the pack-declares/repo-declines consent shape. In §11, mark open question 2 (enforcement vs. observation in v1) decided in favor of observation, with the enablement rationale and a pointer to the spec doc.

- [ ] **Step 4: CHANGELOG entry**

```markdown
- Local amendments: content escapement does not own is preserved everywhere, reported on a new `local` axis, and surfaced in `esc status`. Files added to skill directories are no longer deleted by sync.
- `esc sync` skips artifacts whose managed region was hand-edited instead of overwriting them, warns, and exits 0. `esc sync --force` converges.
- New managed blocks are inserted at the top of a file rather than appended.
- `esc status --json` and `esc sync --json` emit a machine-readable report.
- `reporting.amendments` in a pack manifest and `report_amendments` in repo config resolve the level at which a repo reports local amendments upstream.
```

- [ ] **Step 5: Verify and commit**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: PASS

Run: `grep -n "—" README.md CHANGELOG.md`
Expected: no output

```bash
git add AGENTS.md README.md ai-governance-product-spec.md CHANGELOG.md
git commit -m "docs: local amendment model, sync semantics, top placement"
```

---

## Verification Before Completion

Before claiming this plan complete, run and paste the output:

```bash
go test ./... && go vet ./... && gofmt -l .
```

Then confirm each spec section maps to a shipped task:

| Spec section | Task |
|---|---|
| Amendment §1 two axes | 4, 6 |
| Amendment §2 directory manifests | 5 |
| Amendment §3 skip and report | 7 |
| Amendment §4 exit codes | 7, 11 |
| Amendment §5 reporting surface | 4, 9, 10 |
| Amendment §6 reporting configuration | 8 |
| Amendment §7 naming | 1 |
| Amendment §8 testing | every task |
| Amendment §9 docs | 11 |
| Onboarding §1 top placement | 2 |
