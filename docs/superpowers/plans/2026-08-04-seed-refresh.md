# Seed refresh: guidance upgrades and always-fresh demo data

Status: ready to execute. Spec: `/Users/billyz/code/openescapement/docs/superpowers/specs/2026-08-04-seed-refresh-design.md` (approved, commit 22a4128). Read it before starting; it is authoritative.

## Problem

`guidance.Seed` is create-if-missing only: a guidance dir seeded before the model starters shipped keeps its old `models.yaml` forever, silently hiding the starter blocks and adopt buttons. We add a hash manifest so unedited files refresh across upgrades while hand edits are never touched, and make `esc serve --demo` reset its demo-owned data to pristine on every startup.

## Global Constraints

These hold for every task. A task is not done until all are satisfied.

- **Single external dependency policy.** `gopkg.in/yaml.v3` is the only non-stdlib dependency; the portal's `goldmark` markdown renderer is the one pre-existing exception. This plan adds NO new dependency: hashing uses stdlib `crypto/sha256` + `encoding/hex`, the manifest uses stdlib `encoding/json`. If you reach for anything else, stop.
- **Atomic writes.** Every file this plan writes (seeded files and the `.seeded.json` manifest) is written temp-plus-rename, mirroring the existing `Set.WriteFile` and `store.SaveRegistry` patterns. Never `os.WriteFile` directly onto a live path.
- **Fail-soft posture unchanged.** `guidance.Load` and per-request reads must stay fail-soft (a hand-broken content dir must not take the portal down). `Seed` may return errors as today (`cmdServe` maps them to exit 4). A manifest that will not parse is treated as absent (re-migrate) rather than a hard failure.
- **Sentinel exit codes unchanged.** No new failure modes; `cmdServe` keeps returning 4 on seed/reset errors.
- **Renderer/seed determinism.** Manifest output is deterministic (`encoding/json` sorts map keys), so it is golden-testable.
- **Docs contain no em-dashes.** README and CHANGELOG prose must not use em-dashes or other AI telltales.
- **Gates before done.** `gofmt -w . && go vet ./... && go test ./...` must all pass. The full suite is green after every task, not just at the end.
- **Commits are conventional and carry no AI trailers.** No `Co-Authored-By: Claude`, no generated-with footer. Commit only the files the task touches. Use the exact commit messages given.

## Interfaces

New and changed signatures (all within existing packages):

```go
// internal/guidance/load.go
func Seed(dataDir string) error // signature unchanged; semantics upgraded

func sha256Hex(b []byte) string
func atomicWrite(dst string, b []byte, perm os.FileMode) error
func readManifest(path string) (map[string]string, error)          // (nil, nil) when absent or unparseable
func writeManifest(path string, m map[string]string) error         // atomic
func migrateManifest(root string, recorded map[string]string) error

// internal/cli/serve.go
func resetDemo(dir string, stdout io.Writer) error // removes demo-owned paths, prints one line
```

Manifest file: `<data-dir>/guidance/.seeded.json`, a JSON object of rel path (forward-slash) to lowercase sha256 hex of the content last seeded there. It is not in `KnownFiles` (a fixed allowlist built from the embedded walk), so the portal never lists, reads, or writes it.

---

## Task 1: Guidance seed manifest and three-way refresh semantics

Upgrade `guidance.Seed` per spec §1: manifest-tracked create-or-refresh-unmodified, with migration for pre-manifest dirs. All in `internal/guidance/`.

### Step 1a: Failing tests

Add to `/Users/billyz/code/openescapement/internal/guidance/load_test.go`. These use the package-internal `embeddedModels` FS (tests are `package guidance`) to stand in for "embedded content", and doctor the manifest by hand to simulate an upgrade (we cannot mutate the embedded tree at runtime).

