package search

import (
	"context"

	"mm-machine/internal/app"
	"mm-machine/internal/model"
	"mm-machine/internal/store"
)

// widenInfo records what widening did, so the surface can say so instead of
// quietly showing an unrelated list. The baseline widened silently; this
// makes it honest.
type widenInfo struct {
	Applied bool
	Dropped string // human label of what was dropped to find something
}

// dropStep is one candidate facet to drop while widening, tried in order
// from least to most central to the ask. Negations are never dropped: a
// visitor who said "not Vienna" meant it, even with nothing else to show.
type dropStep struct {
	label string
	apply func(store.OfferFilter, facets) (store.OfferFilter, facets)
}

var dropOrder = []dropStep{
	{"the timeframe", func(b store.OfferFilter, f facets) (store.OfferFilter, facets) {
		f.HasWindow = false
		return b, f
	}},
	{"the budget", func(b store.OfferFilter, f facets) (store.OfferFilter, facets) {
		f.HasBudgetMin, f.HasBudgetMax = false, false
		return b, f
	}},
	{"the document requirements", func(b store.OfferFilter, f facets) (store.OfferFilter, facets) {
		b.Requirements = nil
		return b, f
	}},
	{"the status filter", func(b store.OfferFilter, f facets) (store.OfferFilter, facets) {
		b.Statuses = nil
		return b, f
	}},
	{"the region", func(b store.OfferFilter, f facets) (store.OfferFilter, facets) {
		b.Regions = nil
		return b, f
	}},
	{"the trade", func(b store.OfferFilter, f facets) (store.OfferFilter, facets) {
		b.Trades = nil
		return b, f
	}},
}

// widenSearch runs when the exact filter finds nothing. It drops one facet
// at a time, in dropOrder, until something matches, and reports which one
// mattered. If nothing individually helps, it falls back to everything
// (still honoring exclusions) rather than showing a dead end.
func widenSearch(ctx context.Context, deps app.Deps, base store.OfferFilter, fc facets) ([]model.Offer, widenInfo, error) {
	for _, step := range dropOrder {
		b2, f2 := step.apply(base, fc)
		offers, err := deps.Store.ListOffers(ctx, b2)
		if err != nil {
			return nil, widenInfo{}, err
		}
		offers = applyPostFilter(offers, f2)
		if len(offers) > 0 {
			return offers, widenInfo{Applied: true, Dropped: step.label}, nil
		}
	}
	fallback := facets{ExcludeTrades: fc.ExcludeTrades, ExcludeRegions: fc.ExcludeRegions}
	offers, err := deps.Store.ListOffers(ctx, store.OfferFilter{Limit: base.Limit})
	if err != nil {
		return nil, widenInfo{}, err
	}
	offers = applyPostFilter(offers, fallback)
	return offers, widenInfo{Applied: true, Dropped: "every filter"}, nil
}
