package web

import (
	"html/template"
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

// editHref builds a guidance-file edit link's href. Registered as the
// "editHref" template func: html/template's contextual autoescaper
// percent-encodes "/" when a plain string is substituted into a URL's query
// value (?file={{.}}), which breaks links to nested example paths like
// "examples/anthropic/model-routing.md". Returning template.URL tells the
// escaper this value is already a safe, complete URL, so it is emitted
// verbatim. vendor and file always come from the guidance registry and
// KnownFiles, never from request input, so trusting them here is safe.
func editHref(vendor, file string) template.URL {
	return template.URL("/models/" + vendor + "/edit?file=" + file)
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

// exampleView pairs an example fragment's rel path with its rendered HTML
// and raw source, for the vendor page's rendered-plus-copyable display.
type exampleView struct {
	File string
	HTML template.HTML
	Raw  string
}

// modelVendorData is the /models/{vendor} guidance page's data.
type modelVendorData struct {
	layoutData
	VendorKey  string
	VendorName string
	NoteHTML   template.HTML
	NoteFile   string
	Models     []guidance.Model
	Examples   []exampleView
	Degraded   []string
}

func (s *Server) handleModelVendor(w http.ResponseWriter, r *http.Request) {
	vendor := r.PathValue("vendor")
	g := s.loadGuidance()
	v, ok := findVendor(g.Registry, vendor)
	if !ok {
		http.NotFound(w, r)
		return
	}
	degraded := append([]string(nil), g.Degraded...)

	noteFile := vendor + ".md"
	note, noteDeg, err := g.ReadFile(noteFile)
	if err != nil {
		serverError(w, err)
		return
	}
	if noteDeg {
		degraded = append(degraded, noteFile)
	}

	var examples []exampleView
	for _, rel := range g.ExampleFiles(vendor) {
		content, deg, err := g.ReadFile(rel)
		if err != nil {
			continue
		}
		if deg {
			degraded = append(degraded, rel)
		}
		examples = append(examples, exampleView{File: rel, HTML: mdHTML(content), Raw: string(content)})
	}

	s.render(w, "model_vendor", modelVendorData{
		layoutData: s.baseData("models"),
		VendorKey:  v.Key,
		VendorName: v.Name,
		NoteHTML:   mdHTML(note),
		NoteFile:   noteFile,
		Models:     v.Models,
		Examples:   examples,
		Degraded:   degraded,
	})
}

// The editor handlers are implemented in Task 5.
func (s *Server) handleModelEdit(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }
func (s *Server) handleModelEditSave(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}
