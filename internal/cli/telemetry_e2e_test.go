package cli

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/portal/store"
	"github.com/tensorgroup/openescapement/internal/portal/web"
	"github.com/tensorgroup/openescapement/internal/publisher"
)

// insecureClientForSelfSignedTLS swaps publisher.Client for one that trusts
// self-signed certificates (httptest.NewTLSServer's default), restoring the
// original on cleanup. Every telemetry_e2e test needs it: pack.Validate
// requires reporting.endpoint to start with "https://" (see
// setupGovernedRepoWithEndpoint's doc comment in publish_integration_test.go),
// and a loopback portal in these tests is always an httptest.NewTLSServer.
func insecureClientForSelfSignedTLS(t *testing.T) {
	t.Helper()
	orig := publisher.Client
	publisher.Client = &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	t.Cleanup(func() { publisher.Client = orig })
}

// newTestPortal starts a real portal server (web.New + Server.Handler)
// behind an httptest.NewTLSServer on loopback, matching the pattern
// internal/portal/web/server_test.go's newTestServer uses, minus the
// TLS wrap (that server_test.go helper only needs plain httptest.NewRequest
// against the handler; here esc's real publisher.Client does a real network
// round trip, so a real listener is required). Returns the ingest URL
// (server.URL + the ingest path) and the data dir the store persists to, so
// assertions can re-open the store fresh (per Task 10's brief: assert
// through the store, not by intercepting HTTP).
func newTestPortal(t *testing.T, token string) (ingestURL, dataDir string) {
	t.Helper()
	dataDir = t.TempDir()
	st, err := store.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewTLSServer(web.New(st, nil, token, "test").Handler())
	t.Cleanup(srv.Close)
	return srv.URL + "/api/v1/events", dataDir
}

// setupGovernedRepoWithReportingEndpoint is setupGovernedRepoWithEndpoint's
// sibling (publish_integration_test.go), parameterized on the declared
// amendments level: that helper hardcodes "metrics", but this suite's
// content-level scenario needs "content" declared from the start.
func setupGovernedRepoWithReportingEndpoint(t *testing.T, level, endpoint string) string {
	t.Helper()
	packRepo := newPackRepo(t, "1.0.0")
	writeFiles(t, packRepo, map[string]string{
		"org/pack.yaml": withManifestLines(t, packRepo, "reporting:\n  amendments: "+level+"\n  endpoint: "+endpoint+"\n"),
	})
	gitIn(t, packRepo, "add", ".")
	gitIn(t, packRepo, "commit", "-m", "declare reporting endpoint")
	gitIn(t, packRepo, "tag", "-f", "v1.0.0")
	return newGoverned(t, packRepo, "v1.0.0")
}

// findArtifact returns the event artifact at path, and whether one was
// found.
func findArtifact(e store.Event, path string) (store.EventArtifact, bool) {
	for _, a := range e.Artifacts {
		if a.Path == path {
			return a, true
		}
	}
	return store.EventArtifact{}, false
}

