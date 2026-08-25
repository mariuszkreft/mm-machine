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

var monthNames = map[string]time.Month{
	"jan": time.January, "january": time.January,
	"feb": time.February, "february": time.February,
	"mar": time.March, "march": time.March,
	"apr": time.April, "april": time.April,
	"may": time.May,
	"jun": time.June, "june": time.June,
	"jul": time.July, "july": time.July,
	"aug": time.August, "august": time.August,
	"sep": time.September, "sept": time.September, "september": time.September,
	"oct": time.October, "october": time.October,
	"nov": time.November, "november": time.November,
	"dec": time.December, "december": time.December,
}

var monthRe = regexp.MustCompile(`(?i)\b(jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sept?(?:ember)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)\b`)

var wordNumbers = map[string]int{
	"a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
	"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
}

var durationRe = regexp.MustCompile(`(?i)(\d+|a|an|one|two|three|four|five|six|seven|eight|nine|ten)\s+(day|week|month)s?\b`)

// parseWindowAt turns a date/duration phrase into a concrete [start, end)
// window. It understands "from <month> for <n> <unit>", a bare month
// ("in October"), "next <n> <unit>", and "this week"/"this month". Anything
// else reports ok=false rather than guessing.
func parseWindowAt(raw string, now time.Time) (start, end time.Time, ok bool) {
	low := strings.ToLower(raw)

	monthStart, hasMonth := parseMonthStart(low, now)
	n, unit, hasDur := parseDuration(low)

	switch {
	case hasMonth && hasDur:
		return monthStart, addUnits(monthStart, n, unit), true
	case hasMonth:
		return monthStart, monthStart.AddDate(0, 1, 0), true
	case strings.Contains(low, "next") && hasDur:
		return dayStart(now), addUnits(dayStart(now), n, unit), true
	case hasDur:
		// A bare duration ("for three weeks") with no anchor starts now.
		return dayStart(now), addUnits(dayStart(now), n, unit), true
	case strings.Contains(low, "this week"):
		return dayStart(now), dayStart(now).AddDate(0, 0, 7), true
	case strings.Contains(low, "this month"):
		return dayStart(now), dayStart(now).AddDate(0, 1, 0), true
	}
	return time.Time{}, time.Time{}, false
}

func dayStart(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func addUnits(t time.Time, n int, unit string) time.Time {
	switch unit {
	case "week":
		return t.AddDate(0, 0, 7*n)
	case "month":
		return t.AddDate(0, n, 0)
	default: // day
		return t.AddDate(0, 0, n)
	}
}

func parseDuration(low string) (n int, unit string, ok bool) {
	m := durationRe.FindStringSubmatch(low)
	if m == nil {
		return 0, "", false
	}
	if v, err := strconv.Atoi(m[1]); err == nil {
		n = v
	} else if v, known := wordNumbers[m[1]]; known {
		n = v
	} else {
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

var budgetMaxRe = regexp.MustCompile(`(?i)\b(?:under|below|max(?:imum)?|less than)\s*(?:eur|€)?\s*([\d][\d.,]*)\s*(k|m)?\b`)
var budgetMinRe = regexp.MustCompile(`(?i)\b(?:over|above|min(?:imum)?|at least|more than)\s*(?:eur|€)?\s*([\d][\d.,]*)\s*(k|m)?\b`)

// parseBudgetBounds reads "under 100k" / "over EUR 50k" style phrases out of
// the raw sentence into EUR bounds.
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
	case "k":
		f *= 1_000
	case "m":
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

var negationRe = regexp.MustCompile(`(?i)\b(?:not|except|excluding|no)\s+([a-zA-Z][\w-]*)`)

// parseNegations pulls single-word exclusions ("not Vienna", "excluding
// steel") out of the raw sentence, sorting each into a trade or a region
// exclusion depending on whether it names a known trade.
func parseNegations(raw string) (excludeTrades, excludeRegions []string) {
	for _, m := range negationRe.FindAllStringSubmatch(raw, -1) {
		word := strings.TrimSpace(m[1])
		if word == "" {
			continue
		}
		if hasFold(knownTrades, word) {
			excludeTrades = append(excludeTrades, strings.ToLower(word))
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
