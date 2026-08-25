package devloop

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"mm-machine/internal/app"
	"mm-machine/internal/llm"
	"mm-machine/internal/model"
	"mm-machine/internal/store"
)

// defaultChunkSize caps how much raw feedback goes into one clustering
// request, so a large feedback pool doesn't blow past the model's context or
// its output token budget.
const defaultChunkSize = 20

// chunkTimeout bounds a single clustering call so one slow chunk can't hang
// the whole regeneration.
const chunkTimeout = 60 * time.Second

// clusterResult is one clustered theme with enough detail to both persist a
// model.BacklogItem and render the docs/backlog.md export (which carries a
// couple of fields — effort and the source feedback ids — that the persisted
// BacklogItem does not).
type clusterResult struct {
	Theme     string
	Title     string
	Rationale string
	Kind      string
	Effort    string // S | M | L
	Feedback  []model.Feedback
}

func (c clusterResult) ids() []int64 {
	out := make([]int64, len(c.Feedback))
	for i, f := range c.Feedback {
		out[i] = f.ID
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// scoreOf ranks a cluster: frequency (how many feedback pieces) times average
// severity, discounted by the estimated effort to address it. S is not
// discounted, M is discounted by a third, L by more than half — cheap, common,
// severe problems should always rank above rare, expensive ones.
func scoreOf(count int, avgSeverity float64, effort string) float64 {
	return float64(count) * avgSeverity / effortDiscount(effort)
}

func effortDiscount(effort string) float64 {
	switch strings.ToUpper(strings.TrimSpace(effort)) {
	case "S":
		return 1.0
	case "L":
		return 2.5
	default: // "M" or unknown
		return 1.5
	}
}

func effortRank(effort string) int {
	switch strings.ToUpper(strings.TrimSpace(effort)) {
	case "S":
		return 1
	case "L":
		return 3
	default:
		return 2
	}
}

func normalizeEffort(effort string) string {
	switch strings.ToUpper(strings.TrimSpace(effort)) {
	case "S", "M", "L":
		return strings.ToUpper(strings.TrimSpace(effort))
	default:
		return "M"
	}
}

func normalizeTheme(theme, kind string) string {
	key := strings.ToLower(strings.TrimSpace(theme))
	if key == "" {
		key = strings.ToLower(strings.TrimSpace(kind))
	}
	if key == "" {
		key = "unsorted"
	}
	return key
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func severityOr(sev, fallback int) int {
	if sev <= 0 {
		return fallback
	}
	return sev
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// Regenerate clusters all feedback into ranked backlog items and stores them.
// It prefers the local LLM (see clusterWithLLM) and falls back to a
// mechanical grouping by theme when the model is unreachable or its answer
// can't be trusted, so /dev never breaks because the cluster is down.
//
// Every backlog item stays traceable to the feedback it came from (the
// consumed feedback is marked "triaged"), accepted/shipped/rejected items
// keep their status across regenerations (matched by theme), and the run is
// exported to docs/backlog.md for the next development iteration.
func Regenerate(ctx context.Context, deps app.Deps) ([]model.BacklogItem, error) {
	fb, err := deps.Store.ListFeedback(ctx, store.FeedbackFilter{Limit: 500})
	if err != nil {
		return nil, err
	}
	prior, err := deps.Store.ListBacklog(ctx)
	if err != nil {
		return nil, err
	}

	clusters, source := cluster(ctx, deps, fb)
	entries := buildEntries(clusters, prior)

	items := make([]model.BacklogItem, len(entries))
	for i, e := range entries {
		items[i] = e.Item
	}
	if err := deps.Store.ReplaceBacklog(ctx, items); err != nil {
		return nil, err
	}

	triaged := map[int64]bool{}
	for _, e := range entries {
		for _, f := range e.cluster.Feedback {
			if triaged[f.ID] || f.Status != "" && f.Status != "new" {
				continue
			}
			triaged[f.ID] = true
			if err := deps.Store.SetFeedbackStatus(ctx, f.ID, "triaged"); err != nil {
				log.Printf("devloop: mark feedback %d triaged: %v", f.ID, err)
			}
		}
	}

	if err := writeBacklogDoc(deps, entries, source); err != nil {
		log.Printf("devloop: write %s: %v", backlogPath(), err)
	}

	return items, nil
}

// cluster returns the ranked list of themes plus a human-readable note about
// how they were produced, for docs/backlog.md.
func cluster(ctx context.Context, deps app.Deps, fb []model.Feedback) ([]clusterResult, string) {
	if len(fb) == 0 {
		return nil, "no feedback collected yet"
	}
	if deps.LLM != nil {
		results, err := clusterWithLLM(ctx, deps, fb)
		if err == nil {
			return results, fmt.Sprintf("LLM clustering via %s", deps.LLMModel)
		}
		log.Printf("devloop: LLM clustering unavailable, falling back to mechanical grouping: %v", err)
		return clusterMechanical(fb), fmt.Sprintf("mechanical grouping (LLM unavailable: %v)", err)
	}
	return clusterMechanical(fb), "mechanical grouping (no LLM configured)"
}

// clusterMechanical groups feedback by theme (falling back to kind, then
// "unsorted"). It never fails, so it is always a safe fallback.
func clusterMechanical(fb []model.Feedback) []clusterResult {
	groups := map[string][]model.Feedback{}
	var order []string
	for _, f := range fb {
		key := normalizeTheme(f.Theme, f.Kind)
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], f)
	}
	sort.Strings(order)
	results := make([]clusterResult, 0, len(groups))
	for _, key := range order {
		group := groups[key]
		results = append(results, clusterResult{
			Theme:     key,
			Title:     titleCase(key),
			Rationale: fmt.Sprintf("Grouped mechanically from %d piece(s) of user feedback (LLM clustering unavailable).", len(group)),
			Kind:      group[0].Kind,
			Effort:    "M",
			Feedback:  group,
		})
	}
	return results
}

// --- LLM clustering ---------------------------------------------------------

const clusterSystemPrompt = `You triage raw user feedback about a software product into backlog themes.
Group near-duplicate or closely related feedback into a shared theme; do not
create a separate theme for every single item. Every feedback_ids value you
return MUST be one of the ids given to you — never invent an id. kind must be
one of: bug, confusion, request, praise. effort estimates the engineering size
to address the theme: S (small, quick fix), M (medium), L (large, multi-step).
Answer with a JSON array only.`

const clusterSchemaHint = `[{"title":"short backlog title","rationale":"why this matters, 1-2 sentences","theme":"short-slug","kind":"bug|confusion|request|praise","feedback_ids":[1,2,3],"severity":1,"effort":"S|M|L"}]`

type llmCluster struct {
	Title       string  `json:"title"`
	Rationale   string  `json:"rationale"`
	Theme       string  `json:"theme"`
	Kind        string  `json:"kind"`
	FeedbackIDs []int64 `json:"feedback_ids"`
	Severity    float64 `json:"severity"`
	Effort      string  `json:"effort"`
}

type feedbackLine struct {
	ID       int64  `json:"id"`
	Kind     string `json:"kind"`
	Theme    string `json:"theme,omitempty"`
	Severity int    `json:"severity"`
	Route    string `json:"route,omitempty"`
	Verbatim string `json:"verbatim"`
}

// clusterWithLLM chunks fb, asks the model to cluster each chunk, and merges
// same-theme clusters across chunks. Any failure — a request error, an
// unparseable answer, or an answer with zero usable clusters — aborts the
// whole attempt so the caller falls back to the mechanical grouping; a
// partial LLM result would be worse than a plain one, not better.
func clusterWithLLM(ctx context.Context, deps app.Deps, fb []model.Feedback) ([]clusterResult, error) {
	var raw []clusterResult
	for start := 0; start < len(fb); start += defaultChunkSize {
		end := min(start+defaultChunkSize, len(fb))
		chunk := fb[start:end]
		results, err := clusterChunk(ctx, deps, chunk)
		if err != nil {
			return nil, err
		}
		raw = append(raw, results...)
	}
	merged := mergeClusters(raw)
	if len(merged) == 0 {
		return nil, fmt.Errorf("no usable clusters in model output")
	}
	return merged, nil
}

func clusterChunk(ctx context.Context, deps app.Deps, chunk []model.Feedback) ([]clusterResult, error) {
	byID := make(map[int64]model.Feedback, len(chunk))
	lines := make([]feedbackLine, 0, len(chunk))
	for _, f := range chunk {
		byID[f.ID] = f
		lines = append(lines, feedbackLine{
			ID:       f.ID,
			Kind:     f.Kind,
			Theme:    f.Theme,
			Severity: severityOr(f.Severity, 3),
			Route:    f.Route,
			Verbatim: truncate(f.Verbatim, 300),
		})
	}
	payload, err := json.Marshal(lines)
	if err != nil {
		return nil, err
	}

	chunkCtx, cancel := context.WithTimeout(ctx, chunkTimeout)
	defer cancel()

	var out []llmCluster
	req := llm.Request{
		Messages: []llm.Message{
			{Role: "system", Content: clusterSystemPrompt},
			{Role: "user", Content: "Feedback items:\n" + string(payload)},
		},
		MaxTokens:   1536,
		Temperature: 0.2,
	}
	if err := deps.LLM.JSON(chunkCtx, req, clusterSchemaHint, &out); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty cluster answer")
	}

	results := make([]clusterResult, 0, len(out))
	for _, rc := range out {
		seen := map[int64]bool{}
		var included []model.Feedback
		for _, id := range rc.FeedbackIDs {
			if f, ok := byID[id]; ok && !seen[id] {
				included = append(included, f)
				seen[id] = true
			}
		}
		if len(included) == 0 {
			continue // hallucinated/garbage cluster: drop it, not the whole chunk
		}
		theme := normalizeTheme(rc.Theme, rc.Kind)
		kind := strings.TrimSpace(rc.Kind)
		if kind == "" {
			kind = included[0].Kind
		}
		title := strings.TrimSpace(rc.Title)
		if title == "" {
			title = titleCase(theme)
		}
		results = append(results, clusterResult{
			Theme:     theme,
			Title:     title,
			Rationale: strings.TrimSpace(rc.Rationale),
			Kind:      kind,
			Effort:    normalizeEffort(rc.Effort),
			Feedback:  included,
		})
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("model returned no valid clusters (all feedback_ids unrecognised)")
	}
	return results, nil
}

// mergeClusters combines same-theme clusters produced by different chunks.
func mergeClusters(raw []clusterResult) []clusterResult {
	byTheme := map[string]*clusterResult{}
	var order []string
	for _, c := range raw {
		existing, ok := byTheme[c.Theme]
		if !ok {
			cp := c
			byTheme[c.Theme] = &cp
			order = append(order, c.Theme)
			continue
		}
		seen := map[int64]bool{}
		for _, f := range existing.Feedback {
			seen[f.ID] = true
		}
		for _, f := range c.Feedback {
			if !seen[f.ID] {
				existing.Feedback = append(existing.Feedback, f)
				seen[f.ID] = true
			}
		}
		if existing.Rationale == "" {
			existing.Rationale = c.Rationale
		}
		if effortRank(c.Effort) > effortRank(existing.Effort) {
			existing.Effort = c.Effort
		}
	}
	out := make([]clusterResult, 0, len(order))
	for _, key := range order {
		out = append(out, *byTheme[key])
	}
	return out
}

// --- assembling backlog items ----------------------------------------------

type backlogEntry struct {
	Item    model.BacklogItem
	cluster clusterResult
}

// buildEntries turns clustered themes into ranked, storable backlog items.
// An item whose theme matches a prior item that was accepted, shipped or
// rejected keeps that item's id and status, so human decisions survive
// regeneration; only "proposed" items are free to be replaced wholesale.
func buildEntries(clusters []clusterResult, prior []model.BacklogItem) []backlogEntry {
	priorByTheme := map[string]model.BacklogItem{}
	for _, p := range prior {
		if p.Status != "" && p.Status != "proposed" {
			priorByTheme[normalizeTheme(p.Theme, p.Kind)] = p
		}
	}

	entries := make([]backlogEntry, 0, len(clusters))
	for _, c := range clusters {
		count := len(c.Feedback)
		sum := 0
		evidence := make([]string, 0, min(count, 5))
		for i, f := range c.Feedback {
			sum += severityOr(f.Severity, 3)
			if i < 5 {
				evidence = append(evidence, f.Verbatim)
			}
		}
		avg := 0.0
		if count > 0 {
			avg = float64(sum) / float64(count)
		}
		item := model.BacklogItem{
			Title:       c.Title,
			Rationale:   c.Rationale,
			Theme:       c.Theme,
			Kind:        c.Kind,
			Count:       count,
			AvgSeverity: avg,
			Score:       scoreOf(count, avg, c.Effort),
			Evidence:    evidence,
			Status:      "proposed",
		}
		if p, ok := priorByTheme[c.Theme]; ok {
			item.ID = p.ID
			item.Status = p.Status
		}
		entries = append(entries, backlogEntry{Item: item, cluster: c})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Item.Score != entries[j].Item.Score {
			return entries[i].Item.Score > entries[j].Item.Score
		}
		return entries[i].Item.Theme < entries[j].Item.Theme
	})
	return entries
}

