package web

import (
	"html/template"
	"net/http"
	"net/url"
	"strings"

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

// modelEditData is the /models/{vendor}/edit page's data.
type modelEditData struct {
	layoutData
	VendorKey string
	File      string
	Content   string
	Error     string
}

// fileBelongsToVendor gates the editor to a vendor's own known files, so the
// portal never writes a path assembled from unchecked user input.
func fileBelongsToVendor(g *guidance.Set, vendor, file string) bool {
	if !g.KnownFiles()[file] {
		return false
	}
	return file == vendor+".md" || strings.HasPrefix(file, "examples/"+vendor+"/")
}

func (s *Server) handleModelEdit(w http.ResponseWriter, r *http.Request) {
	vendor := r.PathValue("vendor")
	g := s.loadGuidance()
	if _, ok := findVendor(g.Registry, vendor); !ok {
		http.NotFound(w, r)
		return
	}
	file := r.URL.Query().Get("file")
	if !fileBelongsToVendor(g, vendor, file) {
		http.NotFound(w, r)
		return
	}
	content, _, err := g.ReadFile(file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.render(w, "model_edit", modelEditData{
		layoutData: s.baseData("models"),
		VendorKey:  vendor,
		File:       file,
		Content:    string(content),
	})
}

func (s *Server) handleModelEditSave(w http.ResponseWriter, r *http.Request) {
	vendor := r.PathValue("vendor")
	g := s.loadGuidance()
	if _, ok := findVendor(g.Registry, vendor); !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	file := r.FormValue("file")
	content := r.FormValue("content")
	if !fileBelongsToVendor(g, vendor, file) {
		http.NotFound(w, r)
		return
	}
	if err := g.WriteFile(file, []byte(content)); err != nil {
		s.renderStatus(w, http.StatusUnprocessableEntity, "model_edit", modelEditData{
			layoutData: s.baseData("models"),
			VendorKey:  vendor,
			File:       file,
			Content:    content,
			Error:      err.Error(),
		})
		return
	}
	http.Redirect(w, r, "/models/"+vendor, http.StatusSeeOther)
}
