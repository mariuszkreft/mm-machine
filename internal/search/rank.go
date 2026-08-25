package search

import (
	"fmt"
	"sort"
	"strings"

	"mm-machine/internal/model"
)

// Scoring weights. Named and centralized so the number on a card always
// traces back to one line here — a fit score nobody can explain is worse
// than no number at all.
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
)

// Rank scores offers against the intent, its facets and the profile.
// Scoring is deterministic and every point becomes a reason.
func Rank(offers []model.Offer, intent model.Intent, fc facets, p model.Profile) []model.Match {
	matches := make([]model.Match, 0, len(offers))
	for _, o := range offers {
		score := scoreBaseListed
		why := []string{}

		if hasFold(intent.Trades, trade(o)) {
			score += scoreTradeMatch
			why = append(why, "trade matches "+trade(o))
		}
		for _, region := range intent.Regions {
			if containsFold(region, o.Region) || containsFold(region, o.Location) {
				score += scoreRegionMatch
				why = append(why, "in "+o.Location)
				break
			}
		}
		if len(intent.Documents) > 0 && containsAll(o.Requirements, intent.Documents) {
			score += scoreDocsMatch
			why = append(why, "papers required: "+strings.Join(o.Requirements, ", "))
		}
		if intent.CrewSize > 0 && o.CrewSize > 0 {
			switch {
			case o.CrewSize >= intent.CrewSize:
				score += scoreCrewFits
				why = append(why, fmt.Sprintf("crew of %d covers the %d asked for", o.CrewSize, intent.CrewSize))
			default:
				score += scoreCrewShort
				why = append(why, fmt.Sprintf("crew of %d is short of %d", o.CrewSize, intent.CrewSize))
			}
		}
		for _, kw := range intent.Keywords {
			if len(kw) > 3 && containsFold(kw, o.Title+" "+o.Category+" "+o.Supplier) {
				score += scoreKeywordMatch
				why = append(why, "mentions "+kw)
				break
			}
		}
		if fc.HasBudgetMin || fc.HasBudgetMax {
			if _, ok := parseOfferBudget(o.Budget); ok {
				score += scoreBudgetFits
				why = append(why, o.Budget+" fits the budget you asked for")
			}
		}
		if fc.HasWindow && !o.Start.IsZero() {
			score += scoreWindowFits
			why = append(why, "starts "+o.Start.Format("Jan 2")+", inside "+fc.Start.Format("Jan 2")+"–"+fc.End.Format("Jan 2"))
		}
		if o.Signal == "Attention" {
			score += scoreAttentionNeeded
			why = append(why, "needs attention now")
		}

		// Profile-aware boosts: the visitor's own trade, region, crew size and
		// documents each get their own reason, distinct from the intent match.
		if len(p.Regions) > 0 {
			for _, region := range p.Regions {
				if containsFold(region, o.Region) || containsFold(region, o.Location) {
					score += scoreProfileRegion
					why = append(why, "near your region")
					break
				}
			}
		}
		if len(p.Trades) > 0 && hasFold(p.Trades, trade(o)) {
			score += scoreProfileTrade
			why = append(why, "matches a trade on your profile")
		}
		if p.CrewSize > 0 && o.CrewSize > 0 && o.CrewSize >= p.CrewSize {
			score += scoreProfileCrew
			why = append(why, fmt.Sprintf("covers your usual crew of %d", p.CrewSize))
		}
		if len(p.Documents) > 0 && len(o.Requirements) > 0 && containsAll(p.Documents, o.Requirements) {
			score += scoreProfileDocs
			why = append(why, "you already hold the papers it needs")
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
		matches = append(matches, model.Match{Kind: "offer", Offer: o, Fit: score, Why: why})
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
