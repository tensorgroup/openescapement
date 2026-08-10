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
	s := New(st, nil, "", "test")
	h := s.Handler()

	// Each row carries two pills: Status (currency axis) and State (local-
	// tampering axis), per AGENTS.md's "never collapse the two axes" rule.
	all := get(t, h, "/fleet", nil).Body.String()
	if got := strings.Count(all, `class="pill`); got != 80 {
		t.Fatalf("all rows: %d pills, want 40 rows * 2", got)
	}
	drifted := get(t, h, "/fleet?status=drifted", nil).Body.String()
	if got := strings.Count(drifted, `class="pill`); got != 8 {
		t.Fatalf("drifted rows: %d, want 4 rows * 2", got)
	}
	if !strings.Contains(drifted, "org-baseline@") {
		t.Fatal("pack labels missing")
	}
	stale := get(t, h, "/fleet?status=stale", nil).Body.String()
	if !strings.Contains(stale, "org-baseline@1.1.0") {
		t.Fatal("stale repos should show old pack version")
	}
}

func TestFleetSort(t *testing.T) {
	dir := t.TempDir()
	epoch := time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
	if err := seed.Demo(dir, epoch); err != nil {
		t.Fatal(err)
	}
	st, _ := store.Open(dir)
	h := New(st, nil, "", "test").Handler()

	// aria-sort is set on the active column, ascending by default.
	asc := get(t, h, "/fleet?sort=repo", nil).Body.String()
	if !strings.Contains(asc, `aria-sort="ascending"`) {
		t.Fatal("active column should carry aria-sort=ascending")
	}
	// Toggling the same column flips direction, reflected in the header link.
	desc := get(t, h, "/fleet?sort=repo&dir=desc", nil).Body.String()
	if !strings.Contains(desc, `aria-sort="descending"`) {
		t.Fatal("dir=desc should render aria-sort=descending")
	}
	// HX request returns only the table fragment.
	frag := get(t, h, "/fleet?sort=status", map[string]string{"HX-Request": "true"}).Body.String()
	if strings.Contains(frag, "<html") || !strings.Contains(frag, "<table") {
		t.Fatal("HX sort must return the table fragment only")
	}
}

func TestFleetEmptyState(t *testing.T) {
	h := newTestServer(t, "").Handler() // empty store: no repos
	body := get(t, h, "/fleet", nil).Body.String()
	if !strings.Contains(body, "No repos match") {
		t.Fatal("empty fleet table needs an empty state")
	}
}