// TestTelemetryE2EContentThenClamp is the spec §8 scenario over real parts:
// a real governed temp repo (existing fixtures) whose pack declares
// reporting.amendments: content and a loopback endpoint, publishing to a
// real portal server (web.New + httptest.NewTLSServer) on loopback. It
// covers the first two stages of the scenario:
//
//  1. Amend a governed file (append below the managed block), `esc sync`,
//     and confirm the event lands in the portal's store with augmented
//     state (artifact Local == amended) and the amendment CONTENT present,
//     because the pack declared level content.
//  2. Clamp the repo down to report_amendments: metrics and confirm a
//     fresh event carries the same amendment WITHOUT Content, and
//     Collection{metrics, repo-override}.
//
// Assertions read through the portal's own store (store.Open + Events()),
// never by intercepting HTTP, so the proof covers ingest's mapping (Kind,
// Artifacts, Collection, Drift), not just that a request was sent.
func TestTelemetryE2EContentThenClamp(t *testing.T) {
	insecureClientForSelfSignedTLS(t)
	const token = "e2e-token"
	ingestURL, dataDir := newTestPortal(t, token)

	repo := setupGovernedRepoWithReportingEndpoint(t, "content", ingestURL)
	t.Setenv("ESC_PORTAL_TOKEN", token)

	// First sync: establishes the managed files. No amendment yet, nothing
	// interesting to assert about this call beyond a clean exit.
	runEsc(t, repo, "sync")

	// Amend AGENTS.md below the managed block, matching
	// TestAmendedBlockReportsInSync's pattern (amendment_test.go): this is a
	// team appending its own content, not drift.
	agentsPath := filepath.Join(repo, "AGENTS.md")
	existing, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	const teamNote = "## Team rules\n\nBe kind.\n"
	if err := os.WriteFile(agentsPath, append(existing, []byte("\n"+teamNote)...), 0o644); err != nil {
		t.Fatal(err)
	}

	// Second sync: Apply leaves the amendment untouched (it's outside the
	// managed block), publishSyncResult then re-derives status and publishes
	// it, so this is the event that must carry the amendment.
	runEsc(t, repo, "sync")

	st, err := store.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	events, err := st.Events()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("no events landed in the portal store after sync with a configured endpoint and token")
	}
	contentEvent := events[len(events)-1]
	if contentEvent.Kind != "sync" {
		t.Errorf("event kind = %q, want sync", contentEvent.Kind)
	}
	art, found := findArtifact(contentEvent, "AGENTS.md")
	if !found {
		t.Fatalf("no AGENTS.md artifact in event: %+v", contentEvent)
	}
	if art.Local != "amended" {
		t.Errorf("AGENTS.md local axis = %q, want amended", art.Local)
	}
	if art.Amendment == nil || !strings.Contains(art.Amendment.Content, "Be kind.") {
		t.Errorf("amendment content missing at reporting level content: %+v", art.Amendment)
	}
	if contentEvent.Collection == nil || contentEvent.Collection.Amendments != "content" || contentEvent.Collection.Source != "pack" {
		t.Errorf("collection = %+v, want content/pack", contentEvent.Collection)
	}

	// Clamp the repo down to metrics.
	cfg, err := config.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ReportAmendments = "metrics"
	if err := cfg.Save(repo); err != nil {
		t.Fatal(err)
	}

	// Third sync: same amendment still on disk, but now clamped.
	runEsc(t, repo, "sync")

	events, err = st.Events()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) < 2 {
		t.Fatalf("want at least 2 events after the clamped sync, got %d", len(events))
	}
	metricsEvent := events[len(events)-1]
	art, found = findArtifact(metricsEvent, "AGENTS.md")
	if !found {
		t.Fatalf("no AGENTS.md artifact in clamped event: %+v", metricsEvent)
	}
	if art.Local != "amended" {
		t.Errorf("AGENTS.md local axis = %q, want amended (clamp only affects content, not the local axis)", art.Local)
	}
	if art.Amendment == nil {
		t.Fatalf("amendment metrics (bytes/lines/hash) must survive at metrics level: got nil")
	}
	if art.Amendment.Content != "" {
		t.Errorf("amendment content must be redacted at metrics level, got %q", art.Amendment.Content)
	}
	if metricsEvent.Collection == nil || metricsEvent.Collection.Amendments != "metrics" || metricsEvent.Collection.Source != "repo-override" {
		t.Errorf("collection = %+v, want metrics/repo-override", metricsEvent.Collection)
	}
}

