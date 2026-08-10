// Package web implements the esc admin portal HTTP server: layout, static
// assets, token auth, and (in later tasks) the overview/fleet/packs/usage
// pages and the telemetry ingest endpoint.
package web

import (
	"crypto/subtle"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tensorgroup/openescapement/internal/engine"
	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/portal/charts"
	"github.com/tensorgroup/openescapement/internal/portal/publish"
	"github.com/tensorgroup/openescapement/internal/portal/store"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

const sessionCookie = "esc_session"

// Server holds the portal's dependencies and routes. Packs (the publish
// manager) may be nil, e.g. before Task 10 wires it up from cmdServe — the
// packs pages then behave as if no pack repos are configured.
type Server struct {
	Store   *store.Store
	Packs   *publish.Manager // nil = no pack repos configured
	Token   string           // "" = auth disabled (demo mode)
	Version string
	Now     func() time.Time // injectable clock for tests; default time.Now

	// GuidanceDir is <data-dir>/guidance ("" = embedded guidance only, used
	// by tests and any caller that has not seeded a data dir).
	GuidanceDir string

	layout *template.Template
	pages  map[string]*template.Template
}

// pageNames are the page templates parsed at startup. Each defines the
// "title", "explainer", and "content" blocks that override the layout.
var pageNames = []string{"overview", "fleet", "repo_detail", "packs", "pack", "pack_edit", "usage", "models", "model_vendor", "model_edit", "model_adopt"}

// New builds a Server with its templates parsed and ready to serve.
func New(st *store.Store, packs *publish.Manager, token, version string) *Server {
	s := &Server{
		Store:   st,
		Packs:   packs,
		Token:   token,
		Version: version,
		Now:     time.Now,
	}
	s.layout = template.Must(template.New("layout.html").Funcs(template.FuncMap{
		"abbrev":    abbrevTokens,
		"fmtTime":   fmtLastSync,
		"diffLines": diffLines,
		"host":      urlHost,
	}).ParseFS(templateFS, "templates/layout.html"))
	s.pages = make(map[string]*template.Template, len(pageNames))
	for _, name := range pageNames {
		t := template.Must(s.layout.Clone())
		t = template.Must(t.ParseFS(templateFS, "templates/"+name+".html"))
		s.pages[name] = t
	}
	return s
}

// layoutData is the template data every page renders with today; pages that
// need more will wrap or extend this in later tasks.
type layoutData struct {
	Version string
	Page    string // active nav key: "overview", "fleet", "packs", "usage"
	Org     string // organization name, pinned in the sidebar footer
}

func (s *Server) baseData(page string) layoutData {
	org := ""
	if s.Store != nil {
		org = s.Store.Registry().Org.Name
	}
	return layoutData{Version: s.Version, Page: page, Org: org}
}

// abbrevTokens adapts charts.Abbrev (float64) to the int64 token counts the
// overview template passes it, so it can be registered directly as the
// "abbrev" template func.
func abbrevTokens(v int64) string { return charts.Abbrev(float64(v)) }

// fmtLastSync formats a repo's last-sync timestamp for the fleet table:
// "Jan 2 15:04", or an em-dash for a repo that has never synced.
func fmtLastSync(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("Jan 2 15:04")
}

// diffLine is one line of a unified diff, classed for template coloring.
type diffLine struct {
	Class string // "add", "del", "meta", or "" (context)
	Text  string
}

// diffLines splits a `git diff --no-index` unified diff into classed lines:
// header lines ("diff ", "index ", "---", "+++", "@@") as meta, "+" adds and
// "-" removals colored, everything else unclassed context. Text is rendered
// through html/template, so it is auto-escaped.
func diffLines(s string) []diffLine {
	if s == "" {
		return nil
	}
	raw := strings.Split(strings.TrimRight(s, "\n"), "\n")
	out := make([]diffLine, 0, len(raw))
	for _, ln := range raw {
		class := ""
		switch {
		case strings.HasPrefix(ln, "+++"), strings.HasPrefix(ln, "---"),
			strings.HasPrefix(ln, "@@"), strings.HasPrefix(ln, "diff "),
			strings.HasPrefix(ln, "index "):
			class = "meta"
		case strings.HasPrefix(ln, "+"):
			class = "add"
		case strings.HasPrefix(ln, "-"):
			class = "del"
		}
		out = append(out, diffLine{Class: class, Text: ln})
	}
	return out
}

// overviewData extends layoutData with the stats and chart the overview
// page's content block renders.
type overviewData struct {
	layoutData
	Stats         store.OverviewStats
	AdoptionChart template.HTML
	ModelVendor   map[string]string
}

// render executes the named page template against the shared layout with a
// 200 OK status.
func (s *Server) render(w http.ResponseWriter, page string, data any) {
	s.renderStatus(w, http.StatusOK, page, data)
}

// renderStatus is like render but with an explicit status code, for pages
// that render a non-200 response body (e.g. a 422 validation error) rather
// than a plain http.Error.
func (s *Server) renderStatus(w http.ResponseWriter, status int, page string, data any) {
	t, ok := s.pages[page]
	if !ok {
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := t.ExecuteTemplate(w, "layout.html", data); err != nil {
		// Status and headers are already written; log only, we can't
		// change the response now.
		log.Printf("portal: internal error rendering %s: %v", page, err)
	}
}

// isHX reports whether the request came from htmx, which sets HX-Request: true
// on every AJAX request. Handlers use it to return a fragment instead of a
// full page.
func isHX(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }

// renderFragment executes a single named template block (a swappable region)
// from the given page's template set, for htmx partial swaps. Unlike render it
// emits no <html> shell.
func (s *Server) renderFragment(w http.ResponseWriter, page, block string, data any) {
	t, ok := s.pages[page]
	if !ok {
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, block, data); err != nil {
		log.Printf("portal: internal error rendering fragment %s/%s: %v", page, block, err)
	}
}

// serverError logs the underlying error server-side and returns a generic
// 500 to the client. Internal error text (e.g. absolute file paths from
// store I/O errors) must never reach an HTTP response body.
func serverError(w http.ResponseWriter, err error) {
	log.Printf("portal: internal error: %v", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}

// Handler builds the routed, auth-wrapped HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", s.handleOverview)
	mux.HandleFunc("GET /fleet", s.handleFleet)
	mux.HandleFunc("POST /fleet/register", s.handleFleetRegister)
	mux.HandleFunc("GET /fleet/{repoID}", s.handleRepoDetail)
	mux.HandleFunc("GET /packs", s.handlePacks)
	mux.HandleFunc("GET /packs/{name}", s.handlePackDetail)
	mux.HandleFunc("GET /packs/{name}/edit", s.handlePackEdit)
	mux.HandleFunc("POST /packs/{name}/publish", s.handlePackPublish)
	mux.HandleFunc("GET /usage", s.handleUsage)
	mux.HandleFunc("GET /models", s.handleModels)
	mux.HandleFunc("GET /models/{vendor}", s.handleModelVendor)
	mux.HandleFunc("GET /models/{vendor}/edit", s.handleModelEdit)
	mux.HandleFunc("POST /models/{vendor}/edit", s.handleModelEditSave)
	mux.HandleFunc("GET /models/{vendor}/adopt", s.handleModelAdopt)
	mux.HandleFunc("POST /models/{vendor}/adopt", s.handleModelAdoptSave)
	mux.HandleFunc("POST /api/v1/events", s.handleEvents)
	mux.Handle("/static/", noStore(http.FileServerFS(staticFS)))

	return securityHeaders(s.withAuth(mux))
}

// noStore sets Cache-Control: no-store on static asset responses so a
// stale-cached style.css never survives a portal upgrade.
func noStore(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}

// contentSecurityPolicy is the strict CSP applied to every portal response.
// default-src 'none' denies everything not explicitly allowed; connect-src
// 'self' is required for htmx XHR (it does not fall back from default-src).
const contentSecurityPolicy = "default-src 'none'; style-src 'self'; script-src 'self'; img-src 'self'; connect-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'"

// securityHeaders sets the CSP and nosniff header on every response. It wraps
// the entire handler, so pages, static assets, redirects, and error responses
// all carry the same baseline.
func securityHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		h.ServeHTTP(w, r)
	})
}

