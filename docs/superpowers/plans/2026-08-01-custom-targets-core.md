# Plan 1: Data-driven targets — core (pack-defined custom targets)

Spec: `/Users/billyz/code/openescapement/docs/superpowers/specs/2026-08-01-custom-targets-design.md` (approved, committed 9bc16a5). This plan covers spec §1.2, §2 (all), §3, §7, the core slice of §8, and a README/CHANGELOG docs task. Spec §1.1 metadata content, §4 portal, §6 demo, and portal tests are Plan 2.

## Goal

Make the renderer's target set data-driven so any pack in a repo's config can define managed-block markdown targets (for example `.github/copilot-instructions.md`), gated by strict path validation, single-owner collision rules, and a repo-side acknowledgment list. Custom targets are fully usable from git without the portal.

## Non-goals (Plan 1)

- No portal Targets page, no `internal/targets` metadata content (Readers/DocURLs/Description/Suggestion stay empty structurally-present fields), no demo seed. These are Plan 2.
- No renaming/reconfiguring the six built-ins. No custom whole-file / skills-dir / mcp targets. No per-repo target *definitions* (repos only acknowledge).
- No `PolicyDiff` (`esc diff --ref`) support for custom targets — `DriftDiff` (which iterates `plan.Artifacts`) covers custom drift; alt-ref policy diff of custom files is out of scope for Plan 1 and left as a documented limitation.

## Global Constraints (copied from spec + AGENTS.md)

- **Single external dependency:** `gopkg.in/yaml.v3` only; everything else stdlib; system `git` via `os/exec`. **No `golang.org/x/text`.** NFC normalization is a no-op because `file` is ASCII-only (spec §2.1); case-folding is `strings.ToLower` after ASCII validation.
- **Sentinel exit codes** (`internal/esc/errs.go`, mapped in `internal/cli/cli.go:87`): 0 ok; 1 constraint (`esc.ErrConstraint`); 2 usage; 3 integrity/signature (`esc.ErrSignature`, `esc.ErrLockMismatch`); 4 other (default, `esc.ErrManifest` falls here). **All §2.1–§2.3 and §3 unknown-reference violations are constraint failures (exit 1), i.e. must wrap `esc.ErrConstraint`.** Signature problems stay exit 3.
- **Renderer invariants (golden-testable):** bytes outside a managed block are never modified; nothing is written after a verification or constraint failure; all writes are atomic; renderer output is deterministic.
- **Validation ordering:** custom-target validation runs **after `verifyTrust`, before any write** — parsed from verified pack content only, never during fetch. In the engine this means: after the fetch/verify/load pack loop, before the artifact/compose loop.
- **Ownership:** a custom target is owned by exactly one pack; only that pack's fragments that *explicitly* name it render into it. A fragment with no `targets:` front-matter means all built-in targets only — never any custom target.
- **Custom targets are always `Kind: managed-block`.** All renderer invariants and `constraints` (`max_file_bytes`, `forbidden_patterns`) apply unchanged.
- Schema stays `1`; `pack.yaml` and `config.yaml` keep `KnownFields(true)` (older esc rejects `custom_targets:` loudly).
- Every task keeps the app building and all tests passing. `go test ./...` builds real temp git repos with `ESC_CACHE_DIR`; no mocks.
- `go vet ./...` and `gofmt -w .` clean before completion. No AI co-author trailers in commits.

## Design decisions (read before starting)

These resolve ambiguities the executor must not re-litigate:

1. **Sentinel split for custom-target failures.** Structural/definition failures compute an unusable effective set and are **hard errors** returned from `planFromConfig` wrapping `esc.ErrConstraint`: invalid custom-target definition (§2.1) and cross-pack collision (§2.2). The **acknowledgment gate (§2.3)** is modeled as a `render.Violation` — this flows through the existing `Apply` violation gate (fail-closed, blocks all writes) **and** the existing `Status` violation loop (so `esc status` reports it), exactly as spec §2.3 requires ("sync fails … and `esc status` reports it"). The §3 **unknown/foreign fragment reference** is caught at `pack.Load`/`loadFragment` time (naturally pack-scoped: a fragment can only name its own pack's declared customs) and wraps `esc.ErrConstraint`.
2. **Fragment-target validation stays in `loadFragment`, made pack-aware.** `loadFragment` receives the pack's own declared custom-target names and accepts `built-in-fragment-targets ∪ own-customs`; anything else is an `esc.ErrConstraint` unknown-reference. This preserves the invariant "a loaded `Fragment.Targets` is always valid for its pack" and makes "foreign name" indistinguishable from "unknown" (correct — a foreign pack's target is simply not declared here). The existing `pack_test` "unknown fragment target" case changes its expectation from `ErrManifest` to `ErrConstraint`.
3. **`internal/targets` is the single source of truth for built-in names/kinds and for `ValidateCustom`.** `pack` imports `targets` (for `IsFragmentTarget`). `render` is left largely untouched (keeps its own `Target*` consts and `TargetFile` map — Plan 2 consolidates them); the only additions to `render` are the pure functions `ComposeCustom` and `RemoveBlock`. No import cycle: `targets` imports only stdlib; `pack → targets`; `render → pack`; `engine → render, pack, targets`.
4. **Per-artifact block provenance.** `Artifact` gains `BlockPacks []string`. Built-in block artifacts set it to all pack labels (unchanged behavior); custom block artifacts set it to the owner pack only. `prospectiveContent` and `Apply` splice using `a.BlockPacks` instead of `render.PackLabels(p.PackObjs)`. The managed-block body hash is unaffected (the header `packs=` list is not part of the hashed body), so this is provenance-correctness only.
5. **`ComposeCustom` renders notice header + owner's explicitly-targeting fragments only, no catalog** (catalog carve-out, spec §3).
6. **Orphan removal** uses the lockfile as the record of previously-written artifacts. In `Apply`, any locked `KindBlock` artifact whose path is not in the new desired block set has its managed block spliced out (bytes outside preserved); if the file becomes byte-empty it is deleted. `Status` reports such files as a new `Orphan` state.
7. **Symlink refusal** (`refuseSymlinks`) runs in the apply phase, per-component `Lstat`, immediately before each write, for every artifact kind. Documented residual Lstat→rename race accepted (spec §2.1 M1).

## Files touched

- **New:** `internal/targets/targets.go`, `internal/targets/targets_test.go`
- **New:** `internal/render/testdata/copilot.golden.md`
- **New:** `internal/cli/custom_targets_test.go` (engine integration via CLI, real temp git repos)
- **Edit:** `internal/pack/pack.go` (parse `custom_targets`), `internal/pack/fragment.go` (pack-aware fragment targets), `internal/pack/pack_test.go`
- **Edit:** `internal/config/config.go` (`allow_custom_target_files`)
- **Edit:** `internal/render/render.go` (add `ComposeCustom`), `internal/render/block.go` (add `RemoveBlock`), `internal/render/golden_test.go` (custom golden case)
- **Edit:** `internal/engine/engine.go` (effective set, custom artifacts, validation, `BlockPacks`), `internal/engine/apply.go` (symlink refusal, orphan removal), `internal/engine/diff.go` (`prospectiveContent` signature), `internal/engine/status.go` (`Orphan` state + reporting)
- **Edit:** `README.md`, `CHANGELOG.md`

---

## Task 1 — `internal/targets` package: built-in table + `ValidateCustom`

Self-contained; no other package changes. Establishes the single source of truth for built-in target metadata and every §2.1 name/file rule.

### 1a. Failing test

Create `internal/targets/targets_test.go`:

