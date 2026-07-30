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
