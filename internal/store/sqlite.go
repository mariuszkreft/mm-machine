package store

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"mm-machine/internal/model"
)

//go:embed schema.sql
var schemaSQL string

// SQLite is the on-disk Store backed by modernc.org/sqlite (pure Go, no CGO).
type SQLite struct {
	db *sql.DB
}

var _ Store = (*SQLite)(nil)

// Open returns a SQLite-backed Store. It creates the parent directory, opens
// the database with WAL journaling and a busy timeout, runs idempotent
// migrations, and seeds static content on first boot.
func Open(path string) (Store, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("store: create dir: %w", err)
		}
	}

	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	// A single connection serializes every reader and writer, which is the
	// simplest way to avoid "database is locked" under concurrent HTTP
	// handlers without hand-rolled locking.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}

	s := &SQLite{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.seed(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// numberedMigrations upgrade a database created by an older schema.sql.
// schema.sql itself only ever grows tables that did not exist yet (CREATE
// TABLE/INDEX IF NOT EXISTS is safe on any database); a column added to an
// existing table needs an explicit ALTER, guarded here by schema_migrations
// so it runs exactly once, on fresh and pre-existing databases alike.
var numberedMigrations = []struct {
	version int
	apply   func(*sql.Tx) error
}{
	{2, addOfferFacetColumns},
	{3, backfillOfferFacets},
	{4, dedupeAndIndexSavedSearches},
	{5, buildFullTextIndex},
	{6, germaniseLegacySeedOffers},
}

// germaniseLegacySeedOffers rewrites the four original English seed rows on an
// existing installation. The app is German-first now, and a German market with
// four English rows in it reads as a bug. Rows an operator has already edited
// are left alone: the update only fires when the title still matches the
// English original.
func germaniseLegacySeedOffers(tx *sql.Tx) error {
	replacements := []struct {
		id, oldTitle string
		offer        model.Offer
	}{
		{"MM-1842", "Photovoltaic roof installation", SeedOffers()[0]},
		{"MM-1841", "Retail floor refit", SeedOffers()[1]},
		{"MM-1838", "Warehouse steel assembly", SeedOffers()[2]},
		{"MM-1832", "Hotel bathroom modernization", SeedOffers()[3]},
	}
	for _, r := range replacements {
		if _, err := tx.Exec(`
UPDATE offers
   SET title = ?, location = ?, category = ?, amount = ?, attention = ?
 WHERE id = ? AND title = ?`,
			r.offer.Title, r.offer.Location, r.offer.Category, r.offer.Amount, r.offer.Attention,
			r.id, r.oldTitle); err != nil {
			return fmt.Errorf("germanise %s: %w", r.id, err)
		}
	}
	return nil
}

func (s *SQLite) migrate() error {
	for _, stmt := range strings.Split(schemaSQL, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("store: migrate: %w", err)
		}
	}
	for _, m := range numberedMigrations {
		if err := s.applyMigration(m.version, m.apply); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLite) applyMigration(version int, apply func(*sql.Tx) error) error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, version).Scan(&n); err != nil {
		return fmt.Errorf("store: migration %d check: %w", version, err)
	}
	if n > 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("store: migration %d: %w", version, err)
	}
	defer tx.Rollback()

	if err := apply(tx); err != nil {
		return fmt.Errorf("store: migration %d: %w", version, err)
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, version); err != nil {
		return fmt.Errorf("store: migration %d: %w", version, err)
	}
	return tx.Commit()
}

