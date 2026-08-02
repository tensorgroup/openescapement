package web

import (
	"crypto/tls"
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

func TestActiveNav(t *testing.T) {
	h := newTestServer(t, "").Handler()
	cases := map[string]string{
		"/":      `href="/" class="active"`,
		"/fleet": `href="/fleet" class="active"`,
		"/packs": `href="/packs" class="active"`,
		"/usage": `href="/usage" class="active"`,
	}
	for path, want := range cases {
		body := get(t, h, path, nil).Body.String()
		if !strings.Contains(body, want) {
			t.Fatalf("%s: missing active nav %q", path, want)
		}
	}
}

func TestSecurityHeadersOnEveryRoute(t *testing.T) {
	const wantCSP = "default-src 'none'; style-src 'self'; script-src 'self'; img-src 'self'; connect-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'"
	h := newTestServer(t, "").Handler()
	for _, p := range []string{"/", "/fleet", "/packs", "/usage", "/static/style.css", "/nope"} {
		rr := get(t, h, p, nil)
		if got := rr.Header().Get("Content-Security-Policy"); got != wantCSP {
			t.Fatalf("%s: CSP = %q", p, got)
		}
		if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Fatalf("%s: nosniff = %q", p, got)
		}
	}
	// Unauthorized responses still carry the headers.
	ha := newTestServer(t, "sekrit").Handler()
	if rr := get(t, ha, "/", nil); rr.Code != 401 || rr.Header().Get("Content-Security-Policy") != wantCSP {
		t.Fatalf("401 missing CSP: code=%d", rr.Code)
	}
}

func TestSessionCookieHardening(t *testing.T) {
	h := newTestServer(t, "sekrit").Handler()

	// Plain HTTP: Strict + HttpOnly, not Secure.
	rr := get(t, h, "/?token=sekrit", nil)
	c := findCookie(t, rr, "esc_session")
	if c.SameSite != http.SameSiteStrictMode || !c.HttpOnly || c.Secure {
		t.Fatalf("http cookie: samesite=%v httponly=%v secure=%v", c.SameSite, c.HttpOnly, c.Secure)
	}

	// TLS: Secure set.
	req := httptest.NewRequest("GET", "/?token=sekrit", nil)
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !findCookie(t, rec, "esc_session").Secure {
		t.Fatal("tls cookie must be Secure")
	}
}

func TestModernCSSMarkup(t *testing.T) {
	h := newTestServer(t, "").Handler()

	layout := get(t, h, "/", nil).Body.String()
	if !strings.Contains(layout, `class="skip"`) || !strings.Contains(layout, `id="main"`) {
		t.Fatal("layout missing skip link / main landmark")
	}

	fleet := get(t, h, "/fleet", nil).Body.String()
	if !strings.Contains(fleet, `class="segmented"`) || !strings.Contains(fleet, `class="seg active"`) {
		t.Fatal("fleet filters not segmented")
	}

	usage := get(t, h, "/usage", nil).Body.String()
	if !strings.Contains(usage, `class="segmented"`) {
		t.Fatal("usage day filters not segmented")
	}

	css := get(t, h, "/static/style.css", nil).Body.String()
	for _, want := range []string{"@view-transition", "prefers-reduced-motion", ":focus-visible", "@media (max-width: 700px)", ".htmx-indicator"} {
		if !strings.Contains(css, want) {
			t.Fatalf("style.css missing %q", want)
		}
	}
}

func TestHtmxAssetsServed(t *testing.T) {
	h := newTestServer(t, "").Handler()

	js := get(t, h, "/static/htmx.min.js", nil)
	if js.Code != 200 || len(js.Body.String()) < 1000 {
		t.Fatalf("htmx.min.js: code=%d len=%d", js.Code, js.Body.Len())
	}

	cfg := get(t, h, "/static/htmx-config.js", nil).Body.String()
	for _, want := range []string{"allowEval = false", "includeIndicatorStyles = false", "historyEnabled = false", "selfRequestsOnly = true"} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("htmx-config.js missing %q", want)
		}
	}

	layout := get(t, h, "/", nil).Body.String()
	if !strings.Contains(layout, `src="/static/htmx.min.js"`) || !strings.Contains(layout, `src="/static/htmx-config.js"`) {
		t.Fatal("layout missing htmx script tags")
	}
}

func TestPackMarkdownHasHxDisable(t *testing.T) {
	h := newTestServerWithPacks(t).Handler()
	detail := get(t, h, "/packs/org-baseline", nil).Body.String()
	if !strings.Contains(detail, "hx-disable") {
		t.Fatal("pack fragments must be wrapped with hx-disable")
	}
}

func findCookie(t *testing.T, rr *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, c := range rr.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("cookie %q not set", name)
	return nil
}
