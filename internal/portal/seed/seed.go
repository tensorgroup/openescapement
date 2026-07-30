// Package seed generates a deterministic, fictional Caltech-shaped org with
// 60 days of governance and usage history, for `esc serve --demo`.
package seed

import (
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/tensorgroup/openescapement/internal/portal/store"
)

// costPerMtok is blended $ per 1M tokens (demo figures).
var costPerMtok = map[string]float64{
	"claude-sonnet-5": 6, "claude-opus-5": 30, "claude-haiku-4-5": 2,
	"gpt-5.2": 8, "gemini-3-pro": 5,
}

// modelOrder and modelWeight are parallel slices (fixed order — never range
// over costPerMtok when emitting, map iteration order is nondeterministic).
var modelOrder = []string{"claude-sonnet-5", "claude-opus-5", "claude-haiku-4-5", "gpt-5.2", "gemini-3-pro"}
var modelWeight = []float64{0.45, 0.10, 0.20, 0.15, 0.10}

var agentTools = []string{"claude-code", "cursor", "gemini-cli"}
var agentToolWeight = []float64{0.70, 0.20, 0.10}

type deptDef struct{ id, name string }

var deptDefs = []deptDef{
	{"physics", "Physics"},
	{"biology", "Biology"},
	{"cs", "Computer Science"},
	{"astro", "Astronomy"},
	{"chem", "Chemistry"},
	{"it", "Central IT"},
}

type teamDef struct {
	id, name, deptID string
	repos            []string
}

// 15 teams, 2-3 per department, 40 repos total (flattened in this order).
var teamDefs = []teamDef{
	{"physics-instr", "Physics Instrumentation", "physics", []string{
		"daq-controller", "sensor-calibration", "beamline-monitor"}},
	{"ligo-ops", "LIGO Ops", "physics", []string{
		"ligo-analysis-pipeline", "ligo-detector-control", "ligo-data-archive"}},
	{"physics-hpc", "Physics HPC", "physics", []string{
		"hpc-job-scheduler", "hpc-storage-tools"}},
	{"genomics", "Genomics Pipeline", "biology", []string{
		"variant-caller", "genome-assembler", "sequencing-qc"}},
	{"bio-data", "Biology Data Systems", "biology", []string{
		"lab-data-portal", "sample-tracker"}},
	{"cs-research", "CS Research Platform", "cs", []string{
		"ml-training-cluster", "research-notebook-hub", "dataset-registry"}},
	{"campus-web", "Campus Web", "cs", []string{
		"campus-web-frontend", "campus-web-api", "campus-cms"}},
	{"cs-infra", "CS Infra", "cs", []string{
		"ci-runner-fleet", "internal-devtools"}},
	{"astro-reduction", "Astronomy Data Reduction", "astro", []string{
		"image-pipeline", "photometry-tools", "spectra-reduction"}},
	{"telescope-ops", "Telescope Ops", "astro", []string{
		"telescope-scheduler", "dome-control"}},
	{"chem-compute", "Chemistry Compute", "chem", []string{
		"molecular-sim", "reaction-predictor", "chem-data-lake"}},
	{"chem-lab", "Chem Lab Systems", "chem", []string{
		"lims-connector", "inventory-tracker"}},
	{"it-helpdesk", "IT Helpdesk Tools", "it", []string{
		"ticketing-system", "asset-inventory", "kiosk-imaging"}},
	{"it-security", "IT Security", "it", []string{
		"siem-rules", "phishing-sim", "vuln-scanner"}},
	{"it-platform", "IT Platform Services", "it", []string{
		"sso-gateway", "campus-api-gateway", "config-service"}},
}

// governedCount is the number of leading flattened repos (by index) that
// carry posture events. Drifted/stale indices are exact per the spec.
const governedCount = 28

var drifted = map[int]bool{3: true, 9: true, 17: true, 25: true}
var stale = map[int]bool{5: true, 12: true, 21: true}

// teamPack gives 6 governed, otherwise in-sync repos (by flattened index) an
// extra team-specific pack alongside org-baseline.
var teamPack = map[int]store.EventPack{
	0:  {Name: "physics-instr", Version: "0.4.0", Signed: true},
	1:  {Name: "physics-instr", Version: "0.4.0", Signed: true},
	6:  {Name: "physics-hpc", Version: "0.3.0", Signed: true},
	7:  {Name: "physics-hpc", Version: "0.3.0", Signed: true},
	13: {Name: "cs-research", Version: "0.2.0", Signed: true},
	14: {Name: "cs-research", Version: "0.2.0", Signed: true},
}