```go
func readManifestForTest(t *testing.T, gdir string) map[string]string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(gdir, ".seeded.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return m
}

func TestSeedRecordsManifestOnCreate(t *testing.T) {
	gdir := seedTemp(t)
	m := readManifestForTest(t, gdir)
	emb, _ := embeddedModels.ReadFile("models/models.yaml")
	if m["models.yaml"] != sha256Hex(emb) {
		t.Fatalf("manifest missing/incorrect models.yaml hash: %v", m["models.yaml"])
	}
	// No temp files left behind by the atomic writes.
	entries, _ := os.ReadDir(gdir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".seeded-") || strings.HasPrefix(e.Name(), ".guidance-") {
			t.Fatalf("leftover temp file: %s", e.Name())
		}
	}
}

// Spec §3 regression: an unedited file whose recorded hash predates an embedded
// upgrade is refreshed, and its record is updated.
func TestSeedRefreshesUnmodifiedFile(t *testing.T) {
	dir := t.TempDir()
	if err := Seed(dir); err != nil {
		t.Fatal(err)
	}
	gdir := filepath.Join(dir, "guidance")

	// Simulate a pre-upgrade world: disk holds OLD content and the manifest
	// records the OLD hash (the file was seeded, never edited).
	old := []byte("schema: 1\nvendors: []\n")
	if err := os.WriteFile(filepath.Join(gdir, "models.yaml"), old, 0o644); err != nil {
		t.Fatal(err)
	}
	m := readManifestForTest(t, gdir)
	m["models.yaml"] = sha256Hex(old)
	data, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(gdir, ".seeded.json"), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Seed(dir); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(gdir, "models.yaml"))
	emb, _ := embeddedModels.ReadFile("models/models.yaml")
	if string(got) != string(emb) {
		t.Fatalf("unmodified file not refreshed to embedded content")
	}
	if reg, err := ParseRegistry(got); err != nil || len(reg.Vendors) == 0 {
		t.Fatalf("refreshed registry lost its vendors: err=%v", err)
	}
	if readManifestForTest(t, gdir)["models.yaml"] != sha256Hex(emb) {
		t.Fatal("manifest not updated after refresh")
	}
}

// Spec §1: a hand-edited file (disk differs from its recorded hash) is left
// untouched and its record is not changed.
func TestSeedLeavesEditedFile(t *testing.T) {
	dir := t.TempDir()
	if err := Seed(dir); err != nil {
		t.Fatal(err)
	}
	gdir := filepath.Join(dir, "guidance")
	edited := []byte("# My notes\n")
	if err := os.WriteFile(filepath.Join(gdir, "anthropic.md"), edited, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Seed(dir); err != nil { // disk != recorded -> leave
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(gdir, "anthropic.md"))
	if string(got) != string(edited) {
		t.Fatalf("edited file was overwritten: %q", got)
	}
}

// Spec §1 migration: a pre-manifest dir records only files that still match
// the current embedded content; non-matching files are left alone and
// unrecorded.
func TestSeedMigratesPreManifestDir(t *testing.T) {
	dir := t.TempDir()
	gdir := filepath.Join(dir, "guidance")
	if err := os.MkdirAll(gdir, 0o755); err != nil {
		t.Fatal(err)
	}
	// One file that matches embedded (provably unmodified)...
	emb, _ := embeddedModels.ReadFile("models/models.yaml")
	if err := os.WriteFile(filepath.Join(gdir, "models.yaml"), emb, 0o644); err != nil {
		t.Fatal(err)
	}
	// ...and one that does not (old-seed or user-edit, indistinguishable).
	custom := []byte("# custom anthropic notes\n")
	if err := os.WriteFile(filepath.Join(gdir, "anthropic.md"), custom, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Seed(dir); err != nil {
		t.Fatal(err)
	}
	m := readManifestForTest(t, gdir)
	if m["models.yaml"] != sha256Hex(emb) {
		t.Fatal("matching pre-manifest file not recorded")
	}
	if _, ok := m["anthropic.md"]; ok {
		t.Fatal("non-matching pre-manifest file should not be recorded")
	}
	got, _ := os.ReadFile(filepath.Join(gdir, "anthropic.md"))
	if string(got) != string(custom) {
		t.Fatalf("non-matching file was overwritten: %q", got)
	}
}
```

