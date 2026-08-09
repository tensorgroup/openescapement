package cli

import (
	"bytes"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tensorgroup/openescapement/internal/publisher"
)

// setupGovernedRepoWithEndpoint is setupGovernedRepoWithReporting's sibling:
// a governed repo whose pack declares both reporting.amendments and
// reporting.endpoint, so `esc sync` actually has something to publish to.
// pack.Validate requires reporting.endpoint to start with "https://" (see
// pack.go's endpoint check, byte-parallel to update_check.endpoint), so
// endpoint must be an https:// URL — in practice an httptest.NewTLSServer.
func setupGovernedRepoWithEndpoint(t *testing.T, endpoint string) string {
	t.Helper()
	packRepo := newPackRepo(t, "1.0.0")
	writeFiles(t, packRepo, map[string]string{
		"org/pack.yaml": withManifestLines(t, packRepo, "reporting:\n  amendments: metrics\n  endpoint: "+endpoint+"\n"),
	})
	gitIn(t, packRepo, "add", ".")
	gitIn(t, packRepo, "commit", "-m", "declare reporting endpoint")
	gitIn(t, packRepo, "tag", "-f", "v1.0.0")
	return newGoverned(t, packRepo, "v1.0.0")
}

// runSplit runs the CLI with stdout and stderr captured separately, unlike
// run/runEscOut (cli_test.go, fixtures_test.go) which merge both into one
// buffer. Every other test in this package can afford that merge because
// its golden-path scenarios never write to stderr; this one specifically
// needs to assert stdout is untouched by publisher.Publish while stderr is
// allowed (and, on a failing endpoint, required) to differ, so merging the
// two streams would make Publish's own mandated stderr warning look like a
// stdout regression.
func runSplit(t *testing.T, root string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = Run(root, args, &out, &errOut)
	return out.String(), errOut.String(), code
}

// TestSyncPublishNeverAffectsOutcome is the non-fatality invariant's
// end-to-end proof (AGENTS.md: "publish is ALWAYS non-fatal ... never
// changes an exit code, never suppresses output"): `esc sync`'s exit code
// and stdout must be identical whether the configured reporting endpoint is
// up, refuses connections, or answers with a server error, and must match
// a control repo that declares no endpoint at all. Only stderr may differ
// (see runSplit's doc comment), and only on the two failing endpoints.
func TestSyncPublishNeverAffectsOutcome(t *testing.T) {
	up := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(202)
	}))
	defer up.Close()

	down := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	downURL := down.URL
	down.Close() // closed before use: nothing listens at downURL anymore

	erroring := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer erroring.Close()

	// The fixture servers use self-signed certificates; swap in a client
	// that trusts them for this test's duration only (see publisher.Client's
	// doc comment, which names this exact seam).
	origClient := publisher.Client
	publisher.Client = &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	t.Cleanup(func() { publisher.Client = origClient })

	control := setupGovernedRepo(t) // pack declares no reporting at all
	t.Setenv("ESC_PORTAL_TOKEN", "test-token")
	wantStdout, controlStderr, wantCode := runSplit(t, control, "sync")
	if controlStderr != "" {
		t.Fatalf("control repo (no endpoint) produced stderr, test setup is wrong:\n%s", controlStderr)
	}

	scenarios := []struct {
		name       string
		endpoint   string
		wantStderr bool // true iff Publish is expected to warn on this endpoint
	}{
		{"server up", up.URL, false},
		{"server down", downURL, true},
		{"server erroring", erroring.URL, true},
	}

	for _, tt := range scenarios {
		t.Run(tt.name, func(t *testing.T) {
			repo := setupGovernedRepoWithEndpoint(t, tt.endpoint)
			t.Setenv("ESC_PORTAL_TOKEN", "test-token")
			stdout, stderr, code := runSplit(t, repo, "sync")

			if code != wantCode {
				t.Errorf("exit code = %d, want %d (control)", code, wantCode)
			}
			if stdout != wantStdout {
				t.Errorf("stdout diverged from the no-endpoint control:\n--- got ---\n%s\n--- want ---\n%s", stdout, wantStdout)
			}
			hasStderr := stderr != ""
			if hasStderr != tt.wantStderr {
				t.Errorf("stderr present = %v, want %v; stderr:\n%s", hasStderr, tt.wantStderr, stderr)
			}
		})
	}
}
