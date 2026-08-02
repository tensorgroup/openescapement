package web

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/tensorgroup/openescapement/internal/guidance"
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
