package web

import (
	"strings"
	"testing"

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
		`/models/anthropic/edit?file=anthropic.md`,                        // note Edit button
		`/models/anthropic/edit?file=examples/anthropic/model-routing.md`, // example Edit
		"docs.claude.com", // doc host, not full URL
		"<pre>",           // copyable raw example
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("/models/anthropic missing %q", want)
		}
	}
	if get(t, h, "/models/nope", nil).Code != 404 {
		t.Fatal("unknown vendor should 404")
	}
}
