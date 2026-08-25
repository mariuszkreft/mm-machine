package demo

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"mm-machine/internal/app"
	"mm-machine/internal/i18n"
	"mm-machine/internal/model"
	"mm-machine/internal/onboarding"
	"mm-machine/internal/store"
)

// prevCookie remembers the visitor's own profile while they look around as an
// example persona, so stepping out puts them back where they were.
const prevCookie = "mm_profile_prev"

// Handler serves the example-profile surface.
type Handler struct {
	deps app.Deps
	tpl  *template.Template
}

// Register wires the demo routes onto mux.
func Register(mux *http.ServeMux, deps app.Deps) *Handler {
	h := &Handler{deps: deps, tpl: template.Must(template.New("demo").Parse(pageHTML))}
	mux.HandleFunc("/demo", h.page)
	mux.HandleFunc("/demo/enter", h.enter)
	mux.HandleFunc("/demo/leave", h.leave)
	return h
}

// Seed fills an empty store with the demo market. It is idempotent: an offer
// that already exists is left alone, so a restart never duplicates and an
// operator's own edits are never overwritten. A crew that already exists is
// merged field by field: only fields still equal to what Seed would have
// written are refreshed, so a field an operator changed away from the seed
// value is left as they set it.
//
// Setting MM_DEMO=off turns the demo market off for a real deployment: Seed
// returns immediately without creating anything. It never deletes what is
// already there, seeded or not, so flipping the switch after go-live is safe.
func Seed(ctx context.Context, s store.Store) error {
	if os.Getenv("MM_DEMO") == "off" {
		return nil
	}
	for _, o := range Offers() {
		if _, err := s.GetOffer(ctx, o.ID); err == nil {
			continue
		} else if !errors.Is(err, store.ErrNotFound) {
			return err
		}
		if _, err := s.CreateOffer(ctx, o); err != nil {
			return err
		}
	}
	for _, c := range Crews() {
		existing, err := s.GetCrew(ctx, c.ID)
		if errors.Is(err, store.ErrNotFound) {
			if _, err := s.UpsertCrew(ctx, c); err != nil {
				return err
			}
			continue
		} else if err != nil {
			return err
		}
		if _, err := s.UpsertCrew(ctx, mergeSeededCrew(existing, c)); err != nil {
			return err
		}
	}
	return nil
}

// mergeSeededCrew starts from the stored record and only pulls in a field
// from the seed value when the stored field still matches it, i.e. nothing
// has touched it since Seed last wrote it. A field that differs was changed
// by an operator (or another seed run with different data) and is kept as
// stored.
func mergeSeededCrew(stored, seed model.Crew) model.Crew {
	merged := stored
	if stored.Name == seed.Name {
		merged.Name = seed.Name
	}
	if stored.Company == seed.Company {
		merged.Company = seed.Company
	}
	if slices.Equal(stored.Trades, seed.Trades) {
		merged.Trades = seed.Trades
	}
	if slices.Equal(stored.Regions, seed.Regions) {
		merged.Regions = seed.Regions
	}
	if stored.Size == seed.Size {
		merged.Size = seed.Size
	}
	if slices.Equal(stored.Languages, seed.Languages) {
		merged.Languages = seed.Languages
	}
	if slices.Equal(stored.Documents, seed.Documents) {
		merged.Documents = seed.Documents
	}
	if stored.AvailableFrom.Equal(seed.AvailableFrom) {
		merged.AvailableFrom = seed.AvailableFrom
	}
	if stored.AvailableNote == seed.AvailableNote {
		merged.AvailableNote = seed.AvailableNote
	}
	if stored.Rate == seed.Rate {
		merged.Rate = seed.Rate
	}
	if stored.Rating == seed.Rating {
		merged.Rating = seed.Rating
	}
	if stored.JobsDone == seed.JobsDone {
		merged.JobsDone = seed.JobsDone
	}
	if stored.Note == seed.Note {
		merged.Note = seed.Note
	}
	return merged
}

type personaView struct {
	Key        string
	Label      string
	Summary    string
	Sees       string
	Role       string
	Trades     []string
	Regions    []string
	CrewSize   int
	Documents  []string
	SampleAsks []string
}

type pageView struct {
	T         i18n.Printer
	Personas  []personaView
	Active    string
	Changes   string
	Unchanged string
}

func (h *Handler) view(r *http.Request) pageView {
	p := i18n.NewPrinter(r)
	lang := string(p.Lang)
	out := pageView{T: p, Changes: pick(demoCopy["changes"], lang), Unchanged: pick(demoCopy["unchanged"], lang)}
	for _, persona := range Personas() {
		out.Personas = append(out.Personas, personaView{
			Key:        persona.Key,
			Label:      persona.Label,
			Summary:    pick(persona.Summary, lang),
			Sees:       seesText(persona, lang),
			Role:       persona.Profile.Role,
			Trades:     persona.Profile.Trades,
			Regions:    persona.Profile.Regions,
			CrewSize:   persona.Profile.CrewSize,
			Documents:  persona.Profile.Documents,
			SampleAsks: pickList(persona.SampleAsks, lang),
		})
	}
	if c, err := r.Cookie(onboarding.CookieName); err == nil && len(c.Value) > 5 && c.Value[:5] == "demo-" {
		out.Active = c.Value[5:]
	}
	return out
}

