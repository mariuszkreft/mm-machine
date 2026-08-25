package search

import (
	"testing"

	"mm-machine/internal/i18n"
	"mm-machine/internal/model"
)

func fixedCrews() []model.Crew {
	return []model.Crew{
		{ID: "CR-1", Name: "Perfect fit", Trades: []string{"energy"}, Regions: []string{"München, DE"}, Size: 8, Rating: 4.9, JobsDone: 20},
		{ID: "CR-2", Name: "Right trade, wrong region", Trades: []string{"energy"}, Regions: []string{"Wien, AT"}, Size: 8},
		{ID: "CR-3", Name: "Right region, wrong trade", Trades: []string{"steel"}, Regions: []string{"München, DE"}, Size: 8},
	}
}

func TestRankCrewsOrderLocked(t *testing.T) {
	intent := model.Intent{Trades: []string{"energy"}, Regions: []string{"München"}}
	matches := RankCrews(fixedCrews(), intent, model.Profile{}, nil, i18n.EN)
	if len(matches) != 3 {
		t.Fatalf("len(matches) = %d, want 3", len(matches))
	}
	if matches[0].Crew.ID != "CR-1" {
		t.Errorf("top match = %s, want CR-1", matches[0].Crew.ID)
	}
	for _, m := range matches {
		if m.Kind != "crew" {
			t.Errorf("match %s has Kind=%q, want crew", m.Crew.ID, m.Kind)
		}
		if len(m.Why) == 0 {
			t.Errorf("crew %s has no reason", m.Crew.ID)
		}
	}
}

func TestRankCrewsWhyIsLocalized(t *testing.T) {
	intent := model.Intent{Trades: []string{"energy"}, Regions: []string{"München"}}
	de := RankCrews(fixedCrews(), intent, model.Profile{}, nil, i18n.DE)[0]
	en := RankCrews(fixedCrews(), intent, model.Profile{}, nil, i18n.EN)[0]
	if de.Why[0] == en.Why[0] {
		t.Errorf("expected different-language Why lines, got the same for both: %q", de.Why[0])
	}
}

func TestWantsOffersAndCrewsNeverBothFalse(t *testing.T) {
	cases := []struct {
		name   string
		intent model.Intent
		p      model.Profile
	}{
		{"explicit find_offers", model.Intent{Kind: "find_offers"}, model.Profile{}},
		{"explicit find_crews", model.Intent{Kind: "find_crews"}, model.Profile{}},
		{"ambiguous kind, owner", model.Intent{}, model.Profile{Role: "owner"}},
		{"ambiguous kind, executor", model.Intent{}, model.Profile{Role: "executor"}},
		{"ambiguous kind, unknown role", model.Intent{}, model.Profile{}},
	}
	for _, c := range cases {
		offers := WantsOffers(c.intent, c.p)
		crews := WantsCrews(c.intent, c.p)
		if !offers && !crews {
			t.Errorf("%s: both WantsOffers and WantsCrews are false", c.name)
		}
	}
}

func TestWantsBothForAmbiguousUnknownProfile(t *testing.T) {
	if !WantsOffers(model.Intent{}, model.Profile{}) || !WantsCrews(model.Intent{}, model.Profile{}) {
		t.Error("an ambiguous intent with an unknown profile should want both sides mixed")
	}
}

func TestWantsCrewsOnlyForOwnerWithAmbiguousKind(t *testing.T) {
	p := model.Profile{Role: "owner"}
	if WantsOffers(model.Intent{}, p) {
		t.Error("an owner with an ambiguous intent should not also see the offers-only side")
	}
	if !WantsCrews(model.Intent{}, p) {
		t.Error("an owner with an ambiguous intent should see crews")
	}
}

func TestExplicitKindOverridesRole(t *testing.T) {
	// A find_offers intent settles it even for an owner profile: the
	// sentence itself said which side it wants.
	p := model.Profile{Role: "owner"}
	if !WantsOffers(model.Intent{Kind: "find_offers"}, p) {
		t.Error("explicit find_offers should win over the owner-role default")
	}
	if WantsCrews(model.Intent{Kind: "find_offers"}, p) {
		t.Error("explicit find_offers should exclude crews")
	}
}
