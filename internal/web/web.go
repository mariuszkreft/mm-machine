// Package web serves the marketing + pipeline surface (the original htmx UI),
// now backed by the store instead of hardcoded slices.
package web

import (
	"context"
	"embed"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mm-machine/internal/app"
	"mm-machine/internal/demo"
	"mm-machine/internal/i18n"
	"mm-machine/internal/model"
	"mm-machine/internal/onboarding"
	"mm-machine/internal/store"
)

//go:embed static/*
var staticFS embed.FS

// Dashboard is the view model for the whole page and its partials.
type Dashboard struct {
	Now          string
	View         string
	Query        string
	Role         string
	Version      string
	LLMModel     string
	Offers       []model.Offer
	Counts       map[string]int
	Spotlight    model.Offer
	Perspective  model.Perspective
	Perspectives []model.Perspective
	Modules      []model.Module
	Roadmap      []model.RoadmapItem
	// Statuses drives the per-offer status buttons.
	Statuses []string
}

// Handler serves the public surface.
type Handler struct {
	deps     app.Deps
	page     *template.Template
	shell    *template.Template
	part     *template.Template
	pipeline *template.Template
}

var funcs = template.FuncMap{
	"lower": strings.ToLower,
	"itoa":  strconv.Itoa,
	// signalTone maps a machine-produced Signal onto the badge tone that
	// carries it — colour is never the only cue, the word is always there too.
	"signalTone": func(signal string) string {
		switch signal {
		case "OK":
			return "good"
		case "Review":
			return "warn"
		case "Attention":
			return "bad"
		default:
			return ""
		}
	},
}

// Register wires the public routes onto mux and returns the handler.
func Register(mux *http.ServeMux, deps app.Deps) *Handler {
	h := &Handler{
		deps:     deps,
		page:     template.Must(template.New("page").Funcs(funcs).Parse(aboutHTML + perspectiveHTML)),
		shell:    template.Must(template.New("shell").Funcs(funcs).Parse(shellHTML + greetingHTML)),
		part:     template.Must(template.New("partial").Funcs(funcs).Parse(offersHTML + perspectiveHTML)),
		pipeline: template.Must(template.New("pipeline").Funcs(funcs).Parse(pipelineHTML + pipelineToolbarHTML + offersHTML)),
	}
	mux.Handle("/static/", http.FileServer(http.FS(staticFS)))
	mux.HandleFunc("/", h.home)
	mux.HandleFunc("/about", h.about)
	mux.HandleFunc("/offers", h.offers)
	mux.HandleFunc("/perspective", h.perspective)
	mux.HandleFunc("/offers/new", h.createOffer)
	mux.HandleFunc("/offers/status", h.advanceOffer)
	return h
}

// Shell is the view model of the home surface. It carries only what the prompt
// and the greeting need — everything else arrives in the thread.
type Shell struct {
	T           i18n.Printer
	Headline    string
	Lede        string
	Placeholder string
	Suggestions []string
	Version     string
	LLMModel    string
	Profile     model.Profile
	// PersonaLabel is set while the visitor is looking around as an example
	// profile, so the shell can say so and offer the way out.
	PersonaLabel string
	ProfileLine  string
}

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	profile, _ := onboarding.Current(r, h.deps)
	printer := i18n.NewPrinter(r)
	view := Shell{
		T:           printer,
		Headline:    printer.T("home.headline"),
		Lede:        printer.T("home.lede"),
		Placeholder: printer.T("home.placeholder"),
		Suggestions: suggestions(profile, printer, r),
		Version:     h.deps.Version,
		LLMModel:    h.deps.LLMModel,
		Profile:     profile,
		ProfileLine: profileLine(profile, printer),
	}
	if persona, ok := demo.ActivePersona(r); ok {
		view.PersonaLabel = persona.Label
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.shell.ExecuteTemplate(w, "shell", view); err != nil {
		log.Printf("web: render shell: %v", err)
	}
}

// suggestions are the example prompts under the input. They change with what
// the app already knows, so a returning visitor is never shown "who are you" —
// and inside an example profile they are that persona's own questions.
func suggestions(p model.Profile, printer i18n.Printer, r *http.Request) []string {
	if persona, ok := demo.ActivePersona(r); ok {
		if asks, found := persona.SampleAsks[printer.Code()]; found && len(asks) > 0 {
			return asks
		}
	}
	german := printer.Is("de")
	switch {
	case p.Role == "executor":
		if german {
			return []string{"Welche Aufträge passen zu meiner Kolonne?", "Wer sucht Stahlbau in den Niederlanden?", "Welche Papiere fehlen mir noch?"}
		}
		return []string{"Which jobs fit my crew?", "Who needs steel work in NL?", "Which papers am I missing?"}
	case p.Role == "owner":
		if german {
			return []string{"Elektrokolonne für München ab Oktober", "Wer kann Sanitär in Wien?", "Zeig mir meine Aufträge"}
		}
		return []string{"Electrical crew for Munich from October", "Who can do sanitary in Vienna?", "Show me my offers"}
	default:
		if german {
			return []string{"Ich brauche 6 Monteure in München", "Meine Kolonne macht Elektro im DACH-Raum", "Wie funktioniert das hier?"}
		}
		return []string{"I need 6 fitters in Munich", "My crew does electrical work in DACH", "How does this work?"}
	}
}

