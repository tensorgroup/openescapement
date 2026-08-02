# Models Guidance: Vendor and Model Pages with Governance Advice — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a top-level **Models** section to the `esc` admin portal that renders curated, sourced, editable guidance on each vendor's models — what they excel at, how to govern them, and example rule-pack fragments to adopt — backed by an embedded, disk-seeded guidance tree.

**Architecture:** A new `internal/guidance` package owns a `models.yaml` registry (strict YAML parse, fixed vendor order) plus per-vendor markdown notes and example fragments, all embedded via `go:embed`. `esc serve` seeds the tree into `<data-dir>/guidance/` create-if-missing; the portal reads from disk on every request with per-file fallback to the embedded copy and a fail-soft banner. The portal gains `/models`, `/models/{vendor}`, and a plain-form editor, reusing the existing shell, `mdHTML`, `hx-disable`, atomic-write, and CSP conventions.

**Tech Stack:** Go 1.24 stdlib, `gopkg.in/yaml.v3`, `html/template`, system `git` (unchanged); no new dependencies, no client JS beyond existing htmx (the editor is plain forms).

## Global Constraints

Every task's requirements implicitly include this section. Values copied verbatim from `AGENTS.md` and the spec.

- **Single external dependency:** `gopkg.in/yaml.v3` only. Everything else stdlib. Adding any dependency is forbidden here.
- **No external network requests at runtime.** Guidance is embedded + disk-backed; the portal never fetches vendor docs. (Live vendor research happens only during content-authoring tasks, by the human/agent, via WebSearch — never in shipped code.)
- **Embedded + disk-backed.** The embedded tree is the seed; `<data-dir>/guidance/` is the source of truth once seeded. Portal reads disk on every request, falls back to embedded per-file.
- **Fail-soft, not fail-closed.** Missing/unparseable guidance files fall back to the embedded copy and surface a banner naming the broken file. Nothing downstream executes this content, so sync-style fail-closed is wrong here.
- **Fixed vendor order (determinism):** `anthropic, openai, google, kimi, deepseek, grok`, always, regardless of file order.
- **The portal never creates paths from user input.** Editor writes are restricted to the fixed file set derived from the registry/embedded tree; the `file` param is validated against that set and against the URL vendor. `models.yaml` is NOT editable from the portal.
- **Atomic writes:** temp file + rename, mode `0o644`.
- **No em-dashes in any authored doc or guidance text** (notes, examples, README, CHANGELOG, spec line). Use commas, parentheses, or restructure. (This is a repo writing convention; existing source em-dashes in unrelated code are out of scope.)
- **Gates before "done":** `gofmt -w .`, `go vet ./...`, `go test ./...` all clean. Tests build real temp dirs, no mocks. `ESC_CACHE_DIR` unaffected.
- **Conventional commits, no AI trailers.** No `Co-Authored-By: Claude`, no "Generated with" lines. Never commit unless the executing workflow reaches its commit step.

---

## File Structure

**New package `internal/guidance/`:**
- `models.go` — registry types, `ParseRegistry`, enum/vendor validation, canonical ordering, `//go:embed models`.
- `load.go` — `Set` (disk-backed loader), `Load`, `ReadFile`, `KnownFiles`, `WriteFile`, `VendorForModel`, `Seed`.
- `models_test.go`, `load_test.go`, `checklist_test.go` — table tests, fallback/seed/write tests, §6 content-acceptance checklist.
- `models/` — the embedded tree (seed): `models.yaml`, `anthropic.md`, `openai.md`, `google.md`, `kimi.md`, `deepseek.md`, `grok.md`, `examples/<vendor>/*.md`.

**Portal (`internal/portal/web/`):**
- `models.go` (new) — handlers `handleModels`, `handleModelVendor`, `handleModelEdit`, `handleModelEditSave`; data structs; helpers `findVendor`, `fileBelongsToVendor`, `urlHost`; `Server.loadGuidance`.
- `models_test.go` (new) — page render, anchors, edit round-trip, fallback banner, full-page-without-HX checks.
- `server.go` (modify) — add `GuidanceDir` field, register `host` funcmap, add page names, register routes.
- `templates/layout.html`, `templates/overview.html`, `templates/usage.html` (modify); `templates/models.html`, `templates/model_vendor.html`, `templates/model_edit.html` (new).
- `degrade_test.go`, `overview.go`/`usage.go` sections of `server.go` (modify) — linking + route coverage.

**CLI:** `internal/cli/serve.go` (modify) — seed guidance, set `GuidanceDir`.

**Docs:** `ai-governance-product-spec.md`, `README.md`, `CHANGELOG.md`.

---

### Task 1: guidance registry — parser and embedded tree

**Files:**
- Create: `internal/guidance/models.go`
- Create: `internal/guidance/models/models.yaml`
- Create: `internal/guidance/models/{anthropic,openai,google,kimi,deepseek,grok}.md` (stubs)
- Create: `internal/guidance/models/examples/anthropic/model-routing.md`
- Test: `internal/guidance/models_test.go`

**Interfaces:**
- Produces:
  - `guidance.VendorOrder []string` = `{"anthropic","openai","google","kimi","deepseek","grok"}`
  - `type Model struct { ID, Name, Tier string; Roles []string; Status string; Docs []string; Verified string }` (yaml tags: `id,name,tier,roles,status,docs,verified`)
  - `type Vendor struct { Key, Name string; Models []Model }` (yaml: `key,name,models`)
  - `type Registry struct { Vendors []Vendor }` (yaml: `vendors`)
  - `func ParseRegistry(data []byte) (*Registry, error)` — strict (`KnownFields(true)`), validates enums + `verified` date `2006-01-02`, rejects unknown/duplicate vendor keys, returns vendors in `VendorOrder`, normalizes `Vendor.Name` from the canonical map.
  - `var embeddedModels embed.FS` rooted at `models/…`.

- [ ] **Step 1: Create the embedded `models.yaml` seed**

`internal/guidance/models/models.yaml`:

```yaml
vendors:
  - key: anthropic
    name: Anthropic
    models:
      - id: claude-opus-5
        name: Claude Opus 5
        tier: frontier
        roles: [planning, plan-check, review]
        status: current
        docs: [https://docs.claude.com/en/docs/about-claude/models]
        verified: 2026-08-02
      - id: claude-sonnet-5
        name: Claude Sonnet 5
        tier: mid
        roles: [coding, review]
        status: current
        docs: [https://docs.claude.com/en/docs/about-claude/models]
        verified: 2026-08-02
      - id: claude-haiku-4-5
        name: Claude Haiku 4.5
        tier: fast
        roles: [bulk]
        status: current
        docs: [https://docs.claude.com/en/docs/about-claude/models]
        verified: 2026-08-02
  - key: openai
    name: OpenAI
    models:
      - id: gpt-5.2
        name: GPT-5.2
        tier: frontier
        roles: [planning, coding, review]
        status: current
        docs: [https://platform.openai.com/docs/models]
        verified: 2026-08-02
  - key: google
    name: Google
    models:
      - id: gemini-3-pro
        name: Gemini 3 Pro
        tier: frontier
        roles: [planning, coding]
        status: current
        docs: [https://ai.google.dev/gemini-api/docs/models]
        verified: 2026-08-02
  - key: kimi
    name: Kimi
    models: []
  - key: deepseek
    name: Deepseek
    models: []
  - key: grok
    name: Grok
    models: []
```

- [ ] **Step 2: Create six stub notes and one starter example**

Each `internal/guidance/models/<vendor>.md` (replace `<Vendor>` per file; content tasks 8-11 replace these). Example `anthropic.md`:

```markdown
# Anthropic

Guidance pending live-source verification.

## Sources

- https://docs.claude.com/en/docs/about-claude/models
```

`openai.md` Sources line: `- https://platform.openai.com/docs/models`; `google.md`: `- https://ai.google.dev/gemini-api/docs/models`; `kimi.md`: `- https://github.com/MoonshotAI/kimi-code/blob/main/docs/en/customization/agents.md`; `deepseek.md`: `- https://api-docs.deepseek.com/`; `grok.md`: `- https://docs.x.ai/build/features/project-rules`. Every stub MUST contain a `## Sources` heading.

`internal/guidance/models/examples/anthropic/model-routing.md`:

```markdown
---
targets: [claude, agents]
---
# Model routing

- Use a frontier-tier model for planning, plan-checking, and review.
- Use a mid-tier model for day-to-day coding.
- Use a fast-tier model for bulk or mechanical edits.
```

- [ ] **Step 3: Write the failing parser test**

`internal/guidance/models_test.go`:

