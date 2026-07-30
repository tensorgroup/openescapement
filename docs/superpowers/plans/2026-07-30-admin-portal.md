# Admin Portal (`esc serve`) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `esc serve [--demo] [--addr] [--data-dir]` — a one-binary control-plane portal (four pages + ingest API) with a deterministic seeded demo org, per `docs/superpowers/specs/2026-07-30-admin-portal-design.md`.

**Architecture:** New `internal/portal/{store,seed,publish,charts,web}` packages behind a new `serve` subcommand in `internal/cli`. Storage is JSON registry files + append-only JSONL events under `--data-dir`; rollups computed in memory per request. Pack publish operates on server-side working git clones and reuses `pack.Load` for validation; distribution stays pull-only via `esc sync`.

**Tech Stack:** Go 1.24 stdlib only (`net/http`, `html/template`, `embed`, `crypto/rand`, `math/rand` seeded), `gopkg.in/yaml.v3` (already present), system `git` via `os/exec`.

## Global Constraints

- **Single external dependency policy:** `gopkg.in/yaml.v3` only. NO new `go.mod` entries, NO vendored JS/CSS from third parties. All frontend assets are hand-written and embedded.
- Go `1.24.4` (from go.mod). Module `github.com/tensorgroup/openescapement`.
- `gofmt -w .` and `go vet ./...` must pass before every commit; `go test ./...` before claiming a task done.
- Sentinel errors from `internal/esc` map to exit codes via `cli.exitCode` (`ErrSignature|ErrLockMismatch`→3, `ErrConstraint`→1, default→4, usage→2). `serve` startup failures print to stderr and return 4; flag errors return 2.
- All file writes atomic (temp + rename) except the append-only events log (O_APPEND single write per event).
- Deterministic output everywhere: no map iteration into output, sorted slices, fixed float formatting. Demo seed is byte-identical for a given (seed, epoch).
- Renderer invariant untouched: the portal NEVER writes into governed repos. Publish only commits/tags the server-side pack repo.
- Commit messages: plain conventional style (`feat: …`, `test: …`), no AI co-authorship trailers.
- Tests use real temp git repos (no mocks), `t.Setenv("ESC_CACHE_DIR", t.TempDir())` where sync is involved, and the existing golden-file pattern (`-update` flag) for SVG/HTML goldens.

---

### Task 1: Store — registry + events

**Files:**
- Create: `internal/portal/store/store.go`
- Test: `internal/portal/store/store_test.go`

**Interfaces:**
- Consumes: nothing (stdlib only).
- Produces (later tasks rely on these exact names):

```go
package store

type Registry struct {
	Org         Org          `json:"org"`
	Departments []Department `json:"departments"`
	Teams       []Team       `json:"teams"`
	Repos       []Repo       `json:"repos"`
}
type Org struct{ Name string `json:"name"` }
type Department struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Team struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	DeptID string `json:"dept_id"`
}
type Repo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	TeamID   string `json:"team_id"`
	Governed bool   `json:"governed"`
}
type EventPack struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Signed  bool   `json:"signed"`
}
type Tokens struct {
	Input   int64   `json:"input"`
	Output  int64   `json:"output"`
	CostUSD float64 `json:"cost_usd"`
}
type Event struct {
	TS        time.Time   `json:"ts"`
	Kind      string      `json:"kind"` // sync|status|update_check|provider_usage|mcp_connect
	RepoID    string      `json:"repo_id,omitempty"`
	TeamID    string      `json:"team_id,omitempty"`
	AgentTool string      `json:"agent_tool,omitempty"`
	Model     string      `json:"model,omitempty"`
	Packs     []EventPack `json:"packs,omitempty"`
	Drift     string      `json:"drift,omitempty"` // in-sync|drifted|stale
	Tokens    *Tokens     `json:"tokens,omitempty"`
}

func ValidKind(k string) bool
func Open(dir string) (*Store, error)          // loads registry.json (missing → empty), opens events
func (s *Store) Registry() Registry
func (s *Store) AppendEvent(e Event) error     // one JSON line to events.jsonl, O_APPEND
func (s *Store) Events() ([]Event, error)      // reads all lines; skips blank lines
func SaveRegistry(dir string, r Registry) error // atomic temp+rename, MarshalIndent, trailing \n
```

- [ ] **Step 1: Write the failing test**

```go
package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRegistryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	r := Registry{
		Org:         Org{Name: "Caltech (demo)"},
		Departments: []Department{{ID: "phys", Name: "Physics"}},
		Teams:       []Team{{ID: "ligo", Name: "LIGO Ops", DeptID: "phys"}},
		Repos:       []Repo{{ID: "r1", Name: "ligo-pipeline", TeamID: "ligo", Governed: true}},
	}
	if err := SaveRegistry(dir, r); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := s.Registry()
	if got.Org.Name != "Caltech (demo)" || len(got.Repos) != 1 || got.Repos[0].Name != "ligo-pipeline" {
		t.Fatalf("registry mismatch: %+v", got)
	}
}

func TestOpenMissingRegistry(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Registry().Repos) != 0 {
		t.Fatal("expected empty registry")
	}
}

func TestEventsAppendRead(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	e1 := Event{TS: ts, Kind: "sync", RepoID: "r1", Drift: "in-sync",
		Packs: []EventPack{{Name: "org-baseline", Version: "1.2.0", Signed: true}}}
	e2 := Event{TS: ts.Add(time.Hour), Kind: "provider_usage", TeamID: "ligo",
		Model: "claude-sonnet-5", Tokens: &Tokens{Input: 1000, Output: 200, CostUSD: 0.42}}
	for _, e := range []Event{e1, e2} {
		if err := s.AppendEvent(e); err != nil {
			t.Fatal(err)
		}
	}
	// Reopen: events must survive process restart.
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s2.Events()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Kind != "sync" || got[1].Tokens.CostUSD != 0.42 {
		t.Fatalf("events mismatch: %+v", got)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if n := len(splitLines(string(data))); n != 2 {
		t.Fatalf("want 2 JSONL lines, got %d", n)
	}
}

func TestValidKind(t *testing.T) {
	for _, k := range []string{"sync", "status", "update_check", "provider_usage", "mcp_connect"} {
		if !ValidKind(k) {
			t.Fatalf("%s should be valid", k)
		}
	}
	if ValidKind("exec") || ValidKind("") {
		t.Fatal("invalid kinds accepted")
	}
}
```

(`splitLines` is a tiny test helper in the same file: split on `\n`, drop empty strings.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/portal/store -v`
Expected: FAIL (package does not exist / undefined identifiers).

- [ ] **Step 3: Write the implementation**

`store.go`:

```go
// Package store persists the portal's registry (JSON files) and telemetry
// events (append-only JSONL) under a data directory.
package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	registryFile = "registry.json"
	eventsFile   = "events.jsonl"
)

var kinds = map[string]bool{
	"sync": true, "status": true, "update_check": true,
	"provider_usage": true, "mcp_connect": true,
}

func ValidKind(k string) bool { return kinds[k] }

type Store struct {
	dir string
	reg Registry
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{dir: dir}
	data, err := os.ReadFile(filepath.Join(dir, registryFile))
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &s.reg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", registryFile, err)
	}
	return s, nil
}

func (s *Store) Registry() Registry { return s.reg }

func (s *Store) AppendEvent(e Event) error {
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(s.dir, eventsFile),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return f.Close()
}

func (s *Store) Events() ([]Event, error) {
	f, err := os.Open(filepath.Join(s.dir, eventsFile))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, fmt.Errorf("bad event line: %w", err)
		}
		out = append(out, e)
	}
	return out, sc.Err()
}

func SaveRegistry(dir string, r Registry) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := filepath.Join(dir, registryFile+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, registryFile))
}
```

Plus the type declarations from the Interfaces block above (same file, with `time` import).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/portal/store -v`
Expected: PASS (4 tests).

- [ ] **Step 5: gofmt, vet, commit**

```bash
gofmt -w internal/portal && go vet ./internal/portal/... && \
git add internal/portal/store && \
git commit -m "feat(portal): store package — JSON registry + JSONL event log"
```

---

### Task 2: Rollups — overview, fleet, usage aggregation

**Files:**
- Create: `internal/portal/store/rollup.go`
- Test: `internal/portal/store/rollup_test.go`

**Interfaces:**
- Consumes: Task 1 types (`Registry`, `Event`, `Tokens`).
- Produces:

```go
type OverviewStats struct {
	GovernedRepos, KnownRepos    int
	DriftedRepos, StaleRepos     int
	TokensWeek                   int64
	CostWeek                     float64
	Models                       []string        // sorted, seen in provider_usage/mcp_connect events
	Adoption                     []AdoptionPoint // one per day, last 60 days, cumulative governed repos
}
type AdoptionPoint struct {
	Day      time.Time
	Governed int
}
type FleetRow struct {
	DeptName, TeamName, RepoName, RepoID string
	Packs                                []string // "name@version", sorted
	LastSync                             time.Time
	Status                               string // in-sync|drifted|stale|ungoverned
	Tools                                []string // sorted agent tools seen for repo
}
type UsageCell struct {
	Day    time.Time // truncated to UTC day
	Model  string
	TeamID string
	Tokens int64 // input+output
	Cost   float64
}

func Overview(r Registry, events []Event, now time.Time) OverviewStats
func FleetRows(r Registry, events []Event) []FleetRow // sorted by Dept, Team, Repo name
func UsageDaily(events []Event, teamID, model string, from, to time.Time) []UsageCell
// UsageDaily: filter empty string = all; result sorted by (Day, Model, TeamID).
```