```go
package targets

import "testing"

func TestBuiltIns(t *testing.T) {
	want := map[string]struct {
		file string
		kind string
	}{
		"claude":     {"CLAUDE.md", KindManagedBlock},
		"agents":     {"AGENTS.md", KindManagedBlock},
		"gemini":     {"GEMINI.md", KindManagedBlock},
		"governance": {"GOVERNANCE.md", KindWholeFile},
		"skills":     {"", KindSkillsDir},
		"mcp":        {"", KindMCPConfig},
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
		if in.File != w.file || in.Kind != w.kind || !in.BuiltIn || in.OwnerPack != "" {
			t.Errorf("built-in %q wrong: %+v", in.Name, in)
		}
	}
	if !IsFragmentTarget("claude") || !IsFragmentTarget("governance") {
		t.Error("claude/governance must be fragment targets")
	}
	if IsFragmentTarget("skills") || IsFragmentTarget("mcp") || IsFragmentTarget("copilot") {
		t.Error("skills/mcp/custom must not be fragment targets")
	}
}

func TestValidateCustom(t *testing.T) {
	cases := []struct {
		desc string
		name string
		file string
		ok   bool
	}{
		{"valid single segment", "qwen", "QWEN.md", true},
		{"valid github carve-out", "copilot", ".github/copilot-instructions.md", true},
		{"valid dashed name", "my-target", "docs/policy.md", true},

		{"name uppercase", "Copilot", "x.md", false},
		{"name leading digit", "1copilot", "x.md", false},
		{"name empty", "", "x.md", false},
		{"name too long", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "x.md", false}, // 33 chars
		{"name underscore", "co_pilot", "x.md", false},
		{"name collides builtin", "claude", "x.md", false},

		{"file empty", "copilot", "", false},
		{"file traversal parent", "copilot", "../x.md", false},
		{"file traversal middle", "copilot", "a/../../x.md", false},
		{"file absolute", "copilot", "/etc/x.md", false},
		{"file backslash", "copilot", `a\b.md`, false},
		{"file non-md", "copilot", "foo.txt", false},
		{"file depth 3", "copilot", "a/b/c.md", false},
		{"file trailing dot segment", "copilot", "foo./bar.md", false},
		{"file trailing space segment", "copilot", "foo /bar.md", false},
		{"file leading slash empty seg", "copilot", "/x.md", false},
		{"file double slash", "copilot", "a//b.md", false},
		{"file non-ascii", "copilot", "café.md", false},
		{"denylist .claude", "copilot", ".claude/commands/x.md", false},
		{"denylist .git", "copilot", ".git/x.md", false},
		{"denylist .escapement", "copilot", ".escapement/x.md", false},
		{"denylist github workflows", "copilot", ".github/workflows/x.md", false},
		{"denylist .vscode", "copilot", ".vscode/x.md", false},
		{"denylist .idea", "copilot", ".idea/x.md", false},
		{"file collides builtin claude", "copilot", "claude.md", false},
		{"file collides builtin case", "copilot", "Claude.MD", false},
		{"file collides builtin agents", "copilot", "AGENTS.md", false},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			err := ValidateCustom(tc.name, tc.file)
			if tc.ok && err != nil {
				t.Errorf("ValidateCustom(%q,%q) = %v, want nil", tc.name, tc.file, err)
			}
			if !tc.ok && err == nil {
				t.Errorf("ValidateCustom(%q,%q) = nil, want error", tc.name, tc.file)
			}
		})
	}
}
```

Run: `go test ./internal/targets/` → fails to compile (package/symbols absent). Confirm the failure is the expected "undefined" / "no such package".

### 1b. Implement

Create `internal/targets/targets.go`:

```go
// Package targets owns instruction-file target metadata: the compiled-in
// built-in table and validation for pack-defined custom targets. It is the
// single source of truth shared by the CLI and (in Plan 2) the portal, and
// imports only the standard library.
package targets

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

// Built-in target names.
const (
	NameClaude     = "claude"
	NameAgents     = "agents"
	NameGemini     = "gemini"
	NameGovernance = "governance"
	NameSkills     = "skills"
	NameMCP        = "mcp"
)

// Target kinds.
const (
	KindManagedBlock = "managed-block"
	KindWholeFile    = "whole-file"
	KindSkillsDir    = "skills-dir"
	KindMCPConfig    = "mcp-config"
)

// Info is one target's metadata. For Plan 1 only Name/File/Kind/BuiltIn/
// OwnerPack are populated for built-ins; Readers/DocURLs/Description/Suggestion
// exist for Plan 2 (the portal metadata table) and stay empty here.
type Info struct {
	Name        string   // "claude"
	File        string   // "CLAUDE.md" (empty for skills/mcp)
	Kind        string   // one of the Kind* constants
	Readers     []string // agent tools that read it (Plan 2)
	DocURLs     []string // official vendor documentation (Plan 2)
	Description string   // one paragraph (Plan 2)
	Suggestion  string   // esc's guidance (Plan 2)
	BuiltIn     bool
	OwnerPack   string // defining pack name; empty for built-ins
}

var builtIns = []Info{
	{Name: NameClaude, File: "CLAUDE.md", Kind: KindManagedBlock, BuiltIn: true},
	{Name: NameAgents, File: "AGENTS.md", Kind: KindManagedBlock, BuiltIn: true},
	{Name: NameGemini, File: "GEMINI.md", Kind: KindManagedBlock, BuiltIn: true},
	{Name: NameGovernance, File: "GOVERNANCE.md", Kind: KindWholeFile, BuiltIn: true},
	{Name: NameSkills, File: "", Kind: KindSkillsDir, BuiltIn: true},
	{Name: NameMCP, File: "", Kind: KindMCPConfig, BuiltIn: true},
}

// BuiltIns returns a copy of the compiled-in target table.
func BuiltIns() []Info {
	out := make([]Info, len(builtIns))
	copy(out, builtIns)
	return out
}

// ByName returns the built-in with the given name.
func ByName(name string) (Info, bool) {
	for _, in := range builtIns {
		if in.Name == name {
			return in, true
		}
	}
	return Info{}, false
}

// IsBuiltInName reports whether name is a built-in target name.
func IsBuiltInName(name string) bool {
	_, ok := ByName(name)
	return ok
}

// IsFragmentTarget reports whether name is a built-in target a fragment may
// name in front-matter (the markdown targets: claude, agents, gemini,
// governance; not skills/mcp).
func IsFragmentTarget(name string) bool {
	in, ok := ByName(name)
	return ok && (in.Kind == KindManagedBlock || in.Kind == KindWholeFile)
}

var customName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// fileChars matches the whole file string when every rune is in the allowed
// ASCII set. A single disallowed rune (including any non-ASCII, backslash, or
// space) makes it fail.
var fileChars = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)

// controlDirs are first path segments a custom target file may not use. The
// carve-out is .github/*.md (two segments), which is not in this set;
// .github/workflows/*.md is three segments and rejected by the depth rule.
var controlDirs = map[string]bool{
	".git": true, ".escapement": true, ".claude": true, ".gemini": true,
	".cursor": true, ".codex": true, ".agent": true, ".agents": true,
	".vscode": true, ".idea": true,
}

// ValidateCustom enforces every §2.1 name and file rule. It returns a plain
// descriptive error (no sentinel); the caller wraps it with esc.ErrConstraint,
// and the portal (Plan 2) surfaces the message verbatim. All comparisons are
// case-folded; NFC is a no-op because file is ASCII-only.
func ValidateCustom(name, file string) error {
	if !customName.MatchString(name) {
		return fmt.Errorf("target name %q must match ^[a-z][a-z0-9-]{0,31}$", name)
	}
	if IsBuiltInName(strings.ToLower(name)) {
		return fmt.Errorf("target name %q collides with a built-in target", name)
	}

	if file == "" {
		return fmt.Errorf("target %q: file is required", name)
	}
	if !fileChars.MatchString(file) {
		return fmt.Errorf("target file %q: only ASCII [A-Za-z0-9._/-] is allowed (no spaces, backslashes, or non-ASCII)", file)
	}
	// Clean must be a no-op: no ., no .., no //, no trailing slash. path.Clean
	// operates on slash paths, which is the required separator here.
	if path.Clean(file) != file {
		return fmt.Errorf("target file %q must be a clean, relative, slash-separated path", file)
	}
	segs := strings.Split(file, "/")
	for _, s := range segs {
		if s == "" {
			return fmt.Errorf("target file %q must not contain empty path segments", file)
		}
		if s == ".." {
			return fmt.Errorf("target file %q must not contain %q segments", file, "..")
		}
		if strings.HasSuffix(s, ".") || strings.HasSuffix(s, " ") {
			return fmt.Errorf("target file %q: no path segment may end with a dot or space", file)
		}
	}
	if strings.HasPrefix(file, "/") {
		return fmt.Errorf("target file %q must be relative (no leading slash)", file)
	}
	if len(segs) > 2 {
		return fmt.Errorf("target file %q may have at most 2 path segments", file)
	}
	if !strings.HasSuffix(strings.ToLower(file), ".md") {
		return fmt.Errorf("target file %q must end in .md", file)
	}
	if controlDirs[strings.ToLower(segs[0])] {
		return fmt.Errorf("target file %q: %q is a control directory and may not hold custom targets", file, segs[0])
	}
	lf := strings.ToLower(file)
	for _, in := range builtIns {
		if in.File != "" && strings.ToLower(in.File) == lf {
			return fmt.Errorf("target file %q collides with built-in target file %q", file, in.File)
		}
	}
	return nil
}
```

Run: `go test ./internal/targets/` → passes. Run `go build ./...`.

### 1c. Commit

`feat(targets): add target metadata table and custom-target validation`

Body: introduce internal/targets as the single source of truth for built-in target names/files/kinds and ValidateCustom, enforcing the §2.1 name regex, ASCII/clean-path/depth/denylist/.md and case-folded built-in-collision rules. Stdlib-only; no NFC library needed because file is ASCII-only. Shared verbatim by CLI now and the portal in Plan 2.