// withAuth enforces the token auth model. Token == "" disables auth
// entirely (demo mode). /static/ is always exempt. Otherwise: cookie
// esc_session == Token, or header Authorization: Bearer <Token>, grants
// access. A ?token=<Token> query sets the session cookie and redirects to
// the same path with the query stripped; any other query token is
// unauthorized.
func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.Token == "" || strings.HasPrefix(r.URL.Path, "/static/") {
			next.ServeHTTP(w, r)
			return
		}

		if tok := r.URL.Query().Get("token"); tok != "" {
			if !constantTimeEqual(tok, s.Token) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name:     sessionCookie,
				Value:    s.Token,
				Path:     "/",
				HttpOnly: true,
				Secure:   r.TLS != nil,
				SameSite: http.SameSiteStrictMode,
			})
			u := *r.URL
			u.RawQuery = ""
			http.Redirect(w, r, u.String(), http.StatusSeeOther)
			return
		}

		if c, err := r.Cookie(sessionCookie); err == nil && constantTimeEqual(c.Value, s.Token) {
			next.ServeHTTP(w, r)
			return
		}
		if constantTimeEqual(r.Header.Get("Authorization"), "Bearer "+s.Token) {
			next.ServeHTTP(w, r)
			return
		}
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
}

// constantTimeEqual compares two strings in constant time, so a
// timing side-channel can't be used to guess the session token byte by
// byte. subtle.ConstantTimeCompare requires equal-length inputs to be
// constant-time; the length check itself is a value-independent
// short-circuit (both operands' lengths, not their contents), so it costs
// no timing-safety here.
func constantTimeEqual(a, b string) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	events, err := s.Store.Events()
	if err != nil {
		serverError(w, err)
		return
	}
	stats := store.Overview(s.Store.Registry(), events, s.Now())
	pts := make([]charts.Point, len(stats.Adoption))
	for i, p := range stats.Adoption {
		pts[i] = charts.Point{X: p.Day, Y: float64(p.Governed)}
	}
	g := s.loadGuidance()
	modelVendor := make(map[string]string, len(stats.Models))
	for _, m := range stats.Models {
		if vk, ok := g.VendorForModel(m); ok {
			modelVendor[m] = vk
		}
	}
	data := overviewData{
		layoutData:    s.baseData("overview"),
		Stats:         stats,
		AdoptionChart: charts.Line("Governed repos over time", pts, 640, 220),
		ModelVendor:   modelVendor,
	}
	s.render(w, "overview", data)
}

