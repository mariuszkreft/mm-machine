package search

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"mm-machine/internal/app"
	"mm-machine/internal/model"
	"mm-machine/internal/store"
)

// RankCrews scores the supply side against an intent, mirroring RankOffers.
// A Generalunternehmer asking "who can field six electricians in Munich in
// October" is answered from here.
//
// Baseline weights; the search worker owns the tuning, but the rule holds:
// every point added must also add a line to Why.
func RankCrews(crews []model.Crew, intent model.Intent, p model.Profile) []model.Match {
	matches := make([]model.Match, 0, len(crews))
	for _, c := range crews {
		score := 40
		why := []string{}

		for _, trade := range intent.Trades {
			if containsFold(trade, strings.Join(c.Trades, " ")) {
				score += 20
				why = append(why, "Gewerk passt: "+trade)
				break
			}
		}
		for _, region := range intent.Regions {
			if containsFold(region, strings.Join(c.Regions, " ")) {
				score += 15
				why = append(why, "arbeitet in "+region)
				break
			}
		}
		if intent.CrewSize > 0 {
			if c.Size >= intent.CrewSize {
				score += 12
				why = append(why, fmt.Sprintf("%d Leute decken die gesuchten %d", c.Size, intent.CrewSize))
			} else {
				score -= 8
				why = append(why, fmt.Sprintf("nur %d Leute statt %d", c.Size, intent.CrewSize))
			}
		}
		if len(intent.Documents) > 0 && containsAll(c.Documents, intent.Documents) {
			score += 10
			why = append(why, "Papiere liegen vor: "+strings.Join(c.Documents, ", "))
		}
		if c.Rating >= 4.5 {
			score += 5
			why = append(why, fmt.Sprintf("Bewertung %.1f aus %d Einsätzen", c.Rating, c.JobsDone))
		}
		if len(p.Regions) > 0 && containsFold(p.Regions[0], strings.Join(c.Regions, " ")) {
			score += 5
			why = append(why, "in Ihrer Region")
		}

		if score > 100 {
			score = 100
		}
		if score < 0 {
			score = 0
		}
		if len(why) == 0 {
			why = append(why, "erfüllt Ihre Filter")
		}
		matches = append(matches, model.Match{Kind: "crew", Crew: c, Fit: score, Why: why})
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].Fit > matches[j].Fit })
	return matches
}

// WantsCrews reports whether an intent is about the supply side. A
// Generalunternehmer asking an open question means crews; a Nachunternehmer
// means offers.
func WantsCrews(intent model.Intent, p model.Profile) bool {
	if intent.Kind == "find_crews" {
		return true
	}
	if intent.Kind == "find_offers" {
		return false
	}
	return p.Role == "owner"
}

// RunCrews is the crew-side pipeline: the parsed intent becomes a crew filter,
// and the ranking explains itself the same way the offer side does.
func RunCrews(ctx context.Context, deps app.Deps, intent model.Intent, p model.Profile) ([]model.Match, error) {
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
	return RankCrews(crews, intent, p), nil
}
