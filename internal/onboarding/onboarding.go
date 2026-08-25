// Package onboarding turns a first visit into a profile, by conversation.
//
// First principle: never ask a human to fill in a taxonomy the machine can
// infer. The visitor writes one sentence in their own words; the local model
// extracts whatever fields it can see; onboarding only asks about what is
// still genuinely unknown, one thing at a time.
//
// This is the phase-0 baseline: cookie-backed profile, LLM extraction with a
// mechanical fallback, and the next-question walk. Streaming, richer
// summaries and the profile editor are the onboarding worker's job.
package onboarding

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"mm-machine/internal/app"
	"mm-machine/internal/llm"
	"mm-machine/internal/model"
	"mm-machine/internal/store"
)

// CookieName carries the profile id. It is deliberately not a login: the app
// remembers a visitor without asking them to become an account first.
const CookieName = "mm_profile"

// Handler serves the onboarding thread.
type Handler struct {
	deps app.Deps
	tpl  *template.Template
}

// Register wires the onboarding routes onto mux.
func Register(mux *http.ServeMux, deps app.Deps) *Handler {
	h := &Handler{deps: deps, tpl: template.Must(template.New("onboarding").Parse(threadHTML + profileHTML))}
	mux.HandleFunc("/start", h.start)
	mux.HandleFunc("/start/turn", h.Turn)
	mux.HandleFunc("/start/profile", h.profilePanel)
	mux.HandleFunc("/start/reset", h.reset)
	return h
}

