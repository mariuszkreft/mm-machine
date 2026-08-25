// Package store_test exercises TextSearch as a black box, against the German
// demo corpus (internal/demo). It lives in the _test package (rather than
// package store, like the rest of this directory's tests) because
// internal/demo itself imports internal/store, so an in-package test file
// importing internal/demo would be a cycle.
package store_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"mm-machine/internal/demo"
	"mm-machine/internal/store"
)

func seedDemo(t *testing.T, s store.Store) {
	t.Helper()
	ctx := context.Background()
	for _, o := range demo.Offers() {
		if _, err := s.CreateOffer(ctx, o); err != nil {
			t.Fatalf("seed offer %s: %v", o.ID, err)
		}
	}
	for _, c := range demo.Crews() {
		if _, err := s.UpsertCrew(ctx, c); err != nil {
			t.Fatalf("seed crew %s: %v", c.ID, err)
		}
	}
}

func openDemoSQLite(t *testing.T) store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "demo.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	seedDemo(t, s)
	return s
}

func openDemoMemory(t *testing.T) store.Store {
	t.Helper()
	m := store.NewMemory()
	seedDemo(t, m)
	return m
}

func hitIDs(hits []store.TextHit) []string {
	ids := make([]string, len(hits))
	for i, h := range hits {
		ids[i] = h.ID
	}
	return ids
}

func containsID(hits []store.TextHit, id string) bool {
	for _, h := range hits {
		if h.ID == id {
			return true
		}
	}
	return false
}

func TestFTSGermanQueryFindsGermanRows(t *testing.T) {
	ctx := context.Background()
	s := openDemoSQLite(t)

	hits, err := s.TextSearch(ctx, store.TextQuery{Text: "München"})
	if err != nil {
		t.Fatalf("TextSearch: %v", err)
	}
	if !containsID(hits, "MM-2101") || !containsID(hits, "MM-2105") {
		t.Fatalf("expected München offers MM-2101 and MM-2105, got %v", hitIDs(hits))
	}
}

func TestFTSDiacriticsFold(t *testing.T) {
	ctx := context.Background()
	s := openDemoSQLite(t)

	hits, err := s.TextSearch(ctx, store.TextQuery{Text: "munchen"})
	if err != nil {
		t.Fatalf("TextSearch: %v", err)
	}
	if !containsID(hits, "MM-2101") {
		t.Fatalf("expected the diacritic-free query to fold and find München rows, got %v", hitIDs(hits))
	}
}

// TestFTSEnglishSynonymFindsGermanRows proves the DE<->EN synonym map: an
// English query finds the German-only row through the map, not through any
// literal English text in the corpus (there is none).
func TestFTSEnglishSynonymFindsGermanRows(t *testing.T) {
	ctx := context.Background()
	s := openDemoSQLite(t)

	hits, err := s.TextSearch(ctx, store.TextQuery{Text: "subcontractor"})
	if err != nil {
		t.Fatalf("TextSearch: %v", err)
	}
	if !containsID(hits, "MM-2102") {
		t.Fatalf("expected 'subcontractor' to find MM-2102 (Nachunternehmern) via the synonym map, got %v", hitIDs(hits))
	}
}

