package search

import (
	"fmt"
	"strings"

	"mm-machine/internal/i18n"
)

// localCatalog holds strings this package needs that are not yet in the
// shared catalog (internal/i18n is off limits for this slice — see
// TASK-m2herd-search.md). Reported in the handoff so these keys can be
// folded into internal/i18n/catalog.go under the same names.
var localCatalog = map[string]map[i18n.Lang]string{
	// Why-line fragments for a ranked offer.
	"search.why.trade":         {i18n.EN: "trade matches %s", i18n.DE: "Gewerk passt: %s"},
	"search.why.region":        {i18n.EN: "in %s", i18n.DE: "in %s"},
	"search.why.docs":          {i18n.EN: "papers required: %s", i18n.DE: "Papiere erforderlich: %s"},
	"search.why.crewFits":      {i18n.EN: "crew of %d covers the %d asked for", i18n.DE: "%d Personen decken die gesuchten %d"},
	"search.why.crewShort":     {i18n.EN: "crew of %d is short of %d", i18n.DE: "nur %d Personen statt der gesuchten %d"},
	"search.why.keyword":       {i18n.EN: "mentions %s", i18n.DE: "erwähnt %s"},
	"search.why.budget":        {i18n.EN: "%s fits the budget you asked for", i18n.DE: "%s passt zum gewünschten Budget"},
	"search.why.window":        {i18n.EN: "starts %s, inside %s–%s", i18n.DE: "beginnt %s, innerhalb %s–%s"},
	"search.why.attention":     {i18n.EN: "needs attention now", i18n.DE: "braucht jetzt Aufmerksamkeit"},
	"search.why.profileRegion": {i18n.EN: "near your region", i18n.DE: "in Ihrer Region"},
	"search.why.profileTrade":  {i18n.EN: "matches a trade on your profile", i18n.DE: "passt zu einem Gewerk in Ihrem Profil"},
	"search.why.profileCrew":   {i18n.EN: "covers your usual crew of %d", i18n.DE: "deckt Ihre übliche Kolonnengröße von %d"},
	"search.why.profileDocs":   {i18n.EN: "you already hold the papers it needs", i18n.DE: "Sie haben die nötigen Papiere bereits"},
	"search.why.textRelevance": {i18n.EN: "matches your search text", i18n.DE: "passt zu Ihrem Suchtext"},
	"search.why.default":       {i18n.EN: "matches your filters", i18n.DE: "passt zu Ihren Filtern"},

	// Why-line fragments specific to a ranked crew.
	"search.why.crewRegion": {i18n.EN: "works in %s", i18n.DE: "arbeitet in %s"},
	"search.why.crewRating": {i18n.EN: "rated %.1f from %d jobs", i18n.DE: "Bewertung %.1f aus %d Einsätzen"},

	// What widening dropped, filled into the shared i18n key "search.widened".
	"search.widen.timeframe":  {i18n.EN: "the timeframe", i18n.DE: "den Zeitraum"},
	"search.widen.budget":     {i18n.EN: "the budget", i18n.DE: "das Budget"},
	"search.widen.documents":  {i18n.EN: "the document requirements", i18n.DE: "die Dokumentenanforderungen"},
	"search.widen.status":     {i18n.EN: "the status filter", i18n.DE: "den Statusfilter"},
	"search.widen.region":     {i18n.EN: "the region", i18n.DE: "die Region"},
	"search.widen.trade":      {i18n.EN: "the trade", i18n.DE: "das Gewerk"},
	"search.widen.everything": {i18n.EN: "every filter", i18n.DE: "alle Filter"},

	// Refine chips.
	"search.chip.alsoSee": {i18n.EN: "also see %s", i18n.DE: "auch: %s"},
	"search.chip.near":    {i18n.EN: "near %s", i18n.DE: "in der Nähe von %s"},
	"search.chip.status":  {i18n.EN: "status: %s", i18n.DE: "Status: %s"},

	// Card chrome.
	"search.crewCount":  {i18n.EN: "%d people", i18n.DE: "%d Leute"},
	"search.kind.offer": {i18n.EN: "offer", i18n.DE: "Auftrag"},
	"search.kind.crew":  {i18n.EN: "crew", i18n.DE: "Kolonne"},

	// Mechanical summary sentence fragments.
	"search.summary.for": {i18n.EN: "for %s", i18n.DE: "für %s"},
	"search.summary.in":  {i18n.EN: "in %s", i18n.DE: "in %s"},
}

// tr looks a package-local key up for lang, falling back to English and then
// the bare key, mirroring i18n.T's own fallback rule so a missing
// translation is visible rather than silently blank. Extra args are applied
// with fmt.Sprintf.
func tr(lang i18n.Lang, key string, args ...any) string {
	entry, ok := localCatalog[key]
	if !ok {
		return key
	}
	s := entry[lang]
	if s == "" {
		s = entry[i18n.EN]
	}
	if s == "" {
		return key
	}
	if len(args) == 0 {
		return s
	}
	return fmt.Sprintf(s, args...)
}

// --- German/English domain vocabulary for the mechanical (no-model) parser --

// tradeSynonyms maps each normalized trade slug to every English and German
// surface form the mechanical fallback should recognize. The slug itself is
// always included so the English path keeps working unchanged.
var tradeSynonyms = map[string][]string{
	"electrical": {"electrical", "electric", "elektro", "elektriker"},
	"sanitary":   {"sanitary", "sanitär", "sanitaer", "klempner"},
	"steel":      {"steel", "stahlbau", "stahlbauer", "schweißer", "schweisser"},
	"interior":   {"interior", "innenausbau", "ladenbau"},
	"energy":     {"energy", "energietechnik", "photovoltaik", "solar", "wärmepumpe", "waermepumpe"},
	"drywall":    {"drywall", "trockenbau"},
	"hvac":       {"hvac", "heizung", "klima", "lüftung", "lueftung"},
}

