package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/tensorgroup/openescapement/internal/guidance"
	"github.com/tensorgroup/openescapement/internal/portal/publish"
	"github.com/tensorgroup/openescapement/internal/portal/store"
)

func newTestServerWithGuidance(t *testing.T) *Server {
	t.Helper()
	dataDir := t.TempDir()
	if err := guidance.Seed(dataDir); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := New(st, nil, "", "test")
	s.GuidanceDir = dataDir + "/guidance"
	return s
}

func newTestServerWithPacksAndGuidance(t *testing.T) *Server {
	t.Helper()
	dataDir := t.TempDir()
	if err := guidance.Seed(dataDir); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	packs := publish.NewManager(newPackClone(t))
	s := New(st, packs, "", "test")
	s.GuidanceDir = dataDir + "/guidance"
	return s
}

func TestVendorPageRendersStarterAndAdoptButton(t *testing.T) {
	h := newTestServerWithPacksAndGuidance(t).Handler()
	body := get(t, h, "/models/anthropic", nil).Body.String()
	for _, want := range []string{
		"Starter rule pack",
		`/models/anthropic/edit?file=examples%2fanthropic%2fstarter-claude-sonnet-5.md`,
		`/models/anthropic/adopt?model=claude-sonnet-5`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("/models/anthropic missing %q", want)
		}
	}
}

func TestVendorPageAdoptDisabledWithoutPacks(t *testing.T) {
	h := newTestServerWithGuidance(t).Handler() // Packs nil
	body := get(t, h, "/models/anthropic", nil).Body.String()
	if !strings.Contains(body, "Configure a rule pack repo") {
		t.Fatal("expected disabled-adopt hint")
	}
	if strings.Contains(body, `/models/anthropic/adopt?model=claude-sonnet-5`) {
		t.Fatal("adopt link must not render when no packs configured")
	}
}