### Interfaces introduced

```go
// internal/targets
const NameClaude, NameAgents, NameGemini, NameGovernance, NameSkills, NameMCP string
const KindManagedBlock, KindWholeFile, KindSkillsDir, KindMCPConfig string
type Info struct { Name, File, Kind string; Readers, DocURLs []string; Description, Suggestion string; BuiltIn bool; OwnerPack string }
func BuiltIns() []Info
func ByName(name string) (Info, bool)
func IsBuiltInName(name string) bool
func IsFragmentTarget(name string) bool
func ValidateCustom(name, file string) error   // nil or plain (unwrapped) error
```

---

## Task 2 — `pack`: parse `custom_targets`, accept pack-owned target names in fragments

### 2a. Failing test

Add to `internal/pack/pack_test.go`. First, **change** the existing "unknown fragment target" case expectation and body assertion path: it now returns `esc.ErrConstraint` (not `ErrManifest`). Edit `TestLoadManifestErrors` to split target-reference errors out, and add custom-target coverage:

Replace the `{"unknown fragment target", ...}` entry in the `TestLoadManifestErrors` slice with nothing (remove it from that table — it no longer yields `ErrManifest`), then add this new test function:

```go
func TestCustomTargets(t *testing.T) {
	base := func() map[string]string {
		f := validFiles()
		f["pack.yaml"] = `schema: 1
name: acme-org
version: 1.4.0
custom_targets:
  - name: copilot
    file: .github/copilot-instructions.md
    doc: https://docs.github.com/copilot
    description: Copilot instructions.
rules:
  - rules/secrets.md
`
		f["rules/secrets.md"] = "---\ntargets: [claude, copilot]\n---\nbody\n"
		delete(f, "rules/hosting.md")
		return f
	}

	t.Run("parses and accepts own custom target in fragment", func(t *testing.T) {
		p, err := Load(writePack(t, base()))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if len(p.Manifest.CustomTargets) != 1 {
			t.Fatalf("custom targets: %+v", p.Manifest.CustomTargets)
		}
		ct := p.Manifest.CustomTargets[0]
		if ct.Name != "copilot" || ct.File != ".github/copilot-instructions.md" {
			t.Errorf("custom target fields: %+v", ct)
		}
		if got := p.Fragments[0].Targets; len(got) != 2 || got[1] != "copilot" {
			t.Errorf("fragment targets: %v", got)
		}
	})

	t.Run("unknown/foreign fragment target is a constraint error", func(t *testing.T) {
		f := base()
		f["rules/secrets.md"] = "---\ntargets: [nope]\n---\nbody\n"
		_, err := Load(writePack(t, f))
		if !errors.Is(err, esc.ErrConstraint) {
			t.Fatalf("want ErrConstraint, got %v", err)
		}
	})

	t.Run("missing custom target name is a manifest error", func(t *testing.T) {
		f := base()
		f["pack.yaml"] = "schema: 1\nname: x\nversion: 1.0.0\ncustom_targets:\n  - file: X.md\n"
		delete(f, "rules/secrets.md")
		_, err := Load(writePack(t, f))
		if !errors.Is(err, esc.ErrManifest) {
			t.Fatalf("want ErrManifest, got %v", err)
		}
	})

	t.Run("unknown manifest field still rejected (KnownFields stays true)", func(t *testing.T) {
		f := base()
		f["pack.yaml"] = "schema: 1\nname: x\nversion: 1.0.0\nbogus: true\n"
		delete(f, "rules/secrets.md")
		_, err := Load(writePack(t, f))
		if !errors.Is(err, esc.ErrManifest) {
			t.Fatalf("want ErrManifest, got %v", err)
		}
	})
}
```

Run: `go test ./internal/pack/` → fails (`CustomTargets` undefined; unknown-reference still `ErrManifest`).

### 2b. Implement

`internal/pack/pack.go` — add the type and manifest field, and a presence check.

Add after the `Constraints` type:

```go
// CustomTarget is a pack-defined managed-block markdown target (§1.2). Full
// name/file validation lives in internal/targets.ValidateCustom and runs in
// the engine after verifyTrust; the manifest only checks presence here.
type CustomTarget struct {
	Name        string `yaml:"name"`
	File        string `yaml:"file"`
	Doc         string `yaml:"doc,omitempty"`
	Description string `yaml:"description,omitempty"`
}
```

Add the field to `Manifest` (after `UpdateCheck`):

```go
	CustomTargets []CustomTarget `yaml:"custom_targets,omitempty"`
```

In `Manifest.validate`, add before the final `return nil`:

```go
	for _, ct := range m.CustomTargets {
		if ct.Name == "" {
			return fail("custom target: name is required")
		}
		if ct.File == "" {
			return fail("custom target %q: file is required", ct.Name)
		}
	}
```

In `Load`, thread the declared custom-target names into fragment loading. Replace the fragment loop:

```go
	p := &Pack{Dir: dir, Manifest: m}
	customNames := map[string]bool{}
	for _, ct := range m.CustomTargets {
		customNames[ct.Name] = true
	}
	for _, rel := range m.Rules {
		frag, err := loadFragment(dir, rel, customNames)
		if err != nil {
			return nil, err
		}
		p.Fragments = append(p.Fragments, *frag)
	}
	return p, nil
```

`internal/pack/fragment.go` — make target validation pack-aware and a constraint error. Replace the `ValidTargets` var and `loadFragment` signature/validation:

```go
package pack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/targets"
)

// Fragment is one markdown rule file. Empty Targets means the fragment applies
// to every built-in target (never a custom target).
type Fragment struct {
	Path    string
	Targets []string
	Body    string
}

// loadFragment parses one rule file. A fragment may name built-in fragment
// targets or one of its own pack's declared custom targets (customNames);
// naming anything else — including another pack's custom target — is a
// constraint failure (§3 unknown/foreign reference).
func loadFragment(packDir, rel string, customNames map[string]bool) (*Fragment, error) {
	raw, err := os.ReadFile(filepath.Join(packDir, rel))
	if err != nil {
		return nil, fmt.Errorf("%w: rule %s: %v", esc.ErrManifest, rel, err)
	}
	frag := &Fragment{Path: rel}
	content := string(raw)
	if fm, body, ok := splitFrontmatter(content); ok {
		var meta struct {
			Targets []string `yaml:"targets"`
		}
		if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
			return nil, fmt.Errorf("%w: rule %s frontmatter: %v", esc.ErrManifest, rel, err)
		}
		for _, tgt := range meta.Targets {
			if !targets.IsFragmentTarget(tgt) && !customNames[tgt] {
				return nil, fmt.Errorf("%w: rule %s: unknown or foreign target %q (not a built-in target or a custom target defined by this pack)", esc.ErrConstraint, rel, tgt)
			}
		}
		frag.Targets = meta.Targets
		frag.Body = body
	} else {
		frag.Body = content
	}
	return frag, nil
}
```

(`splitFrontmatter` is unchanged. Note: `strings` is still imported by `splitFrontmatter`.)

Run: `go test ./internal/pack/ ./...` → passes. `go build ./...`.

### 2c. Commit

`feat(pack): parse custom_targets and scope fragment targets to the owning pack`

Body: pack.yaml grows an optional custom_targets section (name/file required, doc/description optional); schema stays 1 and KnownFields stays true so older esc rejects it loudly. Fragment target references are now validated against built-in fragment targets plus the pack's own custom targets, and an unknown or foreign reference is a constraint failure per spec §3 (exit 1) rather than a plain manifest error.

### Interfaces introduced

```go
// internal/pack
type CustomTarget struct { Name, File, Doc, Description string }
// Manifest.CustomTargets []CustomTarget   // yaml: custom_targets
// loadFragment(packDir, rel string, customNames map[string]bool) (*Fragment, error)  // internal
```

---

## Task 3 — `config`: parse `allow_custom_target_files`

### 3a. Failing test

Create `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestLoadAllowCustomTargetFiles(t *testing.T) {
	root := writeConfig(t, `schema: 1
packs:
  - source: file:///tmp/p
    ref: v1.0.0
    trust: unsigned
allow_custom_target_files:
  - .github/copilot-instructions.md
  - PAYMENTS-AGENTS.md
`)
	c, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.AllowCustomTargetFiles) != 2 || c.AllowCustomTargetFiles[0] != ".github/copilot-instructions.md" {
		t.Errorf("allow_custom_target_files: %v", c.AllowCustomTargetFiles)
	}
}

func TestLoadUnknownFieldRejected(t *testing.T) {
	root := writeConfig(t, "schema: 1\npacks: []\nbogus: true\n")
	if _, err := Load(root); err == nil {
		t.Fatal("want error for unknown config field")
	}
}
```

