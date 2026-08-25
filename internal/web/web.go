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
	"mm-machine/internal/model"
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
}

// Handler serves the public surface.
type Handler struct {
	deps app.Deps
	page *template.Template
	part *template.Template
}

var funcs = template.FuncMap{
	"lower": strings.ToLower,
	"itoa":  strconv.Itoa,
}

// Register wires the public routes onto mux and returns the handler.
func Register(mux *http.ServeMux, deps app.Deps) *Handler {
	h := &Handler{
		deps: deps,
		page: template.Must(template.New("page").Funcs(funcs).Parse(pageHTML + offersHTML + perspectiveHTML)),
		part: template.Must(template.New("partial").Funcs(funcs).Parse(offersHTML + perspectiveHTML)),
	}
	mux.Handle("/static/", http.FileServer(http.FS(staticFS)))
	mux.HandleFunc("/", h.home)
	mux.HandleFunc("/offers", h.offers)
	mux.HandleFunc("/perspective", h.perspective)
	mux.HandleFunc("/offers/new", h.createOffer)
	return h
}

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := h.dashboard(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.page.Execute(w, data); err != nil {
		log.Printf("web: render home: %v", err)
	}
}

func (h *Handler) offers(w http.ResponseWriter, r *http.Request) {
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
