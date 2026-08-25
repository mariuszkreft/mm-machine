// Package search answers a sentence, not a filter form.
//
// The split is deliberate and load-bearing:
//   - the model TRANSLATES  (free text -> model.Intent) and EXPLAINS (a summary line),
//   - deterministic code FILTERS and RANKS (store query -> scored model.Match).
//
// The model never decides which rows exist, so a hallucination can only ever
// cost a bad phrasing, never a wrong result set. When the model is
// unavailable, parsing falls back to keyword spotting and the surface says so.
package search

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"mm-machine/internal/app"
	"mm-machine/internal/llm"
	"mm-machine/internal/model"
	"mm-machine/internal/onboarding"
	"mm-machine/internal/store"
)

// Handler serves the search surface.
type Handler struct {
	deps app.Deps
	tpl  *template.Template
}

// Register wires the search routes onto mux.
func Register(mux *http.ServeMux, deps app.Deps) *Handler {
	h := &Handler{deps: deps, tpl: parseTemplates()}
	mux.HandleFunc("/find", h.Query)
	mux.HandleFunc("/find/save", h.save)
	mux.HandleFunc("/find/saved", h.saved)
	mux.HandleFunc("/find/saved/delete", h.deleteSaved)
	mux.HandleFunc("/find/stream", h.stream)
	return h
}

// knownTrades and knownDocs keep the mechanical fallback and the prompt in sync.
var (
	knownTrades = []string{"electrical", "sanitary", "steel", "interior", "energy", "drywall", "hvac"}
	knownDocs   = []string{"a1", "insurance", "certificates", "tax"}
	knownStatus = []string{"open", "requested", "process", "done"}
)

const intentSchema = `{"kind":"find_offers|find_crews|post_job|help","trades":["slug"],` +
	`"regions":["City"],"statuses":["open"],"keywords":["word"],"documents":["a1"],` +
	`"crewSize":0,"timeframe":"","budgetHint":"","confidence":0.0}`

// Parse turns raw text into an Intent and its facets, using the profile as
// context for what the visitor probably means. Dates, durations, budget
// bounds and negations are read from the raw sentence deterministically, on
// both the model and mechanical paths, so neither needs the other to
// understand them.
func Parse(ctx context.Context, deps app.Deps, raw string, p model.Profile) (model.Intent, facets) {
	raw = strings.TrimSpace(raw)
	fc := deriveFacets(raw, time.Now())
	if deps.LLM == nil {
		return finishParse(ParseMechanical(raw), fc), fc
	}
	var got struct {
		Kind       string   `json:"kind"`
		Trades     []string `json:"trades"`
		Regions    []string `json:"regions"`
		Statuses   []string `json:"statuses"`
		Keywords   []string `json:"keywords"`
		Documents  []string `json:"documents"`
		CrewSize   int      `json:"crewSize"`
		Timeframe  string   `json:"timeframe"`
		BudgetHint string   `json:"budgetHint"`
		Confidence float64  `json:"confidence"`
	}
	system := "You turn one sentence from a construction professional into a search intent. " +
		"Only fill fields the sentence supports; leave the rest empty. " +
		"trades must come from: " + strings.Join(knownTrades, ", ") + ". " +
		"statuses must come from: " + strings.Join(knownStatus, ", ") + ". " +
		"documents must come from: " + strings.Join(knownDocs, ", ") + ". " +
		"Ignore anything the sentence explicitly negates (\"not Vienna\") — never put a negated word in trades or regions."
	if p.Role != "" && p.Role != "unknown" {
		system += fmt.Sprintf(" The person is a %q; default kind accordingly (owner looks for crews, executor looks for offers).", p.Role)
	}
	req := llm.Request{
		MaxTokens:   768,
		Temperature: 0.1,
		Messages: []llm.Message{
			{Role: "system", Content: system},
			{Role: "user", Content: raw},
		},
	}
	if err := deps.LLM.JSON(ctx, req, intentSchema, &got); err != nil {
		log.Printf("search: intent parse failed, falling back: %v", err)
		return finishParse(ParseMechanical(raw), fc), fc
	}
	intent := model.Intent{
		Raw:        raw,
		Kind:       got.Kind,
		Trades:     normalize(got.Trades, knownTrades),
		Regions:    trimAll(got.Regions),
		Statuses:   normalize(got.Statuses, knownStatus),
		Keywords:   trimAll(got.Keywords),
		Documents:  normalize(got.Documents, knownDocs),
		CrewSize:   got.CrewSize,
		Timeframe:  strings.TrimSpace(got.Timeframe),
		BudgetHint: strings.TrimSpace(got.BudgetHint),
		Confidence: got.Confidence,
	}
	if intent.Kind == "" {
		intent.Kind = "find_offers"
	}
	return finishParse(intent, fc), fc
}

// finishParse strips anything the sentence explicitly negated out of the
// positive facet lists, regardless of which path (model or mechanical) put
// it there.
func finishParse(intent model.Intent, fc facets) model.Intent {
	intent.Trades = stripNegated(intent.Trades, fc.ExcludeTrades)
	intent.Regions = stripNegated(intent.Regions, fc.ExcludeRegions)
	return intent
}

