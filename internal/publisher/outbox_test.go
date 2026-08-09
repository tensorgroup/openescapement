package publisher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testEnvelope(tag string) Envelope {
	return Envelope{Schema: 1, Remote: "example.com/org/repo", ConfigPath: tag}
}

func readOutboxLines(t *testing.T, root string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".escapement", "outbox.jsonl"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if s == "" {
		return nil
	}
	var lines []string
	for _, l := range splitLines(s) {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func TestAppendOutboxGrowsFile(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	dropped, err := AppendOutbox(root, "https://telemetry.example/ingest", testEnvelope("a"), now)
	if err != nil {
		t.Fatalf("AppendOutbox: %v", err)
	}
	if dropped != 0 {
		t.Fatalf("dropped = %d, want 0", dropped)
	}
	lines := readOutboxLines(t, root)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}

	var rec struct {
		Time     time.Time `json:"ts"`
		Endpoint string    `json:"endpoint"`
		Envelope Envelope  `json:"envelope"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("unmarshal line: %v", err)
	}
	if rec.Endpoint != "https://telemetry.example/ingest" {
		t.Errorf("endpoint = %q", rec.Endpoint)
	}
	if !rec.Time.Equal(now) {
		t.Errorf("ts = %v, want %v", rec.Time, now)
	}
	if rec.Envelope.ConfigPath != "a" {
		t.Errorf("envelope not round-tripped: %+v", rec.Envelope)
	}

	dropped, err = AppendOutbox(root, "https://telemetry.example/ingest", testEnvelope("b"), now.Add(time.Minute))
	if err != nil {
		t.Fatalf("AppendOutbox #2: %v", err)
	}
	if dropped != 0 {
		t.Fatalf("dropped = %d, want 0", dropped)
	}
	lines = readOutboxLines(t, root)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
}

func TestAppendOutboxCapEviction(t *testing.T) {
	root := t.TempDir()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < OutboxCap; i++ {
		now := base.Add(time.Duration(i) * time.Minute)
		dropped, err := AppendOutbox(root, "ep", testEnvelope(string(rune('a'+i%26))), now)
		if err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
		if dropped != 0 {
			t.Fatalf("append %d: dropped = %d, want 0", i, dropped)
		}
	}

	// One more push past the cap must evict exactly the single oldest entry.
	overflowNow := base.Add(time.Duration(OutboxCap) * time.Minute)
	dropped, err := AppendOutbox(root, "ep", testEnvelope("overflow"), overflowNow)
	if err != nil {
		t.Fatalf("overflow append: %v", err)
	}
	if dropped != 1 {
		t.Fatalf("overflow dropped = %d, want 1", dropped)
	}

	lines := readOutboxLines(t, root)
	if len(lines) != OutboxCap {
		t.Fatalf("got %d lines, want %d (cap enforced)", len(lines), OutboxCap)
	}

	var first struct {
		Time time.Time `json:"ts"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatal(err)
	}
	wantOldestSurviving := base.Add(1 * time.Minute) // index 0 was evicted
	if !first.Time.Equal(wantOldestSurviving) {
		t.Errorf("oldest surviving ts = %v, want %v (index 0 evicted)", first.Time, wantOldestSurviving)
	}
}

func TestAppendOutboxAgeEviction(t *testing.T) {
	root := t.TempDir()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	if _, err := AppendOutbox(root, "ep", testEnvelope("old"), base); err != nil {
		t.Fatal(err)
	}

	laterNow := base.Add(OutboxMaxAge + time.Hour)
	dropped, err := AppendOutbox(root, "ep", testEnvelope("new"), laterNow)
	if err != nil {
		t.Fatal(err)
	}
	if dropped != 1 {
		t.Fatalf("dropped = %d, want 1 (age eviction)", dropped)
	}

	lines := readOutboxLines(t, root)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	var rec struct {
		Envelope Envelope `json:"envelope"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Envelope.ConfigPath != "new" {
		t.Errorf("surviving entry = %q, want %q", rec.Envelope.ConfigPath, "new")
	}
}

func TestFlushOutboxOldestFirst(t *testing.T) {
	root := t.TempDir()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for i, tag := range []string{"a", "b", "c"} {
		if _, err := AppendOutbox(root, "https://ep/"+tag, testEnvelope(tag), base.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}

	var order []string
	send := func(endpoint string, e Envelope) error {
		order = append(order, e.ConfigPath)
		return nil
	}

	sent, dropped, remaining, err := FlushOutbox(root, send, base.Add(time.Hour))
	if err != nil {
		t.Fatalf("FlushOutbox: %v", err)
	}
	if sent != 3 || dropped != 0 || remaining != 0 {
		t.Fatalf("sent=%d dropped=%d remaining=%d, want 3,0,0", sent, dropped, remaining)
	}
	want := []string{"a", "b", "c"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order[%d] = %q, want %q", i, order[i], want[i])
		}
	}

	if lines := readOutboxLines(t, root); len(lines) != 0 {
		t.Errorf("outbox should be empty after full flush, got %d lines", len(lines))
	}
}

func TestFlushOutboxStopsOnFailure(t *testing.T) {
	root := t.TempDir()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for i, tag := range []string{"a", "b", "c"} {
		if _, err := AppendOutbox(root, "ep", testEnvelope(tag), base.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}

	var attempted []string
	send := func(endpoint string, e Envelope) error {
		attempted = append(attempted, e.ConfigPath)
		if e.ConfigPath == "b" {
			return errBoom
		}
		return nil
	}

	sent, dropped, remaining, err := FlushOutbox(root, send, base.Add(time.Hour))
	if err == nil {
		t.Fatal("want non-nil err from failed send")
	}
	if sent != 1 || dropped != 0 || remaining != 2 {
		t.Fatalf("sent=%d dropped=%d remaining=%d, want 1,0,2", sent, dropped, remaining)
	}
	if len(attempted) != 2 {
		t.Fatalf("attempted = %v, want exactly [a b] (c never attempted)", attempted)
	}

	lines := readOutboxLines(t, root)
	if len(lines) != 2 {
		t.Fatalf("got %d lines remaining in file, want 2", len(lines))
	}
	var rec struct {
		Envelope Envelope `json:"envelope"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Envelope.ConfigPath != "b" {
		t.Errorf("first remaining entry = %q, want %q (failed entry kept for retry)", rec.Envelope.ConfigPath, "b")
	}
}

var errBoom = &testSendError{"send failed"}

type testSendError struct{ msg string }

func (e *testSendError) Error() string { return e.msg }

func TestFlushOutboxSkipsCorruptLine(t *testing.T) {
	root := t.TempDir()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	if _, err := AppendOutbox(root, "ep", testEnvelope("a"), base); err != nil {
		t.Fatal(err)
	}
	// Hand-inject a corrupt line between two valid entries.
	p := filepath.Join(root, ".escapement", "outbox.jsonl")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("{not valid json\n")...)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	// Append the second valid entry directly to the file (bypassing
	// AppendOutbox, which would itself observe and could rewrite around
	// the corrupt line before this test gets to exercise Flush on it).
	rec := outboxEntry{Time: base.Add(time.Minute), Endpoint: "ep", Envelope: testEnvelope("c")}
	line, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, line...)
	data = append(data, '\n')
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}

	var order []string
	send := func(endpoint string, e Envelope) error {
		order = append(order, e.ConfigPath)
		return nil
	}

	sent, dropped, remaining, err := FlushOutbox(root, send, base.Add(time.Hour))
	if err != nil {
		t.Fatalf("FlushOutbox: %v", err)
	}
	if sent != 2 || dropped != 1 || remaining != 0 {
		t.Fatalf("sent=%d dropped=%d remaining=%d, want 2,1,0 (corrupt line skipped, not fatal, counted as dropped)", sent, dropped, remaining)
	}
	want := []string{"a", "c"}
	if len(order) != len(want) || order[0] != want[0] || order[1] != want[1] {
		t.Fatalf("order = %v, want %v", order, want)
	}
}

