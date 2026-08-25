package store

import (
	"context"
	"sort"
	"strings"
	"time"

	"mm-machine/internal/model"
)

// MatchesCrew applies the crew facets, mirroring MatchesFacets for offers so
// both backends answer a crew query identically.
func MatchesCrew(c model.Crew, f CrewFilter) bool {
	if len(f.Trades) > 0 && !overlapsFold(c.Trades, f.Trades) {
		return false
	}
	if len(f.Regions) > 0 && !anyRegionFold(c.Regions, f.Regions) {
		return false
	}
	if len(f.Documents) > 0 && !containsAllFold(c.Documents, f.Documents) {
		return false
	}
	if f.MinSize > 0 && c.Size < f.MinSize {
		return false
	}
	if f.Query != "" {
		hay := strings.ToLower(c.Name + " " + c.Company + " " + strings.Join(c.Trades, " ") + " " + strings.Join(c.Regions, " ") + " " + c.Note)
		if !strings.Contains(hay, strings.ToLower(f.Query)) {
			return false
		}
	}
	return true
}

func overlapsFold(have, want []string) bool {
	for _, w := range want {
		for _, h := range have {
			if strings.EqualFold(strings.TrimSpace(h), strings.TrimSpace(w)) {
				return true
			}
		}
	}
	return false
}

// anyRegionFold matches loosely in both directions, so "Munich" finds
// "Munich, DE" and a crew covering "DACH" is found by "Germany" upstream.
func anyRegionFold(have, want []string) bool {
	for _, h := range have {
		if anyContainsFold(want, h) {
			return true
		}
	}
	return false
}

// --- Memory implementation ---------------------------------------------------

func (m *Memory) ListCrews(_ context.Context, f CrewFilter) ([]model.Crew, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []model.Crew{}
	for _, c := range m.crews {
		if MatchesCrew(c, f) {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rating > out[j].Rating })
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (m *Memory) GetCrew(_ context.Context, id string) (model.Crew, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, c := range m.crews {
		if c.ID == id {
			return c, nil
		}
	}
	return model.Crew{}, ErrNotFound
}

func (m *Memory) UpsertCrew(_ context.Context, c model.Crew) (model.Crew, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c.UpdatedAt = time.Now()
	for i, cur := range m.crews {
		if cur.ID == c.ID {
			m.crews[i] = c
			return c, nil
		}
	}
	m.crews = append(m.crews, c)
	return c, nil
}

// TextSearch on the memory store is a substring/prefix scan approximating
// the SQLite store's FTS5 index (see textsearch.go): both share expandTerms,
// scoreOffer and scoreCrew so the two backends rank the obvious matches the
// same way.
func (m *Memory) TextSearch(_ context.Context, q TextQuery) ([]TextHit, error) {
	terms, extra := expandTerms(q.Text)
	if len(terms) == 0 && len(extra) == 0 {
		return []TextHit{}, nil
	}
	wantKind := wantTextKind(q.Kinds)

	m.mu.RLock()
	defer m.mu.RUnlock()
	hits := []TextHit{}
	if wantKind("offer") {
		for _, o := range m.offers {
			if score, title := scoreOffer(o, terms, extra); score > 0 {
				hits = append(hits, TextHit{Kind: "offer", ID: o.ID, Score: score, Snippet: title})
			}
		}
	}
	if wantKind("crew") {
		for _, c := range m.crews {
			if score, name := scoreCrew(c, terms, extra); score > 0 {
				hits = append(hits, TextHit{Kind: "crew", ID: c.ID, Score: score, Snippet: name})
			}
		}
	}
	sortHits(hits)
	if q.Limit > 0 && len(hits) > q.Limit {
		hits = hits[:q.Limit]
	}
	return hits, nil
}

// sortHits orders by descending score, stably.
func sortHits(hits []TextHit) {
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
}

// scoreDoc counts matched terms, with a prefix match worth less than a whole
// word — enough to order results sensibly without pretending to be BM25.
func scoreDoc(doc string, terms []string) float64 {
	score := 0.0
	for _, t := range terms {
		if len(t) < 2 {
			continue
		}
		switch {
		case strings.Contains(doc, " "+t+" ") || strings.HasPrefix(doc, t+" "):
			score += 2
		case strings.Contains(doc, t):
			score += 1
		}
	}
	return score
}
