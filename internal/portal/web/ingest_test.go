package web

import (
	"net/http/httptest"
	"strings"
	"testing"
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
