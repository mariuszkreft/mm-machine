package search

import (
	"math"
	"sort"
	"strings"

	"mm-machine/internal/i18n"
	"mm-machine/internal/model"
)

// Scoring weights. Named and centralized so the number on a card always
// traces back to one line here — a fit score nobody can explain is worse
// than no number at all.
//
// scoreTextRelevance is the text index's contribution: textScores holds
// store.TextSearch's per-ID relevance already normalized to 0..1 (see
// textRelevance), so this constant is the maximum a perfect relevance match
// can add — the same "one point, one Why line" rule as every other signal
// here, just scaled by how relevant the row's own text was.
const (
	scoreBaseListed      = 40 // a listed offer already survived the hard filters
	scoreTradeMatch      = 20
	scoreRegionMatch     = 15
	scoreDocsMatch       = 10
	scoreCrewFits        = 10
	scoreCrewShort       = -5
	scoreKeywordMatch    = 3
	scoreAttentionNeeded = 3
	scoreBudgetFits      = 8
	scoreWindowFits      = 8
	scoreProfileRegion   = 5
	scoreProfileTrade    = 5
	scoreProfileCrew     = 5
	scoreProfileDocs     = 8
	scoreTextRelevance   = 15
)

// Rank scores offers against the intent, its facets, the profile and the
// text index's relevance signal, blending all four into one score. Scoring
// is deterministic and every point becomes a reason, in the visitor's own
// language (lang).
//
// textScores is store.TextSearch's relevance over intent.Raw, normalized to
// 0..1 by textRelevance; a nil map (no query text, or nothing relevant)
// simply contributes no relevance points — it never excludes a row, since
// the facet filters already decided which rows exist.
func Rank(offers []model.Offer, intent model.Intent, fc facets, p model.Profile, textScores map[string]float64, lang i18n.Lang) []model.Match {
	matches := make([]model.Match, 0, len(offers))
	for _, o := range offers {
		score := scoreBaseListed
		why := []string{}

		if hasFold(intent.Trades, trade(o)) {
			score += scoreTradeMatch
			why = append(why, tr(lang, "search.why.trade", i18n.T(lang, "trade."+trade(o))))
		}
		for _, region := range intent.Regions {
			if containsFold(region, o.Region) || containsFold(region, o.Location) {
				score += scoreRegionMatch
				why = append(why, tr(lang, "search.why.region", o.Location))
				break
			}
		}
		if len(intent.Documents) > 0 && containsAll(o.Requirements, intent.Documents) {
			score += scoreDocsMatch
			why = append(why, tr(lang, "search.why.docs", localizeDocs(lang, o.Requirements)))
		}
		if intent.CrewSize > 0 && o.CrewSize > 0 {
			switch {
			case o.CrewSize >= intent.CrewSize:
				score += scoreCrewFits
				why = append(why, tr(lang, "search.why.crewFits", o.CrewSize, intent.CrewSize))
			default:
				score += scoreCrewShort
				why = append(why, tr(lang, "search.why.crewShort", o.CrewSize, intent.CrewSize))
			}
		}
		for _, kw := range intent.Keywords {
			if len(kw) > 3 && containsFold(kw, o.Title+" "+o.Category+" "+o.Supplier) {
				score += scoreKeywordMatch
				why = append(why, tr(lang, "search.why.keyword", kw))
				break
			}
		}
		if fc.HasBudgetMin || fc.HasBudgetMax {
			if _, ok := parseOfferBudget(o.Budget); ok {
				score += scoreBudgetFits
				why = append(why, tr(lang, "search.why.budget", o.Budget))
			}
		}
		if fc.HasWindow && !o.Start.IsZero() {
			score += scoreWindowFits
			why = append(why, tr(lang, "search.why.window", i18n.Date(lang, o.Start), i18n.Date(lang, fc.Start), i18n.Date(lang, fc.End)))
		}
		if o.Signal == "Attention" {
			score += scoreAttentionNeeded
			why = append(why, tr(lang, "search.why.attention"))
		}

		// Profile-aware boosts: the visitor's own trade, region, crew size and
		// documents each get their own reason, distinct from the intent match.
		if len(p.Regions) > 0 {
			for _, region := range p.Regions {
				if containsFold(region, o.Region) || containsFold(region, o.Location) {
					score += scoreProfileRegion
					why = append(why, tr(lang, "search.why.profileRegion"))
					break
				}
			}
		}
		if len(p.Trades) > 0 && hasFold(p.Trades, trade(o)) {
			score += scoreProfileTrade
			why = append(why, tr(lang, "search.why.profileTrade"))
		}
		if p.CrewSize > 0 && o.CrewSize > 0 && o.CrewSize >= p.CrewSize {
			score += scoreProfileCrew
			why = append(why, tr(lang, "search.why.profileCrew", p.CrewSize))
		}
		if len(p.Documents) > 0 && len(o.Requirements) > 0 && containsAll(p.Documents, o.Requirements) {
			score += scoreProfileDocs
			why = append(why, tr(lang, "search.why.profileDocs"))
		}
		if rel, ok := textScores[o.ID]; ok {
			if pts := int(math.Round(scoreTextRelevance * rel)); pts > 0 {
				score += pts
				why = append(why, tr(lang, "search.why.textRelevance"))
			}
		}

		if score > 100 {
			score = 100
		}
		if score < 0 {
			score = 0
		}
		if len(why) == 0 {
			why = append(why, tr(lang, "search.why.default"))
		}
		matches = append(matches, model.Match{Kind: "offer", Offer: o, Fit: score, Why: why})
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].Fit > matches[j].Fit })
	return matches
}

// localizeDocs renders a requirement slug list ("a1", "insurance") as the
// visitor's own language's document names, joined for one Why line.
func localizeDocs(lang i18n.Lang, docs []string) string {
	out := make([]string, len(docs))
	for i, d := range docs {
		out[i] = i18n.T(lang, "doc."+d)
	}
	return strings.Join(out, ", ")
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

// applyPostFilter enforces the facets the store's OfferFilter can't express:
// exclusions are always honored; budget and window bounds only drop an
// offer when its own data is known, since an unparseable field is never
// treated as a bad match.
func applyPostFilter(offers []model.Offer, fc facets) []model.Offer {
	out := make([]model.Offer, 0, len(offers))
	for _, o := range offers {
		if hasFold(fc.ExcludeTrades, trade(o)) {
			continue
		}
		if anyContainsFold(fc.ExcludeRegions, o.Region, o.Location) {
			continue
		}
		if fc.HasBudgetMax {
			if amt, ok := parseOfferBudget(o.Budget); ok && amt > fc.BudgetMax {
				continue
			}
		}
		if fc.HasBudgetMin {
			if amt, ok := parseOfferBudget(o.Budget); ok && amt < fc.BudgetMin {
				continue
			}
		}
		if fc.HasWindow && !o.Start.IsZero() {
			if o.Start.Before(fc.Start) || o.Start.After(fc.End) {
				continue
			}
		}
		out = append(out, o)
	}
	return out
}
