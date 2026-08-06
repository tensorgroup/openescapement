package cli

import (
	"context"
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
