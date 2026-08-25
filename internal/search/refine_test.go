package search

import (
	"testing"

	"mm-machine/internal/model"
)

func TestBuildChipsReflectsCorpus(t *testing.T) {
	matches := []model.Match{
		{Offer: model.Offer{ID: "MM-1", Trade: "energy", Region: "Munich, DE", Status: "open"}},
		{Offer: model.Offer{ID: "MM-2", Trade: "steel", Region: "Rotterdam, NL", Status: "requested"}},
		{Offer: model.Offer{ID: "MM-3", Trade: "energy", Region: "Munich, DE", Status: "open"}}, // duplicate facets
	}
	intent := model.Intent{Trades: []string{"energy"}, Regions: []string{"Munich"}}

	chips := buildChips(matches, intent)

	var haveSteel, haveRotterdam, haveRequested, haveEnergyChip, haveMunichChip bool
	for _, c := range chips {
		switch c.Refine {
		case "set:trade:steel":
			haveSteel = true
		case "set:region:Rotterdam, NL":
			haveRotterdam = true
		case "set:status:requested":
			haveRequested = true
		case "set:trade:energy":
			haveEnergyChip = true
		case "set:region:Munich, DE":
			haveMunichChip = true
		}
	}
	if !haveSteel {
		t.Error("expected a chip for the other trade present in results (steel)")
	}
	if !haveRotterdam {
		t.Error("expected a chip for the other region present in results (Rotterdam)")
	}
	if !haveRequested {
		t.Error("expected a chip for the other status present in results (requested)")
	}
	if haveEnergyChip {
		t.Error("did not expect a chip for a trade already in the intent (energy)")
	}
	if haveMunichChip {
		t.Error("did not expect a chip for a region already in the intent (Munich)")
	}

	// Duplicate facets across matches must not produce duplicate chips.
	seen := map[string]int{}
	for _, c := range chips {
		seen[c.Refine]++
	}
	for refine, n := range seen {
		if n > 1 {
			t.Errorf("chip %q appeared %d times, want at most once", refine, n)
		}
	}
}

func TestParseRefineAndApply(t *testing.T) {
	rf, ok := parseRefine("set:trade:steel")
	if !ok {
		t.Fatal("expected parseRefine to succeed")
	}
	intent := applyRefine(model.Intent{Trades: []string{"energy"}}, rf)
	if len(intent.Trades) != 1 || intent.Trades[0] != "steel" {
		t.Errorf("intent.Trades = %v, want [steel]", intent.Trades)
	}

	rf, ok = parseRefine("drop:region:")
	if !ok {
		t.Fatal("expected parseRefine to succeed")
	}
	intent = applyRefine(model.Intent{Regions: []string{"Vienna"}}, rf)
	if len(intent.Regions) != 0 {
		t.Errorf("intent.Regions = %v, want none", intent.Regions)
	}

	if _, ok := parseRefine(""); ok {
		t.Error("expected empty refine to be rejected")
	}
	if _, ok := parseRefine("bogus"); ok {
		t.Error("expected malformed refine to be rejected")
	}
}
