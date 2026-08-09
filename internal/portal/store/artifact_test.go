package store

import (
	"encoding/json"
	"testing"
)

func TestDeriveDrift(t *testing.T) {
	cases := []struct {
		name string
		arts []EventArtifact
		want string
	}{
		{"no artifacts", nil, "in-sync"},
		{"all in-sync", []EventArtifact{{Managed: "in-sync"}, {Managed: "in-sync"}}, "in-sync"},
		{"one altered", []EventArtifact{{Managed: "in-sync"}, {Managed: "altered"}}, "drifted"},
		{"one stale", []EventArtifact{{Managed: "stale"}}, "drifted"},
		{"one missing", []EventArtifact{{Managed: "missing"}}, "drifted"},
		{"one orphan", []EventArtifact{{Managed: "orphan"}}, "drifted"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DeriveDrift(c.arts); got != c.want {
				t.Fatalf("DeriveDrift(%+v) = %q, want %q", c.arts, got, c.want)
			}
		})
	}
}

func TestFleetState(t *testing.T) {
	cases := []struct {
		name string
		arts []EventArtifact
		want string
	}{
		{"no artifacts", nil, "unadulterated"},
		{"all in-sync no local", []EventArtifact{{Managed: "in-sync", Local: "none"}}, "unadulterated"},
		{"local amendment only", []EventArtifact{{Managed: "in-sync", Local: "amended"}}, "augmented"},
		{"altered only", []EventArtifact{{Managed: "altered", Local: "none"}}, "altered"},
		{
			"both axes on the same repo: altered wins the state",
			[]EventArtifact{
				{Managed: "in-sync", Local: "amended"},
				{Managed: "altered", Local: "none"},
			},
			"altered",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FleetState(c.arts); got != c.want {
				t.Fatalf("FleetState(%+v) = %q, want %q", c.arts, got, c.want)
			}
		})
	}
}

// TestOldShapeEventDecodes locks in backward compatibility: a JSONL line
// written by today's seed/ingest shape (no artifacts, no collection) must
// still decode once Event gains those fields, leaving Artifacts and
// Collection nil and Drift untouched. Field values and order mirror the
// "status" event seed.go's governedRepoEvents emits.
func TestOldShapeEventDecodes(t *testing.T) {
	line := `{"ts":"2026-07-01T12:00:00Z","kind":"status","repo_id":"r1","team_id":"ligo","agent_tool":"claude-code","packs":[{"name":"org-baseline","version":"1.2.0","signed":true}],"drift":"drifted"}`

	var e Event
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		t.Fatalf("unmarshal old-shape line: %v", err)
	}
	if e.Artifacts != nil {
		t.Fatalf("Artifacts = %+v, want nil", e.Artifacts)
	}
	if e.Collection != nil {
		t.Fatalf("Collection = %+v, want nil", e.Collection)
	}
	if e.Drift != "drifted" {
		t.Fatalf("Drift = %q, want %q (untouched)", e.Drift, "drifted")
	}
	if e.Kind != "status" || e.RepoID != "r1" || e.TeamID != "ligo" || e.AgentTool != "claude-code" {
		t.Fatalf("event mismatch: %+v", e)
	}
	if len(e.Packs) != 1 || e.Packs[0].Name != "org-baseline" || e.Packs[0].Version != "1.2.0" {
		t.Fatalf("packs mismatch: %+v", e.Packs)
	}

	// Rolls up identically to before: FleetRows still reads e.Drift directly.
	reg := Registry{Repos: []Repo{{ID: "r1", Name: "ligo-pipeline", TeamID: "ligo"}}}
	rows := FleetRows(reg, []Event{e})
	if len(rows) != 1 || rows[0].Status != "drifted" {
		t.Fatalf("FleetRows on old-shape event: %+v", rows)
	}
}