func TestFlushOutboxMissingFileNoOp(t *testing.T) {
	root := t.TempDir()
	send := func(endpoint string, e Envelope) error {
		t.Fatal("send should never be called for a missing outbox")
		return nil
	}

	sent, dropped, remaining, err := FlushOutbox(root, send, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("FlushOutbox on missing file: %v", err)
	}
	if sent != 0 || dropped != 0 || remaining != 0 {
		t.Fatalf("sent=%d dropped=%d remaining=%d, want 0,0,0", sent, dropped, remaining)
	}
}

func TestFlushOutboxAgeEvictsStaleEntryWithoutSending(t *testing.T) {
	root := t.TempDir()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	oldTime := base
	freshTime := base.Add(OutboxMaxAge - time.Hour) // within OutboxMaxAge of oldTime: append won't evict it
	flushNow := freshTime.Add(2 * time.Hour)        // > OutboxMaxAge past oldTime, <= OutboxMaxAge past freshTime

	if _, err := AppendOutbox(root, "ep", testEnvelope("old"), oldTime); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendOutbox(root, "ep", testEnvelope("fresh"), freshTime); err != nil {
		t.Fatal(err)
	}

	var sentTags []string
	send := func(endpoint string, e Envelope) error {
		if e.ConfigPath == "old" {
			t.Fatal("send must not be called for an entry older than OutboxMaxAge at flush time")
		}
		sentTags = append(sentTags, e.ConfigPath)
		return nil
	}

	sent, dropped, remaining, err := FlushOutbox(root, send, flushNow)
	if err != nil {
		t.Fatalf("FlushOutbox: %v", err)
	}
	if sent != 1 || dropped != 1 || remaining != 0 {
		t.Fatalf("sent=%d dropped=%d remaining=%d, want 1,1,0 (stale entry dropped, fresh entry sent)", sent, dropped, remaining)
	}
	if len(sentTags) != 1 || sentTags[0] != "fresh" {
		t.Fatalf("sentTags = %v, want [fresh]", sentTags)
	}

	if lines := readOutboxLines(t, root); len(lines) != 0 {
		t.Errorf("outbox should be empty after flush (stale dropped, fresh sent), got %d lines", len(lines))
	}
}

