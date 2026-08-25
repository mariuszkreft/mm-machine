package store

import (
	"strings"

	"mm-machine/internal/model"
)

// MatchesFacets reports whether an offer satisfies the facet fields of a
// filter. Facets are OR within a field and AND across fields; an empty facet
// field matches everything.
//
// It exists so every backend answers a faceted query identically: the SQLite
// store may push some of this into SQL, but the semantics live here.
func MatchesFacets(o model.Offer, f OfferFilter) bool {
	if !anyEqualFold(f.Statuses, o.Status) {
		return false
	}
	if !anyEqualFold(f.Trades, offerTrade(o)) {
		return false
	}
	if len(f.Regions) > 0 && !anyContainsFold(f.Regions, offerRegion(o)) {
		return false
	}
	if len(f.Requirements) > 0 && !containsAllFold(o.Requirements, f.Requirements) {
		return false
	}
	if f.MinCrewSize > 0 && o.CrewSize > 0 && o.CrewSize < f.MinCrewSize {
		return false
	}
	return true
}

// offerTrade falls back to the human category when the normalized trade is
// missing, so pre-facet rows still answer trade queries.
func offerTrade(o model.Offer) string {
	if strings.TrimSpace(o.Trade) != "" {
		return o.Trade
	}
	return o.Category
}

func offerRegion(o model.Offer) string {
	if strings.TrimSpace(o.Region) != "" {
		return o.Region
	}
	return o.Location
}

func anyEqualFold(want []string, got string) bool {
	if len(want) == 0 {
		return true
	}
	for _, w := range want {
		if strings.EqualFold(strings.TrimSpace(w), strings.TrimSpace(got)) {
			return true
		}
	}
	return false
}

// anyContainsFold matches loosely in both directions: "Munich" matches
// "Munich, DE", and "DACH" is matched by a caller that expanded it upstream.
func anyContainsFold(want []string, got string) bool {
	g := strings.ToLower(got)
	for _, w := range want {
		w = strings.ToLower(strings.TrimSpace(w))
		if w == "" {
			continue
		}
		if strings.Contains(g, w) || strings.Contains(w, g) {
			return true
		}
	}
	return false
}

func containsAllFold(have, want []string) bool {
	for _, w := range want {
		found := false
		for _, h := range have {
			if strings.EqualFold(strings.TrimSpace(h), strings.TrimSpace(w)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Completeness scores a profile 0..100 over model.ProfileFields, so onboarding
// and every progress meter agree on what "done" means.
func Completeness(p model.Profile) int {
	filled := 0
	for _, field := range model.ProfileFields {
		switch field {
		case "role":
			if p.Role != "" && p.Role != "unknown" {
				filled++
			}
		case "trades":
			if len(p.Trades) > 0 {
				filled++
			}
		case "regions":
			if len(p.Regions) > 0 {
				filled++
			}
		case "crewSize":
			if p.CrewSize > 0 {
				filled++
			}
		case "documents":
			if len(p.Documents) > 0 {
				filled++
			}
		case "availability":
			if strings.TrimSpace(p.Availability) != "" {
				filled++
			}
		}
	}
	return filled * 100 / len(model.ProfileFields)
}
