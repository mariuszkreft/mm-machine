package search

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// facets carries intent detail that has nowhere to live in model.Intent: a
// concrete date window, budget bounds and negations. It stays local to this
// package (per the slice's scope) and travels alongside a model.Intent
// rather than inside it.
//
// Everything here is derived from the raw sentence by deterministic code,
// never by the model, so it is exercisable without a live cluster.
type facets struct {
	Start, End           time.Time
	HasWindow            bool
	BudgetMin, BudgetMax int // EUR
	HasBudgetMin         bool
	HasBudgetMax         bool
	ExcludeTrades        []string
	ExcludeRegions       []string
}

// deriveFacets extracts the window/budget/negation facets from the raw
// sentence. It is called for both the model and mechanical parse paths so
// neither has to understand dates or money on its own.
func deriveFacets(raw string, now time.Time) facets {
	var f facets
	if start, end, ok := parseWindowAt(raw, now); ok {
		f.Start, f.End, f.HasWindow = start, end, true
	}
	f.BudgetMin, f.HasBudgetMin, f.BudgetMax, f.HasBudgetMax = parseBudgetBounds(raw)
	f.ExcludeTrades, f.ExcludeRegions = parseNegations(raw)
	return f
}

// stripNegated removes anything the sentence explicitly ruled out from a
// positive facet list, in case the parser (model or mechanical) put it there
// anyway because the word appeared in the sentence.
func stripNegated(list, excluded []string) []string {
	if len(excluded) == 0 || len(list) == 0 {
		return list
	}
	out := make([]string, 0, len(list))
	for _, v := range list {
		if !hasFold(excluded, v) {
			out = append(out, v)
		}
	}
	return out
}

// --- dates and durations -----------------------------------------------------

// monthNames covers both English and German month names/abbreviations
// (including the Austrian "Jänner"), so a raw sentence in either language
// resolves to the same time.Month.
var monthNames = map[string]time.Month{
	"jan": time.January, "january": time.January, "januar": time.January,
	"jän": time.January, "jänner": time.January,
	"feb": time.February, "february": time.February, "februar": time.February,
	"mar": time.March, "march": time.March,
	"mär": time.March, "märz": time.March, "maerz": time.March, "mrz": time.March,
	"apr": time.April, "april": time.April,
	"may": time.May, "mai": time.May,
	"jun": time.June, "june": time.June, "juni": time.June,
	"jul": time.July, "july": time.July, "juli": time.July,
	"aug": time.August, "august": time.August,
	"sep": time.September, "sept": time.September, "september": time.September,
	"oct": time.October, "october": time.October, "okt": time.October, "oktober": time.October,
	"nov": time.November, "november": time.November,
	"dec": time.December, "december": time.December, "dez": time.December, "dezember": time.December,
}

var monthRe = regexp.MustCompile(`(?i)\b(jan(?:uary|uar)?|jän(?:ner)?|feb(?:ruary|ruar)?|mär(?:z)?|maerz|mrz|mar(?:ch)?|apr(?:il)?|mai|may|jun(?:e|i)?|jul(?:y|i)?|aug(?:ust)?|sept?(?:ember)?|okt(?:ober)?|oct(?:ober)?|nov(?:ember)?|dez(?:ember)?|dec(?:ember)?)\b`)

// wordNumbers and deWordNumbers spell out small counts in English and
// German, for both a duration ("three weeks"/"drei Wochen") and a crew size
// ("six fitters"/"sechs Monteure").
var wordNumbers = map[string]int{
	"a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
	"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
}

var deWordNumbers = map[string]int{
	"ein": 1, "eine": 1, "einen": 1, "einem": 1, "zwei": 2, "drei": 3, "vier": 4,
	"fünf": 5, "fuenf": 5, "sechs": 6, "sieben": 7, "acht": 8, "neun": 9, "zehn": 10,
}