Add `"encoding/json"` to the test file imports (`os`, `path/filepath`, `strings`, `testing` are already there or added). Run `go test ./internal/guidance/` and confirm it fails to compile (undefined `sha256Hex`, no manifest) before implementing.

### Step 1b: Implement

In `/Users/billyz/code/openescapement/internal/guidance/load.go`, add to the import block:

```go
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
```

Replace the existing `Seed` function (current lines 152-181, the doc comment and body) with:

```go
const seedManifest = ".seeded.json"

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// atomicWrite writes b to dst via temp-plus-rename, so a concurrent reader
// never sees a partial file. Callers guarantee filepath.Dir(dst) exists.
func atomicWrite(dst string, b []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".seed-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Chmod(name, perm); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, dst); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// readManifest loads the seed manifest. An absent or unparseable manifest
// returns (nil, nil): the caller treats nil as "pre-manifest, migrate". A
// corrupt machine-managed file is not a hard failure (fail-soft posture).
func readManifest(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, nil
	}
	return m, nil
}

// writeManifest atomically persists the manifest. Map keys are marshaled in
// sorted order, so the output is deterministic.
func writeManifest(path string, m map[string]string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return atomicWrite(path, append(data, '\n'), 0o644)
}

// migrateManifest records disk files under root that still byte-match the
// current embedded content (provably unmodified). Non-matching and absent
// files are left unrecorded: old-seed-unmodified and user-edited are
// indistinguishable without history.
func migrateManifest(root string, recorded map[string]string) error {
	return fs.WalkDir(embeddedModels, "models", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || p == "models" {
			return err
		}
		rel := strings.TrimPrefix(p, "models/")
		emb, rerr := embeddedModels.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		disk, derr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if derr != nil {
			return nil // absent -> not recorded
		}
		if sha256Hex(disk) == sha256Hex(emb) {
			recorded[rel] = sha256Hex(emb)
		}
		return nil
	})
}

// Seed writes the embedded guidance tree into <dataDir>/guidance/ with
// create-or-refresh-unmodified semantics tracked by a hash manifest at
// <dataDir>/guidance/.seeded.json. Absent files are written and recorded.
// A file whose disk hash matches its recorded hash (never edited) is
// refreshed when the embedded content has changed, and its record updated.
// A file whose disk hash differs from its recorded hash (hand-edited or
// portal-edited) is left untouched. Files no longer in the embedded tree are
// left on disk with their record retained. Pre-manifest dirs are migrated:
// only files still matching embedded are recorded.
func Seed(dataDir string) error {
	root := filepath.Join(dataDir, "guidance")
	manifestPath := filepath.Join(root, seedManifest)

	recorded, err := readManifest(manifestPath)
	if err != nil {
		return err
	}
	if recorded == nil {
		recorded = map[string]string{}
		if err := migrateManifest(root, recorded); err != nil {
			return err
		}
	}

	err = fs.WalkDir(embeddedModels, "models", func(p string, d fs.DirEntry, err error) error {
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
		emb, rerr := embeddedModels.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		sum := sha256Hex(emb)
		disk, derr := os.ReadFile(dst)
		switch {
		case os.IsNotExist(derr):
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			if err := atomicWrite(dst, emb, 0o644); err != nil {
				return err
			}
			recorded[rel] = sum
		case derr != nil:
			return derr
		default:
			diskSum := sha256Hex(disk)
			if diskSum == recorded[rel] { // unedited
				if diskSum != sum { // embedded changed -> refresh
					if err := atomicWrite(dst, emb, 0o644); err != nil {
						return err
					}
				}
				recorded[rel] = sum
			}
			// else: hand-edited -> leave, record unchanged
		}
		return nil
	})
	if err != nil {
		return err
	}
	return writeManifest(manifestPath, recorded)
}
```