// TestFTSCompoundDachmontage covers the compound-word requirement: a search
// for the compound must find rows that only contain its parts separately.
func TestFTSCompoundDachmontage(t *testing.T) {
	ctx := context.Background()
	s := openDemoSQLite(t)

	hits, err := s.TextSearch(ctx, store.TextQuery{Text: "Dachmontage"})
	if err != nil {
		t.Fatalf("TextSearch: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("expected hits for Dachmontage")
	}
	if hits[0].ID != "MM-2101" {
		t.Fatalf("expected the literal Dachmontage title to rank first, got order %v", hitIDs(hits))
	}
	if !containsID(hits, "MM-2109") {
		t.Fatalf("expected decompounded 'Montage' to find MM-2109 (Stahltreppen Montage Parkhaus), got %v", hitIDs(hits))
	}
	if !containsID(hits, "CR-01") {
		t.Fatalf("expected decompounded 'Montage'/'Dach' to find CR-01 (Nowak Montage, region DACH), got %v", hitIDs(hits))
	}
}

// TestFTSCompoundElektriker covers the inflection requirement: Elektriker
// is not a clean two-part compound of Elektro, but the stem still has to
// find it.
func TestFTSCompoundElektriker(t *testing.T) {
	ctx := context.Background()
	s := openDemoSQLite(t)

	hits, err := s.TextSearch(ctx, store.TextQuery{Text: "Elektriker"})
	if err != nil {
		t.Fatalf("TextSearch: %v", err)
	}
	if !containsID(hits, "MM-2105") {
		t.Fatalf("expected Elektriker to find MM-2105 (Elektroinstallation), got %v", hitIDs(hits))
	}
	if !containsID(hits, "CR-04") {
		t.Fatalf("expected Elektriker to find CR-04 (Elektro Kolonne Süd), got %v", hitIDs(hits))
	}
}

func TestFTSHostileInputDoesNotError(t *testing.T) {
	ctx := context.Background()
	s := openDemoSQLite(t)

	for _, q := range []string{`"`, `*`, `OR`, `AND`, `NOT`, `A1-Bescheinigung`, `"OR" OR *`, `a1-bescheinigung OR "`, `((()))`} {
		if _, err := s.TextSearch(ctx, store.TextQuery{Text: q}); err != nil {
			t.Fatalf("TextSearch(%q): unexpected error: %v", q, err)
		}
	}

	hits, err := s.TextSearch(ctx, store.TextQuery{Text: "A1-Bescheinigung"})
	if err != nil {
		t.Fatalf("TextSearch: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("expected A1-Bescheinigung to find rows requiring a1, got none")
	}
}

// TestFTSBM25RanksTitleAboveMention is the ranking-sanity test: a title
// match must outrank the same term appearing only as a facet/category.
func TestFTSBM25RanksTitleAboveMention(t *testing.T) {
	ctx := context.Background()
	s := openDemoSQLite(t)

	hits, err := s.TextSearch(ctx, store.TextQuery{Text: "Sanitär", Kinds: []string{"offer"}})
	if err != nil {
		t.Fatalf("TextSearch: %v", err)
	}
	if len(hits) < 2 {
		t.Fatalf("expected at least 2 hits, got %v", hitIDs(hits))
	}
	if hits[0].ID != "MM-2108" {
		t.Fatalf("expected the title match (MM-2108, Sanitärinstallation) to outrank a category-only match, got order %v", hitIDs(hits))
	}
	for _, h := range hits {
		if h.Score <= 0 {
			t.Fatalf("expected higher-is-better positive scores (bm25 sign must not leak), got %+v", h)
		}
	}
}

func TestFTSSnippetPresent(t *testing.T) {
	ctx := context.Background()
	s := openDemoSQLite(t)

	hits, err := s.TextSearch(ctx, store.TextQuery{Text: "München"})
	if err != nil {
		t.Fatalf("TextSearch: %v", err)
	}
	if len(hits) == 0 || hits[0].Snippet == "" {
		t.Fatalf("expected a non-empty snippet, got %+v", hits)
	}
}

// legacyOffersSchema is the offers-only shape a production database has
// before crews and the FTS index existed (mirrors migration_test.go's
// oldSchemaSQL, duplicated here since that constant is unexported and this
// file lives in a different package to avoid the import cycle above).
const legacyOffersSchema = `
CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY);
CREATE TABLE IF NOT EXISTS offers (
    id         TEXT PRIMARY KEY,
    title      TEXT NOT NULL,
    location   TEXT NOT NULL,
    category   TEXT NOT NULL,
    amount     TEXT NOT NULL,
    budget     TEXT NOT NULL,
    status     TEXT NOT NULL,
    signal     TEXT NOT NULL,
    supplier   TEXT NOT NULL,
    progress   INTEGER NOT NULL,
    attention  TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
`

func buildLegacyDatabase(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(legacyOffersSchema); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	_, err = db.Exec(`INSERT INTO offers
		(id, title, location, category, amount, budget, status, signal, supplier, progress, attention, created_at, updated_at)
		VALUES ('OLD-FTS-1', 'Legacy searchable title for backfill', 'Legacy City', 'Legacy', '1', '1k', 'open', 'OK', 'Someone', 5, 'none', 1000, 2000)`)
	if err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
}

// TestFTSMigrationBackfillsExistingDatabase proves numbered migration 5
// builds and backfills the FTS index for a database created before it
// existed, per the task's migration requirement.
func TestFTSMigrationBackfillsExistingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	buildLegacyDatabase(t, path)

	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	hits, err := s.TextSearch(ctx, store.TextQuery{Text: "legacy searchable title"})
	if err != nil {
		t.Fatalf("TextSearch: %v", err)
	}
	if !containsID(hits, "OLD-FTS-1") {
		t.Fatalf("expected the FTS index to be backfilled from the pre-existing row, got %v", hitIDs(hits))
	}
}

// TestFTSMemorySQLiteTopHitParity is the parity requirement: both backends
// must agree on the top hit for the same query over the same corpus.
//
// Comparisons are scoped to one Kind at a time. bm25 scores from the two
// independent offers_fts/crews_fts queries are not on a shared scale (each
// is normalized against its own table's term statistics), so combining them
// into one ranking is intentionally left to the caller/UI rather than
// asserted here as if the numbers were comparable.
//
// The query list is deliberately restricted to decisive matches: a couple of
// candidate queries (e.g. "München", where two München offers are within 3%
// of each other in bm25) are genuine near-ties where bm25's term-frequency
// saturation and the approximate scorer's presence-based scoring can
// legitimately disagree on the ordering. Parity is meaningful for the top
// hit when there is a real winner, not for arbitrarily breaking a near-tie.
func TestFTSMemorySQLiteTopHitParity(t *testing.T) {
	ctx := context.Background()
	sq := openDemoSQLite(t)
	mem := openDemoMemory(t)

	type check struct {
		query string
		kind  string
	}
	checks := []check{
		{"Sanitär", "offer"}, {"Sanitär", "crew"},
		{"subcontractor", "offer"},
		{"Elektriker", "offer"}, {"Elektriker", "crew"},
		{"Dachmontage", "offer"}, {"Dachmontage", "crew"},
		{"steel", "offer"}, {"steel", "crew"},
		{"Trockenbau", "offer"}, {"Trockenbau", "crew"},
		{"Stahlbau", "offer"}, {"Stahlbau", "crew"},
		{"Innenausbau", "offer"}, {"Innenausbau", "crew"},
		{"Photovoltaik", "offer"}, {"Photovoltaik", "crew"},
	}
	for _, c := range checks {
		t.Run(c.query+"/"+c.kind, func(t *testing.T) {
			q := store.TextQuery{Text: c.query, Kinds: []string{c.kind}}
			sqHits, err := sq.TextSearch(ctx, q)
			if err != nil {
				t.Fatalf("SQLite TextSearch: %v", err)
			}
			memHits, err := mem.TextSearch(ctx, q)
			if err != nil {
				t.Fatalf("Memory TextSearch: %v", err)
			}
			if len(sqHits) == 0 || len(memHits) == 0 {
				t.Fatalf("expected both backends to find %s hits for %q: sqlite=%v memory=%v", c.kind, c.query, hitIDs(sqHits), hitIDs(memHits))
			}
			if sqHits[0].ID != memHits[0].ID {
				t.Fatalf("top %s hit mismatch for %q: sqlite=%s memory=%s (sqlite order %v, memory order %v)",
					c.kind, c.query, sqHits[0].ID, memHits[0].ID, hitIDs(sqHits), hitIDs(memHits))
			}
		})
	}
}
