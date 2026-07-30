package web

import (
	"strings"
	"testing"
	"time"

	"github.com/tensorgroup/openescapement/internal/portal/seed"
	"github.com/tensorgroup/openescapement/internal/portal/store"
)

func TestFleetFilters(t *testing.T) {
	dir := t.TempDir()
	epoch := time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
	if err := seed.Demo(dir, epoch); err != nil {
		t.Fatal(err)
	}
	st, _ := store.Open(dir)
	s := New(st, "", "test")
	h := s.Handler()

	all := get(t, h, "/fleet", nil).Body.String()
	if got := strings.Count(all, `class="pill`); got != 40 {
		t.Fatalf("all rows: %d pills", got)
	}
	drifted := get(t, h, "/fleet?status=drifted", nil).Body.String()
	if got := strings.Count(drifted, `class="pill`); got != 4 {
		t.Fatalf("drifted rows: %d", got)
	}
	if !strings.Contains(drifted, "org-baseline@") {
		t.Fatal("pack labels missing")
	}
	stale := get(t, h, "/fleet?status=stale", nil).Body.String()
	if !strings.Contains(stale, "org-baseline@1.1.0") {
		t.Fatal("stale repos should show old pack version")
	}
}
