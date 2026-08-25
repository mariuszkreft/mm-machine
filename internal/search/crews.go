package search

import (
	"context"
	"math"
	"sort"
	"strings"

	"mm-machine/internal/app"
	"mm-machine/internal/i18n"
	"mm-machine/internal/model"
	"mm-machine/internal/store"
)

// RankCrews scores the supply side against an intent, mirroring Rank for
// offers: the same facets/profile/text-relevance blend, the same "every
// point earns a Why line" rule, in the visitor's own language.
// A Generalunternehmer asking "who can field six electricians in Munich in
// October" is answered from here.
func RankCrews(crews []model.Crew, intent model.Intent, p model.Profile, textScores map[string]float64, lang i18n.Lang) []model.Match {
	matches := make([]model.Match, 0, len(crews))
	for _, c := range crews {
		score := 40
		why := []string{}

		for _, trade := range intent.Trades {
			if containsFold(trade, strings.Join(c.Trades, " ")) {
				score += 20
				why = append(why, tr(lang, "search.why.trade", i18n.T(lang, "trade."+trade)))
				break
			}
		}
		for _, region := range intent.Regions {
			if containsFold(region, strings.Join(c.Regions, " ")) {
				score += 15
				why = append(why, tr(lang, "search.why.crewRegion", region))
				break
			}
		}
		if intent.CrewSize > 0 {
			if c.Size >= intent.CrewSize {
				score += 12
				why = append(why, tr(lang, "search.why.crewFits", c.Size, intent.CrewSize))
			} else {
				score -= 8
				why = append(why, tr(lang, "search.why.crewShort", c.Size, intent.CrewSize))
			}
		}
		if len(intent.Documents) > 0 && containsAll(c.Documents, intent.Documents) {
			score += 10
			why = append(why, tr(lang, "search.why.docs", localizeDocs(lang, c.Documents)))
		}
		if c.Rating >= 4.5 {
			score += 5
			why = append(why, tr(lang, "search.why.crewRating", c.Rating, c.JobsDone))
		}
		if len(p.Regions) > 0 && containsFold(p.Regions[0], strings.Join(c.Regions, " ")) {
			score += 5
			why = append(why, tr(lang, "search.why.profileRegion"))
		}
		if rel, ok := textScores[c.ID]; ok {
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
		matches = append(matches, model.Match{Kind: "crew", Crew: c, Fit: score, Why: why})
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].Fit > matches[j].Fit })
	return matches
}

// WantsCrews and WantsOffers decide which side(s) of the market answer an
// intent — never both false, so ambiguity always shows something:
//   - an explicit "find_crews"/"find_offers" intent kind settles it outright,
//   - a known role (owner looks for crews, executor looks for offers) settles
//     it when the intent kind didn't,
//   - anything left ambiguous (unknown role, ambiguous kind) answers with
//     both sides mixed into one list rather than guessing which one the
//     visitor meant.
func WantsCrews(intent model.Intent, p model.Profile) bool {
	switch intent.Kind {
	case "find_crews":
		return true
	case "find_offers":
		return false
	}
	return p.Role == "" || p.Role == "unknown" || p.Role == "owner"
}

func WantsOffers(intent model.Intent, p model.Profile) bool {
	switch intent.Kind {
	case "find_offers":
		return true
	case "find_crews":
		return false
	}
	return p.Role == "" || p.Role == "unknown" || p.Role == "executor"
}

// RunCrews is the crew-side pipeline: the parsed intent becomes a crew filter,
// and the ranking explains itself the same way the offer side does,
// including the text index's relevance signal (see textRelevance).
func RunCrews(ctx context.Context, deps app.Deps, intent model.Intent, p model.Profile, lang i18n.Lang) ([]model.Match, error) {
	crews, err := deps.Store.ListCrews(ctx, store.CrewFilter{
		Trades:    intent.Trades,
		Regions:   intent.Regions,
		Documents: intent.Documents,
		MinSize:   intent.CrewSize,
		Limit:     50,
	})
	if err != nil {
		return nil, err
	}
	if len(crews) == 0 {
		// Widen rather than answer with nothing: a near miss is more useful
		// than an empty page, and the summary says what was dropped.
		crews, err = deps.Store.ListCrews(ctx, store.CrewFilter{Trades: intent.Trades, Limit: 50})
		if err != nil {
			return nil, err
		}
	}
	if len(crews) == 0 {
		crews, err = deps.Store.ListCrews(ctx, store.CrewFilter{Limit: 50})
		if err != nil {
			return nil, err
		}
	}
	textScores := textRelevance(ctx, deps, intent.Raw, "crew")
	return RankCrews(crews, intent, p, textScores, lang), nil
}