```go
package guidance

import "testing"

func TestParseEmbeddedRegistry(t *testing.T) {
	data, err := embeddedModels.ReadFile("models/models.yaml")
	if err != nil {
		t.Fatal(err)
	}
	reg, err := ParseRegistry(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var keys []string
	for _, v := range reg.Vendors {
		keys = append(keys, v.Key)
	}
	want := []string{"anthropic", "openai", "google", "kimi", "deepseek", "grok"}
	if len(keys) != len(want) {
		t.Fatalf("vendors = %v, want %v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("order[%d] = %q, want %q", i, keys[i], want[i])
		}
	}
}

func TestParseRegistryReordersAndRejects(t *testing.T) {
	valid := "vendors:\n  - {key: google, name: Google, models: []}\n  - {key: anthropic, name: Anthropic, models: []}\n"
	reg, err := ParseRegistry([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	if reg.Vendors[0].Key != "anthropic" || reg.Vendors[1].Key != "google" {
		t.Fatalf("not reordered: %v", reg.Vendors)
	}
	cases := map[string]string{
		"unknown vendor": "vendors:\n  - {key: acme, name: Acme, models: []}\n",
		"unknown field":  "vendors:\n  - {key: anthropic, name: A, bogus: 1, models: []}\n",
		"bad tier":       "vendors:\n  - key: anthropic\n    name: A\n    models: [{id: x, name: X, tier: huge, roles: [coding], status: current, docs: [], verified: 2026-08-02}]\n",
		"bad role":       "vendors:\n  - key: anthropic\n    name: A\n    models: [{id: x, name: X, tier: mid, roles: [wizard], status: current, docs: [], verified: 2026-08-02}]\n",
		"bad status":     "vendors:\n  - key: anthropic\n    name: A\n    models: [{id: x, name: X, tier: mid, roles: [coding], status: retired, docs: [], verified: 2026-08-02}]\n",
		"bad date":       "vendors:\n  - key: anthropic\n    name: A\n    models: [{id: x, name: X, tier: mid, roles: [coding], status: current, docs: [], verified: yesterday}]\n",
	}
	for name, src := range cases {
		if _, err := ParseRegistry([]byte(src)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
```

- [ ] **Step 4: Run — expect FAIL (package/symbols undefined)**

Run: `go test ./internal/guidance/...`
Expected: FAIL (undefined: `embeddedModels`, `ParseRegistry`).

- [ ] **Step 5: Implement `models.go`**

```go
// Package guidance owns the curated, sourced model-guidance tree: a strict
// models.yaml registry plus per-vendor markdown notes and example rule-pack
// fragments. The embedded copy is the seed; esc serve seeds it to
// <data-dir>/guidance/ and the portal reads that on every request.
package guidance

import (
	"bytes"
	"embed"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed models
var embeddedModels embed.FS

// VendorOrder is the fixed display order (ICP-relevant vendors first). The
// registry always renders vendors in this order regardless of file order.
var VendorOrder = []string{"anthropic", "openai", "google", "kimi", "deepseek", "grok"}

var vendorName = map[string]string{
	"anthropic": "Anthropic", "openai": "OpenAI", "google": "Google",
	"kimi": "Kimi", "deepseek": "Deepseek", "grok": "Grok",
}

var (
	validTier   = map[string]bool{"frontier": true, "mid": true, "fast": true}
	validRole   = map[string]bool{"planning": true, "plan-check": true, "review": true, "coding": true, "bulk": true}
	validStatus = map[string]bool{"current": true, "legacy": true}
)

// Model is one model's registry entry. ID matches usage-telemetry model ids.
type Model struct {
	ID       string   `yaml:"id"`
	Name     string   `yaml:"name"`
	Tier     string   `yaml:"tier"`
	Roles    []string `yaml:"roles"`
	Status   string   `yaml:"status"`
	Docs     []string `yaml:"docs"`
	Verified string   `yaml:"verified"`
}

// Vendor groups a vendor's models. Name is normalized from the canonical map.
type Vendor struct {
	Key    string  `yaml:"key"`
	Name   string  `yaml:"name"`
	Models []Model `yaml:"models"`
}

// Registry is the parsed models.yaml.
type Registry struct {
	Vendors []Vendor `yaml:"vendors"`
}

// ParseRegistry strictly parses and validates models.yaml, returning vendors
// in VendorOrder. Unknown fields, unknown/duplicate vendor keys, invalid
// enums, and unparseable verified dates are errors.
func ParseRegistry(data []byte) (*Registry, error) {
	var reg Registry
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&reg); err != nil {
		return nil, fmt.Errorf("guidance: parsing models.yaml: %v", err)
	}
	seen := map[string]bool{}
	byKey := map[string]Vendor{}
	for _, v := range reg.Vendors {
		if _, ok := vendorName[v.Key]; !ok {
			return nil, fmt.Errorf("guidance: unknown vendor %q", v.Key)
		}
		if seen[v.Key] {
			return nil, fmt.Errorf("guidance: duplicate vendor %q", v.Key)
		}
		seen[v.Key] = true
		for _, m := range v.Models {
			if m.ID == "" {
				return nil, fmt.Errorf("guidance: vendor %s: model missing id", v.Key)
			}
			if !validTier[m.Tier] {
				return nil, fmt.Errorf("guidance: model %s: invalid tier %q", m.ID, m.Tier)
			}
			if !validStatus[m.Status] {
				return nil, fmt.Errorf("guidance: model %s: invalid status %q", m.ID, m.Status)
			}
			for _, role := range m.Roles {
				if !validRole[role] {
					return nil, fmt.Errorf("guidance: model %s: invalid role %q", m.ID, role)
				}
			}
			if _, err := time.Parse("2006-01-02", m.Verified); err != nil {
				return nil, fmt.Errorf("guidance: model %s: invalid verified date %q", m.ID, m.Verified)
			}
		}
		v.Name = vendorName[v.Key]
		byKey[v.Key] = v
	}
	ordered := make([]Vendor, 0, len(byKey))
	for _, key := range VendorOrder {
		if v, ok := byKey[key]; ok {
			ordered = append(ordered, v)
		}
	}
	reg.Vendors = ordered
	return &reg, nil
}
```

- [ ] **Step 6: Run — expect PASS**

Run: `go test ./internal/guidance/...`
Expected: PASS.

- [ ] **Step 7: gofmt/vet, then commit**

```bash
gofmt -w internal/guidance
go vet ./internal/guidance/...
git add internal/guidance
git commit -m "feat(guidance): models.yaml registry parser and embedded tree"
```

---

### Task 2: disk-backed loader, seeding, and §6 content checklist

**Files:**
- Create: `internal/guidance/load.go`
- Test: `internal/guidance/load_test.go`, `internal/guidance/checklist_test.go`

**Interfaces:**
- Consumes: `Registry`, `Vendor`, `Model`, `ParseRegistry`, `embeddedModels`, `VendorOrder` (Task 1).
- Produces:
  - `type Set struct { Registry *Registry; Degraded []string; dir string }`
  - `func Load(dir string) *Set` — never returns error (fail-soft). `dir==""` = embedded-only, `Degraded` empty. If `dir` set and `models.yaml` is missing or unparseable, falls back to embedded and appends `"models.yaml"` to `Degraded`.
  - `func (s *Set) ReadFile(rel string) (content []byte, degraded bool, err error)` — `rel` must be in `KnownFiles()`; disk-first, embedded fallback (sets `degraded=true`).
  - `func (s *Set) KnownFiles() map[string]bool` — vendor notes (`<key>.md`) plus every `examples/**` file from the embedded tree. `models.yaml` is NOT included (not portal-editable).
  - `func (s *Set) ExampleFiles(vendorKey string) []string` — sorted rel paths `examples/<vendor>/*.md`.
  - `func (s *Set) WriteFile(rel string, content []byte) error` — validates `rel` in `KnownFiles()`, requires `dir != ""`, atomic temp+rename `0o644`.
  - `func (s *Set) VendorForModel(id string) (vendorKey string, ok bool)`
  - `func Seed(dataDir string) error` — writes the embedded tree into `<dataDir>/guidance/`, create-if-missing (existing files untouched).

- [ ] **Step 1: Write failing loader/seed/checklist tests**

`internal/guidance/load_test.go`:

