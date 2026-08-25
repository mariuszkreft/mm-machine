package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"mm-machine/internal/model"
)

// oldSchemaSQL is the offers table shape shipped before offers gained facet
// columns and before profiles/saved_searches were folded into schema.sql —
// i.e. what a production database at /app/data/mm.db looks like today.
const oldSchemaSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY
);

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

// buildOldDatabase hand-creates a database in the pre-facet, pre-profiles
// shape and inserts two rows with raw SQL: one whose id matches a seed
// offer (to prove facet backfill from SeedOffers), one that does not (to
// prove the category/location fallback).
func buildOldDatabase(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(oldSchemaSQL); err != nil {
		t.Fatalf("create old schema: %v", err)
	}

	rows := []struct {
		id, title, location, category, amount, budget, status, signal, supplier, attention string
		progress                                                                           int
		createdAt, updatedAt                                                               int64
	}{
		{id: "MM-1842", title: "Legacy title for a seeded offer", location: "Old Munich Loc", category: "Energy",
			amount: "1", budget: "1k", status: "process", signal: "Attention", supplier: "Voltwerk GmbH",
			attention: "legacy", progress: 68, createdAt: 1000, updatedAt: 2000},
		{id: "OLD-1", title: "Pre-facet custom offer", location: "Legacy City", category: "Legacy",
			amount: "2", budget: "2k", status: "open", signal: "OK", supplier: "Someone",
			attention: "none", progress: 5, createdAt: 1500, updatedAt: 2500},
	}
	for _, r := range rows {
		_, err := db.Exec(`INSERT INTO offers
			(id, title, location, category, amount, budget, status, signal, supplier, progress, attention, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r.id, r.title, r.location, r.category, r.amount, r.budget, r.status, r.signal, r.supplier, r.progress, r.attention, r.createdAt, r.updatedAt)
		if err != nil {
			t.Fatalf("insert legacy offer %s: %v", r.id, err)
		}
	}
}

func TestMigrationUpgradesOldDatabaseWithoutDataLoss(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	buildOldDatabase(t, path)

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()
	s := store.(*SQLite)
	ctx := context.Background()

	// No data loss: both hand-inserted rows survive, and no re-seed
	// duplicated the id that also exists in SeedOffers.
	all, err := s.ListOffers(ctx, OfferFilter{})
	if err != nil {
		t.Fatalf("ListOffers: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected the 2 pre-existing rows to survive with no re-seed, got %d offers", len(all))
	}

	seeded, err := s.GetOffer(ctx, "MM-1842")
	if err != nil {
		t.Fatalf("GetOffer MM-1842: %v", err)
	}
	if seeded.Title != "Legacy title for a seeded offer" {
		t.Fatalf("migration must not overwrite pre-existing non-facet columns, got title %q", seeded.Title)
	}
	var wantSeed model.Offer
	for _, o := range SeedOffers() {
		if o.ID == "MM-1842" {
			wantSeed = o
		}
	}
	if seeded.Trade != wantSeed.Trade || seeded.Region != wantSeed.Region || seeded.CrewSize != wantSeed.CrewSize {
		t.Fatalf("expected facets backfilled from SeedOffers, got %+v want trade=%q region=%q crew=%d",
			seeded, wantSeed.Trade, wantSeed.Region, wantSeed.CrewSize)
	}
	if len(seeded.Requirements) != len(wantSeed.Requirements) {
		t.Fatalf("expected requirements backfilled from SeedOffers, got %+v want %+v", seeded.Requirements, wantSeed.Requirements)
	}

	custom, err := s.GetOffer(ctx, "OLD-1")
	if err != nil {
		t.Fatalf("GetOffer OLD-1: %v", err)
	}
	if custom.Trade != "Legacy" {
		t.Fatalf("expected trade to fall back to category, got %q", custom.Trade)
	}
	if custom.Region != "Legacy City" {
		t.Fatalf("expected region to fall back to location, got %q", custom.Region)
	}
	if custom.CrewSize != 0 {
		t.Fatalf("expected crew size to default to 0, got %d", custom.CrewSize)
	}
	if !custom.Start.IsZero() {
		t.Fatalf("expected Start to stay NULL for a row with no known start, got %v", custom.Start)
	}

	// The new columns exist and are queryable directly (not just through Go).
	var trade, region string
	if err := s.db.QueryRowContext(ctx, `SELECT trade, region FROM offers WHERE id = ?`, "OLD-1").Scan(&trade, &region); err != nil {
		t.Fatalf("expected new columns to exist: %v", err)
	}

	// Profiles/saved_searches got folded into the schema too.
	if _, err := s.UpsertProfile(ctx, model.Profile{ID: "p1", Role: "owner"}); err != nil {
		t.Fatalf("UpsertProfile on migrated db: %v", err)
	}
	if _, err := s.SaveSearch(ctx, model.SavedSearch{ProfileID: "p1", Label: "l", Query: "q"}); err != nil {
		t.Fatalf("SaveSearch on migrated db: %v", err)
	}

	// Reopening must not re-run the migration or duplicate rows.
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()
	after, err := reopened.ListOffers(ctx, OfferFilter{})
	if err != nil {
		t.Fatalf("ListOffers after reopen: %v", err)
	}
	if len(after) != len(all) {
		t.Fatalf("expected reopen to be a no-op, got %d offers, want %d", len(after), len(all))
	}
}
