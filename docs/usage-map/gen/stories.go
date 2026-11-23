package main

// The story check: every place, person, team, story and map box that a hand-written story
// (docs/usage-map/stories-data.js) names must exist, and a step on a planned box must be
// planned too. So the Who tab never draws an arrow to nowhere, a "Show in architecture"
// link to a missing box, or a planned feature as built. main runs it after it writes the data.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	moveKinds  = map[string]bool{"share": true, "store": true, "read": true, "sign": true}
	placeKinds = map[string]bool{"machine": true, "repo": true, "home": true, "service": true, "agent": true}
	storyIDRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	infraIDRe  = regexp.MustCompile(`\bid:\s*'([^']+)'`)
	personaRe  = regexp.MustCompile(`\bkind:\s*'persona'`)
	plannedRe  = regexp.MustCompile(`\bstatus:\s*'planned'`)
)

// loadJSJSON returns the strict-JSON object assigned as `window.<name> = {...};`.
func loadJSJSON(text, name string) (map[string]any, error) {
	var keep []string
	for _, l := range strings.Split(text, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(l), "//") {
			keep = append(keep, l)
		}
	}
	body := strings.Join(keep, "\n")
	loc := regexp.MustCompile(`window\.` + regexp.QuoteMeta(name) + `\s*=\s*`).FindStringIndex(body)
	if loc == nil {
		return nil, fmt.Errorf("no window.%s assignment", name)
	}
	body = strings.TrimSuffix(strings.TrimSpace(body[loc[1]:]), ";")
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		return nil, fmt.Errorf("window.%s is not strict JSON: %w", name, err)
	}
	return m, nil
}

// mapIDs returns every box id on the map: generated ids, hand-written ids in infra.js, root.
func mapIDs(generated map[string]any, infra string) map[string]bool {
	ids := map[string]bool{"root": true}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if id, ok := x["id"].(string); ok {
				ids[id] = true
			}
			for _, c := range x {
				walk(c)
			}
		case []any:
			for _, c := range x {
				walk(c)
			}
		}
	}
	walk(generated)
	for _, m := range infraIDRe.FindAllStringSubmatch(infra, -1) {
		ids[m[1]] = true
	}
	return ids
}

// infraMarks returns the persona ids and planned ids of the hand-written map: lines that set
// an id together with kind: 'persona' or status: 'planned'.
func infraMarks(infra string) (personas, planned map[string]bool) {
	personas, planned = map[string]bool{}, map[string]bool{}
	for _, line := range strings.Split(infra, "\n") {
		m := infraIDRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if personaRe.MatchString(line) {
			personas[m[1]] = true
		}
		if plannedRe.MatchString(line) {
			planned[m[1]] = true
		}
	}
	return personas, planned
}

