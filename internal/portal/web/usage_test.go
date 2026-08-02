package web

import (
	"strings"
	"testing"
	"time"

	"github.com/tensorgroup/openescapement/internal/portal/seed"
	"github.com/tensorgroup/openescapement/internal/portal/store"
)

func TestUsagePage(t *testing.T) {
	dir := t.TempDir()
	epoch := time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
	if err := seed.Demo(dir, epoch); err != nil {
		t.Fatal(err)
	}
	st, _ := store.Open(dir)
	s := New(st, nil, "", "test")
	s.Now = func() time.Time { return epoch }
	h := s.Handler()
	body := get(t, h, "/usage", nil).Body.String()
	for _, want := range []string{"claude-sonnet-5", "<svg", "no prompts or code", "est. cost"} {
		if !strings.Contains(strings.ToLower(body), strings.ToLower(want)) {
			t.Fatalf("missing %q", want)
		}
	}
	if strings.Count(body, "<svg") != 2 {
		t.Fatal("want two charts")
	}
	filtered := get(t, h, "/usage?model=claude-opus-5&days=7", nil).Body.String()
	if !strings.Contains(filtered, "claude-opus-5") {
		t.Fatal("filter lost")
	}
}

func TestUsageFragment(t *testing.T) {
	dir := t.TempDir()
	epoch := time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
	if err := seed.Demo(dir, epoch); err != nil {
		t.Fatal(err)
	}
	st, _ := store.Open(dir)
	s := New(st, nil, "", "test")
	s.Now = func() time.Time { return epoch }
	h := s.Handler()

	frag := get(t, h, "/usage", map[string]string{"HX-Request": "true"}).Body.String()
	if strings.Contains(frag, "<html") {
		t.Fatal("HX usage response must be a fragment")
	}
	if strings.Count(frag, "<svg") != 2 {
		t.Fatalf("fragment should carry both charts: %d", strings.Count(frag, "<svg"))
	}
	full := get(t, h, "/usage", nil).Body.String()
	if !strings.Contains(full, "<html") {
		t.Fatal("no-HX usage response must be a full page")
	}
}