Semantics (implement exactly):
- A repo's posture = its **latest** `sync` or `status` event (by TS): `Status` from `Drift`, `Packs` from `Packs`, `LastSync` from TS. No such event → `ungoverned` (even if `Repo.Governed` is true — events are truth for posture; `Repo.Governed` marks *intended* governance and drives `GovernedRepos`... no: `GovernedRepos` = repos whose latest posture event exists. `KnownRepos` = `len(r.Repos)`.)
- `DriftedRepos` counts status `drifted`; `StaleRepos` counts `stale`.
- `TokensWeek`/`CostWeek`: sum of `provider_usage` events with `TS > now-7*24h`.
- `Adoption`: for each of the 60 days ending at `now` (UTC-truncated), count repos whose **first** posture event is ≤ end of that day.
- `Tools`: distinct `AgentTool` values from any event for that repo.

- [ ] **Step 1: Write the failing test**

```go
package store

import (
	"testing"
	"time"
)

func day(d int) time.Time { return time.Date(2026, 7, d, 12, 0, 0, 0, time.UTC) }

func fixtureRegistry() Registry {
	return Registry{
		Org:         Org{Name: "Demo"},
		Departments: []Department{{ID: "phys", Name: "Physics"}, {ID: "it", Name: "Central IT"}},
		Teams: []Team{
			{ID: "ligo", Name: "LIGO Ops", DeptID: "phys"},
			{ID: "web", Name: "Web Platform", DeptID: "it"},
		},
		Repos: []Repo{
			{ID: "r1", Name: "ligo-pipeline", TeamID: "ligo", Governed: true},
			{ID: "r2", Name: "campus-portal", TeamID: "web", Governed: true},
			{ID: "r3", Name: "shadow-poc", TeamID: "web", Governed: false},
		},
	}
}

func fixtureEvents() []Event {
	return []Event{
		{TS: day(1), Kind: "sync", RepoID: "r1", Drift: "in-sync", AgentTool: "claude-code",
			Packs: []EventPack{{Name: "org-baseline", Version: "1.1.0", Signed: true}}},
		{TS: day(10), Kind: "status", RepoID: "r1", Drift: "drifted", AgentTool: "claude-code",
			Packs: []EventPack{{Name: "org-baseline", Version: "1.2.0", Signed: true}}},
		{TS: day(5), Kind: "sync", RepoID: "r2", Drift: "stale",
			Packs: []EventPack{{Name: "org-baseline", Version: "1.1.0", Signed: true}}},
		{TS: day(12), Kind: "provider_usage", TeamID: "ligo", Model: "claude-sonnet-5",
			Tokens: &Tokens{Input: 900, Output: 100, CostUSD: 0.5}},
		{TS: day(13), Kind: "provider_usage", TeamID: "web", Model: "claude-haiku-4-5",
			Tokens: &Tokens{Input: 400, Output: 100, CostUSD: 0.1}},
	}
}

func TestOverview(t *testing.T) {
	got := Overview(fixtureRegistry(), fixtureEvents(), day(14))
	if got.KnownRepos != 3 || got.GovernedRepos != 2 {
		t.Fatalf("known=%d governed=%d", got.KnownRepos, got.GovernedRepos)
	}
	if got.DriftedRepos != 1 || got.StaleRepos != 1 {
		t.Fatalf("drifted=%d stale=%d", got.DriftedRepos, got.StaleRepos)
	}
	if got.TokensWeek != 1500 || got.CostWeek != 0.6 {
		t.Fatalf("tokens=%d cost=%v", got.TokensWeek, got.CostWeek)
	}
	if len(got.Models) != 2 || got.Models[0] != "claude-haiku-4-5" {
		t.Fatalf("models=%v", got.Models)
	}
	if len(got.Adoption) != 60 {
		t.Fatalf("adoption len=%d", len(got.Adoption))
	}
	last := got.Adoption[59]
	if last.Governed != 2 {
		t.Fatalf("final adoption=%d", last.Governed)
	}
}

func TestFleetRows(t *testing.T) {
	rows := FleetRows(fixtureRegistry(), fixtureEvents())
	if len(rows) != 3 {
		t.Fatalf("rows=%d", len(rows))
	}
	// Sorted: Central IT before Physics.
	if rows[0].RepoName != "campus-portal" || rows[0].Status != "stale" {
		t.Fatalf("row0=%+v", rows[0])
	}
	if rows[1].RepoName != "shadow-poc" || rows[1].Status != "ungoverned" {
		t.Fatalf("row1=%+v", rows[1])
	}
	if rows[2].RepoName != "ligo-pipeline" || rows[2].Status != "drifted" ||
		rows[2].Packs[0] != "org-baseline@1.2.0" || rows[2].Tools[0] != "claude-code" {
		t.Fatalf("row2=%+v", rows[2])
	}
}

func TestUsageDaily(t *testing.T) {
	all := UsageDaily(fixtureEvents(), "", "", day(1), day(14))
	if len(all) != 2 || all[0].Model != "claude-sonnet-5" || all[0].Tokens != 1000 {
		t.Fatalf("all=%+v", all)
	}
	ligo := UsageDaily(fixtureEvents(), "ligo", "", day(1), day(14))
	if len(ligo) != 1 || ligo[0].Cost != 0.5 {
		t.Fatalf("ligo=%+v", ligo)
	}
	none := UsageDaily(fixtureEvents(), "", "gpt-x", day(1), day(14))
	if len(none) != 0 {
		t.Fatalf("none=%+v", none)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/portal/store -run 'TestOverview|TestFleetRows|TestUsageDaily' -v`
Expected: FAIL (undefined `Overview` etc.).

- [ ] **Step 3: Implement rollup.go**

```go
package store

import (
	"fmt"
	"sort"
	"time"
)

type postureKey struct{ ts time.Time }

func latestPosture(events []Event) map[string]Event {
	latest := map[string]Event{}
	for _, e := range events {
		if e.Kind != "sync" && e.Kind != "status" {
			continue
		}
		if e.RepoID == "" {
			continue
		}
		if cur, ok := latest[e.RepoID]; !ok || e.TS.After(cur.TS) {
			latest[e.RepoID] = e
		}
	}
	return latest
}

func firstPosture(events []Event) map[string]time.Time {
	first := map[string]time.Time{}
	for _, e := range events {
		if e.Kind != "sync" && e.Kind != "status" {
			continue
		}
		if cur, ok := first[e.RepoID]; !ok || e.TS.Before(cur) {
			first[e.RepoID] = e.TS
		}
	}
	return first
}

func Overview(r Registry, events []Event, now time.Time) OverviewStats {
	s := OverviewStats{KnownRepos: len(r.Repos)}
	latest := latestPosture(events)
	s.GovernedRepos = len(latest)
	for _, e := range latest {
		switch e.Drift {
		case "drifted":
			s.DriftedRepos++
		case "stale":
			s.StaleRepos++
		}
	}
	models := map[string]bool{}
	weekAgo := now.Add(-7 * 24 * time.Hour)
	for _, e := range events {
		if e.Model != "" {
			models[e.Model] = true
		}
		if e.Kind == "provider_usage" && e.Tokens != nil && e.TS.After(weekAgo) {
			s.TokensWeek += e.Tokens.Input + e.Tokens.Output
			s.CostWeek += e.Tokens.CostUSD
		}
	}
	for m := range models {
		s.Models = append(s.Models, m)
	}
	sort.Strings(s.Models)
	first := firstPosture(events)
	end := now.UTC().Truncate(24 * time.Hour)
	for i := 59; i >= 0; i-- {
		d := end.AddDate(0, 0, -i)
		eod := d.Add(24 * time.Hour)
		n := 0
		for _, ts := range first {
			if ts.Before(eod) {
				n++
			}
		}
		s.Adoption = append(s.Adoption, AdoptionPoint{Day: d, Governed: n})
	}
	return s
}

func FleetRows(r Registry, events []Event) []FleetRow {
	deptName := map[string]string{}
	for _, d := range r.Departments {
		deptName[d.ID] = d.Name
	}
	teamByID := map[string]Team{}
	for _, t := range r.Teams {
		teamByID[t.ID] = t
	}
	latest := latestPosture(events)
	tools := map[string]map[string]bool{}
	for _, e := range events {
		if e.RepoID == "" || e.AgentTool == "" {
			continue
		}
		if tools[e.RepoID] == nil {
			tools[e.RepoID] = map[string]bool{}
		}
		tools[e.RepoID][e.AgentTool] = true
	}
	var rows []FleetRow
	for _, repo := range r.Repos {
		team := teamByID[repo.TeamID]
		row := FleetRow{
			DeptName: deptName[team.DeptID], TeamName: team.Name,
			RepoName: repo.Name, RepoID: repo.ID, Status: "ungoverned",
		}
		if e, ok := latest[repo.ID]; ok {
			row.Status = e.Drift
			row.LastSync = e.TS
			for _, p := range e.Packs {
				row.Packs = append(row.Packs, fmt.Sprintf("%s@%s", p.Name, p.Version))
			}
			sort.Strings(row.Packs)
		}
		for tool := range tools[repo.ID] {
			row.Tools = append(row.Tools, tool)
		}
		sort.Strings(row.Tools)
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.DeptName != b.DeptName {
			return a.DeptName < b.DeptName
		}
		if a.TeamName != b.TeamName {
			return a.TeamName < b.TeamName
		}
		return a.RepoName < b.RepoName
	})
	return rows
}

func UsageDaily(events []Event, teamID, model string, from, to time.Time) []UsageCell {
	type key struct {
		day   time.Time
		model string
		team  string
	}
	agg := map[key]*UsageCell{}
	for _, e := range events {
		if e.Kind != "provider_usage" || e.Tokens == nil {
			continue
		}
		if e.TS.Before(from) || e.TS.After(to) {
			continue
		}
		if teamID != "" && e.TeamID != teamID {
			continue
		}
		if model != "" && e.Model != model {
			continue
		}
		k := key{e.TS.UTC().Truncate(24 * time.Hour), e.Model, e.TeamID}
		c, ok := agg[k]
		if !ok {
			c = &UsageCell{Day: k.day, Model: k.model, TeamID: k.team}
			agg[k] = c
		}
		c.Tokens += e.Tokens.Input + e.Tokens.Output
		c.Cost += e.Tokens.CostUSD
	}
	var out []UsageCell
	for _, c := range agg {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if !a.Day.Equal(b.Day) {
			return a.Day.Before(b.Day)
		}
		if a.Model != b.Model {
			return a.Model < b.Model
		}
		return a.TeamID < b.TeamID
	})
	return out
}
```

