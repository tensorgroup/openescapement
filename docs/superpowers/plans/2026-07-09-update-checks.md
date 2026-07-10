# Update-Check (client half) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `esc` a piggyback update-freshness check: packs declare an `update_check` cadence; overdue `esc` commands do a lightweight `git ls-remote` staleness probe, log it, optionally prompt, and surface `pack-stale`/`check-overdue` status findings — all inert unless a pack opts in.

**Architecture:** A new `internal/updatecheck` package owns the per-clone JSONL log (`.escapement/update-log.jsonl`), the cadence math, the `git ls-remote` staleness core, the throttle (`Maybe`), and the sync-time recorder (`RecordSync`). `internal/pack` gains the `update_check` manifest field plus a day-aware duration parser. `internal/source` gains two `ls-remote` helpers that reuse the existing git arg-injection hygiene. `internal/engine`'s `Status` reads the log to emit two new finding states. `internal/cli` wires the throttle into `status`/`diff`/`update`/`render`, records a check after `sync`, and scaffolds the gitignore. No import cycles: `updatecheck` imports `pack`/`config`/`lockfile`/`source`; `engine` and `cli` import `updatecheck`; `updatecheck` imports neither `engine` nor `cli`.

**Tech Stack:** Go 1.24, stdlib only (`encoding/json`, `os/exec` via existing `source.git`, `bufio`, `strconv`, `time`), `gopkg.in/yaml.v3` for the manifest field. System `git` for `ls-remote`.

## Global Constraints

- Single external dependency policy: `gopkg.in/yaml.v3` only; everything else stdlib; system `git` via `os/exec`. No new deps, ever.
- Invariants: bytes outside a managed block are never modified; nothing is written after a verification or constraint failure; all writes atomic (temp file + rename); renderer output deterministic.
- Sentinel errors in `internal/esc` map to exit codes: 0 ok, 1 drift/constraint, 2 usage, 3 integrity/signature, 4 other.
- Tests: integration tests build real temp git repos — no mocks (see `internal/cli/cli_test.go` and `internal/source/source_test.go`). `ESC_CACHE_DIR` overrides the pack cache and tests rely on it. TDD per task.
- Follow existing git arg-injection guards (`internal/source`): every `ls-remote` against a stored source URL goes through the same hygiene (`validRef`, reject `-`-prefixed URLs, `--` separator).
- gofmt/vet clean; `go test ./...` green after every task.
- Feature is inert by construction: if no installed pack declares `update_check`, there are no checks, no log writes, and no findings.

### Design resolutions baked into this plan (do not re-litigate)

- **Cadence lives in the per-clone log, not the lockfile.** Each check/sync entry carries the strictest `every` string. The cheap throttle reads cadence from the latest log entry (no fetch needed); `esc status` and `esc sync` recompute it fresh from loaded manifests. Making the feature inert = drop `update_check` from all packs and `esc sync` (writes a terminal empty-cadence entry).
- **Staleness (matches the real v0.1 pin/lock model):**
  - A pin whose `ref` parses as semver (`v?MAJOR.MINOR.PATCH[-pre]`) is a **tag pin**. "Updates available" = a newer *stable* (non-prerelease) semver tag exists upstream (`git ls-remote --tags`). Prerelease tags are excluded from the "latest" candidate set; a stable release of the same core version counts as newer than a prerelease pin.
  - Any other `ref` (branch name, SHA) is a **branch pin**. "Updates available" = the remote ref hash (`git ls-remote <url> <ref>`) differs from the lockfile's resolved `Commit`.
  - Local plain-dir sources (`source.Parsed.Local`) are skipped entirely — dev mode, edits already surface as ordinary drift.
