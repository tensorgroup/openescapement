package web

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/tensorgroup/openescapement/internal/engine"
	"github.com/tensorgroup/openescapement/internal/portal/seed"
	"github.com/tensorgroup/openescapement/internal/portal/store"
)

// newTestServerWithRepo builds a server with one registry repo (r1, team
// ligo/Physics) and appends evt as its posture event (skipped when evt is
// the zero Event, for the "never synced" case). Returns the server so
// callers can append more events or inspect the store afterward.
func newTestServerWithRepo(t *testing.T, evt *store.Event) *Server {
	t.Helper()
	reg := store.Registry{
		Departments: []store.Department{{ID: "phys", Name: "Physics"}},
		Teams:       []store.Team{{ID: "ligo", Name: "LIGO Ops", DeptID: "phys"}},
		Repos:       []store.Repo{{ID: "r1", Name: "ligo-pipeline", TeamID: "ligo", Governed: true}},
	}
	s := newTestServerWithRegistry(t, "", reg)
	if evt != nil {
		if err := s.Store.AppendEvent(*evt); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestRepoDetail404ForUnknownRepo(t *testing.T) {
	h := newTestServerWithRepo(t, nil).Handler()
	if rr := get(t, h, "/fleet/nope", nil); rr.Code != 404 {
		t.Fatalf("code=%d", rr.Code)
	}
}

func TestRepoDetailNoEventYet(t *testing.T) {
	h := newTestServerWithRepo(t, nil).Handler()
	body := get(t, h, "/fleet/r1", nil).Body.String()
	if !strings.Contains(body, "ligo-pipeline") || !strings.Contains(body, "Physics") || !strings.Contains(body, "LIGO Ops") {
		t.Fatalf("missing repo/org placement: %s", body)
	}
	if !strings.Contains(body, "No posture events yet for this repo.") {
		t.Fatalf("missing never-synced message: %s", body)
	}
	if !strings.Contains(body, `class="pill ungoverned"`) {
		t.Fatalf("expected ungoverned pills: %s", body)
	}
}

func TestRepoDetailContentCase(t *testing.T) {
	evt := &store.Event{
		TS: time.Now(), Kind: "status", RepoID: "r1", TeamID: "ligo", Drift: "in-sync",
		Collection: &engine.Collection{Amendments: "content", Source: "pack"},
		Artifacts: []store.EventArtifact{{
			Path: "CLAUDE.md", Kind: engine.KindBlock, Managed: "in-sync", Local: "amended",
			Amendment: &engine.Amendment{Bytes: 42, Lines: 2, Hash: "abc123", Content: "## team notes\n\nExtra deploy step.\n"},
		}},
	}
	h := newTestServerWithRepo(t, evt).Handler()
	body := get(t, h, "/fleet/r1", nil).Body.String()
	if !strings.Contains(body, "## team notes") {
		t.Fatalf("amendment content not rendered: %s", body)
	}
	if !strings.Contains(body, "42 bytes, 2 lines") {
		t.Fatalf("amendment size not rendered: %s", body)
	}
	if strings.Contains(body, "content withheld by") {
		t.Fatalf("visible content must never render the withheld copy: %s", body)
	}
	if strings.Contains(body, "No amendments.") {
		t.Fatalf("a real amendment must not render the no-amendments copy: %s", body)
	}
}

// TestRepoDetailWithheldCase pins spec §6's exact-copy pattern: an amendment
// whose Content the publisher stripped (reporting level below content)
// renders "content withheld by <source>", naming the source verbatim.
func TestRepoDetailWithheldCase(t *testing.T) {
	evt := &store.Event{
		TS: time.Now(), Kind: "status", RepoID: "r1", TeamID: "ligo", Drift: "in-sync",
		Collection: &engine.Collection{Amendments: "metrics", Source: "repo-override"},
		Artifacts: []store.EventArtifact{{
			Path: "CLAUDE.md", Kind: engine.KindBlock, Managed: "in-sync", Local: "amended",
			Amendment: &engine.Amendment{Bytes: 61, Lines: 3, Hash: "def456"},
		}},
	}
	h := newTestServerWithRepo(t, evt).Handler()
	body := get(t, h, "/fleet/r1", nil).Body.String()
	if !strings.Contains(body, "content withheld by repo-override") {
		t.Fatalf("withheld copy missing or source not named: %s", body)
	}
	if strings.Contains(body, "No amendments.") {
		t.Fatalf("withheld case must never render identically to the no-amendments case: %s", body)
	}
}

// TestRepoDetailNoAmendmentsCase pins the third leg of spec §6's must:
// "no amendments" and "amendments withheld" never render identically.
func TestRepoDetailNoAmendmentsCase(t *testing.T) {
	evt := &store.Event{
		TS: time.Now(), Kind: "status", RepoID: "r1", TeamID: "ligo", Drift: "in-sync",
		Artifacts: []store.EventArtifact{{Path: "CLAUDE.md", Kind: engine.KindBlock, Managed: "in-sync", Local: "none"}},
	}
	h := newTestServerWithRepo(t, evt).Handler()
	body := get(t, h, "/fleet/r1", nil).Body.String()
	if !strings.Contains(body, "No amendments.") {
		t.Fatalf("no-amendments copy missing: %s", body)
	}
	if strings.Contains(body, "content withheld by") {
		t.Fatalf("no-amendments case must never render the withheld copy: %s", body)
	}
}

func TestRepoDetailAlteredArtifactShowsHashesAndDiff(t *testing.T) {
	evt := &store.Event{
		TS: time.Now(), Kind: "status", RepoID: "r1", TeamID: "ligo", Drift: "drifted",
		Artifacts: []store.EventArtifact{{
			Path: "AGENTS.md", Kind: engine.KindFile, Managed: "altered", Local: "none",
			Alteration: &engine.Alteration{ExpectedHash: "exp123", ActualHash: "act456", Diff: "@@ -1 +1 @@\n-old\n+new\n"},
		}},
	}
	h := newTestServerWithRepo(t, evt).Handler()
	body := get(t, h, "/fleet/r1", nil).Body.String()
	for _, want := range []string{"exp123", "act456", `class="pill altered"`, "line add", "line del"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q: %s", want, body)
		}
	}
}

func TestRepoDetailRendersFullPageWithoutHX(t *testing.T) {
	evt := &store.Event{TS: time.Now(), Kind: "status", RepoID: "r1", TeamID: "ligo", Drift: "in-sync"}
	h := newTestServerWithRepo(t, evt).Handler()
	body := get(t, h, "/fleet/r1", nil).Body.String()
	if !strings.Contains(body, "<html") || !strings.Contains(body, "esc <strong>portal</strong>") {
		t.Fatalf("repo detail must be a full page: %s", body)
	}
}

func TestFleetStateColumnRendersAllStatesFromSeed(t *testing.T) {
	dir := t.TempDir()
	epoch := time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
	if err := seed.Demo(dir, epoch); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	h := New(st, nil, "", "test").Handler()
	body := get(t, h, "/fleet", nil).Body.String()
	for _, want := range []string{
		`class="pill unadulterated"`, `class="pill augmented"`, `class="pill altered"`, `class="pill ungoverned"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("fleet state column missing %q", want)
		}
	}
}

func TestUnregisteredBucketListsGroupedRemotesAndRegisters(t *testing.T) {
	reg := store.Registry{
		Repos: []store.Repo{{ID: "r1", Name: "ligo-pipeline"}},
	}
	s := newTestServerWithRegistry(t, "", reg)
	remote := "https://github.com/acme/shadow-repo"
	if err := s.Store.AppendEvent(store.Event{
		TS: time.Now(), Kind: "mcp_connect", Remote: remote, AgentTool: "cursor",
	}); err != nil {
		t.Fatal(err)
	}
	h := s.Handler()

	body := get(t, h, "/fleet", nil).Body.String()
	if !strings.Contains(body, remote) {
		t.Fatalf("unregistered remote not listed: %s", body)
	}
	if !strings.Contains(body, `value="r1"`) {
		t.Fatalf("register target option missing: %s", body)
	}

	form := url.Values{"remote": {remote}, "repo_id": {"r1"}}
	req := httptest.NewRequest("POST", "/fleet/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 303 {
		t.Fatalf("register code=%d body=%s", rr.Code, rr.Body.String())
	}

	got, ok := s.Store.Registry().RepoByRemote(remote)
	if !ok || got.ID != "r1" {
		t.Fatalf("remote not bound to r1: %+v ok=%v", got, ok)
	}

	// The repo is no longer an available register target, and its remote's
	// activity should no longer render in the unregistered bucket text
	// (RepoID resolution happens at ingest time, not retroactively, so we
	// only assert the registry-side effect and the option list here).
	after := get(t, h, "/fleet", nil).Body.String()
	if strings.Contains(after, `value="r1"`) {
		t.Fatalf("registered repo should no longer be a register target: %s", after)
	}
}

func TestUnregisteredBucketEmptyState(t *testing.T) {
	h := newTestServer(t, "").Handler()
	body := get(t, h, "/fleet", nil).Body.String()
	if !strings.Contains(body, "No unregistered activity.") {
		t.Fatalf("missing empty state: %s", body)
	}
}