// demoCopy is the /demo page's own teaching text. It stays package-local
// rather than in the shared i18n catalog because it belongs only to this
// surface. Both languages are listed here for the report.
var demoCopy = map[string]map[string]string{
	"changes": {
		"de": "Beim Einsteigen wird Ihr eigenes Profil beiseitegelegt und durch das Beispielprofil ersetzt, und die Sprache wechselt auf die des Beispiels.",
		"en": "Stepping in puts your own profile aside and swaps in the example's, and switches the language to the example's.",
	},
	"unchanged": {
		"de": "Nichts davon ist dauerhaft: Ihr eigenes Profil bleibt gespeichert und kommt zurück, sobald Sie das Beispiel verlassen.",
		"en": "None of it is permanent: your own profile stays saved and comes back the moment you leave the example.",
	},
}

// seesText describes what a visitor's pipeline looks like the moment they
// step into a persona, so the teaching page can say it instead of leaving it
// implicit.
func seesText(persona model.Persona, lang string) string {
	p := persona.Profile
	trades := strings.Join(p.Trades, "/")
	regions := strings.Join(p.Regions, ", ")
	if p.Role == "owner" {
		if lang == "en" {
			return fmt.Sprintf("Their pipeline filtered to %s in %s, and the crews that fit it.", trades, regions)
		}
		return fmt.Sprintf("Ihre Pipeline gefiltert nach %s in %s, dazu passende Kolonnen.", trades, regions)
	}
	if lang == "en" {
		return fmt.Sprintf("Open offers matching %s in %s.", trades, regions)
	}
	return fmt.Sprintf("Offene Aufträge passend zu %s in %s.", trades, regions)
}

func pick(m map[string]string, lang string) string {
	if v, ok := m[lang]; ok {
		return v
	}
	return m[string(i18n.Default)]
}

func pickList(m map[string][]string, lang string) []string {
	if v, ok := m[lang]; ok {
		return v
	}
	return m[string(i18n.Default)]
}

func (h *Handler) page(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tpl.ExecuteTemplate(w, "demo", h.view(r)); err != nil {
		log.Printf("demo: render: %v", err)
	}
}

// enter steps into a persona: it creates (or refreshes) the persona's profile,
// points the visitor's profile cookie at it, and switches the language to the
// one the persona lives in.
func (h *Handler) enter(w http.ResponseWriter, r *http.Request) {
	key := r.FormValue("persona")
	if key == "" {
		key = r.URL.Query().Get("persona")
	}
	persona, ok := Persona(key)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if cur, err := r.Cookie(onboarding.CookieName); err == nil && cur.Value != "" && !isDemo(cur.Value) {
		http.SetCookie(w, &http.Cookie{Name: prevCookie, Value: cur.Value, Path: "/", MaxAge: int((24 * time.Hour).Seconds()), HttpOnly: true, SameSite: http.SameSiteLaxMode})
	}

	profile := persona.Profile
	profile.ID = "demo-" + persona.Key
	if _, err := h.deps.Store.UpsertProfile(r.Context(), profile); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: onboarding.CookieName, Value: profile.ID, Path: "/", MaxAge: int((24 * time.Hour).Seconds()), HttpOnly: true, SameSite: http.SameSiteLaxMode})
	if lang := i18n.Lang(persona.Lang); i18n.Valid(lang) {
		i18n.Set(w, lang)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// leave restores the visitor's own profile.
func (h *Handler) leave(w http.ResponseWriter, r *http.Request) {
	restore := ""
	if c, err := r.Cookie(prevCookie); err == nil {
		restore = c.Value
	}
	if restore != "" {
		http.SetCookie(w, &http.Cookie{Name: onboarding.CookieName, Value: restore, Path: "/", MaxAge: int((90 * 24 * time.Hour).Seconds()), HttpOnly: true, SameSite: http.SameSiteLaxMode})
	} else {
		http.SetCookie(w, &http.Cookie{Name: onboarding.CookieName, Value: "", Path: "/", MaxAge: -1})
	}
	http.SetCookie(w, &http.Cookie{Name: prevCookie, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func isDemo(id string) bool { return len(id) > 5 && id[:5] == "demo-" }

// ActivePersona reports the persona a request is currently viewing as, if any.
// The shell uses it to show the "you are in an example" banner.
func ActivePersona(r *http.Request) (model.Persona, bool) {
	c, err := r.Cookie(onboarding.CookieName)
	if err != nil || !isDemo(c.Value) {
		return model.Persona{}, false
	}
	return Persona(c.Value[5:])
}
