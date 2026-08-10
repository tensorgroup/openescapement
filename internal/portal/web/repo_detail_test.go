package web

import (
	"net/http"
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

// TestRepoDetailItemsOnlyAmendmentIsNotWithheld covers the review finding:
// an items-only amendment (kind=dir/json-keys) always has empty Content by
// construction (engine.Amendment: Content carries block text, Items carries
// discrete names), regardless of reporting level. It must render its items
// as a complete, visible amendment, never the withheld copy.
func TestRepoDetailItemsOnlyAmendmentIsNotWithheld(t *testing.T) {
	evt := &store.Event{
		TS: time.Now(), Kind: "status", RepoID: "r1", TeamID: "ligo", Drift: "in-sync",
		// Collection is deliberately "content" level: even at the highest
		// level, an items-only amendment's Content is still "".
		Collection: &engine.Collection{Amendments: "content", Source: "pack"},
		Artifacts: []store.EventArtifact{{
			Path: ".claude/skills/esc-reconcile", Kind: engine.KindDir, Managed: "in-sync", Local: "amended",
			Amendment: &engine.Amendment{Bytes: 30, Lines: 2, Hash: "items123", Items: []string{"NOTES.md", "scratch.sh"}},
		}},
	}
	h := newTestServerWithRepo(t, evt).Handler()
	body := get(t, h, "/fleet/r1", nil).Body.String()
	for _, want := range []string{"NOTES.md", "scratch.sh", "30 bytes, 2 lines, 2 items"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q: %s", want, body)
		}
	}
	if strings.Contains(body, "content withheld by") {
		t.Fatalf("an items-only amendment must never render as withheld: %s", body)
	}
	if strings.Contains(body, "No amendments.") {
		t.Fatalf("an items-only amendment is an amendment: %s", body)
	}
}

// TestRepoDetailItemsOnlyAmendmentBelowContentLevelIsStillNotWithheld pins
// the same invariant when the reporting level is metrics, not content: an
// items-only amendment carries its items regardless of level (Items,
// unlike Content, isn't redacted), so it's still complete, not withheld.
func TestRepoDetailItemsOnlyAmendmentBelowContentLevelIsStillNotWithheld(t *testing.T) {
	evt := &store.Event{
		TS: time.Now(), Kind: "status", RepoID: "r1", TeamID: "ligo", Drift: "in-sync",
		Collection: &engine.Collection{Amendments: "metrics", Source: "repo-override"},
		Artifacts: []store.EventArtifact{{
			Path: ".mcp.json", Kind: engine.KindJSONKeys, Managed: "in-sync", Local: "amended",
			Amendment: &engine.Amendment{Bytes: 12, Lines: 1, Hash: "items456", Items: []string{"team-server"}},
		}},
	}
	h := newTestServerWithRepo(t, evt).Handler()
	body := get(t, h, "/fleet/r1", nil).Body.String()
	if !strings.Contains(body, "team-server") {
		t.Fatalf("items not rendered: %s", body)
	}
	if strings.Contains(body, "content withheld by") {
		t.Fatalf("an items-only amendment must never render as withheld, even below content level: %s", body)
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

// TestFleetAxisLabelsNameBothAxes pins the axis-legibility fixes: the fleet
// table's local-tampering column is named (not a bare "State"), both
// sortable/non-sortable axis headers carry a title tooltip, and the page
// explainer names both axes in plain language.
func TestFleetAxisLabelsNameBothAxes(t *testing.T) {
	h := newTestServer(t, "").Handler()
	body := get(t, h, "/fleet", nil).Body.String()
	for _, want := range []string{
		">Local edits<",
		`title="Currency: whether the repo's applied rule pack is up to date."`,
		`title="Local tampering: whether anyone has hand-edited managed content or added their own notes."`,
		"Status is whether its rulebook is current, and Local edits is whether anyone has hand-edited or added to it.",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q: %s", want, body)
		}
	}
	if strings.Contains(body, ">State<") {
		t.Fatalf("bare, unnamed axis header should be gone: %s", body)
	}
}

// TestRepoDetailAxesAreLabeled pins the "unlabeled pill pairs" UI fix: both
// the repo-level (Status/Local edits) and artifact-level (Managed/Local)
// pill pairs render inside a definition list naming each axis, so the page
// is self-explanatory without relying on a reader already knowing the
// governance vocabulary.
func TestRepoDetailAxesAreLabeled(t *testing.T) {
	evt := &store.Event{
		TS: time.Now(), Kind: "status", RepoID: "r1", TeamID: "ligo", Drift: "in-sync",
		Artifacts: []store.EventArtifact{{Path: "CLAUDE.md", Kind: engine.KindBlock, Managed: "in-sync", Local: "none"}},
	}
	h := newTestServerWithRepo(t, evt).Handler()
	body := get(t, h, "/fleet/r1", nil).Body.String()
	for _, want := range []string{"<dt>Status</dt>", "<dt>Local edits</dt>", "<dt>Managed</dt>", "<dt>Local</dt>"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q: %s", want, body)
		}
	}
}