func str(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

// checkStories returns one message per broken reference, missing field or unmarked claim.
// Each bad entry gets its own message; one bad entry never hides the rest. It matches
// check_stories.py in the Balancewheel repo rule for rule. personas nil means any map id may
// be a person's persona; planned holds the map's planned box ids.
func checkStories(s map[string]any, ids, personas, planned map[string]bool) []string {
	var errs []string
	add := func(format string, a ...any) { errs = append(errs, fmt.Sprintf(format, a...)) }
	objs := func(v any, where string) []map[string]any {
		if v == nil {
			return nil
		}
		list, ok := v.([]any)
		if !ok {
			add("%s: must be a list", where)
			return nil
		}
		var out []map[string]any
		for n, x := range list {
			if m, ok := x.(map[string]any); ok {
				out = append(out, m)
			} else {
				add("%s entry %d: must be an object", where, n+1)
			}
		}
		return out
	}
	strList := func(v any, where string) []string {
		if v == nil {
			return nil
		}
		list, ok := v.([]any)
		if !ok {
			add("%s: must be a list", where)
			return nil
		}
		var out []string
		for _, x := range list {
			if v, ok := x.(string); ok {
				out = append(out, v)
			} else {
				add("%s: %v is not a string", where, x)
			}
		}
		return out
	}
	need := func(m map[string]any, where string, fields ...string) {
		for _, f := range fields {
			if str(m, f) == "" {
				add("%s: needs a %s", where, f)
			}
		}
	}
	// optional string fields must be strings when present
	optStr := func(m map[string]any, where string, fields ...string) {
		for _, f := range fields {
			if v, ok := m[f]; ok {
				if _, isStr := v.(string); !isStr {
					add("%s: %s must be a string", where, f)
				}
			}
		}
	}
	idsOf := func(list []map[string]any, kind string) map[string]bool {
		set, seen := map[string]bool{}, map[string]int{}
		for _, m := range list {
			id := str(m, "id")
			if id == "" {
				add("%s ?: needs a id", kind)
				continue
			}
			set[id] = true
			seen[id]++
		}
		var dups []string
		for id, n := range seen {
			if n > 1 {
				dups = append(dups, id)
			}
		}
		sort.Strings(dups)
		for _, id := range dups {
			add("%s %s: duplicate %s id", kind, id, kind)
		}
		return set
	}
	statusOK := func(m map[string]any, where string) {
		if v, ok := m["status"]; ok && v != "planned" && v != "" {
			add("%s: unknown status %v", where, v)
		}
	}

	org, _ := s["org"].(map[string]any)
	why, _ := s["why"].(map[string]any)
	teamList := objs(org["teams"], "org.teams")
	peopleList := objs(org["people"], "org.people")
	placeList := objs(s["places"], "places")
	storyList := objs(s["stories"], "stories")
	teams := idsOf(teamList, "team")
	people := idsOf(peopleList, "person")
	places := idsOf(placeList, "place")
	known := idsOf(storyList, "story")
	plannedPlaces := map[string]bool{}
	for _, p := range placeList {
		if str(p, "status") == "planned" {
			plannedPlaces[str(p, "id")] = true
		}
	}
	listed := map[[2]string]bool{}

	for _, t := range teamList {
		need(t, "team "+str(t, "id"), "name")
	}
	for _, p := range peopleList {
		who := "person " + str(p, "id")
		need(p, who, "name")
		if !teams[str(p, "team")] {
			add("%s: unknown team %q", who, str(p, "team"))
		}
		for _, pid := range strList(p["personas"], who+" personas") {
			if (personas == nil && !ids[pid]) || (personas != nil && !personas[pid]) {
				add("%s: unknown map persona %q", who, pid)
			}
		}
		for _, sid := range strList(p["stories"], who+" stories") {
			listed[[2]string{str(p, "id"), sid}] = true
			if !known[sid] {
				add("%s: unknown story %q", who, sid)
			}
		}
	}
	for _, c := range objs(why["cards"], "why.cards") {
		where := "why card " + str(c, "id")
		need(c, where, "id", "title", "text")
		for _, sid := range strList(c["stories"], where+" stories") {
			if !known[sid] {
				add("%s: unknown story %q", where, sid)
			}
		}
	}
	for _, p := range placeList {
		where := "place " + str(p, "id")
		need(p, where, "title")
		optStr(p, where, "owner", "node")
		if !placeKinds[str(p, "kind")] {
			add("%s: unknown kind %q", where, str(p, "kind"))
		}
		statusOK(p, where)
		if o := str(p, "owner"); o != "" && !teams[o] && !people[o] {
			add("%s: unknown owner %q", where, o)
		}
		if n := str(p, "node"); n != "" && !ids[n] {
			add("%s: unknown map box %q", where, n)
		}
	}
	for _, st := range storyList {
		sid := str(st, "id")
		need(st, "story "+sid, "title")
		if sid != "" && !storyIDRe.MatchString(sid) {
			add("story %s: id must be lowercase letters, digits and dashes", sid)
		}
		if !people[str(st, "person")] {
			add("story %s: unknown person %q", sid, str(st, "person"))
		} else if !listed[[2]string{str(st, "person"), sid}] {
			add("story %s: its person %q does not list it", sid, str(st, "person"))
		}
		statusOK(st, "story "+sid)
		steps := objs(st["steps"], "story "+sid+" steps")
		if len(steps) == 0 {
			add("story %s: has no steps", sid)
		}
		storyPlanned := str(st, "status") == "planned"
		for i, step := range steps {
			where := fmt.Sprintf("story %s step %d", sid, i+1)
			need(step, where, "label", "text")
			optStr(step, where, "node", "truth", "command")
			statusOK(step, where)
			isPlanned := storyPlanned || str(step, "status") == "planned"
			if !isPlanned && planned[str(step, "node")] {
				add("%s: map box %q is planned, so the step must be planned", where, str(step, "node"))
			}
			if !isPlanned && plannedPlaces[str(step, "place")] {
				add("%s: place %q is planned, so the step must be planned", where, str(step, "place"))
			}
			if !places[str(step, "place")] {
				add("%s: unknown place %q", where, str(step, "place"))
			}
			if t := str(step, "truth"); t != "" && !places[t] {
				add("%s: unknown truth place %q", where, t)
			}
			if n := str(step, "node"); n != "" && !ids[n] {
				add("%s: unknown map box %q", where, n)
			}
			for _, mv := range objs(step["moves"], where+" moves") {
				need(mv, where+" move", "what")
				for _, end := range []string{"from", "to"} {
					if !places[str(mv, end)] {
						add("%s: move %s unknown place %q", where, end, str(mv, end))
					}
				}
				if !moveKinds[str(mv, "kind")] {
					add("%s: unknown move kind %q", where, str(mv, "kind"))
				}
			}
		}
	}
	return errs
}

// checkStoriesDir loads stories-data.js, data.generated.js and infra.js from dir and checks.
func checkStoriesDir(dir string) ([]string, error) {
	read := func(name string) (string, error) {
		b, err := os.ReadFile(filepath.Join(dir, name))
		return string(b), err
	}
	storiesText, err := read("stories-data.js")
	if err != nil {
		return nil, err
	}
	genText, err := read("data.generated.js")
	if err != nil {
		return nil, err
	}
	infra, err := read("infra.js")
	if err != nil {
		return nil, err
	}
	stories, err := loadJSJSON(storiesText, "USAGE_MAP_STORIES")
	if err != nil {
		return nil, err
	}
	gen, err := loadJSJSON(genText, "USAGE_MAP_DATA")
	if err != nil {
		return nil, err
	}
	personas, planned := infraMarks(infra)
	return checkStories(stories, mapIDs(gen, infra), personas, planned), nil
}
