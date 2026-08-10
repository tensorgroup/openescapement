// Package store persists the portal's registry (JSON files) and telemetry
// events (append-only JSONL) under a data directory.
package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tensorgroup/openescapement/internal/engine"
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

type Registry struct {
	Org         Org          `json:"org"`
	Departments []Department `json:"departments"`
	Teams       []Team       `json:"teams"`
	Repos       []Repo       `json:"repos"`
}
type Org struct {
	Name string `json:"name"`
}
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
	// Remote is the repo's normalized git remote (publisher.NormalizeRemote),
	// the join key ingest uses to resolve an incoming envelope's Remote to a
	// registered repo. Empty for repos registered before a remote was known.
	Remote string `json:"remote,omitempty"`
}

// RepoByRemote returns the repo whose Remote matches remote, and whether one
// was found. remote is expected to already be normalized
// (publisher.NormalizeRemote); an empty remote never matches, since an empty
// Repo.Remote must not accidentally match an empty incoming remote.
func (r Registry) RepoByRemote(remote string) (Repo, bool) {
	if remote == "" {
		return Repo{}, false
	}
	for _, repo := range r.Repos {
		if repo.Remote != "" && repo.Remote == remote {
			return repo, true
		}
	}
	return Repo{}, false
}

// TeamAndDept resolves a team ID to its team and department name, for pages
// that need one repo's org placement outside FleetRows' bulk computation
// (e.g. the repo detail page). Zero values when teamID matches nothing.
func (r Registry) TeamAndDept(teamID string) (team Team, deptName string) {
	deptByID := map[string]string{}
	for _, d := range r.Departments {
		deptByID[d.ID] = d.Name
	}
	for _, t := range r.Teams {
		if t.ID == teamID {
			return t, deptByID[t.DeptID]
		}
	}
	return Team{}, ""
}

// UnassignedRepos returns registry repos with no Remote bound yet, sorted by
// name, the candidate list for the unregistered-remote register affordance:
// a remote can only be assigned to a repo that doesn't already have one, so
// an accidental reassignment never silently clobbers an existing binding.
func (r Registry) UnassignedRepos() []Repo {
	var out []Repo
	for _, repo := range r.Repos {
		if repo.Remote == "" {
			out = append(out, repo)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
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
	TS     time.Time `json:"ts"`
	Kind   string    `json:"kind"` // sync|status|update_check|provider_usage|mcp_connect
	RepoID string    `json:"repo_id,omitempty"`
	TeamID string    `json:"team_id,omitempty"`
	// Remote is the normalized git remote an envelope-sourced event arrived
	// with (publisher.NormalizeRemote). Set whether or not it resolved to a
	// registered repo: an unmatched remote is retained here, never dropped,
	// so an unregistered repo's activity stays visible as shadow IT rather
	// than vanishing at ingest.
	Remote     string             `json:"remote,omitempty"`
	AgentTool  string             `json:"agent_tool,omitempty"`
	Model      string             `json:"model,omitempty"`
	Packs      []EventPack        `json:"packs,omitempty"`
	Drift      string             `json:"drift,omitempty"` // in-sync|drifted|stale; derived, see DeriveDrift
	Tokens     *Tokens            `json:"tokens,omitempty"`
	Artifacts  []EventArtifact    `json:"artifacts,omitempty"`
	Collection *engine.Collection `json:"collection,omitempty"`
}

type Store struct {
	dir string

	mu  sync.RWMutex
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

// cloneRegistry returns a Registry whose slices share no backing array with
// r's. Registry is a value type, but Go copies a slice field by header
// only (pointer/len/cap) — a plain `x := r` still aliases r's backing
// array, so mutating x.Repos[i] mutates whatever r itself points at. Every
// slice is cloned, not just Repos, so the invariant holds regardless of
// which field a future caller mutates in place.
func cloneRegistry(r Registry) Registry {
	out := r
	out.Departments = append([]Department(nil), r.Departments...)
	out.Teams = append([]Team(nil), r.Teams...)
	out.Repos = append([]Repo(nil), r.Repos...)
	return out
}

// Registry returns a copy that shares no backing array with the Store's
// internal state (cloneRegistry): a caller mutating a Repo field on the
// result (the register affordance's `reg.Repos[i].Remote = ...` pattern)
// can never race a concurrent reader, and can never make that mutation
// visible before it has actually been persisted via SaveRegistry.
func (s *Store) Registry() Registry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneRegistry(s.reg)
}

// SaveRegistry persists r via the package-level atomic SaveRegistry and
// updates the in-memory copy Registry() serves, so a change (e.g. the
// unregistered-remote register affordance binding a Remote to a Repo) is
// visible to the very next request without a re-Open. Guarded by mu: unlike
// AppendEvent, which only ever appends, this is the first path that mutates
// s.reg after Open, and concurrent HTTP handlers read Registry() while it
// runs. r is cloned (cloneRegistry) before the in-memory swap: persist
// happens first against the caller's own value, then s.reg is set from an
// independent copy, so nothing the caller does to r afterward can reach
// internal state, and a failed persist leaves s.reg's backing arrays
// completely untouched.
func (s *Store) SaveRegistry(r Registry) error {
	if err := SaveRegistry(s.dir, r); err != nil {
		return err
	}
	r = cloneRegistry(r)
	s.mu.Lock()
	s.reg = r
	s.mu.Unlock()
	return nil
}

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