// newReportingPackRepo builds a minimal git pack repo (one rule file, no
// frontmatter so it targets every recognized target file, matching
// packRepoFiles' rules/secrets.md) declaring reporting.amendments: metrics
// and endpoint. name must be distinct across packs used together in one
// governed repo's config.yaml, since two packs sharing a name is untested
// territory this suite has no reason to exercise.
func newReportingPackRepo(t *testing.T, name, endpoint string) string {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"org/pack.yaml": `schema: 1
name: ` + name + `
version: 1.0.0
description: fixture
rules:
  - rules/notes.md
reporting:
  amendments: metrics
  endpoint: ` + endpoint + `
`,
		"org/rules/notes.md": "## Notes\nHello from " + name + ".\n",
	})
	gitIn(t, dir, "init", "-q", "-b", "main")
	gitIn(t, dir, "config", "user.email", "t@e.com")
	gitIn(t, dir, "config", "user.name", "T")
	gitIn(t, dir, "config", "commit.gpgsign", "false")
	gitIn(t, dir, "config", "tag.gpgsign", "false")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "v1.0.0")
	gitIn(t, dir, "tag", "-a", "v1.0.0", "-m", "v1.0.0")
	return dir
}

// TestTelemetryE2ETwoPacksTwoEndpoints proves fan-out: a governed repo whose
// config.yaml pins two packs, each declaring its OWN distinct reporting
// endpoint, and one `esc sync` must deliver the envelope to both portals.
// distinctEndpoints (internal/publisher/publisher.go) already builds one
// outbox entry per distinct endpoint across all packs; this is that
// behavior's real-server proof.
func TestTelemetryE2ETwoPacksTwoEndpoints(t *testing.T) {
	insecureClientForSelfSignedTLS(t)
	const token = "e2e-token"
	urlA, dataDirA := newTestPortal(t, token)
	urlB, dataDirB := newTestPortal(t, token)

	packA := newReportingPackRepo(t, "pack-a", urlA)
	packB := newReportingPackRepo(t, "pack-b", urlB)

	repo := t.TempDir()
	writeFiles(t, repo, map[string]string{
		".escapement/config.yaml": `schema: 1
packs:
  - source: file://` + packA + `//org
    ref: v1.0.0
    trust: unsigned
  - source: file://` + packB + `//org
    ref: v1.0.0
    trust: unsigned
`,
	})
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	t.Setenv("ESC_PORTAL_TOKEN", token)

	runEsc(t, repo, "sync")

	for name, dataDir := range map[string]string{"portal A": dataDirA, "portal B": dataDirB} {
		st, err := store.Open(dataDir)
		if err != nil {
			t.Fatal(err)
		}
		events, err := st.Events()
		if err != nil {
			t.Fatal(err)
		}
		if len(events) == 0 {
			t.Errorf("%s: no events landed, want the shared envelope delivered to every distinct endpoint", name)
		}
	}
}

// TestTelemetryE2EEmptyTokenNoEventExitStaysZero is Task 5's
// non-fatality integration (TestSyncPublishNeverAffectsOutcome,
// publish_integration_test.go) re-asserted end to end through a real
// portal: an unset ESC_PORTAL_TOKEN must make Publish skip sending
// entirely (see Publish's doc comment: "ESC_PORTAL_TOKEN unset: print
// exactly one stderr warning and return"), so no event reaches the store,
// while `esc sync` itself still exits 0 — publish failure (or, here,
// publish non-attempt) never changes the exit code.
func TestTelemetryE2EEmptyTokenNoEventExitStaysZero(t *testing.T) {
	insecureClientForSelfSignedTLS(t)
	ingestURL, dataDir := newTestPortal(t, "e2e-token")

	repo := setupGovernedRepoWithReportingEndpoint(t, "metrics", ingestURL)
	t.Setenv("ESC_PORTAL_TOKEN", "") // deliberately unset

	code, out := run(t, repo, "sync")
	if code != 0 {
		t.Fatalf("sync with unset token: exit %d, want 0:\n%s", code, out)
	}

	st, err := store.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	events, err := st.Events()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Errorf("want no events with an unset ESC_PORTAL_TOKEN, got %d", len(events))
	}
}
