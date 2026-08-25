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
	"fmt"
	"html"
	"html/template"
	"log"
	"net/http"
	"sort"
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
	h := &Handler{deps: deps, tpl: template.Must(template.New("search").Parse(resultsHTML))}
	mux.HandleFunc("/find", h.Query)
	mux.HandleFunc("/find/save", h.save)
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

// Parse turns raw text into an Intent, using the profile as context for what
// the visitor probably means.
func Parse(ctx context.Context, deps app.Deps, raw string, p model.Profile) model.Intent {
	raw = strings.TrimSpace(raw)
	if deps.LLM == nil {
		return ParseMechanical(raw)
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
		"documents must come from: " + strings.Join(knownDocs, ", ") + "."
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
		return ParseMechanical(raw)
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

// Rank scores offers against the intent and the profile. Scoring is
// deterministic and every point is turned into a reason, because a fit number
// nobody can explain is worse than no number.
func Rank(offers []model.Offer, intent model.Intent, p model.Profile) []model.Match {
	matches := make([]model.Match, 0, len(offers))
	for _, o := range offers {
		score := 40 // a listed offer already matched the hard filters
		why := []string{}

		if hasFold(intent.Trades, trade(o)) {
			score += 20
			why = append(why, "trade matches "+trade(o))
		}
		for _, region := range intent.Regions {
			if containsFold(region, o.Region) || containsFold(region, o.Location) {
				score += 15
				why = append(why, "in "+o.Location)
				break
			}
		}
		if len(intent.Documents) > 0 && containsAll(o.Requirements, intent.Documents) {
			score += 10
			why = append(why, "papers required: "+strings.Join(o.Requirements, ", "))
		}
		if intent.CrewSize > 0 && o.CrewSize > 0 {
			switch {
			case o.CrewSize >= intent.CrewSize:
				score += 10
				why = append(why, fmt.Sprintf("crew of %d covers the %d asked for", o.CrewSize, intent.CrewSize))
			default:
				score -= 5
				why = append(why, fmt.Sprintf("crew of %d is short of %d", o.CrewSize, intent.CrewSize))
			}
		}
		for _, kw := range intent.Keywords {
			if len(kw) > 3 && containsFold(kw, o.Title+" "+o.Category+" "+o.Supplier) {
				score += 3
				why = append(why, "mentions "+kw)
				break
			}
		}
		if len(p.Regions) > 0 {
			for _, region := range p.Regions {
				if containsFold(region, o.Region) || containsFold(region, o.Location) {
					score += 5
					why = append(why, "near your region")
					break
				}
			}
		}
		if o.Signal == "Attention" {
			score += 3
			why = append(why, "needs attention now")
		}

		if score > 100 {
			score = 100
		}
		if score < 0 {
			score = 0
		}
		if len(why) == 0 {
			why = append(why, "matches your filters")
		}
		matches = append(matches, model.Match{Offer: o, Fit: score, Why: why})
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].Fit > matches[j].Fit })
	return matches
}

func trade(o model.Offer) string {
	if strings.TrimSpace(o.Trade) != "" {
		return o.Trade
	}
	return strings.ToLower(o.Category)
}

func hasFold(list []string, want string) bool {
	for _, v := range list {
		if strings.EqualFold(strings.TrimSpace(v), strings.TrimSpace(want)) {
			return true
		}
	}
	return false
}

func containsFold(needle, haystack string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(strings.TrimSpace(needle)))
}

func containsAll(have, want []string) bool {
	for _, w := range want {
		if !hasFold(have, w) {
			return false
		}
	}
	return true
}

// Run is the whole pipeline: parse, query, rank. Exported so the assistant and
// any future surface can search without going through HTTP.
func Run(ctx context.Context, deps app.Deps, raw string, p model.Profile) (model.Intent, []model.Match, error) {
	intent := Parse(ctx, deps, raw, p)
	filter := store.OfferFilter{
		Trades:       intent.Trades,
		Regions:      intent.Regions,
		Statuses:     intent.Statuses,
		Requirements: intent.Documents,
		Limit:        50,
	}
	offers, err := deps.Store.ListOffers(ctx, filter)
	if err != nil {
		return intent, nil, err
	}
	// Widen once rather than showing an empty result: a search that finds
	// nothing because the parse was too eager is a worse answer than a ranked
	// list of near misses.
	if len(offers) == 0 {
		offers, err = deps.Store.ListOffers(ctx, store.OfferFilter{Limit: 50})
		if err != nil {
			return intent, nil, err
		}
	}
	return intent, Rank(offers, intent, p), nil
}

// --- handlers ---------------------------------------------------------------

type resultsView struct {
	Intent   model.Intent
	Matches  []model.Match
	Summary  string
	Query    string
	Degraded bool
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
	intent, matches, err := Run(ctx, h.deps, raw, p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(matches) > 6 {
		matches = matches[:6]
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<div class="mm-msg you"><span class="mm-who">you</span><p>%s</p></div>`, html.EscapeString(raw))
	view := resultsView{
		Intent:   intent,
		Matches:  matches,
		Query:    raw,
		Degraded: intent.Fallback,
		Summary:  Summarize(intent, matches),
	}
	if err := h.tpl.ExecuteTemplate(w, "results", view); err != nil {
		log.Printf("search: render results: %v", err)
	}
}

// Summarize writes the one line above the cards. The baseline is mechanical
// and always truthful; the search worker replaces it with a streamed sentence
// from the model.
func Summarize(intent model.Intent, matches []model.Match) string {
	if len(matches) == 0 {
		return "Nothing matches that yet."
	}
	parts := []string{fmt.Sprintf("%d match%s", len(matches), plural(len(matches)))}
	if len(intent.Trades) > 0 {
		parts = append(parts, "for "+strings.Join(intent.Trades, ", "))
	}
	if len(intent.Regions) > 0 {
		parts = append(parts, "in "+strings.Join(intent.Regions, ", "))
	}
	best := matches[0]
	return strings.Join(parts, " ") + fmt.Sprintf(". Best fit: %s (%d%%) — %s.", best.Offer.Title, best.Fit, best.Why[0])
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "es"
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
	fmt.Fprint(w, `<span class="mm-badge good">saved</span>`)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