// parseAnyNumber reads a digit string or a spelled-out English/German
// number word, so every count-bearing phrase (duration, crew size) can share
// one parser regardless of the sentence's language.
func parseAnyNumber(tok string) (int, bool) {
	tok = strings.ToLower(tok)
	if v, err := strconv.Atoi(tok); err == nil {
		return v, true
	}
	if v, ok := wordNumbers[tok]; ok {
		return v, true
	}
	if v, ok := deWordNumbers[tok]; ok {
		return v, true
	}
	return 0, false
}

var durationRe = regexp.MustCompile(`(?i)(\d+|a|an|one|two|three|four|five|six|seven|eight|nine|ten|` +
	`ein|eine|einen|zwei|drei|vier|fünf|fuenf|sechs|sieben|acht|neun|zehn)\s+` +
	`(day|week|month|tag|woche|monat)(?:s|e|en|n)?\b`)

// crewSizeRe reads "six fitters"/"sechs Monteure" style phrases; crewOfRe
// reads the "crew of six"/"Kolonne von sechs" style instead.
var crewSizeRe = regexp.MustCompile(`(?i)\b(\d+|one|two|three|four|five|six|seven|eight|nine|ten|` +
	`ein|eine|einen|zwei|drei|vier|fünf|fuenf|sechs|sieben|acht|neun|zehn)\s+` +
	`(monteure?|leute|arbeiter|mitarbeiter|kolonne|mann|personen|crew|electricians?|fitters?|people|workers?|team)\b`)

var crewOfRe = regexp.MustCompile(`(?i)\b(?:crew|team|kolonne)\s+(?:of|von|aus)\s+(\d+)\b`)

// parseCrewSize reads a crew-size phrase out of the raw sentence in either
// language. It is the mechanical-fallback counterpart to the model's
// crewSize field: the LLM path gets the count from the JSON schema, this is
// what the no-model path uses instead.
func parseCrewSize(raw string) (int, bool) {
	low := strings.ToLower(raw)
	if m := crewSizeRe.FindStringSubmatch(low); m != nil {
		if n, ok := parseAnyNumber(m[1]); ok {
			return n, true
		}
	}
	if m := crewOfRe.FindStringSubmatch(low); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n, true
		}
	}
	return 0, false
}

// parseWindowAt turns a date/duration phrase into a concrete [start, end)
// window. It understands "from <month> for <n> <unit>", a bare month
// ("in October"), "next <n> <unit>", and "this week"/"this month". Anything
// else reports ok=false rather than guessing.
func parseWindowAt(raw string, now time.Time) (start, end time.Time, ok bool) {
	low := strings.ToLower(raw)

	monthStart, hasMonth := parseMonthStart(low, now)
	n, unit, hasDur := parseDuration(low)

	next := strings.Contains(low, "next") || strings.Contains(low, "nächst") || strings.Contains(low, "naechst")
	thisWeek := strings.Contains(low, "this week") || strings.Contains(low, "diese woche") || strings.Contains(low, "diesen woche")
	thisMonth := strings.Contains(low, "this month") || strings.Contains(low, "diesen monat") || strings.Contains(low, "diesem monat")

	switch {
	case hasMonth && hasDur:
		return monthStart, addUnits(monthStart, n, unit), true
	case hasMonth:
		return monthStart, monthStart.AddDate(0, 1, 0), true
	case next && hasDur:
		return dayStart(now), addUnits(dayStart(now), n, unit), true
	case hasDur:
		// A bare duration ("for three weeks"/"für drei Wochen") with no
		// anchor starts now.
		return dayStart(now), addUnits(dayStart(now), n, unit), true
	case thisWeek:
		return dayStart(now), dayStart(now).AddDate(0, 0, 7), true
	case thisMonth:
		return dayStart(now), dayStart(now).AddDate(0, 1, 0), true
	}
	return time.Time{}, time.Time{}, false
}

