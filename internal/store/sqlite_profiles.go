package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"mm-machine/internal/model"
)

// Profiles and saved searches live in their own tables, created lazily so an
// existing database picks them up on the next boot.
const profileSchema = `
CREATE TABLE IF NOT EXISTS profiles (
    id           TEXT PRIMARY KEY,
    role         TEXT NOT NULL DEFAULT '',
    company      TEXT NOT NULL DEFAULT '',
    contact      TEXT NOT NULL DEFAULT '',
    trades       TEXT NOT NULL DEFAULT '[]',
    regions      TEXT NOT NULL DEFAULT '[]',
    crew_size    INTEGER NOT NULL DEFAULT 0,
    languages    TEXT NOT NULL DEFAULT '[]',
    documents    TEXT NOT NULL DEFAULT '[]',
    availability TEXT NOT NULL DEFAULT '',
    notes        TEXT NOT NULL DEFAULT '',
    completeness INTEGER NOT NULL DEFAULT 0,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS saved_searches (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    profile_id TEXT NOT NULL,
    label      TEXT NOT NULL,
    query      TEXT NOT NULL,
    created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS saved_searches_profile ON saved_searches(profile_id);
`

func (s *SQLite) ensureProfileTables() error {
	if _, err := s.db.Exec(profileSchema); err != nil {
		return fmt.Errorf("store: profile schema: %w", err)
	}
	return nil
}

func encodeList(v []string) string {
	if v == nil {
		v = []string{}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func decodeList(raw string) []string {
	var out []string
	if raw == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func (s *SQLite) UpsertProfile(ctx context.Context, p model.Profile) (model.Profile, error) {
	if err := s.ensureProfileTables(); err != nil {
		return model.Profile{}, err
	}
	now := time.Now()
	created := now
	if existing, err := s.GetProfile(ctx, p.ID); err == nil {
		created = existing.CreatedAt
	}
	p.CreatedAt = created
	p.UpdatedAt = now
	p.Completeness = Completeness(p)

	_, err := s.db.ExecContext(ctx, `
INSERT INTO profiles (id, role, company, contact, trades, regions, crew_size, languages, documents, availability, notes, completeness, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    role=excluded.role, company=excluded.company, contact=excluded.contact,
    trades=excluded.trades, regions=excluded.regions, crew_size=excluded.crew_size,
    languages=excluded.languages, documents=excluded.documents,
    availability=excluded.availability, notes=excluded.notes,
    completeness=excluded.completeness, updated_at=excluded.updated_at`,
		p.ID, p.Role, p.Company, p.Contact, encodeList(p.Trades), encodeList(p.Regions), p.CrewSize,
		encodeList(p.Languages), encodeList(p.Documents), p.Availability, p.Notes, p.Completeness,
		nonZeroUnix(p.CreatedAt), nonZeroUnix(p.UpdatedAt))
	if err != nil {
		return model.Profile{}, fmt.Errorf("store: upsert profile: %w", err)
	}
	return p, nil
}

func (s *SQLite) GetProfile(ctx context.Context, id string) (model.Profile, error) {
	if err := s.ensureProfileTables(); err != nil {
		return model.Profile{}, err
	}
	var p model.Profile
	var trades, regions, languages, documents string
	var createdAt, updatedAt int64
	err := s.db.QueryRowContext(ctx, `
SELECT id, role, company, contact, trades, regions, crew_size, languages, documents, availability, notes, completeness, created_at, updated_at
FROM profiles WHERE id = ?`, id).Scan(&p.ID, &p.Role, &p.Company, &p.Contact, &trades, &regions,
		&p.CrewSize, &languages, &documents, &p.Availability, &p.Notes, &p.Completeness, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Profile{}, ErrNotFound
	}
	if err != nil {
		return model.Profile{}, fmt.Errorf("store: get profile: %w", err)
	}
	p.Trades, p.Regions, p.Languages, p.Documents = decodeList(trades), decodeList(regions), decodeList(languages), decodeList(documents)
	p.CreatedAt, p.UpdatedAt = unixToTime(createdAt), unixToTime(updatedAt)
	return p, nil
}

func (s *SQLite) SaveSearch(ctx context.Context, search model.SavedSearch) (model.SavedSearch, error) {
	if err := s.ensureProfileTables(); err != nil {
		return model.SavedSearch{}, err
	}
	if search.CreatedAt.IsZero() {
		search.CreatedAt = time.Now()
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO saved_searches (profile_id, label, query, created_at) VALUES (?, ?, ?, ?)`,
		search.ProfileID, search.Label, search.Query, nonZeroUnix(search.CreatedAt))
	if err != nil {
		return model.SavedSearch{}, fmt.Errorf("store: save search: %w", err)
	}
	id, _ := res.LastInsertId()
	search.ID = id
	return search, nil
}

func (s *SQLite) ListSavedSearches(ctx context.Context, profileID string) ([]model.SavedSearch, error) {
	if err := s.ensureProfileTables(); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, profile_id, label, query, created_at FROM saved_searches WHERE profile_id = ? ORDER BY id DESC`, profileID)
	if err != nil {
		return nil, fmt.Errorf("store: list saved searches: %w", err)
	}
	defer rows.Close()
	out := []model.SavedSearch{}
	for rows.Next() {
		var s model.SavedSearch
		var createdAt int64
		if err := rows.Scan(&s.ID, &s.ProfileID, &s.Label, &s.Query, &createdAt); err != nil {
			return nil, err
		}
		s.CreatedAt = unixToTime(createdAt)
		out = append(out, s)
	}
	return out, rows.Err()
}
