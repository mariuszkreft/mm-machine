package demo

import (
	"testing"

	"mm-machine/internal/store"
)

// knownTrades mirrors the seven trade slugs internal/search and
// internal/onboarding validate against. It is duplicated locally, same as
// those packages duplicate it from each other, rather than importing an
// unexported symbol across a package boundary.
var knownTrades = map[string]bool{
	"electrical": true, "sanitary": true, "steel": true, "interior": true,
	"energy": true, "drywall": true, "hvac": true,
}

func TestOffersHaveKnownTradeAndMatchableRegion(t *testing.T) {
	for _, o := range Offers() {
		if !knownTrades[o.Trade] {
			t.Errorf("offer %s: trade %q is not in the known set", o.ID, o.Trade)
		}
		if o.Region == "" {
			t.Errorf("offer %s: empty region", o.ID)
		}
		if !store.MatchesFacets(o, store.OfferFilter{Regions: []string{o.Region}}) {
			t.Errorf("offer %s: region %q does not self-match the facet filter", o.ID, o.Region)
		}
		if !store.MatchesFacets(o, store.OfferFilter{Trades: []string{o.Trade}}) {
			t.Errorf("offer %s: trade %q does not self-match the facet filter", o.ID, o.Trade)
		}
	}
}

func TestCrewsHaveKnownTradesAndRegions(t *testing.T) {
	for _, c := range Crews() {
		if len(c.Trades) == 0 {
			t.Errorf("crew %s: no trades", c.ID)
		}
		for _, tr := range c.Trades {
			if !knownTrades[tr] {
				t.Errorf("crew %s: trade %q is not in the known set", c.ID, tr)
			}
		}
		if len(c.Regions) == 0 {
			t.Errorf("crew %s: no regions", c.ID)
		}
	}
}

func TestAllSevenTradesCovered(t *testing.T) {
	seen := map[string]bool{}
	for _, o := range Offers() {
		seen[o.Trade] = true
	}
	for _, c := range Crews() {
		for _, tr := range c.Trades {
			seen[tr] = true
		}
	}
	for trade := range knownTrades {
		if !seen[trade] {
			t.Errorf("trade %q has no offer or crew in the demo market", trade)
		}
	}
}

// TestAwkwardCasesPresent checks for the market behaviors the task asked for
// explicitly: an offer no seeded crew is big enough to staff, and a crew that
// speaks only Polish.
func TestAwkwardCasesPresent(t *testing.T) {
	maxCrewSize := 0
	for _, c := range Crews() {
		if c.Size > maxCrewSize {
			maxCrewSize = c.Size
		}
	}
	hasUnstaffable := false
	for _, o := range Offers() {
		if o.CrewSize > maxCrewSize {
			hasUnstaffable = true
		}
	}
	if !hasUnstaffable {
		t.Error("no offer needs more people than the largest seeded crew provides")
	}

	hasPolishOnly := false
	for _, c := range Crews() {
		if len(c.Languages) == 1 && c.Languages[0] == "pl" {
			hasPolishOnly = true
		}
	}
	if !hasPolishOnly {
		t.Error("no crew speaks only Polish")
	}
}