// --- docs/backlog.md export -------------------------------------------------

// writeBacklogDoc exports the ranked backlog to BACKLOG_PATH (default
// docs/backlog.md) for the next development iteration to read. A write
// failure degrades to a logged warning — it must never turn a regeneration
// into a 500.
func writeBacklogDoc(deps app.Deps, entries []backlogEntry, source string) error {
	path := backlogPath()
	var b strings.Builder
	b.WriteString("# Backlog\n\n")
	fmt.Fprintf(&b, "- Generated: %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "- Model: %s\n\n", deps.LLMModel)

	if len(entries) == 0 {
		b.WriteString("No backlog items yet. Collect feedback, then regenerate.\n\n")
	}
	for i, e := range entries {
		it := e.Item
		fmt.Fprintf(&b, "## %d. %s\n\n", i+1, it.Title)
		fmt.Fprintf(&b, "- Score: %.2f\n", it.Score)
		fmt.Fprintf(&b, "- Count: %d\n", it.Count)
		fmt.Fprintf(&b, "- Severity: %.1f\n", it.AvgSeverity)
		fmt.Fprintf(&b, "- Effort: %s\n", e.cluster.Effort)
		fmt.Fprintf(&b, "- Kind: %s\n", it.Kind)
		fmt.Fprintf(&b, "- Status: %s\n", it.Status)
		fmt.Fprintf(&b, "- Feedback ids: %s\n\n", joinIDs(e.cluster.ids()))
		if it.Rationale != "" {
			fmt.Fprintf(&b, "%s\n\n", it.Rationale)
		}
		if len(it.Evidence) > 0 {
			b.WriteString("Evidence:\n")
			for _, ev := range it.Evidence {
				fmt.Fprintf(&b, "> %s\n", strings.ReplaceAll(ev, "\n", " "))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "How this was made: %s. Score = feedback count x average severity / effort discount "+
		"(S=1, M=1.5, L=2.5), ranked highest first, ties broken alphabetically by theme.\n", source)

	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func joinIDs(ids []int64) string {
	if len(ids) == 0 {
		return "(none)"
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("%d", id)
	}
	return strings.Join(parts, ", ")
}