func TestFlushOutboxDroppedCoversAgedAndCorrupt(t *testing.T) {
	root := t.TempDir()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	oldTime := base
	freshTime := base.Add(OutboxMaxAge - time.Hour)
	flushNow := freshTime.Add(2 * time.Hour)

	if _, err := AppendOutbox(root, "ep", testEnvelope("old"), oldTime); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendOutbox(root, "ep", testEnvelope("fresh"), freshTime); err != nil {
		t.Fatal(err)
	}
	// Hand-inject a corrupt line alongside the aged one.
	p := filepath.Join(root, ".escapement", "outbox.jsonl")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("{not valid json\n")...)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}

	send := func(endpoint string, e Envelope) error { return nil }

	sent, dropped, remaining, err := FlushOutbox(root, send, flushNow)
	if err != nil {
		t.Fatalf("FlushOutbox: %v", err)
	}
	if sent != 1 || dropped != 2 || remaining != 0 {
		t.Fatalf("sent=%d dropped=%d remaining=%d, want 1,2,0 (1 aged + 1 corrupt = 2 dropped)", sent, dropped, remaining)
	}
}

// TestFlushOutboxDeadEndpointDoesNotStarveOthers is the fix's core case: A
// and B are interleaved in the queue (A, B, A, B) so a naive global
// oldest-first scan that stops at the very first failure would never reach
// B at all, and B's entries would sit queued behind a dead A until age
// eviction silently dropped them. FlushOutbox must instead process each
// endpoint's backlog independently: B's two entries both go out, in order,
// on this same call, while A's two entries are retained in their original
// relative order.
func TestFlushOutboxDeadEndpointDoesNotStarveOthers(t *testing.T) {
	root := t.TempDir()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	if _, err := AppendOutbox(root, "https://a/ep", testEnvelope("a1"), base); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendOutbox(root, "https://b/ep", testEnvelope("b1"), base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendOutbox(root, "https://a/ep", testEnvelope("a2"), base.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendOutbox(root, "https://b/ep", testEnvelope("b2"), base.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}

	var bOrder []string
	send := func(endpoint string, e Envelope) error {
		if endpoint == "https://a/ep" {
			return errBoom
		}
		bOrder = append(bOrder, e.ConfigPath)
		return nil
	}

	sent, dropped, remaining, err := FlushOutbox(root, send, base.Add(time.Hour))
	if err == nil {
		t.Fatal("want non-nil err: endpoint A never succeeds")
	}
	if sent != 2 || dropped != 0 || remaining != 2 {
		t.Fatalf("sent=%d dropped=%d remaining=%d, want 2,0,2 (B's two entries sent, A's two retained)", sent, dropped, remaining)
	}
	if len(bOrder) != 2 || bOrder[0] != "b1" || bOrder[1] != "b2" {
		t.Fatalf("B's send order = %v, want [b1 b2]", bOrder)
	}

	lines := readOutboxLines(t, root)
	if len(lines) != 2 {
		t.Fatalf("got %d lines remaining, want 2 (A's backlog)", len(lines))
	}
	var recs []struct {
		Endpoint string   `json:"endpoint"`
		Envelope Envelope `json:"envelope"`
	}
	for _, l := range lines {
		var rec struct {
			Endpoint string   `json:"endpoint"`
			Envelope Envelope `json:"envelope"`
		}
		if err := json.Unmarshal([]byte(l), &rec); err != nil {
			t.Fatal(err)
		}
		recs = append(recs, rec)
	}
	if recs[0].Envelope.ConfigPath != "a1" || recs[1].Envelope.ConfigPath != "a2" {
		t.Fatalf("retained order = [%s %s], want [a1 a2] (A's original relative order preserved)",
			recs[0].Envelope.ConfigPath, recs[1].Envelope.ConfigPath)
	}
	if recs[0].Endpoint != "https://a/ep" || recs[1].Endpoint != "https://a/ep" {
		t.Fatalf("retained entries must stay bound to endpoint A: got %q, %q", recs[0].Endpoint, recs[1].Endpoint)
	}
}

// TestFlushOutboxBothEndpointsDownRetainsEverything: with no healthy
// endpoint at all, grouping must not lose or reorder anything relative to
// the old single-queue behavior.
func TestFlushOutboxBothEndpointsDownRetainsEverything(t *testing.T) {
	root := t.TempDir()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	if _, err := AppendOutbox(root, "https://a/ep", testEnvelope("a1"), base); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendOutbox(root, "https://b/ep", testEnvelope("b1"), base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	send := func(endpoint string, e Envelope) error { return errBoom }

	sent, dropped, remaining, err := FlushOutbox(root, send, base.Add(time.Hour))
	if err == nil {
		t.Fatal("want non-nil err: both endpoints fail")
	}
	if sent != 0 || dropped != 0 || remaining != 2 {
		t.Fatalf("sent=%d dropped=%d remaining=%d, want 0,0,2 (nothing sent, nothing dropped)", sent, dropped, remaining)
	}
	if lines := readOutboxLines(t, root); len(lines) != 2 {
		t.Fatalf("got %d lines remaining, want 2", len(lines))
	}
}
