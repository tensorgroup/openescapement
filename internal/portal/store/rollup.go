package store

import (
	"fmt"
	"sort"
	"time"
)

// OverviewStats summarizes fleet posture and usage for the portal's landing page.
type OverviewStats struct {
	GovernedRepos, KnownRepos int
	DriftedRepos, StaleRepos  int
	TokensWeek                int64
	CostWeek                  float64
	Models                    []string        // sorted, seen in provider_usage/mcp_connect events
	Adoption                  []AdoptionPoint // one per day, last 60 days, cumulative governed repos
}

// AdoptionPoint is one day's cumulative governed-repo count.
type AdoptionPoint struct {
	Day      time.Time
	Governed int
}

// FleetRow is one repo's governance posture for the fleet table.
type FleetRow struct {
	DeptName, TeamName, RepoName, RepoID string
	Packs                                []string // "name@version", sorted
	LastSync                             time.Time
	Status                               string   // in-sync|drifted|stale|ungoverned
	Tools                                []string // sorted agent tools seen for repo
}

// UsageCell is one (day, model, team) aggregate of token usage and cost.
type UsageCell struct {
	Day    time.Time // truncated to UTC day
	Model  string
	TeamID string
	Tokens int64 // input+output
	Cost   float64
}

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
		if e.RepoID == "" {
			continue
		}
		if cur, ok := first[e.RepoID]; !ok || e.TS.Before(cur) {
			first[e.RepoID] = e.TS
		}
	}
	return first
}

// Overview computes fleet-wide posture and usage stats as of now.
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
		if (e.Kind == "provider_usage" || e.Kind == "mcp_connect") && e.Model != "" {
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

// FleetRows builds one row per repo, sorted by department, team, repo name.
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
			DeptName: deptName[team.DeptID],
			TeamName: team.Name,
			RepoName: repo.Name,
			RepoID:   repo.ID,
			Status:   "ungoverned",
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

// UsageDaily aggregates provider_usage events into per-day/model/team cells
// within [from, to], optionally filtered by team and/or model (empty = all).
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
