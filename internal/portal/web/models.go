package web

import (
	"net/http"
	"net/url"

	"github.com/tensorgroup/openescapement/internal/guidance"
)

// loadGuidance reads the guidance tree from disk on every request (embedded
// fallback), matching the spec's hand-editable content posture.
func (s *Server) loadGuidance() *guidance.Set { return guidance.Load(s.GuidanceDir) }

// urlHost returns a URL's host for compact display, or the raw string if it
// does not parse. Registered as the "host" template func.
func urlHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	return u.Host
}

func findVendor(reg *guidance.Registry, key string) (guidance.Vendor, bool) {
	for _, v := range reg.Vendors {
		if v.Key == key {
			return v, true
		}
	}
	return guidance.Vendor{}, false
}

// modelsData is the /models overview page's data.
type modelsData struct {
	layoutData
	Vendors  []guidance.Vendor
	Degraded []string
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	g := s.loadGuidance()
	s.render(w, "models", modelsData{
		layoutData: s.baseData("models"),
		Vendors:    g.Registry.Vendors,
		Degraded:   g.Degraded,
	})
}

// The vendor page and editor handlers are implemented in Tasks 4-5.
func (s *Server) handleModelVendor(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }
func (s *Server) handleModelEdit(w http.ResponseWriter, r *http.Request)   { http.NotFound(w, r) }
func (s *Server) handleModelEditSave(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}
