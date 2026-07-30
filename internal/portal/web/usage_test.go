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
