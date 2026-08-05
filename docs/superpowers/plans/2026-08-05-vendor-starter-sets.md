# Vendor Starter Sets Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the portal adopt any selection of a vendor's model starters (plus the model-routing overview) as one composed pack fragment, and move absolute pricing out of the adoptable fragments into the vendor guidance pages.

**Architecture:** Selection changes the *composed body*, not the file set: a new `pack.ComposeFragments` merges N fragments (frontmatter targets unioned, bodies concatenated) into one file, so the existing single-file `publish.AddFragment` pipeline is untouched. The adopt handlers generalize from one `model` param to repeated `model` params plus a `routing` flag; the fixed-file-set security gate generalizes to set membership (registry-declared starters and the vendor's routing example only — the request never contributes a file path). The vendor page gains a checkbox panel.

**Tech Stack:** Go 1.24 stdlib + `gopkg.in/yaml.v3` (existing dep), `html/template`, `httptest`.

## Design decisions (captured per spec-driven convention)

1. **Compose, don't multi-publish.** A set adoption writes ONE fragment. No multi-fragment-single-tag publish machinery; `AddFragment` is unchanged.
2. **Gate generalizes to set membership.** Adoptable fragments are exactly: models' registry-declared `starter` paths, plus `examples/<vendor>/model-routing.md` built from the vendor key alone. Any unknown or starter-less requested id fails the whole request (404). This preserves the property that the portal never writes a path assembled from unchecked user input.
3. **Destination naming.** Exactly one starter and no routing → the historical `rules/model-<id>.md` (byte-identical content, full back-compat). Anything else → `rules/models-<vendor>.md`.
4. **Composition order.** Routing overview first, then starters in registry order.
5. **Pricing policy.** Absolute dollar figures live only in the vendor note pages (`anthropic.md`, `openai.md`, `google.md` — they already carry sourced prices). Fragments keep *relative* cost claims phrased durably ("materially more per token", "a fraction of the cost"); exact multiples ("double") are softened too. Enforced by a regression test over all example fragments.
6. **Known accepted edge.** Adopting a subset (e.g. Opus + Sonnet without Haiku) keeps routing lines that mention unselected models; that is an edit-after-adopt situation, not something the composer rewrites.
7. Seeded disk copies of edited embedded fragments refresh automatically via the existing create-or-refresh-unmodified seed semantics; no migration step needed.

## Global Constraints

- Single external dependency policy: `gopkg.in/yaml.v3` only (goldmark display-only exception already granted). This plan adds no dependency.
- Sentinel errors in `internal/esc` map to exit codes; new failure modes go through them (`esc.ErrManifest` for compose errors).
- Renderer invariants untouched; publish pipeline (`AddFragment`) unchanged.
- `go test ./...`, `go vet ./...`, `gofmt -w .` before claiming work done.
- Never commit unless the step says commit; no AI co-authorship trailers.

---

### Task 1: `pack.ComposeFragments`

**Files:**
- Create: `internal/pack/compose.go`
- Test: `internal/pack/compose_test.go`

**Interfaces:**
- Consumes: unexported `splitFrontmatter(content string) (fm, body string, ok bool)` in `internal/pack/fragment.go` (exists; do not modify).
- Produces: `func ComposeFragments(parts [][]byte) ([]byte, error)` — used by Task 3.

- [ ] **Step 1: Write the failing tests**

Create `internal/pack/compose_test.go`:

```go
package pack

import (
	"strings"
	"testing"
)

func TestComposeFragmentsMergesFrontmatterAndBodies(t *testing.T) {
	a := []byte("---\ntargets: [claude, agents]\n---\n# A\n\nBody A.\n")
	b := []byte("---\ntargets: [claude, agents]\n---\n# B\n\nBody B.\n")
	got, err := ComposeFragments([][]byte{a, b})
	if err != nil {
		t.Fatal(err)
	}
	want := "---\ntargets: [claude, agents]\n---\n\n# A\n\nBody A.\n\n# B\n\nBody B.\n"
	if string(got) != want {
		t.Fatalf("composed:\n%q\nwant:\n%q", got, want)
	}
}

func TestComposeFragmentsUnionsDistinctTargets(t *testing.T) {
	a := []byte("---\ntargets: [claude]\n---\nA\n")
	b := []byte("---\ntargets: [agents]\n---\nB\n")
	got, err := ComposeFragments([][]byte{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(got), "---\ntargets: [claude, agents]\n---\n") {
		t.Fatalf("composed frontmatter wrong: %q", got)
	}
}

func TestComposeFragmentsSinglePartVerbatim(t *testing.T) {
	a := []byte("---\ntargets: [claude]\n---\nA body\n")
	got, err := ComposeFragments([][]byte{a})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(a) {
		t.Fatal("single part must pass through byte-identical")
	}
}

func TestComposeFragmentsNoFrontmatterPart(t *testing.T) {
	a := []byte("---\ntargets: [claude]\n---\nA\n")
	b := []byte("Plain body\n")
	got, err := ComposeFragments([][]byte{a, b})
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if !strings.HasPrefix(s, "---\ntargets: [claude]\n---\n") || !strings.Contains(s, "Plain body") {
		t.Fatalf("composed: %q", s)
	}
}

func TestComposeFragmentsBadFrontmatterErrors(t *testing.T) {
	bad := []byte("---\ntargets: [unclosed\n---\nX\n")
	if _, err := ComposeFragments([][]byte{[]byte("A\n"), bad}); err == nil {
		t.Fatal("expected error for unparseable frontmatter")
	}
}

func TestComposeFragmentsEmptyErrors(t *testing.T) {
	if _, err := ComposeFragments(nil); err == nil {
		t.Fatal("expected error for empty parts")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/pack/ -run TestComposeFragments -v`
Expected: FAIL to compile — `undefined: ComposeFragments`

- [ ] **Step 3: Write the implementation**

Create `internal/pack/compose.go`:

```go
package pack

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/tensorgroup/openescapement/internal/esc"
)

// ComposeFragments merges rule fragments into one: a single leading
// frontmatter carrying the union of the parts' targets (first-appearance
// order), then the bodies joined by blank lines. A sole part passes through
// byte-identical so single-fragment adoption keeps its historical output.
// A part with no frontmatter contributes no targets; callers are expected
// to compose fragments that share a targets posture.
func ComposeFragments(parts [][]byte) ([]byte, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("%w: compose: no fragments selected", esc.ErrManifest)
	}
	if len(parts) == 1 {
		return parts[0], nil
	}
	var union []string
	seen := map[string]bool{}
	bodies := make([]string, 0, len(parts))
	for i, p := range parts {
		content := string(p)
		if fm, body, ok := splitFrontmatter(content); ok {
			var meta struct {
				Targets []string `yaml:"targets"`
			}
			if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
				return nil, fmt.Errorf("%w: compose part %d frontmatter: %v", esc.ErrManifest, i, err)
			}
			for _, t := range meta.Targets {
				if !seen[t] {
					seen[t] = true
					union = append(union, t)
				}
			}
			content = body
		}
		bodies = append(bodies, strings.TrimSpace(content))
	}
	var b strings.Builder
	if len(union) > 0 {
		b.WriteString("---\ntargets: [" + strings.Join(union, ", ") + "]\n---\n\n")
	}
	b.WriteString(strings.Join(bodies, "\n\n"))
	b.WriteString("\n")
	return []byte(b.String()), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/pack/ -v`
Expected: all PASS (including pre-existing pack tests)

- [ ] **Step 5: Commit**

```bash
git add internal/pack/compose.go internal/pack/compose_test.go
git commit -m "feat(pack): compose multiple rule fragments into one"
```

---

### Task 2: Move absolute pricing out of example fragments

**Files:**
- Create: `internal/guidance/pricing_test.go`
- Modify: all 10 `internal/guidance/models/examples/*/starter-*.md` and `internal/guidance/models/examples/anthropic/model-routing.md`

**Interfaces:**
- Consumes: `guidance.Load(dir string) *Set`, `Set.ExampleFiles(vendorKey) []string`, `Set.ReadFile(rel) ([]byte, bool, error)`, `guidance.VendorOrder` (all exist).
- Produces: content-only change; no code interface.

- [ ] **Step 1: Write the failing test**

Create `internal/guidance/pricing_test.go`:

```go
package guidance

import (
	"regexp"
	"testing"
)

// Example fragments are adopted verbatim into user rule packs, where absolute
// prices go stale. Dollar figures belong in the vendor note pages; fragments
// keep only relative cost guidance.
func TestExampleFragmentsCarryNoAbsolutePricing(t *testing.T) {
	g := Load("")
	priced := regexp.MustCompile(`\$\d`)
	for _, vendor := range VendorOrder {
		for _, rel := range g.ExampleFiles(vendor) {
			content, _, err := g.ReadFile(rel)
			if err != nil {
				t.Fatalf("%s: %v", rel, err)
			}
			if priced.Match(content) {
				t.Errorf("%s contains absolute pricing; keep dollar figures in the vendor note page", rel)
			}
		}
	}
}
```

(If `VendorOrder` is package-private under a different name, use the identifier at `internal/guidance/models.go:21`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/guidance/ -run TestExampleFragmentsCarryNoAbsolutePricing -v`
Expected: FAIL listing all 10 starter files

- [ ] **Step 3: Edit the fragments**

Each edit is a single-line replacement (line 6 unless noted). Old text is verbatim from the files.

`examples/anthropic/starter-claude-opus-5.md` line 6 — replace
`(1M-token context, $5 / $25 per million input/output tokens).` with `(1M-token context).`

`examples/anthropic/starter-claude-opus-5.md` line 8 — replace
`Fable 5 costs double per token and runs slower.` with `Fable 5 costs materially more per token and runs slower.`

`examples/anthropic/starter-claude-fable-5.md` line 6 — replace
`it is priced at $10 / $50 per million tokens, double Opus 5, and slower in practice.` with `it costs materially more per token than Opus 5 and is slower in practice.`

`examples/anthropic/starter-claude-sonnet-5.md` line 6 — replace
`(1M-token context, $3 / $15 per million input/output tokens).` with `(1M-token context).`

`examples/anthropic/starter-claude-haiku-4-5.md` line 6 — replace
`($1 / $5 per million tokens, 200K-token context).` with `(200K-token context).`

`examples/anthropic/model-routing.md` line 6 — replace
`Fable 5 costs double Opus 5 per token and is not the default frontier choice.` with `Fable 5 costs materially more per token than Opus 5 and is not the default frontier choice.`

`examples/openai/starter-gpt-5.6-sol.md` line 6 — replace
`(1.05M-token context, $5 / $30 per million input/output tokens).` with `(1.05M-token context).`

`examples/openai/starter-gpt-5.6-terra.md` line 6 — replace
`at well under half the cost ($2 / $12 per million tokens, 1.05M-token context).` with `at well under half the cost (1.05M-token context).`

`examples/openai/starter-gpt-5.6-luna.md` line 6 — replace
`($0.20 / $1.20 per million tokens, 1.05M-token context).` with `(1.05M-token context).`

`examples/google/starter-gemini-3.1-pro-preview.md` line 6 — replace
`(1,048,576-token input context, $2 / $12 per million input/output tokens for prompts up to 200K tokens).` with `(1,048,576-token input context).`

`examples/google/starter-gemini-3.6-flash.md` line 6 — replace
`($1.50 / $7.50 per million tokens, 1,048,576-token input context).` with `(1,048,576-token input context).`

`examples/google/starter-gemini-3.5-flash-lite.md` line 6 — replace
`($0.30 / $2.50 per million tokens, 1,048,576-token input context).` with `(1,048,576-token input context).`

Do NOT touch the vendor note pages (`anthropic.md`, `openai.md`, `google.md`) — they keep the sourced dollar figures.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/guidance/ -v`
Expected: all PASS (checklist and load tests must stay green)

- [ ] **Step 5: Commit**

```bash
git add internal/guidance/pricing_test.go internal/guidance/models/examples/
git commit -m "feat(guidance): move absolute pricing out of example fragments"
```

---

### Task 3: Generalize the adopt flow to fragment selections

**Files:**
- Modify: `internal/portal/web/models.go` (replace `starterModel`, `modelAdoptData`, `handleModelAdopt`, `handleModelAdoptSave`; lines 235-391)
- Modify: `internal/portal/web/templates/model_adopt.html` (full rewrite below)
- Test: `internal/portal/web/models_test.go`

**Interfaces:**
- Consumes: `pack.ComposeFragments(parts [][]byte) ([]byte, error)` (Task 1); existing `findVendor`, `writablePacks`, `packOptions`, `mdHTML`, `s.Packs.AddFragment(ctx, name, frag, content, version)`.
- Produces: `resolveAdoptSelection(g *guidance.Set, v guidance.Vendor, ids []string, routing bool) (adoptSelection, bool)`, `adoptDest(v guidance.Vendor, models []guidance.Model, routing bool) string`, `adoptLabel(v guidance.Vendor, models []guidance.Model, routing bool) string` — Task 4's panel submits to these handlers via GET.

- [ ] **Step 1: Write the failing tests**

Append to `internal/portal/web/models_test.go`:

```go
func TestAdoptGETMultiRendersComposedPreview(t *testing.T) {
	h := newTestServerWithPacksAndGuidance(t).Handler()
	rr := get(t, h, "/models/anthropic/adopt?model=claude-opus-5&model=claude-sonnet-5&routing=1", nil)
	if rr.Code != 200 {
		t.Fatalf("multi adopt GET code=%d", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"rules/models-anthropic.md",
		"Model routing (Anthropic)",
		"Claude Opus 5 governance",
		"Claude Sonnet 5 governance",
		`name="model" value="claude-opus-5"`,
		`name="routing" value="1"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("multi adopt form missing %q", want)
		}
	}
}