// fleetStatuses is the set of valid status filters; an unrecognized query
// value falls through to "all rows".
var fleetStatuses = map[string]bool{"in-sync": true, "drifted": true, "stale": true, "ungoverned": true}

// colSort is one sortable header's rendered state: the toggle URL (current
// status preserved), the aria-sort value, and the direction a click will
// request next.
type colSort struct {
	Href    string // "/fleet?status=..&sort=..&dir=.."
	Aria    string // "none" | "ascending" | "descending"
	NextDir string // "asc" | "desc"
}

// fleetSorts maps a sort key to its less-than comparator over FleetRow.
var fleetSorts = map[string]func(a, b store.FleetRow) bool{
	"repo":      func(a, b store.FleetRow) bool { return a.RepoName < b.RepoName },
	"status":    func(a, b store.FleetRow) bool { return a.Status < b.Status },
	"last-sync": func(a, b store.FleetRow) bool { return a.LastSync.Before(b.LastSync) },
}

// fleetHeader computes a sortable column's rendered state. When col is the
// active sort, aria-sort reflects dir and a click toggles direction;
// otherwise a click sorts ascending.
func fleetHeader(col, status, activeCol, dir string) colSort {
	h := colSort{Aria: "none", NextDir: "asc"}
	if col == activeCol {
		if dir == "desc" {
			h.Aria, h.NextDir = "descending", "asc"
		} else {
			h.Aria, h.NextDir = "ascending", "desc"
		}
	}
	h.Href = fmt.Sprintf("/fleet?status=%s&sort=%s&dir=%s", url.QueryEscape(status), col, h.NextDir)
	return h
}