Run: `go test ./internal/config/` → fails (`AllowCustomTargetFiles` undefined).

### 3b. Implement

`internal/config/config.go` — add the field to `Config`:

```go
type Config struct {
	Schema                 int       `yaml:"schema"`
	Packs                  []PackRef `yaml:"packs"`
	AllowedSignersFile     string    `yaml:"allowed_signers_file,omitempty"`
	Targets                []string  `yaml:"targets,omitempty"`                   // empty = all targets
	AllowCustomTargetFiles []string  `yaml:"allow_custom_target_files,omitempty"` // acknowledged custom-target files (§2.3)
}
```

Run: `go test ./internal/config/ ./...` → passes. `go build ./...`.

### 3c. Commit

`feat(config): parse allow_custom_target_files acknowledgment list`

Body: .escapement/config.yaml gains allow_custom_target_files, the repo-side acknowledgment that restores the property that a repo's write surface is enumerable from its own config (spec §2.3). KnownFields stays true. The fail-closed gate that consumes it lands with the engine changes.

### Interfaces introduced

```go
// internal/config
// Config.AllowCustomTargetFiles []string   // yaml: allow_custom_target_files
```

---

## Task 4 — `render`: pack-scoped custom compose + managed-block removal

Pure functions; no engine wiring yet.

### 4a. Failing test

Add the custom golden case to `internal/render/golden_test.go`. Add a helper and extend the `cases` map inside `TestGolden`:

Add above `TestGolden`:

```go
// customOwner is a pack that both defines catalog entries and a fragment
// explicitly naming a custom target, so the golden proves the catalog
// carve-out: custom output carries the fragment but no catalog section.
func customOwner() *pack.Pack {
	return &pack.Pack{
		Manifest: pack.Manifest{
			Name: "acme-org", Version: "1.4.0",
			Catalog: []pack.CatalogEntry{
				{Name: "Tailscale", Category: "hosting-exposure", Status: "preferred", Notes: "Org tailnet"},
			},
			CustomTargets: []pack.CustomTarget{
				{Name: "copilot", File: ".github/copilot-instructions.md"},
			},
		},
		Fragments: []pack.Fragment{
			{Path: "rules/secrets.md", Body: "## Secrets\nUse Vault.\n"}, // no targets: never in custom
			{Path: "rules/copilot.md", Targets: []string{"copilot"}, Body: "## Copilot\nUse suggestions carefully.\n"},
		},
	}
}
```

Add `"github.com/tensorgroup/openescapement/internal/pack"` to the golden_test imports. Extend the `cases` map:

```go
		"copilot.golden.md": ComposeCustom(customOwner(), "copilot"),
```

Add a dedicated assertion test in `render_test.go`:

```go
func TestComposeCustom(t *testing.T) {
	body := ComposeCustom(customOwner(), "copilot")
	if !strings.Contains(body, "## Copilot") {
		t.Errorf("custom body missing explicitly-targeted fragment:\n%s", body)
	}
	if strings.Contains(body, "## Secrets") {
		t.Errorf("untargeted fragment leaked into custom target:\n%s", body)
	}
	if strings.Contains(body, "Tool & service policy") || strings.Contains(body, "Tailscale") {
		t.Errorf("catalog section must not appear in a custom target file (carve-out):\n%s", body)
	}
	if !strings.Contains(body, "Managed by escapement") {
		t.Errorf("notice missing:\n%s", body)
	}
	if !strings.HasSuffix(body, "\n") {
		t.Errorf("custom body must end with newline")
	}
}

func TestRemoveBlock(t *testing.T) {
	spliced, err := Splice([]byte("# Team\n\nkeep me\n"), "policy body\n", BlockMeta{Packs: []string{"p@1"}})
	if err != nil {
		t.Fatal(err)
	}
	out, removed, err := RemoveBlock(spliced)
	if err != nil || !removed {
		t.Fatalf("RemoveBlock: removed=%v err=%v", removed, err)
	}
	if !strings.Contains(string(out), "keep me") || strings.Contains(string(out), "policy body") {
		t.Errorf("RemoveBlock must strip only the managed block:\n%s", out)
	}
	// Idempotent: no block present now.
	if _, again, _ := RemoveBlock(out); again {
		t.Error("RemoveBlock reported a second block")
	}
	// A file that is only a managed block becomes byte-empty.
	only, err := Splice(nil, "x\n", BlockMeta{Packs: []string{"p@1"}})
	if err != nil {
		t.Fatal(err)
	}
	empty, removed, err := RemoveBlock(only)
	if err != nil || !removed {
		t.Fatalf("RemoveBlock only-block: %v %v", removed, err)
	}
	if len(empty) != 0 {
		t.Errorf("removing the sole block should leave byte-empty content, got %q", empty)
	}
}
```

Create the golden file `internal/render/testdata/copilot.golden.md` with **exactly** these bytes (note the em-dash is from the existing `notice` constant; one leading notice line, one blank line, the fragment, trailing newline):

```
> Managed by escapement — do not edit. Run `esc diff` to see source. Team content goes outside this block.

## Copilot
Use suggestions carefully.
```

Run: `go test ./internal/render/` → fails (`ComposeCustom`, `RemoveBlock` undefined).

### 4b. Implement

`internal/render/render.go` — add after `Compose`:

```go
// ComposeCustom renders the managed-block body for one pack-defined custom
// target. Only the owning pack's fragments that explicitly name the target are
// included; fragments with empty targets are never included. No catalog
// section is rendered (the catalog carve-out, §3). Output is deterministic.
func ComposeCustom(p *pack.Pack, target string) string {
	var b strings.Builder
	b.WriteString(notice)
	b.WriteString("\n")
	for _, f := range p.Fragments {
		if !fragmentNamesTarget(f, target) {
			continue
		}
		b.WriteString("\n")
		b.WriteString(strings.TrimSuffix(f.Body, "\n"))
		b.WriteString("\n")
	}
	return b.String()
}

// fragmentNamesTarget reports whether f explicitly names target. Unlike
// fragmentApplies, an empty target list never matches — custom targets require
// explicit opt-in.
func fragmentNamesTarget(f pack.Fragment, target string) bool {
	for _, t := range f.Targets {
		if t == target {
			return true
		}
	}
	return false
}
```

`internal/render/block.go` — add after `Splice`:

```go
// RemoveBlock returns file with its managed block removed, preserving every
// byte outside the block. removed reports whether a block was present. An
// error is returned only when the block structure is corrupt.
func RemoveBlock(file []byte) ([]byte, bool, error) {
	b, err := Extract(file)
	if err != nil {
		return nil, false, err
	}
	if b == nil {
		return file, false, nil
	}
	s := string(file)
	return []byte(s[:b.start] + s[b.end:]), true, nil
}
```

Run: `go test ./internal/render/` → passes (golden matches the committed file). `go build ./...`.

### 4c. Commit

`feat(render): add pack-scoped custom compose and managed-block removal`

Body: ComposeCustom renders a custom target's file from only the owning pack's fragments that explicitly name it, with no catalog section (spec §3 carve-out); a golden test locks the bytes. RemoveBlock strips a managed block while preserving all other bytes, the primitive the engine uses to clean up orphaned target files.

### Interfaces introduced

```go
// internal/render
func ComposeCustom(p *pack.Pack, target string) string
func RemoveBlock(file []byte) (out []byte, removed bool, err error)
```

---

## Task 5 — `engine`: render acknowledged custom targets with collision and acknowledgment gates

The plan-phase core. Adds `Artifact.BlockPacks`, custom-target validation (§2.1 hard error), cross-pack collision (§2.2 hard error), acknowledgment gate (§2.3 Violation), pack-scoped custom artifacts, and threads per-artifact block provenance through `prospectiveContent` and `Apply`.

### 5a. Failing tests

Create `internal/cli/custom_targets_test.go`. This reuses the `internal/cli` helpers (`newPackRepo`, `writeFiles`, `gitIn`, `run`) and adds a custom-pack builder. Real temp git repos, `ESC_CACHE_DIR` set by helpers.

```go
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newCustomPackRepo builds a git pack repo whose pack.yaml declares the given
// custom targets and whose one fragment explicitly names fragTargets.
func newCustomPackRepo(t *testing.T, name, version, packYAMLBody, fragment string) string {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"pack.yaml":      packYAMLBody,
		"rules/main.md":  fragment,
	})
	gitIn(t, dir, "init", "-q", "-b", "main")
	gitIn(t, dir, "config", "user.email", "t@e.com")
	gitIn(t, dir, "config", "user.name", "T")
	gitIn(t, dir, "config", "commit.gpgsign", "false")
	gitIn(t, dir, "config", "tag.gpgsign", "false")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "v"+version)
	gitIn(t, dir, "tag", "-a", "v"+version, "-m", "v"+version)
	return dir
}

const copilotPackYAML = `schema: 1
name: acme-org
version: 1.0.0
custom_targets:
  - name: copilot
    file: .github/copilot-instructions.md
    doc: https://docs.github.com/copilot
    description: Copilot instructions.
