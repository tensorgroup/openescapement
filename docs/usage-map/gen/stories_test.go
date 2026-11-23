package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validStories = `{
 "why": {"lead": "x", "cards": [{"id": "c1", "title": "t", "text": "x", "stories": ["s1"]}]},
 "org": {"name": "Windup Workshop", "teams": [{"id": "t1", "name": "Team"}],
  "people": [{"id": "p1", "name": "P", "team": "t1", "role": "r", "personas": ["you"], "stories": ["s1"]}]},
 "places": [{"id": "a", "title": "A", "kind": "machine", "owner": "p1", "node": "you"},
  {"id": "b", "title": "B", "kind": "repo", "owner": "t1"}],
 "stories": [{"id": "s1", "person": "p1", "title": "S", "why": "w",
  "steps": [{"place": "a", "label": "L", "text": "x", "node": "cmd:sync", "truth": "b",
   "moves": [{"from": "a", "to": "b", "what": "w", "kind": "share"}]}]}]
}`

var testIDs = map[string]bool{"root": true, "you": true, "cmd:sync": true}

func storiesFixture(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(validStories), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func firstStep(m map[string]any) map[string]any {
	return m["stories"].([]any)[0].(map[string]any)["steps"].([]any)[0].(map[string]any)
}

func TestStoriesLoadJSJSON(t *testing.T) {
	got, err := loadJSJSON("// c\nwindow.X = {\"a\": 1};\n", "X")
	if err != nil || got["a"] != float64(1) {
		t.Fatalf("got %v, %v", got, err)
	}
	if _, err := loadJSJSON("window.Y = {};", "X"); err == nil {
		t.Fatal("want an error for a missing assignment")
	}
}

func TestStoriesMapIDs(t *testing.T) {
	gen := map[string]any{"t": map[string]any{"groups": []any{map[string]any{"id": "g", "items": []any{map[string]any{"id": "cmd:sync"}}}}}}
	ids := mapIDs(gen, "{ id: 'you', x: 1 }")
	for _, want := range []string{"g", "cmd:sync", "you", "root"} {
		if !ids[want] {
			t.Errorf("missing %q", want)
		}
	}
}

func TestStoriesInfraMarks(t *testing.T) {
	personas, planned := infraMarks("{ id: 'you', kind: 'persona' },\n{ id: 'x', status: 'planned' }")
	if !personas["you"] || len(personas) != 1 || !planned["x"] || len(planned) != 1 {
		t.Fatalf("personas %v planned %v", personas, planned)
	}
}

func TestStoriesCheckValid(t *testing.T) {
	if errs := checkStories(storiesFixture(t), testIDs, nil, nil); len(errs) != 0 {
		t.Fatal(errs)
	}
}

func TestStoriesCheckErrors(t *testing.T) {
	cases := []struct {
		name, needle string
		edit         func(m map[string]any)
	}{
		{"unknown place", "nowhere", func(m map[string]any) { firstStep(m)["place"] = "nowhere" }},
		{"unknown move end", "zz", func(m map[string]any) {
			firstStep(m)["moves"].([]any)[0].(map[string]any)["to"] = "zz"
		}},
		{"unknown move kind", "teleport", func(m map[string]any) {
			firstStep(m)["moves"].([]any)[0].(map[string]any)["kind"] = "teleport"
		}},
		{"unknown node", "cmd:gone", func(m map[string]any) { firstStep(m)["node"] = "cmd:gone" }},
		{"unknown truth", "qq", func(m map[string]any) { firstStep(m)["truth"] = "qq" }},
		{"step needs label", "label", func(m map[string]any) { delete(firstStep(m), "label") }},
		{"unknown status", "planed", func(m map[string]any) { firstStep(m)["status"] = "planed" }},
		{"unknown place node", "gone", func(m map[string]any) { m["places"].([]any)[0].(map[string]any)["node"] = "gone" }},
		{"unknown place kind", "cloud", func(m map[string]any) { m["places"].([]any)[0].(map[string]any)["kind"] = "cloud" }},
		{"unknown owner", "o9", func(m map[string]any) { m["places"].([]any)[1].(map[string]any)["owner"] = "o9" }},
		{"unknown person", "ghost", func(m map[string]any) { m["stories"].([]any)[0].(map[string]any)["person"] = "ghost" }},
		{"person team", "t9", func(m map[string]any) {
			m["org"].(map[string]any)["people"].([]any)[0].(map[string]any)["team"] = "t9"
		}},
		{"person story", "s9", func(m map[string]any) {
			m["org"].(map[string]any)["people"].([]any)[0].(map[string]any)["stories"] = []any{"s1", "s9"}
		}},
		{"story not listed", "does not list it", func(m map[string]any) {
			m["org"].(map[string]any)["people"].([]any)[0].(map[string]any)["stories"] = []any{}
		}},
		{"moves not a list", "must be a list", func(m map[string]any) { firstStep(m)["moves"] = map[string]any{} }},
		{"null step", "must be an object", func(m map[string]any) {
			st := m["stories"].([]any)[0].(map[string]any)
			st["steps"] = append(st["steps"].([]any), nil)
		}},
		{"person needs name", "needs a name", func(m map[string]any) {
			delete(m["org"].(map[string]any)["people"].([]any)[0].(map[string]any), "name")
		}},
		{"story needs title", "needs a title", func(m map[string]any) { delete(m["stories"].([]any)[0].(map[string]any), "title") }},
		{"place needs title", "needs a title", func(m map[string]any) { delete(m["places"].([]any)[0].(map[string]any), "title") }},
		{"move needs what", "needs a what", func(m map[string]any) {
			delete(firstStep(m)["moves"].([]any)[0].(map[string]any), "what")
		}},
		{"duplicate place", "duplicate place", func(m map[string]any) {
			pl := m["places"].([]any)
			m["places"] = append(pl, pl[1])
		}},
		{"planned place", "place \"a\" is planned", func(m map[string]any) {
			m["places"].([]any)[0].(map[string]any)["status"] = "planned"
		}},
		{"null status", "unknown status", func(m map[string]any) { firstStep(m)["status"] = nil }},
		{"numeric node", "node must be a string", func(m map[string]any) { firstStep(m)["node"] = 123.0 }},
		{"card story", "s9", func(m map[string]any) {
			m["why"].(map[string]any)["cards"].([]any)[0].(map[string]any)["stories"] = []any{"s9"}
		}},
		{"no steps", "no steps", func(m map[string]any) { m["stories"].([]any)[0].(map[string]any)["steps"] = []any{} }},
		{"bad story id", "a/b", func(m map[string]any) {
			m["stories"].([]any)[0].(map[string]any)["id"] = "a/b"
			m["why"].(map[string]any)["cards"].([]any)[0].(map[string]any)["stories"] = []any{"a/b"}
			m["org"].(map[string]any)["people"].([]any)[0].(map[string]any)["stories"] = []any{"a/b"}
		}},
		{"duplicate story", "duplicate", func(m map[string]any) {
			s := m["stories"].([]any)
			m["stories"] = append(s, s[0])
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := storiesFixture(t)
			c.edit(m)
			errs := checkStories(m, testIDs, nil, nil)
			if len(errs) != 1 || !strings.Contains(errs[0], c.needle) {
				t.Fatalf("want one error with %q, got %v", c.needle, errs)
			}
		})
	}
}

func TestStoriesPlannedAndPersona(t *testing.T) {
	m := storiesFixture(t)
	if errs := checkStories(m, testIDs, nil, map[string]bool{"cmd:sync": true}); len(errs) != 1 || !strings.Contains(errs[0], "planned") {
		t.Fatalf("planned box: %v", errs)
	}
	firstStep(m)["status"] = "planned"
	if errs := checkStories(m, testIDs, nil, map[string]bool{"cmd:sync": true}); len(errs) != 0 {
		t.Fatalf("planned step: %v", errs)
	}
	if errs := checkStories(storiesFixture(t), testIDs, map[string]bool{"lead": true}, nil); len(errs) != 1 || !strings.Contains(errs[0], "you") {
		t.Fatalf("persona: %v", errs)
	}
}

func TestStoriesMalformed(t *testing.T) {
	m := storiesFixture(t)
	delete(m["stories"].([]any)[0].(map[string]any), "id")
	m["places"] = append(m["places"].([]any), "oops")
	errs := strings.Join(checkStories(m, testIDs, nil, nil), "\n")
	if !strings.Contains(errs, "needs a id") || !strings.Contains(errs, "places entry 3: must be an object") {
		t.Fatalf("want one message per bad entry, got:\n%s", errs)
	}
}

func TestStoriesRealPlannedBoxesAreSeen(t *testing.T) {
	// infraMarks reads id and status from one line; a reformat must not hide them.
	b, err := os.ReadFile(filepath.Join("..", "infra.js"))
	if err != nil {
		t.Fatal(err)
	}
	_, planned := infraMarks(string(b))
	for _, id := range []string{"pl-fleet", "pl-mcp", "pl-registry", "pl-siem", "pl-review", "d-brew"} {
		if !planned[id] {
			t.Errorf("planned box %q not seen", id)
		}
	}
}

func TestStoriesRealDataValid(t *testing.T) {
	dir := filepath.Join("..")
	if errs, err := checkStoriesDir(dir); err != nil || len(errs) != 0 {
		t.Fatalf("err %v, findings %v", err, errs)
	}
	if _, err := os.Stat(filepath.Join(dir, "stories-data.js")); err != nil {
		t.Fatal(err)
	}
}
