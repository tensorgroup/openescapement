package web

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/guidance"
	"github.com/tensorgroup/openescapement/internal/pack"
	"github.com/tensorgroup/openescapement/internal/portal/publish"
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

// writablePacks lists the configured pack clones the adopt flow can publish
// into. Nil manager (no repos configured) yields an empty slice, not an error.
func (s *Server) writablePacks(ctx context.Context) ([]publish.PackInfo, error) {
	if s.Packs == nil {
		return nil, nil
	}
	return s.Packs.List(ctx)
}

// modelView pairs a registry Model with its rendered starter fragment (when
// one is set) for the vendor page's per-model starter block.
type modelView struct {
	guidance.Model
	StarterFile string
	StarterHTML template.HTML
	StarterRaw  string
	HasStarter  bool
}

// modelVendorData is the /models/{vendor} guidance page's data.
type modelVendorData struct {
	layoutData
	VendorKey      string
	VendorName     string
	NoteHTML       template.HTML
	NoteFile       string
	Models         []modelView
	Examples       []exampleView
	PacksAvailable bool
	HasRouting     bool
	ShowSet        bool
	Degraded       []string
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

	models := make([]modelView, 0, len(v.Models))
	for _, m := range v.Models {
		mv := modelView{Model: m}
		if m.Starter != "" {
			if content, deg, rerr := g.ReadFile(m.Starter); rerr == nil {
				mv.HasStarter = true
				mv.StarterFile = m.Starter
				mv.StarterHTML = mdHTML(content)
				mv.StarterRaw = string(content)
				if deg {
					degraded = append(degraded, m.Starter)
				}
			}
		}
		models = append(models, mv)
	}
	packsAvailable := false
	if infos, perr := s.writablePacks(r.Context()); perr == nil && len(infos) > 0 {
		packsAvailable = true
	}

	starters := 0
	for _, mv := range models {
		if mv.HasStarter {
			starters++
		}
	}
	hasRouting := g.KnownFiles()["examples/"+v.Key+"/model-routing.md"]
	showSet := packsAvailable && (starters >= 2 || (starters == 1 && hasRouting))

	s.render(w, "model_vendor", modelVendorData{
		layoutData:     s.baseData("models"),
		VendorKey:      v.Key,
		VendorName:     v.Name,
		NoteHTML:       mdHTML(note),
		NoteFile:       noteFile,
		Models:         models,
		Examples:       examples,
		PacksAvailable: packsAvailable,
		HasRouting:     hasRouting,
		ShowSet:        showSet,
		Degraded:       degraded,
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

// packOption is one writable pack in the adopt selector.
type packOption struct {
	Name             string
	SuggestedVersion string
}

// packOptions maps writable packs to selector options with a suggested next
// version each.
func packOptions(infos []publish.PackInfo) []packOption {
	opts := make([]packOption, len(infos))
	for i, p := range infos {
		opts[i] = packOption{Name: p.Name, SuggestedVersion: nextPatchVersion(p.Version)}
	}
	return opts
}

// adoptSelection is a validated adopt request: the chosen registry models in
// registry order, and the fragment bytes to compose (routing overview first).
type adoptSelection struct {
	Models []guidance.Model
	Parts  [][]byte
}

// resolveAdoptSelection resolves requested starter ids plus the optional
// routing example against the vendor's registry. All-or-nothing: any unknown
// or starter-less id fails the whole request. Every path read here is either
// registry-declared or built from the vendor key alone — the request never
// contributes a file path.
func resolveAdoptSelection(g *guidance.Set, v guidance.Vendor, ids []string, routing bool) (adoptSelection, bool) {
	requested := map[string]bool{}
	for _, id := range ids {
		requested[id] = true
	}
	var sel adoptSelection
	if routing {
		content, _, err := g.ReadFile("examples/" + v.Key + "/model-routing.md")
		if err != nil {
			return adoptSelection{}, false
		}
		sel.Parts = append(sel.Parts, content)
	}
	for _, m := range v.Models {
		if m.Starter == "" || !requested[m.ID] {
			continue
		}
		delete(requested, m.ID)
		content, _, err := g.ReadFile(m.Starter)
		if err != nil {
			return adoptSelection{}, false
		}
		sel.Models = append(sel.Models, m)
		sel.Parts = append(sel.Parts, content)
	}
	if len(requested) > 0 || len(sel.Parts) == 0 {
		return adoptSelection{}, false
	}
	return sel, true
}

// adoptDest names the destination fragment: the historical per-model path for
// a single starter, one vendor-set file for any composed selection.
func adoptDest(v guidance.Vendor, models []guidance.Model, routing bool) string {
	if len(models) == 1 && !routing {
		return "rules/model-" + models[0].ID + ".md"
	}
	return "rules/models-" + v.Key + ".md"
}

// adoptLabel names the selection for page titles and headings.
func adoptLabel(v guidance.Vendor, models []guidance.Model, routing bool) string {
	names := make([]string, len(models))
	for i, m := range models {
		names[i] = m.Name
	}
	label := strings.Join(names, ", ")
	if routing {
		if label == "" {
			return v.Name + " model routing"
		}
		label += " + model routing"
	}
	return label
}

// modelAdoptData is the /models/{vendor}/adopt page's data.
type modelAdoptData struct {
	layoutData
	VendorKey   string
	VendorName  string
	Selection   []guidance.Model
	Routing     bool
	Label       string
	Dest        string
	StarterHTML template.HTML
	StarterRaw  string
	Packs       []packOption
	PackName    string
	Version     string
	Error       string
}

func (s *Server) handleModelAdopt(w http.ResponseWriter, r *http.Request) {
	vendor := r.PathValue("vendor")
	g := s.loadGuidance()
	v, ok := findVendor(g.Registry, vendor)
	if !ok {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	routing := q.Get("routing") != ""
	sel, ok := resolveAdoptSelection(g, v, q["model"], routing)
	if !ok {
		http.NotFound(w, r)
		return
	}
	content, err := pack.ComposeFragments(sel.Parts)
	if err != nil {
		serverError(w, err)
		return
	}
	infos, err := s.writablePacks(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	if len(infos) == 0 {
		http.NotFound(w, r)
		return
	}
	opts := packOptions(infos)
	s.render(w, "model_adopt", modelAdoptData{
		layoutData:  s.baseData("models"),
		VendorKey:   v.Key,
		VendorName:  v.Name,
		Selection:   sel.Models,
		Routing:     routing,
		Label:       adoptLabel(v, sel.Models, routing),
		Dest:        adoptDest(v, sel.Models, routing),
		StarterHTML: mdHTML(content),
		StarterRaw:  string(content),
		Packs:       opts,
		PackName:    opts[0].Name,
		Version:     opts[0].SuggestedVersion,
	})
}

func (s *Server) handleModelAdoptSave(w http.ResponseWriter, r *http.Request) {
	vendor := r.PathValue("vendor")
	g := s.loadGuidance()
	v, ok := findVendor(g.Registry, vendor)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	routing := r.FormValue("routing") != ""
	sel, ok := resolveAdoptSelection(g, v, r.Form["model"], routing)
	if !ok {
		http.NotFound(w, r)
		return
	}
	content, err := pack.ComposeFragments(sel.Parts)
	if err != nil {
		serverError(w, err)
		return
	}
	infos, err := s.writablePacks(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	if len(infos) == 0 {
		http.NotFound(w, r)
		return
	}
	packName := r.FormValue("pack")
	version := r.FormValue("version")
	valid := false
	for _, p := range infos {
		if p.Name == packName {
			valid = true
		}
	}
	if !valid {
		http.NotFound(w, r)
		return
	}
	dest := adoptDest(v, sel.Models, routing)
	if err := s.Packs.AddFragment(r.Context(), packName, dest, content, version); err != nil {
		if errors.Is(err, publish.ErrFragmentExists) || errors.Is(err, esc.ErrManifest) || errors.Is(err, publish.ErrBadVersion) {
			msg := err.Error()
			if errors.Is(err, publish.ErrFragmentExists) {
				msg = "This pack already has " + dest + ". Edit that fragment in the pack instead; adopting never overwrites."
			}
			s.renderStatus(w, http.StatusUnprocessableEntity, "model_adopt", modelAdoptData{
				layoutData:  s.baseData("models"),
				VendorKey:   v.Key,
				VendorName:  v.Name,
				Selection:   sel.Models,
				Routing:     routing,
				Label:       adoptLabel(v, sel.Models, routing),
				Dest:        dest,
				StarterHTML: mdHTML(content),
				StarterRaw:  string(content),
				Packs:       packOptions(infos),
				PackName:    packName,
				Version:     version,
				Error:       msg,
			})
			return
		}
		serverError(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/packs/%s?published=v%s", packName, version), http.StatusSeeOther)
}
