package web

import (
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/portal/publish"
	"github.com/tensorgroup/openescapement/internal/portal/store"
)

// packGit runs git in dir for test fixture setup, failing the test on error.
func packGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// newPackClone builds the Task 8 fixture shape: a single "org-baseline" pack
// clone at v1.2.0 with one fragment, under a fresh packs directory. Mirrors
// internal/portal/publish's newPackClone helper (unexported there, so
// duplicated here for this package's tests).
func newPackClone(t *testing.T) (mgrDir string) {
	t.Helper()
	mgrDir = t.TempDir()
	dir := filepath.Join(mgrDir, "org-baseline")
	if err := os.MkdirAll(filepath.Join(dir, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"pack.yaml":         "schema: 1\nname: org-baseline\nversion: 1.2.0\ndescription: Org baseline rules\nrules:\n  - rules/security.md\n",
		"rules/security.md": "---\ntargets: [claude, agents]\n---\n# Security\n\n- Never commit secrets.\n",
	}
	for p, c := range files {
		if err := os.WriteFile(filepath.Join(dir, p), []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	packGit(t, dir, "init", "-q", "-b", "main")
	packGit(t, dir, "config", "user.email", "t@t")
	packGit(t, dir, "config", "user.name", "t")
	packGit(t, dir, "config", "commit.gpgsign", "false")
	packGit(t, dir, "config", "tag.gpgsign", "false")
	packGit(t, dir, "add", "-A")
	packGit(t, dir, "commit", "-q", "-m", "init")
	packGit(t, dir, "tag", "-a", "v1.2.0", "-m", "v1.2.0")
	return mgrDir
}

// newTestServerWithPacks builds a Server backed by a real publish.Manager
// over the Task 8 fixture clone, for tests that exercise the packs pages.
func newTestServerWithPacks(t *testing.T) *Server {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	packs := publish.NewManager(newPackClone(t))
	return New(st, packs, "", "test")
}

func TestPacksListAndDetail(t *testing.T) {
	s := newTestServerWithPacks(t)
	h := s.Handler()
	list := get(t, h, "/packs", nil).Body.String()
	if !strings.Contains(list, "org-baseline") || !strings.Contains(list, "1.2.0") {
		t.Fatalf("list: %s", list)
	}
	detail := get(t, h, "/packs/org-baseline", nil).Body.String()
	for _, want := range []string{"v1.2.0", "rules/security.md", "<h2>Security</h2>", "Edit"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("detail missing %q", want)
		}
	}
	if rr := get(t, h, "/packs/nope", nil); rr.Code != 404 {
		t.Fatalf("unknown pack: %d", rr.Code)
	}
}

func TestEditAndPublishFlow(t *testing.T) {
	s := newTestServerWithPacks(t)
	h := s.Handler()
	edit := get(t, h, "/packs/org-baseline/edit?frag=rules/security.md", nil).Body.String()
	if !strings.Contains(edit, "<textarea") || !strings.Contains(edit, `value="1.3.0"`) {
		t.Fatalf("edit page: %s", edit)
	}
	form := url.Values{
		"frag":    {"rules/security.md"},
		"content": {"---\ntargets: [claude, agents]\n---\n# Security\n\n- New rule.\n"},
		"version": {"1.3.0"},
		"action":  {"publish"},
	}
	req := httptest.NewRequest("POST", "/packs/org-baseline/publish", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 303 || rr.Header().Get("Location") != "/packs/org-baseline?published=v1.3.0" {
		t.Fatalf("publish: %d %s %s", rr.Code, rr.Header().Get("Location"), rr.Body.String())
	}
	detail := get(t, h, "/packs/org-baseline?published=v1.3.0", nil).Body.String()
	if !strings.Contains(detail, "v1.3.0") || !strings.Contains(detail, "Published") {
		t.Fatal("new version not visible")
	}
}

func TestDiffColoring(t *testing.T) {
	s := newTestServerWithPacks(t)
	form := url.Values{
		"frag":    {"rules/security.md"},
		"content": {"---\ntargets: [claude, agents]\n---\n# Security\n\n- Added rule.\n"},
		"version": {"1.3.0"},
		"action":  {"diff"},
	}
	req := httptest.NewRequest("POST", "/packs/org-baseline/publish", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("diff render: %d %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, class := range []string{`class="line add"`, `class="line del"`, `class="line meta"`} {
		if !strings.Contains(body, class) {
			t.Fatalf("%s not present:\n%s", class, body)
		}
	}
}

func TestDiffPreviewFragment(t *testing.T) {
	s := newTestServerWithPacks(t)
	form := url.Values{
		"frag":    {"rules/security.md"},
		"content": {"---\ntargets: [claude, agents]\n---\n# Security\n\n- Added rule.\n"},
		"version": {"1.3.0"},
		"action":  {"diff"},
	}
	newReq := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/packs/org-baseline/publish", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, req)
		return rr
	}
	frag := newReq().Body.String()
	if strings.Contains(frag, "<html") {
		t.Fatal("HX diff response must be a fragment, not a full page")
	}
	if !strings.Contains(frag, `class="line add"`) {
		t.Fatalf("fragment missing diff: %s", frag)
	}
}

func TestPublishValidationErrorKeepsContent(t *testing.T) {
	s := newTestServerWithPacks(t)
	form := url.Values{
		"frag":    {"rules/security.md"},
		"content": {"---\ntargets: [nonsense]\n---\nbroken\n"},
		"version": {"1.3.0"},
		"action":  {"publish"},
	}
	req := httptest.NewRequest("POST", "/packs/org-baseline/publish", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != 422 {
		t.Fatalf("code %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "nonsense") || !strings.Contains(body, "class=\"error\"") {
		t.Fatal("error page must preserve content and show error")
	}
}