```go
package guidance

import (
	"os"
	"path/filepath"
	"testing"
)

func seedTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := Seed(dir); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "guidance")
}

func TestSeedCreatesTreeAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := Seed(dir); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(dir, "guidance", "anthropic.md")
	if err := os.WriteFile(f, []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Seed(dir); err != nil { // second seed must not clobber
		t.Fatal(err)
	}
	got, _ := os.ReadFile(f)
	if string(got) != "edited" {
		t.Fatalf("seed clobbered an existing file: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "guidance", "models.yaml")); err != nil {
		t.Fatalf("models.yaml not seeded: %v", err)
	}
}

func TestLoadDiskAndFallback(t *testing.T) {
	gdir := seedTemp(t)
	s := Load(gdir)
	if len(s.Degraded) != 0 {
		t.Fatalf("clean load degraded: %v", s.Degraded)
	}
	if _, ok := s.VendorForModel("claude-sonnet-5"); !ok {
		t.Fatal("seeded model id not found in registry")
	}
	// Corrupt models.yaml on disk: registry falls back, banner records it.
	if err := os.WriteFile(filepath.Join(gdir, "models.yaml"), []byte("vendors: [oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	s = Load(gdir)
	if len(s.Degraded) != 1 || s.Degraded[0] != "models.yaml" {
		t.Fatalf("expected degraded models.yaml, got %v", s.Degraded)
	}
	if _, ok := s.VendorForModel("claude-sonnet-5"); !ok {
		t.Fatal("fallback registry missing seeded model")
	}
}

func TestReadFileFallbackAndWriteRoundTrip(t *testing.T) {
	gdir := seedTemp(t)
	s := Load(gdir)
	if err := s.WriteFile("anthropic.md", []byte("# Anthropic\n\n## Sources\n- x\n")); err != nil {
		t.Fatal(err)
	}
	got, deg, err := s.ReadFile("anthropic.md")
	if err != nil || deg {
		t.Fatalf("read after write: deg=%v err=%v", deg, err)
	}
	if string(got) == "" || string(got)[0] != '#' {
		t.Fatalf("unexpected content %q", got)
	}
	// Removing the disk copy triggers embedded fallback (degraded=true).
	if err := os.Remove(filepath.Join(gdir, "anthropic.md")); err != nil {
		t.Fatal(err)
	}
	if _, deg, _ := s.ReadFile("anthropic.md"); !deg {
		t.Fatal("expected degraded read after removing disk file")
	}
}

func TestWriteFileRejectsUnknownPath(t *testing.T) {
	s := Load(seedTemp(t))
	for _, bad := range []string{"models.yaml", "../escape.md", "unknown.md", "examples/anthropic/../../x"} {
		if err := s.WriteFile(bad, []byte("x")); err == nil {
			t.Errorf("WriteFile(%q) should have been rejected", bad)
		}
	}
}
```

`internal/guidance/checklist_test.go`:

```go
package guidance

import (
	"strings"
	"testing"
)

// seededTelemetryModels mirrors internal/portal/seed/seed.go modelOrder. Keep
// in sync when the demo seed's models change (spec §6 content acceptance).
var seededTelemetryModels = []string{
	"claude-sonnet-5", "claude-opus-5", "claude-haiku-4-5", "gpt-5.2", "gemini-3-pro",
}

func TestEverySeededModelHasRegistryEntry(t *testing.T) {
	s := Load("") // embedded
	for _, id := range seededTelemetryModels {
		if _, ok := s.VendorForModel(id); !ok {
			t.Errorf("seeded telemetry model %q missing from registry", id)
		}
	}
}

func TestEveryVendorNoteHasSources(t *testing.T) {
	s := Load("")
	for _, key := range VendorOrder {
		note, _, err := s.ReadFile(key + ".md")
		if err != nil {
			t.Fatalf("note %s.md: %v", key, err)
		}
		if !strings.Contains(string(note), "## Sources") {
			t.Errorf("%s.md missing a Sources section", key)
		}
	}
}
```

- [ ] **Step 2: Run — expect FAIL (loader symbols undefined)**

Run: `go test ./internal/guidance/...`
Expected: FAIL (undefined `Load`, `Seed`, `Set`, methods).

- [ ] **Step 3: Implement `load.go`**

```go
package guidance

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Set is a disk-backed view of the guidance tree. dir is <data-dir>/guidance
// ("" = embedded only). Degraded names on-disk files that were missing or
// unparseable and fell back to the embedded copy, for the portal banner.
type Set struct {
	Registry *Registry
	Degraded []string
	dir      string
}

// Load reads the registry from disk (falling back to embedded) and returns a
// Set that reads notes and examples per request. Never fails: a broken
// content directory a human hand-edits must not take the portal down.
func Load(dir string) *Set {
	s := &Set{dir: dir}
	reg, degraded := loadRegistry(dir)
	s.Registry = reg
	if degraded {
		s.Degraded = append(s.Degraded, "models.yaml")
	}
	return s
}

func embeddedRegistry() *Registry {
	b, _ := embeddedModels.ReadFile("models/models.yaml")
	reg, _ := ParseRegistry(b) // embedded copy is validated in tests
	return reg
}

func loadRegistry(dir string) (*Registry, bool) {
	if dir != "" {
		if b, err := os.ReadFile(filepath.Join(dir, "models.yaml")); err == nil {
			if reg, perr := ParseRegistry(b); perr == nil {
				return reg, false
			}
		}
		return embeddedRegistry(), true // missing or unparseable on disk
	}
	return embeddedRegistry(), false
}

// KnownFiles is the fixed set of portal-editable guidance files: vendor notes
// plus every embedded example fragment. models.yaml is deliberately excluded.
func (s *Set) KnownFiles() map[string]bool {
	m := make(map[string]bool, len(VendorOrder))
	for _, key := range VendorOrder {
		m[key+".md"] = true
	}
	_ = fs.WalkDir(embeddedModels, "models/examples", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		m[strings.TrimPrefix(p, "models/")] = true
		return nil
	})
	return m
}

// ExampleFiles returns the vendor's example fragment rel paths, sorted.
func (s *Set) ExampleFiles(vendorKey string) []string {
	var out []string
	entries, _ := embeddedModels.ReadDir("models/examples/" + vendorKey)
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, "examples/"+vendorKey+"/"+e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// ReadFile returns a guidance file's content, disk-first with embedded
// fallback. rel must be a known file. degraded is true when the embedded copy
// was used because the disk copy was absent.
func (s *Set) ReadFile(rel string) ([]byte, bool, error) {
	if !s.KnownFiles()[rel] {
		return nil, false, fmt.Errorf("guidance: unknown file %q", rel)
	}
	if s.dir != "" {
		if b, err := os.ReadFile(filepath.Join(s.dir, filepath.FromSlash(rel))); err == nil {
			return b, false, nil
		}
		if b, err := embeddedModels.ReadFile("models/" + rel); err == nil {
			return b, true, nil
		}
		return nil, false, fmt.Errorf("guidance: %s not found", rel)
	}
	b, err := embeddedModels.ReadFile("models/" + rel)
	return b, false, err
}

// WriteFile atomically writes a known guidance file to disk. It never creates
// paths from user input: rel must be in KnownFiles, and dir must be set.
func (s *Set) WriteFile(rel string, content []byte) error {
	if !s.KnownFiles()[rel] {
		return fmt.Errorf("guidance: unknown file %q", rel)
	}
	if s.dir == "" {
		return fmt.Errorf("guidance: no data directory configured")
	}
	dst := filepath.Join(s.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".guidance-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, dst)
}

// VendorForModel finds the vendor key owning a model id.
func (s *Set) VendorForModel(id string) (string, bool) {
	for _, v := range s.Registry.Vendors {
		for _, m := range v.Models {
			if m.ID == id {
				return v.Key, true
			}
		}
	}
	return "", false
}

// Seed writes the embedded guidance tree into <dataDir>/guidance/,
// create-if-missing: existing files are left untouched (same posture as the
// demo repo seed; delete the directory to re-seed after an upgrade).
func Seed(dataDir string) error {
	root := filepath.Join(dataDir, "guidance")
	return fs.WalkDir(embeddedModels, "models", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "models" {
			return nil
		}
		rel := strings.TrimPrefix(p, "models/")
		dst := filepath.Join(root, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		if _, statErr := os.Stat(dst); statErr == nil {
			return nil // create-if-missing
		}
		b, readErr := embeddedModels.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, b, 0o644)
	})
}
```

- [ ] **Step 4: Run — expect PASS**

Run: `go test ./internal/guidance/...`
Expected: PASS (all loader, seed, and checklist tests green).

- [ ] **Step 5: gofmt/vet, commit**

```bash
gofmt -w internal/guidance
go vet ./internal/guidance/...
git add internal/guidance
git commit -m "feat(guidance): disk-backed loader, seeding, content checklist"
```

---

### Task 3: portal wiring, seeding on serve, and the /models overview page

**Files:**
- Modify: `internal/portal/web/server.go` (Server struct, `New` funcmap, `pageNames`, routes, `loadGuidance`, `urlHost`)
- Create: `internal/portal/web/models.go` (`handleModels`, data structs, `findVendor`)
- Create: `internal/portal/web/templates/models.html`
- Modify: `internal/portal/web/templates/layout.html` (nav)
- Modify: `internal/cli/serve.go` (seed + `GuidanceDir`)
- Create: `internal/portal/web/models_test.go`