// ungovernedWithUsage marks ungoverned repos (index >= governedCount) that
// still see AI activity despite carrying no posture — "AI activity with
// zero governance." 6 of the 12 ungoverned repos, per spec.
var ungovernedWithUsage = map[int]bool{28: true, 30: true, 32: true, 34: true, 36: true, 38: true}

// Demo writes registry.json and events.jsonl under dataDir.
// Deterministic: same epoch → byte-identical files. epoch is "now";
// history spans the 60 days before it. Idempotent: wipes the two files first.
func Demo(dataDir string, epoch time.Time) error {
	for _, f := range []string{"registry.json", "events.jsonl"} {
		if err := os.RemoveAll(filepath.Join(dataDir, f)); err != nil {
			return err
		}
	}
	rng := rand.New(rand.NewSource(1849))
	reg, repos := buildRegistry()
	if err := store.SaveRegistry(dataDir, reg); err != nil {
		return err
	}
	events := buildEvents(rng, reg, repos, epoch)
	sort.SliceStable(events, func(i, j int) bool { return events[i].TS.Before(events[j].TS) })
	s, err := store.Open(dataDir)
	if err != nil {
		return err
	}
	for _, e := range events {
		if err := s.AppendEvent(e); err != nil {
			return err
		}
	}
	return nil
}

// repoInfo carries the flattened index alongside the registry repo, since
// drift/stale/team-pack/shadow-usage membership is keyed by that index.
type repoInfo struct {
	idx    int
	id     string
	teamID string
}

func buildRegistry() (store.Registry, []repoInfo) {
	var reg store.Registry
	reg.Org = store.Org{Name: "Caltech Institute of Technology (demo)"}
	for _, d := range deptDefs {
		reg.Departments = append(reg.Departments, store.Department{ID: d.id, Name: d.name})
	}
	var repos []repoInfo
	idx := 0
	for _, t := range teamDefs {
		reg.Teams = append(reg.Teams, store.Team{ID: t.id, Name: t.name, DeptID: t.deptID})
		for _, rname := range t.repos {
			id := t.id + "-" + rname
			governed := idx < governedCount
			reg.Repos = append(reg.Repos, store.Repo{ID: id, Name: rname, TeamID: t.id, Governed: governed})
			repos = append(repos, repoInfo{idx: idx, id: id, teamID: t.id})
			idx++
		}
	}
	return reg, repos
}

func buildEvents(rng *rand.Rand, reg store.Registry, repos []repoInfo, epoch time.Time) []store.Event {
	var events []store.Event

	teamSize := map[string]int{}
	for _, ri := range repos {
		teamSize[ri.teamID]++
	}

	for _, ri := range repos {
		switch {
		case ri.idx < governedCount:
			events = append(events, governedRepoEvents(rng, ri, epoch)...)
		case ungovernedWithUsage[ri.idx]:
			events = append(events, shadowRepoEvents(rng, ri, epoch)...)
		}
	}

	events = append(events, usageEvents(rng, reg.Teams, teamSize, repos, epoch)...)

	return events
}

// weightedChoice picks options[i] via cumulative weights (fixed slice order
// — deterministic given rng's state).
func weightedChoice(rng *rand.Rand, options []string, weights []float64) string {
	r := rng.Float64()
	var cum float64
	for i, w := range weights {
		cum += w
		if r < cum {
			return options[i]
		}
	}
	return options[len(options)-1]
}