func TestAdoptPOSTMultiPublishesVendorSet(t *testing.T) {
	h := newTestServerWithPacksAndGuidance(t).Handler()
	rr := adoptPost(t, h, url.Values{
		"model": {"claude-opus-5", "claude-sonnet-5"}, "routing": {"1"},
		"pack": {"org-baseline"}, "version": {"1.3.0"},
	})
	if rr.Code != 303 || rr.Header().Get("Location") != "/packs/org-baseline?published=v1.3.0" {
		t.Fatalf("multi adopt POST: code=%d loc=%s body=%s", rr.Code, rr.Header().Get("Location"), rr.Body.String())
	}
	detail := get(t, h, "/packs/org-baseline", nil).Body.String()
	if !strings.Contains(detail, "rules/models-anthropic.md") {
		t.Fatal("vendor-set fragment not visible on pack detail")
	}
}

func TestAdoptRoutingOnly(t *testing.T) {
	h := newTestServerWithPacksAndGuidance(t).Handler()
	rr := get(t, h, "/models/anthropic/adopt?routing=1", nil)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "rules/models-anthropic.md") {
		t.Fatalf("routing-only adopt: code=%d", rr.Code)
	}
}

func TestAdoptRejectsUnknownOrForeignSelection(t *testing.T) {
	h := newTestServerWithPacksAndGuidance(t).Handler()
	for _, q := range []string{
		"model=claude-sonnet-5&model=nope",        // unknown id poisons the whole set
		"model=claude-sonnet-5&model=gpt-5.6-sol", // real id, wrong vendor
	} {
		if get(t, h, "/models/anthropic/adopt?"+q, nil).Code != 404 {
			t.Fatalf("adopt GET %s should 404", q)
		}
	}
	if get(t, h, "/models/kimi/adopt?routing=1", nil).Code != 404 {
		t.Fatal("routing adopt for a vendor without examples should 404")
	}
	if adoptPost(t, h, url.Values{"model": {"claude-sonnet-5", "nope"}, "pack": {"org-baseline"}, "version": {"1.3.0"}}).Code != 404 {
		t.Fatal("adopt POST with unknown id should 404")
	}
}

