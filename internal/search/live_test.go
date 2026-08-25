package search

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"mm-machine/internal/app"
	"mm-machine/internal/i18n"
	"mm-machine/internal/llm"
	"mm-machine/internal/model"
	"mm-machine/internal/store"
)

// The cluster is shared, so a live check has to allow for it being busy: the
// point is that the model path works, not that it is fast.
//
// TestLive_Run hits the real fleet endpoint and confirms a real sentence
// parses into a real intent and a ranked, explained result set. It is
// guarded by testing.Short() so `go test -short` never touches the network.
func TestLive_Run(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live check in -short mode")
	}
	client := llm.New(llm.Config{MaxTokens: 768})
	deps := app.Deps{Store: store.NewMemory(), LLM: client}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	raw := "I need an energy crew in Munich from October for three weeks, not Vienna"
	intent, fc := Parse(ctx, deps, raw, model.Profile{})
	if intent.Fallback {
		t.Fatal("expected the live model path, got the mechanical fallback")
	}
	intentJSON, _ := json.MarshalIndent(intent, "", "  ")
	t.Logf("intent:\n%s", intentJSON)
	t.Logf("facets: window=%v..%v excludeRegions=%v", fc.Start, fc.End, fc.ExcludeRegions)

	result, err := Search(ctx, deps, intent, fc, model.Profile{}, i18n.EN)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	for _, m := range result.Matches {
		t.Logf("%s %3d%% %-40s %v", m.Offer.ID, m.Fit, m.Offer.Title, m.Why)
	}
}

// TestLive_RunGerman is TestLive_Run's German mirror: the same kind of
// sentence, in the DACH market's own working language, against the same
// live endpoint — see TASK-m2herd-search.md's "Done" requirement for a real
// German query's intent JSON and ranked output.
func TestLive_RunGerman(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live check in -short mode")
	}
	client := llm.New(llm.Config{MaxTokens: 768})
	deps := app.Deps{Store: store.NewMemory(), LLM: client}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	raw := "6 Monteure in München ab Oktober, A1 vorhanden"
	intent, fc := Parse(ctx, deps, raw, model.Profile{})
	if intent.Fallback {
		t.Fatal("expected the live model path, got the mechanical fallback")
	}
	intentJSON, _ := json.MarshalIndent(intent, "", "  ")
	t.Logf("intent:\n%s", intentJSON)
	t.Logf("facets: window=%v..%v excludeRegions=%v", fc.Start, fc.End, fc.ExcludeRegions)

	result, err := runSides(ctx, deps, intent, fc, model.Profile{}, i18n.DE)
	if err != nil {
		t.Fatalf("runSides: %v", err)
	}
	for _, m := range result.Matches {
		t.Logf("%s %-4s %3d%% %-40s %v", m.Ref(), m.Kind, m.Fit, m.Title(), m.Why)
	}
}