func newID() string {
	var b [10]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// Ensure returns the visitor's profile, creating one (and setting the cookie)
// on first contact.
func Ensure(w http.ResponseWriter, r *http.Request, deps app.Deps) (model.Profile, error) {
	if c, err := r.Cookie(CookieName); err == nil && c.Value != "" {
		p, err := deps.Store.GetProfile(r.Context(), c.Value)
		if err == nil {
			return p, nil
		}
	}
	p := model.Profile{ID: newID(), Role: "unknown", CreatedAt: time.Now()}
	saved, err := deps.Store.UpsertProfile(r.Context(), p)
	if err != nil {
		return model.Profile{}, err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    saved.ID,
		Path:     "/",
		MaxAge:   int((90 * 24 * time.Hour).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return saved, nil
}

// Current returns the profile behind the request without creating one.
func Current(r *http.Request, deps app.Deps) (model.Profile, bool) {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return model.Profile{}, false
	}
	p, err := deps.Store.GetProfile(r.Context(), c.Value)
	if err != nil {
		return model.Profile{}, false
	}
	return p, true
}

// NextQuestion returns the question to ask next, and whether onboarding is
// done. The order follows model.ProfileFields so the progress meter is honest.
func NextQuestion(p model.Profile) (string, bool) {
	for _, field := range model.ProfileFields {
		switch field {
		case "role":
			if p.Role == "" || p.Role == "unknown" {
				return "Are you looking for a crew, or looking for work?", false
			}
		case "trades":
			if len(p.Trades) == 0 {
				if p.Role == "executor" {
					return "What trades does your crew cover?", false
				}
				return "What kind of work do you need done?", false
			}
		case "regions":
			if len(p.Regions) == 0 {
				return "Which region — city or country?", false
			}
		case "crewSize":
			if p.CrewSize == 0 {
				if p.Role == "executor" {
					return "How many people can you field?", false
				}
				return "How many people do you need?", false
			}
		case "documents":
			if len(p.Documents) == 0 {
				return "Which papers are ready — A1, insurance, certificates?", false
			}
		case "availability":
			if strings.TrimSpace(p.Availability) == "" {
				return "When does it start, and for how long?", false
			}
		}
	}
	return "", true
}

const extractionSchema = `{"role":"owner|executor|unknown","trades":["slug"],"regions":["City, CC"],` +
	`"crewSize":0,"languages":["de"],"documents":["a1|insurance|certificates|tax"],` +
	`"availability":"free text","company":"","notes":""}`

// Extract merges whatever the model can see in one message into the profile.
// Fields it cannot see must come back empty — inventing a trade is worse than
// asking one more question.
func Extract(ctx context.Context, deps app.Deps, p model.Profile, message string) model.Profile {
	if deps.LLM == nil {
		return mergeMechanical(p, message)
	}
	var got struct {
		Role         string   `json:"role"`
		Trades       []string `json:"trades"`
		Regions      []string `json:"regions"`
		CrewSize     int      `json:"crewSize"`
		Languages    []string `json:"languages"`
		Documents    []string `json:"documents"`
		Availability string   `json:"availability"`
		Company      string   `json:"company"`
		Notes        string   `json:"notes"`
	}
	req := llm.Request{
		MaxTokens:   1024,
		Temperature: 0.1,
		Messages: []llm.Message{
			{Role: "system", Content: "You extract structured facts from one message a construction professional wrote about themselves. " +
				"Only fill a field if the message actually says it. Leave everything else empty or zero. " +
				"role: owner = needs a crew (Generalunternehmer), executor = offers work (Subunternehmer). " +
				"trades: lowercase slugs from electrical, sanitary, steel, interior, energy, drywall, hvac."},
			{Role: "user", Content: message},
		},
	}
	if err := deps.LLM.JSON(ctx, req, extractionSchema, &got); err != nil {
		log.Printf("onboarding: extraction failed, falling back: %v", err)
		return mergeMechanical(p, message)
	}
	if got.Role == "owner" || got.Role == "executor" {
		p.Role = got.Role
	}
	p.Trades = mergeList(p.Trades, got.Trades)
	p.Regions = mergeList(p.Regions, got.Regions)
	p.Languages = mergeList(p.Languages, got.Languages)
	p.Documents = mergeList(p.Documents, got.Documents)
	if got.CrewSize > 0 {
		p.CrewSize = got.CrewSize
	}
	if strings.TrimSpace(got.Availability) != "" {
		p.Availability = strings.TrimSpace(got.Availability)
	}
	if strings.TrimSpace(got.Company) != "" {
		p.Company = strings.TrimSpace(got.Company)
	}
	if n := strings.TrimSpace(got.Notes); n != "" && !strings.Contains(p.Notes, n) {
		p.Notes = strings.TrimSpace(p.Notes + "\n" + n)
	}
	return p
}

// mergeMechanical is the no-model path: crude keyword spotting, so onboarding
// still moves forward when the cluster is down.
func mergeMechanical(p model.Profile, message string) model.Profile {
	m := strings.ToLower(message)
	switch {
	case strings.Contains(m, "need") || strings.Contains(m, "hire") || strings.Contains(m, "looking for a crew"):
		if p.Role == "" || p.Role == "unknown" {
			p.Role = "owner"
		}
	case strings.Contains(m, "offer") || strings.Contains(m, "my crew") || strings.Contains(m, "looking for work"):
		if p.Role == "" || p.Role == "unknown" {
			p.Role = "executor"
		}
	}
	for _, trade := range []string{"electrical", "sanitary", "steel", "interior", "energy", "drywall", "hvac"} {
		if strings.Contains(m, trade) {
			p.Trades = mergeList(p.Trades, []string{trade})
		}
	}
	for _, doc := range []string{"a1", "insurance", "certificates", "tax"} {
		if strings.Contains(m, doc) {
			p.Documents = mergeList(p.Documents, []string{doc})
		}
	}
	if p.Notes == "" {
		p.Notes = message
	}
	return p
}

func mergeList(have, add []string) []string {
	for _, a := range add {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == "" {
			continue
		}
		found := false
		for _, h := range have {
			if strings.EqualFold(h, a) {
				found = true
				break
			}
		}
		if !found {
			have = append(have, a)
		}
	}
	return have
}

// --- handlers ---------------------------------------------------------------

type threadView struct {
	Profile  model.Profile
	Question string
	Done     bool
	Echo     string
	Progress int
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	p, err := Ensure(w, r, h.deps)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	question, done := NextQuestion(p)
	h.render(w, threadView{Profile: p, Question: question, Done: done, Progress: store.Completeness(p)})
}

// Turn takes one message, learns from it, and asks the next open question.
// It is exported so the /ask router can send a first-time visitor here.
func (h *Handler) Turn(w http.ResponseWriter, r *http.Request) {
	message := strings.TrimSpace(firstNonEmpty(r.FormValue("message"), r.URL.Query().Get("message")))
	p, err := Ensure(w, r, h.deps)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if message == "" {
		question, done := NextQuestion(p)
		h.render(w, threadView{Profile: p, Question: question, Done: done, Progress: store.Completeness(p)})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	p = Extract(ctx, h.deps, p, message)
	saved, err := h.deps.Store.UpsertProfile(ctx, p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	question, done := NextQuestion(saved)
	h.render(w, threadView{Profile: saved, Question: question, Done: done, Echo: message, Progress: saved.Completeness})
}

func (h *Handler) profilePanel(w http.ResponseWriter, r *http.Request) {
	p, ok := Current(r, h.deps)
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tpl.ExecuteTemplate(w, "profile", p); err != nil {
		log.Printf("onboarding: render profile: %v", err)
	}
}

func (h *Handler) reset(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) render(w http.ResponseWriter, v threadView) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if v.Echo != "" {
		fmt.Fprintf(w, `<div class="mm-msg you"><span class="mm-who">you</span><p>%s</p></div>`, html.EscapeString(v.Echo))
	}
	if err := h.tpl.ExecuteTemplate(w, "thread", v); err != nil {
		log.Printf("onboarding: render thread: %v", err)
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
