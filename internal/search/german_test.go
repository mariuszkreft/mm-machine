package search

import (
	"context"
	"strings"
	"testing"
	"time"

	"mm-machine/internal/app"
	"mm-machine/internal/demo"
	"mm-machine/internal/i18n"
	"mm-machine/internal/model"
	"mm-machine/internal/store"
)

// fixedNow anchors the "now" a date phrase like "ab Oktober"/"from October"
// resolves against, so the test doesn't depend on which year it happens to
// run in.
func fixedNow() time.Time { return time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC) }

// seedDemoStore loads the DACH demo corpus (internal/demo) into a fresh
// Memory store, so ranking tests exercise the same offers/crews a real
// visitor sees rather than an ad-hoc fixture.
func seedDemoStore(t *testing.T) store.Store {
	t.Helper()
	db := store.NewMemory()
	ctx := context.Background()
	for _, o := range demo.Offers() {
		if _, err := db.CreateOffer(ctx, o); err != nil {
			t.Fatalf("seed offer %s: %v", o.ID, err)
		}
	}
	for _, c := range demo.Crews() {
		if _, err := db.UpsertCrew(ctx, c); err != nil {
			t.Fatalf("seed crew %s: %v", c.ID, err)
		}
	}
	return db
}

// --- German parses to the same intent as its English equivalent -----------

func TestParseMechanicalGermanMatchesEnglishTrade(t *testing.T) {
	en := ParseMechanical("electricians in Munich")
	de := ParseMechanical("Elektriker in München")
	if len(en.Trades) != 1 || en.Trades[0] != "electrical" {
		t.Fatalf("English trades = %v, want [electrical]", en.Trades)
	}
	if len(de.Trades) != 1 || de.Trades[0] != "electrical" {
		t.Fatalf("German trades = %v, want [electrical]", de.Trades)
	}
	if len(en.Regions) != 1 || len(de.Regions) != 1 || en.Regions[0] != de.Regions[0] {
		t.Fatalf("regions = %v (en) vs %v (de), want the same canonical city", en.Regions, de.Regions)
	}
}

func TestParseMechanicalGermanCrewSizeDocumentsAndDate(t *testing.T) {
	de := ParseMechanical("6 Monteure in München ab Oktober, A1 vorhanden")
	en := ParseMechanical("6 fitters in Munich from October, A1 available")
	if de.CrewSize != 6 {
		t.Errorf("German CrewSize = %d, want 6", de.CrewSize)
	}
	if en.CrewSize != 6 {
		t.Errorf("English CrewSize = %d, want 6", en.CrewSize)
	}
	if len(de.Documents) != 1 || de.Documents[0] != "a1" {
		t.Errorf("German Documents = %v, want [a1]", de.Documents)
	}
	if len(en.Documents) != 1 || en.Documents[0] != "a1" {
		t.Errorf("English Documents = %v, want [a1]", en.Documents)
	}
	deFC := deriveFacets("6 Monteure in München ab Oktober, A1 vorhanden", fixedNow())
	enFC := deriveFacets("6 fitters in Munich from October, A1 available", fixedNow())
	if !deFC.HasWindow || !enFC.HasWindow {
		t.Fatal("expected both sentences to parse a date window")
	}
	if deFC.Start.Month() != 10 || enFC.Start.Month() != 10 {
		t.Errorf("window start month = %v (de) vs %v (en), want October for both", deFC.Start.Month(), enFC.Start.Month())
	}
}

func TestParseMechanicalGermanCrewSeeking(t *testing.T) {
	intent := ParseMechanical("Kolonne für Trockenbau mit Brandschutz")
	if intent.Kind != "find_crews" {
		t.Errorf("Kind = %q, want find_crews (Kolonne signals the supply side)", intent.Kind)
	}
	if len(intent.Trades) != 1 || intent.Trades[0] != "drywall" {
		t.Errorf("Trades = %v, want [drywall] (Trockenbau)", intent.Trades)
	}
}

func TestParseMechanicalGermanCountryRegion(t *testing.T) {
	intent := ParseMechanical("wer sucht Stahlbauer in den Niederlanden")
	if len(intent.Trades) != 1 || intent.Trades[0] != "steel" {
		t.Errorf("Trades = %v, want [steel] (Stahlbauer)", intent.Trades)
	}
	if len(intent.Regions) != 1 || intent.Regions[0] != "NL" {
		t.Errorf("Regions = %v, want [NL] (Niederlande -> country code, matching the corpus's \"City, NL\" format)", intent.Regions)
	}
	if intent.Kind != "find_offers" {
		t.Errorf("Kind = %q, want find_offers (\"wer sucht\" asks about postings, not supply)", intent.Kind)
	}
}

func TestParseNegationsGerman(t *testing.T) {
	trades, regions := parseNegations("Stahlbau, nicht Wien")
	if len(regions) != 1 || regions[0] != "Wien" {
		t.Errorf("excludeRegions = %v, want [Wien]", regions)
	}
	if len(trades) != 0 {
		t.Errorf("excludeTrades = %v, want none", trades)
	}

	trades, regions = parseNegations("jedes Gewerk außer Elektro")
	if len(trades) != 1 || trades[0] != "electrical" {
		t.Errorf("excludeTrades = %v, want [electrical]", trades)
	}
	if len(regions) != 0 {
		t.Errorf("excludeRegions = %v, want none", regions)
	}
}