// profileLine renders the one-line summary the greeting uses, in the visitor's
// language, so a returning visitor immediately sees what the app has on them.
func profileLine(p model.Profile, printer i18n.Printer) string {
	if !p.Known() {
		return ""
	}
	parts := []string{printer.T("role." + p.Role)}
	if parts[0] == "role."+p.Role {
		parts[0] = p.Role
	}
	for _, trade := range p.Trades {
		if label := printer.T("trade." + trade); label != "trade."+trade {
			parts = append(parts, label)
		} else {
			parts = append(parts, trade)
		}
	}
	if len(p.Regions) > 0 {
		parts = append(parts, p.Regions[0])
	}
	if p.CrewSize > 0 {
		parts = append(parts, itoa(p.CrewSize)+" "+printer.T("offer.crew"))
	}
	return strings.Join(parts, " · ")
}

func itoa(n int) string { return strconv.Itoa(n) }

func (h *Handler) about(w http.ResponseWriter, r *http.Request) {
	data, err := h.dashboard(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.page.Execute(w, data); err != nil {
		log.Printf("web: render about: %v", err)
	}
}

// offers serves the pipeline. An htmx request (a filter, a create, a status
// change) gets the fragment it swaps in; a direct visit gets the standalone
// pipeline page the fragment lives inside — so /offers is a real page, not
// just a partial only reachable from inside another one.
func (h *Handler) offers(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") != "true" {
		data, err := h.dashboard(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := h.pipeline.ExecuteTemplate(w, "pipeline", data); err != nil {
			log.Printf("web: render pipeline: %v", err)
		}
		return
	}
	h.renderPartial(w, r, "offers")
}

func (h *Handler) perspective(w http.ResponseWriter, r *http.Request) {
	h.renderPartial(w, r, "perspective")
}

func (h *Handler) renderPartial(w http.ResponseWriter, r *http.Request, name string) {
	data, err := h.dashboard(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.part.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("web: render %s: %v", name, err)
	}
}

// createOffer accepts the htmx "new offer" form and re-renders the pipeline.
func (h *Handler) createOffer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	progress, _ := strconv.Atoi(r.FormValue("progress"))
	offer := model.Offer{
		ID:        "MM-" + strconv.FormatInt(time.Now().Unix()%100000, 10),
		Title:     title,
		Location:  strings.TrimSpace(r.FormValue("location")),
		Category:  strings.TrimSpace(r.FormValue("category")),
		Amount:    strings.TrimSpace(r.FormValue("amount")),
		Budget:    strings.TrimSpace(r.FormValue("budget")),
		Supplier:  strings.TrimSpace(r.FormValue("supplier")),
		Status:    firstNonEmpty(r.FormValue("status"), "open"),
		Signal:    "OK",
		Progress:  progress,
		Attention: strings.TrimSpace(r.FormValue("attention")),
	}
	if _, err := h.deps.Store.CreateOffer(r.Context(), offer); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, r, "offers")
}

// advanceOffer moves one offer to another pipeline status and re-renders the
// pipeline. It is the minimum write path an operator needs: an offer that can
// only be created is not a pipeline.
func (h *Handler) advanceOffer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id := strings.TrimSpace(r.FormValue("id"))
	status := strings.ToLower(strings.TrimSpace(r.FormValue("status")))
	if !validStatus[status] {
		http.Error(w, "unknown status", http.StatusBadRequest)
		return
	}
	offer, err := h.deps.Store.GetOffer(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	offer.Status = status
	switch status {
	case "done":
		offer.Progress = 100
		offer.Signal = "Review"
	case "open":
		offer.Signal = "OK"
	}
	if _, err := h.deps.Store.UpdateOffer(r.Context(), offer); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, r, "offers")
}

var validStatus = map[string]bool{"open": true, "requested": true, "process": true, "done": true}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func (h *Handler) dashboard(r *http.Request) (Dashboard, error) {
	ctx := r.Context()
	view := firstNonEmpty(r.URL.Query().Get("view"), "all")
	role := firstNonEmpty(r.URL.Query().Get("role"), "owner")
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	offers, err := h.deps.Store.ListOffers(ctx, store.OfferFilter{Status: view, Query: query})
	if err != nil {
		return Dashboard{}, err
	}
	counts, err := h.deps.Store.CountOffersByStatus(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	perspectives, err := h.deps.Store.ListPerspectives(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	modules, err := h.deps.Store.ListModules(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	roadmap, err := h.deps.Store.ListRoadmap(ctx)
	if err != nil {
		return Dashboard{}, err
	}

	perspective := model.Perspective{}
	if len(perspectives) > 0 {
		perspective = perspectives[0]
		for _, p := range perspectives {
			if p.Key == role {
				perspective = p
				break
			}
		}
	}

	spotlight := h.spotlight(ctx, offers)

	return Dashboard{
		Now:          time.Now().Format("02 Jan 2006 15:04"),
		View:         view,
		Query:        query,
		Role:         perspective.Key,
		Version:      h.deps.Version,
		LLMModel:     h.deps.LLMModel,
		Offers:       offers,
		Counts:       counts,
		Spotlight:    spotlight,
		Perspective:  perspective,
		Perspectives: perspectives,
		Modules:      modules,
		Roadmap:      roadmap,
		Statuses:     []string{"open", "requested", "process", "done"},
	}, nil
}

// spotlight prefers the most urgent offer, falling back to any offer at all.
func (h *Handler) spotlight(ctx context.Context, filtered []model.Offer) model.Offer {
	all, err := h.deps.Store.ListOffers(ctx, store.OfferFilter{})
	if err != nil || len(all) == 0 {
		if len(filtered) > 0 {
			return filtered[0]
		}
		return model.Offer{Title: "No offers yet", Attention: "Create the first offer in the pipeline."}
	}
	for _, o := range all {
		if o.Signal == "Attention" {
			return o
		}
	}
	return all[0]
}
