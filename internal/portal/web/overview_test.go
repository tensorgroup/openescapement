package web

import (
	"strings"
	"testing"
	"time"

	"github.com/tensorgroup/openescapement/internal/portal/seed"
	"github.com/tensorgroup/openescapement/internal/portal/store"
)

func TestOverviewShowsSeededStats(t *testing.T) {
	dir := t.TempDir()
	epoch := time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
	if err := seed.Demo(dir, epoch); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := New(st, nil, "", "test")
	s.Now = func() time.Time { return epoch }
	rr := get(t, s.Handler(), "/", nil)
	if rr.Code != 200 {
		t.Fatalf("code %d", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{"governed repos", ">28<", "repos with drift", "<svg", "claude-sonnet-5"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in body", want)
		}
	}
}
