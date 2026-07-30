package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func splitLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

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