**Interfaces:**
- Consumes: `guidance.Load`, `guidance.Seed`, `guidance.Set`, `guidance.Vendor`, `guidance.Registry`, `guidance.VendorOrder` (Tasks 1-2).
- Produces:
  - `Server.GuidanceDir string` field (default `""`; set by `cmdServe`).
  - `func (s *Server) loadGuidance() *guidance.Set` = `guidance.Load(s.GuidanceDir)`.
  - `func urlHost(raw string) string` — template func `host`, returns URL host or the raw string.
  - `func findVendor(reg *guidance.Registry, key string) (guidance.Vendor, bool)`.
  - Route `GET /models` → `handleModels`; nav key `"models"`.
  - `type modelsData struct { layoutData; Vendors []guidance.Vendor; Degraded []string }`.

- [ ] **Step 1: Write failing overview-page test**

`internal/portal/web/models_test.go`:

```go
package web

import (
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/guidance"
	"github.com/tensorgroup/openescapement/internal/portal/store"
)

func newTestServerWithGuidance(t *testing.T) *Server {
	t.Helper()
	dataDir := t.TempDir()
	if err := guidance.Seed(dataDir); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := New(st, nil, "", "test")
	s.GuidanceDir = dataDir + "/guidance"
	return s
}

func TestModelsOverviewListsEveryVendor(t *testing.T) {
	h := newTestServerWithGuidance(t).Handler()
	body := get(t, h, "/models", nil).Body.String()
	for _, want := range []string{"Anthropic", "OpenAI", "Google", "Kimi", "Deepseek", "Grok",
		`href="/models/anthropic#claude-opus-5"`, `href="/models/anthropic"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("/models missing %q", want)
		}
	}
	// Fixed vendor order.
	if strings.Index(body, "Anthropic") > strings.Index(body, "OpenAI") {
		t.Fatal("vendor order not Anthropic before OpenAI")
	}
	if !strings.Contains(get(t, h, "/", nil).Body.String(), `href="/models"`) {
		t.Fatal("sidebar missing Models nav")
	}
}
```

- [ ] **Step 2: Run — expect FAIL (route/field undefined)**

Run: `go test ./internal/portal/web/ -run TestModelsOverview`
Expected: FAIL (`GuidanceDir` undefined; `/models` 404).

- [ ] **Step 3: Add the `GuidanceDir` field, funcmap, page names, route, helpers**

In `server.go`, add to the `Server` struct (after `Version string`):

```go
	// GuidanceDir is <data-dir>/guidance ("" = embedded guidance only, used
	// by tests and any caller that has not seeded a data dir).
	GuidanceDir string
```

In `New`, extend the layout `FuncMap` with `"host": urlHost`:

```go
	s.layout = template.Must(template.New("layout.html").Funcs(template.FuncMap{
		"abbrev":    abbrevTokens,
		"fmtTime":   fmtLastSync,
		"diffLines": diffLines,
		"host":      urlHost,
	}).ParseFS(templateFS, "templates/layout.html"))
```

Extend `pageNames`:

```go
var pageNames = []string{"overview", "fleet", "packs", "pack", "pack_edit", "usage", "models", "model_vendor", "model_edit"}
```

Add the route in `Handler()` (immediately after the `GET /usage` line, before `POST /api/v1/events`), keeping vendor pages grouped:

```go
	mux.HandleFunc("GET /models", s.handleModels)
	mux.HandleFunc("GET /models/{vendor}", s.handleModelVendor)
	mux.HandleFunc("GET /models/{vendor}/edit", s.handleModelEdit)
	mux.HandleFunc("POST /models/{vendor}/edit", s.handleModelEditSave)
```

(`handleModelVendor`, `handleModelEdit`, `handleModelEditSave` are added in Tasks 4-5; add all four route lines now and stub the three not-yet-implemented handlers in `models.go` Step 4 so the package compiles.)

- [ ] **Step 4: Create `internal/portal/web/models.go`**

```go
package web

import (
	"net/http"
	"net/url"

	"github.com/tensorgroup/openescapement/internal/guidance"
)

// loadGuidance reads the guidance tree from disk on every request (embedded
// fallback), matching the spec's hand-editable content posture.
func (s *Server) loadGuidance() *guidance.Set { return guidance.Load(s.GuidanceDir) }

// urlHost returns a URL's host for compact display, or the raw string if it
// does not parse. Registered as the "host" template func.
func urlHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	return u.Host
}

func findVendor(reg *guidance.Registry, key string) (guidance.Vendor, bool) {
	for _, v := range reg.Vendors {
		if v.Key == key {
			return v, true
		}
	}
	return guidance.Vendor{}, false
}

// modelsData is the /models overview page's data.
type modelsData struct {
	layoutData
	Vendors  []guidance.Vendor
	Degraded []string
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	g := s.loadGuidance()
	s.render(w, "models", modelsData{
		layoutData: s.baseData("models"),
		Vendors:    g.Registry.Vendors,
		Degraded:   g.Degraded,
	})
}

// The vendor page and editor handlers are implemented in Tasks 4-5.
func (s *Server) handleModelVendor(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }
func (s *Server) handleModelEdit(w http.ResponseWriter, r *http.Request)   { http.NotFound(w, r) }
func (s *Server) handleModelEditSave(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}
```

- [ ] **Step 5: Add the Models nav link in `layout.html`**

Between the Rule Packs and Usage links:

```html
    <a href="/packs"{{if eq .Page "packs"}} class="active"{{end}}>Rule Packs</a>
    <a href="/models"{{if eq .Page "models"}} class="active"{{end}}>Models</a>
    <a href="/usage"{{if eq .Page "usage"}} class="active"{{end}}>Usage</a>
```

- [ ] **Step 6: Create `templates/models.html`**

```html
{{define "title"}}Models{{end}}
{{define "explainer"}}Curated, sourced guidance on each vendor's models: what they excel at, how to govern them, and example rule-pack fragments to adopt.{{end}}
{{define "content"}}
{{if .Degraded}}<div class="error">Showing built-in guidance. These on-disk files are missing or unreadable: {{range $i, $f := .Degraded}}{{if $i}}, {{end}}{{$f}}{{end}}.</div>{{end}}
{{range .Vendors}}
{{$v := .Key}}
<section>
  <h2><a href="/models/{{$v}}">{{.Name}}</a></h2>
  {{if .Models}}
  <div class="cards">
    {{range .Models}}
    <a class="card" href="/models/{{$v}}#{{.ID}}">
      <span class="label">{{.Name}} <span class="muted">{{.ID}}</span></span>
      <ul class="chips"><li>{{.Tier}}</li>{{range .Roles}}<li>{{.}}</li>{{end}}</ul>
      <p class="muted">{{.Status}} · verified {{.Verified}}</p>
    </a>
    {{end}}
  </div>
  {{else}}
  <p class="muted">Guidance in progress. <a href="/models/{{$v}}">View notes</a>.</p>
  {{end}}
</section>
{{end}}
{{end}}
```

- [ ] **Step 7: Wire seeding and `GuidanceDir` into `serve.go`**

In `cmdServe`, after `dir` is resolved (after the `dir = filepath.Join(home, ".escapement", "server")` block, before the demo/token branch) add:

```go
	if err := guidance.Seed(dir); err != nil {
		fmt.Fprintf(stderr, "esc: %v\n", err)
		return 4
	}
```

After `srv := web.New(st, mgr, token, Version)`:

```go
	srv.GuidanceDir = filepath.Join(dir, "guidance")