// fleetData extends layoutData with the (possibly filtered, possibly sorted)
// fleet rows, the active status filter, and per-column sort header state.
type fleetData struct {
	layoutData
	Rows         []store.FleetRow
	Status       string // "" = all
	SortCol      string // "", "repo", "status", "last-sync"
	RepoSort     colSort
	LastSyncSort colSort
	StatusSort   colSort

	// Unregistered is envelope-sourced activity whose remote never matched a
	// registered repo (shadow IT), grouped by remote. RegisterTargets is the
	// set of repos a remote can be assigned to (those with no Remote yet).
	Unregistered    []store.UnregisteredRemote
	RegisterTargets []store.Repo

	// Error is a validation message from a failed POST /fleet/register,
	// rendered back into this same page as an in-page banner (the
	// handlePackPublish/handleModelAdoptSave convention) rather than a bare
	// http.Error that would dump the admin out of the portal chrome. "" on
	// an ordinary GET.
	Error string
}

// buildFleetData computes the fleet page's data for the current request:
// rows (filtered/sorted per query params), sort-header state, and the
// unregistered-remote bucket. Shared by handleFleet (the GET) and
// handleFleetRegister's failure paths (which re-render this same page with
// Error set, following handlePackPublish's validation-error convention).
func (s *Server) buildFleetData(r *http.Request) (fleetData, error) {
	events, err := s.Store.Events()
	if err != nil {
		return fleetData{}, err
	}
	rows := store.FleetRows(s.Store.Registry(), events)

	status := r.URL.Query().Get("status")
	if fleetStatuses[status] {
		filtered := rows[:0:0]
		for _, row := range rows {
			if row.Status == status {
				filtered = append(filtered, row)
			}
		}
		rows = filtered
	} else {
		status = ""
	}

	sortCol := r.URL.Query().Get("sort")
	dir := r.URL.Query().Get("dir")
	if dir != "desc" {
		dir = "asc"
	}
	if less, ok := fleetSorts[sortCol]; ok {
		sort.SliceStable(rows, func(i, j int) bool {
			if dir == "desc" {
				return less(rows[j], rows[i])
			}
			return less(rows[i], rows[j])
		})
	} else {
		sortCol = ""
	}

	return fleetData{
		layoutData:      s.baseData("fleet"),
		Rows:            rows,
		Status:          status,
		SortCol:         sortCol,
		RepoSort:        fleetHeader("repo", status, sortCol, dir),
		LastSyncSort:    fleetHeader("last-sync", status, sortCol, dir),
		StatusSort:      fleetHeader("status", status, sortCol, dir),
		Unregistered:    store.UnregisteredRemotes(events),
		RegisterTargets: s.Store.Registry().UnassignedRepos(),
	}, nil
}

func (s *Server) handleFleet(w http.ResponseWriter, r *http.Request) {
	data, err := s.buildFleetData(r)
	if err != nil {
		serverError(w, err)
		return
	}
	if isHX(r) {
		s.renderFragment(w, "fleet", "fleet-table", data)
		return
	}
	s.render(w, "fleet", data)
}

// fleetRegisterError re-renders the fleet page with msg as an in-page
// banner at the given status code, instead of a bare http.Error that would
// dump the admin out of the portal chrome (matches handlePackPublish's and
// handleModelAdoptSave's validation-error convention). A failure building
// the fleet page itself is a genuine internal error, reported the usual way.
func (s *Server) fleetRegisterError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	data, err := s.buildFleetData(r)
	if err != nil {
		serverError(w, err)
		return
	}
	data.Error = msg
	s.renderStatus(w, status, "fleet", data)
}

// handleFleetRegister is the unregistered-remote register affordance: it
// assigns a remote (seen in envelope-sourced events, matched to no repo) to
// an existing registry repo that has no Remote yet, persisting through
// Store.SaveRegistry's atomic write. This is deliberately the minimal v1 —
// binding a remote to an existing repo — rather than a full repo-creation
// flow; see the task report for why.
func (s *Server) handleFleetRegister(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.fleetRegisterError(w, r, http.StatusBadRequest, "That registration form could not be read. Please try again.")
		return
	}
	remote := r.FormValue("remote")
	repoID := r.FormValue("repo_id")
	if remote == "" || repoID == "" {
		s.fleetRegisterError(w, r, http.StatusBadRequest, "Choose a repo before registering a remote.")
		return
	}

	events, err := s.Store.Events()
	if err != nil {
		serverError(w, err)
		return
	}
	stillUnregistered := false
	for _, u := range store.UnregisteredRemotes(events) {
		if u.Remote == remote {
			stillUnregistered = true
			break
		}
	}
	if !stillUnregistered {
		s.fleetRegisterError(w, r, http.StatusBadRequest, "That remote is no longer unregistered; someone may have already registered it.")
		return
	}

	reg := s.Store.Registry()
	found := false
	for i := range reg.Repos {
		if reg.Repos[i].ID != repoID {
			continue
		}
		if reg.Repos[i].Remote != "" {
			s.fleetRegisterError(w, r, http.StatusConflict, "That repo already has a remote registered.")
			return
		}
		reg.Repos[i].Remote = remote
		found = true
		break
	}
	if !found {
		s.fleetRegisterError(w, r, http.StatusNotFound, "That repo no longer exists.")
		return
	}
	if err := s.Store.SaveRegistry(reg); err != nil {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, "/fleet", http.StatusSeeOther)
}

