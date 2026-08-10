# Skill Vendoring Core (Phase 1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let pack authors vendor external skills with provenance (`esc pack add-skill/update-skill/outdated`), let skills keep their upstream on-disk names via a `skills:` schema union, fail closed on every naming conflict, and report cross-level (project vs `~/.claude/skills`) duplicates — plus the §8 MCP float warning.

**Architecture:** The consumer sync path is unchanged: a vendored skill is an ordinary `KindDir` artifact. New surface is (a) a `SkillEntry` union type in `internal/pack` (plain string = today's `esc-<pack>-<dir>` prefix, object = explicit name), (b) a `sources.yaml` provenance file at the pack root that consumers never read, (c) an `esc pack` authoring command family in `internal/cli` reusing `internal/source.Fetch`/`LsRemoteTags` and the `pack.DirFiles` walk, and (d) two new status signals: an `occupied` managed state for an unmanaged dir squatting a target path, and an informational `duplicate` field on dir findings.

**Tech Stack:** Go 1.24 stdlib + `gopkg.in/yaml.v3` only. System `git` via `internal/source`. No new dependencies (semver comparison is hand-rolled).

## Global Constraints

- Single external dependency policy: `gopkg.in/yaml.v3` only (goldmark is display-only, portal-scoped; nothing in this plan may touch it or add anything else).
- Sentinel errors in `internal/esc` map to exit codes: 0 ok, 1 drift/constraint (`ErrConstraint`), 2 usage (`ErrConfig`), 3 integrity (`ErrSignature`/`ErrLockMismatch`), 4 other (incl. `ErrManifest`, `ErrFetch`, plain errors). Containment refusals are plain errors (exit 4), never `ErrConstraint`.
- Renderer invariants: bytes outside managed regions never modified; nothing written after a verification/constraint failure; all writes atomic; deterministic output.
- Byte preservation is necessary but not sufficient: every new write/reconcile path must consider file shapes (no trailing newline, CRLF, frontmatter, empty file, symlink) and either handle them or fail closed.
- Two orthogonal axes (`managed` × `local`) are never collapsed; a local amendment alone never changes an exit code.
- Vendor copies MUST enumerate files via `pack.DirFiles` — the same walk hashing uses. Never write a second, divergent walk.
- The redaction walk test (`internal/publisher/redact_test.go`) is the privacy enforcement point: any new report field must be metrics-grade (names, hashes, bools, counts) or it fails that test. The cross-level duplicate finding carries ONLY a skill name and a same/differs bool.
- `gofmt -w .`, `go vet ./...`, `go test ./...` must pass before any claim of done. Integration tests build real temp git repos, no mocks; `ESC_CACHE_DIR` isolates the pack cache.
- Commit messages describe the why, drafted from the actual diff. No AI co-authorship trailers.

## Binding spec decisions (do not relitigate)

Vendor-with-provenance; `sources.yaml` at pack root outside skill dirs; upstream names survive via object entries; plain-string entries keep today's prefix exactly (no lockfile churn); conflicts fail closed (plan-time duplicate-path error, add-skill refuses taken names, sync skips an unmanaged dir at a target with warning + status finding and `--force` does NOT override that case); update-skill divergence gate mirrors sync's hand-edit rule; `outdated --check` exits 1; no-tags fallback to default branch head with a warning; resolved commit always recorded; cross-level finding is informational and never affects exit codes. §§6–7 (agents dir, personal skills) are OUT of scope.

## Decisions made by this plan (with rationale)

1. **`sources.yaml` schema:** `schema: 1` plus a `skills:` list of `{name, source, subdir, ref, commit, hash}`. Flat, keyed by on-disk name, sorted on save for deterministic diffs. `source` is the git URL without the `#subdir` suffix; `subdir` is separate so re-fetch logic never re-parses a combined string.
2. **Skill discovery:** a directory is a skill iff it *directly* contains `SKILL.md`. If the fetched dir itself contains `SKILL.md`, it is a single skill named after the subdir's base (or the repo name when there is no subdir). Nested skill dirs are an error (ambiguous). Discovery does not follow symlinked entries; the copy path (`pack.DirFiles`) fails closed on symlinks inside a selected skill.
3. **Duplicate-path sentinels:** two entries inside ONE pack resolving to the same directory name → `esc.ErrManifest` (invalid manifest, exit 4, caught in `Manifest.validate`). Two entries across DIFFERENT configured packs resolving to the same `.claude/skills/` path → `esc.ErrConfig` (exit 2): no single manifest is wrong, the repo's chosen pack combination is. Neither is exit 1 (a CI gate reads exit 1 as routine and self-healing; no sync resolves a collision) nor exit 3 (nothing about pack integrity failed).
4. **`esc pack outdated` output:** rows sorted by skill name, rendered with `text/tabwriter`; columns SKILL / PINNED (ref + short commit) / LATEST / STATE. `--check` returns an `esc.ErrConstraint`-wrapped error when anything is behind (exit 1: exactly the routine CI-gate semantics AGENTS.md assigns to exit 1). A network failure is `esc.ErrFetch` (exit 4) — a dead remote must not read as up to date.
5. **Command layout:** `esc pack <sub>` dispatch in `internal/cli/packcmd.go`; subcommands in `packadd.go`, `packupdate.go`, `packoutdated.go`; shared vendoring helpers in `skillvendor.go`; pack.yaml rewriting in `packyaml.go`. Flags: `add-skill URL[#subdir] [--ref REF] [--only a,b]`; `update-skill [name...] [--all] [--ref REF] [--force]`; `outdated [--check]`. This mirrors the existing convention of keeping authoring/interaction logic in `internal/cli` (`initscan.go`, `initoffer.go`).
6. **Semver comparison:** `internal/source/semver.go` (next to `LsRemoteTags`, its only consumer's data source). Hand-rolled `parseSemver`/`compareSemver`/`HighestSemverTag`, accepting an optional leading `v`, ignoring build metadata, ordering pre-releases below releases per semver §11. Equal versions (e.g. `v1.0.0` vs `1.0.0`) tie-break on the lexicographically larger tag name for determinism.
7. **Ownership marker generalization (forced ripple):** apply.go's orphan-dir removal pass and status.go's orphan-dir pass currently treat the `esc-` prefix on `path.Base` as THE ownership marker. A name-overridden skill (`.claude/skills/brainstorming`) has no prefix, so retirement would exit 4 forever. New rule: escapement may own exactly the paths whose parent is `.claude/skills` (`path.Dir(rel) == ".claude/skills"`). A hostile lockfile is still confined: removal deletes only manifest-listed files, with containment and symlink refusal already run before any of it reads the disk. The existing hash gate (`SkipOrphanDirEdited`) is not part of that containment — the lockfile is attacker-controlled and the hash is deterministic, so a hostile entry can simply carry the correct hash — its real job is declining when a *team member's* edit doesn't match the recorded hash, so that edit isn't silently deleted out from under them. Both passes change together; status must never advise a command Apply refuses.
8. **Occupied gate scope:** the never-silently-adopt rule applies to every `KindDir` artifact with no prior lock entry whose target path already exists — including `esc-`-prefixed ones (previously first sync silently merged into a pre-existing dir; that was silent adoption and is now a skip). New `SkipCause` `SkipUnmanagedDirAtTarget`, new managed `State` `Occupied` (`"occupied"`). Bare `esc status` stays exit 0 (drift family); `esc status --check` exits 1 via the existing `Clean()` path.
9. **Cross-level duplicate placement:** a new `Duplicate *SkillDuplicate` field ON the existing dir-artifact `Finding` (json `duplicate,omitempty`), not a new finding row — a new row with a non-`InSync` state would flip `Clean()` and change exit codes, which the spec forbids. Fields: `Name string`, `Same bool` — metrics-grade only.
10. **No-tags fallback recording:** `ref: HEAD` is recorded in `sources.yaml` (the requested ref was "whatever the default branch is"); `commit` carries the exact resolved SHA, so the pin is exact per spec.
11. **MCP float warning surface:** `pack.LintMCP` populates a new `Pack.Warnings []string` in `pack.Load`; only the `esc pack` subcommands print them (stderr, `warning:` prefix). Sync/status stay silent: a consumer cannot fix an author's floating pin, and warning them every sync is noise directed at the wrong person.

## File structure

| File | Responsibility |
|---|---|
| `internal/pack/skillentry.go` (new) | `SkillEntry` union type, YAML (un)marshal, `DirName` |
| `internal/pack/pack.go` (modify) | `Manifest.Skills []SkillEntry`, validation incl. within-pack dup |
| `internal/pack/sources.go` (new) | `sources.yaml` load/save/upsert |
| `internal/pack/lint.go` (new) | `LintMCP` float warnings; `Pack.Warnings` |
| `internal/engine/engine.go` (modify) | name resolution, cross-pack duplicate-path error |
| `internal/engine/apply.go` (modify) | ownership rule, occupied skip gate, new `SkipCause` |
| `internal/engine/status.go` (modify) | `Occupied` state, ownership rule mirror, home-dir duplicate scan, `SkillDuplicate` |
| `internal/portal/publish/publish.go` (modify) | adapt to `[]SkillEntry` |
| `internal/source/semver.go` (new) | semver tag selection |
| `internal/cli/cli.go` (modify) | `pack` dispatch in `Run`, usage text, skipHint case, status suffix |
| `internal/cli/packcmd.go` (new) | `esc pack` subcommand dispatch, shared ref resolution, warning printing |
| `internal/cli/packyaml.go` (new) | comment-preserving pack.yaml append |
| `internal/cli/skillvendor.go` (new) | discovery, vendor copy, diffstat |
| `internal/cli/packadd.go` (new) | `esc pack add-skill` |
| `internal/cli/packupdate.go` (new) | `esc pack update-skill` |
| `internal/cli/packoutdated.go` (new) | `esc pack outdated` |
| `internal/publisher/redact_test.go` (modify) | duplicate field survives below content |

---

### Task 1: `SkillEntry` schema union in `internal/pack`

**Files:**
- Create: `internal/pack/skillentry.go`
- Modify: `internal/pack/pack.go` (Manifest.Skills type at line ~28; validation loop at lines ~166–177)
- Modify: `internal/engine/engine.go:227` (range over Skills — minimal type adaptation only; naming behavior unchanged in this task)
- Modify: `internal/portal/publish/publish.go:132-134` (appends Skills paths to a fragment list)
- Test: `internal/pack/skillentry_test.go`

**Interfaces:**
- Consumes: `validName` regexp, `safeRel`, `esc.ErrManifest` (all existing in `internal/pack`).
- Produces (later tasks rely on these exact names):
  - `type SkillEntry struct { Path string; Name string }` — `Name == ""` means legacy prefix naming.
  - `func (s *SkillEntry) UnmarshalYAML(value *yaml.Node) error`
  - `func (s SkillEntry) MarshalYAML() (any, error)`
  - `func (s SkillEntry) DirName(packName string) string` — resolved on-disk directory name.
  - `Manifest.Skills` becomes `[]SkillEntry`.

- [ ] **Step 1: Write the failing test**

```go
// internal/pack/skillentry_test.go
package pack

import (
	"bytes"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func decodeSkills(t *testing.T, doc string) []SkillEntry {
	t.Helper()
	var m struct {
		Skills []SkillEntry `yaml:"skills"`
	}
	dec := yaml.NewDecoder(strings.NewReader(doc))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return m.Skills
}

func TestSkillEntryPlainStringKeepsLegacyNaming(t *testing.T) {
	got := decodeSkills(t, "skills:\n  - skills/vault-usage\n")
	if len(got) != 1 || got[0].Path != "skills/vault-usage" || got[0].Name != "" {
		t.Fatalf("plain entry parsed wrong: %+v", got)
	}
	if dn := got[0].DirName("acme-org"); dn != "esc-acme-org-vault-usage" {
		t.Fatalf("legacy DirName = %q, want esc-acme-org-vault-usage", dn)
	}
}

func TestSkillEntryObjectSetsName(t *testing.T) {
	got := decodeSkills(t, "skills:\n  - path: skills/brainstorming\n    name: brainstorming\n")
	if len(got) != 1 || got[0].Path != "skills/brainstorming" || got[0].Name != "brainstorming" {
		t.Fatalf("object entry parsed wrong: %+v", got)
	}
	if dn := got[0].DirName("acme-org"); dn != "brainstorming" {
		t.Fatalf("override DirName = %q, want brainstorming", dn)
	}
}

func TestSkillEntryRejectsUnknownKeysAndBadShapes(t *testing.T) {
	for _, doc := range []string{
		"skills:\n  - path: skills/x\n    nmae: typo\n", // unknown key
		"skills:\n  - name: only-a-name\n",              // path required
		"skills:\n  - [not, a, mapping]\n",              // wrong node kind
	} {
		var m struct {
			Skills []SkillEntry `yaml:"skills"`
		}
		if err := yaml.Unmarshal([]byte(doc), &m); err == nil {
			t.Errorf("decode accepted %q", doc)
		}
	}
}

func TestSkillEntryMarshalRoundTrips(t *testing.T) {
	in := []SkillEntry{{Path: "skills/vault-usage"}, {Path: "skills/brainstorming", Name: "brainstorming"}}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(map[string][]SkillEntry{"skills": in}); err != nil {
		t.Fatal(err)
	}
	enc.Close()
	out := buf.String()
	if !strings.Contains(out, "- skills/vault-usage\n") {
		t.Errorf("plain entry did not marshal as a plain string:\n%s", out)
	}
	if !strings.Contains(out, "path: skills/brainstorming") || !strings.Contains(out, "name: brainstorming") {
		t.Errorf("object entry did not marshal as a mapping:\n%s", out)
	}
	got := decodeSkills(t, out)
	if len(got) != 2 || got[0] != in[0] || got[1] != in[1] {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/pack/ -run TestSkillEntry -v`
Expected: FAIL (compile error: `undefined: SkillEntry`)

- [ ] **Step 3: Implement `SkillEntry`**

```go
// internal/pack/skillentry.go
package pack

import (
	"fmt"
	"path"

	"gopkg.in/yaml.v3"
)

// SkillEntry is one skills: entry in pack.yaml — a union type. A plain
// string entry ("skills/vault-usage") keeps today's on-disk naming exactly,
// prefix included, so existing packs and lockfiles see no change. An object
// entry ({path: skills/brainstorming, name: brainstorming}) sets the on-disk
// directory name explicitly, which is how a vendored suite whose skills
// reference each other by canonical name survives syncing (spec §3).
type SkillEntry struct {
	Path string // pack-relative skill directory (slash-separated)
	Name string // explicit on-disk name; "" = legacy esc-<pack>-<base> prefix
}

// DirName resolves the on-disk directory name under .claude/skills/.
func (s SkillEntry) DirName(packName string) string {
	if s.Name != "" {
		return s.Name
	}
	return "esc-" + packName + "-" + path.Base(s.Path)
}

// UnmarshalYAML accepts the union: a scalar (legacy) or a {path, name}
// mapping. Unknown mapping keys are rejected by hand because a custom
// unmarshaller bypasses the decoder's KnownFields enforcement.
func (s *SkillEntry) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		return value.Decode(&s.Path)
	case yaml.MappingNode:
		for i := 0; i+1 < len(value.Content); i += 2 {
			key := value.Content[i].Value
			switch key {
			case "path":
				if err := value.Content[i+1].Decode(&s.Path); err != nil {
					return err
				}
			case "name":
				if err := value.Content[i+1].Decode(&s.Name); err != nil {
					return err
				}
			default:
				return fmt.Errorf("skill entry: unknown key %q (want path, name)", key)
			}
		}
		if s.Path == "" {
			return fmt.Errorf("skill entry: path is required")
		}
		return nil
	default:
		return fmt.Errorf("skill entry: must be a string or a {path, name} mapping")
	}
}

// MarshalYAML emits the narrowest form that round-trips: a plain string for
// legacy entries, a mapping when a name override is present. A {path} object
// with no name is semantically identical to the plain string and marshals as
// one.
func (s SkillEntry) MarshalYAML() (any, error) {
	if s.Name == "" {
		return s.Path, nil
	}
	return struct {
		Path string `yaml:"path"`
		Name string `yaml:"name"`
	}{s.Path, s.Name}, nil
}
```

- [ ] **Step 4: Change `Manifest.Skills` and validation**

In `internal/pack/pack.go`, change the field:

```go
	Skills      []SkillEntry   `yaml:"skills"`
```

Replace the skills validation loop (lines ~166–177) with:

```go
	seenSkillDir := map[string]string{}
	for _, e := range m.Skills {
		rel := e.Path
		if err := safeRel(rel); err != nil {
			return fail("skill %v", err)
		}
		info, err := os.Stat(filepath.Join(dir, rel))
		if err != nil || !info.IsDir() {
			return fail("skill %s: not a directory", rel)
		}
		if !validName.MatchString(filepath.Base(rel)) {
			return fail("skill %s: directory name must match %s", rel, validName)
		}
		if e.Name != "" && !validName.MatchString(e.Name) {
			return fail("skill %s: name %q must match %s (it becomes a filesystem path component)", rel, e.Name, validName)
		}
		// Conflicts are first-class and fail closed (spec §3): two entries in
		// one pack resolving to the same on-disk directory is a manifest
		// authoring error, caught before anything is planned or written.
		dn := e.DirName(m.Name)
		if prev, ok := seenSkillDir[dn]; ok {
			return fail("skills %q and %q both resolve to directory %q", prev, rel, dn)
		}
		seenSkillDir[dn] = rel
	}
```

- [ ] **Step 5: Adapt the two compile-breaking consumers (behavior unchanged)**

`internal/engine/engine.go` line ~227 — change only the range variable use; naming stays byte-identical in this task (the override lands in Task 2):

```go
				for _, e := range p.Manifest.Skills {
					rel := e.Path
					src := filepath.Join(p.Dir, filepath.FromSlash(rel))
```

(the rest of the block — `DirFiles`, `DirHashOf`, `name := "esc-" + ...`, artifact append — is untouched; `filepath.Base(rel)` still names it).

`internal/portal/publish/publish.go` lines ~132–134 — it appends skill paths to a fragments list:

```go
	frags := make([]string, 0, len(p.Manifest.Rules)+len(p.Manifest.Skills))
	frags = append(frags, p.Manifest.Rules...)
	for _, e := range p.Manifest.Skills {
		frags = append(frags, e.Path)
	}
```

- [ ] **Step 6: Add a validation test for the within-pack duplicate**

Append to `internal/pack/skillentry_test.go` (build the pack on disk — validate stats the dirs):

```go
func TestManifestRejectsDuplicateResolvedSkillDirs(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{"skills/a/SKILL.md", "other/brainstorming/SKILL.md"} {
		writeSkillFile(t, dir, p)
	}
	manifest := `schema: 1
name: acme
version: 1.0.0
skills:
  - path: skills/a
    name: brainstorming
  - path: other/brainstorming
    name: brainstorming
`
	if err := os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil || !errors.Is(err, esc.ErrManifest) {
		t.Fatalf("want ErrManifest for duplicate resolved dir, got %v", err)
	}
}

func writeSkillFile(t *testing.T, root, rel string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte("---\nname: x\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
```

(add `errors`, `os`, `path/filepath`, and `internal/esc` to the test imports).

- [ ] **Step 7: Run the package tests, then the full suite**

Run: `go test ./internal/pack/ -v` — expected: PASS.
Run: `gofmt -w . && go vet ./... && go test ./...` — expected: PASS. The whole existing suite passing is the "no lockfile churn for plain entries" proof: every fixture manifest uses plain strings and every golden/e2e assertion still holds.

- [ ] **Step 8: Commit**

```bash
git add internal/pack/skillentry.go internal/pack/skillentry_test.go internal/pack/pack.go internal/engine/engine.go internal/portal/publish/publish.go
git commit -m "feat(pack): skills entries become a string|{path,name} union

A plain string keeps the esc-<pack>-<dir> prefix byte-for-byte so existing
packs and lockfiles see no change; an object entry names the on-disk
directory explicitly, validated like every other path component. Two
entries in one pack resolving to the same directory fail manifest
validation outright — conflicts are first-class and fail closed."
```

---

### Task 2: Name override + cross-pack duplicate-path error in the engine

**Files:**
- Modify: `internal/engine/engine.go` (the `render.TargetSkills` case, lines ~225–250)
- Test: `internal/cli/skillname_test.go`

**Interfaces:**
- Consumes: `pack.SkillEntry.DirName(packName string) string` (Task 1), `esc.ErrConfig`.
- Produces: `.claude/skills/<DirName>` artifact paths; a plan-time error `"%w: skill directory %s is declared by packs %s and %s..."` wrapping `esc.ErrConfig`. Later tasks rely on the artifact path being `path.Join(".claude", "skills", e.DirName(p.Manifest.Name))`.

- [ ] **Step 1: Write the failing tests**

These use local unsigned packs (no git needed): real temp repos, no mocks, exercising the full `Run` path. The `run(t, root, args...)` helper is the existing one in `internal/cli/cli_test.go`.

```go
// internal/cli/skillname_test.go
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeLocalPack writes a minimal unsigned local pack under root/<dir> with
// one named skill, and returns the pack-relative source ("./<dir>").
func writeLocalPack(t *testing.T, root, dir, packName, skillYAML string) string {
	t.Helper()
	writeFiles(t, root, map[string]string{
		dir + "/pack.yaml": "schema: 1\nname: " + packName + "\nversion: 1.0.0\n" + skillYAML,
		dir + "/skills/brainstorming/SKILL.md": "---\nname: brainstorming\n---\n\nUpstream method.\n",
	})
	return "./" + dir
}

func localConfig(sources ...string) string {
	var b strings.Builder
	b.WriteString("schema: 1\npacks:\n")
	for _, s := range sources {
		b.WriteString("  - source: " + s + "\n    ref: \"\"\n    trust: unsigned\n")
	}
	return b.String()
}

func TestNamedSkillSyncsToUnprefixedPath(t *testing.T) {
	root := t.TempDir()
	src := writeLocalPack(t, root, "policy", "acme",
		"skills:\n  - path: skills/brainstorming\n    name: brainstorming\n")
	writeFiles(t, root, map[string]string{".escapement/config.yaml": localConfig(src)})
	runEsc(t, root, "sync")
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "brainstorming", "SKILL.md")); err != nil {
		t.Fatalf("named skill did not land at upstream name: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "esc-acme-brainstorming")); !os.IsNotExist(err) {
		t.Fatalf("prefixed directory must not exist for a named entry")
	}
}

func TestCrossPackSkillPathCollisionFailsPlan(t *testing.T) {
	root := t.TempDir()
	a := writeLocalPack(t, root, "pa", "alpha",
		"skills:\n  - path: skills/brainstorming\n    name: brainstorming\n")
	b := writeLocalPack(t, root, "pb", "beta",
		"skills:\n  - path: skills/brainstorming\n    name: brainstorming\n")
	writeFiles(t, root, map[string]string{".escapement/config.yaml": localConfig(a, b)})
	out, code := runEscOut(t, root, "sync")
	if code != 2 {
		t.Fatalf("collision must be a config error (exit 2), got %d:\n%s", code, out)
	}
	if !strings.Contains(out, ".claude/skills/brainstorming") ||
		!strings.Contains(out, "alpha") || !strings.Contains(out, "beta") {
		t.Fatalf("error must name the path and both packs:\n%s", out)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run 'TestNamedSkill|TestCrossPack' -v`
Expected: FAIL — the first lands at the prefixed path, the second syncs at exit 0.

- [ ] **Step 3: Implement in `engine.go`**

Replace the `render.TargetSkills` case body's loop with (keeping the DirFiles/DirHashOf lines exactly as they are):

```go
		case render.TargetSkills:
			// Conflicts are first-class and fail closed (spec §3): two entries
			// across all configured packs resolving to the same on-disk path is
			// a config-combination problem — no single manifest is invalid, so
			// this is ErrConfig (exit 2), not ErrManifest, and certainly not
			// exit 1: a CI gate reads exit 1 as routine and self-healing, and
			// no amount of syncing resolves a name collision.
			skillOwner := map[string]string{} // target path -> pack name
			for _, p := range res.PackObjs {
				for _, e := range p.Manifest.Skills {
					src := filepath.Join(p.Dir, filepath.FromSlash(e.Path))
					files, err := pack.DirFiles(src)
					if err != nil {
						return nil, err
					}
					h, err := pack.DirHashOf(src, files)
					if err != nil {
						return nil, err
					}
					name := e.DirName(p.Manifest.Name)
					target := path.Join(".claude", "skills", name)
					if owner, ok := skillOwner[target]; ok {
						return nil, fmt.Errorf("%w: skill directory %s is declared by packs %s and %s; rename one entry (skills: {path, name}) so they do not collide",
							esc.ErrConfig, target, owner, p.Manifest.Name)
					}
					skillOwner[target] = p.Manifest.Name
					res.Artifacts = append(res.Artifacts, Artifact{
						Path: target,
						Kind: KindDir, Hash: h, SrcDir: src, Files: files,
					})
				}
			}
```

Keep the existing comment about the single `DirFiles` walk. Add `"path"` to engine.go's imports if not present. Note `path.Join` produces the same slash path `filepath.ToSlash(filepath.Join(...))` did.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run 'TestNamedSkill|TestCrossPack' -v` — expected: PASS.
Run: `go test ./internal/engine/ ./internal/cli/` — expected: PASS (prefixed fixtures unchanged).

- [ ] **Step 5: Commit**

```bash
git add internal/engine/engine.go internal/cli/skillname_test.go
git commit -m "feat(engine): honor skill name overrides; collide-fail across packs

An object skills: entry lands at .claude/skills/<name> so a vendored suite
keeps its canonical cross-referencing names. Two packs resolving to one
path fail the plan with ErrConfig (exit 2): the combination is the repo's
choice, no single manifest is wrong, and nothing self-heals a collision."
```

---

### Task 3: Generalize the skill-dir ownership rule (apply + status)

**Files:**
- Modify: `internal/engine/apply.go` (orphan-dir removal pass, `strings.HasPrefix(path.Base(prev.Path), "esc-")` at line ~333)
- Modify: `internal/engine/status.go` (orphan-dir pass, same check at line ~225)
- Test: `internal/cli/skillname_test.go` (extend)

**Interfaces:**
- Produces: `func ownedSkillPath(rel string) bool` in `internal/engine/apply.go` (same package as status.go), used by both passes.

- [ ] **Step 1: Write the failing test**

Append to `internal/cli/skillname_test.go`:

```go
func TestNamedSkillRetiresCleanly(t *testing.T) {
	root := t.TempDir()
	src := writeLocalPack(t, root, "policy", "acme",
		"skills:\n  - path: skills/brainstorming\n    name: brainstorming\n")
	writeFiles(t, root, map[string]string{".escapement/config.yaml": localConfig(src)})
	runEsc(t, root, "sync")
	// Drop the skill from the pack and sync again: the unprefixed directory
	// must retire exactly like an esc- prefixed one — via the manifest-file
	// removal pass, not a refusal.
	writeFiles(t, root, map[string]string{
		"policy/pack.yaml": "schema: 1\nname: acme\nversion: 1.0.1\n",
	})
	runEsc(t, root, "sync")
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "brainstorming")); !os.IsNotExist(err) {
		t.Fatalf("retired named skill dir still present (err=%v)", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestNamedSkillRetiresCleanly -v`
Expected: FAIL — second sync exits 4 with `refusing to remove ".claude/skills/brainstorming": not an escapement-owned directory`.

- [ ] **Step 3: Implement the shared ownership predicate**

In `internal/engine/apply.go`, above the removal pass:

```go
// ownedSkillPath reports whether a lockfile dir entry's repo-relative path is
// one escapement could have written: exactly one path element below
// .claude/skills. This replaces the old "esc-" name-prefix marker, which the
// skill name override (pack.SkillEntry.Name) made unsound: a named skill has
// no prefix, so retirement would refuse forever. Decided on the relative
// path, never the absolute one, for the reason documented at the removal
// pass.
//
// What actually confines a hostile lockfile here is this check plus
// manifest-only deletion — removal only ever deletes the paths prev.Files
// lists — with containment and symlink refusal already run before any of
// this reads the disk. The DirHashOf gate below (SkipOrphanDirEdited) is
// not part of that containment chain: the lockfile is attacker-controlled
// and DirHashOf is deterministic, so a hostile entry can simply carry the
// correct hash for whatever it names and pass the gate. Its real job is
// different — catching the case where a *team member* edited a
// pack-provided file after escapement last wrote it, so that edit isn't
// silently deleted out from under them on retirement.
func ownedSkillPath(rel string) bool {
	return path.Dir(path.Clean(rel)) == ".claude/skills"
}
```

In apply.go's removal pass, replace the prefix check:

```go
			if !ownedSkillPath(prev.Path) {
				return nil, fmt.Errorf("refusing to remove %q: not an escapement-owned directory", prev.Path)
			}
```

In status.go's orphan-dir pass, replace `if !strings.HasPrefix(path.Base(la.Path), "esc-") {` with `if !ownedSkillPath(la.Path) {` (message unchanged). Remove the now-unused `strings` usage only if nothing else in the file needs it (it does — leave imports alone; `go vet` will confirm).

- [ ] **Step 4: Run tests**

Run: `go test ./internal/cli/ -run TestNamedSkill -v && go test ./internal/engine/ ./internal/cli/`
Expected: PASS, including the existing hostile-lockfile tests (`hostile_test.go`, `status_orphan_test.go`) — a lock entry outside `.claude/skills` still refuses. If any hostile test asserted the exact "esc-" behavior for a path *inside* `.claude/skills` without a matching hash, verify the hash gate now declines it (skip, exit 0) and update the assertion to the new, still-fail-closed behavior — the file is never deleted either way.

- [ ] **Step 5: Commit**

```bash
git add internal/engine/apply.go internal/engine/status.go internal/cli/skillname_test.go
git commit -m "fix(engine): ownership marker is the parent dir, not the esc- prefix

Named skills carry no prefix, so the old marker made their retirement exit
4 forever and status advise a command sync refuses. Ownership is now
'exactly one element below .claude/skills'; a hostile lockfile stays
confined because removal is manifest-file-only and the recorded-hash gate
declines anything escapement did not write."
```

---

### Task 4: The occupied-target gate — sync skips, `--force` does not override, status reports

**Files:**
- Modify: `internal/engine/apply.go` (Apply loop, new skip before the force-gated block; new `SkipCause`)
- Modify: `internal/engine/status.go` (classify `KindDir`, new `State`)
- Modify: `internal/cli/cli.go` (`skipHint`)
- Test: `internal/cli/skilldir_occupied_test.go`

**Interfaces:**
- Produces: `SkipUnmanagedDirAtTarget SkipCause = "unmanaged-dir-at-target"`; `Occupied State = "occupied"`. Task 5's status test and any report consumer see `"managed": "occupied"`.

- [ ] **Step 1: Write the failing tests**

```go
// internal/cli/skilldir_occupied_test.go
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupOccupied(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	src := writeLocalPack(t, root, "policy", "acme",
		"skills:\n  - path: skills/brainstorming\n    name: brainstorming\n")
	writeFiles(t, root, map[string]string{
		".escapement/config.yaml":                localConfig(src),
		".claude/skills/brainstorming/SKILL.md": "the user's own hand-installed copy\n",
	})
	return root
}

func TestSyncSkipsUnmanagedDirAtTarget(t *testing.T) {
	root := setupOccupied(t)
	for _, args := range [][]string{{"sync"}, {"sync", "--force"}} {
		out, code := runEscOut(t, root, args...)
		if code != 0 {
			t.Fatalf("esc %v exited %d:\n%s", args, code, out)
		}
		if !strings.Contains(out, "skipped .claude/skills/brainstorming") {
			t.Fatalf("esc %v printed no skip warning:\n%s", args, out)
		}
		got, err := os.ReadFile(filepath.Join(root, ".claude", "skills", "brainstorming", "SKILL.md"))
		if err != nil || string(got) != "the user's own hand-installed copy\n" {
			t.Fatalf("esc %v touched the unmanaged directory: %q, %v", args, got, err)
		}
	}
	// No adoption: the lockfile must carry no entry for the occupied path.
	lock, err := os.ReadFile(filepath.Join(root, ".escapement", "escapement.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(lock), ".claude/skills/brainstorming") {
		t.Fatalf("lockfile adopted an unmanaged directory:\n%s", lock)
	}
}

func TestStatusReportsOccupiedAndCheckGates(t *testing.T) {
	root := setupOccupied(t)
	runEscOut(t, root, "sync")
	out, code := runEscOut(t, root, "status")
	if code != 0 {
		t.Fatalf("bare status must exit 0 on drift-family findings, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "occupied") {
		t.Fatalf("status did not report the occupied state:\n%s", out)
	}
	if _, code := runEscOut(t, root, "status", "--check"); code != 1 {
		t.Fatalf("status --check must gate on occupied, got %d", code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run 'Occupied|TestSyncSkipsUnmanaged' -v`
Expected: FAIL — today sync silently merges into the pre-existing directory (silent adoption) and overwrites SKILL.md... actually the pack provides its own SKILL.md, so the user's copy is clobbered. That clobbering is the bug this task removes.

- [ ] **Step 3: Implement the Apply gate**

In `internal/engine/apply.go`, add the cause constant:

```go
	// SkipUnmanagedDirAtTarget: a directory (or file) escapement never wrote
	// already occupies a KindDir artifact's target path — there is no prior
	// lock entry for it. Writing into it would be silent adoption of, and
	// destruction inside, something the team owns, so sync declines. force
	// does NOT override this cause: force is consent to overwrite
	// escapement's own content, and this content never was.
	SkipUnmanagedDirAtTarget SkipCause = "unmanaged-dir-at-target"
```

In the Apply loop, inside the existing `if a.Kind == KindDir {` block (which sets `desiredDirs`), after `desiredDirs[a.Path] = true` and BEFORE the `if !force {` block:

```go
			if prevLock.Artifact(a.Path) == nil {
				// os.Lstat, not Stat: a symlink at the target was already
				// refused above (refuseSymlinks); this catches a plain dir or
				// file. Checked outside the force gate deliberately.
				if _, statErr := os.Lstat(abs); statErr == nil {
					res.Skipped = append(res.Skipped, Skipped{
						Subject: a.Path, Kind: a.Kind, Cause: SkipUnmanagedDirAtTarget,
						Reason:       "an unmanaged directory already occupies this path",
						ExpectedHash: a.Hash,
					})
					continue // no lock entry appended: nothing was adopted
				}
			}
```

- [ ] **Step 4: Implement the status state**

In `internal/engine/status.go`, add to the `State` consts:

```go
	Occupied           State = "occupied"
```

In `classify`'s `case KindDir:` branch, before the existing `os.Stat` missing check:

```go
		if locked == nil {
			// Mirror of Apply's SkipUnmanagedDirAtTarget gate, and it must
			// stay a mirror: status must never promise a sync that Apply then
			// declines. No prior lock entry + something on disk = a directory
			// escapement never wrote occupying the target.
			if _, lerr := os.Lstat(abs); lerr == nil {
				return Finding{Subject: a.Path, Kind: a.Kind, State: Occupied, Local: LocalNone,
					Detail: "an unmanaged directory occupies this path; escapement will not adopt or overwrite it — move it aside, then run `esc sync` (`--force` does not override this)"}
			}
		}
```

- [ ] **Step 5: Add the skip hint**

In `internal/cli/cli.go`'s `skipHint`:

```go
	case engine.SkipUnmanagedDirAtTarget:
		return "escapement never owned that directory · move it aside, then `esc sync` (`--force` does not override this)"
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/cli/ -run 'Occupied|TestSyncSkipsUnmanaged' -v` then `go test ./...`
Expected: PASS. Existing first-sync tests keep passing because no fixture pre-creates a target skill dir; if one does, that fixture was relying on silent adoption — fix the fixture, not the gate.

- [ ] **Step 7: Commit**

```bash
git add internal/engine/apply.go internal/engine/status.go internal/cli/cli.go internal/cli/skilldir_occupied_test.go
git commit -m "feat(engine): never silently adopt a directory at a skill target

A KindDir artifact with no prior lock entry whose target already exists is
skipped (exit 0, stderr warning, no lockfile adoption) and reported by
status as 'occupied', gating --check. --force does not override: force is
consent to overwrite escapement's own content, and an unmanaged directory
never was. Adoption requires the user to move the directory aside."
```

---

### Task 5: Cross-level duplicate finding (project skill also in `~/.claude/skills`)

**Files:**
- Modify: `internal/engine/status.go` (`Finding` struct, `SkillDuplicate` type, home scan in `Status`)
- Modify: `internal/cli/cli.go` (human status suffix)
- Modify: `internal/publisher/redact_test.go` (fixture + survival assertion)
- Test: `internal/cli/skilldup_test.go`

**Interfaces:**
- Produces:
  - `type SkillDuplicate struct { Name string `json:"name"`; Same bool `json:"same"` }`
  - `Finding.Duplicate *SkillDuplicate `json:"duplicate,omitempty"``

- [ ] **Step 1: Write the failing tests**

```go
// internal/cli/skilldup_test.go
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupDupHome syncs a named local-pack skill, then plants a copy of it under
// a fake $HOME/.claude/skills. os.UserHomeDir honors $HOME on unix.
func setupDupHome(t *testing.T, mutate bool) string {
	t.Helper()
	root := t.TempDir()
	src := writeLocalPack(t, root, "policy", "acme",
		"skills:\n  - path: skills/brainstorming\n    name: brainstorming\n")
	writeFiles(t, root, map[string]string{".escapement/config.yaml": localConfig(src)})
	runEsc(t, root, "sync")
	home := t.TempDir()
	t.Setenv("HOME", home)
	content := "---\nname: brainstorming\n---\n\nUpstream method.\n"
	if mutate {
		content += "user's local tweak\n"
	}
	writeFiles(t, home, map[string]string{".claude/skills/brainstorming/SKILL.md": content})
	return root
}

func TestStatusReportsUserLevelDuplicate(t *testing.T) {
	root := setupDupHome(t, false)
	out, code := runEscOut(t, root, "status")
	if code != 0 {
		t.Fatalf("informational finding must not change the exit code, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "also installed at user level (same content)") {
		t.Fatalf("missing duplicate notice:\n%s", out)
	}
	if _, code := runEscOut(t, root, "status", "--check"); code != 0 {
		t.Fatal("duplicate finding must never gate --check")
	}
}

func TestStatusReportsUserLevelDuplicateDiffers(t *testing.T) {
	root := setupDupHome(t, true)
	out, _ := runEscOut(t, root, "status")
	if !strings.Contains(out, "also installed at user level (differs)") {
		t.Fatalf("missing differs notice:\n%s", out)
	}
}

func TestStatusJSONDuplicateIsMetricsGradeOnly(t *testing.T) {
	root := setupDupHome(t, true)
	out, _ := runEscOut(t, root, "status", "--json")
	if !strings.Contains(out, `"duplicate"`) || !strings.Contains(out, `"same": false`) {
		t.Fatalf("json report missing duplicate payload:\n%s", out)
	}
	if strings.Contains(out, "user's local tweak") {
		t.Fatalf("duplicate finding leaked skill content:\n%s", out)
	}
}
```

(Import only `strings` and `testing` in this file; `setupDupHome` uses `writeFiles`/`runEsc` from the existing test helpers.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run TestStatusReportsUserLevel -v`
Expected: FAIL — no notice printed.

- [ ] **Step 3: Implement in `status.go`**

Add the type and field:

```go
// SkillDuplicate reports that a managed project skill's on-disk name also
// exists under the user-level skills directory (~/.claude/skills). Sync runs
// per machine, so this only ever describes the home directory of whoever ran
// it — information, not drift: it never affects any exit code (the same
// stance as the local axis). Name and Same are deliberately the entire
// payload — both metrics-grade — and the redaction walk test enforces that
// no content-shaped field ever grows here.
type SkillDuplicate struct {
	Name string `json:"name"`
	Same bool   `json:"same"`
}
```

On `Finding` (after `Alteration`):

```go
	Duplicate  *SkillDuplicate `json:"duplicate,omitempty"`
```

In `Status`, after the `for _, a := range plan.Artifacts { ... classify ... }` loop, insert:

```go
	// Cross-level duplicates (spec §5): informational only. Attached to the
	// existing dir finding rather than emitted as a new finding row because a
	// new row with any non-InSync state would flip Clean() and change exit
	// codes, which this signal must never do.
	if home, herr := os.UserHomeDir(); herr == nil {
		hashByPath := map[string]string{}
		for _, a := range plan.Artifacts {
			if a.Kind == KindDir {
				hashByPath[a.Path] = a.Hash
			}
		}
		for i := range res.Findings {
			f := &res.Findings[i]
			wantHash, ok := hashByPath[f.Subject]
			if !ok {
				continue
			}
			name := path.Base(f.Subject)
			userDir := filepath.Join(home, ".claude", "skills", name)
			info, serr := os.Lstat(userDir) // never follow a symlinked user dir
			if serr != nil {
				continue
			}
			d := &SkillDuplicate{Name: name}
			if info.IsDir() {
				// Same means the user copy is byte-identical to the
				// pack-provided content: DirHash covers the whole user tree,
				// DirHashOf(a.Files) is the pack tree, and an identical vendor
				// copy hashes equal by construction. Any error (a symlink
				// inside the user dir, unreadable file) reports differs — the
				// cautious answer, and no error text leaves the machine.
				if hh, herr2 := pack.DirHash(userDir); herr2 == nil {
					d.Same = hh == wantHash
				}
			}
			f.Duplicate = d
		}
	}
```

- [ ] **Step 4: Human output suffix in `cmdStatus`**

In `internal/cli/cli.go`, inside the findings print loop, after the `Amendment` suffix switch:

```go
			if f.Duplicate != nil {
				if f.Duplicate.Same {
					suffix += "  ·  also installed at user level (same content)"
				} else {
					suffix += "  ·  also installed at user level (differs)"
				}
			}
```

- [ ] **Step 5: Extend the redaction walk test**

In `internal/publisher/redact_test.go`, add to the dir finding in `fixtureReport()` (after `Alteration`):

```go
				Duplicate: &engine.SkillDuplicate{Name: "team-skill", Same: false},
```

And in `assertStripsSensitiveFields`, after the `items1` assertion:

```go
	// duplicate is metrics-grade by design (a name and a bool) and MUST keep
	// surviving below content; folding it into redaction would trade the
	// whole cross-level signal away.
	dup1, ok := f1["duplicate"].(map[string]any)
	if !ok || dup1["name"] != "team-skill" || dup1["same"] != false {
		t.Errorf("level %q: duplicate finding did not survive: got %v", level, f1["duplicate"])
	}
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/cli/ -run TestStatusReportsUserLevel -v && go test ./internal/publisher/ ./internal/engine/ && go test ./...`
Expected: PASS. If `report_golden_test.go` goldens fail, inspect the diff: only a `duplicate` object on dir findings (and only when a fixture HOME plants one — they shouldn't) is acceptable; goldens must otherwise be byte-identical. Note: existing cli tests run with the real `$HOME` unless they set it — the scan is a no-op when `~/.claude/skills/<name>` doesn't exist, but any test machine could have a colliding name for prefixed fixtures (`esc-acme-org-...`), which is vanishingly unlikely; if flakiness is a concern, have the `run` helper `t.Setenv("HOME", t.TempDir())` — check `cli_test.go` and prefer that hardening.

- [ ] **Step 7: Commit**

```bash
git add internal/engine/status.go internal/cli/cli.go internal/cli/skilldup_test.go internal/publisher/redact_test.go
git commit -m "feat(status): report a user-level copy of a managed project skill

The agent sees both copies; escapement can only see this machine's home
dir, so it reports rather than judges: a duplicate field on the existing
dir finding, carrying exactly a name and a same/differs bool. Never a new
finding row — that would flip Clean() and change exit codes for a signal
that is information, not drift."
```

---

### Task 6: `sources.yaml` provenance file

**Files:**
- Create: `internal/pack/sources.go`
- Test: `internal/pack/sources_test.go`

**Interfaces:**
- Produces:
  - `type SourceSkill struct { Name, Source, Subdir, Ref, Commit, Hash string }` (yaml tags below)
  - `type Sources struct { Schema int; Skills []SourceSkill }`
  - `func LoadSources(dir string) (*Sources, error)` — `(nil, nil)` when absent
  - `func (s *Sources) Save(dir string) error` — deterministic (sorted by Name)
  - `func (s *Sources) Skill(name string) *SourceSkill`
  - `func (s *Sources) Upsert(e SourceSkill)`

- [ ] **Step 1: Write the failing test**

```go
// internal/pack/sources_test.go
package pack

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourcesRoundTripSortedAndAbsent(t *testing.T) {
	dir := t.TempDir()
	got, err := LoadSources(dir)
	if err != nil || got != nil {
		t.Fatalf("absent sources.yaml must be (nil, nil), got %v, %v", got, err)
	}
	s := &Sources{Schema: 1}
	s.Upsert(SourceSkill{Name: "writing-plans", Source: "github.com/obra/superpowers", Subdir: "skills/writing-plans", Ref: "v6.2.0", Commit: "abc123", Hash: "sha256:aa"})
	s.Upsert(SourceSkill{Name: "brainstorming", Source: "github.com/obra/superpowers", Subdir: "skills/brainstorming", Ref: "v6.2.0", Commit: "abc123", Hash: "sha256:bb"})
	if err := s.Save(dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadSources(dir)
	if err != nil || loaded == nil || len(loaded.Skills) != 2 {
		t.Fatalf("reload failed: %+v, %v", loaded, err)
	}
	if loaded.Skills[0].Name != "brainstorming" || loaded.Skills[1].Name != "writing-plans" {
		t.Fatalf("skills not sorted by name: %+v", loaded.Skills)
	}
	// Upsert replaces in place.
	s.Upsert(SourceSkill{Name: "brainstorming", Source: "github.com/obra/superpowers", Subdir: "skills/brainstorming", Ref: "v6.3.0", Commit: "def456", Hash: "sha256:cc"})
	if len(s.Skills) != 2 || s.Skill("brainstorming").Ref != "v6.3.0" {
		t.Fatalf("upsert did not replace: %+v", s.Skills)
	}
	// Deterministic bytes: saving the reloaded state reproduces the file
	// byte-for-byte, so repeated vendoring operations diff cleanly.
	first, err := os.ReadFile(filepath.Join(dir, "sources.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := loaded.Save(dir); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(dir, "sources.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("sources.yaml is not deterministic:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

func TestLoadSourcesRejectsUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sources.yaml"), []byte("schema: 1\nbogus: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSources(dir); err == nil {
		t.Fatal("unknown key accepted")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/pack/ -run TestSources -v` — expected: FAIL (undefined symbols).

- [ ] **Step 3: Implement**

```go
// internal/pack/sources.go
package pack

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// SourcesFile is authoring metadata at the pack root: per vendored skill, the
// upstream source, requested ref, resolved commit, and the dir hash of the
// vendored copy. Consumers never read it — but it travels inside the pack
// repo, so the signed tag and LockPack.Hash cover it automatically (spec §2).
// It deliberately lives OUTSIDE the skill directories so the vendored copy
// stays byte-identical to upstream and no directory walk ever has to filter
// it.
const SourcesFile = "sources.yaml"

type SourceSkill struct {
	Name   string `yaml:"name"`
	Source string `yaml:"source"`
	Subdir string `yaml:"subdir,omitempty"`
	Ref    string `yaml:"ref"`
	Commit string `yaml:"commit"`
	Hash   string `yaml:"hash"`
}

type Sources struct {
	Schema int           `yaml:"schema"`
	Skills []SourceSkill `yaml:"skills"`
}

// LoadSources reads dir/sources.yaml. Returns (nil, nil) when absent: a pack
// with no vendored skills simply has no provenance to record.
func LoadSources(dir string) (*Sources, error) {
	raw, err := os.ReadFile(filepath.Join(dir, SourcesFile))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s Sources
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", SourcesFile, err)
	}
	if s.Schema != 1 {
		return nil, fmt.Errorf("%s: unsupported schema %d (want 1)", SourcesFile, s.Schema)
	}
	return &s, nil
}

// Save writes sources.yaml deterministically (skills sorted by name) so
// repeated vendoring operations produce stable, reviewable diffs.
func (s *Sources) Save(dir string) error {
	sort.Slice(s.Skills, func(i, j int) bool { return s.Skills[i].Name < s.Skills[j].Name })
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(s); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, SourcesFile), buf.Bytes(), 0o644)
}

// Skill returns the entry named name, or nil.
func (s *Sources) Skill(name string) *SourceSkill {
	for i := range s.Skills {
		if s.Skills[i].Name == name {
			return &s.Skills[i]
		}
	}
	return nil
}

// Upsert replaces the entry with e.Name, or appends it.
func (s *Sources) Upsert(e SourceSkill) {
	if cur := s.Skill(e.Name); cur != nil {
		*cur = e
		return
	}
	s.Skills = append(s.Skills, e)
}
```

- [ ] **Step 4: Run tests** — `go test ./internal/pack/ -run 'TestSources|TestLoadSources' -v` — expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/pack/sources.go internal/pack/sources_test.go
git commit -m "feat(pack): sources.yaml records vendored-skill provenance

Authoring metadata at the pack root — source, subdir, requested ref,
resolved commit, dir hash — outside the skill directories so the vendored
copy stays byte-identical to upstream and no walk has to filter anything.
Consumers never read it; the signed tag covers it for free."
```

---

### Task 7: Semver tag selection in `internal/source`

**Files:**
- Create: `internal/source/semver.go`
- Test: `internal/source/semver_test.go`

**Interfaces:**
- Produces: `func HighestSemverTag(tags map[string]string) (name, commit string, ok bool)` — consumed by Tasks 10–12. Internal: `parseSemver(s string) (semver, bool)`, `compareSemver(a, b semver) int`.

- [ ] **Step 1: Write the failing test**

```go
// internal/source/semver_test.go
package source

import "testing"

func TestHighestSemverTag(t *testing.T) {
	cases := []struct {
		name string
		tags map[string]string
		want string
		ok   bool
	}{
		{"empty", map[string]string{}, "", false},
		{"no semver tags", map[string]string{"latest": "a", "release-1": "b"}, "", false},
		{"basic ordering", map[string]string{"v1.2.0": "a", "v1.10.0": "b", "v1.9.9": "c"}, "v1.10.0", true},
		{"v prefix optional", map[string]string{"1.0.0": "a", "v2.0.0": "b"}, "v2.0.0", true},
		{"prerelease below release", map[string]string{"v2.0.0-rc.1": "a", "v2.0.0": "b"}, "v2.0.0", true},
		{"prerelease ordering", map[string]string{"v2.0.0-alpha": "a", "v2.0.0-beta": "b"}, "v2.0.0-beta", true},
		{"numeric prerelease below alnum", map[string]string{"v1.0.0-1": "a", "v1.0.0-alpha": "b"}, "v1.0.0-alpha", true},
		{"build metadata ignored", map[string]string{"v1.0.0+build5": "a"}, "v1.0.0+build5", true},
		{"non-semver ignored beside semver", map[string]string{"nightly": "a", "v0.1.0": "b"}, "v0.1.0", true},
		{"equal versions tie-break on name", map[string]string{"1.0.0": "a", "v1.0.0": "b"}, "v1.0.0", true},
	}
	for _, c := range cases {
		name, commit, ok := HighestSemverTag(c.tags)
		if ok != c.ok || name != c.want {
			t.Errorf("%s: got (%q, ok=%v), want (%q, ok=%v)", c.name, name, ok, c.want, c.ok)
		}
		if ok && commit != c.tags[name] {
			t.Errorf("%s: commit %q does not match tags[%q]", c.name, commit, name)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/source/ -run TestHighestSemverTag -v` — expected: FAIL.

- [ ] **Step 3: Implement**

```go
// internal/source/semver.go
package source

import (
	"strconv"
	"strings"
)

// semver is the parsed core of a semantic version tag. Build metadata is
// discarded at parse time: semver §10 says it must be ignored for ordering.
type semver struct {
	major, minor, patch int
	pre                 []string // empty = a release, which sorts above any prerelease
}

// parseSemver parses "v1.2.3", "1.2.3", "1.2.3-rc.1+meta". Anything that is
// not exactly major.minor.patch (all numeric) with an optional prerelease is
// not semver and reports false — outdated/update-skill must simply ignore
// such tags rather than guess.
func parseSemver(s string) (semver, bool) {
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	var pre []string
	if i := strings.IndexByte(s, '-'); i >= 0 {
		pre = strings.Split(s[i+1:], ".")
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return semver{}, false
	}
	var nums [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || (len(p) > 1 && p[0] == '0') {
			return semver{}, false
		}
		nums[i] = n
	}
	return semver{nums[0], nums[1], nums[2], pre}, true
}

// compareSemver returns -1, 0, or 1, per semver §11: numeric core first;
// a release outranks any prerelease of the same core; prerelease segments
// compare numerically when both numeric, lexically otherwise, and a numeric
// segment is always lower than an alphanumeric one.
func compareSemver(a, b semver) int {
	for _, d := range [3]int{a.major - b.major, a.minor - b.minor, a.patch - b.patch} {
		if d < 0 {
			return -1
		}
		if d > 0 {
			return 1
		}
	}
	switch {
	case len(a.pre) == 0 && len(b.pre) == 0:
		return 0
	case len(a.pre) == 0:
		return 1
	case len(b.pre) == 0:
		return -1
	}
	for i := 0; i < len(a.pre) && i < len(b.pre); i++ {
		an, aNum := strconv.Atoi(a.pre[i])
		bn, bNum := strconv.Atoi(b.pre[i])
		switch {
		case aNum == nil && bNum == nil:
			if an != bn {
				if an < bn {
					return -1
				}
				return 1
			}
		case aNum == nil:
			return -1 // numeric identifiers sort below alphanumeric
		case bNum == nil:
			return 1
		default:
			if c := strings.Compare(a.pre[i], b.pre[i]); c != 0 {
				return c
			}
		}
	}
	switch {
	case len(a.pre) < len(b.pre):
		return -1
	case len(a.pre) > len(b.pre):
		return 1
	}
	return 0
}

// HighestSemverTag returns the highest semantic-version tag in tags
// (name -> commit, as returned by LsRemoteTags), ignoring non-semver names.
// Ties (e.g. "1.0.0" and "v1.0.0") break on the lexicographically larger
// tag name so the result is deterministic regardless of map order.
func HighestSemverTag(tags map[string]string) (name, commit string, ok bool) {
	var best semver
	for n, c := range tags {
		v, isSemver := parseSemver(n)
		if !isSemver {
			continue
		}
		cmp := 1
		if ok {
			cmp = compareSemver(v, best)
		}
		if cmp > 0 || (cmp == 0 && n > name) {
			best, name, commit, ok = v, n, c, true
		}
	}
	return name, commit, ok
}
```

- [ ] **Step 4: Run tests** — `go test ./internal/source/ -v` — expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/source/semver.go internal/source/semver_test.go
git commit -m "feat(source): pick the highest semver tag from ls-remote output

Hand-rolled major.minor.patch + prerelease comparison per semver 11 —
the single-dependency policy rules out a semver module, and outdated/
update-skill only need a total order over tag names. Non-semver tags are
ignored, never guessed at; ties break lexically for determinism."
```

---

### Task 8: Skill discovery, vendor copy, and diffstat helpers

**Files:**
- Create: `internal/cli/skillvendor.go`
- Test: `internal/cli/skillvendor_test.go`

**Interfaces:**
- Consumes: `pack.DirFiles`, `pack.DirHashOf`, `esc.HashBytes`.
- Produces:
  - `type discoveredSkill struct { Name string; Dir string }`
  - `func discoverSkills(dir, fallbackName string) ([]discoveredSkill, error)` — sorted by Name.
  - `func vendorCopy(srcDir, dstDir string) (files []string, hash string, err error)` — refuses an existing dstDir.
  - `func fileHashes(dir string) (map[string]string, error)` — rel path -> content hash, via DirFiles.
  - `func diffstat(old, new map[string]string) (added, removed, changed int)`

- [ ] **Step 1: Write the failing test**

```go
// internal/cli/skillvendor_test.go
package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tensorgroup/openescapement/internal/pack"
)

func TestDiscoverSkillsSuiteAndRoot(t *testing.T) {
	suite := t.TempDir()
	writeFiles(t, suite, map[string]string{
		"skills/brainstorming/SKILL.md": "a\n",
		"skills/writing-plans/SKILL.md": "b\n",
		"skills/writing-plans/ref.md":   "c\n",
		"README.md":                     "not a skill\n",
	})
	got, err := discoverSkills(suite, "suite-root")
	if err != nil || len(got) != 2 {
		t.Fatalf("discover: %+v, %v", got, err)
	}
	if got[0].Name != "brainstorming" || got[1].Name != "writing-plans" {
		t.Fatalf("wrong names/order: %+v", got)
	}

	single := t.TempDir()
	writeFiles(t, single, map[string]string{"SKILL.md": "root skill\n"})
	got, err = discoverSkills(single, "my-skill")
	if err != nil || len(got) != 1 || got[0].Name != "my-skill" || got[0].Dir != single {
		t.Fatalf("root-is-a-skill: %+v, %v", got, err)
	}
}

func TestDiscoverSkillsRejectsNestedAndEmpty(t *testing.T) {
	nested := t.TempDir()
	writeFiles(t, nested, map[string]string{
		"outer/SKILL.md":       "a\n",
		"outer/inner/SKILL.md": "b\n",
	})
	if _, err := discoverSkills(nested, "x"); err == nil {
		t.Fatal("nested skill dirs must be an error")
	}
	if _, err := discoverSkills(t.TempDir(), "x"); err == nil {
		t.Fatal("no skills found must be an error")
	}
}

func TestVendorCopyMatchesDirFilesAndRefusesExisting(t *testing.T) {
	src := t.TempDir()
	writeFiles(t, src, map[string]string{"SKILL.md": "s\n", "scripts/run.sh": "#!/bin/sh\n"})
	if err := os.Chmod(filepath.Join(src, "scripts", "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	dstRoot := t.TempDir()
	dst := filepath.Join(dstRoot, "skills", "demo")
	files, hash, err := vendorCopy(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	wantHash, err := pack.DirHash(dst)
	if err != nil || hash != wantHash {
		t.Fatalf("recorded hash %q != DirHash(dst) %q (%v)", hash, wantHash, err)
	}
	srcHash, _ := pack.DirHash(src)
	if hash != srcHash {
		t.Fatalf("vendored copy not byte-identical to upstream: %q != %q", hash, srcHash)
	}
	if len(files) != 2 {
		t.Fatalf("files: %v", files)
	}
	if info, _ := os.Stat(filepath.Join(dst, "scripts", "run.sh")); info.Mode()&0o111 == 0 {
		t.Fatal("executable bit not preserved")
	}
	if _, _, err := vendorCopy(src, dst); err == nil {
		t.Fatal("existing destination must be refused")
	}
}

func TestDiffstat(t *testing.T) {
	old := map[string]string{"a": "1", "b": "2", "c": "3"}
	new := map[string]string{"a": "1", "b": "9", "d": "4"}
	added, removed, changed := diffstat(old, new)
	if added != 1 || removed != 1 || changed != 1 {
		t.Fatalf("got %d/%d/%d, want 1/1/1", added, removed, changed)
	}
}
```

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/cli/ -run 'TestDiscover|TestVendorCopy|TestDiffstat' -v` — expected: FAIL.

- [ ] **Step 3: Implement**

```go
// internal/cli/skillvendor.go
package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/pack"
)

// discoveredSkill is one skill directory found in a fetched source tree.
type discoveredSkill struct {
	Name string // on-disk name: the directory's base, or fallbackName at the root
	Dir  string // absolute path of the skill directory
}

// discoverSkills finds skill directories by the presence of SKILL.md (spec
// §4). If dir itself directly contains SKILL.md, the whole tree is one skill
// named fallbackName (the subdir's base, or the repo name). Otherwise every
// directory directly containing SKILL.md is a skill named after its base.
// Nested skills are ambiguous and refused. Symlinked entries are not
// followed during discovery — the copy path (pack.DirFiles) fails closed on
// any symlink inside a selected skill, which is the actual trust boundary.
func discoverSkills(dir, fallbackName string) ([]discoveredSkill, error) {
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err == nil {
		return []discoveredSkill{{Name: fallbackName, Dir: dir}}, nil
	}
	var found []discoveredSkill
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil // never follow; if it's inside a chosen skill, DirFiles refuses later
		}
		if !d.IsDir() {
			return nil
		}
		if d.Name() == ".git" {
			return filepath.SkipDir
		}
		if _, serr := os.Stat(filepath.Join(p, "SKILL.md")); serr == nil && p != dir {
			found = append(found, discoveredSkill{Name: d.Name(), Dir: p})
			return filepath.SkipDir // a skill dir's subdirs are its content, and nesting is refused below anyway
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("no SKILL.md found under %s: nothing to vendor", dir)
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Name < found[j].Name })
	for i := 1; i < len(found); i++ {
		if found[i].Name == found[i-1].Name {
			return nil, fmt.Errorf("two skill directories are both named %q; use --only after moving one aside upstream, or vendor from a narrower #subdir", found[i].Name)
		}
	}
	return found, nil
}

// vendorCopy copies srcDir into dstDir, enumerating files with the SAME
// pack.DirFiles walk hashing uses — never a second, divergent walk (the
// hardening cycle documented exactly that bug class). Symlinks anywhere in
// srcDir fail the copy closed. dstDir must not exist: overwriting is the
// caller's explicit, separately-gated decision. The executable bit is
// preserved (skills ship scripts); everything else lands 0o644.
func vendorCopy(srcDir, dstDir string) (files []string, hash string, err error) {
	if _, serr := os.Lstat(dstDir); serr == nil {
		return nil, "", fmt.Errorf("destination %s already exists", dstDir)
	}
	files, err = pack.DirFiles(srcDir)
	if err != nil {
		return nil, "", err
	}
	for _, rel := range files {
		src := filepath.Join(srcDir, filepath.FromSlash(rel))
		dst := filepath.Join(dstDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, "", err
		}
		content, err := os.ReadFile(src)
		if err != nil {
			return nil, "", err
		}
		mode := os.FileMode(0o644)
		if info, err := os.Stat(src); err == nil && info.Mode()&0o111 != 0 {
			mode = 0o755
		}
		if err := os.WriteFile(dst, content, mode); err != nil {
			return nil, "", err
		}
	}
	// Hash the DESTINATION: what sources.yaml records must be the hash of
	// what is actually in the pack repo, not an assumption about the copy.
	hash, err = pack.DirHashOf(dstDir, files)
	if err != nil {
		return nil, "", err
	}
	return files, hash, nil
}

// fileHashes returns rel path -> content hash for every file DirFiles sees.
func fileHashes(dir string) (map[string]string, error) {
	files, err := pack.DirFiles(dir)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(files))
	for _, rel := range files {
		content, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		out[rel] = esc.HashBytes(content)
	}
	return out, nil
}

// diffstat compares two fileHashes maps.
func diffstat(old, new map[string]string) (added, removed, changed int) {
	for rel, h := range new {
		oh, ok := old[rel]
		switch {
		case !ok:
			added++
		case oh != h:
			changed++
		}
	}
	for rel := range old {
		if _, ok := new[rel]; !ok {
			removed++
		}
	}
	return added, removed, changed
}

// splitSkillURL splits the CLI's URL[#subdir] form. The spec's authoring
// commands use '#' (not the pack-source '//' convention) so a URL can be
// pasted verbatim; it is translated to internal/source's form by the caller.
func splitSkillURL(arg string) (url, subdir string) {
	if i := strings.LastIndex(arg, "#"); i >= 0 {
		return arg[:i], arg[i+1:]
	}
	return arg, ""
}

// repoBaseName derives a fallback skill name from a git URL: the last path
// segment with any .git suffix trimmed.
func repoBaseName(url string) string {
	s := strings.TrimSuffix(strings.TrimRight(url, "/"), ".git")
	if i := strings.LastIndexAny(s, "/:"); i >= 0 {
		s = s[i+1:]
	}
	return s
}
```

- [ ] **Step 4: Run tests** — `go test ./internal/cli/ -run 'TestDiscover|TestVendorCopy|TestDiffstat' -v` — expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/skillvendor.go internal/cli/skillvendor_test.go
git commit -m "feat(cli): skill discovery, vendor copy, and diffstat helpers

Discovery is SKILL.md presence, with root-is-a-skill and nested-suite
cases settled; the copy enumerates files with the same DirFiles walk
hashing uses, so the recorded hash and the copied set cannot diverge by
construction, and the recorded hash is computed over the destination —
what is actually in the pack repo."
```

---

### Task 9: Comment-preserving pack.yaml append

**Files:**
- Create: `internal/cli/packyaml.go`
- Test: `internal/cli/packyaml_test.go`

**Interfaces:**
- Consumes: `pack.SkillEntry`, `restoreFile` (existing atomic-write helper in `cli.go`).
- Produces: `func appendSkillEntries(path string, entries []pack.SkillEntry) error`

- [ ] **Step 1: Write the failing test**

```go
// internal/cli/packyaml_test.go
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/pack"
)

func TestAppendSkillEntriesPreservesCommentsAndExisting(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "pack.yaml")
	orig := `schema: 1
# the team's own comment about this pack
name: acme
version: 1.0.0
skills:
  - skills/vault-usage # keep our vault rules
`
	if err := os.WriteFile(p, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	err := appendSkillEntries(p, []pack.SkillEntry{{Path: "skills/brainstorming", Name: "brainstorming"}})
	if err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(p)
	s := string(out)
	for _, want := range []string{
		"the team's own comment about this pack",
		"keep our vault rules",
		"- skills/vault-usage",
		"path: skills/brainstorming",
		"name: brainstorming",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in rewritten pack.yaml:\n%s", want, s)
		}
	}
	// The rewritten file must still load as a valid manifest shape.
	if _, err := decodeManifestSkills(t, s); err != nil {
		t.Fatalf("rewritten pack.yaml no longer parses: %v", err)
	}
}

func TestAppendSkillEntriesCreatesSkillsKey(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "pack.yaml")
	if err := os.WriteFile(p, []byte("schema: 1\nname: acme\nversion: 1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := appendSkillEntries(p, []pack.SkillEntry{{Path: "skills/x", Name: "x"}}); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(p)
	if !strings.Contains(string(out), "skills:") || !strings.Contains(string(out), "name: x") {
		t.Fatalf("skills key not created:\n%s", out)
	}
}

func decodeManifestSkills(t *testing.T, doc string) ([]pack.SkillEntry, error) {
	t.Helper()
	var m struct {
		Skills []pack.SkillEntry `yaml:"skills"`
	}
	return m.Skills, yamlUnmarshalLoose([]byte(doc), &m)
}
```

Add to `packyaml.go` (or the test file) the tiny loose-decode helper used above:

```go
func yamlUnmarshalLoose(b []byte, v any) error { return yaml.Unmarshal(b, v) }
```

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/cli/ -run TestAppendSkillEntries -v` — expected: FAIL.

- [ ] **Step 3: Implement**

```go
// internal/cli/packyaml.go
package cli

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/tensorgroup/openescapement/internal/pack"
)

// appendSkillEntries appends object-form skills: entries to a pack.yaml,
// preserving the author's comments and ordering. A structural yaml.Node round
// trip, not a struct re-marshal: re-marshalling a hand-written manifest
// through Manifest would reorder keys and drop every comment — modifying far
// more than the bytes this operation is about. The byte-preservation
// invariant applies in spirit to authoring files too: touch only the skills
// sequence. The write is atomic and preserves the file's permission bits
// (restoreFile, cli.go).
func appendSkillEntries(path string, entries []pack.SkillEntry) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("%s: not a YAML mapping", path)
	}
	root := doc.Content[0]
	var seq *yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "skills" {
			seq = root.Content[i+1]
			break
		}
	}
	if seq == nil {
		seq = &yaml.Node{Kind: yaml.SequenceNode}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "skills"}, seq)
	}
	if seq.Kind != yaml.SequenceNode {
		return fmt.Errorf("%s: skills is not a list", path)
	}
	for _, e := range entries {
		entryNode := &yaml.Node{}
		if err := entryNode.Encode(e); err != nil {
			return err
		}
		seq.Content = append(seq.Content, entryNode)
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return restoreFile(path, buf.Bytes())
}
```

Note: `yaml.Node.Encode` on a `SkillEntry` goes through its `MarshalYAML`, producing the `{path, name}` mapping (or plain scalar when Name is empty). yaml.v3's node round trip preserves comments attached to nodes; minor whitespace normalization is acceptable and the test asserts content, comments, and reparseability, not byte identity.

- [ ] **Step 4: Run tests** — `go test ./internal/cli/ -run TestAppendSkillEntries -v` — expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/packyaml.go internal/cli/packyaml_test.go
git commit -m "feat(cli): append skills entries to pack.yaml without eating comments

A yaml.Node round trip instead of a struct re-marshal: the author's file
is theirs, and rewriting it wholesale to add two lines would reorder keys
and drop every comment. Only the skills sequence is touched; the write is
atomic and keeps the file's permission bits."
```

---

### Task 10: `esc pack` dispatch + `add-skill`

**Files:**
- Modify: `internal/cli/cli.go` (`Run` switch, `usage` text)
- Create: `internal/cli/packcmd.go`
- Create: `internal/cli/packadd.go`
- Test: `internal/cli/packadd_test.go`

**Interfaces:**
- Consumes: everything from Tasks 6–9, `source.Fetch`, `source.LsRemoteTags`, `source.LsRemoteHash`, `source.HighestSemverTag`, `engine.CacheDir() (string, error)`, `config.PackRef{Source, Ref}`.
- Produces:
  - `func cmdPack(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int`
  - `func resolveSkillRef(ctx context.Context, url, refFlag string, stderr io.Writer) (recorded, fetchRef string, err error)` — shared with Tasks 11–12.
  - `func fetchSkillSource(ctx context.Context, root, url, subdir, fetchRef string) (*source.FetchResult, error)`

- [ ] **Step 1: Write the failing e2e test**

Real temp git repos, no mocks. Check `internal/cli/cli_test.go` first for how `run` isolates `ESC_CACHE_DIR` (the suite relies on it); if `run` does not set it, use `t.Setenv("ESC_CACHE_DIR", t.TempDir())` in these tests.

```go
// internal/cli/packadd_test.go
package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/pack"
)

// newUpstreamSkillRepo builds a real git repo carrying a two-skill suite
// under skills/, tagged v1.0.0, and returns its path.
func newUpstreamSkillRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitIn(t, repo, "init", "-q")
	writeFiles(t, repo, map[string]string{
		"README.md":                       "a skill suite\n",
		"skills/brainstorming/SKILL.md":   "---\nname: brainstorming\n---\n\nExplore first.\n",
		"skills/writing-plans/SKILL.md":   "---\nname: writing-plans\n---\n\nPlan second.\n",
		"skills/writing-plans/example.md": "an example\n",
	})
	gitIn(t, repo, "add", ".")
	gitIn(t, repo, "commit", "-q", "-m", "v1")
	gitIn(t, repo, "tag", "v1.0.0")
	return repo
}

// newAuthorPack scaffolds a pack repo (the author's cwd for esc pack ...).
func newAuthorPack(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"pack.yaml": "schema: 1\nname: acme\nversion: 1.0.0\n",
	})
	return root
}

func TestPackAddSkillVendorsSuiteWithProvenance(t *testing.T) {
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	up := newUpstreamSkillRepo(t)
	root := newAuthorPack(t)
	out, code := runEscOut(t, root, "pack", "add-skill", "file://"+up+"#skills")
	if code != 0 {
		t.Fatalf("add-skill exited %d:\n%s", code, out)
	}
	for _, rel := range []string{
		"skills/brainstorming/SKILL.md",
		"skills/writing-plans/SKILL.md",
		"skills/writing-plans/example.md",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Errorf("vendored file missing: %s (%v)", rel, err)
		}
	}
	// pack.yaml gained object entries and still loads.
	p, err := pack.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, e := range p.Manifest.Skills {
		names[e.Name] = true
	}
	if !names["brainstorming"] || !names["writing-plans"] {
		t.Fatalf("pack.yaml entries wrong: %+v", p.Manifest.Skills)
	}
	// sources.yaml records provenance with an exact resolved commit and the
	// dir hash of each vendored copy.
	srcs, err := pack.LoadSources(root)
	if err != nil || srcs == nil || len(srcs.Skills) != 2 {
		t.Fatalf("sources.yaml: %+v, %v", srcs, err)
	}
	hex40 := regexp.MustCompile(`^[0-9a-f]{40}$`)
	for _, s := range srcs.Skills {
		if s.Ref != "v1.0.0" || !hex40.MatchString(s.Commit) {
			t.Errorf("entry %s: ref=%q commit=%q", s.Name, s.Ref, s.Commit)
		}
		wantHash, err := pack.DirHash(filepath.Join(root, "skills", s.Name))
		if err != nil || s.Hash != wantHash {
			t.Errorf("entry %s: recorded hash %q, dir hashes to %q (%v)", s.Name, s.Hash, wantHash, err)
		}
	}
	// The command told the author what happened at which commit.
	if !strings.Contains(out, "brainstorming") || !strings.Contains(out, "v1.0.0") {
		t.Errorf("summary output missing vendored names/ref:\n%s", out)
	}
}

func TestPackAddSkillOnlyAndRefusals(t *testing.T) {
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	up := newUpstreamSkillRepo(t)
	root := newAuthorPack(t)
	runEsc(t, root, "pack", "add-skill", "file://"+up+"#skills", "--only", "brainstorming")
	if _, err := os.Stat(filepath.Join(root, "skills", "writing-plans")); !os.IsNotExist(err) {
		t.Fatal("--only did not narrow the vendored set")
	}
	// A taken name is refused, and nothing is partially written.
	out, code := runEscOut(t, root, "pack", "add-skill", "file://"+up+"#skills", "--only", "brainstorming")
	if code == 0 {
		t.Fatalf("re-vendoring a taken name must fail:\n%s", out)
	}
	// Unknown --only name is an error, not a silent no-op.
	if _, code := runEscOut(t, root, "pack", "add-skill", "file://"+up+"#skills", "--only", "nope"); code == 0 {
		t.Fatal("--only with an unknown name must fail")
	}
}

func TestPackAddSkillNoTagsFallsBackToHeadWithWarning(t *testing.T) {
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	up := t.TempDir()
	gitIn(t, up, "init", "-q")
	writeFiles(t, up, map[string]string{"SKILL.md": "---\nname: solo\n---\n\nOne skill, no tags.\n"})
	gitIn(t, up, "add", ".")
	gitIn(t, up, "commit", "-q", "-m", "head only")
	root := newAuthorPack(t)
	out, code := runEscOut(t, root, "pack", "add-skill", "file://"+up)
	if code != 0 {
		t.Fatalf("add-skill exited %d:\n%s", code, out)
	}
	if !strings.Contains(out, "no semver tags") {
		t.Fatalf("missing fallback warning:\n%s", out)
	}
	srcs, _ := pack.LoadSources(root)
	if srcs == nil || len(srcs.Skills) != 1 || srcs.Skills[0].Ref != "HEAD" || srcs.Skills[0].Commit == "" {
		t.Fatalf("HEAD fallback not recorded exactly: %+v", srcs)
	}
	// Repo-root-is-a-skill: named after the repo directory.
	if _, err := os.Stat(filepath.Join(root, "skills", filepath.Base(up), "SKILL.md")); err != nil {
		t.Fatalf("root skill not vendored under repo name: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail** — `go test ./internal/cli/ -run TestPackAddSkill -v` — expected: FAIL (`unknown command "pack"`).

- [ ] **Step 3: Wire the dispatch and usage**

In `internal/cli/cli.go`'s `Run` switch, before `case "version":`:

```go
	case "pack":
		return cmdPack(ctx, root, args[1:], stdout, stderr)
```

In the `usage` constant, after the `esc serve` lines:

```go
  esc pack add-skill URL[#subdir] [--ref REF] [--only a,b]
  esc pack update-skill [name...] [--all] [--ref REF] [--force]
  esc pack outdated [--check]
                                 Author commands (run in the pack repo):
                                 vendor and maintain external skills
```

Create `internal/cli/packcmd.go`:

```go
// Package-author command family: esc pack {add-skill, update-skill,
// outdated}. All run in the pack repo root (where pack.yaml lives), not in a
// governed repo, and reuse internal/source's cache and detached-checkout
// machinery plus the pack.DirFiles walk.
package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/engine"
	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/pack"
	"github.com/tensorgroup/openescapement/internal/source"
)

const packUsage = `usage:
  esc pack add-skill URL[#subdir] [--ref REF] [--only a,b]
  esc pack update-skill [name...] [--all] [--ref REF] [--force]
  esc pack outdated [--check]
`

func cmdPack(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, packUsage)
		return 2
	}
	switch args[0] {
	case "add-skill":
		return exitCode(cmdPackAddSkill(ctx, root, args[1:], stdout, stderr), stderr)
	case "update-skill":
		return exitCode(cmdPackUpdateSkill(ctx, root, args[1:], stdout, stderr), stderr)
	case "outdated":
		return exitCode(cmdPackOutdated(ctx, root, args[1:], stdout, stderr), stderr)
	default:
		fmt.Fprintf(stderr, "esc pack: unknown subcommand %q\n\n%s", args[0], packUsage)
		return 2
	}
}

// loadAuthorPack loads and validates the pack rooted at root, and prints any
// lint warnings (e.g. floating MCP pins) to stderr — the authoring commands
// are the authoring surface, so this is where an author hears about them.
func loadAuthorPack(root string, stderr io.Writer) (*pack.Pack, error) {
	p, err := pack.Load(root)
	if err != nil {
		return nil, err
	}
	for _, w := range p.Warnings {
		fmt.Fprintf(stderr, "warning: %s\n", w)
	}
	return p, nil
}

// resolveSkillRef picks what to vendor: an explicit --ref as given, else the
// highest semver tag, else the default branch head with a warning (spec §4:
// the pin is exact either way because the resolved commit is always
// recorded). recorded is what sources.yaml stores as ref ("HEAD" for the
// fallback); fetchRef is what source.Fetch checks out (the head's commit SHA
// for the fallback — "HEAD" itself is not a stable remote ref name to pin).
func resolveSkillRef(ctx context.Context, url, refFlag string, stderr io.Writer) (recorded, fetchRef string, err error) {
	if refFlag != "" {
		return refFlag, refFlag, nil
	}
	tags, err := source.LsRemoteTags(ctx, url)
	if err != nil {
		return "", "", err
	}
	if name, _, ok := source.HighestSemverTag(tags); ok {
		return name, name, nil
	}
	head, err := source.LsRemoteHash(ctx, url, "HEAD")
	if err != nil {
		return "", "", err
	}
	if head == "" {
		return "", "", fmt.Errorf("%w: %s has no resolvable HEAD", esc.ErrFetch, url)
	}
	fmt.Fprintf(stderr, "warning: %s has no semver tags; vendoring the default branch head (%s)\n", url, head[:12])
	return "HEAD", head, nil
}

// fetchSkillSource materializes url (with optional subdir) at fetchRef via
// the shared pack cache. Plain local directories are refused: a vendored
// skill needs a resolvable commit for provenance, and file:// serves the
// local-repo case through git.
func fetchSkillSource(ctx context.Context, root, url, subdir, fetchRef string) (*source.FetchResult, error) {
	parsed, err := source.ParseSource(url)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", esc.ErrFetch, err)
	}
	if parsed.Local {
		return nil, fmt.Errorf("%w: %s is a plain directory; vendoring needs a git source (use file://) so a commit can be recorded", esc.ErrFetch, url)
	}
	cache, err := engine.CacheDir()
	if err != nil {
		return nil, err
	}
	src := url
	if subdir != "" {
		src = url + "//" + subdir
	}
	return source.Fetch(ctx, config.PackRef{Source: src, Ref: fetchRef}, cache, root)
}
```

(`pack.Warnings` arrives in Task 13; until then add the field as an empty declaration in this task — see Step 4 note — or reorder: simplest is to add `Warnings []string` to `pack.Pack` NOW, unpopulated, and fill it in Task 13.)

- [ ] **Step 4: Add the empty `Warnings` field**

In `internal/pack/pack.go`, on `Pack`:

```go
// Pack is a fully loaded, validated rule pack.
type Pack struct {
	Dir       string
	Manifest  Manifest
	Fragments []Fragment
	Warnings  []string // non-fatal authoring lint (populated by Load; printed only by esc pack commands)
}
```

- [ ] **Step 5: Implement `add-skill`**

```go
// internal/cli/packadd.go
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tensorgroup/openescapement/internal/pack"
)

// cmdPackAddSkill vendors skills from an external repo into the pack repo:
// fetch, discover by SKILL.md, copy under skills/<name>/, append object
// entries to pack.yaml, record provenance in sources.yaml. It prints what
// was vendored at which commit so the author reviews the diff before
// committing (spec §4). All refusals happen before anything is written.
func cmdPackAddSkill(ctx context.Context, root string, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("pack add-skill", flag.ContinueOnError)
	fs.SetOutput(stderr)
	ref := fs.String("ref", "", "ref to vendor (default: highest semver tag, else default branch head)")
	only := fs.String("only", "", "comma-separated skill names to vendor from a multi-skill repo")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("%w: esc pack add-skill takes exactly one URL[#subdir]", errUsage)
	}
	p, err := loadAuthorPack(root, stderr)
	if err != nil {
		return err
	}
	url, subdir := splitSkillURL(fs.Arg(0))
	recorded, fetchRef, err := resolveSkillRef(ctx, url, *ref, stderr)
	if err != nil {
		return err
	}
	fr, err := fetchSkillSource(ctx, root, url, subdir, fetchRef)
	if err != nil {
		return err
	}
	fallback := repoBaseName(url)
	if subdir != "" {
		fallback = filepath.Base(filepath.FromSlash(subdir))
	}
	skills, err := discoverSkills(fr.Dir, fallback)
	if err != nil {
		return err
	}
	if *only != "" {
		skills, err = filterSkills(skills, strings.Split(*only, ","))
		if err != nil {
			return err
		}
	}
	// Refuse taken names BEFORE writing anything: resolved on-disk names of
	// existing entries, existing skills/<name> dirs, and sources.yaml entries
	// all count (spec §3: add-skill refuses a name already present).
	srcs, err := pack.LoadSources(root)
	if err != nil {
		return err
	}
	if srcs == nil {
		srcs = &pack.Sources{Schema: 1}
	}
	taken := map[string]bool{}
	for _, e := range p.Manifest.Skills {
		taken[e.DirName(p.Manifest.Name)] = true
	}
	for _, d := range skills {
		if taken[d.Name] {
			return fmt.Errorf("skill name %q is already present in this pack", d.Name)
		}
		if srcs.Skill(d.Name) != nil {
			return fmt.Errorf("skill name %q is already recorded in %s", d.Name, pack.SourcesFile)
		}
		if _, serr := os.Lstat(filepath.Join(root, "skills", d.Name)); serr == nil {
			return fmt.Errorf("skills/%s already exists in the pack repo", d.Name)
		}
	}
	var entries []pack.SkillEntry
	for _, d := range skills {
		files, hash, err := vendorCopy(d.Dir, filepath.Join(root, "skills", d.Name))
		if err != nil {
			return err
		}
		entries = append(entries, pack.SkillEntry{Path: "skills/" + d.Name, Name: d.Name})
		srcs.Upsert(pack.SourceSkill{
			Name: d.Name, Source: url, Subdir: relSkillSubdir(subdir, fr.Dir, d.Dir),
			Ref: recorded, Commit: fr.Commit, Hash: hash,
		})
		fmt.Fprintf(stdout, "vendored %s (%d files) from %s@%s at %s\n", d.Name, len(files), url, recorded, fr.Commit)
	}
	if err := appendSkillEntries(filepath.Join(root, "pack.yaml"), entries); err != nil {
		return err
	}
	if err := srcs.Save(root); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "review the diff, then commit the pack repo.")
	return nil
}

// filterSkills narrows to the named skills; an unknown name is an error, not
// a silent no-op.
func filterSkills(all []discoveredSkill, names []string) ([]discoveredSkill, error) {
	byName := map[string]discoveredSkill{}
	for _, d := range all {
		byName[d.Name] = d
	}
	var out []discoveredSkill
	for _, n := range names {
		n = strings.TrimSpace(n)
		d, ok := byName[n]
		if !ok {
			return nil, fmt.Errorf("--only %s: no such skill in the source (have: %s)", n, skillNames(all))
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func skillNames(ds []discoveredSkill) string {
	names := make([]string, len(ds))
	for i, d := range ds {
		names[i] = d.Name
	}
	return strings.Join(names, ", ")
}

// relSkillSubdir records where inside the upstream repo the skill lives:
// the add-time subdir joined with the skill dir's position under the fetch
// dir. Recorded so update-skill can re-fetch the same tree narrowly.
func relSkillSubdir(subdir, fetchDir, skillDir string) string {
	rel, err := filepath.Rel(fetchDir, skillDir)
	if err != nil || rel == "." {
		return subdir
	}
	rel = filepath.ToSlash(rel)
	if subdir == "" {
		return rel
	}
	return subdir + "/" + rel
}

// errUsage marks a flag/arity mistake in the pack command family so
// exitCode maps it to 2 via esc.ErrConfig-style handling.
var errUsage = errors.New("usage")
```

Then make `exitCode` treat it as usage — in `cli.go`'s `exitCode` switch add:

```go
	case errors.Is(err, errUsage):
		return 2
```

- [ ] **Step 6: Run tests** — `go test ./internal/cli/ -run TestPackAddSkill -v`, then `go test ./...` — expected: PASS. If `examples_consistency_test.go` or a usage golden asserts the help text, update the asserted text to include the new `esc pack` lines — verify the diff is exactly the added lines.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/cli.go internal/cli/packcmd.go internal/cli/packadd.go internal/cli/packadd_test.go internal/pack/pack.go
git commit -m "feat(cli): esc pack add-skill vendors external skills with provenance

Fetch through the shared cache, discover by SKILL.md, copy via the same
DirFiles walk hashing uses, append object entries to pack.yaml, record
source/ref/commit/hash in sources.yaml. Every name refusal happens before
any write; no tags falls back to the default branch head, says so, and
still records the exact commit."
```

---

### Task 11: `esc pack update-skill`

**Files:**
- Create: `internal/cli/packupdate.go`
- Test: `internal/cli/packupdate_test.go`

**Interfaces:**
- Consumes: Task 10's `loadAuthorPack`, `resolveSkillRef`, `fetchSkillSource`, `errUsage`; Task 8's `discoverSkills`, `vendorCopy`, `fileHashes`, `diffstat`; Task 6's sources API.
- Produces: `func cmdPackUpdateSkill(ctx context.Context, root string, args []string, stdout, stderr io.Writer) error`

- [ ] **Step 1: Write the failing tests**

```go
// internal/cli/packupdate_test.go
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/pack"
)

// publishV2 rewrites a skill upstream and tags v2.0.0.
func publishV2(t *testing.T, up string) {
	t.Helper()
	writeFiles(t, up, map[string]string{
		"skills/brainstorming/SKILL.md": "---\nname: brainstorming\n---\n\nExplore first, v2.\n",
	})
	gitIn(t, up, "add", ".")
	gitIn(t, up, "commit", "-q", "-m", "v2")
	gitIn(t, up, "tag", "v2.0.0")
}

func vendoredAuthorPack(t *testing.T) (root, up string) {
	t.Helper()
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	up = newUpstreamSkillRepo(t)
	root = newAuthorPack(t)
	runEsc(t, root, "pack", "add-skill", "file://"+up+"#skills")
	return root, up
}

func TestPackUpdateSkillPullsNewVersionAndPrintsDiffstat(t *testing.T) {
	root, up := vendoredAuthorPack(t)
	publishV2(t, up)
	out, code := runEscOut(t, root, "pack", "update-skill", "brainstorming")
	if code != 0 {
		t.Fatalf("update-skill exited %d:\n%s", code, out)
	}
	got, _ := os.ReadFile(filepath.Join(root, "skills", "brainstorming", "SKILL.md"))
	if !strings.Contains(string(got), "v2") {
		t.Fatalf("vendored copy not updated:\n%s", got)
	}
	if !strings.Contains(out, "changed") { // diffstat line
		t.Fatalf("no diffstat printed:\n%s", out)
	}
	srcs, _ := pack.LoadSources(root)
	if e := srcs.Skill("brainstorming"); e == nil || e.Ref != "v2.0.0" {
		t.Fatalf("provenance not advanced: %+v", e)
	}
}

func TestPackUpdateSkillDivergenceGateMirrorsSync(t *testing.T) {
	root, up := vendoredAuthorPack(t)
	// The author edits the vendored copy after vendoring.
	writeFiles(t, root, map[string]string{
		"skills/brainstorming/SKILL.md": "locally tweaked\n",
	})
	publishV2(t, up)
	before, _ := pack.LoadSources(root)
	out, code := runEscOut(t, root, "pack", "update-skill", "brainstorming")
	if code != 0 {
		t.Fatalf("a skipped update is exit 0 (it keeps reporting), got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "skipped") {
		t.Fatalf("no skip warning:\n%s", out)
	}
	got, _ := os.ReadFile(filepath.Join(root, "skills", "brainstorming", "SKILL.md"))
	if string(got) != "locally tweaked\n" {
		t.Fatal("divergence gate overwrote the author's edit")
	}
	after, _ := pack.LoadSources(root)
	if after.Skill("brainstorming").Commit != before.Skill("brainstorming").Commit {
		t.Fatal("skip must leave sources.yaml unchanged so the divergence keeps reporting")
	}
	// --force converges, exactly like sync --force on a managed region.
	runEsc(t, root, "pack", "update-skill", "brainstorming", "--force")
	got, _ = os.ReadFile(filepath.Join(root, "skills", "brainstorming", "SKILL.md"))
	if !strings.Contains(string(got), "v2") {
		t.Fatal("--force did not converge the vendored copy")
	}
}

func TestPackUpdateSkillAllAndUnknown(t *testing.T) {
	root, up := vendoredAuthorPack(t)
	publishV2(t, up)
	runEsc(t, root, "pack", "update-skill", "--all")
	srcs, _ := pack.LoadSources(root)
	for _, e := range srcs.Skills {
		if e.Ref != "v2.0.0" {
			t.Errorf("%s not advanced by --all: %+v", e.Name, e)
		}
	}
	if _, code := runEscOut(t, root, "pack", "update-skill", "nope"); code == 0 {
		t.Fatal("unknown skill name must fail")
	}
	if _, code := runEscOut(t, root, "pack", "update-skill"); code != 2 {
		t.Fatal("no names and no --all is a usage error")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail** — `go test ./internal/cli/ -run TestPackUpdateSkill -v` — expected: FAIL.

- [ ] **Step 3: Implement**

```go
// internal/cli/packupdate.go
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/tensorgroup/openescapement/internal/pack"
)

// cmdPackUpdateSkill re-vendors named skills (or --all) at the requested ref
// (default: highest semver tag, else default branch head). Divergence gate,
// mirroring sync's hand-edit rule (spec §4): if the vendored copy no longer
// matches its recorded hash, the author edited it after vendoring, so the
// skill is warned about and skipped — and sources.yaml is left untouched so
// the divergence keeps reporting — unless --force.
func cmdPackUpdateSkill(ctx context.Context, root string, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("pack update-skill", flag.ContinueOnError)
	fs.SetOutput(stderr)
	all := fs.Bool("all", false, "update every vendored skill")
	ref := fs.String("ref", "", "ref to vendor (default: highest semver tag, else default branch head)")
	force := fs.Bool("force", false, "overwrite a vendored copy that was edited after vendoring")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if fs.NArg() == 0 && !*all {
		return fmt.Errorf("%w: name at least one skill, or pass --all", errUsage)
	}
	p, err := loadAuthorPack(root, stderr)
	if err != nil {
		return err
	}
	srcs, err := pack.LoadSources(root)
	if err != nil {
		return err
	}
	if srcs == nil || len(srcs.Skills) == 0 {
		return fmt.Errorf("no vendored skills recorded in %s", pack.SourcesFile)
	}
	names := fs.Args()
	if *all {
		names = nil
		for _, e := range srcs.Skills {
			names = append(names, e.Name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		entry := srcs.Skill(name)
		if entry == nil {
			return fmt.Errorf("%s is not a vendored skill (not in %s)", name, pack.SourcesFile)
		}
		dir := vendoredDir(root, p, name)
		cur, err := pack.DirHash(dir)
		if err != nil {
			return fmt.Errorf("hashing skills/%s: %w", name, err)
		}
		if cur != entry.Hash && !*force {
			fmt.Fprintf(stderr, "skipped %s: the vendored copy was edited after vendoring (hash mismatch)\n", name)
			fmt.Fprintf(stderr, "  restore it as vendored, or `esc pack update-skill %s --force` to overwrite\n", name)
			continue
		}
		recorded, fetchRef, err := resolveSkillRef(ctx, entry.Source, *ref, stderr)
		if err != nil {
			return err
		}
		fr, err := fetchSkillSource(ctx, root, entry.Source, entry.Subdir, fetchRef)
		if err != nil {
			return err
		}
		// entry.Subdir points at the skill's own directory (relSkillSubdir),
		// so the fetched dir either IS the skill or holds exactly it.
		found, err := discoverSkills(fr.Dir, name)
		if err != nil {
			return err
		}
		src := ""
		for _, d := range found {
			if d.Name == name {
				src = d.Dir
			}
		}
		if src == "" {
			return fmt.Errorf("%s no longer exists upstream at %s@%s", name, entry.Source, recorded)
		}
		oldHashes, err := fileHashes(dir)
		if err != nil {
			return err
		}
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
		_, hash, err := vendorCopy(src, dir)
		if err != nil {
			return err
		}
		newHashes, err := fileHashes(dir)
		if err != nil {
			return err
		}
		added, removed, changed := diffstat(oldHashes, newHashes)
		fmt.Fprintf(stdout, "%s: %s -> %s (%s), %d added, %d removed, %d changed\n",
			name, entry.Ref, recorded, fr.Commit[:12], added, removed, changed)
		srcs.Upsert(pack.SourceSkill{
			Name: name, Source: entry.Source, Subdir: entry.Subdir,
			Ref: recorded, Commit: fr.Commit, Hash: hash,
		})
	}
	return srcs.Save(root)
}

// vendoredDir resolves the pack-relative directory for a vendored skill from
// the manifest entry whose resolved name matches; falls back to the
// skills/<name>/ convention add-skill writes.
func vendoredDir(root string, p *pack.Pack, name string) string {
	for _, e := range p.Manifest.Skills {
		if e.DirName(p.Manifest.Name) == name {
			return filepath.Join(root, filepath.FromSlash(e.Path))
		}
	}
	return filepath.Join(root, "skills", name)
}
```

- [ ] **Step 4: Run tests** — `go test ./internal/cli/ -run TestPackUpdateSkill -v` — expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/packupdate.go internal/cli/packupdate_test.go
git commit -m "feat(cli): esc pack update-skill re-vendors with a divergence gate

Defaults to the highest semver tag, prints a diffstat, and rewrites copy
plus provenance. A vendored copy that no longer matches its recorded hash
was edited on purpose, so it is warned about and skipped — sources.yaml
untouched, the divergence keeps reporting — exactly the posture sync
takes on a hand-edited managed block; --force converges."
```

---

### Task 12: `esc pack outdated [--check]`

**Files:**
- Create: `internal/cli/packoutdated.go`
- Test: `internal/cli/packoutdated_test.go`

**Interfaces:**
- Consumes: `source.LsRemoteTags`, `source.LsRemoteHash`, `source.HighestSemverTag`, `esc.ErrConstraint`, sources API.
- Produces: `func cmdPackOutdated(ctx context.Context, root string, args []string, stdout, stderr io.Writer) error`

- [ ] **Step 1: Write the failing tests**

```go
// internal/cli/packoutdated_test.go
package cli

import (
	"strings"
	"testing"
)

func TestPackOutdatedReportsAndGates(t *testing.T) {
	root, up := vendoredAuthorPack(t)
	// Fresh vendoring: everything up to date, --check exits 0.
	out, code := runEscOut(t, root, "pack", "outdated", "--check")
	if code != 0 {
		t.Fatalf("up-to-date --check must exit 0, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "up to date") {
		t.Fatalf("missing up-to-date rows:\n%s", out)
	}
	publishV2(t, up)
	out, code = runEscOut(t, root, "pack", "outdated")
	if code != 0 {
		t.Fatalf("bare outdated is informational (exit 0), got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "v2.0.0") || !strings.Contains(out, "behind") {
		t.Fatalf("missing behind rows:\n%s", out)
	}
	// Deterministic ordering: brainstorming before writing-plans.
	if strings.Index(out, "brainstorming") > strings.Index(out, "writing-plans") {
		t.Fatalf("rows not sorted by name:\n%s", out)
	}
	if _, code = runEscOut(t, root, "pack", "outdated", "--check"); code != 1 {
		t.Fatalf("behind --check must exit 1, got %d", code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail** — `go test ./internal/cli/ -run TestPackOutdated -v` — expected: FAIL.

- [ ] **Step 3: Implement**

```go
// internal/cli/packoutdated.go
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/pack"
	"github.com/tensorgroup/openescapement/internal/source"
)

// cmdPackOutdated compares each vendored skill's pinned commit against
// upstream (highest semver tag, or default branch head when there are no
// tags — with a warning, spec §4). Output is deterministic: rows sorted by
// name (Sources.Save keeps them sorted), fixed columns via tabwriter.
// --check exits 1 when anything is behind: the routine, self-healing CI
// semantics exit 1 carries everywhere else (esc.ErrConstraint). A network
// failure is ErrFetch (exit 4): a dead remote must not read as up to date.
func cmdPackOutdated(ctx context.Context, root string, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("pack outdated", flag.ContinueOnError)
	fs.SetOutput(stderr)
	check := fs.Bool("check", false, "exit 1 when any vendored skill is behind upstream")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if _, err := loadAuthorPack(root, stderr); err != nil {
		return err
	}
	srcs, err := pack.LoadSources(root)
	if err != nil {
		return err
	}
	if srcs == nil || len(srcs.Skills) == 0 {
		fmt.Fprintln(stdout, "no vendored skills recorded in sources.yaml")
		return nil
	}
	// One ls-remote per distinct source URL, not per skill.
	tagsBySource := map[string]map[string]string{}
	behind := 0
	tw := tabwriter.NewWriter(stdout, 2, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SKILL\tPINNED\tLATEST\tSTATE")
	for _, e := range srcs.Skills {
		tags, ok := tagsBySource[e.Source]
		if !ok {
			tags, err = source.LsRemoteTags(ctx, e.Source)
			if err != nil {
				return err
			}
			tagsBySource[e.Source] = tags
		}
		latestName, latestCommit, hasTag := source.HighestSemverTag(tags)
		if !hasTag {
			fmt.Fprintf(stderr, "warning: %s has no semver tags; comparing against the default branch head\n", e.Source)
			latestName = "HEAD"
			latestCommit, err = source.LsRemoteHash(ctx, e.Source, "HEAD")
			if err != nil {
				return err
			}
		}
		state := "up to date"
		if latestCommit != e.Commit {
			state = "behind"
			behind++
		}
		fmt.Fprintf(tw, "%s\t%s (%s)\t%s\t%s\n", e.Name, e.Ref, short(e.Commit), latestName, state)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if *check && behind > 0 {
		return fmt.Errorf("%w: %d vendored skill(s) behind upstream", esc.ErrConstraint, behind)
	}
	return nil
}

func short(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}
```

- [ ] **Step 4: Run tests** — `go test ./internal/cli/ -run TestPackOutdated -v` — expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/packoutdated.go internal/cli/packoutdated_test.go
git commit -m "feat(cli): esc pack outdated compares pins against upstream tags

One ls-remote per distinct source, rows sorted by name for deterministic
CI logs. --check maps behind to ErrConstraint (exit 1) — the routine gate
semantics every other exit-1 carries — while a network failure stays exit
4, because a dead remote must never read as up to date."
```

---

### Task 13: MCP float warning (§8)

**Files:**
- Create: `internal/pack/lint.go`
- Modify: `internal/pack/pack.go` (`Load` populates `Warnings`)
- Test: `internal/pack/lint_test.go`

**Interfaces:**
- Produces: `func LintMCP(m MCPSpec) []string` — sorted by server name; `Pack.Warnings` populated.

- [ ] **Step 1: Write the failing test**

```go
// internal/pack/lint_test.go
package pack

import (
	"strings"
	"testing"
)

func servers(s map[string]map[string]any) MCPSpec { return MCPSpec{Servers: s} }

func TestLintMCPFlagsFloatingVersions(t *testing.T) {
	warns := LintMCP(servers(map[string]map[string]any{
		"b-latest": {"command": "npx", "args": []any{"-y", "@scope/server@latest"}},
		"a-nopin":  {"command": "npx", "args": []any{"-y", "@scope/server"}},
		"pinned":   {"command": "npx", "args": []any{"-y", "@scope/server@1.2.3"}},
		"binary":   {"command": "/usr/local/bin/my-server", "args": []any{"--port", "3000"}},
	}))
	if len(warns) != 2 {
		t.Fatalf("want 2 warnings, got %d: %v", len(warns), warns)
	}
	// Deterministic: sorted by server name.
	if !strings.Contains(warns[0], "a-nopin") || !strings.Contains(warns[1], "b-latest") {
		t.Fatalf("warnings unsorted or misattributed: %v", warns)
	}
	if !strings.Contains(warns[1], "@latest") {
		t.Fatalf("@latest warning must name the float: %v", warns[1])
	}
}

func TestLintMCPUvxAndEmpty(t *testing.T) {
	if w := LintMCP(servers(nil)); len(w) != 0 {
		t.Fatalf("no servers, no warnings: %v", w)
	}
	w := LintMCP(servers(map[string]map[string]any{
		"u": {"command": "uvx", "args": []any{"some-tool"}},
	}))
	if len(w) != 1 {
		t.Fatalf("uvx without a pin must warn: %v", w)
	}
}
```

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/pack/ -run TestLintMCP -v` — expected: FAIL.

- [ ] **Step 3: Implement**

```go
// internal/pack/lint.go
package pack

import (
	"fmt"
	"sort"
	"strings"
)

// runnerCommands are launchers that fetch a package at invocation time, where
// an unpinned package spec means every consuming machine may run different
// code (spec §8: version discipline, not vendoring).
var runnerCommands = map[string]bool{"npx": true, "pnpx": true, "bunx": true, "uvx": true}

// LintMCP returns advisory warnings for MCP server definitions that float:
// an explicit @latest anywhere, or a runner (npx/uvx/...) whose package
// argument carries no version pin. Warnings, never errors — some servers are
// legitimately unversioned (a local binary, a checked-in script). Sorted by
// server name for deterministic output.
func LintMCP(m MCPSpec) []string {
	names := make([]string, 0, len(m.Servers))
	for name := range m.Servers {
		names = append(names, name)
	}
	sort.Strings(names)
	var warns []string
	for _, name := range names {
		def := m.Servers[name]
		cmd, _ := def["command"].(string)
		var args []string
		if raw, ok := def["args"].([]any); ok {
			for _, a := range raw {
				if s, ok := a.(string); ok {
					args = append(args, s)
				}
			}
		}
		all := append([]string{cmd}, args...)
		floated := ""
		for _, s := range all {
			if strings.Contains(s, "@latest") {
				floated = fmt.Sprintf("uses %q", s)
				break
			}
		}
		if floated == "" && runnerCommands[cmd] {
			pinned := false
			for _, a := range args {
				if strings.HasPrefix(a, "-") {
					continue
				}
				// "@scope/pkg@1.2.3" or "pkg@1.2.3": a version pin is an "@"
				// after the first character (a leading @ is a scope, not a pin).
				if strings.LastIndex(a, "@") > 0 {
					pinned = true
					break
				}
			}
			if !pinned {
				floated = fmt.Sprintf("runs %s without a version pin", cmd)
			}
		}
		if floated != "" {
			warns = append(warns, fmt.Sprintf("mcp server %q %s; consuming machines may run different code — pin a version", name, floated))
		}
	}
	return warns
}
```

In `pack.Load`, after `p := &Pack{Dir: dir, Manifest: m}`:

```go
	p.Warnings = LintMCP(m.MCP)
```

- [ ] **Step 4: Add a CLI-level assertion**

Append to `internal/cli/packadd_test.go`:

```go
func TestPackCommandsPrintMCPFloatWarning(t *testing.T) {
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	root := newAuthorPack(t)
	writeFiles(t, root, map[string]string{
		"pack.yaml": "schema: 1\nname: acme\nversion: 1.0.0\nmcp:\n  servers:\n    floaty:\n      command: npx\n      args: [\"-y\", \"@scope/server@latest\"]\n",
	})
	out, _ := runEscOut(t, root, "pack", "outdated")
	if !strings.Contains(out, "warning:") || !strings.Contains(out, "floaty") {
		t.Fatalf("pack command did not surface the float warning:\n%s", out)
	}
}
```

- [ ] **Step 5: Run tests** — `go test ./internal/pack/ ./internal/cli/ -run 'TestLintMCP|TestPackCommandsPrint' -v`, then `go test ./...` — expected: PASS (sync/status output is unchanged: only `esc pack` prints warnings).

- [ ] **Step 6: Commit**

```bash
git add internal/pack/lint.go internal/pack/lint_test.go internal/pack/pack.go internal/cli/packadd_test.go
git commit -m "feat(pack): warn when an MCP server definition floats

@latest or an unpinned npx/uvx package means consuming machines may run
different code. A warning, never an error — some servers are legitimately
unversioned — surfaced only on the esc pack authoring commands: the
author can fix it, a consumer at sync time cannot."
```

---

### Task 14: AGENTS.md invariants + final verification

**Files:**
- Modify: `AGENTS.md` (Gotchas and invariants section)

- [ ] **Step 1: Add the new invariants to `AGENTS.md`**

Append these bullets to the "Gotchas and invariants" section (this file is the repo's canonical agent instructions and records every cycle's invariants; this is maintenance of an existing doc, not a new one):

```markdown
- **`skills:` entries are a union type** (`internal/pack/skillentry.go`): a plain string keeps the `esc-<pack>-<dir>` on-disk prefix byte-for-byte (existing packs and lockfiles see no change); an object entry `{path, name}` sets the on-disk name. Conflicts fail closed: duplicate resolved names within one pack are `ErrManifest`; the same `.claude/skills/` path claimed by two configured packs fails the plan with `ErrConfig` (exit 2 — not exit 1, nothing self-heals a collision).
- **Skill-dir ownership is the parent directory, not a name prefix.** Apply's retirement pass and status's orphan pass both use `ownedSkillPath` (exactly one element below `.claude/skills`); the old `esc-` prefix marker broke for name-overridden skills. A hostile lockfile stays confined by `ownedSkillPath` plus manifest-only deletion, with containment and symlink refusal already run before any of it reads the disk — not by the recorded-hash gate, which a hostile entry can simply carry the correct value for; that gate's real job is catching a team member's edit so retirement never deletes it silently.
- **Sync never adopts a directory it did not create.** A `KindDir` artifact with no prior lock entry whose target path already exists is skipped (`SkipUnmanagedDirAtTarget`, exit 0) and reported by status as `occupied` (gates `--check`). `--force` does NOT override this case: force is consent to overwrite escapement's own content, and this content never was. Adoption requires moving the directory aside.
- **`sources.yaml` at the pack root is authoring provenance** (source, subdir, requested ref, resolved commit, dir hash per vendored skill). Consumers never read it; the signed tag covers it. It lives outside skill directories so vendored copies stay byte-identical to upstream and no directory walk filters anything.
- **`esc pack add-skill/update-skill/outdated` are author commands** run in the pack repo. Vendor copies enumerate files via `pack.DirFiles` (the hashing walk — never a second walk). update-skill's divergence gate mirrors sync's hand-edit rule: an edited vendored copy is skipped and `sources.yaml` is left unchanged so it keeps reporting, `--force` converges. `outdated --check` maps "behind" to exit 1; a network failure stays exit 4.
- **The cross-level duplicate finding (`Finding.Duplicate`) carries a skill name and a same/differs bool, nothing else.** It rides on the existing dir finding so it can never change `Clean()` or any exit code, and the redaction walk test pins it as metrics-grade — do not add content-shaped fields to it.
```

- [ ] **Step 2: Full verification**

Run, in order, and read the output before claiming anything:

```bash
gofmt -w . && git diff --stat   # expect: no reformat churn beyond your files
go vet ./...
go test ./...
```

Expected: all PASS. If a golden test fails, inspect the diff and confirm it is exactly an intended new field/line (e.g. usage text, `duplicate` in a JSON report fixture) before regenerating.

- [ ] **Step 3: Commit**

```bash
git add AGENTS.md
git commit -m "docs: record the vendoring-cycle invariants

Schema union semantics, the parent-dir ownership rule, the never-adopt
gate and why --force does not apply to it, sources.yaml's place outside
skill dirs, the author-command posture, and the metrics-grade bound on
the cross-level duplicate finding."
```

---

## Test strategy summary

- Unit: `internal/pack` (SkillEntry union, sources round-trip, LintMCP), `internal/source` (semver table), `internal/cli` (discovery/copy/diffstat, pack.yaml append).
- Integration (real temp git repos, `ESC_CACHE_DIR` isolated, no mocks): add-skill/update-skill/outdated e2e; local unsigned packs for name-override sync, cross-pack collision, retirement, occupied gate, and home-dir duplicates (`t.Setenv("HOME", ...)`).
- Privacy: `internal/publisher/redact_test.go` extended so `duplicate` must survive below `content` and the walk keeps failing on any content-shaped growth.
- Invariant regression: full suite green after Task 1 is the no-lockfile-churn proof for plain entries; hostile/orphan tests re-prove fail-closed after the ownership change.

## Risks and open questions

- **yaml.v3 node round-trip formatting:** comments survive but some whitespace may normalize when `add-skill` rewrites `pack.yaml`. Tests assert content/comments/reparseability, not byte identity. Acceptable for an authoring file the author reviews in the diff.
- **`run` helper and `$HOME`:** if `internal/cli`'s `run` does not already isolate `HOME`, the Task 5 scan reads the real home directory in every cli test; it is a no-op unless a name collides, but hardening `run` with a temp `HOME` is cheap and worth doing while in there.
- **Behavior change in Task 4:** first-sync into a pre-existing target dir used to merge silently; now it skips. Any downstream doc/example that relied on adoption needs the move-aside step.
- **Occupied + deleted lockfile:** deleting `.escapement/escapement.lock` now makes every existing skill dir report `occupied` instead of being overwritten — safer than the documented pre-existing hazard, but a visible behavior shift worth a release note.
