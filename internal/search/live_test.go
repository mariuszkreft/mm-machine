package search

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"mm-machine/internal/app"
	"mm-machine/internal/llm"
	"mm-machine/internal/model"
	"mm-machine/internal/store"
)

// TestLive_Run hits the real fleet endpoint and confirms a real sentence
// parses into a real intent and a ranked, explained result set. It is
// guarded by testing.Short() so `go test -short` never touches the network.
func TestLive_Run(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live check in -short mode")
	}
	client := llm.New(llm.Config{MaxTokens: 768})
	deps := app.Deps{Store: store.NewMemory(), LLM: client}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	raw := "I need an energy crew in Munich from October for three weeks, not Vienna"
	intent, fc := Parse(ctx, deps, raw, model.Profile{})
	if intent.Fallback {
		t.Fatal("expected the live model path, got the mechanical fallback")
	}
	intentJSON, _ := json.MarshalIndent(intent, "", "  ")
	t.Logf("intent:\n%s", intentJSON)
	t.Logf("facets: window=%v..%v excludeRegions=%v", fc.Start, fc.End, fc.ExcludeRegions)

	result, err := Search(ctx, deps, intent, fc, model.Profile{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	for _, m := range result.Matches {
		t.Logf("%s %3d%% %-40s %v", m.Offer.ID, m.Fit, m.Offer.Title, m.Why)
	}
}
