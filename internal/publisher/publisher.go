package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/engine"
	"github.com/tensorgroup/openescapement/internal/pack"
)

// errPermanent marks a postEnvelope failure the server will never accept on
// retry: a problem with this exact payload (see permanentStatus), not a
// transient outage. FlushOutbox distinguishes it via errors.Is rather than
// its own status-code inspection, so the classification lives in exactly
// one place. Kept unexported: both postEnvelope (which produces it) and
// FlushOutbox (which consumes it, in outbox.go) live in this package.
var errPermanent = errors.New("permanent send failure")

// permanentStatus reports whether status is one FlushOutbox should treat as
// permanent. These six mean the server has said, unambiguously, that this
// exact payload will never be accepted: retaining it would only requeue the
// same failure forever and head-of-line-block every later entry queued
// behind it on the same endpoint. Every other status, including network
// errors, 5xx, 429, 401, and 403, stays transient: FlushOutbox's existing
// stop-group-and-retain behavior is exactly the retry mechanism those need
// (a bad token fixed on a later run, or a server recovered from an outage,
// is what the outbox exists for).
func permanentStatus(status int) bool {
	switch status {
	case http.StatusBadRequest, http.StatusNotFound, http.StatusMethodNotAllowed,
		http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		return true
	default:
		return false
	}
}

// Envelope is the wire document a publisher transport sends: repo identity
// alongside the (possibly redacted, see Redact) Report. Defined here, ahead
// of the client that sends it, because Redact's test needs to marshal a
// Report exactly as production code will — inside the envelope, not bare —
// so a future field named "content" or "diff" added to Envelope itself
// falls under the same walk-test coverage as one added to Report. Task 5
// fills in the client that constructs and transmits an Envelope.
type Envelope struct {
	Schema     int            `json:"schema"`
	Remote     string         `json:"remote"`
	ConfigPath string         `json:"config_path"`
	Report     *engine.Report `json:"report"`
}

// EnvelopeSchema is the current wire schema for Envelope.
const EnvelopeSchema = 1

// tokenEnv names the environment variable a caller sets to authenticate
// with the portal. Kept as a constant rather than inlined so the token
// check and its error message can never drift from what Publish actually
// reads.
const tokenEnv = "ESC_PORTAL_TOKEN"

// Client is the HTTP client Publish uses to send envelopes. Exported as a
// test seam: production code never overwrites it, so it stays the 5s-timeout
// default with real TLS verification. Tests that need to reach an
// httptest.NewTLSServer's self-signed certificate — the only way to satisfy
// pack.Validate's https://-only rule for reporting.endpoint with a local
// server — swap in a client with InsecureSkipVerify for the test's
// duration and restore it after.
var Client = &http.Client{Timeout: 5 * time.Second}