rules:
  - rules/main.md
`

const copilotFragment = "---\ntargets: [claude, copilot]\n---\n## Copilot rule\nUse Vault.\n"

// governedWith writes a governed repo pinning packRepo (root of the git repo,
// pack.yaml at repo root -> no //subdir), with the given extra config lines.
func governedWith(t *testing.T, packRepo, ref, extra string) string {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		".escapement/config.yaml": "schema: 1\npacks:\n  - source: file://" + packRepo + "\n    ref: " + ref + "\n    trust: unsigned\n" + extra,
		"CLAUDE.md":               "# Team notes\n",
	})
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	return dir
}

func TestCustomTargetAcknowledgmentGate(t *testing.T) {
	repo := newCustomPackRepo(t, "acme-org", "1.0.0", copilotPackYAML, copilotFragment)

	// Without acknowledgment: sync fails closed, exit 1, names pack + target + file.
	root := governedWith(t, repo, "v1.0.0", "")
	code, out := run(t, root, "sync")
	if code != 1 {
		t.Fatalf("unacknowledged sync exit %d, want 1:\n%s", code, out)
	}
	for _, want := range []string{"copilot", "acme-org", ".github/copilot-instructions.md", "allow_custom_target_files"} {
		if !strings.Contains(out, want) {
			t.Errorf("error missing %q:\n%s", want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".github", "copilot-instructions.md")); !os.IsNotExist(err) {
		t.Error("file must not be written when unacknowledged (fail closed, not skip)")
	}

	// status reports the unacknowledged target.
	if code, out := run(t, root, "status"); code == 0 || !strings.Contains(out, "copilot") {
		t.Errorf("status should report unacknowledged target, exit=%d:\n%s", code, out)
	}

	// With acknowledgment: sync succeeds and lands the block beside seeded content.
	root2 := governedWith(t, repo, "v1.0.0", "allow_custom_target_files:\n  - .github/copilot-instructions.md\n")
	writeFiles(t, root2, map[string]string{".github/copilot-instructions.md": "# Existing user content\n"})
	if code, out := run(t, root2, "sync"); code != 0 {
		t.Fatalf("acknowledged sync exit %d:\n%s", code, out)
	}
	got, err := os.ReadFile(filepath.Join(root2, ".github", "copilot-instructions.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if !strings.Contains(s, "# Existing user content") {
		t.Error("seeded user content must be preserved")
	}
	if !strings.Contains(s, "## Copilot rule") || !strings.Contains(s, "escapement:begin") {
		t.Errorf("managed block missing from custom file:\n%s", s)
	}
	if strings.Contains(s, "Tool & service policy") {
		t.Error("custom file must not carry a catalog section")
	}
	// CLAUDE.md also got the copilot fragment (it names claude).
	claude, _ := os.ReadFile(filepath.Join(root2, "CLAUDE.md"))
	if !strings.Contains(string(claude), "## Copilot rule") {
		t.Error("claude target should include the fragment")
	}
}

func TestCustomTargetOwnershipIsolation(t *testing.T) {
	// A second pack whose fragment names copilot but does NOT declare it: fails
	// at load as a foreign reference (exit 1).
	foreignYAML := `schema: 1
name: team-pack
version: 1.0.0
rules:
  - rules/main.md
`
	repo := newCustomPackRepo(t, "team-pack", "1.0.0", foreignYAML, "---\ntargets: [copilot]\n---\nbody\n")
	root := governedWith(t, repo, "v1.0.0", "allow_custom_target_files:\n  - .github/copilot-instructions.md\n")
	if code, out := run(t, root, "sync"); code != 1 || !strings.Contains(out, "copilot") {
		t.Fatalf("foreign fragment reference should fail exit 1, got %d:\n%s", code, out)
	}
}

func TestCustomTargetEmptyFrontmatterNeverRenders(t *testing.T) {
	// Fragment with no targets: applies to built-ins only, never the custom file.
	repo := newCustomPackRepo(t, "acme-org", "1.0.0", copilotPackYAML, "## Plain rule\nno frontmatter\n")
	root := governedWith(t, repo, "v1.0.0", "allow_custom_target_files:\n  - .github/copilot-instructions.md\n")
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("sync exit %d:\n%s", code, out)
	}
	// The custom file is either absent or has an empty (notice-only) block; it
	// must never contain the fragment body.
	if b, err := os.ReadFile(filepath.Join(root, ".github", "copilot-instructions.md")); err == nil {
		if strings.Contains(string(b), "Plain rule") {
			t.Errorf("empty-frontmatter fragment leaked into custom target:\n%s", b)
		}
	}
	claude, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if !strings.Contains(string(claude), "Plain rule") {
		t.Error("empty-frontmatter fragment should reach built-in claude")
	}
}

func TestCustomTargetCollisions(t *testing.T) {
	// Two packs declaring the same custom file: collision, exit 1, names both.
	p1YAML := `schema: 1
name: org-pack
version: 1.0.0
custom_targets:
  - name: copilot
    file: SHARED.md
rules:
  - rules/main.md
`
	p2YAML := `schema: 1
name: team-pack
version: 1.0.0
custom_targets:
  - name: helper
    file: SHARED.md
rules:
  - rules/main.md
`
	r1 := newCustomPackRepo(t, "org-pack", "1.0.0", p1YAML, "---\ntargets: [copilot]\n---\nbody\n")
	r2 := newCustomPackRepo(t, "team-pack", "1.0.0", p2YAML, "---\ntargets: [helper]\n---\nbody\n")
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		".escapement/config.yaml": "schema: 1\npacks:\n" +
			"  - source: file://" + r1 + "\n    ref: v1.0.0\n    trust: unsigned\n" +
			"  - source: file://" + r2 + "\n    ref: v1.0.0\n    trust: unsigned\n" +
			"allow_custom_target_files:\n  - SHARED.md\n",
		"CLAUDE.md": "# t\n",
	})
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	code, out := run(t, dir, "sync")
	if code != 1 {
		t.Fatalf("file collision should exit 1, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "org-pack") || !strings.Contains(out, "team-pack") {
		t.Errorf("collision error should name both packs:\n%s", out)
	}
}

func TestCustomTargetInvalidDefinition(t *testing.T) {
	badYAML := `schema: 1
name: acme-org
version: 1.0.0
custom_targets:
  - name: copilot
    file: .claude/commands/x.md
rules:
  - rules/main.md
`
	repo := newCustomPackRepo(t, "acme-org", "1.0.0", badYAML, "---\ntargets: [copilot]\n---\nbody\n")
	root := governedWith(t, repo, "v1.0.0", "allow_custom_target_files:\n  - .claude/commands/x.md\n")
	if code, out := run(t, root, "sync"); code != 1 {
		t.Fatalf("invalid custom-target definition should exit 1, got %d:\n%s", code, out)
	}
}

func TestCustomTargetFilterInteraction(t *testing.T) {
	repo := newCustomPackRepo(t, "acme-org", "1.0.0", copilotPackYAML, copilotFragment)
	// targets filter excludes copilot: the custom file is not rendered.
	root := governedWith(t, repo, "v1.0.0",
		"targets: [claude]\nallow_custom_target_files:\n  - .github/copilot-instructions.md\n")
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("sync exit %d:\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(root, ".github", "copilot-instructions.md")); !os.IsNotExist(err) {
		t.Error("filtered-out custom target must not be rendered")
	}
}

func TestCustomTargetDeterministic(t *testing.T) {
	repo := newCustomPackRepo(t, "acme-org", "1.0.0", copilotPackYAML, copilotFragment)
	root := governedWith(t, repo, "v1.0.0", "allow_custom_target_files:\n  - .github/copilot-instructions.md\n")
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("sync: %d\n%s", code, out)
	}
	first, _ := os.ReadFile(filepath.Join(root, ".github", "copilot-instructions.md"))
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("re-sync: %d\n%s", code, out)
	}
	second, _ := os.ReadFile(filepath.Join(root, ".github", "copilot-instructions.md"))
	if string(first) != string(second) {
		t.Error("custom target output is not deterministic across syncs")
	}
	if code, _ := run(t, root, "status"); code != 0 {
		t.Error("status should be clean after sync")
	}
}
```

Run: `go test ./internal/cli/ -run TestCustomTarget` → fails (custom targets not yet wired; acknowledged sync writes nothing / unknown target).

