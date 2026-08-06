package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/lockfile"
)

func TestReportIncludesContentRegardlessOfLevel(t *testing.T) {
	st := &StatusResult{Findings: []Finding{{
		Path: "AGENTS.md", Kind: KindBlock, State: InSync, Local: LocalAmended,
		Amendment: newAmendment("our own rules\n", nil),
	}}}
	// Local surfaces always report complete local truth. The level is
	// reported, never applied; redaction is the publisher's job.
	rep := NewReport(st, Collection{Amendments: ReportOff, Source: "default"}, nil)
	out, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "our own rules") {
		t.Errorf("local report must include content even at level off:\n%s", out)
	}
	if !strings.Contains(string(out), `"amendments":"off"`) {
		t.Errorf("collection must be reported:\n%s", out)
	}
}

func TestReportSchemaVersion(t *testing.T) {
	rep := NewReport(&StatusResult{}, Collection{}, nil)
	if rep.Schema != 1 {
		t.Errorf("schema = %d, want 1", rep.Schema)
	}
}

// TestReportPackDerivedFields locks in the three facts the brief flagged as
// guesses: Pinned is the lockfile's content hash (not Commit, which is empty
// for local dev packs), Signed is true unless config.PackRef.Trust is
// exactly "unsigned", and Latest is read from StatusResult.LatestBySource
// (populated by Status from the existing update-check log) rather than from
// any live network probe.
func TestReportPackDerivedFields(t *testing.T) {
	plan := &PlanResult{
		Config: &config.Config{Packs: []config.PackRef{
			{Source: "signed-src", Ref: "v1.0.0"},
			{Source: "unsigned-src", Ref: "main", Trust: "unsigned"},
		}},
		Packs: []lockfile.LockPack{
			{Source: "signed-src", Ref: "v1.0.0", Commit: "deadbeef", Hash: "sha256:aaa"},
			{Source: "unsigned-src", Ref: "main", Hash: "sha256:bbb"}, // Commit empty: local dev pack
		},
	}
	st := &StatusResult{
		Plan:           plan,
		LatestBySource: map[string]string{"unsigned-src": "v2.0.0-latest"},
	}
	rep := NewReport(st, Collection{}, nil)
	if len(rep.Packs) != 2 {
		t.Fatalf("packs = %d, want 2", len(rep.Packs))
	}
	byName := map[string]ReportPack{}
	for _, p := range rep.Packs {
		byName[p.Source] = p
	}

	signed := byName["signed-src"]
	if signed.Pinned != "sha256:aaa" {
		t.Errorf("Pinned = %q, want the lockfile content hash sha256:aaa (not Commit, which local packs never have)", signed.Pinned)
	}
	if !signed.Signed {
		t.Error("empty Trust must resolve to Signed=true (the default)")
	}
	if signed.Latest != "" {
		t.Errorf("Latest = %q, want empty: no update-check data recorded for this source", signed.Latest)
	}

	unsigned := byName["unsigned-src"]
	if unsigned.Signed {
		t.Error("Trust: unsigned must resolve to Signed=false")
	}
	if unsigned.Latest != "v2.0.0-latest" {
		t.Errorf("Latest = %q, want v2.0.0-latest from StatusResult.LatestBySource", unsigned.Latest)
	}
}

// TestPopulateDiffsSkipsDirAndJSONKeys is the regression test for the trap
// the brief called out explicitly: KindDir's ExpectedHash/ActualHash is a
// whole-tree hash, and shelling `git diff` on it would be meaningless, not
// merely empty. KindJSONKeys is skipped for a related reason (see
// PopulateDiffs' doc comment). Both must be left with Diff == "" — the
// hashes still identify the alteration, just not with a diff attached.
func TestPopulateDiffsSkipsDirAndJSONKeys(t *testing.T) {
	root := t.TempDir()
	plan := &PlanResult{Artifacts: []Artifact{
		{Path: "skills/x", Kind: KindDir},
		{Path: ".mcp.json", Kind: KindJSONKeys},
	}}
	rep := &Report{Artifacts: []Finding{
		{Path: "skills/x", Kind: KindDir, State: Altered, Alteration: &Alteration{ExpectedHash: "e", ActualHash: "a"}},
		{Path: ".mcp.json", Kind: KindJSONKeys, State: Altered, Alteration: &Alteration{ExpectedHash: "e", ActualHash: "a"}},
	}}
	if err := PopulateDiffs(context.Background(), root, plan, rep); err != nil {
		t.Fatal(err)
	}
	for _, f := range rep.Artifacts {
		if f.Alteration.Diff != "" {
			t.Errorf("%s (kind %s): Diff = %q, want empty", f.Path, f.Kind, f.Alteration.Diff)
		}
	}
}