// Publish sends rep to every distinct reporting endpoint declared across
// packs, if the resolved collection level permits it. It NEVER returns an
// error and NEVER panics on a transport failure: sync and status already
// wrote the lockfile and printed their own output by the time a caller
// reaches Publish (see the three call sites in internal/cli/cli.go), and
// nothing this function does may retroactively change that command's exit
// code. Every failure — connection refused, timeout, a non-2xx status, a
// missing token — is handled by queuing to the local outbox (or, for a
// missing token, not sending at all) and printing one line to stderr.
//
// Behavior:
//   - No pack declares a reporting.endpoint: return, silently.
//   - coll.Amendments == engine.ReportOff: return, silently. Collecting
//     endpoints first and checking Off second (rather than the reverse)
//     is what makes the "off sends nothing" case produce no stderr output
//     even when ESC_PORTAL_TOKEN is unset — an unconfigured token is only
//     ever worth a warning when something was actually about to be sent.
//   - ESC_PORTAL_TOKEN unset: print exactly one stderr warning and return.
//     Nothing is redacted, built, or queued — a missing token is a
//     configuration state to fix, not an outage to retry.
//   - Otherwise: redact rep once at coll.Amendments, build one Envelope,
//     queue it to every endpoint's outbox entry, then flush the whole
//     outbox oldest-first. Queuing before flushing (rather than sending
//     directly and only queuing on failure) is what makes a backlog from a
//     prior failed Publish call go out ahead of this call's fresh envelope,
//     in order, the moment the endpoint recovers — FlushOutbox's contract
//     already guarantees oldest-first delivery and per-entry endpoint
//     routing, so Publish does not need its own send-then-queue branch to
//     get that behavior. Every entry FlushOutbox hands to send is
//     re-redacted at coll.Amendments right before it goes out, since an
//     entry queued while the level was content (or under a pack that has
//     since changed) can sit in the outbox for up to OutboxMaxAge, and the
//     level a caller resolves NOW is the only one that may ever leave the
//     machine.
func Publish(ctx context.Context, root string, rep *engine.Report, packs []*pack.Pack, coll engine.Collection, stderr io.Writer, now time.Time) {
	endpoints := distinctEndpoints(packs)
	if len(endpoints) == 0 {
		return
	}
	if coll.Amendments == engine.ReportOff {
		return
	}
	token := os.Getenv(tokenEnv)
	if token == "" {
		fmt.Fprintf(stderr, "esc: reporting is configured but %s is not set; skipping publish\n", tokenEnv)
		return
	}

	remote := NormalizeRemote(RepoRemote(ctx, root))
	env := Envelope{
		Schema:     EnvelopeSchema,
		Remote:     remote,
		ConfigPath: repoRelConfigPath(root),
		Report:     Redact(rep, coll.Amendments),
	}

	var queued int
	appendDropped := 0
	for _, endpoint := range endpoints {
		dropped, err := AppendOutbox(root, endpoint, env, now)
		appendDropped += dropped
		if err != nil {
			fmt.Fprintf(stderr, "esc: queuing publish to %s failed: %v\n", endpoint, err)
			continue
		}
		queued++
	}
	if appendDropped > 0 {
		fmt.Fprintf(stderr, "esc: outbox dropped %d %s while queuing\n", appendDropped, plural(appendDropped, "entry", "entries"))
	}
	if queued == 0 {
		// Every AppendOutbox call failed; nothing was queued, so there is
		// nothing to flush.
		return
	}

	// Re-redact at coll.Amendments, the CURRENTLY resolved level, rather
	// than trusting whatever level the entry was queued at: an entry can
	// sit in the outbox for up to OutboxMaxAge, and the level an owner has
	// configured can be clamped down (or the endpoint's pack changed)
	// between when it was queued and when it finally flushes. Redacting
	// unconditionally, on every entry, is the simple correct move here:
	// Redact at content is identity and Redact below content is
	// idempotent, so re-applying it to an entry that was already redacted
	// at queue time changes nothing.
	send := func(endpoint string, e Envelope) error {
		e.Report = Redact(e.Report, coll.Amendments)
		return postEnvelope(ctx, endpoint, token, e)
	}
	sent, flushDropped, remaining, err := FlushOutbox(root, send, now)
	if flushDropped > 0 {
		fmt.Fprintf(stderr, "esc: outbox dropped %d %s during flush\n", flushDropped, plural(flushDropped, "entry", "entries"))
	}
	if err != nil {
		fmt.Fprintf(stderr, "esc: publish failed after %d sent, %d queued: %v\n", sent, remaining, err)
	}
}

// distinctEndpoints collects every non-empty reporting.endpoint declared
// across packs, deduplicated and in first-seen (pack) order.
func distinctEndpoints(packs []*pack.Pack) []string {
	seen := make(map[string]bool, len(packs))
	var endpoints []string
	for _, p := range packs {
		if p == nil || p.Manifest.Reporting == nil {
			continue
		}
		ep := p.Manifest.Reporting.Endpoint
		if ep == "" || seen[ep] {
			continue
		}
		seen[ep] = true
		endpoints = append(endpoints, ep)
	}
	return endpoints
}

// repoRelConfigPath returns the repo-relative slash-form path to the
// governed repo's config.yaml (e.g. ".escapement/config.yaml"), derived
// from config.Path rather than hardcoding the file name so the two can
// never drift apart.
func repoRelConfigPath(root string) string {
	rel, err := filepath.Rel(root, config.Path(root))
	if err != nil {
		// config.Path always returns root joined with a relative suffix, so
		// filepath.Rel(root, ...) cannot fail in practice; this fallback
		// only guards against a future change to that invariant.
		rel = config.Path(root)
	}
	return filepath.ToSlash(rel)
}

// postEnvelope POSTs e as JSON to endpoint, bearer-authenticated with
// token. A 2xx status is success; anything else — including a transport
// failure such as connection refused or a client-timeout — is reported as
// an error for the caller to queue and report on stderr.
func postEnvelope(ctx context.Context, endpoint, token string, e Envelope) error {
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck // draining lets the connection be reused; a copy error here does not change the outcome

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := fmt.Errorf("publish to %s: unexpected status %d", endpoint, resp.StatusCode)
		if permanentStatus(resp.StatusCode) {
			return fmt.Errorf("%w: %v", errPermanent, err)
		}
		return err
	}
	return nil
}

// plural returns one when n == 1, many otherwise. Mirrors internal/cli's
// helper of the same name and shape; kept as its own unexported copy for
// the same reason identity.go's gitCommand is (see its doc comment): this
// package must not depend on internal/cli.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
