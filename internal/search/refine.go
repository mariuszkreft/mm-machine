package search

import (
	"strings"

	"mm-machine/internal/i18n"
	"mm-machine/internal/model"
)

// chip is one refine control: clicking it re-runs the search with a single
// facet changed, so an empty or over-narrow result is a next move, not a
// dead end.
type chip struct {
	Label  string
	Refine string // "<action>:<facet>:<value>", read by parseRefine
}

const maxChips = 6

// buildChips looks at the actual ranked result set — not the whole
// catalogue — and offers the facets it finds that the intent didn't already
// ask for: other trades present, nearby regions, adjacent statuses. Labels
// are rendered in lang; the Refine value stays the raw slug/name so clicking
// the chip still overrides the right facet regardless of display language.
func buildChips(all []model.Match, intent model.Intent, lang i18n.Lang) []chip {
	chips := []chip{}
	chips = append(chips, tradeChips(all, intent, lang)...)
	chips = append(chips, regionChips(all, intent, lang)...)
	chips = append(chips, statusChips(all, intent, lang)...)
	if len(chips) > maxChips {
		chips = chips[:maxChips]
	}
	return chips
}

func tradeChips(all []model.Match, intent model.Intent, lang i18n.Lang) []chip {
	seen := map[string]bool{}
	out := []chip{}
	for _, m := range all {
		t := trade(m.Offer)
		if t == "" || hasFold(intent.Trades, t) || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, chip{Label: tr(lang, "search.chip.alsoSee", i18n.T(lang, "trade."+t)), Refine: "set:trade:" + t})
	}
	return out
}

func regionChips(all []model.Match, intent model.Intent, lang i18n.Lang) []chip {
	seen := map[string]bool{}
	out := []chip{}
	for _, m := range all {
		r := m.Offer.Region
		if r == "" {
			r = m.Offer.Location
		}
		if r == "" || anyContainsFold(intent.Regions, r) || seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, chip{Label: tr(lang, "search.chip.near", r), Refine: "set:region:" + r})
	}
	return out
}

func statusChips(all []model.Match, intent model.Intent, lang i18n.Lang) []chip {
	seen := map[string]bool{}
	out := []chip{}
	for _, m := range all {
		s := m.Offer.Status
		if s == "" || hasFold(intent.Statuses, s) || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, chip{Label: tr(lang, "search.chip.status", i18n.T(lang, "offer.status."+s)), Refine: "set:status:" + s})
	}
	return out
}

// refine is one facet override requested by clicking a chip.
type refine struct {
	Action string // "set" or "drop"
	Facet  string // "trade" | "region" | "status"
	Value  string
}

// parseRefine reads the compact "<action>:<facet>:<value>" encoding a chip
// submits. An empty string is not a refine request.
func parseRefine(raw string) (refine, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return refine{}, false
	}
	parts := strings.SplitN(raw, ":", 3)
	if len(parts) < 2 {
		return refine{}, false
	}
	rf := refine{Action: parts[0], Facet: parts[1]}
	if len(parts) == 3 {
		rf.Value = parts[2]
	}
	if rf.Action != "set" && rf.Action != "drop" {
		return refine{}, false
	}
	return rf, true
}

// applyRefine overrides one facet on an already-parsed intent, so a chip
// click re-runs the search without asking the model to parse anything
// again.
func applyRefine(intent model.Intent, rf refine) model.Intent {
	var set []string
	if rf.Action == "set" && rf.Value != "" {
		set = []string{rf.Value}
	}
	switch rf.Facet {
	case "trade":
		intent.Trades = set
	case "region":
		intent.Regions = set
	case "status":
		intent.Statuses = set
	}
	return intent
}