(Also add the result-type declarations from the Interfaces block; delete the unused `postureKey` if the compiler flags it.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/portal/store -v`
Expected: PASS (all Task 1 + Task 2 tests).

- [ ] **Step 5: gofmt, vet, commit**

```bash
gofmt -w internal/portal && go vet ./internal/portal/... && \
git add internal/portal/store && \
git commit -m "feat(portal): rollups — overview stats, fleet rows, daily usage"
```

---

### Task 3: Charts — server-rendered SVG

**Files:**
- Create: `internal/portal/charts/charts.go`
- Test: `internal/portal/charts/charts_test.go`, goldens in `internal/portal/charts/testdata/`

**Interfaces:**
- Consumes: nothing.
- Produces:

```go
package charts

type Point struct {
	X time.Time
	Y float64
}
type Series struct {
	Label  string
	Values []float64 // one per label slot; len(Values) == len(labels)
}

func Line(pts []Point, w, h int) template.HTML
func StackedBars(labels []string, series []Series, w, h int) template.HTML
// Both return a complete inline <svg>. Empty input returns a styled
// "No data yet" placeholder <svg> of the same dimensions.
```

Rules (implement exactly):
- Fixed palette (order matters, cycle if more series): `#2563a8`, `#4d9078`, `#b0713f`, `#7a5aa0`, `#a84b57`, `#5b7a99`.
- All coordinates formatted with `strconv.FormatFloat(v, 'f', 1, 64)` — never `%v`/`%g` (determinism).
- Margins: 40 left, 20 right, 10 top, 24 bottom. Y axis: 3 gridlines with labels at 0 / mid / max (max rounded up to 2 significant digits via `niceCeil`). X axis: first and last label only.
- Line: single polyline, stroke palette[0], width 2, no dots. X positions spaced evenly.
- StackedBars: one `<g>` per label slot; bars bottom-up in series order; bar width = plot width / len(labels) * 0.7; include a `<title>` child per rect (`"<label> <series>: <value>"`) for native hover tooltips. Legend: colored squares + labels along the top, left to right.
- Numbers ≥ 1000 in axis/legend labels abbreviated: `12.3k`, `4.5M` (helper `abbrev(float64) string`).
- Every text element: `font-family="system-ui,sans-serif" font-size="11" fill="#5f6b76"`.

- [ ] **Step 1: Write the failing golden test**

```go
package charts

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite golden files")

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden (run with -update): %v", err)
	}
	if string(want) != got {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s", name, got)
	}
}

func TestLineGolden(t *testing.T) {
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	var pts []Point
	for i := 0; i < 10; i++ {
		pts = append(pts, Point{X: base.AddDate(0, 0, i), Y: float64(3 + i*2)})
	}
	golden(t, "line.golden.svg", string(Line(pts, 480, 160)))
}

func TestStackedBarsGolden(t *testing.T) {
	labels := []string{"Jul 1", "Jul 2", "Jul 3"}
	series := []Series{
		{Label: "claude-sonnet-5", Values: []float64{1200, 3400, 2100}},
		{Label: "claude-haiku-4-5", Values: []float64{800, 600, 1500}},
	}
	golden(t, "bars.golden.svg", string(StackedBars(labels, series, 480, 200)))
}

func TestEmptyStates(t *testing.T) {
	if got := string(Line(nil, 480, 160)); !contains(got, "No data yet") {
		t.Fatalf("line empty state: %s", got)
	}
	if got := string(StackedBars(nil, nil, 480, 200)); !contains(got, "No data yet") {
		t.Fatalf("bars empty state: %s", got)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
```

(add `strings` import)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/portal/charts -v`
Expected: FAIL (package missing).

- [ ] **Step 3: Implement charts.go**

Implement per the rules above. Skeleton with the fiddly parts spelled out:

```go
package charts

import (
	"fmt"
	"html/template"
	"math"
	"strconv"
	"strings"
	"time"
)

var palette = []string{"#2563a8", "#4d9078", "#b0713f", "#7a5aa0", "#a84b57", "#5b7a99"}

const (
	mLeft, mRight, mTop, mBottom = 40.0, 20.0, 10.0, 24.0
	textAttrs                    = `font-family="system-ui,sans-serif" font-size="11" fill="#5f6b76"`
)

func f(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }

func abbrev(v float64) string {
	switch {
	case v >= 1e6:
		return strconv.FormatFloat(v/1e6, 'f', 1, 64) + "M"
	case v >= 1e3:
		return strconv.FormatFloat(v/1e3, 'f', 1, 64) + "k"
	default:
		return strconv.FormatFloat(v, 'f', 0, 64)
	}
}

func niceCeil(v float64) float64 {
	if v <= 0 {
		return 1
	}
	exp := math.Floor(math.Log10(v)) - 1
	step := math.Pow(10, exp)
	return math.Ceil(v/step) * step
}

func empty(w, h int) template.HTML {
	return template.HTML(fmt.Sprintf(
		`<svg viewBox="0 0 %d %d" width="%d" height="%d" role="img"><text x="%s" y="%s" text-anchor="middle" %s>No data yet</text></svg>`,
		w, h, w, h, f(float64(w)/2), f(float64(h)/2), textAttrs))
}

func Line(pts []Point, w, h int) template.HTML {
	if len(pts) == 0 {
		return empty(w, h)
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 %d %d" width="%d" height="%d" role="img">`, w, h, w, h)
	maxY := 0.0
	for _, p := range pts {
		maxY = math.Max(maxY, p.Y)
	}
	maxY = niceCeil(maxY)
	pw := float64(w) - mLeft - mRight
	ph := float64(h) - mTop - mBottom
	// gridlines + y labels at 0, mid, max
	for i, v := range []float64{0, maxY / 2, maxY} {
		y := mTop + ph - ph*v/maxY
		fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="#e3e7ea"/>`,
			f(mLeft), f(y), f(mLeft+pw), f(y))
		fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="end" %s>%s</text>`,
			f(mLeft-6), f(y+4), textAttrs, abbrev(v))
		_ = i
	}
	var poly []string
	for i, p := range pts {
		x := mLeft
		if len(pts) > 1 {
			x = mLeft + pw*float64(i)/float64(len(pts)-1)
		}
		y := mTop + ph - ph*p.Y/maxY
		poly = append(poly, f(x)+","+f(y))
	}
	fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="2"/>`,
		strings.Join(poly, " "), palette[0])
	// first/last x labels
	fmt.Fprintf(&b, `<text x="%s" y="%s" %s>%s</text>`,
		f(mLeft), f(float64(h)-6), textAttrs, pts[0].X.Format("Jan 2"))
	fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="end" %s>%s</text>`,
		f(mLeft+pw), f(float64(h)-6), textAttrs, pts[len(pts)-1].X.Format("Jan 2"))
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

func StackedBars(labels []string, series []Series, w, h int) template.HTML {
	if len(labels) == 0 || len(series) == 0 {
		return empty(w, h)
	}
	// ... same margin/scale approach: maxY = niceCeil(max column sum);
	// legend row at top (square 10x10 + label, advance x by 14 + 7*len(label));
	// per label slot i: x = mLeft + slotW*i + slotW*0.15, barW = slotW*0.7,
	// stack rects bottom-up with <title>label series: abbrev(v)</title>;
	// gridlines + y labels as in Line; every Nth x label where N = ceil(len/8).
	...
}
```

Complete `StackedBars` following the comment; keep escapes: labels pass through `template.HTMLEscapeString` before interpolation (series labels and axis labels are data).

- [ ] **Step 4: Generate goldens, eyeball, verify pass**

Run: `go test ./internal/portal/charts -update && go test ./internal/portal/charts -v`
Expected: PASS. Open `internal/portal/charts/testdata/bars.golden.svg` in a browser and sanity-check it visually before committing.

- [ ] **Step 5: gofmt, vet, commit**

```bash
gofmt -w internal/portal && go vet ./internal/portal/... && \
git add internal/portal/charts && \
git commit -m "feat(portal): deterministic server-rendered SVG charts"
```

---

### Task 4: Demo seed — deterministic org + history

**Files:**
- Create: `internal/portal/seed/seed.go`
- Test: `internal/portal/seed/seed_test.go`

**Interfaces:**
- Consumes: `store.Registry`, `store.Event`, `store.SaveRegistry`, `store.Open/AppendEvent`.
- Produces:

```go
package seed

// Demo writes registry.json and events.jsonl under dataDir.
// Deterministic: same epoch → byte-identical files. epoch is "now";
// history spans the 60 days before it. Idempotent: wipes the two files first.
func Demo(dataDir string, epoch time.Time) error
```

Content requirements:
- Org "Caltech Institute of Technology (demo)". 6 departments: Physics, Biology, Computer Science, Astronomy, Chemistry, Central IT. 15 teams (2–3 per dept, plausible names like "LIGO Ops", "Genomics Pipeline", "Campus Web"). 40 repos spread across teams.
- Adoption mix: 28 repos governed (posture events), 12 ungoverned (no posture events; ~half of those still get `provider_usage` — the "AI activity with zero governance" story).
- Posture: of the 28 governed — 21 `in-sync`, 4 `drifted`, 3 `stale`. Packs: most repos `org-baseline@1.2.0` (signed), stale ones `org-baseline@1.1.0`, 6 repos add a team pack (e.g. `physics-hpc@0.3.0`).
- First-sync dates staggered over the 60 days (adoption curve grows); each governed repo gets a recent `sync`/`status` event carrying its posture, plus periodic `update_check` events.
- Usage: `provider_usage` events per (team, model, day) for 60 days. Models: `claude-sonnet-5`, `claude-opus-5`, `claude-haiku-4-5`, `gpt-5.2`, `gemini-3-pro`. Weight by team size with weekly rhythm (weekends ~25%) and mild growth; costs from a per-model $/Mtok map. Daily totals in the tens-of-$ range so the weekly cost is plausible for a mid-size org.
- Agent tools on posture/mcp_connect events: `claude-code` (dominant), `cursor`, `gemini-cli`.
- Randomness: `rand.New(rand.NewSource(1849))` (math/rand, seeded — deterministic). NO `time.Now()` anywhere in the package; all times derive from `epoch`.
- Events appended in chronological order (sort before writing).

- [ ] **Step 1: Write the failing test**

```go
package seed

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tensorgroup/openescapement/internal/portal/store"
)

