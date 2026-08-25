package store

import "database/sql"

// Full-text search is FTS5 external-content tables over offers and crews:
// the FTS shadow tables store only the tokenized index, keyed on the base
// table's implicit rowid, while the human-readable columns stay in offers/
// crews. Triggers keep the index in sync with every insert/update/delete on
// the base tables, so no write path (including future ones) has to remember
// to touch the index explicitly.
//
// tokenize='unicode61 remove_diacritics 2' folds diacritics (München ->
// munchen) so plain-ASCII queries find accented rows and vice versa; German
// compound words and inflected forms are handled on top of that in
// textsearch.go, since FTS5 has no built-in decompounder.
const offersFTSSchema = `CREATE VIRTUAL TABLE IF NOT EXISTS offers_fts USING fts5(
    title, location, category, supplier, trade, region, attention, requirements,
    content='offers', content_rowid='rowid',
    tokenize='unicode61 remove_diacritics 2'
)`

const offersFTSTriggerInsert = `CREATE TRIGGER IF NOT EXISTS offers_fts_ai AFTER INSERT ON offers BEGIN
    INSERT INTO offers_fts(rowid, title, location, category, supplier, trade, region, attention, requirements)
    VALUES (new.rowid, new.title, new.location, new.category, new.supplier, new.trade, new.region, new.attention, new.requirements);
END`

const offersFTSTriggerDelete = `CREATE TRIGGER IF NOT EXISTS offers_fts_ad AFTER DELETE ON offers BEGIN
    INSERT INTO offers_fts(offers_fts, rowid, title, location, category, supplier, trade, region, attention, requirements)
    VALUES ('delete', old.rowid, old.title, old.location, old.category, old.supplier, old.trade, old.region, old.attention, old.requirements);
END`

const offersFTSTriggerUpdate = `CREATE TRIGGER IF NOT EXISTS offers_fts_au AFTER UPDATE ON offers BEGIN
    INSERT INTO offers_fts(offers_fts, rowid, title, location, category, supplier, trade, region, attention, requirements)
    VALUES ('delete', old.rowid, old.title, old.location, old.category, old.supplier, old.trade, old.region, old.attention, old.requirements);
    INSERT INTO offers_fts(rowid, title, location, category, supplier, trade, region, attention, requirements)
    VALUES (new.rowid, new.title, new.location, new.category, new.supplier, new.trade, new.region, new.attention, new.requirements);
END`

const crewsFTSSchema = `CREATE VIRTUAL TABLE IF NOT EXISTS crews_fts USING fts5(
    name, company, trades, regions, note, documents,
    content='crews', content_rowid='rowid',
    tokenize='unicode61 remove_diacritics 2'
)`

const crewsFTSTriggerInsert = `CREATE TRIGGER IF NOT EXISTS crews_fts_ai AFTER INSERT ON crews BEGIN
    INSERT INTO crews_fts(rowid, name, company, trades, regions, note, documents)
    VALUES (new.rowid, new.name, new.company, new.trades, new.regions, new.note, new.documents);
END`

const crewsFTSTriggerDelete = `CREATE TRIGGER IF NOT EXISTS crews_fts_ad AFTER DELETE ON crews BEGIN
    INSERT INTO crews_fts(crews_fts, rowid, name, company, trades, regions, note, documents)
    VALUES ('delete', old.rowid, old.name, old.company, old.trades, old.regions, old.note, old.documents);
END`

const crewsFTSTriggerUpdate = `CREATE TRIGGER IF NOT EXISTS crews_fts_au AFTER UPDATE ON crews BEGIN
    INSERT INTO crews_fts(crews_fts, rowid, name, company, trades, regions, note, documents)
    VALUES ('delete', old.rowid, old.name, old.company, old.trades, old.regions, old.note, old.documents);
    INSERT INTO crews_fts(rowid, name, company, trades, regions, note, documents)
    VALUES (new.rowid, new.name, new.company, new.trades, new.regions, new.note, new.documents);
END`

// offersFTSWeights and crewsFTSWeights are bm25() column weights: the title
// (offers) and name (crews) column counts far more than a passing mention
// elsewhere, so a title hit outranks the same term buried in a note.
const offersFTSWeights = `5.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0`
const crewsFTSWeights = `5.0, 1.0, 1.0, 1.0, 1.0, 1.0`

// buildFullTextIndex is numbered migration 5. It creates the crews table if
// an older database predates it (crews was previously created lazily by
// ensureCrewTables on first use), builds both FTS5 tables and their sync
// triggers, then runs FTS5's 'rebuild' command to backfill the index from
// whatever rows already exist. On a fresh database all of this is a cheap
// no-op over zero rows; on a production database it indexes everything that
// was written before the index existed.
func buildFullTextIndex(tx *sql.Tx) error {
	stmts := []string{
		crewSchema,
		offersFTSSchema, offersFTSTriggerInsert, offersFTSTriggerDelete, offersFTSTriggerUpdate,
		crewsFTSSchema, crewsFTSTriggerInsert, crewsFTSTriggerDelete, crewsFTSTriggerUpdate,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT INTO offers_fts(offers_fts) VALUES('rebuild')`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO crews_fts(crews_fts) VALUES('rebuild')`); err != nil {
		return err
	}
	return nil
}
