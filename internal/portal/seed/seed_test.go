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
	// DeriveDrift is three-valued (adjudicated ruling, telemetry-surface
	// plan Task 8 ledger): altered/missing/orphan Managed states win as
	// "drifted", but a stale-only artifact list derives "stale" — the
	// distinction survives derivation because rollup.FleetRow.Status
	// renders it directly. Of the 28 governed repos: alteredIdx (3) +
	// bothAxesIdx (1, Managed=altered) surface as drifted (4); staleIdx (3)
	// surface as stale; the remaining 21 are in-sync.
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

// latestStatusByRepo mirrors rollup's latestPosture selection (most recent
// sync|status event per repo) so tests can inspect a governed repo's final
// Artifacts/Collection without importing rollup's unexported helper.
func latestStatusByRepo(events []store.Event) map[string]store.Event {
	latest := map[string]store.Event{}
	for _, e := range events {
		if (e.Kind != "sync" && e.Kind != "status") || e.RepoID == "" {
			continue
		}
		if cur, ok := latest[e.RepoID]; !ok || e.TS.After(cur.TS) {
			latest[e.RepoID] = e
		}
	}
	return latest
}

func TestDemoGovernedReposCoverAllFleetStates(t *testing.T) {
	dir := t.TempDir()
	if err := Demo(dir, epoch); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	r := s.Registry()
	events, err := s.Events()
	if err != nil {
		t.Fatal(err)
	}
	latest := latestStatusByRepo(events)

	governed := map[string]bool{}
	for _, repo := range r.Repos {
		if repo.Governed {
			governed[repo.ID] = true
		}
	}
	if len(governed) != 28 {
		t.Fatalf("governed repos: %d, want 28", len(governed))
	}

	states := map[string]int{}
	var withheldRepo string
	for repoID := range governed {
		e, ok := latest[repoID]
		if !ok {
			t.Fatalf("governed repo %s has no posture event", repoID)
		}
		state := store.FleetState(e.Artifacts)
		states[state]++

		hasWithheldAmendment := false
		for _, a := range e.Artifacts {
			if a.Local == "amended" && a.Amendment != nil && a.Amendment.Content == "" {
				hasWithheldAmendment = true
			}
		}
		if hasWithheldAmendment && e.Collection != nil && e.Collection.Amendments == "metrics" && e.Collection.Source == "repo-override" {
			withheldRepo = repoID
		}
	}

	for _, want := range []string{"unadulterated", "augmented", "altered"} {
		if states[want] == 0 {
			t.Fatalf("no governed repo in state %q; distribution: %+v", want, states)
		}
	}
	if withheldRepo == "" {
		t.Fatal("no governed repo carries the withheld case (Collection metrics/repo-override with an amendment whose Content is absent)")
	}

	// Ungoverned repos stay event-less: no sync/status/mcp_connect/etc. keyed
	// to their RepoID at all (shadow IT is a distinct bucket, spec-tested
	// separately in repos_test.go / TestDemoShape's governed count).
	ungoverned := map[string]bool{}
	for _, repo := range r.Repos {
		if !repo.Governed {
			ungoverned[repo.ID] = true
		}
	}
	for _, e := range events {
		if e.RepoID != "" && ungoverned[e.RepoID] && (e.Kind == "sync" || e.Kind == "status") {
			t.Fatalf("ungoverned repo %s carries a %s event; ungoverned repos must stay event-less for posture", e.RepoID, e.Kind)
		}
	}
}