### 5b. Implement

`internal/engine/engine.go`:

1. Add import `"github.com/tensorgroup/openescapement/internal/targets"`.

2. Add `BlockPacks` to `Artifact` (after `Body`):

```go
	// BlockPacks are the "name@version" labels recorded in a managed block's
	// header (kind=block). Built-in blocks carry every pack; a custom target
	// carries only its owning pack.
	BlockPacks []string
```

3. Replace the target-selection section of `planFromConfig` (everything from `targets := cfg.Targets` down to the end of the `for _, t := range targets` loop) with:

```go
	// Custom targets (§2). Runs after the fetch/verify/load loop above — parsed
	// from verified pack content only, never during fetch — and before any
	// artifact is composed or written.
	customByName, allDeclared, err := collectCustomTargets(res.PackObjs)
	if err != nil {
		return nil, err
	}
	ack := map[string]bool{}
	for _, f := range cfg.AllowCustomTargetFiles {
		ack[strings.ToLower(f)] = true
	}
	// Acknowledgment gate (§2.3): a declared custom target renders only if its
	// file is acknowledged. Unacknowledged targets are a fail-closed Violation
	// (blocks Apply, reported by esc status) — not a silent skip.
	rendered := map[string]custom{} // name -> target to render
	for name, c := range customByName {
		if ack[strings.ToLower(c.file)] {
			rendered[name] = c
			continue
		}
		res.Violations = append(res.Violations, render.Violation{
			Path: c.file,
			Rule: fmt.Sprintf("custom target %q from pack %s is not acknowledged; add this line to allow_custom_target_files in %s: %s",
				name, c.owner.Manifest.Name, config.Path(root), c.file),
		})
	}

	targets := cfg.Targets
	if len(targets) == 0 {
		targets = append(append([]string{}, allTargets...), sortedNames(rendered)...)
	} else {
		for _, t := range targets {
			if !isBuiltInTarget(t) && !allDeclared[strings.ToLower(t)] {
				return nil, fmt.Errorf("config: unknown target %q", t)
			}
		}
	}

	for _, t := range targets {
		if c, ok := rendered[t]; ok {
			body := render.ComposeCustom(c.owner, c.name)
			res.Artifacts = append(res.Artifacts, Artifact{
				Path: c.file, Kind: KindBlock, Hash: render.BodyHash(body), Body: body,
				BlockPacks: render.PackLabels([]*pack.Pack{c.owner}),
			})
			continue
		}
		if allDeclared[strings.ToLower(t)] {
			continue // declared custom target that is not acknowledged: Violation already recorded
		}
		switch t {
		case render.TargetClaude, render.TargetAgents, render.TargetGemini:
			body := render.Compose(res.PackObjs, t)
			res.Artifacts = append(res.Artifacts, Artifact{
				Path: render.TargetFile[t], Kind: KindBlock, Hash: render.BodyHash(body), Body: body,
				BlockPacks: render.PackLabels(res.PackObjs),
			})
		case render.TargetGovernance:
			content := render.Governance(res.PackObjs)
			res.Artifacts = append(res.Artifacts, Artifact{
				Path: render.TargetFile[t], Kind: KindFile, Hash: esc.HashBytes([]byte(content)), Body: content,
			})
		case render.TargetSkills:
			for _, p := range res.PackObjs {
				for _, rel := range p.Manifest.Skills {
					src := filepath.Join(p.Dir, filepath.FromSlash(rel))
					h, err := pack.DirHash(src)
					if err != nil {
						return nil, err
					}
					name := "esc-" + p.Manifest.Name + "-" + filepath.Base(rel)
					res.Artifacts = append(res.Artifacts, Artifact{
						Path: filepath.ToSlash(filepath.Join(".claude", "skills", name)),
						Kind: KindDir, Hash: h, SrcDir: src,
					})
				}
			}
		case render.TargetMCP:
			servers := map[string]map[string]any{}
			for _, p := range res.PackObjs {
				for k, v := range p.Manifest.MCP.Servers {
					servers[k] = v
				}
			}
			if len(servers) > 0 {
				h, err := render.DesiredMCPHash(servers)
				if err != nil {
					return nil, err
				}
				keys := make([]string, 0, len(servers))
				for k := range servers {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				res.Artifacts = append(res.Artifacts, Artifact{
					Path: ".mcp.json", Kind: KindJSONKeys, Hash: h, Keys: keys, Servers: servers,
				})
			}
		default:
			return nil, fmt.Errorf("config: unknown target %q", t)
		}
	}
```

4. Update the constraint-validation loop's `prospectiveContent` call: change `prospectiveContent(root, a, res.PackObjs)` to `prospectiveContent(root, a)` (signature dropping the packs arg, see below).

5. Add these helpers near the bottom of `engine.go`:

```go
// custom is one acknowledged/declared pack custom target during planning.
type custom struct {
	owner *pack.Pack
	name  string
	file  string
}

// collectCustomTargets validates every pack's custom targets (§2.1) and
// enforces cross-pack single-owner collisions (§2.2). It returns the declared
// targets keyed by name and a lower-cased declared-name set (used to validate
// the config targets filter). All failures are constraint errors (exit 1).
func collectCustomTargets(packs []*pack.Pack) (map[string]custom, map[string]bool, error) {
	byName := map[string]custom{}
	declared := map[string]bool{}
	seenName := map[string]string{} // lower name -> owning pack
	seenFile := map[string]string{} // lower file -> owning pack
	for _, p := range packs {
		for _, ct := range p.Manifest.CustomTargets {
			if err := targets.ValidateCustom(ct.Name, ct.File); err != nil {
				return nil, nil, fmt.Errorf("%w: pack %s: custom target %q: %v", esc.ErrConstraint, p.Manifest.Name, ct.Name, err)
			}
			ln, lf := strings.ToLower(ct.Name), strings.ToLower(ct.File)
			if other, ok := seenName[ln]; ok {
				return nil, nil, fmt.Errorf("%w: custom target name %q is declared by both %s and %s; a custom target belongs to exactly one pack", esc.ErrConstraint, ct.Name, other, p.Manifest.Name)
			}
			if other, ok := seenFile[lf]; ok {
				return nil, nil, fmt.Errorf("%w: custom target file %q is declared by both %s and %s; a custom target belongs to exactly one pack", esc.ErrConstraint, ct.File, other, p.Manifest.Name)
			}
			seenName[ln], seenFile[lf] = p.Manifest.Name, p.Manifest.Name
			byName[ct.Name] = custom{owner: p, name: ct.Name, file: ct.File}
			declared[ln] = true
		}
	}
	return byName, declared, nil
}

func isBuiltInTarget(name string) bool {
	for _, t := range allTargets {
		if t == name {
			return true
		}
	}
	return false
}

// sortedNames returns the map keys sorted, for deterministic target ordering.
func sortedNames(m map[string]custom) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
```

6. Change `prospectiveContent` to use per-artifact block provenance and drop the packs param:

```go
// prospectiveContent computes what an artifact's file would contain after
// apply, without writing.
func prospectiveContent(root string, a Artifact) ([]byte, error) {
	existing, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(a.Path)))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	switch a.Kind {
	case KindBlock:
		return render.Splice(existing, a.Body, render.BlockMeta{Packs: a.BlockPacks})
	case KindFile:
		return []byte(a.Body), nil
	}
	return existing, nil
}
```

`internal/engine/diff.go` — update the `DriftDiff` caller: `prospectiveContent(root, a, plan.PackObjs)` → `prospectiveContent(root, a)`.

`internal/engine/apply.go` — in the `KindBlock` case of `Apply`, replace `render.BlockMeta{Packs: render.PackLabels(p.PackObjs)}` with `render.BlockMeta{Packs: a.BlockPacks}`.

Run: `go test ./internal/cli/ -run TestCustomTarget ./...` → expect the acknowledgment/collision/isolation/filter/determinism tests to pass. (Orphan + symlink tests are added in Task 6.) `go build ./...`, `go vet ./...`.

### 5c. Commit

`feat(engine): render acknowledged pack custom targets with ownership gates`

Body: the effective target set is now built-ins plus every configured pack's acknowledged custom targets, computed after verifyTrust and before any write. Custom targets validate via targets.ValidateCustom (§2.1), enforce single-owner cross-pack collisions (§2.2, naming both packs), and require repo-side allow_custom_target_files acknowledgment (§2.3, fail-closed as a constraint violation that blocks sync and shows in esc status). Compose is pack-scoped with the catalog carve-out, and each managed block records its true provenance via Artifact.BlockPacks.

### Interfaces introduced