// ParseMechanical is the no-model path: keyword spotting only, flagged so the
// UI can admit it is running degraded.
func ParseMechanical(raw string) model.Intent {
	low := strings.ToLower(raw)
	intent := model.Intent{Raw: raw, Kind: "find_offers", Fallback: true, Confidence: 0.3}
	for _, t := range knownTrades {
		if strings.Contains(low, t) {
			intent.Trades = append(intent.Trades, t)
		}
	}
	for _, d := range knownDocs {
		if strings.Contains(low, d) {
			intent.Documents = append(intent.Documents, d)
		}
	}
	for _, s := range knownStatus {
		if strings.Contains(low, s) {
			intent.Statuses = append(intent.Statuses, s)
		}
	}
	intent.Keywords = strings.Fields(low)
	return intent
}

func normalize(got, allowed []string) []string {
	out := []string{}
	for _, g := range got {
		g = strings.ToLower(strings.TrimSpace(g))
		for _, a := range allowed {
			if g == a {
				out = append(out, a)
			}
		}
	}
	return out
}

func trimAll(in []string) []string {
	out := []string{}
	for _, v := range in {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// Result is the whole answer to one search: what was understood, what
// matched, and how honest the process had to be about widening to get
// there.
type Result struct {
	Intent  model.Intent
	Facets  facets
	Matches []model.Match // full ranked set, not truncated for display
	Widen   widenInfo
}

// Search runs the filter/widen/rank pipeline for an already-parsed intent.
// Split out from Run so a refine chip can override one facet and re-run
// this without asking the model to parse the sentence again.
func Search(ctx context.Context, deps app.Deps, intent model.Intent, fc facets, p model.Profile) (Result, error) {
	base := store.OfferFilter{
		Trades:       intent.Trades,
		Regions:      intent.Regions,
		Statuses:     intent.Statuses,
		Requirements: intent.Documents,
		Limit:        50,
	}
	offers, err := deps.Store.ListOffers(ctx, base)
	if err != nil {
		return Result{}, err
	}
	offers = applyPostFilter(offers, fc)

	var widen widenInfo
	if len(offers) == 0 {
		offers, widen, err = widenSearch(ctx, deps, base, fc)
		if err != nil {
			return Result{}, err
		}
	}
	return Result{Intent: intent, Facets: fc, Matches: Rank(offers, intent, fc, p), Widen: widen}, nil
}

// Run is the whole pipeline: parse, then Search. Exported so the assistant and
// any future surface can search without going through HTTP.
func Run(ctx context.Context, deps app.Deps, raw string, p model.Profile) (Result, error) {
	intent, fc := Parse(ctx, deps, raw, p)
	return Search(ctx, deps, intent, fc, p)
}

// --- handlers ---------------------------------------------------------------

const displayLimit = 6

type resultsView struct {
	Intent    model.Intent
	Matches   []model.Match
	Query     string
	Degraded  bool
	Widen     widenInfo
	Chips     []chip
	StreamURL string
	Fallback  string
}

// Query answers a natural-language search and renders the result cards. It is
// exported so the /ask router can send a known visitor straight here.
func (h *Handler) Query(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSpace(firstNonEmpty(r.FormValue("message"), r.FormValue("q"), r.URL.Query().Get("q")))
	if raw == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	p, _ := onboarding.Current(r, h.deps)

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	intent, fc := Parse(ctx, h.deps, raw, p)
	if rf, ok := parseRefine(r.FormValue("refine")); ok {
		intent = applyRefine(intent, rf)
	}
	result, err := Search(ctx, h.deps, intent, fc, p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// A Generalunternehmer asking for people wants the supply side: crews,
	// not more job postings.
	if WantsCrews(result.Intent, p) {
		crewMatches, crewErr := RunCrews(ctx, h.deps, result.Intent, p)
		if crewErr != nil {
			http.Error(w, crewErr.Error(), http.StatusInternalServerError)
			return
		}
		if len(crewMatches) > 0 {
			result.Matches = crewMatches
		}
	}

	display := result.Matches
	if len(display) > displayLimit {
		display = display[:displayLimit]
	}
	brief := toBrief(display)
	briefJSON, _ := json.Marshal(brief)
	streamURL := "/find/stream?" + url.Values{
		"q":       {raw},
		"matches": {string(briefJSON)},
		"trades":  {strings.Join(result.Intent.Trades, ",")},
		"regions": {strings.Join(result.Intent.Regions, ",")},
	}.Encode()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<div class="mm-msg you"><span class="mm-who">you</span><p>%s</p></div>`, html.EscapeString(raw))
	view := resultsView{
		Intent:    result.Intent,
		Matches:   display,
		Query:     raw,
		Degraded:  result.Intent.Fallback,
		Widen:     result.Widen,
		Chips:     buildChips(result.Matches, result.Intent),
		StreamURL: streamURL,
		Fallback:  mechanicalSummary(raw, result.Intent, brief),
	}
	if err := h.tpl.ExecuteTemplate(w, "results", view); err != nil {
		log.Printf("search: render results: %v", err)
	}

	if saved, err := h.deps.Store.ListSavedSearches(ctx, p.ID); err == nil {
		_ = h.tpl.ExecuteTemplate(w, "saved", savedView{Searches: saved})
	}
}

func (h *Handler) save(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	p, ok := onboarding.Current(r, h.deps)
	if !ok {
		http.Error(w, "no profile", http.StatusBadRequest)
		return
	}
	query := strings.TrimSpace(r.FormValue("q"))
	if query == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if _, err := h.deps.Store.SaveSearch(r.Context(), model.SavedSearch{ProfileID: p.ID, Label: query, Query: query}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeBadge(w, "good", "saved")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