// governedRepoEvents emits a repo's onboarding sync, periodic update checks,
// a recent posture status (carrying the repo's final drift), and one
// mcp_connect. First-sync dates stagger across the 60-day window so the
// adoption curve grows over time.
func governedRepoEvents(rng *rand.Rand, ri repoInfo, epoch time.Time) []store.Event {
	var out []store.Event

	firstDaysAgo := 10 + rng.Intn(45) // 10..54 days ago
	finalDaysAgo := rng.Intn(3)       // 0..2 days ago: recent status check

	t0 := epoch.AddDate(0, 0, -firstDaysAgo)
	out = append(out, store.Event{
		TS:        t0,
		Kind:      "sync",
		RepoID:    ri.id,
		TeamID:    ri.teamID,
		AgentTool: weightedChoice(rng, agentTools, agentToolWeight),
		Packs:     []store.EventPack{{Name: "org-baseline", Version: "1.2.0", Signed: true}},
		Drift:     "in-sync",
	})

	for d := firstDaysAgo - 7; d > finalDaysAgo+1; d -= 7 {
		out = append(out, store.Event{
			TS:     epoch.AddDate(0, 0, -d),
			Kind:   "update_check",
			RepoID: ri.id,
			TeamID: ri.teamID,
		})
	}

	drift := "in-sync"
	switch {
	case drifted[ri.idx]:
		drift = "drifted"
	case stale[ri.idx]:
		drift = "stale"
	}
	packVersion := "1.2.0"
	if drift == "stale" {
		packVersion = "1.1.0"
	}
	packs := []store.EventPack{{Name: "org-baseline", Version: packVersion, Signed: true}}
	if extra, ok := teamPack[ri.idx]; ok {
		packs = append(packs, extra)
	}
	out = append(out, store.Event{
		TS:        epoch.AddDate(0, 0, -finalDaysAgo),
		Kind:      "status",
		RepoID:    ri.id,
		TeamID:    ri.teamID,
		AgentTool: weightedChoice(rng, agentTools, agentToolWeight),
		Packs:     packs,
		Drift:     drift,
	})

	out = append(out, store.Event{
		TS:        epoch.AddDate(0, 0, -finalDaysAgo).Add(-2 * time.Hour),
		Kind:      "mcp_connect",
		RepoID:    ri.id,
		TeamID:    ri.teamID,
		AgentTool: weightedChoice(rng, agentTools, agentToolWeight),
		Model:     weightedChoice(rng, modelOrder, modelWeight),
	})

	return out
}

// shadowRepoEvents emits a handful of mcp_connect events for an ungoverned
// repo — AI tools are in use, but the repo carries no governance pack.
func shadowRepoEvents(rng *rand.Rand, ri repoInfo, epoch time.Time) []store.Event {
	var out []store.Event
	for d := 55; d >= 3; d -= 14 {
		out = append(out, store.Event{
			TS:        epoch.AddDate(0, 0, -d).Add(time.Duration(10+rng.Intn(6)) * time.Hour),
			Kind:      "mcp_connect",
			RepoID:    ri.id,
			TeamID:    ri.teamID,
			AgentTool: weightedChoice(rng, agentTools, agentToolWeight),
			Model:     weightedChoice(rng, modelOrder, modelWeight),
		})
	}
	return out
}

// usageEvents emits provider_usage events per (team, model, day) for the 60
// days before epoch: weighted by team size, with a weekly rhythm (weekends
// ~25%) and mild growth toward the present. The dominant model additionally
// carries RepoID for teams with a shadow (ungoverned-but-used) repo.
func usageEvents(rng *rand.Rand, teams []store.Team, teamSize map[string]int, repos []repoInfo, epoch time.Time) []store.Event {
	teamShadowRepo := map[string]string{}
	for _, ri := range repos {
		if ungovernedWithUsage[ri.idx] {
			if _, ok := teamShadowRepo[ri.teamID]; !ok {
				teamShadowRepo[ri.teamID] = ri.id
			}
		}
	}

	const totalDailyTokens = 5_000_000.0
	var events []store.Event
	for dayIdx := 0; dayIdx < 60; dayIdx++ {
		daysAgo := 59 - dayIdx
		day := epoch.AddDate(0, 0, -daysAgo)
		growth := 0.85 + 0.30*float64(dayIdx)/59.0
		weekday := 1.0
		if wd := day.Weekday(); wd == time.Saturday || wd == time.Sunday {
			weekday = 0.25
		}
		for _, t := range teams {
			teamShare := float64(teamSize[t.ID]) / 40.0
			for mi, model := range modelOrder {
				jitter := 0.85 + 0.30*rng.Float64()
				tokens := totalDailyTokens * growth * weekday * teamShare * modelWeight[mi] * jitter
				if tokens < 1 {
					continue
				}
				input := int64(tokens * 0.7)
				output := int64(tokens) - input
				cost := round2(tokens / 1_000_000.0 * costPerMtok[model])
				e := store.Event{
					TS:     day.Add(time.Duration(9+rng.Intn(8)) * time.Hour),
					Kind:   "provider_usage",
					TeamID: t.ID,
					Model:  model,
					Tokens: &store.Tokens{Input: input, Output: output, CostUSD: cost},
				}
				if model == "claude-sonnet-5" {
					if repoID, ok := teamShadowRepo[t.ID]; ok {
						e.RepoID = repoID
					}
				}
				events = append(events, e)
			}
		}
	}
	return events
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}