Note: the existing `TestSeedCreatesTreeAndIsIdempotent` stays green unchanged (it edits `anthropic.md` after the first seed, so the second seed sees disk != recorded and leaves it).

### Step 1c: Gates and commit

Run `gofmt -w . && go vet ./... && go test ./...`. All green.

```
git add internal/guidance/load.go internal/guidance/load_test.go
git commit -m "feat(guidance): seed manifest with create-or-refresh-unmodified semantics"
```

---

## Task 2: Reset demo-owned data on every `--demo` startup

Per spec §2: `esc serve --demo` deletes the demo-owned paths under the resolved data dir before seeding, then reseeds, printing one stdout line. Non-demo is unchanged (still refresh-unmodified via Task 1). Demo-owned paths, enumerated from the code, are exactly: `registry.json`, `events.jsonl` (`seed.Demo`/`store`), `packs/` (`seed.Repos` + publish manager), `demo-repo/` (`seed.Repos`), `guidance/` (`guidance.Seed`). The reset deletes only these specific paths, never the data dir itself, so pointing `--demo` at a directory is bounded.

### Step 2a: Failing tests

Create `/Users/billyz/code/openescapement/internal/cli/serve_reset_test.go`:

```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tensorgroup/openescapement/internal/guidance"
	"github.com/tensorgroup/openescapement/internal/portal/seed"
)

func TestResetDemoWipesDemoPathsAndPrints(t *testing.T) {
	dir := t.TempDir()
	// Populate every demo-owned path.
	for _, name := range []string{"registry.json", "events.jsonl"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, sub := range []string{"packs", "demo-repo", "guidance"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := resetDemo(dir, &out); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"registry.json", "events.jsonl", "packs", "demo-repo", "guidance"} {
		if _, err := os.Stat(filepath.Join(dir, p)); !os.IsNotExist(err) {
			t.Fatalf("demo path %s not removed: %v", p, err)
		}
	}
	if !strings.Contains(out.String(), "esc: demo data reset") {
		t.Fatalf("missing reset message: %q", out.String())
	}
}

// Mirrors cmdServe's demo seeding order. A modified guidance note and a
// modified demo-repo file are pristine again after a second demo startup.
func demoStartup(t *testing.T, dir string) {
	t.Helper()
	var out bytes.Buffer
	if err := resetDemo(dir, &out); err != nil {
		t.Fatal(err)
	}
	if err := guidance.Seed(dir); err != nil {
		t.Fatal(err)
	}
	if err := seed.Demo(dir, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := seed.Repos(dir); err != nil {
		t.Fatal(err)
	}
}

func TestDemoSecondStartupRestoresPristine(t *testing.T) {
	dir := t.TempDir()
	demoStartup(t, dir)

	note := filepath.Join(dir, "guidance", "anthropic.md")
	repoFile := filepath.Join(dir, "demo-repo", "CLAUDE.md")
	pristineNote, _ := os.ReadFile(note)
	pristineRepo, _ := os.ReadFile(repoFile)

	if err := os.WriteFile(note, []byte("# tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(repoFile, []byte("# tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	demoStartup(t, dir) // second startup

	if got, _ := os.ReadFile(note); string(got) != string(pristineNote) {
		t.Fatalf("guidance note not reset: %q", got)
	}
	if got, _ := os.ReadFile(repoFile); string(got) != string(pristineRepo) {
		t.Fatalf("demo-repo file not reset: %q", got)
	}
}

// Non-demo startup (guidance.Seed only) keeps a hand edit.
func TestNonDemoStartupKeepsEdits(t *testing.T) {
	dir := t.TempDir()
	if err := guidance.Seed(dir); err != nil {
		t.Fatal(err)
	}
	note := filepath.Join(dir, "guidance", "anthropic.md")
	if err := os.WriteFile(note, []byte("# mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := guidance.Seed(dir); err != nil { // second non-demo startup
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(note); string(got) != "# mine\n" {
		t.Fatalf("non-demo startup overwrote edit: %q", got)
	}
}
```

