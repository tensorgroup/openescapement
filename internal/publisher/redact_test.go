package publisher

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/tensorgroup/openescapement/internal/engine"
)

// fixtureReport builds a Report carrying Amendment.Content and
// Alteration.Diff on findings of all three kinds that can carry an
// Amendment/Alteration pair: block, dir, and json-keys. In real reports
// only kind=block ever populates Amendment.Content (Amendment's doc
// comment: Items carries dir/json-keys instead), and PopulateDiffs never
// fills Alteration.Diff for dir or json-keys (report.go's doc comment on
// PopulateDiffs — a whole-tree hash diff is meaningless for dir, and a
// json-keys diff would leak the user's unowned MCP server entries). This
// fixture deliberately exceeds that: Redact must not rely on those
// upstream invariants holding, or a wire type that trusts them to stay
// true would leak the moment a future bug populates a diff/content field
// on a kind that "shouldn't" carry one.
func fixtureReport() *engine.Report {
	skipped := []engine.Skipped{
		{
			Subject:      "OTHER.md",
			Kind:         engine.KindBlock,
			Cause:        engine.SkipHandEdited,
			Reason:       "managed region hand-edited",
			ExpectedHash: "skip-expected-hash",
			ActualHash:   "skip-actual-hash",
		},
	}
	return &engine.Report{
		Schema:  1,
		Command: "sync",
		Packs: []engine.ReportPack{
			{
				Source: "github.com/acme/pack",
				Ref:    "v1.2.0",
				Pinned: "pack-pinned-hash",
				Latest: "v1.3.0",
				Signed: true,
			},
		},
		Findings: []engine.Finding{
			{
				Subject: "CLAUDE.md",
				Kind:    engine.KindBlock,
				State:   engine.Altered,
				Local:   engine.LocalAmended,
				Detail:  "hand-edited managed block",
				Amendment: &engine.Amendment{
					Bytes:   42,
					Lines:   3,
					Hash:    "block-amendment-hash",
					Content: "team added this paragraph to the managed block\n",
				},
				Alteration: &engine.Alteration{
					ExpectedHash: "block-expected-hash",
					ActualHash:   "block-actual-hash",
					Diff:         "--- a/CLAUDE.md\n+++ b/CLAUDE.md\n@@ team edit @@\n",
				},
			},
			{
				Subject: ".claude/skills",
				Kind:    engine.KindDir,
				State:   engine.Altered,
				Local:   engine.LocalAmended,
				Detail:  "team added a skill file",
				Amendment: &engine.Amendment{
					Bytes:   7,
					Lines:   1,
					Hash:    "dir-amendment-hash",
					Items:   []string{"team-skill.md"},
					Content: "dir amendment content for redaction coverage\n",
				},
				Alteration: &engine.Alteration{
					ExpectedHash: "dir-expected-hash",
					ActualHash:   "dir-actual-hash",
					Diff:         "dir diff placeholder for redaction coverage\n",
				},
			},
			{
				Subject: ".mcp.json",
				Kind:    engine.KindJSONKeys,
				State:   engine.Altered,
				Local:   engine.LocalAmended,
				Detail:  "team added an MCP server",
				Amendment: &engine.Amendment{
					Bytes:   9,
					Lines:   1,
					Hash:    "json-amendment-hash",
					Items:   []string{"team-server"},
					Content: "json-keys amendment content for redaction coverage\n",
				},
				Alteration: &engine.Alteration{
					ExpectedHash: "json-expected-hash",
					ActualHash:   "json-actual-hash",
					Diff:         "json-keys diff placeholder for redaction coverage\n",
				},
			},
		},
		Collection: engine.Collection{Amendments: engine.ReportContent, Source: "pack"},
		Skipped:    &skipped,
	}
}

// snapshotReport is an independent deep-copy helper (same marshal/unmarshal
// trick Redact itself is sanctioned to use) so the "Redact never mutates
// its input" test does not depend on Redact's own copy being correct.
func snapshotReport(t *testing.T, rep *engine.Report) *engine.Report {
	t.Helper()
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("snapshotReport: marshal: %v", err)
	}
	var out engine.Report
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("snapshotReport: unmarshal: %v", err)
	}
	return &out
}

// walkJSON visits every key/value pair found in a decoded JSON tree (maps
// nested in maps and slices, to any depth), calling visit for each map key
// it encounters. It has no knowledge of engine.Report's Go field names or
// struct shape — it only sees the wire tree — so a future Report field
// that happens to be named "content" or "diff" trips the metrics-level
// test below without that test changing at all.
func walkJSON(node any, visit func(key string, val any)) {
	switch v := node.(type) {
	case map[string]any:
		for k, val := range v {
			visit(k, val)
			walkJSON(val, visit)
		}
	case []any:
		for _, item := range v {
			walkJSON(item, visit)
		}
	}
}

