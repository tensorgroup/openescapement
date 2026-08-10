package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tensorgroup/openescapement/internal/engine"
	"github.com/tensorgroup/openescapement/internal/pack"
)

// withClient swaps the package-level Client for the duration of the test
// and restores the original after, so no test's transport (short timeout,
// or none at all) leaks into another.
func withClient(t *testing.T, c *http.Client) {
	t.Helper()
	orig := Client
	Client = c
	t.Cleanup(func() { Client = orig })
}

// packWithEndpoint builds the minimal *pack.Pack Publish needs: just enough
// Manifest.Reporting to be picked up by distinctEndpoints.
func packWithEndpoint(name, endpoint string) *pack.Pack {
	return &pack.Pack{
		Manifest: pack.Manifest{
			Name: name,
			Reporting: &pack.Reporting{
				Amendments: engine.ReportMetrics,
				Endpoint:   endpoint,
			},
		},
	}
}

func metricsCollection() engine.Collection {
	return engine.Collection{Amendments: engine.ReportMetrics, Source: "pack"}
}

// recordingServer captures every request body it receives, in arrival
// order, and responds with status for every request.
type recordingServer struct {
	mu     sync.Mutex
	bodies [][]byte
	auths  []string
}

func (r *recordingServer) handler(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		r.mu.Lock()
		r.bodies = append(r.bodies, body)
		r.auths = append(r.auths, req.Header.Get("Authorization"))
		r.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
	}
}

func (r *recordingServer) requests() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.bodies)
}

func TestPublishSuccessPostsEnvelope(t *testing.T) {
	rs := &recordingServer{}
	srv := httptest.NewServer(rs.handler(202))
	defer srv.Close()
	withClient(t, &http.Client{Timeout: 5 * time.Second})

	root := t.TempDir()
	t.Setenv("ESC_PORTAL_TOKEN", "test-token")
	rep := fixtureReport()
	packs := []*pack.Pack{packWithEndpoint("acme", srv.URL)}
	var stderr bytes.Buffer
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	Publish(context.Background(), root, rep, packs, metricsCollection(), &stderr, now)

	if n := rs.requests(); n != 1 {
		t.Fatalf("server received %d requests, want 1", n)
	}
	if got := rs.auths[0]; got != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", got, "Bearer test-token")
	}
	var env Envelope
	if err := json.Unmarshal(rs.bodies[0], &env); err != nil {
		t.Fatalf("body did not decode as Envelope: %v\nbody: %s", err, rs.bodies[0])
	}
	if env.Schema != EnvelopeSchema {
		t.Errorf("Schema = %d, want %d", env.Schema, EnvelopeSchema)
	}
	if env.ConfigPath != ".escapement/config.yaml" {
		t.Errorf("ConfigPath = %q, want %q", env.ConfigPath, ".escapement/config.yaml")
	}
	if env.Report == nil || len(env.Report.Findings) != len(rep.Findings) {
		t.Errorf("Report findings did not survive the envelope: %+v", env.Report)
	}
	// coll.Amendments is metrics: content-only fields must have been redacted
	// before the body ever left the process.
	for _, f := range env.Report.Findings {
		if f.Amendment != nil && f.Amendment.Content != "" {
			t.Errorf("Amendment.Content survived redaction: %q", f.Amendment.Content)
		}
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr should be empty on success, got: %s", stderr.String())
	}
	// The outbox must end up empty: the one queued entry was flushed.
	entries, _, err := loadOutbox(root)
	if err != nil {
		t.Fatalf("loadOutbox: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("outbox has %d leftover entries, want 0", len(entries))
	}
}