// TestFleetRegisterSelectHasPlaceholderOption pins the register-form safety
// fix: a disabled, selected placeholder option means a bare click-submit
// (without explicitly choosing a repo) cannot silently bind the first repo
// in the list.
func TestFleetRegisterSelectHasPlaceholderOption(t *testing.T) {
	reg := store.Registry{Repos: []store.Repo{{ID: "r1", Name: "ligo-pipeline"}}}
	s := newTestServerWithRegistry(t, "", reg)
	remote := "https://github.com/acme/shadow-repo"
	if err := s.Store.AppendEvent(store.Event{TS: time.Now(), Kind: "mcp_connect", Remote: remote}); err != nil {
		t.Fatal(err)
	}
	body := get(t, s.Handler(), "/fleet", nil).Body.String()
	if !strings.Contains(body, `<option value="" disabled selected>Choose repo`) {
		t.Fatalf("missing disabled placeholder option: %s", body)
	}
}

// TestFleetRegisterMissingRepoRendersInPageBanner pins the error-handling
// fix: a validation failure re-renders the fleet page (full chrome, correct
// status code) with the message as a banner, instead of a bare http.Error
// that would dump the admin out to a plaintext response.
func TestFleetRegisterMissingRepoRendersInPageBanner(t *testing.T) {
	h := newTestServerWithRepo(t, nil).Handler()
	form := url.Values{"remote": {"https://github.com/acme/shadow-repo"}, "repo_id": {""}}
	req := httptest.NewRequest("POST", "/fleet/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "<html") || !strings.Contains(body, "esc <strong>portal</strong>") {
		t.Fatalf("error response must stay inside the portal chrome: %s", body)
	}
	if !strings.Contains(body, `class="error"`) || !strings.Contains(body, "Choose a repo before registering a remote.") {
		t.Fatalf("missing in-page error banner: %s", body)
	}
}

// TestFleetRegisterRejectsRemoteNotCurrentlyUnregistered covers the parked
// minor closed in this rework: a remote that was never seen in an
// unregistered event (e.g. a stale form resubmission after someone else
// already registered it) must be rejected, not silently bound.
func TestFleetRegisterRejectsRemoteNotCurrentlyUnregistered(t *testing.T) {
	h := newTestServerWithRepo(t, nil).Handler()
	form := url.Values{"remote": {"https://github.com/acme/never-seen"}, "repo_id": {"r1"}}
	req := httptest.NewRequest("POST", "/fleet/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "no longer unregistered") {
		t.Fatalf("missing stale-remote banner: %s", rr.Body.String())
	}
}

// TestFleetRegisterRejectsRemoteAlreadyBoundToAnotherRepo covers the
// register-integrity gap: events for a bound remote never gain a RepoID
// retroactively, so the stillUnregistered gate must consult the registry,
// not just events, or a second bind attempt for the same remote would
// succeed and leave RepoByRemote picking between two repos arbitrarily.
func TestFleetRegisterRejectsRemoteAlreadyBoundToAnotherRepo(t *testing.T) {
	reg := store.Registry{Repos: []store.Repo{
		{ID: "r1", Name: "ligo-pipeline"},
		{ID: "r2", Name: "campus-portal"},
	}}
	s := newTestServerWithRegistry(t, "", reg)
	remote := "https://github.com/acme/shadow-repo"
	if err := s.Store.AppendEvent(store.Event{TS: time.Now(), Kind: "mcp_connect", Remote: remote}); err != nil {
		t.Fatal(err)
	}
	h := s.Handler()

	form1 := url.Values{"remote": {remote}, "repo_id": {"r1"}}
	req1 := httptest.NewRequest("POST", "/fleet/register", strings.NewReader(form1.Encode()))
	req1.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr1 := httptest.NewRecorder()
	h.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusSeeOther {
		t.Fatalf("first register code=%d body=%s", rr1.Code, rr1.Body.String())
	}

	// The bound remote's events still carry no RepoID (ingest resolution is
	// not retroactive), so only excluding registry-bound remotes keeps it
	// out of the unregistered bucket.
	events, err := s.Store.Events()
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range store.UnregisteredRemotes(s.Store.Registry(), events) {
		if u.Remote == remote {
			t.Fatalf("remote %q still listed as unregistered after binding", remote)
		}
	}

	form2 := url.Values{"remote": {remote}, "repo_id": {"r2"}}
	req2 := httptest.NewRequest("POST", "/fleet/register", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusBadRequest {
		t.Fatalf("second register code=%d body=%s, want %d (remote already bound to r1)", rr2.Code, rr2.Body.String(), http.StatusBadRequest)
	}
	if got, _ := s.Store.Registry().RepoByRemote(remote); got.ID != "r1" {
		t.Fatalf("remote must stay bound to r1, got %+v", got)
	}
}

// TestFleetRegisterRejectsAlreadyRegisteredRepo covers the conflict path:
// registering against a repo that already has a Remote must fail with 409
// and an in-page banner, not silently overwrite the existing binding.
func TestFleetRegisterRejectsAlreadyRegisteredRepo(t *testing.T) {
	reg := store.Registry{Repos: []store.Repo{{ID: "r1", Name: "ligo-pipeline", Remote: "github.com/acme/existing"}}}
	s := newTestServerWithRegistry(t, "", reg)
	remote := "https://github.com/acme/shadow-repo"
	if err := s.Store.AppendEvent(store.Event{TS: time.Now(), Kind: "mcp_connect", Remote: remote}); err != nil {
		t.Fatal(err)
	}
	form := url.Values{"remote": {remote}, "repo_id": {"r1"}}
	req := httptest.NewRequest("POST", "/fleet/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	if got, _ := s.Store.Registry().RepoByRemote("github.com/acme/existing"); got.ID != "r1" {
		t.Fatalf("existing binding must survive a rejected re-registration: %+v", got)
	}
}
