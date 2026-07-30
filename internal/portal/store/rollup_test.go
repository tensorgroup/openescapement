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
