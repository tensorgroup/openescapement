package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/lockfile"
)

func TestReportIncludesContentRegardlessOfLevel(t *testing.T) {
	st := &StatusResult{Findings: []Finding{{
		Subject: "AGENTS.md", Kind: KindBlock, State: InSync, Local: LocalAmended,
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

// TestReportFindingsAndPacksNeverNil is the regression test for the defect
// where an empty repo (e.g. targets: []) rendered "findings": null /
// "packs": null — valid JSON, but something a Python or TypeScript consumer
// iterating the array throws on. Both must default to "[]", not null.
func TestReportFindingsAndPacksNeverNil(t *testing.T) {
	rep := NewReport(&StatusResult{}, Collection{}, nil)
	if rep.Findings == nil {
		t.Error("Findings is nil, want a non-nil empty slice")
	}
	if rep.Packs == nil {
		t.Error("Packs is nil, want a non-nil empty slice")
	}
	out, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "null") {
		t.Errorf("marshaled report contains a null: %s", out)
	}
	if !strings.Contains(string(out), `"findings":[]`) || !strings.Contains(string(out), `"packs":[]`) {
		t.Errorf("want findings and packs to render as [], got: %s", out)
	}
}

// TestReportCommandAndSkippedShape covers item C: Command distinguishes a
// status document from a sync document (the two are otherwise
// byte-identical when nothing was skipped), and Skipped's presence follows
// Command — omitted entirely for status, always an array (even empty) for
// sync — rather than following whether anything was actually skipped.
func TestReportCommandAndSkippedShape(t *testing.T) {
	statusRep := NewReport(&StatusResult{}, Collection{}, nil)
	if statusRep.Command != "status" {
		t.Errorf("status report Command = %q, want %q", statusRep.Command, "status")
	}
	if statusRep.Skipped != nil {
		t.Errorf("status report Skipped = %v, want nil (omitted from JSON)", statusRep.Skipped)
	}
	out, err := json.Marshal(statusRep)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), `"skipped"`) {
		t.Errorf("status report must omit the skipped key entirely: %s", out)
	}

	syncRepEmpty := NewReport(&StatusResult{}, Collection{}, &SyncResult{})
	if syncRepEmpty.Command != "sync" {
		t.Errorf("sync report Command = %q, want %q", syncRepEmpty.Command, "sync")
	}
	if syncRepEmpty.Skipped == nil {
		t.Fatal("sync report Skipped is nil, want a non-nil pointer to an empty slice")
	}
	out, err = json.Marshal(syncRepEmpty)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"skipped":[]`) {
		t.Errorf("sync report with nothing skipped must still emit \"skipped\":[], got: %s", out)
	}

	syncRepWithSkip := NewReport(&StatusResult{}, Collection{}, &SyncResult{
		Skipped: []Skipped{{Subject: "CLAUDE.md", Kind: KindBlock, Reason: "hand-edited"}},
	})
	out, err = json.Marshal(syncRepWithSkip)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"CLAUDE.md"`) {
		t.Errorf("sync report must carry the actual skipped entries: %s", out)
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

// TestPackSignedPairsByIndexNotSource is the regression test for the defect
// where packSigned matched on Source: two pack refs sharing a Source (e.g.
// during a migration between mirrors) would mis-associate Trust, and a
// Source with no match at all failed OPEN (reported Signed=true). Pairing
// by index — the two slices are always parallel, since planFromConfig
// appends res.Packs in cfg.Packs order — fixes both: a repeated source
// resolves each pin's own Trust correctly, and an out-of-range index fails
// CLOSED (Signed=false).
func TestPackSignedPairsByIndexNotSource(t *testing.T) {
	cfg := &config.Config{Packs: []config.PackRef{
		{Source: "same-src", Ref: "v1.0.0"},                    // index 0: signed
		{Source: "same-src", Ref: "v2.0.0", Trust: "unsigned"}, // index 1: unsigned, same Source as index 0
	}}
	if !packSigned(cfg, 0) {
		t.Error("index 0 (signed) must resolve true even though index 1 shares its Source")
	}
	if packSigned(cfg, 1) {
		t.Error("index 1 (unsigned) must resolve false even though index 0 shares its Source")
	}
	if packSigned(cfg, 2) {
		t.Error("out-of-range index must fail closed (false), not true")
	}
	if packSigned(nil, 0) {
		t.Error("nil config must fail closed (false), not true")
	}
}

// TestPopulateDiffsSkipsDirAndJSONKeys is the regression test for the trap
// the brief called out explicitly: KindDir's ExpectedHash/ActualHash is a
// whole-tree hash, and shelling `git diff` on it would be meaningless, not
// merely empty. KindJSONKeys is skipped for a sharper reason (see
// PopulateDiffs' doc comment): a whole-file diff would leak the user's own
// unowned MCP server entries into a document a publisher consumes. Both
// must be left with Diff == "" — the hashes still identify the alteration,
// just not with a diff attached. A third, altered KindBlock finding is
// included as a positive control: without it, this test would also pass if
// PopulateDiffs silently no-op'd for every kind, proving nothing about
// selectivity.
func TestPopulateDiffsSkipsDirAndJSONKeys(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("actual on disk\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &PlanResult{Artifacts: []Artifact{
		{Path: "skills/x", Kind: KindDir},
		{Path: ".mcp.json", Kind: KindJSONKeys},
		{Path: "CLAUDE.md", Kind: KindBlock, Body: "expected body\n"},
	}}
	rep := &Report{Findings: []Finding{
		{Subject: "skills/x", Kind: KindDir, State: Altered, Alteration: &Alteration{ExpectedHash: "e", ActualHash: "a"}},
		{Subject: ".mcp.json", Kind: KindJSONKeys, State: Altered, Alteration: &Alteration{ExpectedHash: "e", ActualHash: "a"}},
		{Subject: "CLAUDE.md", Kind: KindBlock, State: Altered, Alteration: &Alteration{ExpectedHash: "e", ActualHash: "a"}},
	}}
	if err := PopulateDiffs(context.Background(), root, plan, rep); err != nil {
		t.Fatal(err)
	}
	for _, f := range rep.Findings {
		if f.Kind == KindBlock {
			continue
		}
		if f.Alteration.Diff != "" {
			t.Errorf("%s (kind %s): Diff = %q, want empty", f.Subject, f.Kind, f.Alteration.Diff)
		}
	}
	block := rep.Findings[2]
	if block.Alteration.Diff == "" {
		t.Error("positive control: the altered KindBlock finding must get a non-empty Diff, or this test cannot distinguish selective skipping from PopulateDiffs no-op'ing entirely")
	}
}
