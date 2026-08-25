package store

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"mm-machine/internal/model"
)

// --- Query tokenization -----------------------------------------------------

// termPattern extracts alnum runs from raw user input and is also used to
// tokenize document text for the approximate (Memory / fallback) scorer, so
// both sides split words the same way. Using it exclusively to build the FTS
// MATCH expression is what makes hostile input safe: the query string sent
// to SQLite is assembled entirely from these extracted runs plus a literal
// "*" and " OR ", so raw quotes, asterisks, colons, parens or the bareword
// operators AND/OR/NOT/NEAR from the user's text never reach FTS5 syntax.
var termPattern = regexp.MustCompile(`[\p{L}\p{N}]+`)

// foldGerman mirrors unicode61's remove_diacritics=2 behaviour for the small
// set of characters this corpus actually uses (verified against the FTS5
// tokenizer directly: ä/ö/ü fold to a/o/u, ß is left alone), so a query term
// we fold ourselves lines up with what the tokenizer already folded in the
// index, and so the Memory/fallback scorer can do the same comparison in Go.
func foldGerman(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case 'ä', 'à', 'á', 'â':
			return 'a'
		case 'ö', 'ò', 'ó', 'ô':
			return 'o'
		case 'ü', 'ù', 'ú', 'û':
			return 'u'
		case 'é', 'è', 'ê':
			return 'e'
		default:
			return r
		}
	}, s)
}

// extractTokens lowercases, diacritic-folds and splits text into the words
// worth searching on. Tokens shorter than 2 runes are dropped as noise (also
// keeps a lone "*" or quote from a hostile query from producing any token at
// all, short-circuiting search entirely rather than risking a query with no
// safe terms).
func extractTokens(text string) []string {
	raw := termPattern.FindAllString(strings.ToLower(text), -1)
	out := make([]string, 0, len(raw))
	for _, t := range raw {
		t = foldGerman(t)
		if len([]rune(t)) < 2 {
			continue
		}
		out = append(out, t)
	}
	return out
}

// --- German synonyms ---------------------------------------------------------

// synonymGroups is the DE<->EN vocabulary map for this domain, expressed as
// groups of alternative token sequences: finding any sequence of a group in
// the query pulls in every other sequence's tokens too. A group of
// single-token sequences is a plain word synonym (Kolonne <-> crew); the
// A1-Bescheinigung group uses two-token sequences because both the German
// and English forms are two words once tokenized ("a1 bescheinigung",
// "a1 certificate"). Extend this table, not the matching logic, to cover
// more vocabulary.
var synonymGroups = [][][]string{
	{{"monteur"}, {"fitter"}},
	{{"kolonne"}, {"crew"}},
	{{"gewerk"}, {"trade"}},
	{{"nachunternehmer"}, {"subcontractor"}},
	{{"a1", "bescheinigung"}, {"a1", "certificate"}},
	{{"trockenbau"}, {"drywall"}},
	{{"stahlbau"}, {"steel"}},
	{{"sanitar"}, {"sanitary"}}, // sanitär, folded
	{{"elektro"}, {"electrical"}},
}