// artifactDetail is one artifact's rendered state for the repo detail page,
// carrying both axes (Managed/Local), amendment size, and either the
// amendment's visible content, its withheld notice, or neither — "no
// amendments" and "amendments withheld" are computed as mutually exclusive,
// differently-worded cases so the two can never render identically (spec
// §6's must).
type artifactDetail struct {
	Path, Kind, Managed, Local string

	HasAmendment bool
	Bytes, Lines int
	Items        []string
	Content      string // set only when the amendment's text is visible

	Withheld       bool
	WithheldSource string // "pack" | "repo-override" (Collection.Source)

	Altered                        bool
	ExpectedHash, ActualHash, Diff string
}

// buildArtifactDetail derives one artifact's detail-page view. Withheld
// detection cannot key on Content == "" alone: an items-only amendment
// (kind=dir/json-keys — engine.Amendment's Content carries surrounding
// text for kind=block, Items carries discrete names for dir/json-keys) has
// an always-empty Content regardless of reporting level, so a fully
// visible items amendment must never be mistaken for a withheld one.
// Genuinely withheld means: there IS a local amendment, it carries neither
// Content nor Items (impossible from construction — engine.newAmendment
// refuses to build an Amendment with both empty — so their absence here
// means the publisher stripped a Content-bearing amendment down), and the
// Collection the event carries confirms the reporting level was below
// content (an items-only amendment reported at the content level is simply
// complete, not withheld).
func buildArtifactDetail(a store.EventArtifact, coll *engine.Collection) artifactDetail {
	d := artifactDetail{Path: a.Path, Kind: a.Kind, Managed: a.Managed, Local: a.Local}
	if a.Amendment != nil {
		d.HasAmendment = true
		d.Bytes = a.Amendment.Bytes
		d.Lines = a.Amendment.Lines
		d.Items = a.Amendment.Items
		switch {
		case a.Amendment.Content != "":
			d.Content = a.Amendment.Content
		case len(a.Amendment.Items) > 0:
			// Items-only amendment: complete as-is, nothing withheld.
		case a.Local == "amended" && (coll == nil || coll.Amendments != engine.ReportContent):
			d.Withheld = true
			d.WithheldSource = "policy"
			if coll != nil && coll.Source != "" {
				d.WithheldSource = coll.Source
			}
		}
	}
	if a.Managed == "altered" && a.Alteration != nil {
		d.Altered = true
		d.ExpectedHash = a.Alteration.ExpectedHash
		d.ActualHash = a.Alteration.ActualHash
		d.Diff = a.Alteration.Diff
	}
	return d
}

// repoDetailData is the /fleet/{repoID} page's data: the repo's org
// placement, both governance axes, and its latest event's per-artifact
// detail (Task 9: the withheld rendering lives in Artifacts).
type repoDetailData struct {
	layoutData
	RepoID, RepoName, DeptName, TeamName string
	Status, State                        string // "ungoverned" when HasEvent is false
	HasEvent                             bool
	LastSync                             time.Time
	Packs, Tools                         []string
	CollectionLevel, CollectionSource    string // "" when the event carries no Collection
	Artifacts                            []artifactDetail
}

