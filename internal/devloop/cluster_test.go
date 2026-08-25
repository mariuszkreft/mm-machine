package devloop

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mm-machine/internal/app"
	"mm-machine/internal/llm"
	"mm-machine/internal/model"
	"mm-machine/internal/store"
)

// fakeLLM is a test double for llm.Client. jsonFn drives what JSON() returns;
// the other methods are unused by the dev loop but implemented to satisfy the
// interface.
type fakeLLM struct {
	model  string
	jsonFn func(req llm.Request, out any) error
}

func (f *fakeLLM) Chat(context.Context, llm.Request) (llm.Response, error) {
	return llm.Response{}, nil
}
func (f *fakeLLM) Stream(context.Context, llm.Request, func(llm.Delta) error) error { return nil }
func (f *fakeLLM) JSON(_ context.Context, req llm.Request, _ string, out any) error {
	if f.jsonFn == nil {
		return fmt.Errorf("fakeLLM: no jsonFn configured")
	}
	return f.jsonFn(req, out)
}
func (f *fakeLLM) Health(context.Context) error { return nil }
func (f *fakeLLM) Model() string                { return f.model }

var _ llm.Client = (*fakeLLM)(nil)

// setTempBacklogPath keeps Regenerate's docs/backlog.md write inside the
// test's scratch dir instead of the package directory.
func setTempBacklogPath(t *testing.T) {
	t.Helper()
	t.Setenv("BACKLOG_PATH", filepath.Join(t.TempDir(), "backlog.md"))
}

func seedFeedback(t *testing.T, mem *store.Memory, items []model.Feedback) []model.Feedback {
	t.Helper()
	ctx := context.Background()
	out := make([]model.Feedback, 0, len(items))
	for _, f := range items {
		saved, err := mem.CreateFeedback(ctx, f)
		if err != nil {
			t.Fatalf("seed feedback: %v", err)
		}
		out = append(out, saved)
	}
	return out
}

// canned two-cluster response used by several tests: the first two feedback
// ids become a small-effort bug theme, the rest a large-effort request theme.
func twoClusterJSON(ids []int64) []llmCluster {
	return []llmCluster{
		{
			Title:       "Search is slow",
			Rationale:   "Users report search lag.",
			Theme:       "slow-search",
			Kind:        "bug",
			FeedbackIDs: ids[:2],
			Severity:    4,
			Effort:      "S",
		},
		{
			Title:       "Add export filters",
			Rationale:   "Users want to filter exports.",
			Theme:       "export-filters",
			Kind:        "request",
			FeedbackIDs: ids[2:],
			Severity:    2,
			Effort:      "L",
		},
	}
}

func jsonFnReturning(clusters []llmCluster) func(llm.Request, any) error {
	return func(_ llm.Request, out any) error {
		buf, err := json.Marshal(clusters)
		if err != nil {
			return err
		}
		return json.Unmarshal(buf, out)
	}
}

func newFeedback(kind, theme, verbatim string, severity int) model.Feedback {
	return model.Feedback{Kind: kind, Theme: theme, Verbatim: verbatim, Severity: severity, Source: "chat", Role: "owner"}
}

func TestRegenerate_LLMClusteringHappyPath(t *testing.T) {
	setTempBacklogPath(t)
	mem := store.NewMemory()
	fb := seedFeedback(t, mem, []model.Feedback{
		newFeedback("bug", "", "search takes forever", 4),
		newFeedback("bug", "", "search is really slow", 4),
		newFeedback("request", "", "let me filter my exports", 2),
		newFeedback("request", "", "need export filters please", 2),
	})
	ids := []int64{fb[0].ID, fb[1].ID, fb[2].ID, fb[3].ID}

	fake := &fakeLLM{model: "fake-model", jsonFn: jsonFnReturning(twoClusterJSON(ids))}
	deps := app.Deps{Store: mem, LLM: fake, LLMModel: "fake-model"}

	items, err := Regenerate(context.Background(), deps)
	if err != nil {
		t.Fatalf("Regenerate: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 backlog items, got %d", len(items))
	}
	// slow-search: count=2, avgSeverity=4, effort S -> score = 2*4/1 = 8
	// export-filters: count=2, avgSeverity=2, effort L -> score = 2*2/2.5 = 1.6
	if items[0].Theme != "slow-search" || items[1].Theme != "export-filters" {
		t.Fatalf("unexpected order: %+v", items)
	}
	if got, want := items[0].Score, 8.0; got != want {
		t.Errorf("slow-search score = %v, want %v", got, want)
	}
	if got, want := items[1].Score, 1.6; got != want {
		t.Errorf("export-filters score = %v, want %v", got, want)
	}
	if items[0].Count != 2 || len(items[0].Evidence) != 2 {
		t.Errorf("slow-search count/evidence wrong: %+v", items[0])
	}
	if items[0].Status != "proposed" {
		t.Errorf("new item should start proposed, got %q", items[0].Status)
	}

	// every consumed feedback id is marked triaged
	all, err := mem.ListFeedback(context.Background(), store.FeedbackFilter{})
	if err != nil {
		t.Fatalf("ListFeedback: %v", err)
	}
	for _, f := range all {
		if f.Status != "triaged" {
			t.Errorf("feedback %d status = %q, want triaged", f.ID, f.Status)
		}
	}
}