```

Add the import `"github.com/tensorgroup/openescapement/internal/guidance"`.

- [ ] **Step 8: Run — expect PASS**

Run: `go test ./internal/portal/web/ -run TestModelsOverview && go build ./...`
Expected: PASS and clean build.

- [ ] **Step 9: gofmt/vet, commit**

```bash
gofmt -w internal/portal/web internal/cli
go vet ./...
git add internal/portal/web internal/cli
git commit -m "feat(portal): Models overview page, nav, guidance seeding on serve"
```

---

### Task 4: the /models/{vendor} guidance page

**Files:**
- Modify: `internal/portal/web/models.go` (implement `handleModelVendor`, add data structs)
- Create: `internal/portal/web/templates/model_vendor.html`
- Modify: `internal/portal/web/models_test.go` (add vendor-page test)

**Interfaces:**
- Consumes: `Set.ReadFile`, `Set.ExampleFiles`, `Set.Degraded`, `findVendor`, `mdHTML`, `urlHost` (host func).
- Produces:
  - `type exampleView struct { File string; HTML template.HTML; Raw string }`
  - `type modelVendorData struct { layoutData; VendorKey, VendorName string; NoteHTML template.HTML; NoteFile string; Models []guidance.Model; Examples []exampleView; Degraded []string }`

- [ ] **Step 1: Write failing vendor-page test**

Append to `models_test.go`:

```go
func TestModelVendorPageRendersNoteModelsExamplesAnchors(t *testing.T) {
	h := newTestServerWithGuidance(t).Handler()
	body := get(t, h, "/models/anthropic", nil).Body.String()
	for _, want := range []string{
		`id="claude-opus-5"`,                                 // registry anchor
		"claude-sonnet-5",                                    // model id shown
		"hx-disable",                                         // rendered markdown wrapped
		`/models/anthropic/edit?file=anthropic.md`,           // note Edit button
		`/models/anthropic/edit?file=examples/anthropic/model-routing.md`, // example Edit
		"docs.claude.com",                                    // doc host, not full URL
		"<pre>",                                              // copyable raw example
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("/models/anthropic missing %q", want)
		}
	}
	if get(t, h, "/models/nope", nil).Code != 404 {
		t.Fatal("unknown vendor should 404")
	}
}
```

- [ ] **Step 2: Run — expect FAIL (stub returns 404)**

Run: `go test ./internal/portal/web/ -run TestModelVendorPage`
Expected: FAIL.

- [ ] **Step 3: Implement `handleModelVendor`**

Replace the stub `handleModelVendor` in `models.go` and add the imports `"html/template"` and `guidance` (already imported):

```go
type exampleView struct {
	File string
	HTML template.HTML
	Raw  string
}

type modelVendorData struct {
	layoutData
	VendorKey  string
	VendorName string
	NoteHTML   template.HTML
	NoteFile   string
	Models     []guidance.Model
	Examples   []exampleView
	Degraded   []string
}

func (s *Server) handleModelVendor(w http.ResponseWriter, r *http.Request) {
	vendor := r.PathValue("vendor")
	g := s.loadGuidance()
	v, ok := findVendor(g.Registry, vendor)
	if !ok {
		http.NotFound(w, r)
		return
	}
	degraded := append([]string(nil), g.Degraded...)

	noteFile := vendor + ".md"
	note, noteDeg, err := g.ReadFile(noteFile)
	if err != nil {
		serverError(w, err)
		return
	}
	if noteDeg {
		degraded = append(degraded, noteFile)
	}

	var examples []exampleView
	for _, rel := range g.ExampleFiles(vendor) {
		content, deg, err := g.ReadFile(rel)
		if err != nil {
			continue
		}
		if deg {
			degraded = append(degraded, rel)
		}
		examples = append(examples, exampleView{File: rel, HTML: mdHTML(content), Raw: string(content)})
	}

	s.render(w, "model_vendor", modelVendorData{
		layoutData: s.baseData("models"),
		VendorKey:  v.Key,
		VendorName: v.Name,
		NoteHTML:   mdHTML(note),
		NoteFile:   noteFile,
		Models:     v.Models,
		Examples:   examples,
		Degraded:   degraded,
	})
}
```

- [ ] **Step 4: Create `templates/model_vendor.html`**

```html
{{define "title"}}{{.VendorName}}{{end}}
{{define "explainer"}}Model guidance and governance advice for {{.VendorName}}.{{end}}
{{define "content"}}
{{if .Degraded}}<div class="error">Showing built-in guidance. These on-disk files are missing or unreadable: {{range $i, $f := .Degraded}}{{if $i}}, {{end}}{{$f}}{{end}}.</div>{{end}}
<section hx-disable>
  <article class="fragment">
    <h3>Guidance <a href="/models/{{.VendorKey}}/edit?file={{.NoteFile}}">Edit</a></h3>
    {{.NoteHTML}}
  </article>
</section>
<section>
  <h3>Models</h3>
  {{range .Models}}
  <article class="fragment" id="{{.ID}}">
    <h4>{{.Name}} <span class="muted">{{.ID}}</span></h4>
    <ul class="chips"><li>{{.Tier}}</li>{{range .Roles}}<li>{{.}}</li>{{end}}<li>{{.Status}}</li></ul>
    <p class="muted">Verified {{.Verified}}</p>
    {{if .Docs}}<ul>{{range .Docs}}<li><a href="{{.}}" rel="noreferrer">{{host .}}</a></li>{{end}}</ul>{{end}}
  </article>
  {{else}}
  <p class="muted">No models catalogued yet.</p>
  {{end}}
</section>
{{if .Examples}}
<section hx-disable>
  <h3>Example rule-pack fragments</h3>
  {{range .Examples}}
  <article class="fragment">
    <h4>{{.File}} <a href="/models/{{$.VendorKey}}/edit?file={{.File}}">Edit</a></h4>
    {{.HTML}}
    <pre>{{.Raw}}</pre>
  </article>
  {{end}}
</section>
{{end}}
{{end}}
```

- [ ] **Step 5: Run — expect PASS**

Run: `go test ./internal/portal/web/ -run TestModelVendorPage`
Expected: PASS.

- [ ] **Step 6: gofmt/vet, commit**

```bash
gofmt -w internal/portal/web
go vet ./internal/portal/web/...
git add internal/portal/web
git commit -m "feat(portal): per-vendor guidance page with notes, models, examples"
```

---

### Task 5: the guidance editor (GET form + POST atomic save)

**Files:**
- Modify: `internal/portal/web/models.go` (implement `handleModelEdit`, `handleModelEditSave`, `fileBelongsToVendor`)
- Create: `internal/portal/web/templates/model_edit.html`
- Modify: `internal/portal/web/models_test.go` (edit round-trip + path-safety tests)

**Interfaces:**
- Consumes: `Set.ReadFile`, `Set.WriteFile`, `Set.KnownFiles`, `findVendor`.
- Produces:
  - `type modelEditData struct { layoutData; VendorKey, File, Content, Error string }`
  - `func fileBelongsToVendor(g *guidance.Set, vendor, file string) bool` — true only if `file` is a known file AND is `vendor+".md"` or has prefix `"examples/"+vendor+"/"`.

- [ ] **Step 1: Write failing editor tests**

Append to `models_test.go`:

```go
import (
	"net/http/httptest"
	"net/url"
)

func TestModelEditRoundTrip(t *testing.T) {
	s := newTestServerWithGuidance(t)
	h := s.Handler()
	edit := get(t, h, "/models/anthropic/edit?file=anthropic.md", nil).Body.String()
	if !strings.Contains(edit, "<textarea") || !strings.Contains(edit, "hx-disable") == true {
		// editor is a plain form: must NOT be inside hx-disable, must have textarea
	}
	if !strings.Contains(edit, "<textarea") {
		t.Fatalf("edit page missing textarea: %s", edit)
	}
	form := url.Values{
		"file":    {"anthropic.md"},
		"content": {"# Anthropic\n\nUpdated guidance body.\n\n## Sources\n- https://docs.claude.com/\n"},
	}
	req := httptest.NewRequest("POST", "/models/anthropic/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 303 || rr.Header().Get("Location") != "/models/anthropic" {
		t.Fatalf("save: code=%d loc=%s", rr.Code, rr.Header().Get("Location"))
	}
	body := get(t, h, "/models/anthropic", nil).Body.String()
	if !strings.Contains(body, "Updated guidance body.") {
		t.Fatal("next GET did not show the saved edit")
	}
}

func TestModelEditRejectsForeignAndUnknownFiles(t *testing.T) {
	s := newTestServerWithGuidance(t)
	h := s.Handler()
	// models.yaml is not editable; a note under the wrong vendor is rejected.
	for _, q := range []string{"file=models.yaml", "file=openai.md", "file=../secret", "file=examples/openai/x.md"} {
		if get(t, h, "/models/anthropic/edit?"+q, nil).Code != 404 {
			t.Errorf("GET edit %s should 404", q)
		}
	}
	form := url.Values{"file": {"models.yaml"}, "content": {"x"}}
	req := httptest.NewRequest("POST", "/models/anthropic/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 404 {
		t.Fatalf("POST models.yaml should 404, got %d", rr.Code)
	}
}
```

- [ ] **Step 2: Run — expect FAIL (stubs 404 the round-trip)**

Run: `go test ./internal/portal/web/ -run TestModelEdit`
Expected: FAIL (`TestModelEditRoundTrip` gets 404).

- [ ] **Step 3: Implement the editor handlers**

Replace the stub `handleModelEdit`/`handleModelEditSave` in `models.go` and add `"strings"` to imports:

```go
type modelEditData struct {
	layoutData
	VendorKey string
	File      string
	Content   string
	Error     string
}