// nonEmptyJSON reports whether a decoded JSON value is present and not the
// empty value for its type (nil, or "").
func nonEmptyJSON(val any) bool {
	switch v := val.(type) {
	case nil:
		return false
	case string:
		return v != ""
	default:
		return true
	}
}

// collectByKey walks decoded and returns every non-empty value found under
// any of the given keys, at any depth.
func collectByKey(decoded any, keys ...string) map[string][]any {
	want := make(map[string]bool, len(keys))
	for _, k := range keys {
		want[k] = true
	}
	found := make(map[string][]any)
	walkJSON(decoded, func(key string, val any) {
		if !want[key] || !nonEmptyJSON(val) {
			return
		}
		found[key] = append(found[key], val)
	})
	return found
}

func marshalEnvelope(t *testing.T, rep *engine.Report) map[string]any {
	t.Helper()
	env := &Envelope{
		Schema:     1,
		Remote:     "github.com/acme/repo",
		ConfigPath: "/repo/.escapement/config.yml",
		Report:     rep,
	}
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	return decoded
}

// assertStripsSensitiveFields is the shared body for every "below content"
// level: engine.ReportMetrics itself, and the fail-closed cases
// (engine.ReportOff and an unrecognized level) that must redact exactly
// like metrics does. Running the identical assertions for all three pins
// the `level == engine.ReportContent` gate direction in Redact — a future
// refactor to something like `level != engine.ReportMetrics -> whole`
// would silently ship Content/Diff/Detail/Reason at ReportOff or a typoed
// level, and this helper is what catches it.
func assertStripsSensitiveFields(t *testing.T, level string) {
	t.Helper()
	rep := fixtureReport()
	redacted := Redact(rep, level)

	decoded := marshalEnvelope(t, redacted)

	found := collectByKey(decoded, "content", "diff", "detail", "reason")
	if len(found["content"]) > 0 || len(found["diff"]) > 0 {
		t.Fatalf("level %q leaked amendment/alteration content: %+v", level, found)
	}
	if len(found["detail"]) > 0 {
		t.Fatalf("level %q leaked Finding.Detail: %+v", level, found["detail"])
	}
	if len(found["reason"]) > 0 {
		t.Fatalf("level %q leaked Skipped.Reason: %+v", level, found["reason"])
	}

	report, ok := decoded["report"].(map[string]any)
	if !ok {
		t.Fatalf("decoded envelope has no report object: %+v", decoded)
	}

	packs, ok := report["packs"].([]any)
	if !ok || len(packs) != 1 {
		t.Fatalf("packs missing or wrong shape: %+v", report["packs"])
	}
	pack0 := packs[0].(map[string]any)
	if pack0["pinned"] != "pack-pinned-hash" {
		t.Errorf("level %q: pack pin did not survive: got %v", level, pack0["pinned"])
	}
	if pack0["signed"] != true {
		t.Errorf("level %q: pack signed did not survive: got %v", level, pack0["signed"])
	}

	findings, ok := report["findings"].([]any)
	if !ok || len(findings) != 3 {
		t.Fatalf("findings missing or wrong shape: %+v", report["findings"])
	}

	f0 := findings[0].(map[string]any)
	if f0["subject"] != "CLAUDE.md" {
		t.Errorf("level %q: finding subject did not survive: got %v", level, f0["subject"])
	}
	if f0["managed"] != string(engine.Altered) {
		t.Errorf("level %q: finding state did not survive: got %v", level, f0["managed"])
	}
	if f0["local"] != string(engine.LocalAmended) {
		t.Errorf("level %q: finding local state did not survive: got %v", level, f0["local"])
	}
	if _, exists := f0["detail"]; exists {
		t.Errorf("level %q: finding.detail key present: %v", level, f0["detail"])
	}

	amendment0 := f0["amendment"].(map[string]any)
	if amendment0["hash"] != "block-amendment-hash" {
		t.Errorf("level %q: amendment hash did not survive: got %v", level, amendment0["hash"])
	}
	if amendment0["bytes"] != float64(42) {
		t.Errorf("level %q: amendment bytes did not survive: got %v", level, amendment0["bytes"])
	}
	if amendment0["lines"] != float64(3) {
		t.Errorf("level %q: amendment lines did not survive: got %v", level, amendment0["lines"])
	}
	if _, exists := amendment0["content"]; exists {
		t.Errorf("level %q: amendment.content key present: %v", level, amendment0["content"])
	}

	alteration0 := f0["alteration"].(map[string]any)
	if alteration0["expected_hash"] != "block-expected-hash" {
		t.Errorf("level %q: alteration expected_hash did not survive: got %v", level, alteration0["expected_hash"])
	}
	if alteration0["actual_hash"] != "block-actual-hash" {
		t.Errorf("level %q: alteration actual_hash did not survive: got %v", level, alteration0["actual_hash"])
	}
	if _, exists := alteration0["diff"]; exists {
		t.Errorf("level %q: alteration.diff key present: %v", level, alteration0["diff"])
	}

	// dir finding: Items (counts, not content) survive.
	f1 := findings[1].(map[string]any)
	amendment1 := f1["amendment"].(map[string]any)
	items1, ok := amendment1["items"].([]any)
	if !ok || len(items1) != 1 || items1[0] != "team-skill.md" {
		t.Errorf("level %q: dir amendment items did not survive: got %v", level, amendment1["items"])
	}

	collection, ok := report["collection"].(map[string]any)
	if !ok || collection["amendments"] != string(engine.ReportContent) {
		t.Errorf("level %q: collection did not survive: got %+v", level, report["collection"])
	}

	skippedArr, ok := report["skipped"].([]any)
	if !ok || len(skippedArr) != 1 {
		t.Fatalf("skipped missing or wrong shape: %+v", report["skipped"])
	}
	skip0 := skippedArr[0].(map[string]any)
	if skip0["cause"] != string(engine.SkipHandEdited) {
		t.Errorf("level %q: skipped cause did not survive: got %v", level, skip0["cause"])
	}
	if skip0["subject"] != "OTHER.md" {
		t.Errorf("level %q: skipped subject did not survive: got %v", level, skip0["subject"])
	}
}