func TestRegenerate_FallsBackWhenLLMErrors(t *testing.T) {
	setTempBacklogPath(t)
	mem := store.NewMemory()
	seedFeedback(t, mem, []model.Feedback{
		newFeedback("bug", "checkout", "checkout button does nothing", 5),
		newFeedback("bug", "checkout", "checkout is broken", 5),
		newFeedback("praise", "", "love the new dashboard", 1),
	})

	fake := &fakeLLM{model: "fake-model", jsonFn: func(llm.Request, any) error {
		return fmt.Errorf("connection refused")
	}}
	deps := app.Deps{Store: mem, LLM: fake, LLMModel: "fake-model"}

	items, err := Regenerate(context.Background(), deps)
	if err != nil {
		t.Fatalf("Regenerate should degrade, not fail: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected mechanical fallback to produce backlog items")
	}
	found := false
	for _, it := range items {
		if it.Theme == "checkout" {
			found = true
			if it.Count != 2 {
				t.Errorf("checkout group count = %d, want 2", it.Count)
			}
		}
	}
	if !found {
		t.Errorf("expected a mechanical 'checkout' group, got %+v", items)
	}
}

func TestRegenerate_FallsBackWhenLLMReturnsGarbageIDs(t *testing.T) {
	setTempBacklogPath(t)
	mem := store.NewMemory()
	seedFeedback(t, mem, []model.Feedback{
		newFeedback("confusion", "onboarding", "not sure what to do first", 3),
		newFeedback("confusion", "onboarding", "onboarding is confusing", 3),
	})

	// Valid JSON, but every feedback id is hallucinated (doesn't exist),
	// so every cluster must be dropped and the whole chunk treated as garbage.
	fake := &fakeLLM{model: "fake-model", jsonFn: jsonFnReturning([]llmCluster{
		{Title: "Ghost", Theme: "ghost", Kind: "bug", FeedbackIDs: []int64{9001, 9002}, Effort: "S"},
	})}
	deps := app.Deps{Store: mem, LLM: fake, LLMModel: "fake-model"}

	items, err := Regenerate(context.Background(), deps)
	if err != nil {
		t.Fatalf("Regenerate should degrade, not fail: %v", err)
	}
	for _, it := range items {
		if it.Theme == "ghost" {
			t.Errorf("garbage LLM cluster should have been discarded, got %+v", it)
		}
	}
	if len(items) != 1 || items[0].Theme != "onboarding" {
		t.Fatalf("expected mechanical fallback grouped by theme, got %+v", items)
	}
}

func TestRegenerate_RankingOrder(t *testing.T) {
	setTempBacklogPath(t)
	mem := store.NewMemory()
	// No LLM configured at all -> straight to mechanical grouping.
	seedFeedback(t, mem, []model.Feedback{
		newFeedback("bug", "a", "a1", 5),
		newFeedback("bug", "a", "a2", 5),
		newFeedback("bug", "a", "a3", 5),
		newFeedback("request", "b", "b1", 1),
	})
	deps := app.Deps{Store: mem, LLM: nil, LLMModel: "none"}

	items, err := Regenerate(context.Background(), deps)
	if err != nil {
		t.Fatalf("Regenerate: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 items, got %d: %+v", len(items), items)
	}
	if items[0].Theme != "a" {
		t.Fatalf("theme 'a' (score 3*5/1.5=10) should rank above 'b' (1*1/1.5=0.67), got order %+v", items)
	}
	for i := 1; i < len(items); i++ {
		if items[i-1].Score < items[i].Score {
			t.Errorf("backlog not sorted by score descending: %+v", items)
		}
	}
}