// addOfferFacetColumns adds the normalized search facets to an offers table
// that predates them.
func addOfferFacetColumns(tx *sql.Tx) error {
	stmts := []string{
		`ALTER TABLE offers ADD COLUMN trade TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE offers ADD COLUMN region TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE offers ADD COLUMN crew_size INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE offers ADD COLUMN start_at INTEGER`,
		`ALTER TABLE offers ADD COLUMN requirements TEXT NOT NULL DEFAULT '[]'`,
		`ALTER TABLE offers ADD COLUMN languages TEXT NOT NULL DEFAULT '[]'`,
		`CREATE INDEX IF NOT EXISTS idx_offers_status ON offers(status)`,
		`CREATE INDEX IF NOT EXISTS idx_offers_trade ON offers(trade)`,
		`CREATE INDEX IF NOT EXISTS idx_offers_category ON offers(category)`,
		`CREATE INDEX IF NOT EXISTS idx_offers_region ON offers(region)`,
		`CREATE INDEX IF NOT EXISTS idx_offers_crew_size ON offers(crew_size)`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// backfillOfferFacets fills the new columns on rows written before they
// existed: rows whose id matches a seed offer get the seed's facets, every
// other row derives trade from category and region from location.
func backfillOfferFacets(tx *sql.Tx) error {
	seedByID := map[string]model.Offer{}
	for _, o := range SeedOffers() {
		seedByID[o.ID] = o
	}

	rows, err := tx.Query(`SELECT id, category, location FROM offers`)
	if err != nil {
		return err
	}
	type row struct{ id, category, location string }
	var pending []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.category, &r.location); err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, r := range pending {
		if seed, ok := seedByID[r.id]; ok {
			_, err := tx.Exec(`UPDATE offers SET trade=?, region=?, crew_size=?, start_at=?, requirements=?, languages=? WHERE id=?`,
				seed.Trade, seed.Region, seed.CrewSize, nullableUnix(seed.Start), encodeList(seed.Requirements), encodeList(seed.Languages), r.id)
			if err != nil {
				return err
			}
			continue
		}
		if _, err := tx.Exec(`UPDATE offers SET trade=?, region=? WHERE id=?`, r.category, r.location, r.id); err != nil {
			return err
		}
	}
	return nil
}

// dedupeAndIndexSavedSearches collapses any duplicate (profile_id, query)
// rows left over from before saved searches were deduplicated, keeping the
// newest one, then enforces the constraint with a unique index so SaveSearch
// can rely on ON CONFLICT.
func dedupeAndIndexSavedSearches(tx *sql.Tx) error {
	if _, err := tx.Exec(`
DELETE FROM saved_searches
WHERE id NOT IN (
    SELECT MAX(id) FROM saved_searches GROUP BY profile_id, query
)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_saved_searches_dedup ON saved_searches(profile_id, query)`); err != nil {
		return err
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

func (s *SQLite) seed() error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM offers`).Scan(&n); err != nil {
		return fmt.Errorf("store: seed check offers: %w", err)
	}
	if n == 0 {
		for _, o := range SeedOffers() {
			if _, err := s.CreateOffer(context.Background(), o); err != nil {
				return fmt.Errorf("store: seed offers: %w", err)
			}
		}
	}

	if err := s.db.QueryRow(`SELECT COUNT(*) FROM perspectives`).Scan(&n); err != nil {
		return fmt.Errorf("store: seed check perspectives: %w", err)
	}
	if n == 0 {
		for i, p := range SeedPerspectives() {
			statsJSON, err := json.Marshal(p.Stats)
			if err != nil {
				return err
			}
			workflowJSON, err := json.Marshal(p.Workflow)
			if err != nil {
				return err
			}
			painJSON, err := json.Marshal(p.Pain)
			if err != nil {
				return err
			}
			_, err = s.db.Exec(`INSERT INTO perspectives
				(key, sort_order, label, title, subtitle, quote, primary_action, secondary_action, action_name, stats_json, workflow_json, pain_json)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				p.Key, i, p.Label, p.Title, p.Subtitle, p.Quote, p.Primary, p.Secondary, p.ActionName, string(statsJSON), string(workflowJSON), string(painJSON))
			if err != nil {
				return fmt.Errorf("store: seed perspectives: %w", err)
			}
		}
	}

	if err := s.db.QueryRow(`SELECT COUNT(*) FROM modules`).Scan(&n); err != nil {
		return fmt.Errorf("store: seed check modules: %w", err)
	}
	if n == 0 {
		for i, m := range SeedModules() {
			if _, err := s.db.Exec(`INSERT INTO modules (sort_order, name, body, impact) VALUES (?, ?, ?, ?)`,
				i, m.Name, m.Body, m.Impact); err != nil {
				return fmt.Errorf("store: seed modules: %w", err)
			}
		}
	}

	if err := s.db.QueryRow(`SELECT COUNT(*) FROM roadmap`).Scan(&n); err != nil {
		return fmt.Errorf("store: seed check roadmap: %w", err)
	}
	if n == 0 {
		for i, r := range SeedRoadmap() {
			if _, err := s.db.Exec(`INSERT INTO roadmap (sort_order, phase, title, body) VALUES (?, ?, ?, ?)`,
				i, r.Phase, r.Title, r.Body); err != nil {
				return fmt.Errorf("store: seed roadmap: %w", err)
			}
		}
	}

	return nil
}

