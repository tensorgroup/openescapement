package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/engine"
)

// TestReportingFixtureResolves exercises setupGovernedRepoWithReporting (a
// Task 0 fixture that could not compile until pack.Manifest.Reporting
// existed): a real governed repo, with a real pack fetched over file://,
// declaring reporting.amendments, resolves through engine.Plan +
// engine.ResolveReporting to the level the pack declared.
func TestReportingFixtureResolves(t *testing.T) {
	root := setupGovernedRepoWithReporting(t, "content")

	plan, err := engine.Plan(context.Background(), root)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	got, err := engine.ResolveReporting(plan.PackObjs, cfg)
	if err != nil {
		t.Fatalf("ResolveReporting: %v", err)
	}
	if got.Amendments != "content" || got.Source != "pack" {
		t.Errorf("got %+v, want content/pack", got)
	}
}

// TestStatusOutputShowsAmendmentAndNotice: a team appending its own rules to
// AGENTS.md is expected, healthy behavior, not drift, so `esc status` must
// still exit 0. The amendment renders as a neutral suffix on the in-sync
// line, and because the pack declares reporting.amendments: content, the
// collection notice must also print, unconditionally (never behind a
// verbose flag).
func TestStatusOutputShowsAmendmentAndNotice(t *testing.T) {
	repo := setupGovernedRepoWithReporting(t, "content")
	runEsc(t, repo, "sync")
	path := filepath.Join(repo, "AGENTS.md")
	existing, _ := os.ReadFile(path)
	if err := os.WriteFile(path, append(existing, []byte("\n## Team\n\nours\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := runEscOut(t, repo, "status")
	if code != 0 {
		t.Errorf("exit = %d, want 0; an amendment alone is not drift", code)
	}
	if !strings.Contains(out, "lines local") {
		t.Errorf("missing amendment suffix:\n%s", out)
	}
	if !strings.Contains(out, "reported upstream") {
		t.Errorf("missing collection notice:\n%s", out)
	}
	if strings.Contains(out, "—") {
		t.Errorf("no em-dashes in user-facing copy:\n%s", out)
	}
}

// TestStatusNoticeAbsentWhenReportingOff: the collection notice is a trust
// commitment gated on the resolved reporting level, not on whether an
// amendment exists. Local reporting of the amendment's own size is
// unaffected by that level; only the notice about what leaves the repo is.
func TestStatusNoticeAbsentWhenReportingOff(t *testing.T) {
	repo := setupGovernedRepo(t) // pack declares no reporting
	runEsc(t, repo, "sync")
	path := filepath.Join(repo, "AGENTS.md")
	existing, _ := os.ReadFile(path)
	if err := os.WriteFile(path, append(existing, []byte("\n## Team\n\nours\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	out, _ := runEscOut(t, repo, "status")
	if strings.Contains(out, "reported upstream") {
		t.Errorf("notice must not print when nothing is sent:\n%s", out)
	}
	if !strings.Contains(out, "lines local") {
		t.Errorf("local reporting is unaffected by the level:\n%s", out)
	}
}