- **Accept action (corrects the spec's imprecise "Enter runs `esc sync`"):** for tag pins, `esc sync` alone keeps the old pin, so accept first re-pins each stale **tag** pack to its latest tag (the `esc update --ref <latest>` equivalent) and then syncs; branch pins need no re-pin (sync picks up the new tip).
- **Endpoint:** parsed and validated (`https://` required) for forward-compat, but **never contacted** in v0.1 — the check always uses `git ls-remote`.
- **Findings ride the existing mechanism:** two new `engine.State` values (`pack-stale`, `check-overdue`) are non-`InSync`, so `StatusResult.Clean()` returns false and `esc status --check` returns exit 1 (the existing exit-1 drift family). No new sentinel error.

---

### Task 1: `update_check` manifest field + day-aware duration parser

**Files:**
- Modify: `internal/pack/pack.go` (imports; `Manifest` struct ~line 19-29; `validate` ~line 104-149; add `UpdateCheck` type + `ParseEvery`)
- Test: `internal/pack/updatecheck_field_test.go` (create)

**Interfaces:**
- Produces:
  - `type UpdateCheck struct { Every string; Endpoint string }`
  - field `Manifest.UpdateCheck *UpdateCheck` (yaml `update_check,omitempty`)
  - `func ParseEvery(s string) (time.Duration, error)` — accepts `"7d"`, `"24h"`, `"90m"`, `"30s"`; `d` = 24h; rejects empty/zero/negative/unknown.

- [ ] **Step 1: Write the failing test**

```go
// internal/pack/updatecheck_field_test.go
package pack

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseEvery(t *testing.T) {
	ok := map[string]time.Duration{
		"7d":  7 * 24 * time.Hour,
		"1d":  24 * time.Hour,
		"24h": 24 * time.Hour,
		"90m": 90 * time.Minute,
		"30s": 30 * time.Second,
	}
	for in, want := range ok {
		got, err := ParseEvery(in)
		if err != nil || got != want {
			t.Errorf("ParseEvery(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "0d", "-1h", "7", "d", "7w", "1.5d", "abc"} {
		if _, err := ParseEvery(bad); err == nil {
			t.Errorf("ParseEvery(%q): want error", bad)
		}
	}
}

// loadManifestPack writes a one-file pack with the given pack.yaml and loads it.
func loadManifestPack(t *testing.T, manifest string) (*Pack, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("## A\nx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return Load(dir)
}

func TestManifestUpdateCheck(t *testing.T) {
	base := "schema: 1\nname: p\nversion: 1.0.0\nrules: [a.md]\n"
	// Valid: every + https endpoint.
	p, err := loadManifestPack(t, base+"update_check:\n  every: 7d\n  endpoint: https://cp.example/check\n")
	if err != nil {
		t.Fatalf("valid update_check rejected: %v", err)
	}
	if p.Manifest.UpdateCheck == nil || p.Manifest.UpdateCheck.Every != "7d" {
		t.Fatalf("update_check not parsed: %+v", p.Manifest.UpdateCheck)
	}
	// Absent → nil, no error.
	p, err = loadManifestPack(t, base)
	if err != nil || p.Manifest.UpdateCheck != nil {
		t.Fatalf("absent update_check: %+v err %v", p, err)
	}
	// Bad every → manifest error.
	if _, err := loadManifestPack(t, base+"update_check:\n  every: 7w\n"); err == nil {
		t.Error("bad every: want error")
	}
	// Non-https endpoint → manifest error.
	if _, err := loadManifestPack(t, base+"update_check:\n  every: 7d\n  endpoint: http://cp.example\n"); err == nil {
		t.Error("http endpoint: want error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/pack/ -run 'TestParseEvery|TestManifestUpdateCheck' -v`
Expected: FAIL — `undefined: ParseEvery` and unknown field `update_check`.

- [ ] **Step 3: Add imports, the type, the struct field, and the parser**

In `internal/pack/pack.go`, extend the import block (add `strconv` and `time`):

```go
import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/tensorgroup/openescapement/internal/esc"
)
```

Add the `UpdateCheck` field to `Manifest` (after `Constraints Constraints` line):

```go
	Constraints Constraints    `yaml:"constraints"`
	UpdateCheck *UpdateCheck   `yaml:"update_check,omitempty"`
}

// UpdateCheck declares how often clients should probe this pack's source for
// newer versions. Endpoint is parsed for forward-compatibility (a future
// control-plane check surface) but is NEVER contacted in v0.1 — the client
// always uses `git ls-remote` against the pack's git source.
type UpdateCheck struct {
	Every    string `yaml:"every"`
	Endpoint string `yaml:"endpoint,omitempty"`
}
```

Add `ParseEvery` at the end of the file:

```go
// ParseEvery parses an update-check cadence. Go's time.ParseDuration has no
// day unit, so "<n>d" is handled explicitly; all other units delegate to the
// stdlib. The result must be strictly positive.
func ParseEvery(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid day duration %q (use e.g. 7d)", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q (use e.g. 7d, 24h, 90m)", s)
	}
	if d <= 0 {
		return 0, fmt.Errorf("duration must be positive: %q", s)
	}
	return d, nil
}
```

- [ ] **Step 4: Validate `update_check` in `Manifest.validate`**

In `internal/pack/pack.go`, add this block to `validate` immediately before its final `return nil`:

```go
	if m.UpdateCheck != nil {
		if _, err := ParseEvery(m.UpdateCheck.Every); err != nil {
			return fail("update_check.every: %v", err)
		}
		if e := m.UpdateCheck.Endpoint; e != "" && !strings.HasPrefix(e, "https://") {
			return fail("update_check.endpoint %q: must be an https:// URL", e)
		}
	}
	return nil
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/pack/ -v`
Expected: PASS (new tests plus existing `pack_test.go` all ok).

- [ ] **Step 6: gofmt/vet and commit**

```bash
gofmt -w internal/pack/pack.go internal/pack/updatecheck_field_test.go
go vet ./internal/pack/
git add internal/pack/pack.go internal/pack/updatecheck_field_test.go
git commit -m "feat(pack): parse and validate update_check manifest field"
```

---

### Task 2: `git ls-remote` source helpers

**Files:**
- Modify: `internal/source/source.go` (add two exported functions; reuse `git`, `validRef`, `esc.ErrFetch`)
- Test: `internal/source/lsremote_test.go` (create)

**Interfaces:**
- Consumes: `source.git(ctx, dir, args...)`, `source.validRef`, `esc.ErrFetch` (all existing in `internal/source`).
- Produces:
  - `func LsRemoteTags(ctx context.Context, url string) (map[string]string, error)` — tag name (without `refs/tags/`) → commit hash. Annotated-tag `^{}` deref lines override the tag-object hash.
  - `func LsRemoteHash(ctx context.Context, url, ref string) (string, error)` — commit hash the remote resolves `ref` to; `""` if the ref is absent.

- [ ] **Step 1: Write the failing test**

```go
// internal/source/lsremote_test.go
package source

import (
	"context"
	"testing"
)

func TestLsRemoteTags(t *testing.T) {
	repo := initGitRepo(t, packFiles(), "v1.0.0")
	url := "file://" + repo
	tags, err := LsRemoteTags(context.Background(), url)
	if err != nil {
		t.Fatalf("LsRemoteTags: %v", err)
	}
	h, ok := tags["v1.0.0"]
	if !ok || len(h) != 40 {
		t.Fatalf("v1.0.0 not resolved: %v", tags)
	}
	// Option-shaped URL is rejected before reaching git.
	if _, err := LsRemoteTags(context.Background(), "--upload-pack=evil"); err == nil {
		t.Error("option-shaped url: want error")
	}
}

func TestLsRemoteHash(t *testing.T) {
	repo := initGitRepo(t, packFiles(), "v1.0.0")
	url := "file://" + repo
	h, err := LsRemoteHash(context.Background(), url, "main")
	if err != nil || len(h) != 40 {
		t.Fatalf("LsRemoteHash(main) = %q, %v", h, err)
	}
	// Absent ref → empty, no error.
	h, err = LsRemoteHash(context.Background(), url, "nope")
	if err != nil || h != "" {
		t.Fatalf("absent ref = %q, %v; want '', nil", h, err)
	}
	// Injection-shaped ref is rejected.
	if _, err := LsRemoteHash(context.Background(), url, "--exec=evil"); err == nil {
		t.Error("bad ref: want error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/source/ -run 'TestLsRemote' -v`
Expected: FAIL — `undefined: LsRemoteTags` / `LsRemoteHash`.

- [ ] **Step 3: Implement the helpers**

Append to `internal/source/source.go`:

```go
// LsRemoteTags returns tag name -> commit hash for all tags at url, without
// fetching any content. It reuses the same arg-injection hygiene as Fetch.
func LsRemoteTags(ctx context.Context, url string) (map[string]string, error) {
	if strings.HasPrefix(url, "-") {
		return nil, fmt.Errorf("%w: source %q looks like a command-line option", esc.ErrFetch, url)
	}
	out, err := git(ctx, "", "ls-remote", "--tags", "--", url)
	if err != nil {
		return nil, fmt.Errorf("%w: ls-remote %s: %v", esc.ErrFetch, url, err)
	}
	tags := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		hash, ref := fields[0], fields[1]
		name := strings.TrimPrefix(ref, "refs/tags/")
		if name == ref {
			continue // not a tag ref
		}
		// Annotated tags emit both refs/tags/X (tag object) and
		// refs/tags/X^{} (the commit). Prefer the dereferenced commit.
		deref := strings.HasSuffix(name, "^{}")
		name = strings.TrimSuffix(name, "^{}")
		if _, seen := tags[name]; !seen || deref {
			tags[name] = hash
		}
	}
	return tags, nil
}

// LsRemoteHash returns the commit hash url resolves ref to, or "" if absent.
func LsRemoteHash(ctx context.Context, url, ref string) (string, error) {
	if strings.HasPrefix(url, "-") {
		return "", fmt.Errorf("%w: source %q looks like a command-line option", esc.ErrFetch, url)
	}
	if !validRef.MatchString(ref) {
		return "", fmt.Errorf("%w: ref %q contains disallowed characters", esc.ErrFetch, ref)
	}
	out, err := git(ctx, "", "ls-remote", "--", url, ref)
	if err != nil {
		return "", fmt.Errorf("%w: ls-remote %s %s: %v", esc.ErrFetch, url, ref, err)
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			return fields[0], nil
		}
	}
	return "", nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/source/ -v`
Expected: PASS (new + existing).

- [ ] **Step 5: gofmt/vet and commit**

```bash
gofmt -w internal/source/source.go internal/source/lsremote_test.go
go vet ./internal/source/
git add internal/source/source.go internal/source/lsremote_test.go
git commit -m "feat(source): add ls-remote tag and ref helpers with injection guards"
```

---

### Task 3: `updatecheck` core — types, semver, cadence, `check`

**Files:**
- Create: `internal/updatecheck/updatecheck.go`
- Test: `internal/updatecheck/updatecheck_test.go`

**Interfaces:**
- Consumes: `source.ParseSource`, `source.LsRemoteTags`, `source.LsRemoteHash`, `pack.ParseEvery`, `pack.Pack`, `config.Config`, `lockfile.Lock`.
- Produces:
  - `type Outcome string` with consts `OutcomeOKCurrent="ok_current"`, `OutcomeOKUpdates="ok_updates"`, `OutcomeError="error"`.
  - `type PackStatus struct { Source, Kind, Pinned, Latest string; Updates bool }` (json tags: `source,kind,pinned,latest,updates`).
  - `func Cadence(packs []*pack.Pack) (time.Duration, string)` — strictest cadence + its raw string; `(0, "")` if none.
  - `func check(ctx, cfg *config.Config, lock *lockfile.Lock) ([]PackStatus, error)` (unexported).
  - `const checkTimeout = 10 * time.Second`.

- [ ] **Step 1: Write the failing test**

```go
// internal/updatecheck/updatecheck_test.go
package updatecheck

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/lockfile"
)

func TestSemverOrdering(t *testing.T) {
	less := func(a, b string) bool {
		sa, _ := parseSemver(a)
		sb, _ := parseSemver(b)
		return semverLess(sa, sb)
	}
	if !less("v1.0.0", "v1.0.1") || !less("v1.9.0", "v2.0.0") {
		t.Error("core ordering wrong")
	}
	if !less("v1.0.0-rc1", "v1.0.0") {
		t.Error("prerelease should sort below release")
	}
	if less("v1.0.0", "v1.0.0-rc1") {
		t.Error("release should not sort below its prerelease")
	}
	if _, ok := parseSemver("main"); ok {
		t.Error("non-semver parsed as semver")
	}
	if _, ok := parseSemver("v1.0.0"); !ok {
		t.Error("semver rejected")
	}
	tags := map[string]string{"v1.0.0": "a", "v2.0.0": "b", "v2.1.0-rc1": "c"}
	got, ok := maxStableSemver(tags)
	if !ok || got != "v2.0.0" {
		t.Errorf("maxStableSemver = %q,%v; want v2.0.0 (prerelease excluded)", got, ok)
	}
}

// gitCommit stages all files and commits, returning nothing.
func gitCommit(t *testing.T, dir, msg string) {
	t.Helper()
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", msg}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// newRepo builds a git repo tagged v1.0.0 on branch main.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("one\n"), 0o644)
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "t@e.com"},
		{"config", "user.name", "T"},
		{"config", "commit.gpgsign", "false"},
		{"config", "tag.gpgsign", "false"},
		{"add", "-A"}, {"commit", "-q", "-m", "one"},
		{"tag", "-a", "v1.0.0", "-m", "v1"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func tagRepo(t *testing.T, dir, tag string) {
	t.Helper()
	cmd := exec.Command("git", "tag", "-a", tag, "-m", tag)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git tag %s: %v\n%s", tag, err, out)
	}
}

func TestCheckTagPinNewerTag(t *testing.T) {
	repo := newRepo(t)
	cfg := &config.Config{Schema: 1, Packs: []config.PackRef{
		{Source: "file://" + repo, Ref: "v1.0.0", Trust: "unsigned"},
	}}
	// No newer tag yet.
	st, err := check(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(st) != 1 || st[0].Kind != "tag" || st[0].Updates {
		t.Fatalf("want no updates, got %+v", st)
	}
	// Publish v2.0.0.
	os.WriteFile(filepath.Join(repo, "f.txt"), []byte("two\n"), 0o644)
	gitCommit(t, repo, "two")
	tagRepo(t, repo, "v2.0.0")
	st, _ = check(context.Background(), cfg, nil)
	if !st[0].Updates || st[0].Latest != "v2.0.0" {
		t.Fatalf("want updates to v2.0.0, got %+v", st[0])
	}
}

func TestCheckBranchPinMoved(t *testing.T) {
	repo := newRepo(t)
	head := func() string {
		cmd := exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = repo
		out, _ := cmd.Output()
		return string(out[:40])
	}
	cfg := &config.Config{Schema: 1, Packs: []config.PackRef{
		{Source: "file://" + repo, Ref: "main", Trust: "unsigned"},
	}}
	lock := &lockfile.Lock{Schema: 1, Packs: []lockfile.LockPack{
		{Source: "file://" + repo, Ref: "main", Commit: head()},
	}}
	// Locked at current tip → no updates.
	st, err := check(context.Background(), cfg, lock)
	if err != nil || st[0].Kind != "branch" || st[0].Updates {
		t.Fatalf("want no updates on branch, got %+v err %v", st, err)
	}
	// Advance main → updates.
	os.WriteFile(filepath.Join(repo, "f.txt"), []byte("two\n"), 0o644)
	gitCommit(t, repo, "two")
	st, _ = check(context.Background(), cfg, lock)
	if !st[0].Updates {
		t.Fatalf("want updates after branch moved, got %+v", st[0])
	}
}

func TestCheckSkipsLocalDirs(t *testing.T) {
	cfg := &config.Config{Schema: 1, Packs: []config.PackRef{
		{Source: "/abs/local/pack", Ref: "", Trust: "unsigned"},
	}}
	st, err := check(context.Background(), cfg, nil)
	if err != nil || len(st) != 0 {
		t.Fatalf("local dir should be skipped, got %+v err %v", st, err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/updatecheck/ -run 'TestSemver|TestCheck' -v`
Expected: FAIL — package `internal/updatecheck` does not compile (`undefined: parseSemver`, `check`, etc.).

- [ ] **Step 3: Implement the core file**

```go
// internal/updatecheck/updatecheck.go

// Package updatecheck implements the client-side update-freshness check:
// cadence math, a git ls-remote staleness probe, a per-clone JSONL log, the
// invocation throttle, and the sync-time recorder. It is inert unless an
// installed pack declares update_check.
package updatecheck

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/lockfile"
	"github.com/tensorgroup/openescapement/internal/pack"
	"github.com/tensorgroup/openescapement/internal/source"
)

// checkTimeout bounds the ls-remote probe so a dead remote can never stall an
// interactive command.
const checkTimeout = 10 * time.Second

// Outcome classifies one check attempt.
type Outcome string

const (
	OutcomeOKCurrent Outcome = "ok_current"
	OutcomeOKUpdates Outcome = "ok_updates"
	OutcomeError     Outcome = "error"
)

// PackStatus is the pinned-vs-latest comparison for one pack.
type PackStatus struct {
	Source  string `json:"source"`
	Kind    string `json:"kind"` // "tag" | "branch"
	Pinned  string `json:"pinned"`
	Latest  string `json:"latest"`
	Updates bool   `json:"updates"`
}

// Cadence returns the strictest (shortest) declared cadence across packs and
// its raw string. Returns (0, "") when no pack declares update_check.
func Cadence(packs []*pack.Pack) (time.Duration, string) {
	var min time.Duration
	var str string
	for _, p := range packs {
		if p.Manifest.UpdateCheck == nil {
			continue
		}
		d, err := pack.ParseEvery(p.Manifest.UpdateCheck.Every)
		if err != nil || d <= 0 {
			continue
		}
		if min == 0 || d < min {
			min, str = d, p.Manifest.UpdateCheck.Every
		}
	}
	return min, str
}

// check probes each non-local pack's git source for newer versions. It fetches
// no content. A declared update_check.endpoint is intentionally ignored in
// v0.1 — the probe always uses git ls-remote.
func check(ctx context.Context, cfg *config.Config, lock *lockfile.Lock) ([]PackStatus, error) {
	var out []PackStatus
	for _, ref := range cfg.Packs {
		parsed, err := source.ParseSource(ref.Source)
		if err != nil {
			return nil, err
		}
		if parsed.Local {
			continue // dev-mode local dirs surface as ordinary drift
		}
		st := PackStatus{Source: ref.Source, Pinned: ref.Ref}
		if _, ok := parseSemver(ref.Ref); ok {
			st.Kind = "tag"
			tags, err := source.LsRemoteTags(ctx, parsed.URL)
			if err != nil {
				return nil, err
			}
			if latest, ok := maxStableSemver(tags); ok {
				st.Latest = latest
				pv, _ := parseSemver(ref.Ref)
				lv, _ := parseSemver(latest)
				st.Updates = semverLess(pv, lv)
			} else {
				st.Latest = ref.Ref
			}
		} else {
			st.Kind = "branch"
			hash, err := source.LsRemoteHash(ctx, parsed.URL, ref.Ref)
			if err != nil {
				return nil, err
			}
			st.Latest = short(hash)
			if lp := lock.Pack(ref.Source, ref.Ref); lp != nil && lp.Commit != "" && hash != "" {
				st.Updates = hash != lp.Commit
			}
		}
		out = append(out, st)
	}
	return out, nil
}

func short(hash string) string {
	if len(hash) > 12 {
		return hash[:12]
	}
	return hash
}

type semver struct {
	major, minor, patch int
	pre                 string
}

// parseSemver accepts v?MAJOR.MINOR.PATCH[-prerelease]. Anything else is not a
// tag pin (branch/SHA) and returns ok=false.
func parseSemver(s string) (semver, bool) {
	s = strings.TrimPrefix(s, "v")
	core, pre := s, ""
	if i := strings.IndexByte(s, '-'); i >= 0 {
		core, pre = s[:i], s[i+1:]
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return semver{}, false
	}
	var n [3]int
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil || v < 0 {
			return semver{}, false
		}
		n[i] = v
	}
	return semver{n[0], n[1], n[2], pre}, true
}

// semverLess reports a < b. For equal cores, a prerelease is lower than the
// bare release; two prereleases compare lexically (documented approximation).
func semverLess(a, b semver) bool {
	if a.major != b.major {
		return a.major < b.major
	}
	if a.minor != b.minor {
		return a.minor < b.minor
	}
	if a.patch != b.patch {
		return a.patch < b.patch
	}
	if a.pre == b.pre {
		return false
	}
	if a.pre == "" {
		return false // a is release; not less than b
	}
	if b.pre == "" {
		return true // a is prerelease, b is release
	}
	return a.pre < b.pre
}

// maxStableSemver returns the greatest non-prerelease tag name, or ok=false.
func maxStableSemver(tags map[string]string) (string, bool) {
	var best semver
	var bestName string
	found := false
	for name := range tags {
		v, ok := parseSemver(name)
		if !ok || v.pre != "" {
			continue // skip non-semver and prerelease tags
		}
		if !found || semverLess(best, v) {
			best, bestName, found = v, name, true
		}
	}
	return bestName, found
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/updatecheck/ -run 'TestSemver|TestCheck' -v`
Expected: PASS.

- [ ] **Step 5: gofmt/vet and commit**

```bash
gofmt -w internal/updatecheck/
go vet ./internal/updatecheck/
git add internal/updatecheck/updatecheck.go internal/updatecheck/updatecheck_test.go
git commit -m "feat(updatecheck): staleness core with semver + branch comparison"
```

---

### Task 4: Per-clone JSONL log store

**Files:**
- Create: `internal/updatecheck/log.go`
- Test: `internal/updatecheck/log_test.go`

**Interfaces:**
- Consumes: `config.Dir`.
- Produces:
  - `type Entry struct { Time time.Time; Outcome Outcome; Cadence string; Packs []PackStatus; Prompt string }` (json: `time,outcome,cadence,packs,prompt`; `cadence,packs` omitempty).
  - `func LoadLog(root string) ([]Entry, error)` — `(nil, nil)` when the file is absent.
  - `func LastEntry(es []Entry) *Entry` / `func LastSuccess(es []Entry) *Entry` (most recent `ok_*`).
  - `func appendLog(root string, e Entry) error` (unexported) — appends, trims to last `maxEntries` (50), writes atomically, ensures the gitignore entry.
  - `const maxEntries = 50`.

- [ ] **Step 1: Write the failing test**

```go
// internal/updatecheck/log_test.go
package updatecheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogRoundTripTrimAndGitignore(t *testing.T) {
	root := t.TempDir()
	if es, err := LoadLog(root); err != nil || es != nil {
		t.Fatalf("empty log = %v, %v; want nil,nil", es, err)
	}
	for i := 0; i < 60; i++ {
		e := Entry{Time: time.Now().UTC(), Outcome: OutcomeOKCurrent, Cadence: "7d", Prompt: "none"}
		if i == 59 {
			e.Outcome = OutcomeError // most recent is an error
		}
		if err := appendLog(root, e); err != nil {
			t.Fatal(err)
		}
	}
	es, err := LoadLog(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != maxEntries {
		t.Fatalf("trim: got %d entries, want %d", len(es), maxEntries)
	}
	if LastEntry(es).Outcome != OutcomeError {
		t.Error("LastEntry should be the error entry")
	}
	if LastSuccess(es) == nil || LastSuccess(es).Outcome != OutcomeOKCurrent {
		t.Error("LastSuccess should skip the trailing error")
	}
	// Gitignore scaffolded with the log filename.
	gi, err := os.ReadFile(filepath.Join(root, ".escapement", ".gitignore"))
	if err != nil || !strings.Contains(string(gi), "update-log.jsonl") {
		t.Errorf(".gitignore not ensured: %q err %v", gi, err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/updatecheck/ -run TestLogRoundTrip -v`
Expected: FAIL — `undefined: LoadLog` / `appendLog` / `Entry`.

- [ ] **Step 3: Implement the log store**

```go
// internal/updatecheck/log.go
package updatecheck

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tensorgroup/openescapement/internal/config"
)

const (
	logFileName = "update-log.jsonl"
	maxEntries  = 50
)

// Entry is one line of .escapement/update-log.jsonl.
type Entry struct {
	Time    time.Time    `json:"time"`
	Outcome Outcome      `json:"outcome"`
	Cadence string       `json:"cadence,omitempty"`
	Packs   []PackStatus `json:"packs,omitempty"`
	Prompt  string       `json:"prompt"` // none | accepted | declined
}

func logPath(root string) string {
	return filepath.Join(root, config.Dir, logFileName)
}

// LoadLog reads the JSONL log; returns (nil, nil) if the file does not exist.
func LoadLog(root string) ([]Entry, error) {
	data, err := os.ReadFile(logPath(root))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Entry
	dec := json.NewDecoder(bytes.NewReader(data))
	for dec.More() {
		var e Entry
		if err := dec.Decode(&e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// LastEntry returns the newest entry, or nil.
func LastEntry(es []Entry) *Entry {
	if len(es) == 0 {
		return nil
	}
	return &es[len(es)-1]
}

// LastSuccess returns the newest ok_current/ok_updates entry, or nil.
func LastSuccess(es []Entry) *Entry {
	for i := len(es) - 1; i >= 0; i-- {
		if es[i].Outcome == OutcomeOKCurrent || es[i].Outcome == OutcomeOKUpdates {
			return &es[i]
		}
	}
	return nil
}

// appendLog appends e, trims to the last maxEntries, and rewrites the file
// atomically. It also ensures the gitignore entry so the per-clone log is
// never committed regardless of when the repo was initialized.
func appendLog(root string, e Entry) error {
	ensureGitignore(root)
	entries, _ := LoadLog(root)
	entries = append(entries, e)
	if len(entries) > maxEntries {
		entries = entries[len(entries)-maxEntries:]
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, en := range entries {
		if err := enc.Encode(en); err != nil {
			return err
		}
	}
	return atomicWrite(logPath(root), buf.Bytes())
}

// atomicWrite writes via a temp file + rename in the destination directory.
func atomicWrite(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".esc-log-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// ensureGitignore makes .escapement/.gitignore ignore the update log. It is
// best-effort: a failure here must never fail the invoking command.
func ensureGitignore(root string) {
	p := filepath.Join(root, config.Dir, ".gitignore")
	data, err := os.ReadFile(p)
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) == logFileName {
				return
			}
		}
		f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return
		}
		defer f.Close()
		if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
			f.WriteString("\n")
		}
		f.WriteString(logFileName + "\n")
		return
	}
	if os.MkdirAll(filepath.Join(root, config.Dir), 0o755) == nil {
		os.WriteFile(p, []byte(logFileName+"\n"), 0o644)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/updatecheck/ -v`
Expected: PASS (log + core tests).

- [ ] **Step 5: gofmt/vet and commit**

```bash
gofmt -w internal/updatecheck/
go vet ./internal/updatecheck/
git add internal/updatecheck/log.go internal/updatecheck/log_test.go
git commit -m "feat(updatecheck): atomic trimmed JSONL log with gitignore scaffolding"
```

---

### Task 5: Throttle (`Maybe`), sync recorder (`RecordSync`), prompt/notice

**Files:**
- Create: `internal/updatecheck/throttle.go`
- Test: `internal/updatecheck/throttle_test.go`

**Interfaces:**
- Consumes: `check`, `Cadence`, `LoadLog`, `LastEntry`, `LastSuccess`, `appendLog`, `pack.ParseEvery`, `config.Load`, `lockfile.Load`.
- Produces:
  - `type Decision struct { Accepted bool; Packs []PackStatus }`.
  - `func Maybe(ctx context.Context, root string, stdin *os.File, stderr io.Writer) *Decision` — returns nil when inert/not-overdue/no-updates/non-TTY; otherwise a Decision reflecting the prompt answer.
  - `func RecordSync(ctx context.Context, root string, packs []*pack.Pack)` — records the check entry a successful sync implies; never returns an error.
  - `func isInteractive(f *os.File) bool` / `func readPromptDecision(r io.Reader, w io.Writer) bool` (unexported).

- [ ] **Step 1: Write the failing test**

```go
// internal/updatecheck/throttle_test.go
package updatecheck

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestReadPromptDecision(t *testing.T) {
	var w strings.Builder
	if !readPromptDecision(strings.NewReader("\n"), &w) {
		t.Error("empty line should accept")
	}
	if readPromptDecision(strings.NewReader("n\n"), &w) {
		t.Error("'n' should decline")
	}
	if readPromptDecision(strings.NewReader("\x1b\n"), &w) {
		t.Error("ESC sequence should decline")
	}
	if !strings.Contains(w.String(), "Update now?") {
		t.Error("prompt text not written")
	}
}

func TestMaybeInertWithoutLog(t *testing.T) {
	root := t.TempDir()
	// No log at all → inert, returns nil, writes nothing.
	if d := Maybe(context.Background(), root, nil, &strings.Builder{}); d != nil {
		t.Fatalf("no log should be inert, got %+v", d)
	}
	if es, _ := LoadLog(root); es != nil {
		t.Error("Maybe wrote a log when inert")
	}
}

func TestMaybeNotOverdue(t *testing.T) {
	root := t.TempDir()
	// A fresh successful check within cadence → not overdue.
	appendLog(root, Entry{Time: time.Now().UTC(), Outcome: OutcomeOKCurrent, Cadence: "7d", Prompt: "none"})
	before, _ := LoadLog(root)
	if d := Maybe(context.Background(), root, nil, &strings.Builder{}); d != nil {
		t.Fatalf("within cadence should be no-op, got %+v", d)
	}
	after, _ := LoadLog(root)
	if len(after) != len(before) {
		t.Error("Maybe wrote a log entry while within cadence")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/updatecheck/ -run 'TestReadPrompt|TestMaybe' -v`
Expected: FAIL — `undefined: Maybe` / `readPromptDecision`.

- [ ] **Step 3: Implement the throttle and recorder**

```go
// internal/updatecheck/throttle.go
package updatecheck

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/lockfile"
	"github.com/tensorgroup/openescapement/internal/pack"
)

// Decision is the throttle's result when a prompt happened.
type Decision struct {
	Accepted bool
	Packs    []PackStatus
}

// Maybe performs an update check if one is overdue, logs the outcome, prints a
// stderr notice when updates exist, and (interactive TTY only) prompts. It
// never blocks or fails the invoking command: all errors log an `error`
// outcome plus one short stderr line and return nil.
func Maybe(ctx context.Context, root string, stdin *os.File, stderr io.Writer) *Decision {
	entries, err := LoadLog(root)
	if err != nil {
		return nil
	}
	last := LastEntry(entries)
	if last == nil || last.Cadence == "" {
		return nil // inert: feature never activated (or explicitly deactivated)
	}
	cadence, err := pack.ParseEvery(last.Cadence)
	if err != nil {
		return nil
	}
	if ls := LastSuccess(entries); ls != nil && time.Since(ls.Time) < cadence {
		return nil // not overdue
	}

	cctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	cfg, err := config.Load(root)
	if err != nil {
		return nil
	}
	lock, _ := lockfile.Load(root)
	statuses, cerr := check(cctx, cfg, lock)

	entry := Entry{Time: time.Now().UTC(), Cadence: last.Cadence, Prompt: "none"}
	if cerr != nil {
		entry.Outcome = OutcomeError
		fmt.Fprintf(stderr, "esc: update check failed: %v\n", cerr)
		_ = appendLog(root, entry)
		return nil
	}
	entry.Packs = statuses
	if !anyUpdates(statuses) {
		entry.Outcome = OutcomeOKCurrent
		_ = appendLog(root, entry)
		return nil
	}
	entry.Outcome = OutcomeOKUpdates
	printNotice(stderr, statuses)
	if !isInteractive(stdin) {
		_ = appendLog(root, entry) // non-TTY: notice only, never a prompt
		return nil
	}
	accept := readPromptDecision(stdin, stderr)
	if accept {
		entry.Prompt = "accepted"
	} else {
		entry.Prompt = "declined"
	}
	_ = appendLog(root, entry)
	return &Decision{Accepted: accept, Packs: statuses}
}

// RecordSync records the check entry a successful sync implies (sync already
// fetched upstream state). It recomputes cadence from the just-synced
// manifests, so dropping update_check from all packs makes the feature inert.
func RecordSync(ctx context.Context, root string, packs []*pack.Pack) {
	cadence, cadStr := Cadence(packs)
	entries, _ := LoadLog(root)
	if cadence == 0 {
		if len(entries) > 0 {
			// Was active, now inert: write a terminal empty-cadence marker so
			// the throttle stops on the next command.
			_ = appendLog(root, Entry{Time: time.Now().UTC(), Outcome: OutcomeOKCurrent, Prompt: "none"})
		}
		return
	}
	cctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	cfg, err := config.Load(root)
	if err != nil {
		return
	}
	lock, _ := lockfile.Load(root)
	statuses, cerr := check(cctx, cfg, lock)
	entry := Entry{Time: time.Now().UTC(), Cadence: cadStr, Prompt: "none"}
	if cerr != nil {
		entry.Outcome = OutcomeError
		_ = appendLog(root, entry)
		return
	}
	entry.Packs = statuses
	if anyUpdates(statuses) {
		entry.Outcome = OutcomeOKUpdates
	} else {
		entry.Outcome = OutcomeOKCurrent
	}
	_ = appendLog(root, entry)
}

func anyUpdates(statuses []PackStatus) bool {
	for _, s := range statuses {
		if s.Updates {
			return true
		}
	}
	return false
}

func printNotice(w io.Writer, statuses []PackStatus) {
	fmt.Fprintln(w, "esc: policy pack updates available:")
	for _, s := range statuses {
		if s.Updates {
			fmt.Fprintf(w, "  %s: %s -> %s\n", s.Source, s.Pinned, s.Latest)
		}
	}
}

// isInteractive reports whether f is a character device (a real terminal).
// Uses only os.Stat — no golang.org/x/term dependency.
func isInteractive(f *os.File) bool {
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// readPromptDecision prints the prompt and reads one line: empty = accept,
// anything else (including an ESC escape sequence) = decline.
func readPromptDecision(r io.Reader, w io.Writer) bool {
	fmt.Fprint(w, "Update now? [Enter=yes, n=no]: ")
	line, _ := bufio.NewReader(r).ReadString('\n')
	return strings.TrimRight(line, "\r\n") == ""
}
```

Add the `strings` import to `throttle.go` (used by `readPromptDecision`): include `"strings"` in the import block.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/updatecheck/ -v`
Expected: PASS.

- [ ] **Step 5: gofmt/vet and commit**

```bash
gofmt -w internal/updatecheck/
go vet ./internal/updatecheck/
git add internal/updatecheck/throttle.go internal/updatecheck/throttle_test.go
git commit -m "feat(updatecheck): overdue throttle, sync recorder, TTY prompt/notice"
```

---

### Task 6: Status findings — `pack-stale` and `check-overdue`

**Files:**
- Modify: `internal/engine/status.go` (imports; add two `State` consts ~line 18-24; extend `Status` ~line 50-78)
- Test: `internal/engine/status_updatecheck_test.go` (create)

**Interfaces:**
- Consumes: `updatecheck.Cadence`, `updatecheck.LoadLog`, `updatecheck.LastEntry`, `updatecheck.LastSuccess`, `updatecheck.OutcomeError`, `PlanResult.PackObjs`.
- Produces: `PackStale State = "pack-stale"`, `CheckOverdue State = "check-overdue"`. Both are non-`InSync`, so `Clean()` returns false → `esc status --check` exits 1.

- [ ] **Step 1: Write the failing test**

```go
// internal/engine/status_updatecheck_test.go
package engine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/tensorgroup/openescapement/internal/updatecheck"
)

