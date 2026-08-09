package web

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/engine"
	"github.com/tensorgroup/openescapement/internal/portal/store"
	"github.com/tensorgroup/openescapement/internal/publisher"
)

func TestIngestAcceptsValidEvent(t *testing.T) {
	s := newTestServer(t, "tok")
	body := `{"kind":"sync","repo_id":"r9","drift":"in-sync","future_field":123}`
	req := httptest.NewRequest("POST", "/api/v1/events", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != 202 {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	events, err := s.Store.Events()
	if err != nil || len(events) != 1 || events[0].RepoID != "r9" {
		t.Fatalf("events=%v err=%v", events, err)
	}
	if events[0].TS.IsZero() {
		t.Fatal("ts not stamped")
	}
}

func TestIngestRejects(t *testing.T) {
	s := newTestServer(t, "tok")
	h := s.Handler()
	cases := []struct {
		body, auth string
		want       int
	}{
		{`{"kind":"sync"}`, "", 401},
		{`{"kind":"exec"}`, "Bearer tok", 400},
		{`{not json`, "Bearer tok", 400},
	}
	for _, c := range cases {
		req := httptest.NewRequest("POST", "/api/v1/events", strings.NewReader(c.body))
		if c.auth != "" {
			req.Header.Set("Authorization", c.auth)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != c.want {
			t.Fatalf("body %q auth %q: got %d want %d", c.body, c.auth, rr.Code, c.want)
		}
	}
	if events, _ := s.Store.Events(); len(events) != 0 {
		t.Fatal("rejected events were stored")
	}
}

// newTestServerWithRegistry is newTestServer plus a seeded registry, for
// envelope tests that need a repo to resolve a remote against.
func newTestServerWithRegistry(t *testing.T, token string, reg store.Registry) *Server {
	t.Helper()
	dir := t.TempDir()
	if err := store.SaveRegistry(dir, reg); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return New(st, nil, token, "test")
}

// testEnvelope builds a valid publisher.Envelope carrying a sync report with
// one pack pin and three findings: an in-sync block, an altered file, and a
// pack-kind finding (which must never become an EventArtifact).
func testEnvelope(remote string) publisher.Envelope {
	return publisher.Envelope{
		Schema: publisher.EnvelopeSchema,
		Remote: remote,
		Report: &engine.Report{
			Schema:  1,
			Command: "sync",
			Packs: []engine.ReportPack{
				{Source: "https://github.com/acme/org-baseline", Ref: "v1.2.0", Pinned: "abc123", Signed: true},
			},
			Findings: []engine.Finding{
				{Subject: "CLAUDE.md", Kind: engine.KindBlock, State: engine.InSync, Local: engine.LocalNone},
				{
					Subject: "AGENTS.md", Kind: engine.KindFile, State: engine.Altered, Local: engine.LocalNone,
					Alteration: &engine.Alteration{ExpectedHash: "exp1", ActualHash: "act1"},
				},
				{Subject: "https://github.com/acme/org-baseline", Kind: engine.KindPack, State: engine.InSync},
			},
			Collection: engine.Collection{Amendments: "metrics", Source: "pack"},
		},
	}
}

func postEnvelope(t *testing.T, s *Server, token string, env any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/api/v1/events", bytes.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	return rr
}

// TestIngestEnvelopeAcceptedAndReadableThroughStore is the core envelope
// path: 202, and the event lands in the store with the expected derived
// states (Kind from Report.Command, artifact-only Artifacts, drifted
// because one artifact is altered, Collection carried through).
func TestIngestEnvelopeAcceptedAndReadableThroughStore(t *testing.T) {
	s := newTestServer(t, "tok")
	env := testEnvelope("https://github.com/acme/unregistered.git")
	rr := postEnvelope(t, s, "tok", env)
	if rr.Code != 202 {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}

	events, err := s.Store.Events()
	if err != nil || len(events) != 1 {
		t.Fatalf("events=%v err=%v", events, err)
	}
	e := events[0]
	if e.Kind != "sync" {
		t.Fatalf("Kind = %q, want sync", e.Kind)
	}
	if e.TS.IsZero() {
		t.Fatal("ts not stamped")
	}
	if len(e.Artifacts) != 2 {
		t.Fatalf("Artifacts = %+v, want 2 (pack-kind finding must be excluded)", e.Artifacts)
	}
	if e.Drift != "drifted" {
		t.Fatalf("Drift = %q, want drifted (one artifact altered)", e.Drift)
	}
	if e.Collection == nil || e.Collection.Amendments != "metrics" {
		t.Fatalf("Collection = %+v, want Amendments=metrics", e.Collection)
	}
	if len(e.Packs) != 1 || e.Packs[0].Name != "https://github.com/acme/org-baseline" || e.Packs[0].Version != "abc123" || !e.Packs[0].Signed {
		t.Fatalf("Packs = %+v", e.Packs)
	}
}

// TestIngestEnvelopeSchemaMismatch pins the 400 contract for an envelope
// declaring a schema this server does not understand: the body must name
// the expected version so a caller can diagnose it.
func TestIngestEnvelopeSchemaMismatch(t *testing.T) {
	s := newTestServer(t, "tok")
	env := testEnvelope("https://github.com/acme/repo.git")
	env.Schema = 2
	rr := postEnvelope(t, s, "tok", env)
	if rr.Code != 400 {
		t.Fatalf("code=%d body=%s, want 400", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "1") {
		t.Fatalf("body %q does not name the expected schema version", rr.Body.String())
	}
}

// TestIngestEnvelopeBadTokenUnauthorized extends the existing auth coverage
// to the envelope shape: withAuth gates this route regardless of body shape.
func TestIngestEnvelopeBadTokenUnauthorized(t *testing.T) {
	s := newTestServer(t, "tok")
	env := testEnvelope("https://github.com/acme/repo.git")
	rr := postEnvelope(t, s, "wrong-token", env)
	if rr.Code != 401 {
		t.Fatalf("code=%d, want 401", rr.Code)
	}
	if events, _ := s.Store.Events(); len(events) != 0 {
		t.Fatal("unauthorized envelope was stored")
	}
}

// TestIngestEnvelopeUnregisteredRemoteRetained is spec's shadow-IT
// requirement: a remote with no registry match is never dropped, just
// stamped with an empty RepoID and the normalized remote on the event.
func TestIngestEnvelopeUnregisteredRemoteRetained(t *testing.T) {
	s := newTestServer(t, "tok") // no registry seeded at all
	env := testEnvelope("https://github.com/acme/unregistered.git")
	rr := postEnvelope(t, s, "tok", env)
	if rr.Code != 202 {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	events, err := s.Store.Events()
	if err != nil || len(events) != 1 {
		t.Fatalf("events=%v err=%v", events, err)
	}
	e := events[0]
	if e.RepoID != "" {
		t.Fatalf("RepoID = %q, want empty for an unregistered remote", e.RepoID)
	}
	if e.Remote != "github.com/acme/unregistered" {
		t.Fatalf("Remote = %q, want normalized github.com/acme/unregistered", e.Remote)
	}
}

// TestIngestEnvelopeRegisteredRemoteResolves is the matching half: a remote
// that resolves against the registry (after normalization on both sides)
// stamps RepoID/TeamID from the matched repo.
func TestIngestEnvelopeRegisteredRemoteResolves(t *testing.T) {
	reg := store.Registry{
		Repos: []store.Repo{{ID: "r1", Name: "acme-repo", TeamID: "t1", Remote: "github.com/acme/repo"}},
	}
	s := newTestServerWithRegistry(t, "tok", reg)
	// scp-like shorthand form; NormalizeRemote must reduce it to the same
	// "github.com/acme/repo" the registry entry above carries.
	env := testEnvelope("git@github.com:acme/repo.git")
	rr := postEnvelope(t, s, "tok", env)
	if rr.Code != 202 {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	events, err := s.Store.Events()
	if err != nil || len(events) != 1 {
		t.Fatalf("events=%v err=%v", events, err)
	}
	e := events[0]
	if e.RepoID != "r1" || e.TeamID != "t1" {
		t.Fatalf("RepoID/TeamID = %q/%q, want r1/t1", e.RepoID, e.TeamID)
	}
}

// TestIngestLegacyRawEventStillAccepted is the regression guard: a raw
// store.Event body (no top-level "schema" key) still hits the original
// code path and is accepted, unchanged by the envelope work.
func TestIngestLegacyRawEventStillAccepted(t *testing.T) {
	s := newTestServer(t, "tok")
	body := `{"kind":"status","repo_id":"legacy-repo","drift":"in-sync"}`
	req := httptest.NewRequest("POST", "/api/v1/events", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != 202 {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	events, err := s.Store.Events()
	if err != nil || len(events) != 1 || events[0].RepoID != "legacy-repo" {
		t.Fatalf("events=%v err=%v", events, err)
	}
}

// TestIngestEnvelopeRejectsFabricatedKind closes the gap between
// store.ValidKind's full set and what a real esc report can ever produce:
// engine.NewReport only ever sets Command to "status" or "sync" (see
// report.go), so a Command of "provider_usage" (a store.ValidKind member,
// but not a report-producible one) can only be a fabricated injection by a
// token-holder, not a real client. The envelope path must reject it, even
// though the legacy raw-Event path legitimately accepts provider_usage as a
// first-class kind from a different producer.
func TestIngestEnvelopeRejectsFabricatedKind(t *testing.T) {
	s := newTestServer(t, "tok")
	env := testEnvelope("https://github.com/acme/repo.git")
	env.Report.Command = "provider_usage"
	rr := postEnvelope(t, s, "tok", env)
	if rr.Code != 400 {
		t.Fatalf("code=%d body=%s, want 400", rr.Code, rr.Body.String())
	}
	if events, _ := s.Store.Events(); len(events) != 0 {
		t.Fatal("fabricated-kind envelope was stored")
	}
}

// TestIngestEnvelopeOversizedBodyRejected413 is the poison-pill guard: a
// content-level envelope just over maxIngestBody must get 413 (a size
// problem, not "malformed json"), so a caller with a legitimately large but
// still-too-big payload gets an accurate, retryable-only-after-shrinking
// signal instead of looking indistinguishable from garbage JSON.
func TestIngestEnvelopeOversizedBodyRejected413(t *testing.T) {
	s := newTestServer(t, "tok")
	env := testEnvelope("https://github.com/acme/repo.git")
	// Pad well past maxIngestBody via a single large Detail field, the
	// simplest way to blow the byte budget without changing the envelope's
	// shape.
	env.Report.Findings[0].Detail = strings.Repeat("x", maxIngestBody+1024)
	body, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) <= maxIngestBody {
		t.Fatalf("test body is %d bytes, want > maxIngestBody (%d)", len(body), maxIngestBody)
	}
	req := httptest.NewRequest("POST", "/api/v1/events", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != 413 {
		t.Fatalf("code=%d body=%s, want 413", rr.Code, rr.Body.String())
	}
	if events, _ := s.Store.Events(); len(events) != 0 {
		t.Fatal("oversized envelope was stored")
	}
}
