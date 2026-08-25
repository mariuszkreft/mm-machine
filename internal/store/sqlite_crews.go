package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"mm-machine/internal/model"
)

// Crews live in their own table. TextSearch (FTS5-backed) lives in
// textsearch.go; this file only holds crew CRUD.
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