func TestPublishFailureModesQueueAndWarn(t *testing.T) {
	sleeper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(202)
	}))
	defer sleeper.Close()

	refused := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	refusedURL := refused.URL
	refused.Close() // closed before use: nothing listens at refusedURL anymore

	unauthorized := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer unauthorized.Close()

	serverError := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer serverError.Close()

	tests := []struct {
		name     string
		endpoint string
		timeout  time.Duration
	}{
		{"connection refused", refusedURL, 5 * time.Second},
		{"timeout", sleeper.URL, 50 * time.Millisecond},
		{"401", unauthorized.URL, 5 * time.Second},
		{"500", serverError.URL, 5 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withClient(t, &http.Client{Timeout: tt.timeout})
			root := t.TempDir()
			t.Setenv("ESC_PORTAL_TOKEN", "test-token")
			rep := fixtureReport()
			packs := []*pack.Pack{packWithEndpoint("acme", tt.endpoint)}
			var stderr bytes.Buffer
			now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

			Publish(context.Background(), root, rep, packs, metricsCollection(), &stderr, now)

			lines := nonEmptyLines(stderr.String())
			if len(lines) != 1 {
				t.Fatalf("stderr lines = %d, want 1:\n%s", len(lines), stderr.String())
			}
			entries, _, err := loadOutbox(root)
			if err != nil {
				t.Fatalf("loadOutbox: %v", err)
			}
			if len(entries) != 1 {
				t.Fatalf("outbox has %d entries, want 1 (payload must land in outbox)", len(entries))
			}
			if entries[0].Endpoint != tt.endpoint {
				t.Errorf("queued entry endpoint = %q, want %q", entries[0].Endpoint, tt.endpoint)
			}
		})
	}
}