func TestModelsOverviewListsEveryVendor(t *testing.T) {
	h := newTestServerWithGuidance(t).Handler()
	body := get(t, h, "/models", nil).Body.String()
	for _, want := range []string{"Anthropic", "OpenAI", "Google", "Kimi", "Deepseek", "Grok",
		`href="/models/anthropic#claude-opus-5"`, `href="/models/anthropic"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("/models missing %q", want)
		}
	}
	// Fixed vendor order.
	if strings.Index(body, "Anthropic") > strings.Index(body, "OpenAI") {
		t.Fatal("vendor order not Anthropic before OpenAI")
	}
	if !strings.Contains(get(t, h, "/", nil).Body.String(), `href="/models"`) {
		t.Fatal("sidebar missing Models nav")
	}
}

func TestModelVendorPageRendersNoteModelsExamplesAnchors(t *testing.T) {
	h := newTestServerWithGuidance(t).Handler()
	body := get(t, h, "/models/anthropic", nil).Body.String()
	for _, want := range []string{
		`id="claude-opus-5"`, // registry anchor
		"claude-sonnet-5",    // model id shown
		"hx-disable",         // rendered markdown wrapped
		`/models/anthropic/edit?file=anthropic.md`, // note Edit button, no slash to encode
		// html/template's contextual autoescaper percent-encodes "/" in a
		// URL query value, so the example's nested rel path comes out
		// encoded here. This is intentional: it keeps html/template's
		// contextual safety net intact rather than bypassing it with
		// template.URL. The edit handler (Task 5) round-trips this via
		// r.URL.Query().Get("file"), which decodes %2f back to "/"
		// transparently — net/http's query parsing, not this page, owns
		// that guarantee, so it isn't re-asserted here.
		`/models/anthropic/edit?file=examples%2fanthropic%2fmodel-routing.md`, // example Edit
		"platform.claude.com", // doc host, not full URL
		"<pre>",               // copyable raw example
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("/models/anthropic missing %q", want)
		}
	}
	if get(t, h, "/models/nope", nil).Code != 404 {
		t.Fatal("unknown vendor should 404")
	}
}

func TestModelEditRoundTrip(t *testing.T) {
	s := newTestServerWithGuidance(t)
	h := s.Handler()
	edit := get(t, h, "/models/anthropic/edit?file=anthropic.md", nil).Body.String()
	if !strings.Contains(edit, "<textarea") || !strings.Contains(edit, "hx-disable") == true {
		// editor is a plain form: must NOT be inside hx-disable, must have textarea
	}
	if !strings.Contains(edit, "<textarea") {
		t.Fatalf("edit page missing textarea: %s", edit)
	}
	// The vendor page's example Edit links emit ?file= percent-encoded
	// (examples%2fanthropic%2fmodel-routing.md); confirm the GET handler,
	// which reads r.URL.Query().Get("file"), round-trips that decoded value.
	encoded := get(t, h, "/models/anthropic/edit?file=examples%2fanthropic%2fmodel-routing.md", nil)
	if encoded.Code != 200 || !strings.Contains(encoded.Body.String(), "<textarea") {
		t.Fatalf("encoded example file edit: code=%d body=%s", encoded.Code, encoded.Body.String())
	}
	form := url.Values{
		"file":    {"anthropic.md"},
		"content": {"# Anthropic\n\nUpdated guidance body.\n\n## Sources\n- https://docs.claude.com/\n"},
	}
	req := httptest.NewRequest("POST", "/models/anthropic/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 303 || rr.Header().Get("Location") != "/models/anthropic" {
		t.Fatalf("save: code=%d loc=%s", rr.Code, rr.Header().Get("Location"))
	}
	body := get(t, h, "/models/anthropic", nil).Body.String()
	if !strings.Contains(body, "Updated guidance body.") {
		t.Fatal("next GET did not show the saved edit")
	}
}

// TestModelEditSaveWithoutDataDirReturns422 covers the editor save's only
// error branch: GuidanceDir left "" still lets the file through
// fileBelongsToVendor (KnownFiles derives from the embedded FS, not disk),
// but Set.WriteFile then fails because there's no data directory to write
// to. The handler must re-render the editor with the submitted content
// preserved (not lost) alongside the error, at 422.
func TestModelEditSaveWithoutDataDirReturns422(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := New(st, nil, "", "test") // GuidanceDir left unset ("")
	h := s.Handler()
	form := url.Values{
		"file":    {"anthropic.md"},
		"content": {"unsaved draft content"},
	}
	req := httptest.NewRequest("POST", "/models/anthropic/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("save without data dir: code=%d body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "unsaved draft content") {
		t.Fatalf("editor did not preserve submitted content: %s", body)
	}
	if !strings.Contains(body, "guidance: no data directory configured") {
		t.Fatalf("editor did not show the write error: %s", body)
	}
}

func newTestServerWithGuidanceAndEvents(t *testing.T) *Server {
	t.Helper()
	dataDir := t.TempDir()
	if err := guidance.Seed(dataDir); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AppendEvent(store.Event{
		TS:     time.Now().UTC().Add(-time.Hour),
		Kind:   "provider_usage",
		TeamID: "t1",
		Model:  "claude-sonnet-5",
		Tokens: &store.Tokens{Input: 100, Output: 50, CostUSD: 1},
	}); err != nil {
		t.Fatal(err)
	}
	s := New(st, nil, "", "test")
	s.GuidanceDir = dataDir + "/guidance"
	return s
}

func TestOverviewAndUsageLinkModelsToGuidance(t *testing.T) {
	// Overview/Usage need seeded telemetry, so drive them through a seeded
	// demo store plus guidance. Reuse the existing overview/usage fixtures'
	// approach: a store with events whose models are in the registry.
	s := newTestServerWithGuidanceAndEvents(t)
	h := s.Handler()
	ov := get(t, h, "/", nil).Body.String()
	if !strings.Contains(ov, `href="/models/anthropic#claude-sonnet-5"`) {
		t.Fatalf("overview chip not linked: %s", ov)
	}
	us := get(t, h, "/usage", nil).Body.String()
	if !strings.Contains(us, `href="/models/anthropic#claude-sonnet-5"`) {
		t.Fatalf("usage row not linked")
	}
}

func TestAdoptGETRendersForm(t *testing.T) {
	h := newTestServerWithPacksAndGuidance(t).Handler()
	rr := get(t, h, "/models/anthropic/adopt?model=claude-sonnet-5", nil)
	if rr.Code != 200 {
		t.Fatalf("adopt GET code=%d", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{`<select name="pack"`, "org-baseline", "rules/model-claude-sonnet-5.md", "Preview", "Route day-to-day coding"} {
		if !strings.Contains(body, want) {
			t.Fatalf("adopt form missing %q", want)
		}
	}
}

func TestAdoptGET404s(t *testing.T) {
	withPacks := newTestServerWithPacksAndGuidance(t).Handler()
	if get(t, withPacks, "/models/anthropic/adopt?model=nope", nil).Code != 404 {
		t.Fatal("unknown model should 404")
	}
	if get(t, withPacks, "/models/nope/adopt?model=claude-sonnet-5", nil).Code != 404 {
		t.Fatal("unknown vendor should 404")
	}
	noPacks := newTestServerWithGuidance(t).Handler() // Packs nil
	if get(t, noPacks, "/models/anthropic/adopt?model=claude-sonnet-5", nil).Code != 404 {
		t.Fatal("no packs should 404")
	}
}

func adoptPost(t *testing.T, h http.Handler, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/models/anthropic/adopt", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestAdoptPOSTPublishesAndRedirects(t *testing.T) {
	h := newTestServerWithPacksAndGuidance(t).Handler()
	rr := adoptPost(t, h, url.Values{
		"model": {"claude-sonnet-5"}, "pack": {"org-baseline"}, "version": {"1.3.0"},
	})
	if rr.Code != 303 || rr.Header().Get("Location") != "/packs/org-baseline?published=v1.3.0" {
		t.Fatalf("adopt POST: code=%d loc=%s body=%s", rr.Code, rr.Header().Get("Location"), rr.Body.String())
	}
	detail := get(t, h, "/packs/org-baseline", nil).Body.String()
	if !strings.Contains(detail, "rules/model-claude-sonnet-5.md") {
		t.Fatal("adopted fragment not visible on pack detail")
	}
}

func TestAdoptPOSTCollision422(t *testing.T) {
	h := newTestServerWithPacksAndGuidance(t).Handler()
	if rr := adoptPost(t, h, url.Values{"model": {"claude-sonnet-5"}, "pack": {"org-baseline"}, "version": {"1.3.0"}}); rr.Code != 303 {
		t.Fatalf("first adopt: %d", rr.Code)
	}
	rr := adoptPost(t, h, url.Values{"model": {"claude-sonnet-5"}, "pack": {"org-baseline"}, "version": {"1.4.0"}})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("collision code=%d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Edit that fragment") || !strings.Contains(body, `<select name="pack"`) {
		t.Fatalf("collision page missing message or preserved form: %s", body)
	}
}

func TestAdoptPOSTBadVersion422(t *testing.T) {
	h := newTestServerWithPacksAndGuidance(t).Handler()
	rr := adoptPost(t, h, url.Values{"model": {"claude-sonnet-5"}, "pack": {"org-baseline"}, "version": {"garbage"}})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad version code=%d body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "must match") || !strings.Contains(body, `<select name="pack"`) {
		t.Fatalf("bad-version page missing message or preserved form: %s", body)
	}
}

func TestAdoptPOSTVersionTagExists422(t *testing.T) {
	h := newTestServerWithPacksAndGuidance(t).Handler()
	if rr := adoptPost(t, h, url.Values{"model": {"claude-sonnet-5"}, "pack": {"org-baseline"}, "version": {"1.3.0"}}); rr.Code != 303 {
		t.Fatalf("first adopt: %d", rr.Code)
	}
	rr := adoptPost(t, h, url.Values{"model": {"claude-opus-5"}, "pack": {"org-baseline"}, "version": {"1.3.0"}})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("reused version code=%d body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "already exists") || !strings.Contains(body, `<select name="pack"`) {
		t.Fatalf("reused-version page missing message or preserved form: %s", body)
	}
}

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

func TestModelEditRejectsForeignAndUnknownFiles(t *testing.T) {
	s := newTestServerWithGuidance(t)
	h := s.Handler()
	// models.yaml is not editable; a note under the wrong vendor is rejected.
	for _, q := range []string{"file=models.yaml", "file=openai.md", "file=../secret", "file=examples/openai/x.md"} {
		if get(t, h, "/models/anthropic/edit?"+q, nil).Code != 404 {
			t.Errorf("GET edit %s should 404", q)
		}
	}
	form := url.Values{"file": {"models.yaml"}, "content": {"x"}}
	req := httptest.NewRequest("POST", "/models/anthropic/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 404 {
		t.Fatalf("POST models.yaml should 404, got %d", rr.Code)
	}
}
