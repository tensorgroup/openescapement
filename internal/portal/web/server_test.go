package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/portal/store"
)

func newTestServer(t *testing.T, token string) *Server {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return New(st, nil, token, "test")
}

func get(t *testing.T, h http.Handler, path string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestPagesRenderWithoutAuth(t *testing.T) {
	h := newTestServer(t, "").Handler()
	for _, p := range []string{"/", "/fleet", "/packs", "/usage"} {
		rr := get(t, h, p, nil)
		if rr.Code != 200 {
			t.Fatalf("%s: code %d", p, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "esc <strong>portal</strong>") {
			t.Fatalf("%s: layout missing", p)
		}
	}
	if rr := get(t, h, "/static/style.css", nil); rr.Code != 200 {
		t.Fatalf("css: %d", rr.Code)
	}
}

func TestAuthRequired(t *testing.T) {
	h := newTestServer(t, "sekrit").Handler()
	if rr := get(t, h, "/", nil); rr.Code != 401 {
		t.Fatalf("unauth: %d", rr.Code)
	}
	if rr := get(t, h, "/", map[string]string{"Authorization": "Bearer sekrit"}); rr.Code != 200 {
		t.Fatalf("bearer: %d", rr.Code)
	}
	rr := get(t, h, "/?token=sekrit", nil)
	if rr.Code != 303 {
		t.Fatalf("token login: %d", rr.Code)
	}
	found := false
	for _, c := range rr.Result().Cookies() {
		if c.Name == "esc_session" && c.Value == "sekrit" && c.HttpOnly {
			found = true
		}
	}
	if !found {
		t.Fatal("session cookie not set")
	}
	if rr := get(t, h, "/?token=wrong", nil); rr.Code != 401 {
		t.Fatalf("bad token: %d", rr.Code)
	}
}

func TestUnknownRoute404(t *testing.T) {
	h := newTestServer(t, "").Handler()
	if rr := get(t, h, "/nope", nil); rr.Code != 404 {
		t.Fatalf("code %d", rr.Code)
	}
}