func TestAdoptMultiCollisionPreservesSelection(t *testing.T) {
	h := newTestServerWithPacksAndGuidance(t).Handler()
	form := url.Values{"model": {"claude-opus-5", "claude-sonnet-5"}, "routing": {"1"}, "pack": {"org-baseline"}, "version": {"1.3.0"}}
	if rr := adoptPost(t, h, form); rr.Code != 303 {
		t.Fatalf("first adopt: %d", rr.Code)
	}
	form.Set("version", "1.4.0")
	rr := adoptPost(t, h, form)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("collision code=%d", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{"Edit that fragment", `name="model" value="claude-opus-5"`, `name="model" value="claude-sonnet-5"`, `name="routing" value="1"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("collision page missing %q", body)
		}
	}
}
```

Also update the existing `TestAdoptGETRendersForm` (models_test.go:233): change the expected string `"Starter preview"` to `"Preview"` (the heading generalizes in the template rewrite).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/portal/web/ -run 'TestAdopt' -v`
Expected: new tests FAIL (404s where 200/303 expected); existing single-model adopt tests still PASS.

- [ ] **Step 3: Replace the selection gate and handlers in `internal/portal/web/models.go`**

Add `"github.com/tensorgroup/openescapement/internal/pack"` to the imports. Delete `starterModel` (models.go:235-244) and replace `modelAdoptData`, `handleModelAdopt`, `handleModelAdoptSave` (models.go:252-391) with:

```go
// adoptSelection is a validated adopt request: the chosen registry models in
// registry order, and the fragment bytes to compose (routing overview first).
type adoptSelection struct {
	Models []guidance.Model
	Parts  [][]byte
}

// resolveAdoptSelection resolves requested starter ids plus the optional
// routing example against the vendor's registry. All-or-nothing: any unknown
// or starter-less id fails the whole request. Every path read here is either
// registry-declared or built from the vendor key alone — the request never
// contributes a file path.
func resolveAdoptSelection(g *guidance.Set, v guidance.Vendor, ids []string, routing bool) (adoptSelection, bool) {
	requested := map[string]bool{}
	for _, id := range ids {
		requested[id] = true
	}
	var sel adoptSelection
	if routing {
		content, _, err := g.ReadFile("examples/" + v.Key + "/model-routing.md")
		if err != nil {
			return adoptSelection{}, false
		}
		sel.Parts = append(sel.Parts, content)
	}
	for _, m := range v.Models {
		if m.Starter == "" || !requested[m.ID] {
			continue
		}
		delete(requested, m.ID)
		content, _, err := g.ReadFile(m.Starter)
		if err != nil {
			return adoptSelection{}, false
		}
		sel.Models = append(sel.Models, m)
		sel.Parts = append(sel.Parts, content)
	}
	if len(requested) > 0 || len(sel.Parts) == 0 {
		return adoptSelection{}, false
	}
	return sel, true
}

// adoptDest names the destination fragment: the historical per-model path for
// a single starter, one vendor-set file for any composed selection.
func adoptDest(v guidance.Vendor, models []guidance.Model, routing bool) string {
	if len(models) == 1 && !routing {
		return "rules/model-" + models[0].ID + ".md"
	}
	return "rules/models-" + v.Key + ".md"
}

// adoptLabel names the selection for page titles and headings.
func adoptLabel(v guidance.Vendor, models []guidance.Model, routing bool) string {
	names := make([]string, len(models))
	for i, m := range models {
		names[i] = m.Name
	}
	label := strings.Join(names, ", ")
	if routing {
		if label == "" {
			return v.Name + " model routing"
		}
		label += " + model routing"
	}
	return label
}

// modelAdoptData is the /models/{vendor}/adopt page's data.
type modelAdoptData struct {
	layoutData
	VendorKey   string
	VendorName  string
	Selection   []guidance.Model
	Routing     bool
	Label       string
	Dest        string
	StarterHTML template.HTML
	StarterRaw  string
	Packs       []packOption
	PackName    string
	Version     string
	Error       string
}

func (s *Server) handleModelAdopt(w http.ResponseWriter, r *http.Request) {
	vendor := r.PathValue("vendor")
	g := s.loadGuidance()
	v, ok := findVendor(g.Registry, vendor)
	if !ok {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	routing := q.Get("routing") != ""
	sel, ok := resolveAdoptSelection(g, v, q["model"], routing)
	if !ok {
		http.NotFound(w, r)
		return
	}
	content, err := pack.ComposeFragments(sel.Parts)
	if err != nil {
		serverError(w, err)
		return
	}
	infos, err := s.writablePacks(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	if len(infos) == 0 {
		http.NotFound(w, r)
		return
	}
	opts := packOptions(infos)
	s.render(w, "model_adopt", modelAdoptData{
		layoutData:  s.baseData("models"),
		VendorKey:   v.Key,
		VendorName:  v.Name,
		Selection:   sel.Models,
		Routing:     routing,
		Label:       adoptLabel(v, sel.Models, routing),
		Dest:        adoptDest(v, sel.Models, routing),
		StarterHTML: mdHTML(content),
		StarterRaw:  string(content),
		Packs:       opts,
		PackName:    opts[0].Name,
		Version:     opts[0].SuggestedVersion,
	})
}

func (s *Server) handleModelAdoptSave(w http.ResponseWriter, r *http.Request) {
	vendor := r.PathValue("vendor")
	g := s.loadGuidance()
	v, ok := findVendor(g.Registry, vendor)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	routing := r.FormValue("routing") != ""
	sel, ok := resolveAdoptSelection(g, v, r.Form["model"], routing)
	if !ok {
		http.NotFound(w, r)
		return
	}
	content, err := pack.ComposeFragments(sel.Parts)
	if err != nil {
		serverError(w, err)
		return
	}
	infos, err := s.writablePacks(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	if len(infos) == 0 {
		http.NotFound(w, r)
		return
	}
	packName := r.FormValue("pack")
	version := r.FormValue("version")
	valid := false
	for _, p := range infos {
		if p.Name == packName {
			valid = true
		}
	}
	if !valid {
		http.NotFound(w, r)
		return
	}
	dest := adoptDest(v, sel.Models, routing)
	if err := s.Packs.AddFragment(r.Context(), packName, dest, content, version); err != nil {
		if errors.Is(err, publish.ErrFragmentExists) || errors.Is(err, esc.ErrManifest) || errors.Is(err, publish.ErrBadVersion) {
			msg := err.Error()
			if errors.Is(err, publish.ErrFragmentExists) {
				msg = "This pack already has " + dest + ". Edit that fragment in the pack instead; adopting never overwrites."
			}
			s.renderStatus(w, http.StatusUnprocessableEntity, "model_adopt", modelAdoptData{
				layoutData:  s.baseData("models"),
				VendorKey:   v.Key,
				VendorName:  v.Name,
				Selection:   sel.Models,
				Routing:     routing,
				Label:       adoptLabel(v, sel.Models, routing),
				Dest:        dest,
				StarterHTML: mdHTML(content),
				StarterRaw:  string(content),
				Packs:       packOptions(infos),
				PackName:    packName,
				Version:     version,
				Error:       msg,
			})
			return
		}
		serverError(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/packs/%s?published=v%s", packName, version), http.StatusSeeOther)
}
```

`packOption` and `packOptions` (models.go:247-276) stay as they are.

- [ ] **Step 4: Rewrite `internal/portal/web/templates/model_adopt.html`**

Full new content:

```html
{{define "title"}}Adopt: {{.Label}}{{end}}
{{define "explainer"}}Add {{.VendorName}} starter guidance to a rule pack.{{end}}
{{define "content"}}
{{if .Error}}<div class="error">{{.Error}}</div>{{end}}
<form method="post" action="/models/{{.VendorKey}}/adopt">
  {{range .Selection}}<input type="hidden" name="model" value="{{.ID}}">{{end}}
  {{if .Routing}}<input type="hidden" name="routing" value="1">{{end}}
  <p>
    <label>Rule pack
      <select name="pack">
        {{range .Packs}}<option value="{{.Name}}"{{if eq .Name $.PackName}} selected{{end}}>{{.Name}} (next {{.SuggestedVersion}})</option>{{end}}
      </select>
    </label>
  </p>
  <p>Destination fragment: <code>{{.Dest}}</code></p>
  <p>
    <label>Version
      <input name="version" value="{{.Version}}">
    </label>
  </p>
  <p><button type="submit">Add to rule pack</button></p>
</form>
<section hx-disable>
  <h3>Preview</h3>
  <article class="fragment">
    {{.StarterHTML}}
    <pre>{{.StarterRaw}}</pre>
  </article>
</section>
{{end}}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/portal/... -v`
Expected: all PASS, including the pre-existing `TestAdoptPOSTPublishesAndRedirects` (single-model dest `rules/model-claude-sonnet-5.md` unchanged) and the updated `TestAdoptGETRendersForm`.

- [ ] **Step 6: Commit**

```bash
git add internal/portal/web/models.go internal/portal/web/templates/model_adopt.html internal/portal/web/models_test.go
git commit -m "feat(portal): adopt a composed selection of vendor starters"
```

---

### Task 4: Vendor starter-set panel and pointer line

**Files:**
- Modify: `internal/portal/web/models.go` (`modelVendorData` at :84-94, `handleModelVendor` at :96-160)
- Modify: `internal/portal/web/templates/model_vendor.html`
- Test: `internal/portal/web/models_test.go`

**Interfaces:**
- Consumes: the generalized GET `/models/{vendor}/adopt` from Task 3 (repeated `model` params + `routing=1`); `g.KnownFiles() map[string]bool`.
- Produces: template-only UI; new `modelVendorData` fields `HasRouting bool`, `ShowSet bool`.

- [ ] **Step 1: Write the failing test**

Append to `internal/portal/web/models_test.go`:

```go
func TestVendorPageRendersStarterSetPanel(t *testing.T) {
	h := newTestServerWithPacksAndGuidance(t).Handler()
	body := get(t, h, "/models/anthropic", nil).Body.String()
	for _, want := range []string{
		"Vendor starter set",
		`<input type="checkbox" name="model" value="claude-opus-5" checked>`,
		`<input type="checkbox" name="routing" value="1" checked>`,
		"Use the vendor starter set panel above",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("/models/anthropic missing %q", want)
		}
	}
	noPacks := newTestServerWithGuidance(t).Handler()
	if strings.Contains(get(t, noPacks, "/models/anthropic", nil).Body.String(), "Vendor starter set") {
		t.Fatal("starter-set panel must not render when no packs configured")
	}
	if strings.Contains(get(t, h, "/models/kimi", nil).Body.String(), "Vendor starter set") {
		t.Fatal("starter-set panel must not render for vendors without starters")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/portal/web/ -run TestVendorPageRendersStarterSetPanel -v`
Expected: FAIL — "missing \"Vendor starter set\""

- [ ] **Step 3: Extend `modelVendorData` and `handleModelVendor`**

In `modelVendorData` (models.go:84-94) add two fields after `PacksAvailable bool`:

```go
	HasRouting     bool
	ShowSet        bool
```

In `handleModelVendor`, after the `packsAvailable` computation (models.go:144-147), add:

```go
	starters := 0
	for _, mv := range models {
		if mv.HasStarter {
			starters++
		}
	}
	hasRouting := g.KnownFiles()["examples/"+v.Key+"/model-routing.md"]
	showSet := packsAvailable && (starters >= 2 || (starters == 1 && hasRouting))
```

and pass `HasRouting: hasRouting, ShowSet: showSet,` in the `modelVendorData` literal (after `PacksAvailable`).

- [ ] **Step 4: Add the panel and pointer to `model_vendor.html`**

Insert after the Guidance section (after line 10's `</section>`), before `<section>`/`<h3>Models</h3>`:

```html
{{if .ShowSet}}
<section>
  <h3>Vendor starter set</h3>
  <p class="muted">Adopt several starters and the routing overview as one composed fragment.</p>
  <form method="get" action="/models/{{.VendorKey}}/adopt">
    {{range .Models}}{{if .HasStarter}}
    <label><input type="checkbox" name="model" value="{{.ID}}" checked> {{.Name}}</label>
    {{end}}{{end}}
    {{if .HasRouting}}
    <label><input type="checkbox" name="routing" value="1" checked> Model routing overview</label>
    {{end}}
    <p><button type="submit">Review &amp; adopt selection</button></p>
  </form>
</section>
{{end}}
```

Replace the single-adopt line (line 26):

```html
    <p><a class="button" href="/models/{{$.VendorKey}}/adopt?model={{.ID}}">Add to rule pack</a>{{if $.ShowSet}} <span class="muted">Adopting the whole {{$.VendorName}} set? Use the vendor starter set panel above.</span>{{end}}</p>
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/portal/web/ -v`
Expected: all PASS (including `TestVendorPageRendersStarterAndAdoptButton` — the single adopt link href is unchanged)

- [ ] **Step 6: Commit**

```bash
git add internal/portal/web/models.go internal/portal/web/templates/model_vendor.html internal/portal/web/models_test.go
git commit -m "feat(portal): vendor starter-set panel with multi-select"
```

---

### Task 5: Full verification and CHANGELOG

**Files:**
- Modify: `CHANGELOG.md` (Unreleased → Added)

**Interfaces:** none — verification and docs.

- [ ] **Step 1: Full validation**

Run, in order, and confirm clean output:

```bash
gofmt -l .        # expect no output
go vet ./...      # expect no output
go test ./...     # expect all packages ok
```

- [ ] **Step 2: Add the CHANGELOG entry**

In `CHANGELOG.md`, under `## [Unreleased]` / `### Added`, insert as the first bullet:

```markdown
- Vendor starter sets: each full-depth vendor page grows a starter-set panel
  that adopts any selection of model starters plus the model-routing overview
  as one composed fragment (`rules/models-<vendor>.md`). Single-model adoption
  and its `rules/model-<id>.md` naming are unchanged. Starter and example
  fragments no longer carry absolute prices — relative cost guidance stays in
  the fragments; dollar figures live in the vendor guidance pages.
```

- [ ] **Step 3: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs: vendor starter sets in CHANGELOG"
```

---

## Self-review notes

- Spec coverage: multi-select adopt (Tasks 3-4), set-level composition without new publish machinery (Task 1), pricing out of fragments (Task 2), one-line pointer (Task 4), gate generalization with the previously-missing negative tests (Task 3). 
- Back-compat pinned by existing tests: single-model dest and bytes unchanged (`ComposeFragments` single-part passthrough + `adoptDest`).
- Type consistency: `resolveAdoptSelection` returns `adoptSelection{Models, Parts}`; `adoptDest`/`adoptLabel` take `(guidance.Vendor, []guidance.Model, bool)`; handlers and 422 re-render use the same fields.
- One judgment call for the executor: if `guidance.Load("")` misbehaves with an empty dir in the Task 2 test, seed a `t.TempDir()` via `guidance.Seed` as `models_test.go:16` does and load that instead.