func dayStart(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func addUnits(t time.Time, n int, unit string) time.Time {
	switch strings.ToLower(unit) {
	case "week", "woche":
		return t.AddDate(0, 0, 7*n)
	case "month", "monat":
		return t.AddDate(0, n, 0)
	default: // day, tag
		return t.AddDate(0, 0, n)
	}
}

func parseDuration(low string) (n int, unit string, ok bool) {
	m := durationRe.FindStringSubmatch(low)
	if m == nil {
		return 0, "", false
	}
	n, ok = parseAnyNumber(m[1])
	if !ok {
		return 0, "", false
	}
	return n, m[2], true
}

func parseMonthStart(low string, now time.Time) (time.Time, bool) {
	m := monthRe.FindStringSubmatch(low)
	if m == nil {
		return time.Time{}, false
	}
	month, known := monthNames[strings.ToLower(m[1])]
	if !known {
		return time.Time{}, false
	}
	year := now.Year()
	// If that month already passed this year, the visitor means next year's.
	if month < now.Month() {
		year++
	}
	return time.Date(year, month, 1, 0, 0, 0, 0, now.Location()), true
}

// --- budget bounds -----------------------------------------------------------

var budgetMaxRe = regexp.MustCompile(`(?i)\b(?:under|below|max(?:imum)?|less than|` +
	`unter|höchstens|hoechstens|maximal|weniger als)\s*(?:eur|€)?\s*([\d][\d.,]*)\s*(k|m|mio|tsd)?\b`)
var budgetMinRe = regexp.MustCompile(`(?i)\b(?:over|above|min(?:imum)?|at least|more than|` +
	`über|ueber|mindestens|mehr als)\s*(?:eur|€)?\s*([\d][\d.,]*)\s*(k|m|mio|tsd)?\b`)

// parseBudgetBounds reads "under 100k" / "over EUR 50k" / "unter 100k" /
// "mindestens 50k" style phrases out of the raw sentence into EUR bounds.
func parseBudgetBounds(raw string) (min int, hasMin bool, max int, hasMax bool) {
	if m := budgetMaxRe.FindStringSubmatch(raw); m != nil {
		if v, ok := parseMoneyToken(m[1], m[2]); ok {
			max, hasMax = v, true
		}
	}
	if m := budgetMinRe.FindStringSubmatch(raw); m != nil {
		if v, ok := parseMoneyToken(m[1], m[2]); ok {
			min, hasMin = v, true
		}
	}
	return
}

func parseMoneyToken(numStr, suffix string) (int, bool) {
	numStr = strings.ReplaceAll(numStr, ",", "")
	f, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, false
	}
	switch strings.ToLower(suffix) {
	case "k", "tsd":
		f *= 1_000
	case "m", "mio":
		f *= 1_000_000
	}
	return int(f), true
}

var offerBudgetRe = regexp.MustCompile(`([\d][\d.,]*)\s*(?i:(k|m))?`)

// parseOfferBudget reads an offer's free-text Budget field ("EUR 146k") into
// a comparable EUR amount. Offers whose budget doesn't parse are never
// excluded by a budget bound — an unparseable field is not the same as a
// bad match.
func parseOfferBudget(s string) (int, bool) {
	m := offerBudgetRe.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	return parseMoneyToken(m[1], m[2])
}

// --- negations ----------------------------------------------------------------

var negationRe = regexp.MustCompile(`(?i)\b(?:not|except|excluding|no|` +
	`nicht|außer|ausser|ohne|kein|keine|keinen)\s+([\p{L}][\p{L}0-9-]*)`)

// parseNegations pulls single-word exclusions ("not Vienna"/"nicht Wien",
// "excluding steel"/"außer Stahlbau") out of the raw sentence, sorting each
// into a trade or a region exclusion depending on whether it names a known
// trade.
func parseNegations(raw string) (excludeTrades, excludeRegions []string) {
	for _, m := range negationRe.FindAllStringSubmatch(raw, -1) {
		word := strings.TrimSpace(m[1])
		if word == "" {
			continue
		}
		if slug, ok := matchTradeSlug(word); ok {
			excludeTrades = append(excludeTrades, slug)
		} else {
			excludeRegions = append(excludeRegions, word)
		}
	}
	return
}

func anyContainsFold(want []string, haystacks ...string) bool {
	for _, w := range want {
		for _, h := range haystacks {
			if containsFold(w, h) {
				return true
			}
		}
	}
	return false
}
