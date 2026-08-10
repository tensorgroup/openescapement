package cli

import (
	"os"
	"strings"
	"testing"
)

func TestPackOutdatedReportsAndGates(t *testing.T) {
	root, up := vendoredAuthorPack(t)
	// Fresh vendoring: everything up to date, --check exits 0.
	out, code := runEscOut(t, root, "pack", "outdated", "--check")
	if code != 0 {
		t.Fatalf("up-to-date --check must exit 0, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "up to date") {
		t.Fatalf("missing up-to-date rows:\n%s", out)
	}
	publishSkillV2(t, up)
	out, code = runEscOut(t, root, "pack", "outdated")
	if code != 0 {
		t.Fatalf("bare outdated is informational (exit 0), got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "v2.0.0") || !strings.Contains(out, "behind") {
		t.Fatalf("missing behind rows:\n%s", out)
	}
	// Deterministic ordering: brainstorming before writing-plans.
	if strings.Index(out, "brainstorming") > strings.Index(out, "writing-plans") {
		t.Fatalf("rows not sorted by name:\n%s", out)
	}
	if _, code = runEscOut(t, root, "pack", "outdated", "--check"); code != 1 {
		t.Fatalf("behind --check must exit 1, got %d", code)
	}
}

// TestPackOutdatedFetchFailureIsNotUpToDate pins the distinction the spec
// calls out explicitly: an unreachable remote must surface as a fetch
// failure (exit 4, esc.ErrFetch), never silently read as "up to date".
func TestPackOutdatedFetchFailureIsNotUpToDate(t *testing.T) {
	root, up := vendoredAuthorPack(t)
	gone := up + ".gone"
	if err := os.Rename(up, gone); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Rename(gone, up) })
	out, code := runEscOut(t, root, "pack", "outdated")
	if code != 4 {
		t.Fatalf("unreachable remote must exit 4 (ErrFetch), got %d:\n%s", code, out)
	}
	if strings.Contains(out, "up to date") {
		t.Fatalf("unreachable remote must never read as up to date:\n%s", out)
	}
}