// gov builds a governed repo synced against a pack repo that declares
// update_check, returning the governed root and the pack repo path.
func govWithUpdateCheck(t *testing.T) (root, packRepo string) {
	t.Helper()
	packRepo = t.TempDir()
	files := map[string]string{
		"org/pack.yaml": "schema: 1\nname: acme\nversion: 1.0.0\nrules: [rules/a.md]\n" +
			"update_check:\n  every: 7d\n",
		"org/rules/a.md": "## A\none\n",
	}
	for rel, c := range files {
		p := filepath.Join(packRepo, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(c), 0o644)
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "t@e.com"}, {"config", "user.name", "T"},
		{"config", "commit.gpgsign", "false"}, {"config", "tag.gpgsign", "false"},
		{"add", "-A"}, {"commit", "-q", "-m", "v1"}, {"tag", "-a", "v1.0.0", "-m", "v1"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = packRepo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	root = t.TempDir()
	os.MkdirAll(filepath.Join(root, ".escapement"), 0o755)
	os.WriteFile(filepath.Join(root, ".escapement", "config.yaml"),
		[]byte("schema: 1\npacks:\n  - source: file://"+packRepo+"//org\n    ref: v1.0.0\n    trust: unsigned\n"), 0o644)
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	if err := ApplyPlan(t, root); err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	return root, packRepo
}

// ApplyPlan runs Plan+Apply once (helper local to the test).
func ApplyPlan(t *testing.T, root string) error {
	t.Helper()
	p, err := Plan(context.Background(), root)
	if err != nil {
		return err
	}
	return Apply(root, p)
}

func TestStatusPackStale(t *testing.T) {
	root, _ := govWithUpdateCheck(t)
	// A recent successful check that saw updates available.
	appendTestLog(t, root, updatecheck.Entry{
		Time:    time.Now().UTC(),
		Outcome: updatecheck.OutcomeOKUpdates,
		Cadence: "7d",
		Prompt:  "none",
		Packs: []updatecheck.PackStatus{{
			Source: "file://x//org", Kind: "tag", Pinned: "v1.0.0", Latest: "v2.0.0", Updates: true,
		}},
	})
	st, err := Status(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !hasState(st, PackStale) {
		t.Fatalf("want pack-stale finding, got %+v", st.Findings)
	}
	if st.Clean() {
		t.Error("pack-stale should make status not clean")
	}
}

func TestStatusCheckOverdue(t *testing.T) {
	root, _ := govWithUpdateCheck(t)
	// Last success is ancient (> cadence) and the newest attempt errored.
	appendTestLog(t, root, updatecheck.Entry{
		Time: time.Now().Add(-100 * 24 * time.Hour).UTC(), Outcome: updatecheck.OutcomeOKCurrent,
		Cadence: "7d", Prompt: "none",
	})
	appendTestLog(t, root, updatecheck.Entry{
		Time: time.Now().UTC(), Outcome: updatecheck.OutcomeError, Cadence: "7d", Prompt: "none",
	})
	st, err := Status(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !hasState(st, CheckOverdue) {
		t.Fatalf("want check-overdue finding, got %+v", st.Findings)
	}
}

func hasState(st *StatusResult, s State) bool {
	for _, f := range st.Findings {
		if f.State == s {
			return true
		}
	}
	return false
}
```

Add this helper to the same test file (writes the JSONL directly to keep the test independent of `updatecheck`'s unexported `appendLog`):

```go
func appendTestLog(t *testing.T, root string, e updatecheck.Entry) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(root, ".escapement", "update-log.jsonl"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b, _ := json.Marshal(e)
	f.Write(append(b, '\n'))
}
```

Add `"encoding/json"` to the test file's imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/engine/ -run 'TestStatusPackStale|TestStatusCheckOverdue' -v`
Expected: FAIL — `undefined: PackStale` / `CheckOverdue`.

- [ ] **Step 3: Add the states and extend `Status`**

In `internal/engine/status.go`, add the two states to the const block:

```go
const (
	InSync             State = "in-sync"
	Modified           State = "modified"
	Missing            State = "missing"
	Stale              State = "stale"
	ConstraintViolated State = "constraint-violated"
	PackStale          State = "pack-stale"
	CheckOverdue       State = "check-overdue"
)
```

Extend the import block:

```go
import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/lockfile"
	"github.com/tensorgroup/openescapement/internal/pack"
	"github.com/tensorgroup/openescapement/internal/render"
	"github.com/tensorgroup/openescapement/internal/updatecheck"
)
```

In `Status`, insert the freshness findings just before `return res, nil`:

```go
	// Update-freshness findings (inert unless a pack declares update_check).
	if cadence, _ := updatecheck.Cadence(plan.PackObjs); cadence > 0 {
		entries, _ := updatecheck.LoadLog(root)
		if ls := updatecheck.LastSuccess(entries); ls != nil {
			for _, ps := range ls.Packs {
				if ps.Updates {
					res.Findings = append(res.Findings, Finding{
						Path:  ps.Source,
						State: PackStale,
						Detail: fmt.Sprintf("update available: %s -> %s (run `esc update` then `esc sync`)",
							ps.Pinned, ps.Latest),
					})
				}
			}
		}
		last := updatecheck.LastEntry(entries)
		ls := updatecheck.LastSuccess(entries)
		overdue := ls == nil || time.Since(ls.Time) > cadence
		attemptFailed := last != nil && last.Outcome == updatecheck.OutcomeError
		if overdue && attemptFailed {
			res.Findings = append(res.Findings, Finding{
				Path:   "update-check",
				State:  CheckOverdue,
				Detail: "no successful update check within cadence and the latest attempt failed",
			})
		}
	}
	return res, nil
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/engine/ -v`
Expected: PASS (new findings tests plus existing engine tests).

- [ ] **Step 5: gofmt/vet and commit**

```bash
gofmt -w internal/engine/status.go internal/engine/status_updatecheck_test.go
go vet ./internal/engine/
git add internal/engine/status.go internal/engine/status_updatecheck_test.go
git commit -m "feat(engine): pack-stale and check-overdue status findings"
```

---

### Task 7: CLI wiring — throttle, sync recorder, accept→re-pin+sync, init gitignore

**Files:**
- Modify: `internal/cli/cli.go` (imports; `Run` dispatch ~line 54-77; `cmdInit` ~line 112-131; `cmdSync` ~line 133-146; `cmdStatus` ~line 148; `cmdDiff` ~line 177; `cmdUpdate` sig+body ~line 209; `cmdRender` sig+body ~line 253; add `checkForUpdates`, `bumpPins`)
- Test: `internal/cli/updatecheck_test.go` (create)

**Interfaces:**
- Consumes: `updatecheck.Maybe`, `updatecheck.RecordSync`, `updatecheck.Decision`, `updatecheck.PackStatus`, `engine.Plan`, `engine.Apply`, `config.Load`/`Save`.
- Produces: `func checkForUpdates(ctx context.Context, root string, stderr io.Writer)`; `func bumpPins(root string, statuses []updatecheck.PackStatus) error`.

- [ ] **Step 1: Write the failing test**

```go
// internal/cli/updatecheck_test.go
package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// packRepoWithCheck builds a pack repo whose manifest declares update_check.
func packRepoWithCheck(t *testing.T, version, every string) string {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"org/pack.yaml": "schema: 1\nname: acme-org\nversion: " + version +
			"\ndescription: d\nrules: [rules/a.md]\nupdate_check:\n  every: " + every + "\n",
		"org/rules/a.md": "## A\nv" + version + "\n",
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

// backdateLog rewrites every log entry's timestamp to the distant past so the
// next command is guaranteed overdue.
func backdateLog(t *testing.T, root string) {
	t.Helper()
	p := filepath.Join(root, ".escapement", "update-log.jsonl")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	out := strings.ReplaceAll(string(data), `"time":"20`, `"time":"19`)
	os.WriteFile(p, []byte(out), 0o644)
}

func TestSyncRecordsCheckAndScaffoldsGitignore(t *testing.T) {
	repo := packRepoWithCheck(t, "1.0.0", "7d")
	root := newGoverned(t, repo, "v1.0.0")
	// newGoverned points at the org subdir already.
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("sync: %d\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(root, ".escapement", "update-log.jsonl")); err != nil {
		t.Fatalf("sync did not record a check entry: %v", err)
	}
	gi, err := os.ReadFile(filepath.Join(root, ".escapement", ".gitignore"))
	if err != nil || !strings.Contains(string(gi), "update-log.jsonl") {
		t.Errorf("gitignore not scaffolded: %q %v", gi, err)
	}
}

func TestNoUpdateCheckStaysInert(t *testing.T) {
	repo := newPackRepo(t, "1.0.0") // no update_check
	root := newGoverned(t, repo, "v1.0.0")
	run(t, root, "sync")
	if _, err := os.Stat(filepath.Join(root, ".escapement", "update-log.jsonl")); !os.IsNotExist(err) {
		t.Error("inert feature must not write a log")
	}
	if code, out := run(t, root, "status", "--check"); code != 0 {
		t.Errorf("inert repo status --check: exit %d\n%s", code, out)
	}
}

func TestStatusSurfacesPackStale(t *testing.T) {
	repo := packRepoWithCheck(t, "1.0.0", "7d")
	root := newGoverned(t, repo, "v1.0.0")
	run(t, root, "sync")
	// Publish v2.0.0.
	writeFiles(t, repo, map[string]string{
		"org/pack.yaml": "schema: 1\nname: acme-org\nversion: 2.0.0\ndescription: d\nrules: [rules/a.md]\nupdate_check:\n  every: 7d\n",
		"org/rules/a.md": "## A\nv2.0.0\n",
	})
	gitIn(t, repo, "add", "-A")
	gitIn(t, repo, "commit", "-q", "-m", "v2")
	gitIn(t, repo, "tag", "-a", "v2.0.0", "-m", "v2")
	// Force the next command to be overdue; it runs a real ls-remote check.
	backdateLog(t, root)
	code, out := run(t, root, "status", "--check")
	if code != 1 {
		t.Fatalf("stale pack: want exit 1, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "pack-stale") || !strings.Contains(out, "v2.0.0") {
		t.Errorf("status should report pack-stale to v2.0.0:\n%s", out)
	}
}

func TestInitScaffoldsGitignore(t *testing.T) {
	root := t.TempDir()
	if code, out := run(t, root, "init"); code != 0 {
		t.Fatalf("init: %d\n%s", code, out)
	}
	gi, err := os.ReadFile(filepath.Join(root, ".escapement", ".gitignore"))
	if err != nil || !strings.Contains(string(gi), "update-log.jsonl") {
		t.Errorf("init gitignore: %q %v", gi, err)
	}
}

var _ = exec.Command // keep os/exec import if unused elsewhere
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run 'TestSyncRecords|TestNoUpdateCheck|TestStatusSurfaces|TestInitScaffolds' -v`
Expected: FAIL — no gitignore, no log entry, no `pack-stale` output.

- [ ] **Step 3: Extend cli imports and `Run` dispatch**

In `internal/cli/cli.go`, add `updatecheck` to imports:

```go
	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/engine"
	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/lockfile"
	"github.com/tensorgroup/openescapement/internal/updatecheck"
```

Change the `update` and `render` dispatch lines in `Run` to pass `stderr`:

```go
	case "update":
		err = cmdUpdate(ctx, root, args[1:], stdout, stderr)
	case "render":
		err = cmdRender(ctx, root, args[1:], stdout, stderr)
```

- [ ] **Step 4: Add `checkForUpdates` and `bumpPins`; scaffold gitignore in `cmdInit`; record after `cmdSync`**

Add these helpers to `internal/cli/cli.go`:

```go
// checkForUpdates runs the overdue throttle before a command's main logic. On
// an interactive accept it re-pins each stale tag pack to its latest tag and
// syncs (a bare sync would keep the old pin for tag-pinned packs). It never
// fails the invoking command.
func checkForUpdates(ctx context.Context, root string, stderr io.Writer) {
	d := updatecheck.Maybe(ctx, root, os.Stdin, stderr)
	if d == nil || !d.Accepted {
		return
	}
	if err := bumpPins(root, d.Packs); err != nil {
		fmt.Fprintf(stderr, "esc: applying updates failed: %v\n", err)
		return
	}
	if err := cmdSync(ctx, root, stderr); err != nil {
		fmt.Fprintf(stderr, "esc: update sync failed: %v\n", err)
	}
}

// bumpPins re-pins each stale tag pack in config.yaml to its latest tag.
// Branch pins are left unchanged — a plain sync picks up the new tip.
func bumpPins(root string, statuses []updatecheck.PackStatus) error {
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	changed := false
	for _, s := range statuses {
		if !s.Updates || s.Kind != "tag" {
			continue
		}
		for i := range cfg.Packs {
			if cfg.Packs[i].Source == s.Source {
				cfg.Packs[i].Ref = s.Latest
				changed = true
			}
		}
	}
	if !changed {
		return nil
	}
	return cfg.Save(root)
}
```

In `cmdInit`, after the signers block and before the final `fmt.Fprintf`, scaffold the gitignore:

```go
	gitignore := filepath.Join(root, config.Dir, ".gitignore")
	if _, err := os.Stat(gitignore); os.IsNotExist(err) {
		if err := os.WriteFile(gitignore, []byte("update-log.jsonl\n"), 0o644); err != nil {
			return err
		}
	}
	fmt.Fprintf(stdout, "Initialized %s\nAdd pack sources to the config, then run `esc sync`.\n", cfgPath)
	return nil
```

In `cmdSync`, record the check after a successful apply (before the summary print is fine; put it right after `Apply` succeeds):

```go
	if err := engine.Apply(root, plan); err != nil {
		return err
	}
	updatecheck.RecordSync(ctx, root, plan.PackObjs)
	fmt.Fprintf(stdout, "Synced %d pack(s), %d artifact(s):\n", len(plan.Packs), len(plan.Artifacts))
```

- [ ] **Step 5: Call `checkForUpdates` from status/diff/update/render**

`cmdStatus` — add as the first line after `fs.Parse(args)` succeeds:

```go
	if err := fs.Parse(args); err != nil {
		return 2
	}
	checkForUpdates(ctx, root, stderr)
	st, err := engine.Status(ctx, root)
```

`cmdDiff` — after its `fs.Parse(args)` success:

```go
	if err := fs.Parse(args); err != nil {
		return 2
	}
	checkForUpdates(ctx, root, stderr)
	cur, err := engine.Plan(ctx, root)
```

`cmdUpdate` — change the signature and add the call:

```go
func cmdUpdate(ctx context.Context, root string, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(stdout)
	src := fs.String("source", "", "which configured pack source to update")
	ref := fs.String("ref", "", "new ref to pin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	checkForUpdates(ctx, root, stderr)
	if *ref == "" {
		return errors.New("update: --ref is required")
	}
```

`cmdRender` — change the signature and add the call:

```go
func cmdRender(ctx context.Context, root string, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	fs.SetOutput(stdout)
	toStdout := fs.Bool("stdout", false, "print rendered targets to stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	checkForUpdates(ctx, root, stderr)
	if !*toStdout {
		return errors.New("render: only --stdout is supported (sync writes files)")
	}
```

- [ ] **Step 6: Run the full CLI suite**

Run: `go test ./internal/cli/ -v`
Expected: PASS — new tests plus all existing tests (existing packs declare no `update_check`, so they stay inert and their exit codes are unchanged).

- [ ] **Step 7: Full build + vet + test, then commit**

```bash
gofmt -w internal/cli/cli.go internal/cli/updatecheck_test.go
go vet ./...
go test ./...
git add internal/cli/cli.go internal/cli/updatecheck_test.go
git commit -m "feat(cli): wire update-check throttle, sync recorder, init gitignore"
```

Expected `go test ./...`: `ok` for every package.

---

### Task 8: Docs — CHANGELOG entry + spec wording correction

**Files:**
- Modify: `CHANGELOG.md` (under `## [Unreleased]` → `### Added`)
- Modify: `ai-governance-product-spec.md` (§6 "Update propagation & freshness (DECIDED)", the "On updates found" bullet, ~line 127)

**Interfaces:** none (documentation only).

- [ ] **Step 1: Add the CHANGELOG entry**

In `CHANGELOG.md`, under `## [Unreleased]` → `### Added`, add:

```markdown
- Update-check (client): packs may declare `update_check: { every: <7d|24h|90m>, endpoint: <https URL> }`.
  Overdue `esc sync`/`status`/`diff`/`update`/`render` invocations run a lightweight
  `git ls-remote` staleness probe (10s timeout, never blocks the command), record it to
  the gitignored per-clone `.escapement/update-log.jsonl` (last 50 entries), and — on an
  interactive TTY — offer to re-pin and sync. `esc status --check` reports `pack-stale`
  and `check-overdue` findings (exit 1). Inert unless a pack opts in. The `endpoint` field
  is parsed for forward-compatibility but not contacted in v0.1.
```

- [ ] **Step 2: Correct the spec's "Enter runs `esc sync`" wording**

In `ai-governance-product-spec.md` §6, replace the "On updates found" sentence
"Interactive TTY: prompt — Enter runs `esc sync` now, N/Esc skips (skip is logged)." with:

```markdown
- **On updates found.** Stderr notice always. Interactive TTY: prompt — Enter re-pins each stale
  pack to its latest version and runs `esc sync` now (a bare `esc sync` keeps the old pin for
  tag-pinned packs), N/Esc skips (skip is logged). Non-TTY (CI, agent-invoked): notice only, never a
  hanging prompt.
```

- [ ] **Step 3: Verify no code changed and commit**

Run: `go test ./...`
Expected: `ok` for every package (docs-only change; nothing should break).

```bash
git add CHANGELOG.md ai-governance-product-spec.md
git commit -m "docs: changelog + spec correction for update-check accept semantics"
```

---

## Self-Review

**1. Spec coverage (§6 "Update propagation & freshness (DECIDED)", client half):**
- Cadence in the pack manifest (`update_check.every`, strictest wins, inert if none) → Task 1 (parse/validate), Task 3 (`Cadence`), Tasks 5/6 (inert behavior).
- Endpoint parsed/validated https, not contacted → Task 1 (validation), Task 3 (`check` comment).
- Trigger: piggyback on `sync`/`status`/`diff`/`update`/`render`; `init`/`version`/`help` never check; sync counts as a check; never blocks/fails; 10s timeout → Task 5 (`Maybe`, `RecordSync`, `checkTimeout`), Task 7 (wiring; `init`/`version`/`help` untouched).
- Check via `git ls-remote`, no content → Task 2, Task 3.
- Local log `.escapement/update-log.jsonl`, append-only, trim 50, atomic, gitignored, init scaffolds → Task 4 (store + `ensureGitignore`), Task 7 (`cmdInit`). Entry fields (timestamp/outcome/per-pack/prompt) → Task 4 `Entry` + Task 3 `PackStatus`.
- On updates found: stderr notice always; TTY prompt (empty=accept, else decline, decline logged); non-TTY notice only; TTY via char-device stat, no new deps → Task 5.
- Governance teeth: `pack-stale` + `check-overdue` findings → `status --check` exit 1 → Task 6.
- Server side (v0.2) → explicitly out of scope (noted in header).
- Duration `d` support + custom parser → Task 1 `ParseEvery`.
- Accept re-pins tag packs then syncs (spec correction) → Task 7 `bumpPins`/`checkForUpdates`, Task 8 spec edit.

No client-half requirement is left without a task.

**2. Placeholder scan:** No "TBD"/"similar to Task N"/"add error handling"; every code step carries complete code. Test steps show full test bodies. Commands include expected outcomes.

**3. Type consistency (verified across tasks):**
- `ParseEvery` (pack) — defined Task 1, used Task 3 (`Cadence`) and Task 5 (`Maybe`). Consistent name/signature.
- `LsRemoteTags`/`LsRemoteHash` (source) — defined Task 2, used Task 3.
- `PackStatus{Source,Kind,Pinned,Latest,Updates}` — defined Task 3, consumed unchanged in Tasks 4 (log `Packs`), 5 (`printNotice`/`Decision`), 6 (findings), 7 (`bumpPins`).
- `Outcome` consts (`OutcomeOKCurrent`/`OutcomeOKUpdates`/`OutcomeError`) — defined Task 3, used Tasks 4/5/6.
- `Entry`, `LoadLog`, `LastEntry`, `LastSuccess`, `appendLog`, `maxEntries` — defined Task 4, used Tasks 5/6.
- `Cadence`, `check`, `checkTimeout` — defined Task 3, used Tasks 5/6.
- `Maybe`/`Decision`/`RecordSync` — defined Task 5, used Task 7.
- `PackStale`/`CheckOverdue` states — defined Task 6, printed by existing `cmdStatus` loop (uses `f.State`/`f.Detail`) unchanged.
- `cmdUpdate`/`cmdRender` signature change (added `stderr`) — updated at both the `Run` call sites (Task 7 Step 3) and the definitions (Task 7 Step 5). Consistent.

**Import-cycle check:** `updatecheck` imports `config`/`lockfile`/`source`/`pack` only; `engine` and `cli` import `updatecheck`; nothing imports `engine`/`cli` from `updatecheck`. Acyclic.

**Existing-test safety:** all pre-existing packs declare no `update_check`, so `Cadence` returns 0 everywhere → feature inert → no log writes, no new findings, no changed exit codes. Existing `cli`/`engine`/`render`/`source`/`pack` tests remain green.