// --- ranking order on the demo corpus, locked in both languages -----------

func TestRankOrderLockedGermanCorpus(t *testing.T) {
	db := seedDemoStore(t)
	deps := app.Deps{Store: db} // no LLM: forces the mechanical path

	for _, raw := range []string{"electricians in Munich", "Elektriker in München"} {
		t.Run(raw, func(t *testing.T) {
			intent, fc := Parse(context.Background(), deps, raw, model.Profile{})
			result, err := Search(context.Background(), deps, intent, fc, model.Profile{}, i18n.DE)
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if len(result.Matches) == 0 {
				t.Fatal("expected at least one match")
			}
			if result.Matches[0].Offer.ID != "MM-2105" {
				t.Errorf("top match = %s (%s), want MM-2105 (the only electrical offer in Munich)",
					result.Matches[0].Offer.ID, result.Matches[0].Offer.Title)
			}
		})
	}
}

// --- every match carries a reason in the requested language ---------------

func TestEveryMatchHasReasonInRequestedLanguage(t *testing.T) {
	db := seedDemoStore(t)
	deps := app.Deps{Store: db}
	intent, fc := Parse(context.Background(), deps, "Elektriker in München", model.Profile{})

	deResult, err := Search(context.Background(), deps, intent, fc, model.Profile{}, i18n.DE)
	if err != nil {
		t.Fatalf("Search (de): %v", err)
	}
	enResult, err := Search(context.Background(), deps, intent, fc, model.Profile{}, i18n.EN)
	if err != nil {
		t.Fatalf("Search (en): %v", err)
	}
	if len(deResult.Matches) == 0 || len(enResult.Matches) == 0 {
		t.Fatal("expected matches on both languages")
	}
	for _, m := range deResult.Matches {
		if len(m.Why) == 0 || m.Why[0] == "" {
			t.Errorf("offer %s has no German reason", m.Offer.ID)
		}
	}
	deTop, enTop := deResult.Matches[0], enResult.Matches[0]
	if deTop.Why[0] == enTop.Why[0] {
		t.Errorf("expected the top match's reason to differ by language, got the same for both: %q", deTop.Why[0])
	}
}

// --- mixed offer/crew results ----------------------------------------------

func TestMixedOfferAndCrewResults(t *testing.T) {
	db := seedDemoStore(t)
	deps := app.Deps{Store: db}
	// An ambiguous ask ("Elektro Kolonne" mentions both sides) with no
	// profile role should answer with both offers and crews, kind visible
	// on every card.
	intent, fc := Parse(context.Background(), deps, "Elektro", model.Profile{})
	result, err := runSides(context.Background(), deps, intent, fc, model.Profile{}, i18n.DE)
	if err != nil {
		t.Fatalf("runSides: %v", err)
	}
	var haveOffer, haveCrew bool
	for _, m := range result.Matches {
		switch m.Kind {
		case "offer":
			haveOffer = true
		case "crew":
			haveCrew = true
		default:
			t.Errorf("match with unexpected Kind %q", m.Kind)
		}
	}
	if !haveOffer || !haveCrew {
		t.Errorf("expected a mixed list, got haveOffer=%v haveCrew=%v", haveOffer, haveCrew)
	}
	// Fit-sorted across both sides, not offers-then-crews or vice versa.
	for i := 1; i < len(result.Matches); i++ {
		if result.Matches[i-1].Fit < result.Matches[i].Fit {
			t.Fatalf("matches not sorted by Fit descending at index %d: %d < %d", i, result.Matches[i-1].Fit, result.Matches[i].Fit)
		}
	}
}

// --- hostile input never reaches the rendered HTML unescaped ---------------

func TestResultsTemplateEscapesWhyAndTitle(t *testing.T) {
	h := &Handler{tpl: parseTemplates()}
	view := resultsView{
		Query: `<img src=x onerror=alert(1)>`,
		Matches: []model.Match{
			{Kind: "offer", Offer: model.Offer{ID: "MM-1", Title: `<script>alert('t')</script>`, Status: "open"},
				Fit: 50, Why: []string{`<script>alert('why')</script>`}},
		},
		Fallback: `<script>alert('fallback')</script>`,
		Lang:     i18n.EN,
	}
	var buf strings.Builder
	if err := h.tpl.ExecuteTemplate(&buf, "results", view); err != nil {
		t.Fatalf("ExecuteTemplate: %v", err)
	}
	out := buf.String()
	// The page's own static wiring <script> block is expected; only the
	// payloads carried in Why/Title/Query/Fallback must never appear raw.
	for _, hostile := range []string{"<script>alert", "<img src=x onerror="} {
		if strings.Contains(out, hostile) {
			t.Errorf("unescaped hostile content leaked into rendered HTML: %q found in output", hostile)
		}
	}
	if !strings.Contains(out, "&lt;script&gt;alert") {
		t.Error("expected the hostile Why/title text to appear HTML-escaped")
	}
}