var docSynonyms = map[string][]string{
	"a1":           {"a1"},
	"insurance":    {"insurance", "versicherung", "haftpflicht"},
	"certificates": {"certificates", "certificate", "zertifikat", "zertifikate", "nachweis", "nachweise"},
	"tax":          {"tax", "steuer"},
}

var statusSynonyms = map[string][]string{
	"open":      {"open", "offen"},
	"requested": {"requested", "angefragt", "anfrage"},
	"process":   {"process", "in arbeit", "in bearbeitung", "laufend"},
	"done":      {"done", "abgeschlossen", "fertig"},
}

// crewSeekingWords and offerSeekingWords are the mechanical-fallback signal
// for which side of the market a sentence is about, in both languages —
// see inferKind.
var crewSeekingWords = []string{"crew", "team", "kolonne", "who can field", "who has", "wer hat", "wer kann", "wer stellt"}
var offerSeekingWords = []string{"job", "offer", "opportunity", "auftrag", "ausschreibung", "stelle", "who is looking", "wer sucht"}

// inferKind reads the crew-vs-offer seeking cues out of a raw sentence. An
// empty return means neither side signaled strongly, so the caller should
// stay ambiguous rather than guess (see WantsOffers/WantsCrews).
func inferKind(raw string) string {
	low := strings.ToLower(raw)
	for _, w := range crewSeekingWords {
		if strings.Contains(low, w) {
			return "find_crews"
		}
	}
	for _, w := range offerSeekingWords {
		if strings.Contains(low, w) {
			return "find_offers"
		}
	}
	return ""
}

// matchTradeSlug reports whether word is a known trade surface form in
// either language, returning its normalized slug.
func matchTradeSlug(word string) (string, bool) {
	w := strings.ToLower(strings.TrimSpace(word))
	for slug, syns := range tradeSynonyms {
		for _, s := range syns {
			if w == s {
				return slug, true
			}
		}
	}
	return "", false
}

// regionVocab maps a canonical (corpus-matching) city name to every German
// and English surface form the mechanical parser should recognize.
var regionVocab = map[string][]string{
	"München":   {"münchen", "munich"},
	"Berlin":    {"berlin"},
	"Hamburg":   {"hamburg"},
	"Stuttgart": {"stuttgart"},
	"Frankfurt": {"frankfurt"},
	"Köln":      {"köln", "koeln", "cologne"},
	"Dortmund":  {"dortmund"},
	"Leipzig":   {"leipzig"},
	"Dresden":   {"dresden"},
	"Nürnberg":  {"nürnberg", "nuernberg", "nuremberg"},
	"Bremen":    {"bremen"},
	"Hannover":  {"hannover", "hanover"},
	"Wien":      {"wien", "vienna"},
	"Salzburg":  {"salzburg"},
	"Graz":      {"graz"},
	"Linz":      {"linz"},
	"Zürich":    {"zürich", "zuerich", "zurich"},
	"Basel":     {"basel"},
	"Genf":      {"genf", "geneva", "genève", "geneve"},
	"Rotterdam": {"rotterdam"},
	"Amsterdam": {"amsterdam"},
	"Utrecht":   {"utrecht"},
	"DACH":      {"dach"},
}

// countryVocab maps a country name (either language) to the two-letter
// country code the demo corpus's "City, CC" region strings end with, so a
// country-level ask ("in the Netherlands"/"in den Niederlanden") narrows
// correctly even though no offer literally contains that country's name.
var countryVocab = map[string][]string{
	"DE": {"germany", "deutschland", "german"},
	"AT": {"austria", "österreich", "oesterreich", "austrian"},
	"CH": {"switzerland", "schweiz", "swiss"},
	"NL": {"netherlands", "niederlande", "niederlanden", "dutch", "holland"},
	"BE": {"belgium", "belgien"},
}

// normalizeRegion maps a region word (in either language) onto the form the
// corpus actually uses, so an English or German ask for the same place
// narrows the same way. A region with no known mapping passes through
// unchanged — normalizeRegion never invents a place that wasn't asked for.
func normalizeRegion(word string) string {
	w := strings.ToLower(strings.TrimSpace(word))
	for canonical, forms := range regionVocab {
		for _, f := range forms {
			if w == f {
				return canonical
			}
		}
	}
	for code, forms := range countryVocab {
		for _, f := range forms {
			if w == f {
				return code
			}
		}
	}
	return word
}

// extractRegions scans raw text for every known city or country vocabulary
// hit, used by the mechanical fallback (the model path gets regions
// straight from the LLM). Substring matching mirrors how the rest of the
// mechanical parser works (knownTrades, knownDocs, knownStatus).
func extractRegions(low string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(canonical string) {
		if !seen[canonical] {
			seen[canonical] = true
			out = append(out, canonical)
		}
	}
	for canonical, forms := range regionVocab {
		for _, f := range forms {
			if strings.Contains(low, f) {
				add(canonical)
				break
			}
		}
	}
	for code, forms := range countryVocab {
		for _, f := range forms {
			if strings.Contains(low, f) {
				add(code)
				break
			}
		}
	}
	return out
}