func (s *Server) handleRepoDetail(w http.ResponseWriter, r *http.Request) {
	repoID := r.PathValue("repoID")
	reg := s.Store.Registry()
	var repo store.Repo
	found := false
	for _, rp := range reg.Repos {
		if rp.ID == repoID {
			repo = rp
			found = true
			break
		}
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	events, err := s.Store.Events()
	if err != nil {
		serverError(w, err)
		return
	}
	team, deptName := reg.TeamAndDept(repo.TeamID)

	data := repoDetailData{
		layoutData: s.baseData("fleet"),
		RepoID:     repo.ID,
		RepoName:   repo.Name,
		DeptName:   deptName,
		TeamName:   team.Name,
		Status:     "ungoverned",
		State:      "ungoverned",
	}
	if e, ok := store.LatestPostureEvent(events, repoID); ok {
		data.HasEvent = true
		data.Status = e.Drift
		data.State = store.FleetState(e.Artifacts)
		data.LastSync = e.TS
		for _, p := range e.Packs {
			data.Packs = append(data.Packs, fmt.Sprintf("%s@%s", p.Name, p.Version))
		}
		sort.Strings(data.Packs)
		if e.Collection != nil {
			data.CollectionLevel = e.Collection.Amendments
			data.CollectionSource = e.Collection.Source
		}
		for _, a := range e.Artifacts {
			data.Artifacts = append(data.Artifacts, buildArtifactDetail(a, e.Collection))
		}
	}
	tools := map[string]bool{}
	for _, e := range events {
		if e.RepoID == repoID && e.AgentTool != "" {
			tools[e.AgentTool] = true
		}
	}
	for t := range tools {
		data.Tools = append(data.Tools, t)
	}
	sort.Strings(data.Tools)

	s.render(w, "repo_detail", data)
}

// packsData extends layoutData with every configured pack, for the /packs
// list page.
type packsData struct {
	layoutData
	Packs []publish.PackInfo
}

func (s *Server) handlePacks(w http.ResponseWriter, r *http.Request) {
	var infos []publish.PackInfo
	if s.Packs != nil {
		var err error
		infos, err = s.Packs.List(r.Context())
		if err != nil {
			serverError(w, err)
			return
		}
	}
	data := packsData{layoutData: s.baseData("packs"), Packs: infos}
	s.render(w, "packs", data)
}

// fragmentView pairs a fragment's path with its rendered HTML, for the pack
// detail page.
type fragmentView struct {
	Path string
	HTML template.HTML
}

// packDetailData extends layoutData with one pack's version history and
// rendered fragments, plus an optional "just published" banner version.
type packDetailData struct {
	layoutData
	Pack      publish.PackInfo
	Fragments []fragmentView
	Published string // "" = no banner; else e.g. "v1.3.0"
}

func (s *Server) handlePackDetail(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if s.Packs == nil {
		http.NotFound(w, r)
		return
	}
	info, err := s.Packs.Get(r.Context(), name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	frags := make([]fragmentView, 0, len(info.Fragments))
	for _, f := range info.Fragments {
		content, err := s.Packs.ReadFragment(name, f)
		if err != nil {
			serverError(w, err)
			return
		}
		frags = append(frags, fragmentView{Path: f, HTML: mdHTML(content)})
	}
	data := packDetailData{
		layoutData: s.baseData("packs"),
		Pack:       *info,
		Fragments:  frags,
		Published:  r.URL.Query().Get("published"),
	}
	s.render(w, "pack", data)
}

// packEditData extends layoutData with the edit form's state: the fragment
// being edited, its content (the on-disk version, or the user's submitted
// version on a diff/validation-error re-render), the suggested next
// version, and optionally a diff preview or a validation error.
type packEditData struct {
	layoutData
	Name             string
	Frag             string
	Content          string
	SuggestedVersion string
	Diff             string
	DiffRequested    bool // a diff action ran; render feedback even when Diff is empty
	Error            string
}

// nextPatchVersion suggests the next version for a pack's edit form: the
// minor component bumped by one with patch reset to 0 (1.2.0 -> 1.3.0),
// matching how this pack's fixture and publish tests treat "the next
// version" throughout. version is expected to already be a valid x.y.z
// (PackInfo.Version comes from a validated manifest); if it isn't, version
// is returned unchanged rather than guessing.
func nextPatchVersion(version string) string {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) != 3 {
		return version
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return version
	}
	return fmt.Sprintf("%s.%d.0", parts[0], minor+1)
}

func (s *Server) handlePackEdit(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if s.Packs == nil {
		http.NotFound(w, r)
		return
	}
	info, err := s.Packs.Get(r.Context(), name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	frag := r.URL.Query().Get("frag")
	content, err := s.Packs.ReadFragment(name, frag)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	data := packEditData{
		layoutData:       s.baseData("packs"),
		Name:             name,
		Frag:             frag,
		Content:          string(content),
		SuggestedVersion: nextPatchVersion(info.Version),
	}
	s.render(w, "pack_edit", data)
}

func (s *Server) handlePackPublish(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if s.Packs == nil {
		http.NotFound(w, r)
		return
	}
	if _, err := s.Packs.Get(r.Context(), name); err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	frag := r.FormValue("frag")
	content := r.FormValue("content")
	version := r.FormValue("version")

	switch r.FormValue("action") {
	case "diff":
		diff, err := s.Packs.Diff(r.Context(), name, frag, []byte(content))
		if err != nil {
			serverError(w, err)
			return
		}
		data := packEditData{
			layoutData:       s.baseData("packs"),
			Name:             name,
			Frag:             frag,
			Content:          content,
			SuggestedVersion: version,
			Diff:             diff,
			DiffRequested:    true,
		}
		if isHX(r) {
			s.renderFragment(w, "pack_edit", "diff-region", data)
			return
		}
		s.render(w, "pack_edit", data)
	case "publish":
		if err := s.Packs.Publish(r.Context(), name, frag, []byte(content), version); err != nil {
			if errors.Is(err, esc.ErrManifest) || errors.Is(err, publish.ErrBadVersion) {
				data := packEditData{
					layoutData:       s.baseData("packs"),
					Name:             name,
					Frag:             frag,
					Content:          content,
					SuggestedVersion: version,
					Error:            err.Error(),
				}
				s.renderStatus(w, http.StatusUnprocessableEntity, "pack_edit", data)
				return
			}
			serverError(w, err)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/packs/%s?published=v%s", name, version), http.StatusSeeOther)
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
	}
}

// usageDays is the set of valid days filters; an unrecognized or missing
// query value falls back to 30.
var usageDays = map[int]bool{7: true, 30: true, 60: true}

// usageModelRow is one model's row in the usage summary table.
type usageModelRow struct {
	Model  string
	Tokens int64
	Cost   float64
	Share  float64 // percent of total tokens, 0-100
}

// usageData extends layoutData with the filter state, both charts, and the
// per-model summary table (with totals) for the usage page.
type usageData struct {
	layoutData
	Teams       []store.Team
	Models      []string
	Team        string
	Model       string
	Days        int
	TokensChart template.HTML
	CostChart   template.HTML
	Rows        []usageModelRow
	TotalTokens int64
	TotalCost   float64
	ModelVendor map[string]string
}

func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	events, err := s.Store.Events()
	if err != nil {
		serverError(w, err)
		return
	}
	reg := s.Store.Registry()

	days := 30
	if d, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && usageDays[d] {
		days = d
	}
	team := r.URL.Query().Get("team")
	model := r.URL.Query().Get("model")

	end := s.Now().UTC().Truncate(24 * time.Hour)
	from := end.AddDate(0, 0, -(days - 1))
	// UsageDaily's "to" bound is an inclusive event-timestamp cutoff
	// (e.TS.After(to)), not a day boundary — events on the `end` day carry
	// real hours (e.g. 14:00), so passing the bare truncated `end` would
	// exclude nearly all of the most recent day. Extend the cutoff to the
	// start of the following day so every timestamp within `end`'s 24h
	// window is included; cells that truncate to a day outside [from, end]
	// are still dropped below via dayIndex.
	cells := store.UsageDaily(events, team, model, from, end.AddDate(0, 0, 1))

	teamName := map[string]string{}
	for _, t := range reg.Teams {
		teamName[t.ID] = t.Name
	}
	teams := append([]store.Team(nil), reg.Teams...)
	sort.Slice(teams, func(i, j int) bool { return teams[i].Name < teams[j].Name })

	// Cost-chart series cover every team ID seen in either the registry or
	// the filtered cells, sorted by ID for determinism (never map
	// iteration). A cell's TeamID with no registry match falls back to the
	// raw ID as its series label.
	teamIDSet := map[string]bool{}
	for _, t := range reg.Teams {
		teamIDSet[t.ID] = true
	}
	for _, c := range cells {
		teamIDSet[c.TeamID] = true
	}
	var teamIDs []string
	for id := range teamIDSet {
		teamIDs = append(teamIDs, id)
	}
	sort.Strings(teamIDs)

	modelSet := map[string]bool{}
	for _, e := range events {
		if e.Kind == "provider_usage" && e.Model != "" {
			modelSet[e.Model] = true
		}
	}
	var models []string
	for m := range modelSet {
		models = append(models, m)
	}
	sort.Strings(models)

	// Day labels covering [from, end], zero-filled.
	var labels []string
	var dayList []time.Time
	for d := from; !d.After(end); d = d.AddDate(0, 0, 1) {
		labels = append(labels, d.Format("Jan 2"))
		dayList = append(dayList, d)
	}
	dayIndex := make(map[time.Time]int, len(dayList))
	for i, d := range dayList {
		dayIndex[d] = i
	}

	// Chart 1: tokens/day stacked by model.
	tokenSeries := make(map[string][]float64, len(models))
	for _, m := range models {
		tokenSeries[m] = make([]float64, len(labels))
	}
	// Chart 2: cost/day stacked by team.
	costSeries := make(map[string][]float64, len(teamIDs))
	for _, id := range teamIDs {
		costSeries[id] = make([]float64, len(labels))
	}

	tokensByModel := map[string]int64{}
	costByModel := map[string]float64{}
	var totalTokens int64
	var totalCost float64

	for _, c := range cells {
		idx, ok := dayIndex[c.Day]
		if !ok {
			continue
		}
		if vals, ok := tokenSeries[c.Model]; ok {
			vals[idx] += float64(c.Tokens)
		}
		if vals, ok := costSeries[c.TeamID]; ok {
			vals[idx] += c.Cost
		}
		tokensByModel[c.Model] += c.Tokens
		costByModel[c.Model] += c.Cost
		totalTokens += c.Tokens
		totalCost += c.Cost
	}

	var tokenChartSeries []charts.Series
	for _, m := range models {
		tokenChartSeries = append(tokenChartSeries, charts.Series{Label: m, Values: tokenSeries[m]})
	}
	var costChartSeries []charts.Series
	for _, id := range teamIDs {
		label := teamName[id]
		if label == "" {
			label = id
		}
		costChartSeries = append(costChartSeries, charts.Series{Label: label, Values: costSeries[id]})
	}

	// Summary table rows, per model with usage, sorted by model name.
	var summaryModels []string
	for m := range tokensByModel {
		summaryModels = append(summaryModels, m)
	}
	sort.Strings(summaryModels)
	var rows []usageModelRow
	for _, m := range summaryModels {
		tok := tokensByModel[m]
		cost := costByModel[m]
		share := 0.0
		if totalTokens > 0 {
			share = float64(tok) / float64(totalTokens) * 100
		}
		rows = append(rows, usageModelRow{Model: m, Tokens: tok, Cost: cost, Share: share})
	}

	g := s.loadGuidance()
	modelVendor := make(map[string]string, len(models))
	for _, m := range models {
		if vk, ok := g.VendorForModel(m); ok {
			modelVendor[m] = vk
		}
	}
	for _, row := range rows {
		if _, done := modelVendor[row.Model]; !done {
			if vk, ok := g.VendorForModel(row.Model); ok {
				modelVendor[row.Model] = vk
			}
		}
	}

	data := usageData{
		layoutData:  s.baseData("usage"),
		Teams:       teams,
		Models:      models,
		Team:        team,
		Model:       model,
		Days:        days,
		TokensChart: charts.StackedBars("Tokens per day by model", labels, tokenChartSeries, 640, 220),
		CostChart:   charts.StackedBars("Cost per day by team", labels, costChartSeries, 640, 220),
		Rows:        rows,
		TotalTokens: totalTokens,
		TotalCost:   totalCost,
		ModelVendor: modelVendor,
	}
	if isHX(r) {
		s.renderFragment(w, "usage", "usage-results", data)
		return
	}
	s.render(w, "usage", data)
}
