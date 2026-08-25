package search

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"

	"mm-machine/internal/i18n"
	"mm-machine/internal/llm"
	"mm-machine/internal/model"
)

// briefMatch is the only view of a result the summary stream ever sees: an
// id to check against, and the numbers already shown on the card. It is
// small enough to travel as a query parameter, which keeps the whole
// exchange stateless.
type briefMatch struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Fit    int    `json:"fit"`
	Reason string `json:"reason"`
}

func toBrief(matches []model.Match) []briefMatch {
	out := make([]briefMatch, 0, len(matches))
	for _, m := range matches {
		out = append(out, briefMatch{ID: m.Ref(), Title: m.Title(), Fit: m.Fit, Reason: m.Why[0]})
	}
	return out
}

// mentionedIDRe finds offer-id-shaped tokens ("MM-1842") in free text. It is
// how the stream handler catches a model that names a result outside the
// list it was given — the one thing it must never be able to do.
var mentionedIDRe = regexp.MustCompile(`\b[A-Z]{2,}-[A-Za-z0-9]+\b`)

// mentionsUnlisted reports whether text names an offer id that isn't in
// allowed.
func mentionsUnlisted(text string, allowed map[string]bool) bool {
	for _, id := range mentionedIDRe.FindAllString(text, -1) {
		if !allowed[id] {
			return true
		}
	}
	return false
}

// mechanicalSummary is the deterministic, always-truthful line: it never
// names anything outside brief, because it is built only from brief. It is
// both the no-model fallback and the safety net when the streamed answer
// tries to introduce a result that isn't there. lang picks the visitor's
// language, independent of whatever language the query itself was in.
func mechanicalSummary(query string, intent model.Intent, brief []briefMatch, lang i18n.Lang) string {
	if len(brief) == 0 {
		return i18n.T(lang, "search.nothing")
	}
	var head string
	if lang == i18n.DE {
		head = fmt.Sprintf("%d %s", len(brief), i18n.T(lang, "search.matches"))
	} else {
		head = fmt.Sprintf("%d match%s", len(brief), plural(len(brief)))
	}
	parts := []string{head}
	if len(intent.Trades) > 0 {
		parts = append(parts, tr(lang, "search.summary.for", strings.Join(intent.Trades, ", ")))
	}
	if len(intent.Regions) > 0 {
		parts = append(parts, tr(lang, "search.summary.in", strings.Join(intent.Regions, ", ")))
	}
	best := brief[0]
	return strings.Join(parts, " ") + fmt.Sprintf(". %s: %s (%d%%) — %s.", i18n.T(lang, "search.bestFit"), best.Title, best.Fit, best.Reason)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "es"
}

// stream answers GET /find/stream: a single narrated sentence over the
// matches Query already computed and rendered as cards. The model may only
// phrase them; it is never given the chance to change which offers exist,
// and mentionsUnlisted catches anything it invents anyway.
func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	query := q.Get("q")
	lang := i18n.Detect(r)

	var brief []briefMatch
	_ = json.Unmarshal([]byte(q.Get("matches")), &brief)
	intent := model.Intent{
		Trades:  splitNonEmpty(q.Get("trades")),
		Regions: splitNonEmpty(q.Get("regions")),
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	defer func() {
		writeSSE(w, "done", "")
		flusher.Flush()
	}()

	fallback := mechanicalSummary(query, intent, brief, lang)
	if h.deps.LLM == nil || len(brief) == 0 || ctx.Err() != nil {
		emitWords(w, flusher, fallback)
		return
	}

	text := h.narrate(ctx, query, brief, lang)
	if text == "" || mentionsUnlisted(text, allowedIDs(brief)) {
		text = fallback
	}
	emitWords(w, flusher, text)
}

func allowedIDs(brief []briefMatch) map[string]bool {
	allowed := make(map[string]bool, len(brief))
	for _, b := range brief {
		allowed[b.ID] = true
	}
	return allowed
}

// narrate asks the model for one sentence over the given matches, in the
// visitor's language (i18n.AnswerIn). The whole answer is buffered here —
// never forwarded delta by delta — so nothing unvalidated ever reaches the
// client; emitWords re-streams it once it has passed mentionsUnlisted.
func (h *Handler) narrate(ctx context.Context, query string, brief []briefMatch, lang i18n.Lang) string {
	briefJSON, err := json.Marshal(brief)
	if err != nil {
		return ""
	}
	system := "You narrate search results for a construction marketplace in one short, plain sentence " +
		"(max 30 words). You are given the visitor's query and a fixed JSON list of matches, each with " +
		"an id, title, fit percentage and reason. Mention only offers from that list, by title. Never " +
		"invent an id, a title or a match that isn't in the list. No markdown, no lists. " +
		i18n.AnswerIn(lang)
	user := fmt.Sprintf("Query: %s\nMatches: %s", query, string(briefJSON))

	var buf strings.Builder
	err = h.deps.LLM.Stream(ctx, llm.Request{
		MaxTokens:   512,
		Temperature: 0.3,
		Messages: []llm.Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}, func(d llm.Delta) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		buf.WriteString(d.Content)
		return nil
	})
	if err != nil {
		return ""
	}
	return strings.TrimSpace(buf.String())
}

// emitWords sends text as a run of small SSE "message" events, so a
// validated-but-buffered answer still types itself in like a live stream.
func emitWords(w http.ResponseWriter, flusher http.Flusher, text string) {
	words := strings.Fields(text)
	for i, word := range words {
		chunk := word
		if i > 0 {
			chunk = " " + word
		}
		writeSSE(w, "message", html.EscapeString(chunk))
		flusher.Flush()
	}
}

// writeSSE writes one server-sent event, splitting data on newlines as the
// wire format requires.
func writeSSE(w http.ResponseWriter, event, data string) {
	fmt.Fprintf(w, "event: %s\n", event)
	if data == "" {
		fmt.Fprint(w, "data: \n")
	} else {
		for _, line := range strings.Split(data, "\n") {
			fmt.Fprintf(w, "data: %s\n", line)
		}
	}
	fmt.Fprint(w, "\n")
}

func splitNonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