func TestRedact_BelowContentStripsSensitiveFields(t *testing.T) {
	// Skipped.Reason has no `omitempty` (engine/apply.go), so an emptied
	// Reason still ships as "reason":"" rather than dropping the key
	// entirely — unlike Content/Diff/Detail, which all carry omitempty and
	// vanish from the wire once emptied. assertStripsSensitiveFields's
	// non-empty-value rule tolerates that; it is cosmetic wire noise, not
	// a leak, and is called out in the task report rather than fixed here.
	levels := map[string]string{
		"metrics":             engine.ReportMetrics,
		"off (fail-closed)":   engine.ReportOff,
		"bogus (fail-closed)": "bogus-level-that-does-not-exist",
	}
	for name, level := range levels {
		t.Run(name, func(t *testing.T) {
			assertStripsSensitiveFields(t, level)
		})
	}
}

func TestRedact_ContentLevelPreservesVerbatim(t *testing.T) {
	rep := fixtureReport()
	redacted := Redact(rep, engine.ReportContent)

	decoded := marshalEnvelope(t, redacted)
	found := collectByKey(decoded, "content", "diff", "detail", "reason")

	wantContent := []string{
		"team added this paragraph to the managed block\n",
		"dir amendment content for redaction coverage\n",
		"json-keys amendment content for redaction coverage\n",
	}
	wantDiff := []string{
		"--- a/CLAUDE.md\n+++ b/CLAUDE.md\n@@ team edit @@\n",
		"dir diff placeholder for redaction coverage\n",
		"json-keys diff placeholder for redaction coverage\n",
	}
	wantDetail := []string{
		"hand-edited managed block",
		"team added a skill file",
		"team added an MCP server",
	}
	wantReason := []string{
		"managed region hand-edited",
	}

	for _, want := range wantContent {
		if !containsString(found["content"], want) {
			t.Errorf("content level dropped amendment content %q; found %v", want, found["content"])
		}
	}
	for _, want := range wantDiff {
		if !containsString(found["diff"], want) {
			t.Errorf("content level dropped alteration diff %q; found %v", want, found["diff"])
		}
	}
	for _, want := range wantDetail {
		if !containsString(found["detail"], want) {
			t.Errorf("content level dropped finding detail %q; found %v", want, found["detail"])
		}
	}
	for _, want := range wantReason {
		if !containsString(found["reason"], want) {
			t.Errorf("content level dropped skipped reason %q; found %v", want, found["reason"])
		}
	}
}

func containsString(vals []any, want string) bool {
	for _, v := range vals {
		if s, ok := v.(string); ok && s == want {
			return true
		}
	}
	return false
}

func TestRedact_DoesNotMutateInput(t *testing.T) {
	for _, level := range []string{engine.ReportMetrics, engine.ReportContent} {
		t.Run(level, func(t *testing.T) {
			rep := fixtureReport()
			snap := snapshotReport(t, rep)

			_ = Redact(rep, level)

			if !reflect.DeepEqual(rep, snap) {
				t.Fatalf("Redact mutated its input at level %q:\n got  %+v\n want %+v", level, rep, snap)
			}
		})
	}
}

func TestRedact_ReturnsDeepCopyNotAlias(t *testing.T) {
	rep := fixtureReport()
	redacted := Redact(rep, engine.ReportContent)

	if redacted == rep {
		t.Fatal("Redact returned the same *Report pointer as its input")
	}

	// Mutating the returned copy's nested pointer must not reach the
	// original: a shallow copy of the Findings slice would still alias
	// each Finding's *Amendment/*Alteration pointers.
	redacted.Findings[0].Amendment.Content = "mutated after redact"
	if rep.Findings[0].Amendment.Content == "mutated after redact" {
		t.Fatal("Redact's returned copy aliases the input's Amendment pointer")
	}
}