Run `go test ./internal/cli/` and confirm it fails to compile (`resetDemo` undefined).

### Step 2b: Implement

In `/Users/billyz/code/openescapement/internal/cli/serve.go`, add the helper (near `isLoopback`):

```go
// resetDemo removes the demo-owned paths under dir so `esc serve --demo`
// always boots pristine example data. It deletes only these specific paths,
// never the data dir itself, so the reset is bounded even when --demo is
// pointed at a directory. Non-demo mode never calls this.
func resetDemo(dir string, stdout io.Writer) error {
	for _, p := range []string{"registry.json", "events.jsonl", "packs", "demo-repo", "guidance"} {
		if err := os.RemoveAll(filepath.Join(dir, p)); err != nil {
			return err
		}
	}
	fmt.Fprintln(stdout, "esc: demo data reset")
	return nil
}
```

Then insert the reset call immediately before the existing `guidance.Seed(dir)` (current line 56), so the wipe happens before any seeding:

```go
	if *demo {
		if err := resetDemo(dir, stdout); err != nil {
			fmt.Fprintf(stderr, "esc: %v\n", err)
			return 4
		}
	}

	if err := guidance.Seed(dir); err != nil {
		fmt.Fprintf(stderr, "esc: %v\n", err)
		return 4
	}
```

The existing demo block below (lines 63-77: `seed.Demo` then `seed.Repos`) is unchanged and now runs against a freshly wiped dir, so it rebuilds pristine content. Update the now-stale comment on that block (lines 64-67) from "Re-seed every start ... if they aren't there yet ... a returning demo session keeps whatever the user published last time" to reflect that the reset above already cleared prior demo state, so `seed.Repos` rebuilds from scratch each start.

### Step 2c: Gates and commit

Run `gofmt -w . && go vet ./... && go test ./...`. All green (including the existing `TestDemoPublishSyncLoop`, which calls `seed.Repos` directly and is unaffected).

```
git add internal/cli/serve.go internal/cli/serve_reset_test.go
git commit -m "feat(portal): reset demo-owned data on every --demo startup"
```

---

## Task 3: Docs

Spec §4: README guidance paragraph replaces the delete-to-re-seed caveat with refresh-unmodified; demo section states reset-on-startup and the discard trade; CHANGELOG bullet. No em-dashes.

### Step 3a: README guidance paragraph

In `/Users/billyz/code/openescapement/README.md`, replace the last sentence of the Models paragraph (line 82) that reads:

> The guidance ships embedded and is seeded to your data directory on first run, so you can hand-edit it or edit it from the portal.

with:

> The guidance ships embedded and is seeded to `<data-dir>/guidance/` on first run. Files you have not edited are refreshed to the latest embedded content on later runs, so upgrades reach you automatically; any file you edit by hand or from the portal is never overwritten. To restore a shipped file, delete it and restart.

Keep the trailing "Raw HTML in guidance or fragment markdown is dropped by the safe renderer; write markdown." sentence.

### Step 3b: README demo note

In the `esc serve` section, after the paragraph ending "same as any other pack source." (line 76), add a new paragraph:

> `esc serve --demo` resets its example data to pristine on every startup and prints `esc: demo data reset`, so the demo always shows current content. Anything you change during a demo (published pack versions, adopted starters, guidance edits) is discarded when you restart. Non-demo servers keep your data and only refresh guidance files you have not edited.

### Step 3c: CHANGELOG

