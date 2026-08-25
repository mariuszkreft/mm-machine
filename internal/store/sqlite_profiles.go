package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"mm-machine/internal/model"
)

// Profiles and saved searches are defined in schema.sql; this file only
// holds the read/write paths.

func (s *SQLite) UpsertProfile(ctx context.Context, p model.Profile) (model.Profile, error) {
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

// SaveSearch dedupes an identical query for the same profile (the newest
// write wins, bumping its created_at) and caps each profile at the newest 50
// searches so the table cannot grow without bound.
func (s *SQLite) SaveSearch(ctx context.Context, search model.SavedSearch) (model.SavedSearch, error) {
	if search.CreatedAt.IsZero() {
		search.CreatedAt = time.Now()
	}
	created := nonZeroUnix(search.CreatedAt)
	_, err := s.db.ExecContext(ctx, `
INSERT INTO saved_searches (profile_id, label, query, created_at) VALUES (?, ?, ?, ?)
ON CONFLICT(profile_id, query) DO UPDATE SET label=excluded.label, created_at=excluded.created_at`,
		search.ProfileID, search.Label, search.Query, created)
	if err != nil {
		return model.SavedSearch{}, fmt.Errorf("store: save search: %w", err)
	}
	var id int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT id FROM saved_searches WHERE profile_id = ? AND query = ?`, search.ProfileID, search.Query,
	).Scan(&id); err != nil {
		return model.SavedSearch{}, fmt.Errorf("store: save search: %w", err)
	}
	search.ID = id
	search.CreatedAt = unixToTime(created)

	if _, err := s.db.ExecContext(ctx, `
DELETE FROM saved_searches
WHERE profile_id = ? AND id NOT IN (
    SELECT id FROM saved_searches WHERE profile_id = ? ORDER BY created_at DESC, id DESC LIMIT 50
)`, search.ProfileID, search.ProfileID); err != nil {
		return model.SavedSearch{}, fmt.Errorf("store: save search: %w", err)
	}
	return search, nil
}

func (s *SQLite) ListSavedSearches(ctx context.Context, profileID string) ([]model.SavedSearch, error) {
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
