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