```go
// internal/engine
// Artifact.BlockPacks []string
func collectCustomTargets(packs []*pack.Pack) (map[string]custom, map[string]bool, error) // internal
func prospectiveContent(root string, a Artifact) ([]byte, error) // signature changed: packs arg dropped
```

---

## Task 6 — `engine` apply: symlink-refusing writes + orphan managed-block removal + status reporting

### 6a. Failing tests

Append to `internal/cli/custom_targets_test.go`:

```go
func TestCustomTargetSymlinkRefusal(t *testing.T) {
	repo := newCustomPackRepo(t, "acme-org", "1.0.0", copilotPackYAML, copilotFragment)
	root := governedWith(t, repo, "v1.0.0", "allow_custom_target_files:\n  - .github/copilot-instructions.md\n")
	// Make .github a symlink to an out-of-repo directory.
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".github")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	code, out := run(t, root, "sync")
	if code != 1 {
		t.Fatalf("writing through a symlinked parent should fail exit 1, got %d:\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(outside, "copilot-instructions.md")); !os.IsNotExist(err) {
		t.Error("must not have written through the symlink")
	}
}

func TestOrphanBlockRemoval(t *testing.T) {
	// v1 defines copilot; sync writes the block into a file with user content.
	repo := newCustomPackRepo(t, "acme-org", "1.0.0", copilotPackYAML, copilotFragment)
	root := governedWith(t, repo, "v1.0.0", "allow_custom_target_files:\n  - .github/copilot-instructions.md\n")
	writeFiles(t, root, map[string]string{".github/copilot-instructions.md": "# Existing user content\n"})
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("initial sync: %d\n%s", code, out)
	}

	// Un-acknowledge the target (drop it from the effective set) by rewriting
	// config with no allow list, and remove the pack's custom target so it is
	// no longer even declared. Simplest: point config at a v2 pack with no
	// custom_targets. Here we just drop acknowledgment AND the filter so the
	// block is orphaned. Repin to a pack version that no longer defines it.
	repo2 := newCustomPackRepo(t, "acme-org", "2.0.0",
		"schema: 1\nname: acme-org\nversion: 2.0.0\nrules:\n  - rules/main.md\n",
		"## Claude rule\nbody\n")
	writeFiles(t, root, map[string]string{
		".escapement/config.yaml": "schema: 1\npacks:\n  - source: file://" + repo2 + "\n    ref: v2.0.0\n    trust: unsigned\n",
	})
	_ = repo // v1 repo no longer referenced

	// status should report the orphan before removal.
	if code, out := run(t, root, "status"); code == 0 || !strings.Contains(strings.ToLower(out), "orphan") {
		t.Errorf("status should report orphan, exit=%d:\n%s", code, out)
	}

	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("re-sync: %d\n%s", code, out)
	}
	got, err := os.ReadFile(filepath.Join(root, ".github", "copilot-instructions.md"))
	if err != nil {
		t.Fatalf("file should still exist (had user content): %v", err)
	}
	if strings.Contains(string(got), "escapement:begin") {
		t.Errorf("orphaned managed block should be removed:\n%s", got)
	}
	if !strings.Contains(string(got), "# Existing user content") {
		t.Errorf("user content must be preserved:\n%s", got)
	}
}

func TestOrphanBlockByteEmptyDeletion(t *testing.T) {
	// Same as above but the custom file had NO user content, so removing the
	// block leaves it byte-empty and the file is deleted.
	repo := newCustomPackRepo(t, "acme-org", "1.0.0", copilotPackYAML, copilotFragment)
	root := governedWith(t, repo, "v1.0.0", "allow_custom_target_files:\n  - .github/copilot-instructions.md\n")
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("initial sync: %d\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(root, ".github", "copilot-instructions.md")); err != nil {
		t.Fatalf("expected custom file after first sync: %v", err)
	}
	repo2 := newCustomPackRepo(t, "acme-org", "2.0.0",
		"schema: 1\nname: acme-org\nversion: 2.0.0\nrules:\n  - rules/main.md\n",
		"## Claude rule\nbody\n")
	writeFiles(t, root, map[string]string{
		".escapement/config.yaml": "schema: 1\npacks:\n  - source: file://" + repo2 + "\n    ref: v2.0.0\n    trust: unsigned\n",
	})
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("re-sync: %d\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(root, ".github", "copilot-instructions.md")); !os.IsNotExist(err) {
		t.Error("byte-empty orphaned custom file should be deleted")
	}
}
```

Run: `go test ./internal/cli/ -run 'TestOrphan|TestCustomTargetSymlink'` → fails (no symlink refusal, no orphan removal, no `orphan` status state).

### 6b. Implement

`internal/engine/status.go`:

1. Add the state constant:

```go
	Orphan             State = "orphan"
```

2. In `Status`, after the `for _, a := range plan.Artifacts` classify loop and before the violations loop, add orphan detection:

```go
	// Orphan managed blocks: files that carry an esc block for a target no
	// longer in the effective set (pack dropped it, repo un-acknowledged it, or
	// a filter excludes it). Reported here and removed by the next sync.
	desiredBlocks := map[string]bool{}
	for _, a := range plan.Artifacts {
		if a.Kind == KindBlock {
			desiredBlocks[a.Path] = true
		}
	}
	for _, la := range lock.Artifacts {
		if la.Kind != KindBlock || desiredBlocks[la.Path] {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(la.Path)))
		if err != nil {
			continue // already gone
		}
		if block, err := render.Extract(content); err == nil && block != nil {
			res.Findings = append(res.Findings, Finding{
				Path: la.Path, State: Orphan,
				Detail: "carries an esc block for a target no longer in the effective set — run `esc sync` to remove it",
			})
		}
	}
```

(`lock` and `render` are already in scope/imported in `status.go`.)

`internal/engine/apply.go`:

1. Add a symlink-refusal helper:

```go
// refuseSymlinks fails closed if any existing path component of rel under root
// is a symlink, immediately before a write. It never follows a symlinked
// parent or target file (§2.1). A residual race between this check and the
// rename remains on shared checkouts and is accepted, documented as the same
// class as any local tooling.
func refuseSymlinks(root, rel string) error {
	cur := root
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		cur = filepath.Join(cur, part)
		fi, err := os.Lstat(cur)
		if err != nil {
			if os.IsNotExist(err) {
				return nil // this and deeper components do not exist yet
			}
			return err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: %s: refusing to write through symlink %s", esc.ErrConstraint, rel, cur)
		}
	}
	return nil
}
```

2. In `Apply`, immediately after `abs, err := containedPath(root, a.Path)` (inside the `for _, a := range p.Artifacts` loop) and before the `switch a.Kind`, add:

```go
		if err := refuseSymlinks(root, a.Path); err != nil {
			return err
		}
```

3. After the existing owned-skill-dir removal block (the `if prevLock != nil { ... }` for `KindDir`), add orphan managed-block removal:

```go
	// Remove stale managed blocks for targets that left the effective set.
	// Bytes outside the block are preserved; a file left byte-empty is deleted.
	// Each removal is a write and carries the same symlink protection.
	desiredBlocks := map[string]bool{}
	for _, a := range p.Artifacts {
		if a.Kind == KindBlock {
			desiredBlocks[a.Path] = true
		}
	}
	if prevLock != nil {
		for _, prev := range prevLock.Artifacts {
			if prev.Kind != KindBlock || desiredBlocks[prev.Path] {
				continue
			}
			abs, err := containedPath(root, prev.Path)
			if err != nil {
				return err
			}
			if err := refuseSymlinks(root, prev.Path); err != nil {
				return err
			}
			existing, err := os.ReadFile(abs)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			out, removed, err := render.RemoveBlock(existing)
			if err != nil {
				return fmt.Errorf("%s: %w", prev.Path, err)
			}
			if !removed {
				continue
			}
			if len(out) == 0 {
				if err := os.Remove(abs); err != nil {
					return err
				}
				continue
			}
			if err := atomicWrite(abs, out); err != nil {
				return err
			}
		}
	}
```

Run: `go test ./internal/cli/ ./internal/engine/ ./...` → all pass. `go build ./...`, `go vet ./...`.

### 6c. Commit

`feat(engine): refuse symlinked writes and remove orphaned managed blocks`

Body: apply now Lstat-checks every path component immediately before each write and fails closed on a symlinked parent or target (spec §2.1; residual local race documented). When a target leaves the effective set, sync strips its stale managed block while preserving all other bytes and deletes the file if it becomes byte-empty; esc status reports such orphans. This is new behavior — previously only orphaned esc- skills dirs were cleaned up.

### Interfaces introduced

```go
// internal/engine
// State Orphan
func refuseSymlinks(root, rel string) error // internal
```

---

## Task 7 — Docs: README "Custom targets" subsection + CHANGELOG

No code. Concise, no em-dashes, no AI-sounding prose.

