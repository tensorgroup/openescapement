package publisher

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestPostEnvelopeClassifiesPermanentStatus pins the closed set of response
// statuses postEnvelope must treat as permanent: the server has said,
// unambiguously, that this exact payload will never be accepted, so
// FlushOutbox must be able to tell "give up on this one" apart from "try
// again later" via errors.Is(err, errPermanent).
func TestPostEnvelopeClassifiesPermanentStatus(t *testing.T) {
	for _, status := range []int{400, 404, 405, 413, 415, 422} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer srv.Close()

			err := postEnvelope(context.Background(), srv.URL, "tok", testEnvelope("x"))
			if err == nil {
				t.Fatal("want non-nil error")
			}
			if !errors.Is(err, errPermanent) {
				t.Errorf("status %d: err = %v, want errors.Is(err, errPermanent)", status, err)
			}
		})
	}
}

// TestPostEnvelopeClassifiesTransientStatus pins the complementary set: a
// caller-fixable-later condition (bad token, rate limit) or the server's own
// fault must stay ordinary errors, since errPermanent tells FlushOutbox to
// drop the entry forever and these are exactly the cases where retrying
// later is the correct behavior (a bad token fixed on a later run is what
// the outbox exists for).
func TestPostEnvelopeClassifiesTransientStatus(t *testing.T) {
	for _, status := range []int{500, 502, 429, 401, 403} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer srv.Close()

			err := postEnvelope(context.Background(), srv.URL, "tok", testEnvelope("x"))
			if err == nil {
				t.Fatal("want non-nil error")
			}
			if errors.Is(err, errPermanent) {
				t.Errorf("status %d: err = %v, want NOT errors.Is(err, errPermanent)", status, err)
			}
		})
	}
}

// TestFlushOutboxPermanentFailureDropsAndContinuesSameEndpoint is the
// poison-pill fix's core case: an oversized (or otherwise permanently
// rejected) entry must not head-of-line-block every later entry queued
// behind it on the SAME endpoint. The first entry gets 413 from the server
// (permanent); the second, queued behind it on the same endpoint, must
// still be attempted and must still succeed.
func TestFlushOutboxPermanentFailureDropsAndContinuesSameEndpoint(t *testing.T) {
	root := t.TempDir()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(202)
	}))
	defer srv.Close()

	if _, err := AppendOutbox(root, srv.URL, testEnvelope("poison"), base); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendOutbox(root, srv.URL, testEnvelope("good"), base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	send := func(endpoint string, e Envelope) error {
		return postEnvelope(context.Background(), endpoint, "tok", e)
	}

	sent, dropped, remaining, err := FlushOutbox(root, send, base.Add(time.Hour))
	if err != nil {
		t.Fatalf("FlushOutbox: %v", err)
	}
	if sent != 1 || dropped != 1 || remaining != 0 {
		t.Fatalf("sent=%d dropped=%d remaining=%d, want 1,1,0 (poison entry dropped, good entry still sent)", sent, dropped, remaining)
	}
	if calls != 2 {
		t.Fatalf("server received %d calls, want 2 (the second entry must still be attempted)", calls)
	}
	if lines := readOutboxLines(t, root); len(lines) != 0 {
		t.Errorf("outbox should be empty (poison dropped, good sent), got %d lines", len(lines))
	}
}

// TestFlushOutboxTransientFailureStillRetainsAndStops is the control case:
// a 500 (server's own fault, may well succeed on retry) must keep today's
// behavior exactly: stop the group and retain the failed entry plus
// everything queued behind it on that endpoint.
func TestFlushOutboxTransientFailureStillRetainsAndStops(t *testing.T) {
	root := t.TempDir()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	if _, err := AppendOutbox(root, srv.URL, testEnvelope("a"), base); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendOutbox(root, srv.URL, testEnvelope("b"), base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	send := func(endpoint string, e Envelope) error {
		return postEnvelope(context.Background(), endpoint, "tok", e)
	}

	sent, dropped, remaining, err := FlushOutbox(root, send, base.Add(time.Hour))
	if err == nil {
		t.Fatal("want non-nil err: server always 500s")
	}
	if sent != 0 || dropped != 0 || remaining != 2 {
		t.Fatalf("sent=%d dropped=%d remaining=%d, want 0,0,2 (transient failure retains both entries)", sent, dropped, remaining)
	}
	if lines := readOutboxLines(t, root); len(lines) != 2 {
		t.Errorf("outbox should still hold both entries, got %d lines", len(lines))
	}
}