func TestPublishNextSuccessFlushesOldestFirst(t *testing.T) {
	rs := &recordingServer{}
	srv := httptest.NewServer(rs.handler(202))
	defer srv.Close()

	root := t.TempDir()
	t.Setenv("ESC_PORTAL_TOKEN", "test-token")
	rep := fixtureReport()
	packs := []*pack.Pack{packWithEndpoint("acme", srv.URL)}

	// First call: server is down, so the envelope is queued rather than sent.
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	downURL := down.URL
	down.Close()
	packsDown := []*pack.Pack{packWithEndpoint("acme", downURL)}
	withClient(t, &http.Client{Timeout: 5 * time.Second})
	var stderr1 bytes.Buffer
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	Publish(context.Background(), root, rep, packsDown, metricsCollection(), &stderr1, older)
	entries, _, err := loadOutbox(root)
	if err != nil {
		t.Fatalf("loadOutbox: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("setup: outbox has %d entries, want 1", len(entries))
	}

	// Manually rebind the queued entry's endpoint to the live server, and
	// mark it distinctly from the fresh envelope Publish is about to build,
	// so this test can observe it flushed BEFORE the fresh one, not merely
	// that both eventually arrive.
	entries[0].Endpoint = srv.URL
	entries[0].Envelope.Report.Command = "queued-older"
	if err := atomicWriteOutbox(root, entries); err != nil {
		t.Fatalf("atomicWriteOutbox: %v", err)
	}

	var stderr2 bytes.Buffer
	newer := older.Add(time.Minute)
	Publish(context.Background(), root, rep, packs, metricsCollection(), &stderr2, newer)

	if n := rs.requests(); n != 2 {
		t.Fatalf("server received %d requests, want 2 (queued entry + fresh envelope)", n)
	}
	var first, second Envelope
	if err := json.Unmarshal(rs.bodies[0], &first); err != nil {
		t.Fatalf("decode first request body: %v", err)
	}
	if err := json.Unmarshal(rs.bodies[1], &second); err != nil {
		t.Fatalf("decode second request body: %v", err)
	}
	if first.Report.Command != "queued-older" {
		t.Errorf("first request Command = %q, want %q (the queued backlog entry must arrive first)", first.Report.Command, "queued-older")
	}
	if second.Report.Command != rep.Command {
		t.Errorf("second request Command = %q, want %q (the fresh envelope must arrive after the backlog)", second.Report.Command, rep.Command)
	}
	remaining, _, err := loadOutbox(root)
	if err != nil {
		t.Fatalf("loadOutbox: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("outbox has %d leftover entries after a successful flush, want 0", len(remaining))
	}
}

// TestPublishFlushReRedactsAtCurrentLevel pins the privacy fix: an envelope
// queued while the resolved level was content (endpoint down) must not
// leave content-grade bytes on a later flush if the level has since been
// clamped down to metrics. The outbox stores what was true when the entry
// was queued; only the send path, which holds the CURRENT Collection, can
// re-redact it correctly.
func TestPublishFlushReRedactsAtCurrentLevel(t *testing.T) {
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	downURL := down.URL
	down.Close() // closed before use: nothing listens at downURL anymore

	withClient(t, &http.Client{Timeout: 5 * time.Second})
	root := t.TempDir()
	t.Setenv("ESC_PORTAL_TOKEN", "test-token")
	rep := fixtureReport()
	contentCollection := engine.Collection{Amendments: engine.ReportContent, Source: "repo-override"}
	packsDown := []*pack.Pack{packWithEndpoint("acme", downURL)}
	var stderr1 bytes.Buffer
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Queue at level content against a down endpoint: Redact at content is
	// identity, so the queued entry carries unredacted content-grade bytes.
	Publish(context.Background(), root, rep, packsDown, contentCollection, &stderr1, older)

	entries, _, err := loadOutbox(root)
	if err != nil {
		t.Fatalf("loadOutbox: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("setup: outbox has %d entries, want 1", len(entries))
	}
	if entries[0].Envelope.Report.Findings[0].Amendment.Content == "" {
		t.Fatalf("setup: queued entry should carry unredacted content, got empty Content")
	}

	// The owner clamps to metrics and the endpoint recovers: rebind the
	// queued entry to a live recording server and flush at the new level.
	rs := &recordingServer{}
	srv := httptest.NewServer(rs.handler(202))
	defer srv.Close()
	entries[0].Endpoint = srv.URL
	if err := atomicWriteOutbox(root, entries); err != nil {
		t.Fatalf("atomicWriteOutbox: %v", err)
	}

	packs := []*pack.Pack{packWithEndpoint("acme", srv.URL)}
	var stderr2 bytes.Buffer
	newer := older.Add(time.Minute)
	Publish(context.Background(), root, rep, packs, metricsCollection(), &stderr2, newer)

	if n := rs.requests(); n != 2 {
		t.Fatalf("server received %d requests, want 2 (flushed backlog + fresh envelope)", n)
	}
	for i, body := range rs.bodies {
		var env Envelope
		if err := json.Unmarshal(body, &env); err != nil {
			t.Fatalf("request %d body did not decode as Envelope: %v\nbody: %s", i, err, body)
		}
		for _, f := range env.Report.Findings {
			if f.Detail != "" {
				t.Errorf("request %d: Finding.Detail survived flush-time redaction: %q", i, f.Detail)
			}
			if f.Amendment != nil && f.Amendment.Content != "" {
				t.Errorf("request %d: Amendment.Content survived flush-time redaction: %q", i, f.Amendment.Content)
			}
			if f.Alteration != nil && f.Alteration.Diff != "" {
				t.Errorf("request %d: Alteration.Diff survived flush-time redaction: %q", i, f.Alteration.Diff)
			}
		}
		if env.Report.Skipped != nil {
			for _, sk := range *env.Report.Skipped {
				if sk.Reason != "" {
					t.Errorf("request %d: Skipped.Reason survived flush-time redaction: %q", i, sk.Reason)
				}
			}
		}
		if strings.Contains(string(body), "team added this paragraph") {
			t.Errorf("request %d: raw content-grade text leaked into the flushed body: %s", i, body)
		}
	}

	remaining, _, err := loadOutbox(root)
	if err != nil {
		t.Fatalf("loadOutbox: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("outbox has %d leftover entries after a successful flush, want 0", len(remaining))
	}
}

func TestPublishTwoEndpointsBothReceive(t *testing.T) {
	rsA := &recordingServer{}
	srvA := httptest.NewServer(rsA.handler(202))
	defer srvA.Close()
	rsB := &recordingServer{}
	srvB := httptest.NewServer(rsB.handler(202))
	defer srvB.Close()
	withClient(t, &http.Client{Timeout: 5 * time.Second})

	root := t.TempDir()
	t.Setenv("ESC_PORTAL_TOKEN", "test-token")
	rep := fixtureReport()
	packs := []*pack.Pack{
		packWithEndpoint("acme-a", srvA.URL),
		packWithEndpoint("acme-b", srvB.URL),
	}
	var stderr bytes.Buffer
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	Publish(context.Background(), root, rep, packs, metricsCollection(), &stderr, now)

	if n := rsA.requests(); n != 1 {
		t.Errorf("endpoint A received %d requests, want 1", n)
	}
	if n := rsB.requests(); n != 1 {
		t.Errorf("endpoint B received %d requests, want 1", n)
	}
}

// TestPublishFirstEndpointRefusedSecondStillReceives is the integration
// case for outbox.go's FlushOutbox grouping fix: two endpoints declared on
// the same Publish call, the first refusing connections outright. Before
// that fix, FlushOutbox drained one shared queue oldest-first and stopped
// at the very first failure, so the second (healthy) endpoint's entry would
// never be attempted at all on this call — it would queue behind the dead
// first endpoint and eventually be dropped by age eviction without ever
// reaching a server that was ready the whole time. The second endpoint must
// receive the payload on THIS SAME call, not a later retry.
func TestPublishFirstEndpointRefusedSecondStillReceives(t *testing.T) {
	refused := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	refusedURL := refused.URL
	refused.Close() // closed before use: nothing listens at refusedURL anymore

	rsB := &recordingServer{}
	srvB := httptest.NewServer(rsB.handler(202))
	defer srvB.Close()
	withClient(t, &http.Client{Timeout: 5 * time.Second})

	root := t.TempDir()
	t.Setenv("ESC_PORTAL_TOKEN", "test-token")
	rep := fixtureReport()
	packs := []*pack.Pack{
		packWithEndpoint("acme-a", refusedURL),
		packWithEndpoint("acme-b", srvB.URL),
	}
	var stderr bytes.Buffer
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	Publish(context.Background(), root, rep, packs, metricsCollection(), &stderr, now)

	if n := rsB.requests(); n != 1 {
		t.Fatalf("healthy endpoint B received %d requests, want 1 (must not queue behind dead endpoint A)", n)
	}
	entries, _, err := loadOutbox(root)
	if err != nil {
		t.Fatalf("loadOutbox: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("outbox has %d entries, want 1 (only A's failed entry retained)", len(entries))
	}
	if entries[0].Endpoint != refusedURL {
		t.Errorf("retained entry endpoint = %q, want %q", entries[0].Endpoint, refusedURL)
	}
}

func TestPublishOffSendsNothing(t *testing.T) {
	rs := &recordingServer{}
	srv := httptest.NewServer(rs.handler(202))
	defer srv.Close()
	withClient(t, &http.Client{Timeout: 5 * time.Second})

	root := t.TempDir()
	// Deliberately no ESC_PORTAL_TOKEN: off must return before the token
	// check ever runs, so an absent token must not produce a warning either.
	rep := fixtureReport()
	packs := []*pack.Pack{packWithEndpoint("acme", srv.URL)}
	var stderr bytes.Buffer
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	Publish(context.Background(), root, rep, packs, engine.Collection{Amendments: engine.ReportOff, Source: "default"}, &stderr, now)

	if n := rs.requests(); n != 0 {
		t.Errorf("server received %d requests, want 0", n)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr should be empty when off, got: %s", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".escapement", "outbox.jsonl")); !os.IsNotExist(err) {
		t.Errorf("outbox file should not exist when off, stat err = %v", err)
	}
}

func TestPublishMissingTokenWarnsAndSkips(t *testing.T) {
	rs := &recordingServer{}
	srv := httptest.NewServer(rs.handler(202))
	defer srv.Close()
	withClient(t, &http.Client{Timeout: 5 * time.Second})

	root := t.TempDir()
	t.Setenv("ESC_PORTAL_TOKEN", "")
	rep := fixtureReport()
	packs := []*pack.Pack{packWithEndpoint("acme", srv.URL)}
	var stderr bytes.Buffer
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	Publish(context.Background(), root, rep, packs, metricsCollection(), &stderr, now)

	if n := rs.requests(); n != 0 {
		t.Errorf("server received %d requests, want 0", n)
	}
	lines := nonEmptyLines(stderr.String())
	if len(lines) != 1 {
		t.Fatalf("stderr lines = %d, want 1:\n%s", len(lines), stderr.String())
	}
	if !strings.Contains(lines[0], "ESC_PORTAL_TOKEN") {
		t.Errorf("warning does not name the missing env var: %q", lines[0])
	}
	if _, err := os.Stat(filepath.Join(root, ".escapement", "outbox.jsonl")); !os.IsNotExist(err) {
		t.Errorf("outbox file should not exist when token is missing (nothing enqueued), stat err = %v", err)
	}
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}