### 7a. Implement

**README.md** — add a "Custom targets" subsection (place it near the existing targets/config documentation; the executor should read `README.md` first to find the right anchor and match heading depth and prose style). Content:

- **What:** a pack may define extra managed-block markdown targets in `pack.yaml` under `custom_targets`, so an org can govern files like `.github/copilot-instructions.md` or `QWEN.md` without waiting for an esc release. Show the `pack.yaml` snippet:

  ```yaml
  custom_targets:
    - name: copilot
      file: .github/copilot-instructions.md
      doc: https://docs.github.com/en/copilot/customizing-copilot
      description: Repository custom instructions for GitHub Copilot.
  ```

  Note the rules briefly: `name` matches `^[a-z][a-z0-9-]{0,31}$` and is used in fragment `targets:`; `file` is a clean relative `.md` path of at most two segments, ASCII only, not in a control directory. A fragment reaches a custom target only when it names it explicitly; a fragment with no `targets:` goes to built-in files only.

- **Single owner:** a custom target belongs to the one pack that defines it. Only that pack's fragments render into it. Two packs declaring the same name or file is an error and sync refuses. If an org and a team need to co-write one file, put the fragments in the same pack.

- **Acknowledgment and why:** a custom target renders only if its file is listed in the repo's `.escapement/config.yaml`:

  ```yaml
  allow_custom_target_files:
    - .github/copilot-instructions.md
  ```

  Explain the reason plainly: pinning a pack grants it write access to a known, fixed set of files. Custom targets would let a pack choose new paths, so the acknowledgment list keeps a repo's write surface enumerable from the repo's own config. Filename rules alone are not enough because markdown transcludes (one instruction file can pull in another), so a file the pack does not name directly could still change what an agent reads. Without the acknowledgment, sync fails closed and names the file to add.

- **Layering:** show the org-base-plus-team-pack config from spec §5 as the recommended layout.

**CHANGELOG.md** — add one bullet under `## [Unreleased]` → `### Added`, matching the existing dense style:

```
- Custom targets: a pack's `pack.yaml` may define managed-block markdown targets
  under `custom_targets` (`name`, `file`, optional `doc`/`description`), usable in
  fragment `targets:`. Each target is owned by its defining pack (no cross-pack
  co-writing; duplicate name or file across packs fails), renders only the owning
  pack's explicitly-targeting fragments with no catalog section, and lands only
  after the repo acknowledges its file in `allow_custom_target_files` (fail-closed,
  reported by `esc status`). Path rules reject traversal, control directories,
  non-ASCII, and built-in collisions; writes refuse symlinked parents; targets
  that leave the effective set have their managed block removed on the next sync.
```

### 7b. Commit

`docs: document pack custom targets and the acknowledgment gate`

Body: README gains a Custom targets subsection covering the pack.yaml syntax, the single-owner rule, and the allow_custom_target_files acknowledgment with its rationale (a repo's write surface stays enumerable from its own config; markdown transclusion is why filename rules alone are insufficient). CHANGELOG records the feature under Unreleased.

---

## Task 8 — Final verification

No code. Run and confirm clean output:

```
gofmt -l .            # must print nothing
go vet ./...          # must exit 0, no findings
go test ./...         # all packages pass
```

If `gofmt -l .` lists files, run `gofmt -w .` and re-run the trio. Do not commit unless the user asks; if asked, commit any formatting-only changes as `chore: gofmt`.

---

## Test strategy summary

- **Unit (targets):** table-driven `ValidateCustom` covering every §2.1/§8 attack case — traversal, absolute, backslash, control-dir denials, `.github/*.md` carve-out, depth 3, non-md, trailing dot/space, case-fold built-in collisions, non-ASCII, and all name rules.
- **Unit (render):** golden test for a custom target file (proves catalog carve-out and byte-exact output); `RemoveBlock` preserve/idempotent/byte-empty behavior.
- **Unit (pack/config):** custom_targets parse, own-vs-foreign fragment reference sentinels, `allow_custom_target_files` parse, KnownFields still rejects unknowns.
- **Integration (cli, real temp git repos, `ESC_CACHE_DIR`):** acknowledgment gate fail-closed with the right message + status reporting; both collision classes (name and file, naming both packs); ownership isolation (foreign reference fails; empty-frontmatter never reaches custom); catalog carve-out; targets-filter interaction; determinism + clean status; invalid-definition; symlink refusal; orphan removal with user content preserved; orphan byte-empty deletion.

## Verification commands

- `go test ./...`
- `go vet ./...`
- `gofmt -l .` (expect empty)
- Targeted: `go test ./internal/targets/ ./internal/render/ ./internal/pack/ ./internal/config/ ./internal/engine/`; `go test ./internal/cli/ -run TestCustomTarget`; `go test ./internal/cli/ -run 'TestOrphan|Symlink'`

## Risks and open questions

1. **`PolicyDiff` (`esc diff --ref`) skips custom targets** (its `TargetFile[t] == ""` guard). `DriftDiff` covers custom drift. This is a documented Plan-1 limitation; extending alt-ref policy diff to custom targets is not required by the spec and is deferred.
2. **Symlink refusal is best-effort** (Lstat→rename race on shared checkouts) — spec §2.1 M1 explicitly accepts this. The test uses a symlinked parent dir; on filesystems without symlink support it `t.Skip`s.
3. **Sentinel-split assumption** (design decision 1): §2.1 invalid-definition and §2.2 collisions are hard errors; §2.3 acknowledgment is a Violation. Both routes exit 1 via `esc.ErrConstraint`. If a reviewer wants §2.1/§2.2 also surfaced as graceful `esc status` findings rather than hard errors, that is a small follow-up — flagged, not assumed away.
4. **Existing test change:** `pack_test.go`'s "unknown fragment target" case moves from `ErrManifest` to `ErrConstraint` (correct per spec §7). This is an intentional behavior change for built-in typos too — a fragment naming a bogus target is now exit 1, not exit 4.
5. **`allTargets` var retained** as the built-in default that seeds the effective set; §3's "iterate the effective set" is satisfied by the dynamically-built `targets` slice in `planFromConfig`. Callers of `render.TargetFile`/`render.Target*` in `diff.go` are unchanged (built-in-only), which is correct for Plan 1.

## Self-review

- **Plan-1 spec coverage:** §1.2 pack custom_targets (Task 2); §2.1 name/file rules (Task 1) + write-time symlink (Task 6); §2.2 collisions (Task 5); §2.3 acknowledgment (Tasks 3+5); §2.4 trust/sentinels (Global Constraints; unsigned packs may define customs since fetch/verify is unchanged and validation runs post-verify); §3 pack-scoped compose + catalog carve-out + filter + validation ordering + orphan removal + effective-set iteration (Tasks 4/5/6); §7 sentinel routing (Global Constraints + design decision 1); §8 core slice — ValidateCustom table, engine integration (union via multi-pack, both collision classes, ownership, carve-out, ack fail-closed, filter, orphan + byte-empty, symlink, determinism), custom golden (Tasks 1/4/5/6); README + CHANGELOG (Task 7). §1.1 metadata content, §4, §6 correctly excluded (Plan 2).
- **Placeholder scan:** no `TODO`, `...`, `<placeholder>`, or elided bodies; every code block is complete Go with real identifiers matched to the current source (verified against `render.Splice`/`Extract` offsets, `Artifact`, `PlanResult`, `lockfile.LockArtifact{Path,Kind,Hash,Keys}`, `render.Violation{Path,Rule}`, `config.Path`, the `cli` test helpers `newPackRepo`/`writeFiles`/`gitIn`/`run`).
- **Type consistency:** `ValidateCustom(name, file string) error` returns unwrapped errors, wrapped with `esc.ErrConstraint` only at engine call sites; `Artifact.BlockPacks []string` set at every block-artifact construction and consumed in `prospectiveContent`/`Apply`; `prospectiveContent` signature change propagated to its two callers (`engine.go`, `diff.go`); `render.RemoveBlock` returns `([]byte, bool, error)` and is consumed with that arity; new `State Orphan` added alongside existing states and printed by the unchanged `cli.go` status formatter.

---

Files referenced (all absolute): spec `/Users/billyz/code/openescapement/docs/superpowers/specs/2026-08-01-custom-targets-design.md`; sources `/Users/billyz/code/openescapement/internal/{targets(new),pack/pack.go,pack/fragment.go,config/config.go,render/render.go,render/block.go,engine/engine.go,engine/apply.go,engine/diff.go,engine/status.go}`; tests `/Users/billyz/code/openescapement/internal/cli/cli_test.go` (helpers reused); docs `/Users/billyz/code/openescapement/README.md`, `/Users/billyz/code/openescapement/CHANGELOG.md`.