func unixToTime(sec int64) time.Time { return time.Unix(sec, 0).UTC() }

// nonZeroUnix returns the unix-seconds form of t, substituting now for a
// zero time so a timestamp column is never written as zero.
func nonZeroUnix(t time.Time) int64 {
	if t.IsZero() {
		t = time.Now()
	}
	return t.UTC().Unix()
}

// nullableUnix leaves a zero time as SQL NULL instead of substituting now,
// so an offer without a known start date never gets a fabricated one.
func nullableUnix(t time.Time) sql.NullInt64 {
	if t.IsZero() {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: t.UTC().Unix(), Valid: true}
}

// --- Offers -----------------------------------------------------------

const offerColumns = `id, title, location, category, amount, budget, status, signal, supplier, progress, attention, trade, region, crew_size, start_at, requirements, languages, created_at, updated_at`

// ListOffers answers the pipeline and search queries. Status, free text and
// the facet fields (Statuses, Trades, Regions, Requirements, MinCrewSize) are
// all pushed into SQL for indexed narrowing; MatchesFacets is then applied as
// a final guard so the SQL path and the Go (Memory) path can never diverge.
func (s *SQLite) ListOffers(ctx context.Context, f OfferFilter) ([]model.Offer, error) {
	query := `SELECT ` + offerColumns + ` FROM offers WHERE 1=1`
	args := []any{}
	if f.Status != "" && !strings.EqualFold(f.Status, "all") {
		query += ` AND LOWER(status) = LOWER(?)`
		args = append(args, f.Status)
	}
	if f.Query != "" {
		query += ` AND LOWER(title || ' ' || location || ' ' || category || ' ' || supplier) LIKE ?`
		args = append(args, "%"+strings.ToLower(f.Query)+"%")
	}
	if len(f.Statuses) > 0 {
		query += ` AND LOWER(status) IN (` + placeholders(len(f.Statuses)) + `)`
		for _, v := range f.Statuses {
			args = append(args, strings.ToLower(strings.TrimSpace(v)))
		}
	}
	if len(f.Trades) > 0 {
		query += ` AND LOWER(CASE WHEN TRIM(trade) <> '' THEN trade ELSE category END) IN (` + placeholders(len(f.Trades)) + `)`
		for _, v := range f.Trades {
			args = append(args, strings.ToLower(strings.TrimSpace(v)))
		}
	}
	if clause, clauseArgs := regionClause(f.Regions); clause != "" {
		query += ` AND (` + clause + `)`
		args = append(args, clauseArgs...)
	}
	for _, v := range f.Requirements {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		query += ` AND LOWER(requirements) LIKE ?`
		args = append(args, `%"`+strings.ToLower(v)+`"%`)
	}
	if f.MinCrewSize > 0 {
		query += ` AND (crew_size = 0 OR crew_size >= ?)`
		args = append(args, f.MinCrewSize)
	}
	query += ` ORDER BY updated_at DESC`
	if f.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, f.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list offers: %w", err)
	}
	defer rows.Close()

	out := []model.Offer{}
	for rows.Next() {
		o, err := scanOffer(rows)
		if err != nil {
			return nil, err
		}
		if !MatchesFacets(o, f) {
			continue
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// regionClause mirrors anyContainsFold's bidirectional substring match in
// SQL: a region value matches if it contains, or is contained by, the
// offer's effective region (region, falling back to location).
func regionClause(regions []string) (string, []any) {
	effective := `LOWER(CASE WHEN TRIM(region) <> '' THEN region ELSE location END)`
	var clauses []string
	var args []any
	for _, v := range regions {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" {
			continue
		}
		clauses = append(clauses, `(`+effective+` LIKE '%' || ? || '%' OR ? LIKE '%' || `+effective+` || '%')`)
		args = append(args, v, v)
	}
	if len(clauses) == 0 {
		return "", nil
	}
	return strings.Join(clauses, " OR "), args
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanOffer(row rowScanner) (model.Offer, error) {
	var o model.Offer
	var createdAt, updatedAt int64
	var start sql.NullInt64
	var requirementsJSON, languagesJSON string
	err := row.Scan(&o.ID, &o.Title, &o.Location, &o.Category, &o.Amount, &o.Budget, &o.Status, &o.Signal, &o.Supplier, &o.Progress, &o.Attention,
		&o.Trade, &o.Region, &o.CrewSize, &start, &requirementsJSON, &languagesJSON, &createdAt, &updatedAt)
	if err != nil {
		return model.Offer{}, err
	}
	if start.Valid {
		o.Start = unixToTime(start.Int64)
	}
	o.Requirements = decodeList(requirementsJSON)
	o.Languages = decodeList(languagesJSON)
	o.CreatedAt = unixToTime(createdAt)
	o.UpdatedAt = unixToTime(updatedAt)
	return o, nil
}

func (s *SQLite) GetOffer(ctx context.Context, id string) (model.Offer, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+offerColumns+` FROM offers WHERE id = ?`, id)
	o, err := scanOffer(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Offer{}, ErrNotFound
	}
	if err != nil {
		return model.Offer{}, fmt.Errorf("store: get offer: %w", err)
	}
	return o, nil
}

func (s *SQLite) CreateOffer(ctx context.Context, o model.Offer) (model.Offer, error) {
	created := nonZeroUnix(o.CreatedAt)
	updated := nonZeroUnix(time.Now())
	_, err := s.db.ExecContext(ctx, `INSERT INTO offers
		(id, title, location, category, amount, budget, status, signal, supplier, progress, attention, trade, region, crew_size, start_at, requirements, languages, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		o.ID, o.Title, o.Location, o.Category, o.Amount, o.Budget, o.Status, o.Signal, o.Supplier, o.Progress, o.Attention,
		o.Trade, o.Region, o.CrewSize, nullableUnix(o.Start), encodeList(o.Requirements), encodeList(o.Languages), created, updated)
	if err != nil {
		return model.Offer{}, fmt.Errorf("store: create offer: %w", err)
	}
	o.CreatedAt = unixToTime(created)
	o.UpdatedAt = unixToTime(updated)
	return o, nil
}

func (s *SQLite) UpdateOffer(ctx context.Context, o model.Offer) (model.Offer, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Offer{}, fmt.Errorf("store: update offer: %w", err)
	}
	defer tx.Rollback()

	var created int64
	err = tx.QueryRowContext(ctx, `SELECT created_at FROM offers WHERE id = ?`, o.ID).Scan(&created)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Offer{}, ErrNotFound
	}
	if err != nil {
		return model.Offer{}, fmt.Errorf("store: update offer: %w", err)
	}

	updated := nonZeroUnix(time.Now())
	_, err = tx.ExecContext(ctx, `UPDATE offers SET title=?, location=?, category=?, amount=?, budget=?, status=?, signal=?, supplier=?, progress=?, attention=?, trade=?, region=?, crew_size=?, start_at=?, requirements=?, languages=?, updated_at=? WHERE id=?`,
		o.Title, o.Location, o.Category, o.Amount, o.Budget, o.Status, o.Signal, o.Supplier, o.Progress, o.Attention,
		o.Trade, o.Region, o.CrewSize, nullableUnix(o.Start), encodeList(o.Requirements), encodeList(o.Languages), updated, o.ID)
	if err != nil {
		return model.Offer{}, fmt.Errorf("store: update offer: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.Offer{}, fmt.Errorf("store: update offer: %w", err)
	}

	o.CreatedAt = unixToTime(created)
	o.UpdatedAt = unixToTime(updated)
	return o, nil
}

func (s *SQLite) CountOffersByStatus(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM offers GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("store: count offers: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{"all": 0}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		counts[strings.ToLower(status)] += n
		counts["all"] += n
	}
	return counts, rows.Err()
}

// --- Static-ish content -------------------------------------------------

func (s *SQLite) ListPerspectives(ctx context.Context) ([]model.Perspective, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, label, title, subtitle, quote, primary_action, secondary_action, action_name, stats_json, workflow_json, pain_json FROM perspectives ORDER BY sort_order`)
	if err != nil {
		return nil, fmt.Errorf("store: list perspectives: %w", err)
	}
	defer rows.Close()

	out := []model.Perspective{}
	for rows.Next() {
		var p model.Perspective
		var statsJSON, workflowJSON, painJSON string
		if err := rows.Scan(&p.Key, &p.Label, &p.Title, &p.Subtitle, &p.Quote, &p.Primary, &p.Secondary, &p.ActionName, &statsJSON, &workflowJSON, &painJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(statsJSON), &p.Stats); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(workflowJSON), &p.Workflow); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(painJSON), &p.Pain); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *SQLite) ListModules(ctx context.Context) ([]model.Module, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, body, impact FROM modules ORDER BY sort_order`)
	if err != nil {
		return nil, fmt.Errorf("store: list modules: %w", err)
	}
	defer rows.Close()

	out := []model.Module{}
	for rows.Next() {
		var m model.Module
		if err := rows.Scan(&m.Name, &m.Body, &m.Impact); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *SQLite) ListRoadmap(ctx context.Context) ([]model.RoadmapItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT phase, title, body FROM roadmap ORDER BY sort_order`)
	if err != nil {
		return nil, fmt.Errorf("store: list roadmap: %w", err)
	}
	defer rows.Close()

	out := []model.RoadmapItem{}
	for rows.Next() {
		var r model.RoadmapItem
		if err := rows.Scan(&r.Phase, &r.Title, &r.Body); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- Conversations and chat ---------------------------------------------

func (s *SQLite) CreateConversation(ctx context.Context, c model.Conversation) (model.Conversation, error) {
	created := nonZeroUnix(c.CreatedAt)
	updated := nonZeroUnix(time.Now())
	_, err := s.db.ExecContext(ctx, `INSERT INTO conversations (id, role, route, created_at, updated_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET role=excluded.role, route=excluded.route, created_at=excluded.created_at, updated_at=excluded.updated_at`,
		c.ID, c.Role, c.Route, created, updated)
	if err != nil {
		return model.Conversation{}, fmt.Errorf("store: create conversation: %w", err)
	}
	c.CreatedAt = unixToTime(created)
	c.UpdatedAt = unixToTime(updated)
	return c, nil
}

func (s *SQLite) GetConversation(ctx context.Context, id string) (model.Conversation, error) {
	var c model.Conversation
	var created, updated int64
	err := s.db.QueryRowContext(ctx, `SELECT id, role, route, created_at, updated_at FROM conversations WHERE id = ?`, id).
		Scan(&c.ID, &c.Role, &c.Route, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Conversation{}, ErrNotFound
	}
	if err != nil {
		return model.Conversation{}, fmt.Errorf("store: get conversation: %w", err)
	}
	c.CreatedAt = unixToTime(created)
	c.UpdatedAt = unixToTime(updated)
	return c, nil
}

func (s *SQLite) AppendMessage(ctx context.Context, m model.ChatMessage) (model.ChatMessage, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.ChatMessage{}, fmt.Errorf("store: append message: %w", err)
	}
	defer tx.Rollback()

	created := nonZeroUnix(m.CreatedAt)
	res, err := tx.ExecContext(ctx, `INSERT INTO messages (conversation_id, role, content, reasoning, created_at) VALUES (?, ?, ?, ?, ?)`,
		m.ConversationID, m.Role, m.Content, m.Reasoning, created)
	if err != nil {
		return model.ChatMessage{}, fmt.Errorf("store: append message: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return model.ChatMessage{}, fmt.Errorf("store: append message: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `UPDATE conversations SET updated_at = ? WHERE id = ?`, created, m.ConversationID); err != nil {
		return model.ChatMessage{}, fmt.Errorf("store: append message: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return model.ChatMessage{}, fmt.Errorf("store: append message: %w", err)
	}

	m.ID = id
	m.CreatedAt = unixToTime(created)
	return m, nil
}

func (s *SQLite) ListMessages(ctx context.Context, conversationID string) ([]model.ChatMessage, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, conversation_id, role, content, reasoning, created_at FROM messages WHERE conversation_id = ? ORDER BY id ASC`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("store: list messages: %w", err)
	}
	defer rows.Close()

	out := []model.ChatMessage{}
	for rows.Next() {
		var m model.ChatMessage
		var created int64
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.Reasoning, &created); err != nil {
			return nil, err
		}
		m.CreatedAt = unixToTime(created)
		out = append(out, m)
	}
	return out, rows.Err()
}

// --- Feedback -------------------------------------------------------------

func (s *SQLite) CreateFeedback(ctx context.Context, f model.Feedback) (model.Feedback, error) {
	if f.Status == "" {
		f.Status = "new"
	}
	created := nonZeroUnix(f.CreatedAt)
	res, err := s.db.ExecContext(ctx, `INSERT INTO feedback
		(conversation_id, kind, theme, severity, verbatim, requested, route, role, source, created_at, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ConversationID, f.Kind, f.Theme, f.Severity, f.Verbatim, f.Requested, f.Route, f.Role, f.Source, created, f.Status)
	if err != nil {
		return model.Feedback{}, fmt.Errorf("store: create feedback: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return model.Feedback{}, fmt.Errorf("store: create feedback: %w", err)
	}
	f.ID = id
	f.CreatedAt = unixToTime(created)
	return f, nil
}

func (s *SQLite) ListFeedback(ctx context.Context, f FeedbackFilter) ([]model.Feedback, error) {
	query := `SELECT id, conversation_id, kind, theme, severity, verbatim, requested, route, role, source, created_at, status FROM feedback WHERE 1=1`
	args := []any{}
	if f.Status != "" {
		query += ` AND status = ?`
		args = append(args, f.Status)
	}
	if f.Kind != "" {
		query += ` AND kind = ?`
		args = append(args, f.Kind)
	}
	if f.Since > 0 {
		query += ` AND created_at >= ?`
		args = append(args, f.Since)
	}
	query += ` ORDER BY created_at DESC`
	if f.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, f.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list feedback: %w", err)
	}
	defer rows.Close()

	out := []model.Feedback{}
	for rows.Next() {
		var item model.Feedback
		var created int64
		if err := rows.Scan(&item.ID, &item.ConversationID, &item.Kind, &item.Theme, &item.Severity, &item.Verbatim, &item.Requested, &item.Route, &item.Role, &item.Source, &created, &item.Status); err != nil {
			return nil, err
		}
		item.CreatedAt = unixToTime(created)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *SQLite) SetFeedbackStatus(ctx context.Context, id int64, status string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE feedback SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return fmt.Errorf("store: set feedback status: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: set feedback status: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Backlog ---------------------------------------------------------------

func (s *SQLite) ReplaceBacklog(ctx context.Context, items []model.BacklogItem) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: replace backlog: %w", err)
	}
	defer tx.Rollback()

	existingStatus := map[string]string{}
	rows, err := tx.QueryContext(ctx, `SELECT theme, status FROM backlog`)
	if err != nil {
		return fmt.Errorf("store: replace backlog: %w", err)
	}
	for rows.Next() {
		var theme, status string
		if err := rows.Scan(&theme, &status); err != nil {
			rows.Close()
			return err
		}
		if _, ok := existingStatus[theme]; !ok {
			existingStatus[theme] = status
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	if _, err := tx.ExecContext(ctx, `DELETE FROM backlog`); err != nil {
		return fmt.Errorf("store: replace backlog: %w", err)
	}

	now := nonZeroUnix(time.Now())
	for i := range items {
		status := items[i].Status
		if prev, ok := existingStatus[items[i].Theme]; ok {
			status = prev
		} else if status == "" {
			status = "proposed"
		}

		evidenceJSON, err := json.Marshal(items[i].Evidence)
		if err != nil {
			return err
		}

		res, err := tx.ExecContext(ctx, `INSERT INTO backlog
			(title, rationale, theme, kind, count, avg_severity, score, evidence_json, status, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			items[i].Title, items[i].Rationale, items[i].Theme, items[i].Kind, items[i].Count, items[i].AvgSeverity, items[i].Score, string(evidenceJSON), status, now)
		if err != nil {
			return fmt.Errorf("store: replace backlog: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("store: replace backlog: %w", err)
		}
		items[i].ID = id
		items[i].Status = status
		items[i].UpdatedAt = unixToTime(now)
	}

	return tx.Commit()
}

func (s *SQLite) ListBacklog(ctx context.Context) ([]model.BacklogItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, title, rationale, theme, kind, count, avg_severity, score, evidence_json, status, updated_at FROM backlog ORDER BY score DESC`)
	if err != nil {
		return nil, fmt.Errorf("store: list backlog: %w", err)
	}
	defer rows.Close()

	out := []model.BacklogItem{}
	for rows.Next() {
		var item model.BacklogItem
		var evidenceJSON string
		var updated int64
		if err := rows.Scan(&item.ID, &item.Title, &item.Rationale, &item.Theme, &item.Kind, &item.Count, &item.AvgSeverity, &item.Score, &evidenceJSON, &item.Status, &updated); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(evidenceJSON), &item.Evidence); err != nil {
			return nil, err
		}
		item.UpdatedAt = unixToTime(updated)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *SQLite) SetBacklogStatus(ctx context.Context, id int64, status string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE backlog SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return fmt.Errorf("store: set backlog status: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: set backlog status: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLite) Close() error { return s.db.Close() }