func TestRegenerate_WritesBacklogDoc(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "backlog.md")
	t.Setenv("BACKLOG_PATH", path)

	mem := store.NewMemory()
	fb := seedFeedback(t, mem, []model.Feedback{
		newFeedback("bug", "", "the export button is broken", 4),
		newFeedback("bug", "", "export button does nothing", 4),
	})
	fake := &fakeLLM{model: "fake-model", jsonFn: jsonFnReturning([]llmCluster{
		{Title: "Fix export button", Rationale: "It's broken.", Theme: "export-bug", Kind: "bug",
			FeedbackIDs: []int64{fb[0].ID, fb[1].ID}, Effort: "S"},
	})}
	deps := app.Deps{Store: mem, LLM: fake, LLMModel: "fake-model"}

	if _, err := Regenerate(context.Background(), deps); err != nil {
		t.Fatalf("Regenerate: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
	doc := string(raw)
	for _, want := range []string{
		"# Backlog",
		"Fix export button",
		"Score:",
		"Effort: S",
		fmt.Sprintf("Feedback ids: %d, %d", fb[0].ID, fb[1].ID),
		"How this was made",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("docs/backlog.md missing %q; got:\n%s", want, doc)
		}
	}
}

func TestRegenerate_PreservesAcceptedStatus(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BACKLOG_PATH", filepath.Join(dir, "backlog.md"))

	mem := store.NewMemory()
	fb := seedFeedback(t, mem, []model.Feedback{
		newFeedback("bug", "login", "can't log in on mobile", 5),
		newFeedback("bug", "login", "login fails on mobile safari", 5),
	})
	deps := app.Deps{Store: mem, LLM: nil, LLMModel: "none"}

	items, err := Regenerate(context.Background(), deps)
	if err != nil {
		t.Fatalf("Regenerate #1: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %+v", items)
	}
	id := items[0].ID
	if err := mem.SetBacklogStatus(context.Background(), id, "accepted"); err != nil {
		t.Fatalf("SetBacklogStatus: %v", err)
	}

	// New feedback arrives on the same theme; regenerate again.
	seedFeedback(t, mem, []model.Feedback{newFeedback("bug", "login", "still broken on mobile", 5)})
	_ = fb
	items2, err := Regenerate(context.Background(), deps)
	if err != nil {
		t.Fatalf("Regenerate #2: %v", err)
	}
	if len(items2) != 1 {
		t.Fatalf("want 1 item after second regen, got %+v", items2)
	}
	if items2[0].ID != id {
		t.Errorf("accepted item should keep its id across regeneration: got %d, want %d", items2[0].ID, id)
	}
	if items2[0].Status != "accepted" {
		t.Errorf("accepted item lost its status: got %q", items2[0].Status)
	}
	if items2[0].Count != 3 {
		t.Errorf("accepted item should still absorb new feedback, count = %d, want 3", items2[0].Count)
	}
}

func TestRegenerate_NoLLMConfigured(t *testing.T) {
	setTempBacklogPath(t)
	mem := store.NewMemory()
	seedFeedback(t, mem, []model.Feedback{newFeedback("bug", "x", "x", 3)})
	deps := app.Deps{Store: mem, LLM: nil, LLMModel: "none"}
	if _, err := Regenerate(context.Background(), deps); err != nil {
		t.Fatalf("Regenerate without an LLM client should degrade cleanly: %v", err)
	}
}

// TestLiveCluster exercises the real fleet model. It is skipped under -short
// and requires the cluster at llm.DefaultBaseURL to be reachable.
func TestLiveCluster(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live cluster call in -short mode")
	}
	client := llm.New(llm.Config{})

	dir := t.TempDir()
	t.Setenv("BACKLOG_PATH", filepath.Join(dir, "backlog.md"))

	mem := store.NewMemory()
	seedFeedback(t, mem, []model.Feedback{
		newFeedback("bug", "", "the offer list never refreshes after I create one", 4),
		newFeedback("bug", "", "creating an offer doesn't show up until I reload", 4),
		newFeedback("confusion", "", "I can't tell which offers need my attention", 3),
		newFeedback("confusion", "", "not obvious what 'Attention' status means", 3),
		newFeedback("request", "", "let me export the pipeline to CSV", 2),
		newFeedback("praise", "", "the dashboard looks great on mobile", 1),
	})
	deps := app.Deps{Store: mem, LLM: client, LLMModel: client.Model()}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	items, err := Regenerate(ctx, deps)
	if err != nil {
		t.Fatalf("Regenerate against live model: %v", err)
	}
	t.Logf("live clustering produced %d backlog items", len(items))
	for _, it := range items {
		t.Logf("  %-20s score=%.2f count=%d %q", it.Theme, it.Score, it.Count, it.Title)
	}
}