In `/Users/billyz/code/openescapement/CHANGELOG.md`, under `## [Unreleased]`, add a `### Changed` subsection (after the existing `### Added` block):

```
### Changed
- Guidance seeding now refreshes unedited files to the latest embedded content
  instead of create-if-missing only, tracked by a
  `<data-dir>/guidance/.seeded.json` hash manifest; hand-edited and
  portal-edited files are still never overwritten, and pre-manifest dirs are
  migrated by recording only files that still match the shipped content.
- `esc serve --demo` now resets its demo-owned data (org store, pack repos,
  demo repo, guidance) to pristine on every startup and prints
  `esc: demo data reset`. Non-demo servers are unaffected.
```

Verify no em-dashes were introduced in any of the three edits (search the diff for the character).

### Step 3d: Commit

No code changed, so gates are a formality; still run `go test ./...` to confirm green.

```
git add README.md CHANGELOG.md
git commit -m "docs: seed refresh and demo reset in README and CHANGELOG"
```

---

## Task 4: Final verification

1. Run the full gate suite and confirm each passes with visible output:
   - `gofmt -l .` prints nothing.
   - `go vet ./...` exits 0.
   - `go test ./...` all packages pass.
2. `go build ./cmd/esc` succeeds.
3. Confirm no new dependency crept in: `git diff main -- go.mod go.sum` is empty.
4. Manual demo double-start walkthrough (dispatch to a controller / run locally):
   - `esc serve --demo --data-dir <tmp>` in the background; confirm stdout shows `esc: demo data reset` and the portal URL. Load the portal, confirm the Models pages render starters and adopt buttons, and the Packs page shows org-baseline history.
   - Edit `<tmp>/demo-repo/CLAUDE.md` and `<tmp>/guidance/anthropic.md` on disk. Stop the server (SIGINT).
   - Restart with the same command; confirm `esc: demo data reset` prints again and both edited files are back to their pristine shipped content.
   - Start once in non-demo mode against a fresh `--data-dir`, edit a guidance note, restart, confirm the edit survives (refresh-unmodified leaves edits alone).
5. Inspect `<tmp>/guidance/.seeded.json`: it exists, is valid JSON with sorted keys, is not surfaced anywhere in the portal UI, and `esc`'s `KnownFiles` rejects it (portal cannot open or overwrite it).

## Risks and notes

- **Cannot mutate embedded content in tests.** The refresh path is exercised by doctoring the manifest to an old hash (Step 1a regression test), which is behaviorally equivalent to an embedded upgrade. Accept this; it is the only way without a second embedded fixture.
- **Corrupt manifest is fail-soft.** `readManifest` treats an unparseable `.seeded.json` as absent and re-migrates rather than failing startup. This can, in a pathological hand-edit, leave an already-edited file recorded as unmodified (and thus refreshable). Acceptable: the manifest is machine-managed and hand edits are explicitly unsupported (spec §1).
- **Deleting a seeded file restores it.** An unedited file the user deletes is treated as absent and re-created on the next seed. This is the documented "delete and restart to restore" affordance, not a bug.
- **Migration one-time gap** (spec §1): pre-manifest files that differ from current embedded stay frozen until hand-refreshed. In demo mode this is moot (Task 2 wipes and reseeds); in non-demo it is the accepted trade.

## Self-review

- Every task ends green (`go test ./...`) and each has an exact conventional commit message with no AI trailer.
- No forward references: `sha256Hex`/`resetDemo` are defined in the same task whose tests use them.
- No new dependency; hashing, manifest, and JSON are all stdlib. Task 4 asserts `go.mod`/`go.sum` are untouched.
- All writes (seed files, manifest) are atomic; `Load` and per-request reads keep their fail-soft posture.
- Demo-owned path set is enumerated from the code (`registry.json`, `events.jsonl`, `packs`, `demo-repo`, `guidance`) and the reset never touches the data dir itself.
- Docs edits give exact before/after text and are checked for em-dashes.