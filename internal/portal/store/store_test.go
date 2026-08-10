package store

import (
	"fmt"
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

func TestStoreSaveRegistryPersistsAndUpdatesInMemory(t *testing.T) {
	dir := t.TempDir()
	r := Registry{Repos: []Repo{{ID: "r1", Name: "ligo-pipeline", TeamID: "ligo"}}}
	if err := SaveRegistry(dir, r); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	updated := s.Registry()
	updated.Repos[0].Remote = "github.com/acme/ligo-pipeline"
	if err := s.SaveRegistry(updated); err != nil {
		t.Fatal(err)
	}
	// In-memory copy reflects the change without a re-Open.
	if got := s.Registry().Repos[0].Remote; got != "github.com/acme/ligo-pipeline" {
		t.Fatalf("in-memory remote = %q", got)
	}
	// On-disk copy reflects it too, across a fresh Open.
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := s2.Registry().Repos[0].Remote; got != "github.com/acme/ligo-pipeline" {
		t.Fatalf("on-disk remote = %q", got)
	}
}

// TestStoreSaveRegistryFailureLeavesInMemoryUnchanged pins the other half of
// cloneRegistry's contract: a failed persist must never let the in-memory
// registry diverge from disk. Before cloning was added, the register
// affordance's `reg := s.Registry(); reg.Repos[i].Remote = x` mutated
// s.reg's own backing array immediately (Repos aliased it), so even a save
// that then failed left memory holding the mutation disk never got.
func TestStoreSaveRegistryFailureLeavesInMemoryUnchanged(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses the permission check this test relies on")
	}
	dir := t.TempDir()
	orig := Registry{Repos: []Repo{{ID: "r1", Name: "ligo-pipeline", Remote: "orig-remote"}}}
	if err := SaveRegistry(dir, orig); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Remove write permission on dir so the atomic write's rename fails.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	mutated := s.Registry()
	mutated.Repos[0].Remote = "new-remote"
	if err := s.SaveRegistry(mutated); err == nil {
		t.Fatal("expected save against a read-only directory to fail")
	}

	if got := s.Registry().Repos[0].Remote; got != "orig-remote" {
		t.Fatalf("in-memory registry diverged after a failed save: got %q, want %q", got, "orig-remote")
	}
}

// TestStoreRegistryConcurrentReadWriteIsRaceFree exercises the same pattern
// the register affordance uses (read, mutate the copy, save) from one
// goroutine while another goroutine reads in a tight loop. Before
// cloneRegistry, Registry()'s returned Repos slice aliased s.reg's backing
// array, so the writer's in-place mutation raced the reader with no lock
// covering either side. This test doesn't assert on values (the interleaving
// is nondeterministic); its purpose is to fail under `go test -race`.
func TestStoreRegistryConcurrentReadWriteIsRaceFree(t *testing.T) {
	dir := t.TempDir()
	r := Registry{Repos: []Repo{{ID: "r1", Name: "ligo-pipeline"}}}
	if err := SaveRegistry(dir, r); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	const iterations = 200
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < iterations; i++ {
			reg := s.Registry()
			reg.Repos[0].Remote = fmt.Sprintf("remote-%d", i)
			if err := s.SaveRegistry(reg); err != nil {
				t.Error(err)
			}
		}
	}()
	for {
		select {
		case <-done:
			return
		default:
			reg := s.Registry()
			_ = reg.Repos[0].Remote
		}
	}
}

func TestRegistryTeamAndDept(t *testing.T) {
	r := Registry{
		Departments: []Department{{ID: "phys", Name: "Physics"}},
		Teams:       []Team{{ID: "ligo", Name: "LIGO Ops", DeptID: "phys"}},
	}
	team, dept := r.TeamAndDept("ligo")
	if team.Name != "LIGO Ops" || dept != "Physics" {
		t.Fatalf("team=%+v dept=%q", team, dept)
	}
	if team, dept := r.TeamAndDept("nope"); team.Name != "" || dept != "" {
		t.Fatalf("unknown team should be zero: team=%+v dept=%q", team, dept)
	}
}

func TestRegistryUnassignedRepos(t *testing.T) {
	r := Registry{Repos: []Repo{
		{ID: "r2", Name: "b-repo", Remote: "github.com/acme/b"},
		{ID: "r1", Name: "a-repo"},
		{ID: "r3", Name: "c-repo"},
	}}
	got := r.UnassignedRepos()
	if len(got) != 2 || got[0].ID != "r1" || got[1].ID != "r3" {
		t.Fatalf("unassigned=%+v", got)
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
