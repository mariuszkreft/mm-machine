package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"mm-machine/internal/model"
)

// Crews live in their own table. The full-text index over crews and offers is
// the store worker's job; until it lands, TextSearch answers out of SQL with
// LIKE and the shared scoring helper, so callers see the same shape either way.
const crewSchema = `
CREATE TABLE IF NOT EXISTS crews (
    id             TEXT PRIMARY KEY,
    name           TEXT NOT NULL DEFAULT '',
    company        TEXT NOT NULL DEFAULT '',
    trades         TEXT NOT NULL DEFAULT '[]',
    regions        TEXT NOT NULL DEFAULT '[]',
    size           INTEGER NOT NULL DEFAULT 0,
    languages      TEXT NOT NULL DEFAULT '[]',
    documents      TEXT NOT NULL DEFAULT '[]',
    available_from INTEGER,
    available_note TEXT NOT NULL DEFAULT '',
    rate           TEXT NOT NULL DEFAULT '',
    rating         REAL NOT NULL DEFAULT 0,
    jobs_done      INTEGER NOT NULL DEFAULT 0,
    note           TEXT NOT NULL DEFAULT '',
    updated_at     INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS crews_size ON crews(size);
`

func (s *SQLite) ensureCrewTables() error {
	if _, err := s.db.Exec(crewSchema); err != nil {
		return fmt.Errorf("store: crew schema: %w", err)
	}
	return nil
}

func scanCrew(row rowScanner) (model.Crew, error) {
	var c model.Crew
	var trades, regions, languages, documents string
	var availableFrom sql.NullInt64
	var updatedAt int64
	err := row.Scan(&c.ID, &c.Name, &c.Company, &trades, &regions, &c.Size, &languages, &documents,
		&availableFrom, &c.AvailableNote, &c.Rate, &c.Rating, &c.JobsDone, &c.Note, &updatedAt)
	if err != nil {
		return model.Crew{}, err
	}
	c.Trades, c.Regions = decodeList(trades), decodeList(regions)
	c.Languages, c.Documents = decodeList(languages), decodeList(documents)
	if availableFrom.Valid {
		c.AvailableFrom = unixToTime(availableFrom.Int64)
	}
	c.UpdatedAt = unixToTime(updatedAt)
	return c, nil
}

const crewColumns = `id, name, company, trades, regions, size, languages, documents,
	available_from, available_note, rate, rating, jobs_done, note, updated_at`

func (s *SQLite) ListCrews(ctx context.Context, f CrewFilter) ([]model.Crew, error) {
	if err := s.ensureCrewTables(); err != nil {
		return nil, err
	}
	query := `SELECT ` + crewColumns + ` FROM crews WHERE 1 = 1`
	args := []any{}
	if f.MinSize > 0 {
		query += ` AND size >= ?`
		args = append(args, f.MinSize)
	}
	query += ` ORDER BY rating DESC, jobs_done DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list crews: %w", err)
	}
	defer rows.Close()
	out := []model.Crew{}
	for rows.Next() {
		c, err := scanCrew(rows)
		if err != nil {
			return nil, err
		}
		// The list facets are JSON columns, so they are applied through the
		// shared helper rather than in SQL.
		if !MatchesCrew(c, f) {
			continue
		}
		out = append(out, c)
		if f.Limit > 0 && len(out) >= f.Limit {
			break
		}
	}
	return out, rows.Err()
}

func (s *SQLite) GetCrew(ctx context.Context, id string) (model.Crew, error) {
	if err := s.ensureCrewTables(); err != nil {
		return model.Crew{}, err
	}
	row := s.db.QueryRowContext(ctx, `SELECT `+crewColumns+` FROM crews WHERE id = ?`, id)
	c, err := scanCrew(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Crew{}, ErrNotFound
	}
	if err != nil {
		return model.Crew{}, fmt.Errorf("store: get crew: %w", err)
	}
	return c, nil
}

func (s *SQLite) UpsertCrew(ctx context.Context, c model.Crew) (model.Crew, error) {
	if err := s.ensureCrewTables(); err != nil {
		return model.Crew{}, err
	}
	c.UpdatedAt = time.Now()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO crews (`+crewColumns+`)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    name=excluded.name, company=excluded.company, trades=excluded.trades,
    regions=excluded.regions, size=excluded.size, languages=excluded.languages,
    documents=excluded.documents, available_from=excluded.available_from,
    available_note=excluded.available_note, rate=excluded.rate, rating=excluded.rating,
    jobs_done=excluded.jobs_done, note=excluded.note, updated_at=excluded.updated_at`,
		c.ID, c.Name, c.Company, encodeList(c.Trades), encodeList(c.Regions), c.Size,
		encodeList(c.Languages), encodeList(c.Documents), nullableUnix(c.AvailableFrom),
		c.AvailableNote, c.Rate, c.Rating, c.JobsDone, c.Note, nonZeroUnix(c.UpdatedAt))
	if err != nil {
		return model.Crew{}, fmt.Errorf("store: upsert crew: %w", err)
	}
	return c, nil
}

// TextSearch scans both corpora with LIKE and scores with the shared helper.
// The store worker replaces the body with an FTS5 query; the contract — hits
// ordered by descending score — must not change.
func (s *SQLite) TextSearch(ctx context.Context, q TextQuery) ([]TextHit, error) {
	terms := strings.Fields(strings.ToLower(q.Text))
	if len(terms) == 0 {
		return []TextHit{}, nil
	}
	if err := s.ensureCrewTables(); err != nil {
		return nil, err
	}
	wantKind := func(kind string) bool {
		if len(q.Kinds) == 0 {
			return true
		}
		for _, k := range q.Kinds {
			if k == kind {
				return true
			}
		}
		return false
	}

	hits := []TextHit{}
	if wantKind("offer") {
		offers, err := s.ListOffers(ctx, OfferFilter{})
		if err != nil {
			return nil, err
		}
		for _, o := range offers {
			doc := strings.ToLower(strings.Join([]string{o.Title, o.Location, o.Category, o.Supplier, o.Trade, o.Region, o.Attention, strings.Join(o.Requirements, " ")}, " "))
			if score := scoreDoc(doc, terms); score > 0 {
				hits = append(hits, TextHit{Kind: "offer", ID: o.ID, Score: score, Snippet: o.Title})
			}
		}
	}
	if wantKind("crew") {
		crews, err := s.ListCrews(ctx, CrewFilter{})
		if err != nil {
			return nil, err
		}
		for _, c := range crews {
			doc := strings.ToLower(strings.Join([]string{c.Name, c.Company, strings.Join(c.Trades, " "), strings.Join(c.Regions, " "), c.Note, strings.Join(c.Documents, " ")}, " "))
			if score := scoreDoc(doc, terms); score > 0 {
				hits = append(hits, TextHit{Kind: "crew", ID: c.ID, Score: score, Snippet: c.Name})
			}
		}
	}
	sortHits(hits)
	if q.Limit > 0 && len(hits) > q.Limit {
		hits = hits[:q.Limit]
	}
	return hits, nil
}
