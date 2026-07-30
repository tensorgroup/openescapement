// Package web implements the esc admin portal HTTP server: layout, static
// assets, token auth, and (in later tasks) the overview/fleet/packs/usage
// pages and the telemetry ingest endpoint.
package web

import (
	"crypto/subtle"
	"embed"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/tensorgroup/openescapement/internal/portal/store"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

const sessionCookie = "esc_session"

// Server holds the portal's dependencies and routes. Packs (the publish
// manager) is added in Task 8/9; handlers before then don't need it.
type Server struct {
	Store   *store.Store
	Token   string // "" = auth disabled (demo mode)
	Version string
	Now     func() time.Time // injectable clock for tests; default time.Now

	layout *template.Template
	pages  map[string]*template.Template
}

// pageNames are the page templates parsed at startup. Each defines the
// "title", "explainer", and "content" blocks that override the layout.
var pageNames = []string{"overview", "fleet", "packs", "usage"}

// New builds a Server with its templates parsed and ready to serve.
func New(st *store.Store, token, version string) *Server {
	s := &Server{
		Store:   st,
		Token:   token,
		Version: version,
		Now:     time.Now,
	}
	s.layout = template.Must(template.New("layout.html").ParseFS(templateFS, "templates/layout.html"))
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
}

func (s *Server) baseData() layoutData {
	return layoutData{Version: s.Version}
}

// render executes the named page template against the shared layout.
func (s *Server) render(w http.ResponseWriter, page string, data any) {
	t, ok := s.pages[page]
	if !ok {
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
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

	return s.withAuth(mux)
}

// noStore sets Cache-Control: no-store on static asset responses so a
// stale-cached style.css never survives a portal upgrade.
func noStore(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
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
				SameSite: http.SameSiteLaxMode,
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
	s.render(w, "overview", s.baseData())
}

func (s *Server) handleFleet(w http.ResponseWriter, r *http.Request) {
	s.render(w, "fleet", s.baseData())
}

func (s *Server) handlePacks(w http.ResponseWriter, r *http.Request) {
	s.render(w, "packs", s.baseData())
}

func (s *Server) handlePackDetail(w http.ResponseWriter, r *http.Request) {
	s.render(w, "packs", s.baseData())
}

func (s *Server) handlePackEdit(w http.ResponseWriter, r *http.Request) {
	s.render(w, "packs", s.baseData())
}

func (s *Server) handlePackPublish(w http.ResponseWriter, r *http.Request) {
	// Wired in Task 9 once the publish manager exists.
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	s.render(w, "usage", s.baseData())
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	// Wired in Task 6b.
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