var epoch = time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)

func TestDemoDeterministic(t *testing.T) {
	d1, d2 := t.TempDir(), t.TempDir()
	if err := Demo(d1, epoch); err != nil {
		t.Fatal(err)
	}
	if err := Demo(d2, epoch); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"registry.json", "events.jsonl"} {
		a, err := os.ReadFile(filepath.Join(d1, name))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(d2, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(a) != string(b) {
			t.Fatalf("%s not byte-identical across runs", name)
		}
	}
}

func TestDemoShape(t *testing.T) {
	dir := t.TempDir()
	if err := Demo(dir, epoch); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	r := s.Registry()
	if len(r.Departments) != 6 || len(r.Teams) != 15 || len(r.Repos) != 40 {
		t.Fatalf("shape: %d depts %d teams %d repos",
			len(r.Departments), len(r.Teams), len(r.Repos))
	}
	events, err := s.Events()
	if err != nil {
		t.Fatal(err)
	}
	ov := store.Overview(r, events, epoch)
	if ov.GovernedRepos != 28 || ov.DriftedRepos != 4 || ov.StaleRepos != 3 {
		t.Fatalf("posture: %+v", ov)
	}
	if ov.TokensWeek == 0 || ov.CostWeek == 0 || len(ov.Models) < 4 {
		t.Fatalf("usage empty: %+v", ov)
	}
	for i := 1; i < len(events); i++ {
		if events[i].TS.Before(events[i-1].TS) {
			t.Fatal("events not chronological")
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/portal/seed -v`
Expected: FAIL (package missing).

- [ ] **Step 3: Implement seed.go**

Structure (fill with literal name tables; no external data):

```go
package seed

import (
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/tensorgroup/openescapement/internal/portal/store"
)

var costPerMtok = map[string]float64{ // blended $ per 1M tokens (demo figures)
	"claude-sonnet-5": 6, "claude-opus-5": 30, "claude-haiku-4-5": 2,
	"gpt-5.2": 8, "gemini-3-pro": 5,
}

func Demo(dataDir string, epoch time.Time) error {
	for _, f := range []string{"registry.json", "events.jsonl"} {
		if err := os.RemoveAll(filepath.Join(dataDir, f)); err != nil {
			return err
		}
	}
	rng := rand.New(rand.NewSource(1849))
	reg := buildRegistry() // literal tables: depts, teams, 40 repos
	if err := store.SaveRegistry(dataDir, reg); err != nil {
		return err
	}
	events := buildEvents(rng, reg, epoch) // posture + update_check + usage
	sort.SliceStable(events, func(i, j int) bool { return events[i].TS.Before(events[j].TS) })
	s, err := store.Open(dataDir)
	if err != nil {
		return err
	}
	for _, e := range events {
		if err := s.AppendEvent(e); err != nil {
			return err
		}
	}
	return nil
}
```

`buildRegistry` and `buildEvents` are ordinary deterministic loops over literal slices (never range over a map when emitting). Posture counts must hit the test's exact numbers: govern repos[0:28]; drifted = repos at indices {3, 9, 17, 25}; stale = {5, 12, 21} (stale repos' latest event carries `org-baseline@1.1.0` and Drift "stale").

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/portal/seed -v`
Expected: PASS (both tests). Determinism test is the gate — if it flakes, hunt the map iteration.

- [ ] **Step 5: gofmt, vet, commit**

```bash
gofmt -w internal/portal && go vet ./internal/portal/... && \
git add internal/portal/seed && \
git commit -m "feat(portal): deterministic demo seed — Caltech-shaped org, 60-day history"
```

---

### Task 5: `esc serve` scaffold — command, server, layout, auth

**Files:**
- Create: `internal/portal/web/server.go`, `internal/portal/web/templates/layout.html`, `internal/portal/web/static/style.css`
- Create: `internal/cli/serve.go`
- Modify: `internal/cli/cli.go` (add `serve` to the switch + usage string)
- Test: `internal/portal/web/server_test.go`, extend `internal/cli/cli_test.go`

**Interfaces:**
- Consumes: `store.Open`, `seed.Demo` (wired fully in Task 10).
- Produces:

```go
package web

type Server struct {
	Store    *store.Store
	Packs    *publish.Manager // nil until Task 8; handlers must nil-check
	Token    string           // "" = auth disabled (demo mode)
	Version  string
	Now      func() time.Time // injectable clock for tests; default time.Now
}

func New(st *store.Store, token, version string) *Server
func (s *Server) Handler() http.Handler
```

Routes registered in `Handler()` (Go 1.22 pattern syntax):
- `GET /{$}` overview (Task 6), `GET /fleet` (Task 7), `GET /packs`, `GET /packs/{name}`, `GET /packs/{name}/edit`, `POST /packs/{name}/publish` (Tasks 8–9), `GET /usage` (Task 11), `POST /api/v1/events` (Task 6b), `GET /static/style.css`.
- In this task, page routes render the layout with a "coming in this plan" body — replaced task by task.

Auth (this task):
- If `Token == ""`: no auth.
- Else: every non-`/static/` route requires cookie `esc_session` equal to Token, or header `Authorization: Bearer <Token>`. A request with query `?token=<Token>` sets the cookie (HttpOnly, SameSite=Lax) and 303-redirects to the same path without the query. Wrong/missing → 401 plain text.

CLI (this task):

```go
// internal/cli/serve.go
func cmdServe(root string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:8484", "listen address")
	dataDir := fs.String("data-dir", "", "server data directory (default: ~/.escapement/server)")
	demo := fs.Bool("demo", false, "seed demo data; disable auth (localhost only)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	// resolve dataDir; if --demo: seed.Demo(dataDir, time.Now().UTC()) + Task 10 repo seeding
	// token: "" if demo, else hex of 16 bytes from crypto/rand
	// print "esc portal: http://<addr>/?token=<token>" (or without token in demo)
	// http.Server{Addr, Handler}; shut down on SIGINT/SIGTERM via signal.NotifyContext
	// --demo refuses non-loopback --addr: error "demo mode binds localhost only", exit 2
}
```

**Critical:** in `cli.go`, dispatch `case "serve": return cmdServe(root, args[1:], stdout, stderr)` — do NOT pass the shared `ctx`: `Run` wraps commands in a 10-minute `context.WithTimeout` (`internal/cli/cli.go`) which would kill a long-running server. `cmdServe` builds its own `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`. Also skip the `maybeUpdates` pre-check for `serve`.

Layout template (`templates/layout.html`) — nav with the four pages + version in footer; each page template defines `"title"`, `"explainer"` (the plain-English strip), and `"content"`:

```html
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{block "title" .}}esc portal{{end}}</title>
<link rel="stylesheet" href="/static/style.css">
</head>
<body>
<header>
  <span class="brand">esc <strong>portal</strong></span>
  <nav>
    <a href="/">Overview</a><a href="/fleet">Fleet</a><a href="/packs">Rule Packs</a><a href="/usage">Usage</a>
  </nav>
</header>
<p class="explainer">{{block "explainer" .}}{{end}}</p>
<main>{{block "content" .}}{{end}}</main>
<footer>esc {{.Version}} — deterministic governance for AI usage</footer>
</body>
</html>
```

`style.css`: hand-written, ~90 lines. Light background `#fafbfc`, ink `#1c2733`, muted `#5f6b76`, accent `#2563a8`; system-ui type stack; sticky header; `.cards` grid of stat cards (white, 1px `#e3e7ea` border, 8px radius, big number + small label); clean table styles (row hover, status pills: `.pill.in-sync` green `#4d9078`, `.pill.drifted` amber `#b0713f`, `.pill.stale`/`.pill.ungoverned` red `#a84b57` / gray); `.explainer` muted single-line strip. No external fonts, no JS.

Templates embedded:

```go
//go:embed templates/*.html
var templateFS embed.FS
//go:embed static/*
var staticFS embed.FS
```

Parse once in `New` with `template.Must(template.New("").Funcs(funcs).ParseFS(templateFS, "templates/*.html"))`; per-page templates are added in later tasks via the same glob. Serve static via `http.FileServerFS` with `Cache-Control: no-store`.

- [ ] **Step 1: Write failing web tests**

```go
package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/portal/store"
)

func newTestServer(t *testing.T, token string) *Server {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return New(st, token, "test")
}

func get(t *testing.T, h http.Handler, path string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestPagesRenderWithoutAuth(t *testing.T) {
	h := newTestServer(t, "").Handler()
	for _, p := range []string{"/", "/fleet", "/packs", "/usage"} {
		rr := get(t, h, p, nil)
		if rr.Code != 200 {
			t.Fatalf("%s: code %d", p, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "esc <strong>portal</strong>") {
			t.Fatalf("%s: layout missing", p)
		}
	}
	if rr := get(t, h, "/static/style.css", nil); rr.Code != 200 {
		t.Fatalf("css: %d", rr.Code)
	}
}

func TestAuthRequired(t *testing.T) {
	h := newTestServer(t, "sekrit").Handler()
	if rr := get(t, h, "/", nil); rr.Code != 401 {
		t.Fatalf("unauth: %d", rr.Code)
	}
	if rr := get(t, h, "/", map[string]string{"Authorization": "Bearer sekrit"}); rr.Code != 200 {
		t.Fatalf("bearer: %d", rr.Code)
	}
	rr := get(t, h, "/?token=sekrit", nil)
	if rr.Code != 303 {
		t.Fatalf("token login: %d", rr.Code)
	}
	found := false
	for _, c := range rr.Result().Cookies() {
		if c.Name == "esc_session" && c.Value == "sekrit" && c.HttpOnly {
			found = true
		}
	}
	if !found {
		t.Fatal("session cookie not set")
	}
	if rr := get(t, h, "/?token=wrong", nil); rr.Code != 401 {
		t.Fatalf("bad token: %d", rr.Code)
	}
}

func TestUnknownRoute404(t *testing.T) {
	h := newTestServer(t, "").Handler()
	if rr := get(t, h, "/nope", nil); rr.Code != 404 {
		t.Fatalf("code %d", rr.Code)
	}
}
```

And in `internal/cli/cli_test.go` (uses existing `run` helper):

```go
func TestServeUsageErrors(t *testing.T) {
	code, out := run(t, t.TempDir(), "serve", "--bogus")
	if code != 2 {
		t.Fatalf("code=%d out=%s", code, out)
	}
	code, out = run(t, t.TempDir(), "serve", "--demo", "--addr", "0.0.0.0:9999")
	if code != 2 || !strings.Contains(out, "localhost") {
		t.Fatalf("demo non-loopback: code=%d out=%s", code, out)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/portal/web ./internal/cli -run 'TestPages|TestAuth|TestUnknown|TestServeUsage' -v`
Expected: FAIL (package/`cmdServe` missing).

- [ ] **Step 3: Implement server.go, templates/layout.html, static/style.css, serve.go, cli.go wiring**

Per the Interfaces block. Placeholder page templates for this task: four files `overview.html`, `fleet.html`, `packs.html`, `usage.html` each like

```html
{{define "title"}}Overview — esc portal{{end}}
{{define "content"}}<p class="muted">Coming soon.</p>{{end}}
```

rendered by a shared `render(w, name, data)` helper on `Server` that executes the layout with the page's blocks (use one `template.Template` per page, cloned from layout: `layoutTmpl.Clone()` then `ParseFS` the page file — the standard block-override pattern). Update the `usage` const in `cli.go` to list `serve`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/portal/web ./internal/cli -v`
Expected: PASS, including all pre-existing CLI tests.

- [ ] **Step 5: Manual smoke, gofmt, vet, commit**

Run: `go run ./cmd/esc serve --addr 127.0.0.1:8485` in background, `curl -s http://127.0.0.1:8485/ -H "Authorization: Bearer $(token from stderr)" | head -5`, kill it.

```bash
gofmt -w . && go vet ./... && \
git add internal/portal/web internal/cli && \
git commit -m "feat(portal): esc serve scaffold — http server, layout, token auth"
```

---

### Task 6: Overview page + ingest API

**Files:**
- Modify: `internal/portal/web/server.go` (handlers), `internal/portal/web/templates/overview.html`
- Create: `internal/portal/web/ingest.go`
- Test: `internal/portal/web/overview_test.go`, `internal/portal/web/ingest_test.go`

**Interfaces:**
- Consumes: `store.Overview`, `charts.Line`, `store.ValidKind`, `store.AppendEvent`.
- Produces: `POST /api/v1/events` accepting the `store.Event` JSON shape (unknown fields tolerated — plain `json.Unmarshal`). Rules: `kind` must pass `ValidKind` else 400 `{"error":"unknown kind"}`; zero `ts` is stamped server-side with `s.Now()`; success → 202 `{"ok":true}`. Auth: same policy as pages (bearer works; demo mode open).

Overview template content (exec framing, exactly five cards + one chart):

```html
{{define "title"}}Overview — esc portal{{end}}
{{define "explainer"}}Your organization's AI coding activity and the rules governing it, at a glance.{{end}}
{{define "content"}}
<div class="cards">
  <a class="card" href="/fleet"><span class="num">{{.Stats.GovernedRepos}}</span>
    <span class="label">governed repos <span class="muted">of {{.Stats.KnownRepos}} known</span></span></a>
  <a class="card{{if .Stats.DriftedRepos}} warn{{end}}" href="/fleet?status=drifted"><span class="num">{{.Stats.DriftedRepos}}</span>
    <span class="label">repos with drift</span></a>
  <a class="card{{if .Stats.StaleRepos}} warn{{end}}" href="/fleet?status=stale"><span class="num">{{.Stats.StaleRepos}}</span>
    <span class="label">repos on stale rules</span></a>
  <a class="card" href="/usage"><span class="num">${{printf "%.0f" .Stats.CostWeek}}</span>
    <span class="label">AI spend this week <span class="muted">{{abbrev .Stats.TokensWeek}} tokens</span></span></a>
  <a class="card" href="/usage"><span class="num">{{len .Stats.Models}}</span>
    <span class="label">models in use</span></a>
</div>
<section>
  <h2>Adoption — governed repos over time</h2>
  {{.AdoptionChart}}
</section>
<section>
  <h2>Models in use</h2>
  <ul class="chips">{{range .Stats.Models}}<li>{{.}}</li>{{end}}</ul>
</section>
{{end}}
```

(`abbrev` exposed as a template func wrapping `charts`-style abbreviation for int64; add `charts.Abbrev(v float64) string` as an exported helper and register `"abbrev"` in the func map.)

- [ ] **Step 1: Write failing tests**

`overview_test.go`:

```go
func TestOverviewShowsSeededStats(t *testing.T) {
	dir := t.TempDir()
	epoch := time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
	if err := seed.Demo(dir, epoch); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := New(st, "", "test")
	s.Now = func() time.Time { return epoch }
	rr := get(t, s.Handler(), "/", nil)
	if rr.Code != 200 {
		t.Fatalf("code %d", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{"governed repos", ">28<", "repos with drift", "<svg", "claude-sonnet-5"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in body", want)
		}
	}
}
```

`ingest_test.go`:

```go
func TestIngestAcceptsValidEvent(t *testing.T) {
	s := newTestServer(t, "tok")
	body := `{"kind":"sync","repo_id":"r9","drift":"in-sync","future_field":123}`
	req := httptest.NewRequest("POST", "/api/v1/events", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != 202 {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	events, err := s.Store.Events()
	if err != nil || len(events) != 1 || events[0].RepoID != "r9" {
		t.Fatalf("events=%v err=%v", events, err)
	}
	if events[0].TS.IsZero() {
		t.Fatal("ts not stamped")
	}
}

func TestIngestRejects(t *testing.T) {
	s := newTestServer(t, "tok")
	h := s.Handler()
	cases := []struct {
		body, auth string
		want       int
	}{
		{`{"kind":"sync"}`, "", 401},
		{`{"kind":"exec"}`, "Bearer tok", 400},
		{`{not json`, "Bearer tok", 400},
	}
	for _, c := range cases {
		req := httptest.NewRequest("POST", "/api/v1/events", strings.NewReader(c.body))
		if c.auth != "" {
			req.Header.Set("Authorization", c.auth)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != c.want {
			t.Fatalf("body %q auth %q: got %d want %d", c.body, c.auth, rr.Code, c.want)
		}
	}
	if events, _ := s.Store.Events(); len(events) != 0 {
		t.Fatal("rejected events were stored")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/portal/web -run 'TestOverview|TestIngest' -v`
Expected: FAIL.

- [ ] **Step 3: Implement overview handler, template, ingest.go**

Handler reads `s.Store.Events()` + `Registry()` per request, calls `store.Overview(reg, events, s.Now())`, builds `charts.Line` from `Adoption` points. Ingest per the Produces rules; limit body with `http.MaxBytesReader(w, r.Body, 64<<10)`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/portal/web -v`
Expected: PASS.

- [ ] **Step 5: gofmt, vet, commit**

```bash
gofmt -w . && go vet ./... && \
git add internal/portal/web && \
git commit -m "feat(portal): overview page + events ingest API"
```

---

### Task 7: Fleet page

**Files:**
- Modify: `internal/portal/web/server.go`, `internal/portal/web/templates/fleet.html`
- Test: `internal/portal/web/fleet_test.go`

**Interfaces:**
- Consumes: `store.FleetRows`. Query param `status` ∈ `{in-sync, drifted, stale, ungoverned}` filters rows (empty = all; unknown value = all + ignore).

Template: explainer "Every team and repository, which rulebook version it follows, and whether it's up to date." Filter links (`All · In sync · Drifted · Stale · Ungoverned`, current one bold), then table: Department / Team / Repository / Rule packs / Last sync / Status pill / Agent tools. `LastSync` formatted `Jan 2 15:04` or `—` when zero. Status cell: `<span class="pill {{.Status}}">{{.Status}}</span>`.

- [ ] **Step 1: Write failing test**

```go
func TestFleetFilters(t *testing.T) {
	dir := t.TempDir()
	epoch := time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
	if err := seed.Demo(dir, epoch); err != nil {
		t.Fatal(err)
	}
	st, _ := store.Open(dir)
	s := New(st, "", "test")
	h := s.Handler()

	all := get(t, h, "/fleet", nil).Body.String()
	if got := strings.Count(all, `class="pill`); got != 40 {
		t.Fatalf("all rows: %d pills", got)
	}
	drifted := get(t, h, "/fleet?status=drifted", nil).Body.String()
	if got := strings.Count(drifted, `class="pill`); got != 4 {
		t.Fatalf("drifted rows: %d", got)
	}
	if !strings.Contains(drifted, "org-baseline@") {
		t.Fatal("pack labels missing")
	}
	stale := get(t, h, "/fleet?status=stale", nil).Body.String()
	if !strings.Contains(stale, "org-baseline@1.1.0") {
		t.Fatal("stale repos should show old pack version")
	}
}
```

- [ ] **Step 2: Run to verify it fails** — `go test ./internal/portal/web -run TestFleetFilters -v` → FAIL.

- [ ] **Step 3: Implement handler + template.**

- [ ] **Step 4: Run to verify it passes** — `go test ./internal/portal/web -v` → PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w . && go vet ./... && git add internal/portal/web && \
git commit -m "feat(portal): fleet page — teams, repos, drift status, filters"
```

---

### Task 8: Publish manager — server-side pack clones

**Files:**
- Create: `internal/portal/publish/publish.go`
- Test: `internal/portal/publish/publish_test.go`

**Interfaces:**
- Consumes: `pack.Load(dir string) (*pack.Pack, error)` (validation), system `git`.
- Produces:

```go
package publish

// Manager works on working git clones under Dir: Dir/<name>/ is a git repo
// whose root is an esc pack (pack.yaml at top level).
type Manager struct{ Dir string }

type PackInfo struct {
	Name, Version, Description, Dir string
	Fragments                       []string // sorted relative paths from manifest Rules+Skills
	Tags                            []Tag    // newest first
}
type Tag struct {
	Name string // e.g. "v1.2.0"
	Date string // "2026-07-30"
}

func NewManager(dir string) *Manager
func (m *Manager) List(ctx context.Context) ([]PackInfo, error) // sorted by Name; skips non-repo dirs
func (m *Manager) Get(ctx context.Context, name string) (*PackInfo, error)
func (m *Manager) ReadFragment(name, frag string) ([]byte, error) // rejects path escape via pack-style safeRel check
func (m *Manager) Diff(ctx context.Context, name, frag string, proposed []byte) (string, error)
// Diff: git diff --no-index of current fragment vs proposed (temp file); "" if identical.
func (m *Manager) Publish(ctx context.Context, name, frag string, content []byte, newVersion string) error
```

`Publish` algorithm (exact, order matters — nothing half-published):
1. Guard: `newVersion` matches `^\d+\.\d+\.\d+$`; fragment path is inside the clone (reject `..`, absolute, symlink targets); tag `v<newVersion>` must not already exist (`git tag -l`).
2. Write `content` to the fragment file; rewrite `version:` in `pack.yaml` via `yaml.Node` (unmarshal to `yaml.Node`, walk top-level mapping for key `version`, replace scalar value — preserves comments/order), write back.
3. Validate: `pack.Load(cloneDir)` — on error, restore with `git checkout -- . && git clean -fd`, return the error wrapped: `fmt.Errorf("%w: pack validation failed: %v", esc.ErrManifest, err)`.
4. `git add -A`, `git -c user.name="esc portal" -c user.email=portal@escapement.local -c commit.gpgsign=false commit -m "portal: publish <name> v<newVersion>"`, `git -c user.name=... -c user.email=... -c tag.gpgsign=false tag -a v<newVersion> -m "publish v<newVersion>"`.
5. Any git failure after step 2: attempt the same restore, return error.

(Note: portal tags are unsigned in v1 — demo consumers use `trust: unsigned`. Signing hooks are a follow-up; noted in CHANGELOG.)

Run git via a small helper `gitRun(ctx context.Context, dir string, args ...string) (string, error)` using `exec.CommandContext`, capturing combined output into the error.

- [ ] **Step 1: Write failing test**

```go
package publish

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func newPackClone(t *testing.T) (mgrDir string) {
	t.Helper()
	mgrDir = t.TempDir()
	dir := filepath.Join(mgrDir, "org-baseline")
	if err := os.MkdirAll(filepath.Join(dir, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"pack.yaml": "schema: 1\nname: org-baseline\nversion: 1.2.0\n# keep this comment\nrules:\n  - path: rules/security.md\n    targets: [claude, agents]\n",
		"rules/security.md": "---\ntargets: [claude, agents]\n---\n# Security\n\n- Never commit secrets.\n",
	}
	for p, c := range files {
		if err := os.WriteFile(filepath.Join(dir, p), []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "user.email", "t@t")
	git(t, dir, "config", "user.name", "t")
	git(t, dir, "config", "commit.gpgsign", "false")
	git(t, dir, "config", "tag.gpgsign", "false")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "init")
	git(t, dir, "tag", "-a", "v1.2.0", "-m", "v1.2.0")
	return mgrDir
}

func TestListAndGet(t *testing.T) {
	m := NewManager(newPackClone(t))
	infos, err := m.List(context.Background())
	if err != nil || len(infos) != 1 {
		t.Fatalf("infos=%v err=%v", infos, err)
	}
	p := infos[0]
	if p.Name != "org-baseline" || p.Version != "1.2.0" ||
		len(p.Fragments) != 1 || p.Fragments[0] != "rules/security.md" ||
		len(p.Tags) != 1 || p.Tags[0].Name != "v1.2.0" {
		t.Fatalf("info=%+v", p)
	}
}

func TestPublishHappyPath(t *testing.T) {
	m := NewManager(newPackClone(t))
	ctx := context.Background()
	newBody := "---\ntargets: [claude, agents]\n---\n# Security\n\n- Never commit secrets.\n- All new ports need review.\n"
	if err := m.Publish(ctx, "org-baseline", "rules/security.md", []byte(newBody), "1.3.0"); err != nil {
		t.Fatal(err)
	}
	p, err := m.Get(ctx, "org-baseline")
	if err != nil || p.Version != "1.3.0" || p.Tags[0].Name != "v1.3.0" {
		t.Fatalf("after publish: %+v err=%v", p, err)
	}
	manifest, _ := os.ReadFile(filepath.Join(m.Dir, "org-baseline", "pack.yaml"))
	if !strings.Contains(string(manifest), "version: 1.3.0") ||
		!strings.Contains(string(manifest), "# keep this comment") {
		t.Fatalf("manifest rewrite lost content:\n%s", manifest)
	}
	// worktree clean
	if out := git(t, filepath.Join(m.Dir, "org-baseline"), "status", "--porcelain"); out != "" {
		t.Fatalf("dirty worktree: %s", out)
	}
}

func TestPublishValidationFailureRestores(t *testing.T) {
	m := NewManager(newPackClone(t))
	ctx := context.Background()
	// Invalid: fragment declares a target outside ValidTargets.
	bad := "---\ntargets: [nonsense]\n---\nbody\n"
	err := m.Publish(ctx, "org-baseline", "rules/security.md", []byte(bad), "1.3.0")
	if err == nil {
		t.Fatal("expected validation error")
	}
	p, _ := m.Get(ctx, "org-baseline")
	if p.Version != "1.2.0" || len(p.Tags) != 1 {
		t.Fatalf("not restored: %+v", p)
	}
	body, _ := m.ReadFragment("org-baseline", "rules/security.md")
	if strings.Contains(string(body), "nonsense") {
		t.Fatal("fragment not restored")
	}
}

func TestPublishRejects(t *testing.T) {
	m := NewManager(newPackClone(t))
	ctx := context.Background()
	good := []byte("---\ntargets: [claude]\n---\nok\n")
	if err := m.Publish(ctx, "org-baseline", "../escape.md", good, "1.3.0"); err == nil {
		t.Fatal("path escape accepted")
	}
	if err := m.Publish(ctx, "org-baseline", "rules/security.md", good, "1.2.0"); err == nil {
		t.Fatal("existing version accepted")
	}
	if err := m.Publish(ctx, "org-baseline", "rules/security.md", good, "nope"); err == nil {
		t.Fatal("bad version accepted")
	}
}

func TestDiff(t *testing.T) {
	m := NewManager(newPackClone(t))
	ctx := context.Background()
	cur, _ := m.ReadFragment("org-baseline", "rules/security.md")
	same, err := m.Diff(ctx, "org-baseline", "rules/security.md", cur)
	if err != nil || same != "" {
		t.Fatalf("identical diff: %q err=%v", same, err)
	}
	d, err := m.Diff(ctx, "org-baseline", "rules/security.md", append(cur, []byte("- New rule.\n")...))
	if err != nil || !strings.Contains(d, "+- New rule.") {
		t.Fatalf("diff=%q err=%v", d, err)
	}
}
```

(Note: `git diff --no-index` exits 1 when files differ — treat exit code 1 as success in `Diff`.)

- [ ] **Step 2: Run to verify it fails** — `go test ./internal/portal/publish -v` → FAIL.

- [ ] **Step 3: Implement publish.go per the algorithm above.**

For `List`: read subdirectories of `Dir`; a subdir qualifies if `pack.yaml` exists and `.git` exists; `pack.Load` it for Name/Version/Description/fragment paths (`Rules[].Path` + `Skills[].Path`); tags via `gitRun(ctx, dir, "for-each-ref", "--sort=-creatordate", "--format=%(refname:short) %(creatordate:short)", "refs/tags")`.

- [ ] **Step 4: Run to verify it passes** — `go test ./internal/portal/publish -v` → PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w . && go vet ./... && git add internal/portal/publish && \
git commit -m "feat(portal): publish manager — validate, commit, tag pack versions"
```

---

### Task 9: Packs pages — list, detail, edit → publish

**Files:**
- Modify: `internal/portal/web/server.go` (wire `Packs *publish.Manager` + handlers), `internal/portal/web/templates/packs.html`
- Create: `internal/portal/web/templates/pack.html`, `internal/portal/web/templates/pack_edit.html`, `internal/portal/web/markdown.go`
- Test: `internal/portal/web/packs_test.go`, `internal/portal/web/markdown_test.go`

**Interfaces:**
- Consumes: Task 8 `publish.Manager` (`List/Get/ReadFragment/Diff/Publish`), `esc.ErrManifest` for 422 mapping.
- Produces: `mdHTML(src []byte) template.HTML` — minimal safe markdown: escape everything first (`template.HTMLEscapeString` line by line), then `#`/`##`/`###` → `h2/h3/h4`, `- ` runs → `<ul><li>`, ``` fences → `<pre><code>`, blank-line-separated paragraphs, `**bold**` and `` `code` `` inline. Nothing else (no links/images/raw HTML — by design, this renders policy text).

Pages:
- `/packs`: explainer "These are the AI rulebooks your teams' coding agents read on every session." Card per pack: name, current version, description, tag count, last published date, "View" link. Empty state: "No pack repos configured. Clone a pack repo into the server's packs directory." When `s.Packs == nil` treat as empty.
- `/packs/{name}`: version history (tag list with dates), fragment list; each fragment shows `mdHTML` rendering with an "Edit" link → `/packs/{name}/edit?frag=rules/security.md`.
- `/packs/{name}/edit?frag=...`: form — `<textarea name="content">` (current fragment), `<input name="version">` (placeholder = suggested next patch version, computed by bumping current), buttons "Preview diff" (`name="action" value="diff"` re-renders the form with `Diff` output in a `<pre class="diff">`) and "Publish" (`value="publish"`).
- `POST /packs/{name}/publish`: form fields `frag`, `content`, `version`, `action`. `diff` → re-render edit page with diff. `publish` → `Publish`; success 303 → `/packs/{name}?published=v<version>` (detail page shows a success banner); validation failure → 422, re-render edit form with the error message shown in a `.error` box and the user's content preserved.

- [ ] **Step 1: Write failing tests**

`markdown_test.go`:

```go
func TestMdHTML(t *testing.T) {
	src := "# Title\n\nPara with **bold** and `code`.\n\n- a\n- b\n\n```\nx < y\n```\n\n<script>alert(1)</script>\n"
	got := string(mdHTML([]byte(src)))
	for _, want := range []string{"<h2>Title</h2>", "<strong>bold</strong>", "<code>code</code>",
		"<li>a</li>", "<pre><code>x &lt; y", "&lt;script&gt;"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "<script>") {
		t.Fatal("raw HTML leaked")
	}
}
```

`packs_test.go` (reuses `newPackClone` pattern — export a test helper `newTestServerWithPacks(t)` that builds the Task 8 fixture clone, `publish.NewManager`, and a `Server` with it):

```go
func TestPacksListAndDetail(t *testing.T) {
	s := newTestServerWithPacks(t)
	h := s.Handler()
	list := get(t, h, "/packs", nil).Body.String()
	if !strings.Contains(list, "org-baseline") || !strings.Contains(list, "1.2.0") {
		t.Fatalf("list: %s", list)
	}
	detail := get(t, h, "/packs/org-baseline", nil).Body.String()
	for _, want := range []string{"v1.2.0", "rules/security.md", "<h2>Security</h2>", "Edit"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("detail missing %q", want)
		}
	}
	if rr := get(t, h, "/packs/nope", nil); rr.Code != 404 {
		t.Fatalf("unknown pack: %d", rr.Code)
	}
}

func TestEditAndPublishFlow(t *testing.T) {
	s := newTestServerWithPacks(t)
	h := s.Handler()
	edit := get(t, h, "/packs/org-baseline/edit?frag=rules/security.md", nil).Body.String()
	if !strings.Contains(edit, "<textarea") || !strings.Contains(edit, `value="1.3.0"`) {
		t.Fatalf("edit page: %s", edit)
	}
	form := url.Values{
		"frag":    {"rules/security.md"},
		"content": {"---\ntargets: [claude, agents]\n---\n# Security\n\n- New rule.\n"},
		"version": {"1.3.0"},
		"action":  {"publish"},
	}
	req := httptest.NewRequest("POST", "/packs/org-baseline/publish", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 303 || rr.Header().Get("Location") != "/packs/org-baseline?published=v1.3.0" {
		t.Fatalf("publish: %d %s %s", rr.Code, rr.Header().Get("Location"), rr.Body.String())
	}
	detail := get(t, h, "/packs/org-baseline?published=v1.3.0", nil).Body.String()
	if !strings.Contains(detail, "v1.3.0") || !strings.Contains(detail, "Published") {
		t.Fatal("new version not visible")
	}
}

func TestPublishValidationErrorKeepsContent(t *testing.T) {
	s := newTestServerWithPacks(t)
	form := url.Values{
		"frag":    {"rules/security.md"},
		"content": {"---\ntargets: [nonsense]\n---\nbroken\n"},
		"version": {"1.3.0"},
		"action":  {"publish"},
	}
	req := httptest.NewRequest("POST", "/packs/org-baseline/publish", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != 422 {
		t.Fatalf("code %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "nonsense") || !strings.Contains(body, "class=\"error\"") {
		t.Fatal("error page must preserve content and show error")
	}
}
```

- [ ] **Step 2: Run to verify they fail** — `go test ./internal/portal/web -run 'TestMd|TestPacks|TestEdit|TestPublish' -v` → FAIL.

- [ ] **Step 3: Implement markdown.go, handlers, three templates.**

Wire `Packs` into `New`: change signature to `New(st *store.Store, packs *publish.Manager, token, version string) *Server` and update Task 5/6/7 call sites (tests pass `nil` where packs don't matter).

- [ ] **Step 4: Run to verify pass** — `go test ./internal/portal/web -v` → PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w . && go vet ./... && git add internal/portal/web && \
git commit -m "feat(portal): rule pack pages — view, edit, diff, publish"
```

---

### Task 10: Demo mode end-to-end — seeded repos + full loop

**Files:**
- Create: `internal/portal/seed/repos.go`
- Modify: `internal/cli/serve.go` (wire `--demo` fully: `seed.Demo` + `seed.Repos` + `publish.NewManager(dataDir/packs)`)
- Test: `internal/portal/seed/repos_test.go`, `internal/cli/serve_e2e_test.go`

**Interfaces:**
- Consumes: `publish.Manager`, `cli.Run` (for `esc sync`), existing config YAML shape (`.escapement/config.yaml` with `source: file://<dir>`, `trust: unsigned`).
- Produces:

```go
package seed

// Repos creates, if absent (idempotent by existence check):
//   dataDir/packs/org-baseline/   — git repo, the Task 8 fixture shape, tags v1.1.0 + v1.2.0
//   dataDir/demo-repo/            — git repo with .escapement/config.yaml pointing at the
//                                   pack repo via file://, ref v1.2.0, trust: unsigned
// Returns the two paths.
func Repos(dataDir string) (packDir, demoRepo string, err error)
```

Pack content: `pack.yaml` (schema 1, name org-baseline, version 1.2.0, one rule `rules/security.md` targeting claude+agents), the security fragment with 3 memorable demo rules ("Never commit secrets or API keys.", "All authentication goes through the campus SSO service.", "Any newly opened port requires security review."). Commit v1.1.0 first (2 rules), then v1.2.0 (3 rules) — so version history has two entries. Git identity set locally (`user.name "esc demo"`, `user.email demo@escapement.local`, gpgsign off) — never touches global config.

`cmdServe --demo` final behavior: resolve dataDir → `seed.Demo` (always, re-seed each start for a clean pitch) → `seed.Repos` (idempotent) → `store.Open` → `publish.NewManager(filepath.Join(dataDir, "packs"))` → `web.New(st, mgr, "", cli.Version)` → print:

```
esc portal (demo): http://127.0.0.1:8484/
demo governed repo: <demoRepo>   (cd there and run `esc sync` after publishing)
```

Non-demo: same wiring minus seeding, with token auth; packs dir may be empty (packs page shows the empty state).

- [ ] **Step 1: Write failing tests**

`repos_test.go`:

```go
func TestReposIdempotent(t *testing.T) {
	dir := t.TempDir()
	p1, r1, err := Repos(dir)
	if err != nil {
		t.Fatal(err)
	}
	p2, r2, err := Repos(dir)
	if err != nil || p1 != p2 || r1 != r2 {
		t.Fatalf("not idempotent: %v", err)
	}
	cfg, err := os.ReadFile(filepath.Join(r1, ".escapement", "config.yaml"))
	if err != nil || !strings.Contains(string(cfg), "trust: unsigned") {
		t.Fatalf("config: %s err=%v", cfg, err)
	}
}
```

`serve_e2e_test.go` (package `cli` — the pitch loop as a test, using existing `run` helper):

```go
func TestDemoPublishSyncLoop(t *testing.T) {
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	dataDir := t.TempDir()
	packDir, demoRepo, err := seed.Repos(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	// 1. Initial sync: managed block lands at v1.2.0.
	code, out := run(t, demoRepo, "sync")
	if code != 0 {
		t.Fatalf("sync: %d %s", code, out)
	}
	claude, _ := os.ReadFile(filepath.Join(demoRepo, "CLAUDE.md"))
	if !strings.Contains(string(claude), "org-baseline@1.2.0") {
		t.Fatalf("initial block: %s", claude)
	}
	// 2. Publish v1.3.0 through the portal's publish manager.
	m := publish.NewManager(filepath.Dir(packDir))
	cur, err := m.ReadFragment("org-baseline", "rules/security.md")
	if err != nil {
		t.Fatal(err)
	}
	newBody := append(cur, []byte("- Model routing: use fast models for code, reasoning models for review.\n")...)
	if err := m.Publish(context.Background(), "org-baseline", "rules/security.md", newBody, "1.3.0"); err != nil {
		t.Fatal(err)
	}
	// 3. Re-pin demo repo to v1.3.0 and sync (the on-screen pitch step).
	cfgPath := filepath.Join(demoRepo, ".escapement", "config.yaml")
	cfg, _ := os.ReadFile(cfgPath)
	if err := os.WriteFile(cfgPath,
		[]byte(strings.ReplaceAll(string(cfg), "v1.2.0", "v1.3.0")), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out = run(t, demoRepo, "sync")
	if code != 0 {
		t.Fatalf("resync: %d %s", code, out)
	}
	claude, _ = os.ReadFile(filepath.Join(demoRepo, "CLAUDE.md"))
	if !strings.Contains(string(claude), "org-baseline@1.3.0") ||
		!strings.Contains(string(claude), "Model routing") {
		t.Fatalf("published rule did not arrive:\n%s", claude)
	}
}
```

(Imports: `internal/portal/seed`, `internal/portal/publish`. This is THE test that proves the pitch loop: portal publish → `esc sync` → rule text in CLAUDE.md, with no server-side push.)

- [ ] **Step 2: Run to verify they fail** — `go test ./internal/portal/seed ./internal/cli -run 'TestRepos|TestDemoPublish' -v` → FAIL.

- [ ] **Step 3: Implement repos.go and finish serve.go wiring.**

- [ ] **Step 4: Run to verify pass** — `go test ./internal/portal/seed ./internal/cli -v` → PASS (all existing CLI tests too).

- [ ] **Step 5: Manual demo smoke + commit**

Run `go run ./cmd/esc serve --demo`, open the printed URL, click all four pages, publish a rule edit from the browser, run `esc sync` in the printed demo repo, confirm CLAUDE.md changes. Then:

```bash
gofmt -w . && go vet ./... && git add internal/portal internal/cli && \
git commit -m "feat(portal): --demo mode — seeded org, pack repo, live publish→sync loop"
```

---

### Task 11: Usage page

**Files:**
- Modify: `internal/portal/web/server.go`, `internal/portal/web/templates/usage.html`
- Test: `internal/portal/web/usage_test.go`

**Interfaces:**
- Consumes: `store.UsageDaily`, `charts.StackedBars`, `charts.Abbrev`.
- Query params: `team` (team ID), `model`, `days` ∈ {7, 30, 60} default 30. Invalid values fall back to defaults silently.

Page content:
- Explainer: "What your organization spends on AI models, by team and model. Aggregate token counts only — no prompts or code are ever collected."
- Filter row: `<select>`s for team (from registry, "All teams") and model (distinct models in events, "All models"), days links `7d · 30d · 60d`; plain GET form, no JS beyond browser-native form submit.
- Chart 1: tokens per day stacked **by model** (series = models, labels = days: build a `charts.Series` per model over the day range, zero-filled).
- Chart 2: cost per day stacked **by team** (series = team names).
- Summary table below: per model — total tokens, est. cost, share %; footer row with totals. Attribution honesty line under the charts: `<p class="muted">Aggregate usage by API workspace. Per-tool attribution arrives with MCP connection telemetry.</p>`.

- [ ] **Step 1: Write failing test**

```go
func TestUsagePage(t *testing.T) {
	dir := t.TempDir()
	epoch := time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
	if err := seed.Demo(dir, epoch); err != nil {
		t.Fatal(err)
	}
	st, _ := store.Open(dir)
	s := New(st, nil, "", "test")
	s.Now = func() time.Time { return epoch }
	h := s.Handler()
	body := get(t, h, "/usage", nil).Body.String()
	for _, want := range []string{"claude-sonnet-5", "<svg", "no prompts or code", "est. cost"} {
		if !strings.Contains(strings.ToLower(body), strings.ToLower(want)) {
			t.Fatalf("missing %q", want)
		}
	}
	if strings.Count(body, "<svg") != 2 {
		t.Fatal("want two charts")
	}
	filtered := get(t, h, "/usage?model=claude-opus-5&days=7", nil).Body.String()
	if !strings.Contains(filtered, "claude-opus-5") {
		t.Fatal("filter lost")
	}
}
```

- [ ] **Step 2: Run to verify it fails** — `go test ./internal/portal/web -run TestUsagePage -v` → FAIL.

- [ ] **Step 3: Implement handler + template.** Consult the `dataviz` skill before finalizing chart composition details if adjusting `charts` output.

- [ ] **Step 4: Run to verify pass** — `go test ./internal/portal/web -v` → PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w . && go vet ./... && git add internal/portal/web && \
git commit -m "feat(portal): usage page — token/cost charts with team and model filters"
```

---

### Task 12: Docs, changelog, full verification

**Files:**
- Modify: `CHANGELOG.md`, `README.md`

**Interfaces:** none.

- [ ] **Step 1: CHANGELOG entry** under `## [Unreleased]` / `### Added` (match existing bullet style):

```markdown
- **`esc serve`** — the admin portal: a one-binary control plane with overview, fleet,
  rule-pack, and usage pages, a bearer-token events ingest API, and portal-side pack
  publishing (validate → commit → tag; distribution still happens only via `esc sync`).
  `--demo` seeds a deterministic fictional org plus a local pack repo and governed repo
  for an end-to-end publish→sync walkthrough. Portal-published tags are unsigned in v1.
```

- [ ] **Step 2: README** — add a short `esc serve` subsection under the existing "Quickstart" section (3–6 lines: the `--demo` one-liner, what the four pages show, note that the portal never pushes rules into repos). Keep the README's existing tone; no em-dash-free requirement applies here but stay concise.

- [ ] **Step 3: Full verification**

Run: `gofmt -l . && go vet ./... && go test ./...`
Expected: gofmt lists nothing, vet clean, ALL packages pass (including pre-existing engine/render/updatecheck suites).

- [ ] **Step 4: Commit**

```bash
git add CHANGELOG.md README.md && \
git commit -m "docs: changelog + README for esc serve admin portal"
```

---

## Self-Review Notes (resolved during planning)

- **Spec coverage:** architecture→T1/T5, ingest→T6, seed→T4/T10, pack admin→T8/T9, four pages→T6/T7/T9/T11, auth→T5, demo loop→T10, error handling→T6/T8/T9, testing→every task; CHANGELOG/README→T12. Out-of-scope items from the spec (OIDC, MCP, collectors, SIEM, multi-tenancy) have no tasks — intentional.
- **10-minute context timeout** in `cli.Run` would kill the server — `serve` dispatches without the shared ctx (T5).
- **`New` signature changes in T9** (adds `packs` param) — T9 explicitly updates earlier call sites.
- **`git diff --no-index` exit code 1 on difference** — handled in T8.
- **Spec's "`ungoverned`" vs `Repo.Governed`:** posture events are the source of truth for status; the registry flag only marks known repos (T2 semantics block).
