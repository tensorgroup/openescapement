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
	// campus-portal's fixture event carries no Artifacts: State must still
	// resolve (unadulterated), distinct from its stale Status — the two
	// axes never collapse into each other.
	if rows[0].State != "unadulterated" {
		t.Fatalf("row0 state=%+v", rows[0])
	}
	if rows[1].RepoName != "shadow-poc" || rows[1].Status != "ungoverned" || rows[1].State != "ungoverned" {
		t.Fatalf("row1=%+v", rows[1])
	}
	if rows[2].RepoName != "ligo-pipeline" || rows[2].Status != "drifted" ||
		rows[2].Packs[0] != "org-baseline@1.2.0" || rows[2].Tools[0] != "claude-code" {
		t.Fatalf("row2=%+v", rows[2])
	}
}

// TestFleetRowsStateFromArtifacts pins the cross-product AGENTS.md requires:
// a repo whose managed region is hand-edited is State=altered regardless of
// Status, and a repo with only a local amendment is State=augmented while
// Status stays in-sync (augmented is not a drift signal).
func TestFleetRowsStateFromArtifacts(t *testing.T) {
	reg := Registry{
		Repos: []Repo{{ID: "r1", Name: "altered-repo"}, {ID: "r2", Name: "augmented-repo"}},
	}
	events := []Event{
		{TS: day(1), Kind: "status", RepoID: "r1", Drift: "drifted",
			Artifacts: []EventArtifact{{Path: "CLAUDE.md", Managed: "altered", Local: "none"}}},
		{TS: day(1), Kind: "status", RepoID: "r2", Drift: "in-sync",
			Artifacts: []EventArtifact{{Path: "CLAUDE.md", Managed: "in-sync", Local: "amended"}}},
	}
	rows := FleetRows(reg, events)
	byID := map[string]FleetRow{}
	for _, r := range rows {
		byID[r.RepoID] = r
	}
	if byID["r1"].State != "altered" || byID["r1"].Status != "drifted" {
		t.Fatalf("r1=%+v", byID["r1"])
	}
	if byID["r2"].State != "augmented" || byID["r2"].Status != "in-sync" {
		t.Fatalf("r2=%+v", byID["r2"])
	}
}

func TestLatestPostureEvent(t *testing.T) {
	events := fixtureEvents()
	e, ok := LatestPostureEvent(events, "r1")
	if !ok || e.Drift != "drifted" || !e.TS.Equal(day(10)) {
		t.Fatalf("r1 latest=%+v ok=%v", e, ok)
	}
	if _, ok := LatestPostureEvent(events, "nope"); ok {
		t.Fatal("unknown repo should not be found")
	}
}

func TestUnregisteredRemotes(t *testing.T) {
	events := []Event{
		{TS: day(1), Kind: "mcp_connect", Remote: "github.com/acme/shadow-a", AgentTool: "cursor"},
		{TS: day(3), Kind: "status", Remote: "github.com/acme/shadow-a", AgentTool: "claude-code"},
		{TS: day(2), Kind: "mcp_connect", Remote: "github.com/acme/shadow-b", AgentTool: "cursor"},
		// Resolved (RepoID set): must never appear in the unregistered bucket.
		{TS: day(5), Kind: "status", RepoID: "r1", Remote: "github.com/acme/known"},
		// No remote at all (e.g. legacy raw-Event path): excluded too.
		{TS: day(4), Kind: "provider_usage", TeamID: "t1"},
	}
	got := UnregisteredRemotes(Registry{}, events)
	if len(got) != 2 {
		t.Fatalf("remotes=%+v", got)
	}
	// Most recently active first.
	if got[0].Remote != "github.com/acme/shadow-a" || got[0].Events != 2 || !got[0].LastSeen.Equal(day(3)) {
		t.Fatalf("remote0=%+v", got[0])
	}
	if len(got[0].AgentTools) != 2 || got[0].AgentTools[0] != "claude-code" || got[0].AgentTools[1] != "cursor" {
		t.Fatalf("remote0 tools=%+v", got[0].AgentTools)
	}
	if got[1].Remote != "github.com/acme/shadow-b" || got[1].Events != 1 {
		t.Fatalf("remote1=%+v", got[1])
	}
}

// TestUnregisteredRemotesExcludesRegistryBoundRemote pins the register-
// integrity fix: a remote bound to a repo via the register flow keeps its
// pre-bind events (RepoID is never rewritten retroactively), so relying on
// events alone would keep listing it as unregistered forever and would let
// the fleet-register handler bind the same remote to a second repo.
// UnregisteredRemotes must exclude any remote the registry already has
// bound, even though every event for it still carries an empty RepoID.
func TestUnregisteredRemotesExcludesRegistryBoundRemote(t *testing.T) {
	events := []Event{
		{TS: day(1), Kind: "mcp_connect", Remote: "github.com/acme/shadow-a", AgentTool: "cursor"},
		{TS: day(2), Kind: "mcp_connect", Remote: "github.com/acme/shadow-b", AgentTool: "cursor"},
	}
	reg := Registry{Repos: []Repo{{ID: "r1", Name: "bound-repo", Remote: "github.com/acme/shadow-a"}}}
	got := UnregisteredRemotes(reg, events)
	if len(got) != 1 || got[0].Remote != "github.com/acme/shadow-b" {
		t.Fatalf("remotes=%+v, want only shadow-b (shadow-a is bound in the registry)", got)
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
