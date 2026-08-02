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
	"sort"
	"strconv"
	"strings"
	"time"

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

	layout *template.Template
	pages  map[string]*template.Template
}

// pageNames are the page templates parsed at startup. Each defines the
// "title", "explainer", and "content" blocks that override the layout.
var pageNames = []string{"overview", "fleet", "packs", "pack", "pack_edit", "usage"}

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
	mux.HandleFunc("GET /packs", s.handlePacks)
	mux.HandleFunc("GET /packs/{name}", s.handlePackDetail)
	mux.HandleFunc("GET /packs/{name}/edit", s.handlePackEdit)
	mux.HandleFunc("POST /packs/{name}/publish", s.handlePackPublish)
	mux.HandleFunc("GET /usage", s.handleUsage)
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
	data := overviewData{
		layoutData:    s.baseData("overview"),
		Stats:         stats,
		AdoptionChart: charts.Line(pts, 640, 220),
	}
	s.render(w, "overview", data)
}

// fleetStatuses is the set of valid status filters; an unrecognized query
// value falls through to "all rows".
var fleetStatuses = map[string]bool{"in-sync": true, "drifted": true, "stale": true, "ungoverned": true}

// fleetData extends layoutData with the (possibly filtered) fleet rows and
// the active status filter, for the filter-links row to bold.
type fleetData struct {
	layoutData
	Rows   []store.FleetRow
	Status string // "" = all
}

func (s *Server) handleFleet(w http.ResponseWriter, r *http.Request) {
	events, err := s.Store.Events()
	if err != nil {
		serverError(w, err)
		return
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

	data := fleetData{
		layoutData: s.baseData("fleet"),
		Rows:       rows,
		Status:     status,
	}
	s.render(w, "fleet", data)
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
		}
		s.render(w, "pack_edit", data)
	case "publish":
		if err := s.Packs.Publish(r.Context(), name, frag, []byte(content), version); err != nil {
			if errors.Is(err, esc.ErrManifest) {
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

	data := usageData{
		layoutData:  s.baseData("usage"),
		Teams:       teams,
		Models:      models,
		Team:        team,
		Model:       model,
		Days:        days,
		TokensChart: charts.StackedBars(labels, tokenChartSeries, 640, 220),
		CostChart:   charts.StackedBars(labels, costChartSeries, 640, 220),
		Rows:        rows,
		TotalTokens: totalTokens,
		TotalCost:   totalCost,
	}
	s.render(w, "usage", data)
}
