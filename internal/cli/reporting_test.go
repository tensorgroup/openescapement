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

// TestStatusAlteredShowsFixHint locks in the Altered-state follow-up hint
// line (`esc diff` / `esc sync --force`), which is the one piece of Task 10's
// design intent that points a team at recovery from real drift rather than
// an expected amendment. Asserts the exact two lines, not a loose substring,
// so a future edit that drops the hint or breaks its column alignment (24
// leading spaces, matching the "  ✗ %-20s" prefix width) fails the suite.
func TestStatusAlteredShowsFixHint(t *testing.T) {
	repo := setupGovernedRepo(t)
	runEsc(t, repo, "sync")
	govPath := filepath.Join(repo, "GOVERNANCE.md")
	gov, err := os.ReadFile(govPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(govPath, append(gov, []byte("\nHAND EDITED\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	out, _ := runEscOut(t, repo, "status")
	const wantFinding = "  ✗ GOVERNANCE.md        altered: file was hand-edited (hash mismatch)\n"
	const wantHint = "                         `esc diff` to inspect · `esc sync --force` to overwrite\n"
	if !strings.Contains(out, wantFinding) {
		t.Errorf("missing exact Altered finding line:\nwant substring: %q\ngot:\n%s", wantFinding, out)
	}
	if !strings.Contains(out, wantHint) {
		t.Errorf("missing exact fix-hint line:\nwant substring: %q\ngot:\n%s", wantHint, out)
	}
	if !strings.Contains(out, wantFinding+wantHint) {
		t.Errorf("hint line must immediately follow its Altered finding:\n%s", out)
	}
}

// TestStatusItemsAmendmentSuffix locks in the Items-shaped amendment suffix
// (unmanaged files preserved under a skill directory), the second rendering
// path alongside the Content/Lines shape already covered by
// TestStatusOutputShowsAmendmentAndNotice. Uses setupGovernedRepoWithSkills,
// whose synced skill directory lands at
// .claude/skills/esc-acme-org-esc-security (see that fixture's doc comment
// for the naming derivation), and drops one unmanaged file into it.
func TestStatusItemsAmendmentSuffix(t *testing.T) {
	repo := setupGovernedRepoWithSkills(t)
	runEsc(t, repo, "sync")
	skillDir := filepath.Join(repo, ".claude", "skills", "esc-acme-org-esc-security")
	if err := os.WriteFile(filepath.Join(skillDir, "TEAM-NOTES.md"), []byte("our notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := runEscOut(t, repo, "status")
	if code != 0 {
		t.Errorf("exit = %d, want 0; an amendment alone is not drift", code)
	}
	const want = "  ✓ .claude/skills/esc-acme-org-esc-security in sync  ·  1 unmanaged file preserved\n"
	if !strings.Contains(out, want) {
		t.Errorf("missing exact Items-shaped amendment line:\nwant substring: %q\ngot:\n%s", want, out)
	}
}

// TestStatusNoticeWordingByLevel locks in the two collection-notice wordings
// the resolved level chooses between: metrics ("counts and hashes only") and
// content ("including content"). Both must name the same config key and
// file, since a team withholds either level the same way.
func TestStatusNoticeWordingByLevel(t *testing.T) {
	tests := []struct {
		level string
		want  string
	}{
		{"metrics", "\nLocal amendments are reported upstream, counts and hashes only.\n(pack policy; set report_amendments: metrics or off in .escapement.yaml to withhold)\n"},
		{"content", "\nLocal amendments are reported upstream, including content.\n(pack policy; set report_amendments: metrics or off in .escapement.yaml to withhold)\n"},
	}
	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			repo := setupGovernedRepoWithReporting(t, tt.level)
			runEsc(t, repo, "sync")
			path := filepath.Join(repo, "AGENTS.md")
			existing, _ := os.ReadFile(path)
			if err := os.WriteFile(path, append(existing, []byte("\n## Team\n\nours\n")...), 0o644); err != nil {
				t.Fatal(err)
			}

			out, _ := runEscOut(t, repo, "status")
			if !strings.Contains(out, tt.want) {
				t.Errorf("level %s: missing exact notice:\nwant substring: %q\ngot:\n%s", tt.level, tt.want, out)
			}
		})
	}
}
