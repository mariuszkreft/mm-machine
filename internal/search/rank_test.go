package search

import (
	"testing"
	"time"

	"mm-machine/internal/i18n"
	"mm-machine/internal/model"
)

// fixedCorpus is a small, deliberately differentiated set of offers used to
// lock the ranking order. Any change to the scoring weights that reorders
// this list should be a conscious decision, not an accident.
func fixedCorpus() []model.Offer {
	return []model.Offer{
		{ID: "MM-1", Title: "Perfect fit", Location: "Munich, DE", Category: "Energy", Budget: "EUR 90k",
			Status: "open", Signal: "Attention", Trade: "energy", Region: "Munich, DE", CrewSize: 8,
			Requirements: []string{"a1"}},
		{ID: "MM-2", Title: "Right trade, wrong region", Location: "Vienna, AT", Category: "Energy", Budget: "EUR 90k",
			Status: "open", Trade: "energy", Region: "Vienna, AT", CrewSize: 8},
		{ID: "MM-3", Title: "Right region, wrong trade", Location: "Munich, DE", Category: "Steel", Budget: "EUR 90k",
			Status: "open", Trade: "steel", Region: "Munich, DE", CrewSize: 8},
		{ID: "MM-4", Title: "Right trade, wrong region, short crew", Location: "Rotterdam, NL", Category: "Energy", Budget: "EUR 90k",
			Status: "open", Trade: "energy", Region: "Rotterdam, NL", CrewSize: 2},
	}
}

func fixedIntent() model.Intent {
	return model.Intent{Trades: []string{"energy"}, Regions: []string{"Munich"}, CrewSize: 6}
}

func TestRankOrderLocked(t *testing.T) {
	matches := Rank(fixedCorpus(), fixedIntent(), facets{}, model.Profile{}, nil, i18n.EN)
	got := make([]string, len(matches))
	for i, m := range matches {
		got[i] = m.Offer.ID
	}
	want := []string{"MM-1", "MM-2", "MM-3", "MM-4"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rank order = %v, want %v", got, want)
		}
	}
}

func TestRankEveryMatchHasAReason(t *testing.T) {
	matches := Rank(fixedCorpus(), fixedIntent(), facets{}, model.Profile{}, nil, i18n.EN)
	for _, m := range matches {
		if len(m.Why) == 0 {
			t.Errorf("offer %s has no reason", m.Offer.ID)
		}
		for _, why := range m.Why {
			if why == "" {
				t.Errorf("offer %s has an empty reason", m.Offer.ID)
			}
		}
	}
}

func TestRankBudgetAndWindowBoosts(t *testing.T) {
	offers := []model.Offer{
		{ID: "MM-A", Title: "In budget, in window", Budget: "EUR 40k", Start: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)},
		{ID: "MM-B", Title: "No budget data", Budget: "negotiable"},
	}
	fc := facets{
		HasBudgetMax: true, BudgetMax: 100_000,
		HasWindow: true,
		Start:     time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		End:       time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC),
	}
	matches := Rank(offers, model.Intent{}, fc, model.Profile{}, nil, i18n.EN)
	byID := map[string]model.Match{}
	for _, m := range matches {
		byID[m.Offer.ID] = m
	}
	if byID["MM-A"].Fit <= byID["MM-B"].Fit {
		t.Errorf("offer with known budget/window should score higher: MM-A=%d MM-B=%d", byID["MM-A"].Fit, byID["MM-B"].Fit)
	}
}

func TestRankProfileBoosts(t *testing.T) {
	offer := model.Offer{ID: "MM-1", Trade: "steel", Region: "Rotterdam, NL", CrewSize: 10, Requirements: []string{"a1", "insurance"}}
	base := Rank([]model.Offer{offer}, model.Intent{}, facets{}, model.Profile{}, nil, i18n.EN)[0].Fit

	p := model.Profile{Trades: []string{"steel"}, Regions: []string{"Rotterdam"}, CrewSize: 4, Documents: []string{"a1", "insurance"}}
	boosted := Rank([]model.Offer{offer}, model.Intent{}, facets{}, p, nil, i18n.EN)[0].Fit

	if boosted <= base {
		t.Errorf("profile-matching offer should score higher: base=%d boosted=%d", base, boosted)
	}
}