// fileBelongsToVendor gates the editor to a vendor's own known files, so the
// portal never writes a path assembled from unchecked user input.
func fileBelongsToVendor(g *guidance.Set, vendor, file string) bool {
	if !g.KnownFiles()[file] {
		return false
	}
	return file == vendor+".md" || strings.HasPrefix(file, "examples/"+vendor+"/")
}

func (s *Server) handleModelEdit(w http.ResponseWriter, r *http.Request) {
	vendor := r.PathValue("vendor")
	g := s.loadGuidance()
	if _, ok := findVendor(g.Registry, vendor); !ok {
		http.NotFound(w, r)
		return
	}
	file := r.URL.Query().Get("file")
	if !fileBelongsToVendor(g, vendor, file) {
		http.NotFound(w, r)
		return
	}
	content, _, err := g.ReadFile(file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.render(w, "model_edit", modelEditData{
		layoutData: s.baseData("models"),
		VendorKey:  vendor,
		File:       file,
		Content:    string(content),
	})
}

func (s *Server) handleModelEditSave(w http.ResponseWriter, r *http.Request) {
	vendor := r.PathValue("vendor")
	g := s.loadGuidance()
	if _, ok := findVendor(g.Registry, vendor); !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	file := r.FormValue("file")
	content := r.FormValue("content")
	if !fileBelongsToVendor(g, vendor, file) {
		http.NotFound(w, r)
		return
	}
	if err := g.WriteFile(file, []byte(content)); err != nil {
		s.renderStatus(w, http.StatusUnprocessableEntity, "model_edit", modelEditData{
			layoutData: s.baseData("models"),
			VendorKey:  vendor,
			File:       file,
			Content:    content,
			Error:      err.Error(),
		})
		return
	}
	http.Redirect(w, r, "/models/"+vendor, http.StatusSeeOther)
}
```

- [ ] **Step 4: Create `templates/model_edit.html` (plain full-page form, no htmx)**

```html
{{define "title"}}Edit {{.File}}{{end}}
{{define "explainer"}}Edit guidance for {{.VendorKey}}. Saving writes the file and returns to the rendered view.{{end}}
{{define "content"}}
{{if .Error}}<div class="error">{{.Error}}</div>{{end}}
<h2>{{.VendorKey}} <span class="muted">/ {{.File}}</span></h2>
<form method="post" action="/models/{{.VendorKey}}/edit">
  <input type="hidden" name="file" value="{{.File}}">
  <p><textarea name="content" rows="24" cols="90">{{.Content}}</textarea></p>
  <p>
    <button type="submit">Save</button>
    <a href="/models/{{.VendorKey}}">Cancel</a>
  </p>
</form>
{{end}}
```

- [ ] **Step 5: Run — expect PASS**

Run: `go test ./internal/portal/web/ -run TestModelEdit`
Expected: PASS.

- [ ] **Step 6: gofmt/vet, commit**

```bash
gofmt -w internal/portal/web
go vet ./internal/portal/web/...
git add internal/portal/web
git commit -m "feat(portal): guidance editor with fixed-file-set path safety"
```

---

### Task 6: link model names on Overview and Usage to guidance anchors

**Files:**
- Modify: `internal/portal/web/server.go` (`overviewData`, `handleOverview`, `usageData`, `handleUsage`)
- Modify: `internal/portal/web/templates/overview.html`, `templates/usage.html`
- Modify: `internal/portal/web/models_test.go` (linking test)

**Interfaces:**
- Consumes: `Set.VendorForModel`, `loadGuidance`.
- Produces: `overviewData.ModelVendor map[string]string`, `usageData.ModelVendor map[string]string` (model id -> vendor key; absent ids render unlinked).

- [ ] **Step 1: Write failing linking test**

Append to `models_test.go`:

```go
func TestOverviewAndUsageLinkModelsToGuidance(t *testing.T) {
	// Overview/Usage need seeded telemetry, so drive them through a seeded
	// demo store plus guidance. Reuse the existing overview/usage fixtures'
	// approach: a store with events whose models are in the registry.
	s := newTestServerWithGuidanceAndEvents(t)
	h := s.Handler()
	ov := get(t, h, "/", nil).Body.String()
	if !strings.Contains(ov, `href="/models/anthropic#claude-sonnet-5"`) {
		t.Fatalf("overview chip not linked: %s", ov)
	}
	us := get(t, h, "/usage", nil).Body.String()
	if !strings.Contains(us, `href="/models/anthropic#claude-sonnet-5"`) {
		t.Fatalf("usage row not linked")
	}
}
```

Add the fixture helper (seeds guidance + one `provider_usage` event) to `models_test.go`:

```go
import "time"
import "github.com/tensorgroup/openescapement/internal/portal/store"

func newTestServerWithGuidanceAndEvents(t *testing.T) *Server {
	t.Helper()
	dataDir := t.TempDir()
	if err := guidance.Seed(dataDir); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AppendEvent(store.Event{
		TS:     time.Now().UTC().Add(-time.Hour),
		Kind:   "provider_usage",
		TeamID: "t1",
		Model:  "claude-sonnet-5",
		Tokens: &store.Tokens{Input: 100, Output: 50, CostUSD: 1},
	}); err != nil {
		t.Fatal(err)
	}
	s := New(st, nil, "", "test")
	s.GuidanceDir = dataDir + "/guidance"
	return s
}
```

(If `store.Event`/`store.Tokens` field names differ, mirror `internal/portal/seed/seed.go` usage: `Kind: "provider_usage"`, `Model`, `Tokens: &store.Tokens{Input, Output, CostUSD}`.)

- [ ] **Step 2: Run — expect FAIL**

Run: `go test ./internal/portal/web/ -run TestOverviewAndUsageLink`
Expected: FAIL (`ModelVendor` undefined).

- [ ] **Step 3: Add `ModelVendor` to overview**

In `server.go`, add to `overviewData`:

```go
	ModelVendor map[string]string
```

In `handleOverview`, after `stats := store.Overview(...)` and before building `data`:

```go
	g := s.loadGuidance()
	modelVendor := make(map[string]string, len(stats.Models))
	for _, m := range stats.Models {
		if vk, ok := g.VendorForModel(m); ok {
			modelVendor[m] = vk
		}
	}
```

Set `ModelVendor: modelVendor` in the `overviewData` literal.

- [ ] **Step 4: Add `ModelVendor` to usage**

Add to `usageData`:

```go
	ModelVendor map[string]string
```

In `handleUsage`, after `models` is built (and before assembling `data`):

```go
	g := s.loadGuidance()
	modelVendor := make(map[string]string, len(models))
	for _, m := range models {
		if vk, ok := g.VendorForModel(m); ok {
			modelVendor[m] = vk
		}
	}
	for _, row := range rows {
		if _, done := modelVendor[row.Model]; !done {
			if vk, ok := g.VendorForModel(row.Model); ok {
				modelVendor[row.Model] = vk
			}
		}
	}
```

Set `ModelVendor: modelVendor` in the `usageData` literal.

- [ ] **Step 5: Update templates**

`overview.html` "Models in use" list:

```html
  <ul class="chips">{{range .Stats.Models}}<li>{{$v := index $.ModelVendor .}}{{if $v}}<a href="/models/{{$v}}#{{.}}">{{.}}</a>{{else}}{{.}}{{end}}</li>{{end}}</ul>
```

`usage.html` table row Model cell (inside `{{range .Rows}}`):

```html
      <td>{{$v := index $.ModelVendor .Model}}{{if $v}}<a href="/models/{{$v}}#{{.Model}}">{{.Model}}</a>{{else}}{{.Model}}{{end}}</td>
```

- [ ] **Step 6: Run — expect PASS (linking + no regressions)**

Run: `go test ./internal/portal/web/...`
Expected: PASS. (Existing `overview_test.go`/`usage_test.go` use `strings.Contains(body, "claude-sonnet-5")`, still satisfied when wrapped in `<a>`.)

- [ ] **Step 7: gofmt/vet, commit**

```bash
gofmt -w internal/portal/web
go vet ./internal/portal/web/...
git add internal/portal/web
git commit -m "feat(portal): link model names to guidance anchors on overview and usage"
```

---

### Task 7: extend the degradation route contract to Models pages

**Files:**
- Modify: `internal/portal/web/degrade_test.go`

**Interfaces:** none new (test-only).

- [ ] **Step 1: Add the Models GET routes to `pageRoutes`**

`/models` and `/models/anthropic` render full pages from the embedded guidance even with the default (unseeded) test server (`GuidanceDir == ""`).

```go
var pageRoutes = []string{"/", "/fleet", "/usage", "/packs", "/models", "/models/anthropic"}
```

- [ ] **Step 2: Add a full-page test for the editor GET route**

Append to `degrade_test.go`:

```go
func TestModelEditRendersFullPageWithoutHX(t *testing.T) {
	h := newTestServer(t, "").Handler() // embedded guidance is readable without a data dir
	body := get(t, h, "/models/anthropic/edit?file=anthropic.md", nil).Body.String()
	if !strings.Contains(body, "<html") {
		t.Fatal("edit route must be a full page")
	}
	if !strings.Contains(body, "esc <strong>portal</strong>") {
		t.Fatal("edit route missing sidebar")
	}
	if !strings.Contains(body, "<textarea") {
		t.Fatal("edit route missing textarea")
	}
}
```

- [ ] **Step 3: Run — expect PASS**

Run: `go test ./internal/portal/web/ -run 'Degrade|EveryRoute|ModelEditRendersFullPage'`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/portal/web/degrade_test.go
git commit -m "test(portal): extend degradation contract to Models routes"
```

---

## Content Authoring Tasks (8-11) — shared shape

Tasks 8-11 are **research tasks**: this plan cannot contain the final prose. Each replaces stub note(s) with verified guidance, adds registry entries, and (full-depth vendors only) adds one example fragment.

**Shared vendor-note format** (write each `<vendor>.md` to this skeleton):

```markdown
# <Vendor>

<One or two sentences positioning the vendor's current lineup and where it fits in an agentic SDLC.>

## Models

### <Model name> (<id>) — <tier>

<What this model excels at. Recommended roles. When to reach for it versus a cheaper or stronger sibling.>

## Governance

- <Review-gate advice for high-autonomy frontier models.>
- <Cost controls for expensive models (routing away from frontier for bulk work, spend caps).>
- <Routing policy: which tier for planning/plan-check/review, which for coding, which for bulk.>

## Sources

- <primary URL actually used>
- <primary URL actually used>
```

**Fully-worked MINIATURE example** (illustrative only; produce real, verified content):

```markdown
# ExampleCorp

ExampleCorp ships three tiers: Titan (frontier), Worker (mid), Sprint (fast).

## Models

### Titan 2 (titan-2) — frontier

Strongest reasoning and long-context planning. Reach for it to plan multi-file
changes, to plan-check another agent's plan, and to review diffs for risk.
Expensive; do not use it for mechanical edits.

## Governance

- Require human review gates on any repo where Titan runs with high autonomy.
- Route bulk and mechanical work to Sprint to hold spend down.
- Plan, plan-check, and review on Titan; code on Worker; bulk edits on Sprint.

## Sources

- https://examplecorp.com/docs/models
```

**Mandatory research step (every content task):** Use **WebSearch** to live-verify the vendor's CURRENT model lineup, official names, ids, and positioning against **primary sources** (vendor docs and engineering blogs). No claims from memory. The starting role frame (frontier tier -> planning/plan-check/review; mid -> coding; fast -> bulk) is a hypothesis the research either confirms or corrects per vendor. Every note ends with a `## Sources` list of the URLs actually used.

**Quality bar (every content task):** guidance covers what each model excels at (with the role mapping), governance advice per model class (review gates for high-autonomy frontier models, cost controls for expensive ones, routing policy), and accurate ids that match the registry.

**Global constraints reminder:** no em-dashes anywhere in authored text; single-dependency and no-runtime-network rules are unaffected (research is done by the author, not shipped code).

**Acceptance (every content task):** `go test ./...` green (including the §6 checklist test in `internal/guidance/checklist_test.go`), `gofmt -w .` and `go vet ./...` clean, every touched note ends with `## Sources`, and every registry id referenced in the note exists in `models.yaml`.

---

### Task 8: Anthropic guidance (full depth)

**Files:**
- Modify: `internal/guidance/models/anthropic.md`
- Modify: `internal/guidance/models/models.yaml` (anthropic vendor block)
- Modify/Create: `internal/guidance/models/examples/anthropic/model-routing.md` (replace starter with verified content)

- [ ] **Step 1: Research (WebSearch, primary sources)**

Verify Anthropic's current Claude lineup, official model names and API ids, and per-model positioning against: `claude.com/blog`, `anthropic.com/engineering` (context-engineering series), and Claude Code docs (`docs.claude.com`). **Additionally**, cite this repo's `docs/roadmap/vendor-guidance-tracking.md` Claude 5 entries as sources (the "new rules of context engineering for Claude 5 generation models", claude.com/blog 2026-07-24). Confirm the seeded ids `claude-opus-5`, `claude-sonnet-5`, `claude-haiku-4-5` are the correct current ids or correct them (and update `models.yaml` + the seed reference comment in `checklist_test.go` if an id changes).

- [ ] **Step 2: Rewrite `anthropic.md`** to the shared format with verified prose. The `## Sources` list MUST include the vendor-guidance-tracking Claude 5 entry plus the primary docs URLs used. No em-dashes.

- [ ] **Step 3: Update the anthropic block in `models.yaml`** — confirm/adjust `name`, `tier`, `roles`, `status`, `docs`, and set `verified: 2026-08-02` on each entry. Add any additional current Anthropic models discovered (frontier/mid/fast). Keep `claude-opus-5`, `claude-sonnet-5`, `claude-haiku-4-5` present (they appear in seeded telemetry).

- [ ] **Step 4: Rewrite `examples/anthropic/model-routing.md`** as a real rule-pack fragment (front-matter `targets:` plus markdown) encoding model-routing guidance: plan/plan-check/review on the frontier tier, code on mid, bulk on fast, referencing the verified ids. Must parse as a fragment (valid front-matter block delimited by `---`).

- [ ] **Step 5: Verify acceptance**

Run: `go test ./... && gofmt -l . && go vet ./...`
Expected: tests PASS, `gofmt -l .` prints nothing, vet clean.

- [ ] **Step 6: Commit**

```bash
git add internal/guidance/models
git commit -m "docs(guidance): verified Anthropic model guidance, registry, routing example"
```

---

### Task 9: OpenAI guidance (full depth)

**Files:**
- Modify: `internal/guidance/models/openai.md`
- Modify: `internal/guidance/models/models.yaml` (openai block)
- Create: `internal/guidance/models/examples/openai/model-routing.md`

- [ ] **Step 1: Research (WebSearch)** OpenAI's current model lineup, names, and ids against primary sources: `platform.openai.com/docs/models`, the Codex docs, and the OpenAI cookbook. Confirm/correct the seeded id `gpt-5.2`.

- [ ] **Step 2: Rewrite `openai.md`** to the shared format, verified prose, `## Sources`, no em-dashes.

- [ ] **Step 3: Update the openai block in `models.yaml`** — keep `gpt-5.2` present (seeded telemetry), add other current models with correct `tier`/`roles`/`status`/`docs`, `verified: 2026-08-02`.

- [ ] **Step 4: Create `examples/openai/model-routing.md`** — real fragment with `targets:` front-matter encoding role-based routing across OpenAI tiers.

- [ ] **Step 5: Verify acceptance**

Run: `go test ./... && gofmt -l . && go vet ./...`
Expected: PASS, clean.

- [ ] **Step 6: Commit**

```bash
git add internal/guidance/models
git commit -m "docs(guidance): verified OpenAI model guidance, registry, routing example"
```

---

### Task 10: Google guidance (full depth)

**Files:**
- Modify: `internal/guidance/models/google.md`
- Modify: `internal/guidance/models/models.yaml` (google block)
- Create: `internal/guidance/models/examples/google/model-routing.md`

- [ ] **Step 1: Research (WebSearch)** Google's current Gemini lineup, names, and ids against primary sources: `ai.google.dev/gemini-api/docs/models` and the Gemini CLI / GEMINI.md docs (`github.com/google-gemini/gemini-cli`). Confirm/correct the seeded id `gemini-3-pro`.

- [ ] **Step 2: Rewrite `google.md`** to the shared format, verified prose, `## Sources`, no em-dashes.

- [ ] **Step 3: Update the google block in `models.yaml`** — keep `gemini-3-pro` present, add other current models with correct fields, `verified: 2026-08-02`.

- [ ] **Step 4: Create `examples/google/model-routing.md`** — real fragment with `targets:` front-matter encoding role-based routing across Gemini tiers.

- [ ] **Step 5: Verify acceptance**

Run: `go test ./... && gofmt -l . && go vet ./...`
Expected: PASS, clean.

- [ ] **Step 6: Commit**

```bash
git add internal/guidance/models
git commit -m "docs(guidance): verified Google model guidance, registry, routing example"
```

---

### Task 11: Kimi, Deepseek, and Grok guidance (combined, sparse depth)

**Files:**
- Modify: `internal/guidance/models/{kimi,deepseek,grok}.md`
- Modify: `internal/guidance/models/models.yaml` (kimi, deepseek, grok blocks)
- No example fragments required for these three (sparse depth per spec §1).

- [ ] **Step 1: Research (WebSearch)** the current lineups, names, and ids for Kimi (Moonshot), Deepseek, and Grok (xAI) against primary sources: Kimi Code docs (`github.com/MoonshotAI/kimi-code`), Deepseek API docs (`api-docs.deepseek.com`), and Grok Build / project-rules docs (`docs.x.ai/build`).

- [ ] **Step 2: Rewrite each of `kimi.md`, `deepseek.md`, `grok.md`** to the shared format but at sparse depth: a short positioning note, the model list, governance advice in a few bullets, and a `## Sources` list with the doc links. No em-dashes.

- [ ] **Step 3: Populate the kimi/deepseek/grok blocks in `models.yaml`** — replace the empty `models: []` lists with the verified current models (`id`, `name`, `tier`, `roles`, `status`, `docs`, `verified: 2026-08-02` each). Keep vendor order fixed.

- [ ] **Step 4: Verify acceptance**

Run: `go test ./... && gofmt -l . && go vet ./...`
Expected: PASS, clean. (The `/models` page now shows model cards for all six vendors.)

- [ ] **Step 5: Commit**

```bash
git add internal/guidance/models
git commit -m "docs(guidance): verified Kimi, Deepseek, and Grok model guidance and registry"
```

---

### Task 12: product-spec line, README paragraph, CHANGELOG bullet

**Files:**
- Modify: `ai-governance-product-spec.md` (§4 boundary bullet, line ~89)
- Modify: `README.md` (Models paragraph)
- Modify: `CHANGELOG.md` ([Unreleased] Added)

No em-dashes in any of these edits.

- [ ] **Step 1: Product-spec §4 clarifying sub-line**

Change the `- Not a model evaluation platform` bullet to add a nested sub-line:

```markdown
- Not a model evaluation platform
  - The Models guidance is curated, sourced paved-path pointing (which model class suits planning, plan-checking, review, coding, or bulk work, and how to govern each), not benchmarking or model evaluation.
```

- [ ] **Step 2: README Models paragraph**

In the `### esc serve — the admin portal` section, add a short paragraph after the existing portal description:

```markdown
The portal's **Models** section is curated, sourced guidance on each vendor's models (Anthropic, OpenAI, Google, Kimi, Deepseek, Grok): what each model class is good at, which roles it suits (planning, review, coding, bulk), how to govern it, and example rule-pack fragments to adopt. Every claim carries a source. The guidance ships embedded and is seeded to your data directory on first run, so you can hand-edit it or edit it from the portal.
```

- [ ] **Step 3: CHANGELOG bullet under `## [Unreleased]` -> `### Added`**

```markdown
- Models guidance: a new portal **Models** section with curated, sourced,
  editable guidance per vendor (Anthropic, OpenAI, Google, Kimi, Deepseek,
  Grok). A strict `models.yaml` registry (fixed vendor order), per-vendor
  markdown notes, and example rule-pack fragments ship embedded and are seeded
  to `<data-dir>/guidance/` create-if-missing. The portal reads from disk on
  every request with per-file fallback to the embedded copy and a fail-soft
  banner. Overview chips and usage-table model names link to their vendor page
  anchors. Editor writes are atomic and restricted to the fixed guidance file
  set (`models.yaml` is repo-managed, not portal-editable).
```

- [ ] **Step 4: Verify (no em-dashes, docs build)**

Run: `grep -Rn "—" ai-governance-product-spec.md README.md CHANGELOG.md | grep -i "model\|guidance\|paved" || echo "no em-dashes in new lines"`
Expected: the new lines contain no em-dashes. (Pre-existing em-dashes elsewhere are out of scope.)

- [ ] **Step 5: Commit**

```bash
git add ai-governance-product-spec.md README.md CHANGELOG.md
git commit -m "docs: Models guidance spec boundary line, README, and changelog"
```

---

### Task 13: final verification and demo walkthrough

**Files:** none (verification only).

- [ ] **Step 1: Full gate sweep**

Run: `gofmt -l . && go vet ./... && go test ./...`
Expected: `gofmt -l .` prints nothing, vet clean, all tests PASS (including `internal/guidance` checklist and all portal route tests).

- [ ] **Step 2: Build the binary**

Run: `go build ./cmd/esc`
Expected: builds `esc` with no errors.

- [ ] **Step 3: Demo walkthrough (manual, capture screenshots)**

Run `./esc serve --demo` and walk this controller list, screenshotting each:
1. Sidebar shows **Models** between Rule Packs and Usage; it is `active` on `/models`.
2. `/models` renders all six vendor sections in fixed order (Anthropic, OpenAI, Google, Kimi, Deepseek, Grok), each with model cards showing tier and role chips, status, and verified date.
3. A model card links to `/models/<vendor>#<model-id>` and scrolls to that model's registry entry (anchor present).
4. `/models/anthropic` shows the rendered note (inside `hx-disable`), a per-note **Edit** button, model registry entries with doc links displayed as hosts, and example fragments each shown rendered plus a copyable raw `<pre>` with its own Edit button.
5. Editing a note: **Edit** opens the plain full-page textarea form; **Save** writes the file and redirects back to the rendered view showing the change.
6. Overview "Models in use" chips and the Usage table model names are links to the guidance anchors.
7. Fail-soft: with the server stopped, corrupt `<data-dir>/guidance/models.yaml`, restart, and confirm `/models` renders from the embedded copy with a banner naming `models.yaml`; restore the file.

- [ ] **Step 4: Optional cheap e2e extension**

If an `esc serve` e2e test already exists (search `internal/cli` for a serve smoke test), extend it to `GET /models` and assert a 200 with a vendor name in the body. If none exists, skip (do not add a new server harness for this).

- [ ] **Step 5: Confirm completion**

State explicitly which gate commands were run and their exit status before claiming the feature complete (per `superpowers:verification-before-completion`). Do not commit anything in this task; it is verification only.

---

## Self-Review

**1. Spec coverage.**
- §1 content tree (models.yaml, notes, examples, tiered depth, accuracy rule) -> Tasks 1, 8-11.
- §2 disk-backed/seeding/atomic writes/models.yaml not editable -> Tasks 2, 3, 5.
- §3 pages (/models, /models/{vendor}, editor, anchors, doc hosts, verified dates, hx-disable, raw copy) -> Tasks 3, 4, 5, 6.
- §4 layout/nav/fixed order -> Tasks 1 (order), 3 (nav).
- §5 error handling (fail-soft banner, path safety) -> Tasks 2, 4, 5.
- §6 testing (parse, seed, pages, edit round-trip, fallback banner, degradation gate, content checklist) -> Tasks 1, 2, 4, 5, 7.
- §7 phasing/ordering -> task order 1-13.
- Product-spec line + docs -> Task 12.

**2. Placeholder scan.** All infra tasks carry full Go, full templates, and full test bodies. Content tasks are research tasks by nature; their "complete content" is the shared format skeleton, the worked miniature example, the mandatory WebSearch step, and concrete acceptance/commit commands, per the plan's special content shape. No "TBD"/"add error handling"/"similar to Task N" placeholders in code steps.

**3. Type consistency.** `Set`, `Registry`, `Vendor`, `Model`, `ParseRegistry`, `Load`, `ReadFile`, `WriteFile`, `KnownFiles`, `ExampleFiles`, `VendorForModel`, `Seed`, `Server.GuidanceDir`, `loadGuidance`, `findVendor`, `fileBelongsToVendor`, `urlHost`, `modelsData`, `modelVendorData`, `exampleView`, `modelEditData`, `ModelVendor` are named identically everywhere they appear. Template page names (`models`, `model_vendor`, `model_edit`) match `pageNames` and the `s.render` calls. Route patterns match handler names.

**Known assumptions / open items flagged for the executor:**
- The seeded telemetry ids (`claude-sonnet-5`, `claude-opus-5`, `claude-haiku-4-5`, `gpt-5.2`, `gemini-3-pro`) are copied from `internal/portal/seed/seed.go` `modelOrder` into `checklist_test.go`. If a content task corrects an id (e.g. research shows a different current id), it MUST update both `models.yaml` and this test's `seededTelemetryModels`, and confirm the demo seed still emits ids present in the registry.
- `store.Event`/`store.Tokens` field names in Task 6's fixture are taken from `seed.go`; verify against `internal/portal/store` when writing the helper.
- The editor deliberately uses no htmx (plain form POST), consistent with spec §3; the global CSP already permits `form-action 'self'`, so no CSP change is needed.

---
