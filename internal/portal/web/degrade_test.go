package web

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// pageRoutes are the full-page GET routes the degradation contract covers.
var pageRoutes = []string{"/", "/fleet", "/usage", "/packs"}

func TestEveryRouteRendersFullPageWithoutHX(t *testing.T) {
	h := newTestServer(t, "").Handler()
	for _, p := range pageRoutes {
		body := get(t, h, p, nil).Body.String()
		if !strings.Contains(body, "<html") {
			t.Fatalf("%s: missing <html", p)
		}
		if !strings.Contains(body, "esc <strong>portal</strong>") {
			t.Fatalf("%s: missing sidebar", p)
		}
	}
}

func TestFleetAndUsageFragmentsRequireHXHeader(t *testing.T) {
	h := newTestServer(t, "").Handler()
	for _, p := range []string{"/fleet?sort=repo", "/usage"} {
		if full := get(t, h, p, nil).Body.String(); !strings.Contains(full, "<html") {
			t.Fatalf("%s without HX must be a full page", p)
		}
		frag := get(t, h, p, map[string]string{"HX-Request": "true"}).Body.String()
		if strings.Contains(frag, "<html") {
			t.Fatalf("%s with HX must be a fragment", p)
		}
	}
}

func TestDiffEndpointFragmentRequiresHXHeader(t *testing.T) {
	s := newTestServerWithPacks(t)
	form := url.Values{
		"frag":    {"rules/security.md"},
		"content": {"---\ntargets: [claude, agents]\n---\n# Security\n\n- Added rule.\n"},
		"version": {"1.3.0"},
		"action":  {"diff"},
	}
	post := func(hx bool) string {
		req := httptest.NewRequest("POST", "/packs/org-baseline/publish", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if hx {
			req.Header.Set("HX-Request", "true")
		}
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, req)
		return rr.Body.String()
	}
	if full := post(false); !strings.Contains(full, "<html") {
		t.Fatal("diff without HX must be a full page")
	}
	if frag := post(true); strings.Contains(frag, "<html") {
		t.Fatal("diff with HX must be a fragment")
	}
}