// containsSeq reports whether seq occurs as a contiguous run inside tokens.
func containsSeq(tokens, seq []string) bool {
	if len(seq) == 0 || len(seq) > len(tokens) {
		return false
	}
	for start := 0; start+len(seq) <= len(tokens); start++ {
		match := true
		for i, s := range seq {
			if tokens[start+i] != s {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// synonymAdditions returns every token pulled in by a synonym group that has
// a sequence present in tokens.
func synonymAdditions(tokens []string) []string {
	var add []string
	for _, group := range synonymGroups {
		matched := false
		for _, seq := range group {
			if containsSeq(tokens, seq) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		for _, seq := range group {
			add = append(add, seq...)
		}
	}
	return add
}

// --- German compounds and inflection -----------------------------------------

// Cheap decompounding, no dictionary or NLP: German compounds (Dachmontage)
// and inflected forms (Elektriker vs. Elektro) are handled by generating
// extra candidate substrings from each long query token and searching them
// as word-prefixes rather than raw substrings. stemLen truncates a term to
// its first few characters (Elektriker -> Elektr, which prefix-matches
// Elektro); for terms at least splitMinLen long, every cut point from
// splitMinPart in also yields the remainder as a candidate (Dachmontage,
// cut at 4, yields "montage", which prefix-matches the standalone word).
// Tradeoff: this is a blind window, not a real split — most candidates from
// an arbitrary word will match nothing, and a few (short, generic remainders
// like "tage") could coincidentally prefix-match an unrelated word. That
// risk is bounded three ways: candidates only ever prefix-match a whole
// document word (never a raw substring, so "Etagen" is not hit by "tage"),
// they score lower than a literal/synonym term, and bm25/scoreDoc rank exact
// hits above them either way.
const (
	stemLen         = 6
	splitMinLen     = 9
	splitMinPart    = 4
	maxSplitPerTerm = 6
)

func compoundCandidates(token string) []string {
	r := []rune(token)
	var out []string
	if len(r) > stemLen {
		out = append(out, string(r[:stemLen]))
	}
	if len(r) >= splitMinLen {
		count := 0
		for cut := splitMinPart; cut <= len(r)-splitMinPart && count < maxSplitPerTerm; cut++ {
			cand := string(r[cut:])
			out = append(out, cand)
			count++
		}
	}
	return out
}

// --- Shared expansion ---------------------------------------------------------

func dedupeSorted(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// expandTerms turns raw query text into two term sets used identically by
// the SQLite/FTS path and the Memory/fallback path:
//
//   - terms: the literal words the user typed plus their synonyms. Scored
//     as a strong (whole-word or exact-token) match.
//   - extra: compound/inflection candidates derived from long terms. Scored
//     as a weaker (prefix) match.
//
// Both return values are sorted for deterministic output (important for
// tests and for building a stable FTS MATCH string).
func expandTerms(text string) (terms, extra []string) {
	tokens := extractTokens(text)
	if len(tokens) == 0 {
		return nil, nil
	}

	termSet := map[string]bool{}
	for _, t := range tokens {
		termSet[t] = true
	}
	for _, t := range synonymAdditions(tokens) {
		termSet[t] = true
	}

	extraSet := map[string]bool{}
	for _, t := range tokens {
		for _, c := range compoundCandidates(t) {
			if !termSet[c] {
				extraSet[c] = true
			}
		}
	}

	return dedupeSorted(termSet), dedupeSorted(extraSet)
}

// wantTextKind builds the Kinds predicate shared by both TextSearch
// implementations: empty Kinds means "everything".
func wantTextKind(kinds []string) func(string) bool {
	return func(kind string) bool {
		if len(kinds) == 0 {
			return true
		}
		for _, k := range kinds {
			if k == kind {
				return true
			}
		}
		return false
	}
}

// --- FTS5 query building ------------------------------------------------------

// buildMatchExpr turns terms/extra into an FTS5 MATCH expression built
// entirely from our own extracted, alnum-only tokens: every term becomes a
// bareword prefix query (term*), which is always safe syntax (it can never
// equal a bare AND/OR/NOT/NEAR operator, since those never carry a trailing
// "*"), OR'd together. Using prefix search uniformly (rather than exact
// phrase for literal terms) is deliberate: it also absorbs minor German
// inflection for free (searching "Nachunternehmer" via "nachunternehmer*"
// still finds the corpus's declined "Nachunternehmern").
func buildMatchExpr(terms, extra []string) string {
	seen := map[string]bool{}
	parts := make([]string, 0, len(terms)+len(extra))
	for _, t := range terms {
		if seen[t] {
			continue
		}
		seen[t] = true
		parts = append(parts, t+"*")
	}
	for _, t := range extra {
		if seen[t] {
			continue
		}
		seen[t] = true
		parts = append(parts, t+"*")
	}
	return strings.Join(parts, " OR ")
}

// --- Approximate scorer (Memory store and the SQLite fallback) --------------

func foldDoc(fields ...string) string {
	return foldGerman(strings.ToLower(strings.Join(fields, " ")))
}

// scoreDocPrefix credits a document for containing a word that starts with
// one of the compound/inflection candidates, mirroring the FTS5 prefix-query
// semantics used for the same candidates in buildMatchExpr (a match requires
// a whole document word to start with the candidate, not a raw substring
// anywhere). Like scoreDoc, each candidate contributes at most once,
// regardless of how many document words it matches, so a repeated word does
// not skew the score against how scoreDoc weighs literal terms.
func scoreDocPrefix(doc string, candidates []string) float64 {
	if len(candidates) == 0 {
		return 0
	}
	words := termPattern.FindAllString(doc, -1)
	score := 0.0
	for _, c := range candidates {
		for _, word := range words {
			if strings.HasPrefix(word, c) {
				score++
				break
			}
		}
	}
	return score
}

// fieldWeight mirrors the title/name bm25() column weight in fts.go
// (offersFTSWeights/crewsFTSWeights): without it, Memory would rank a
// passing mention in a note exactly as high as a title match, which
// disagrees with both the ranking-sanity requirement and SQLite's ranking.
const fieldWeight = 5.0

func fieldScore(doc string, terms, extra []string) float64 {
	return scoreDoc(doc, terms) + scoreDocPrefix(doc, extra)
}

func scoreOffer(o model.Offer, terms, extra []string) (float64, string) {
	title := foldDoc(o.Title)
	rest := foldDoc(o.Location, o.Category, o.Supplier, o.Trade, o.Region, o.Attention, strings.Join(o.Requirements, " "))
	score := fieldWeight*fieldScore(title, terms, extra) + fieldScore(rest, terms, extra)
	return score * lengthNorm(title, rest), o.Title
}

func scoreCrew(c model.Crew, terms, extra []string) (float64, string) {
	name := foldDoc(c.Name)
	rest := foldDoc(c.Company, strings.Join(c.Trades, " "), strings.Join(c.Regions, " "), c.Note, strings.Join(c.Documents, " "))
	score := fieldWeight*fieldScore(name, terms, extra) + fieldScore(rest, terms, extra)
	return score * lengthNorm(name, rest), c.Name
}

// lengthNorm approximates bm25's bias towards shorter documents: the same
// term in a terse record is stronger evidence than in a long one. Without it
// the two backends disagree on which of two equally-matching records ranks
// first, which is exactly what the parity test exists to catch.
//
// b mirrors bm25's length-normalisation constant, and avgWords is the rough
// average document length of this corpus — precision beyond that would be
// false: the goal is agreement on the top hit, not an identical score.
func lengthNorm(fields ...string) float64 {
	const (
		b        = 0.75
		avgWords = 24.0
	)
	words := 0
	for _, f := range fields {
		words += len(termPattern.FindAllString(f, -1))
	}
	if words == 0 {
		return 1
	}
	return 1 / (1 - b + b*(float64(words)/avgWords))
}

// --- SQLite: FTS5-backed TextSearch ------------------------------------------

// TextSearch answers out of the FTS5 index, ranked by bm25() (negated, since
// bm25 is lower-is-better and TextHit.Score is documented higher-is-better).
// Any error from the FTS path (including one this package failed to
// anticipate) falls back to the substring scan rather than surfacing a 500,
// per the Store contract.
func (s *SQLite) TextSearch(ctx context.Context, q TextQuery) ([]TextHit, error) {
	terms, extra := expandTerms(q.Text)
	if len(terms) == 0 && len(extra) == 0 {
		return []TextHit{}, nil
	}
	if err := s.ensureCrewTables(); err != nil {
		return nil, err
	}
	wantKind := wantTextKind(q.Kinds)
	matchExpr := buildMatchExpr(terms, extra)

	hits, err := s.ftsSearch(ctx, matchExpr, wantKind)
	if err != nil {
		return s.textSearchFallback(ctx, terms, extra, wantKind, q.Limit)
	}
	sortHits(hits)
	if q.Limit > 0 && len(hits) > q.Limit {
		hits = hits[:q.Limit]
	}
	return hits, nil
}

func (s *SQLite) ftsSearch(ctx context.Context, matchExpr string, wantKind func(string) bool) ([]TextHit, error) {
	var hits []TextHit
	if wantKind("offer") {
		h, err := s.ftsQuery(ctx, "offers_fts", "offers", "offer", offersFTSWeights, matchExpr)
		if err != nil {
			return nil, err
		}
		hits = append(hits, h...)
	}
	if wantKind("crew") {
		h, err := s.ftsQuery(ctx, "crews_fts", "crews", "crew", crewsFTSWeights, matchExpr)
		if err != nil {
			return nil, err
		}
		hits = append(hits, h...)
	}
	return hits, nil
}

// ftsQuery joins an FTS5 table back to its content table on rowid to recover
// the TEXT primary key, and asks bm25/snippet for the rank and preview.
// table/joinTable/weights are internal constants, never user input.
func (s *SQLite) ftsQuery(ctx context.Context, table, joinTable, kind, weights, matchExpr string) ([]TextHit, error) {
	query := fmt.Sprintf(`
SELECT j.id, bm25(%s, %s) AS rank, snippet(%s, -1, '', '', '...', 10)
FROM %s
JOIN %s j ON j.rowid = %s.rowid
WHERE %s MATCH ?`, table, weights, table, table, joinTable, table, table)
	rows, err := s.db.QueryContext(ctx, query, matchExpr)
	if err != nil {
		return nil, fmt.Errorf("store: fts %s: %w", table, err)
	}
	defer rows.Close()

	var hits []TextHit
	for rows.Next() {
		var id, snippet string
		var rank float64
		if err := rows.Scan(&id, &rank, &snippet); err != nil {
			return nil, err
		}
		hits = append(hits, TextHit{Kind: kind, ID: id, Score: -rank, Snippet: snippet})
	}
	return hits, rows.Err()
}

// textSearchFallback is the substring scan the Store contract requires when
// the FTS path errors. It shares its scoring (scoreOffer/scoreCrew) with the
// Memory store, so a degraded SQLite store and Memory agree on results.
func (s *SQLite) textSearchFallback(ctx context.Context, terms, extra []string, wantKind func(string) bool, limit int) ([]TextHit, error) {
	hits := []TextHit{}
	if wantKind("offer") {
		offers, err := s.ListOffers(ctx, OfferFilter{})
		if err != nil {
			return nil, err
		}
		for _, o := range offers {
			if score, title := scoreOffer(o, terms, extra); score > 0 {
				hits = append(hits, TextHit{Kind: "offer", ID: o.ID, Score: score, Snippet: title})
			}
		}
	}
	if wantKind("crew") {
		crews, err := s.ListCrews(ctx, CrewFilter{})
		if err != nil {
			return nil, err
		}
		for _, c := range crews {
			if score, name := scoreCrew(c, terms, extra); score > 0 {
				hits = append(hits, TextHit{Kind: "crew", ID: c.ID, Score: score, Snippet: name})
			}
		}
	}
	sortHits(hits)
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}
