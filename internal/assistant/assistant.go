// Package assistant is the app's self-aware chat surface: it answers questions
// about Montage Manager using the local model, and mines every conversation for
// feedback about the app itself, which the dev loop then turns into a backlog.
//
// Streaming answers, per-visitor conversation memory and LLM feedback
// extraction all live here: /assistant/panel renders the widget and any
// existing history, /assistant/turn + /assistant/stream drive the
// server-sent-events flow, /assistant/message is the synchronous fallback for
// clients without SSE, and Extract is the background feedback miner.
package assistant

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"mm-machine/internal/app"
	"mm-machine/internal/llm"
	"mm-machine/internal/model"
	"mm-machine/internal/store"
)

// cookieName holds the conversation id across reloads, so a visitor continues
// the same thread instead of starting a fresh one on every page view.
const cookieName = "mm_conversation"

// historyTokenBudget caps how much conversation history is replayed into the
// model context. Oldest turns are dropped first.
const historyTokenBudget = 3000

// Handler serves the assistant panel and its htmx endpoints.
type Handler struct {
	deps app.Deps
	tpl  *template.Template
}

// Register wires the assistant routes onto mux.
func Register(mux *http.ServeMux, deps app.Deps) *Handler {
	h := &Handler{
		deps: deps,
		tpl:  template.Must(template.New("assistant").Parse(panelHTML + bubbleHTML)),
	}
	mux.HandleFunc("/assistant/panel", h.panel)
	mux.HandleFunc("/assistant/message", h.message)
	mux.HandleFunc("/assistant/turn", h.turn)
	mux.HandleFunc("/assistant/stream", h.stream)
	mux.HandleFunc("/feedback", h.feedback)
	return h
}

// panelView is the view model for the assistant panel.
type panelView struct {
	ConversationID string
	Role           string
	Route          string
	Model          string
	Greeting       string
	History        []model.ChatMessage
}

func newID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// conversation resolves the visitor's conversation: reuse the one named by
// the cookie if it still exists, otherwise start a new one and set the
// cookie. role/route from the query only apply to a freshly created
// conversation; a resumed conversation keeps the perspective it started with.
func (h *Handler) conversation(w http.ResponseWriter, r *http.Request, role, route string) (model.Conversation, error) {
	ctx := r.Context()
	if c, err := r.Cookie(cookieName); err == nil && c.Value != "" {
		if conv, err := h.deps.Store.GetConversation(ctx, c.Value); err == nil {
			return conv, nil
		}
	}
	conv := model.Conversation{ID: newID(), Role: role, Route: route, CreatedAt: time.Now()}
	created, err := h.deps.Store.CreateConversation(ctx, conv)
	if err != nil {
		return model.Conversation{}, err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    created.ID,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return created, nil
}

func (h *Handler) panel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	role := r.URL.Query().Get("role")
	if role == "" {
		role = "owner"
	}
	route := r.URL.Query().Get("route")

	conv, err := h.conversation(w, r, role, route)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	history, _ := h.deps.Store.ListMessages(ctx, conv.ID)
	turns := make([]model.ChatMessage, 0, len(history))
	for _, m := range history {
		if m.Role == "user" || m.Role == "assistant" {
			turns = append(turns, m)
		}
	}

	view := panelView{
		ConversationID: conv.ID,
		Role:           conv.Role,
		Route:          conv.Route,
		Model:          h.deps.LLMModel,
		Greeting:       "I am the Montage Manager assistant, running on the local cluster model. Ask me anything about this app — and tell me what is wrong with it. Every complaint becomes a backlog item.",
		History:        turns,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.tpl.ExecuteTemplate(w, "panel", view)
}

// SystemPrompt describes the app to itself, pulling in live state so the
// assistant can answer "what can I do here?", "what did other users complain
// about?" and "what are you going to fix next?" truthfully.
func SystemPrompt(ctx context.Context, deps app.Deps, role, route, conversationID string) string {
	lines := []string{
		"You are the in-app assistant of Montage Manager (mm.machinemachine.ai), a marketplace that connects Generalunternehmer (GU, project owners) with Subunternehmer (SU, subcontractors) for assembly and installation work in the DACH region, without broker margin leakage.",
		"The app is a Go + htmx server. Public routes: / (hero, assistant, perspectives, pipeline, modules, roadmap), /offers (pipeline partial, filter by status and free text), /perspective (role switch), /offers/new (create an offer), /assistant/* (this chat), /feedback (feedback capture), /dev (the development loop backlog), /healthz.",
		"Modules: AI Job Assistant, Team Builder, Document Safe, Status Documentation, Dispute Desk.",
		"You run on a local vLLM cluster; no user data leaves the fleet.",
		"Your second job matters as much as the first: collect honest feedback about this app. Ask what confused the user, what is missing, what broke. Be short, concrete and never salesy. Two or three sentences per answer.",
	}
	if deps.Version != "" {
		lines = append(lines, fmt.Sprintf("Running app version %s.", deps.Version))
	}
	if counts, err := deps.Store.CountOffersByStatus(ctx); err == nil {
		lines = append(lines, fmt.Sprintf("Live offer pipeline right now: %d total, %d open, %d requested, %d in process, %d done.",
			counts["all"], counts["open"], counts["requested"], counts["process"], counts["done"]))
	}
	if backlog, err := deps.Store.ListBacklog(ctx); err == nil && len(backlog) > 0 {
		top := backlog
		if len(top) > 3 {
			top = top[:3]
		}
		items := make([]string, 0, len(top))
		for _, b := range top {
			items = append(items, fmt.Sprintf("%q (%d reports, score %.1f)", b.Title, b.Count, b.Score))
		}
		lines = append(lines, "Top of the dev backlog right now, clustered from real user feedback: "+strings.Join(items, "; ")+". That is honestly what gets fixed next, in that order.")
	} else {
		lines = append(lines, "The dev backlog is currently empty: no feedback has been clustered yet.")
	}
	lines = append(lines, fmt.Sprintf("The visitor is currently in the %q perspective on route %q.", role, route))
	if conversationID != "" && !hasFeedback(ctx, deps, conversationID) {
		lines = append(lines, "This visitor has not given concrete feedback in this conversation yet. Unless their latest message already is feedback, close your answer with one short, specific question about what confused them, what is missing, or what broke.")
	}
	return strings.Join(lines, "\n")
}

// hasFeedback reports whether this conversation has already produced at
// least one feedback record, so SystemPrompt only nudges once.
func hasFeedback(ctx context.Context, deps app.Deps, conversationID string) bool {
	fb, err := deps.Store.ListFeedback(ctx, store.FeedbackFilter{Limit: 200})
	if err != nil {
		return true
	}
	for _, f := range fb {
		if f.ConversationID == conversationID {
			return true
		}
	}
	return false
}

// approxTokens is a cheap stand-in for a real tokenizer: roughly 4 chars/token.
func approxTokens(s string) int { return len(s)/4 + 1 }

// trimHistory keeps the most recent messages within budget, dropping the
// oldest turns first, and returns them back in chronological order.
func trimHistory(msgs []model.ChatMessage, budget int) []model.ChatMessage {
	kept := make([]model.ChatMessage, 0, len(msgs))
	total := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		t := approxTokens(msgs[i].Content)
		if total+t > budget && len(kept) > 0 {
			break
		}
		total += t
		kept = append(kept, msgs[i])
	}
	for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
		kept[i], kept[j] = kept[j], kept[i]
	}
	return kept
}

// chatMessages rebuilds the LLM request history from stored turns: a system
// prompt followed by the trimmed, ordered user/assistant exchange.
func chatMessages(ctx context.Context, deps app.Deps, convID, role, route string) []llm.Message {
	history, _ := deps.Store.ListMessages(ctx, convID)
	msgs := []llm.Message{{Role: "system", Content: SystemPrompt(ctx, deps, role, route, convID)}}
	for _, m := range trimHistory(history, historyTokenBudget) {
		if m.Role == "user" || m.Role == "assistant" {
			msgs = append(msgs, llm.Message{Role: m.Role, Content: m.Content})
		}
	}
	return msgs
}

// mineFeedback runs Extract on a detached context so a slow or failed
// extraction pass never delays or breaks the user-facing answer.
func (h *Handler) mineFeedback(conversationID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if _, err := Extract(ctx, h.deps, conversationID); err != nil {
		log.Printf("assistant: feedback extraction for %s failed: %v", conversationID, err)
	}
}

// message is the synchronous fallback: it persists both turns and returns
// the full answer in one response, for clients without SSE support.
func (h *Handler) message(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	convID := r.FormValue("conversation")
	role := r.FormValue("role")
	route := r.FormValue("route")
	text := strings.TrimSpace(r.FormValue("message"))
	if text == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	_, _ = h.deps.Store.AppendMessage(ctx, model.ChatMessage{ConversationID: convID, Role: "user", Content: text})

	msgs := chatMessages(ctx, h.deps, convID, role, route)

	answer := ""
	resp, err := h.deps.LLM.Chat(ctx, llm.Request{Messages: msgs, MaxTokens: 1024, Temperature: 0.4})
	switch {
	case err != nil:
		answer = "The local model did not answer: " + err.Error()
	case strings.TrimSpace(resp.Content) == "":
		answer = "The local model returned an empty answer (finish reason: " + resp.FinishReason + ")."
	default:
		answer = strings.TrimSpace(resp.Content)
		if _, err := h.deps.Store.AppendMessage(ctx, model.ChatMessage{ConversationID: convID, Role: "assistant", Content: answer, Reasoning: resp.Reasoning}); err == nil {
			go h.mineFeedback(convID)
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writeBubble(w, "user", text)
	writeBubble(w, "assistant", answer)
}

// turn is the SSE kick-off: it persists the user turn and returns the user
// bubble plus an assistant bubble wired to /assistant/stream via the htmx SSE
// extension, so the answer types itself in.
func (h *Handler) turn(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	convID := q.Get("conversation")
	role := q.Get("role")
	route := q.Get("route")
	text := strings.TrimSpace(q.Get("message"))
	if text == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if _, err := h.deps.Store.AppendMessage(ctx, model.ChatMessage{ConversationID: convID, Role: "user", Content: text}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	streamURL := "/assistant/stream?" + url.Values{
		"conversation": {convID},
		"role":         {role},
		"route":        {route},
	}.Encode()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writeBubble(w, "user", text)
	fmt.Fprintf(w, `<div class="mm-msg mm" hx-ext="sse" sse-connect="%s" sse-swap="message" sse-close="done" hx-target="find p" hx-swap="beforeend"><span class="mm-who">assistant</span><p></p></div>`,
		html.EscapeString(streamURL))
}

// stream answers a turn already persisted by turn as server-sent events: one
// "message" event per chunk, then a "done" event that closes the connection.
func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	convID := q.Get("conversation")
	role := q.Get("role")
	route := q.Get("route")

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

	if convID == "" || ctx.Err() != nil {
		return
	}

	msgs := chatMessages(ctx, h.deps, convID, role, route)

	var answer, reasoning strings.Builder
	streamErr := h.deps.LLM.Stream(ctx, llm.Request{Messages: msgs, MaxTokens: 1024, Temperature: 0.4}, func(d llm.Delta) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.Content != "" {
			answer.WriteString(d.Content)
			writeSSE(w, "message", html.EscapeString(d.Content))
			flusher.Flush()
		}
		if d.Reasoning != "" {
			reasoning.WriteString(d.Reasoning)
		}
		return nil
	})

	final := strings.TrimSpace(answer.String())
	if final == "" && streamErr != nil && ctx.Err() == nil {
		msg := "The local model did not answer: " + streamErr.Error()
		writeSSE(w, "message", html.EscapeString(msg))
		flusher.Flush()
		return
	}
	if final == "" {
		return
	}

	// Detach from the request context: the client may already be gone, but
	// the answer we just generated is still worth keeping.
	persistCtx := context.WithoutCancel(ctx)
	if _, err := h.deps.Store.AppendMessage(persistCtx, model.ChatMessage{ConversationID: convID, Role: "assistant", Content: final, Reasoning: reasoning.String()}); err == nil {
		go h.mineFeedback(convID)
	}
}

// writeSSE writes one server-sent event, splitting data on newlines as the
// SSE wire format requires.
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

// writeBubble renders one message in the shared thread primitives, so an
// assistant answer sits in the same conversation as onboarding and search.
func writeBubble(w http.ResponseWriter, role, text string) {
	fmt.Fprintf(w, `<div class="mm-msg %s"><span class="mm-who">%s</span><p>%s</p></div>`,
		bubbleClass(role), html.EscapeString(role), html.EscapeString(text))
}

func bubbleClass(role string) string {
	if role == "user" {
		return "you"
	}
	return "mm"
}

// Ask is the entry point behind the home prompt: it resolves (or starts) the
// visitor's conversation and answers in the thread, streaming. It exists so
// main.go can route a question about the app here without this package
// knowing anything about routing.
func (h *Handler) Ask(w http.ResponseWriter, r *http.Request) {
	text := strings.TrimSpace(firstNonEmpty(r.FormValue("message"), r.URL.Query().Get("message")))
	if text == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	role := firstNonEmpty(r.FormValue("role"), "owner")
	route := firstNonEmpty(r.FormValue("route"), "home")
	conv, err := h.conversation(w, r, role, route)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// turn reads its inputs from the query string; hand it a request carrying
	// the resolved conversation.
	q := url.Values{
		"conversation": {conv.ID},
		"role":         {role},
		"route":        {route},
		"message":      {text},
	}
	r2 := r.Clone(r.Context())
	r2.Method = http.MethodGet
	r2.URL.RawQuery = q.Encode()
	h.turn(w, r2)
}

// feedback stores an explicit piece of feedback from the widget.
func (h *Handler) feedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	text := strings.TrimSpace(r.FormValue("verbatim"))
	if text == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	fb := model.Feedback{
		ConversationID: r.FormValue("conversation"),
		Kind:           firstNonEmpty(r.FormValue("kind"), "request"),
		Theme:          strings.TrimSpace(r.FormValue("theme")),
		Severity:       3,
		Verbatim:       text,
		Route:          r.FormValue("route"),
		Role:           r.FormValue("role"),
		Source:         "widget",
		Status:         "new",
	}
	if _, err := h.deps.Store.CreateFeedback(r.Context(), fb); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<div class="feedback-thanks">Logged. It will show up in the <a href="/dev">dev loop</a>.</div>`)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// extractedFeedback is the shape the mining pass asks the model for.
type extractedFeedback struct {
	Kind      string `json:"kind"`
	Theme     string `json:"theme"`
	Severity  int    `json:"severity"`
	Verbatim  string `json:"verbatim"`
	Requested string `json:"requested"`
}

const extractionSchema = `[{"kind":"bug|confusion|request|praise","theme":"short-slug","severity":1-5,"verbatim":"the user's own words","requested":"the concrete change implied"}]`

// Extract is the hook the dev loop uses to mine a conversation for feedback.
// It runs a cheap LLM pass over the conversation's recent turns, never
// invents feedback (an empty array is the normal case), and skips anything
// that duplicates feedback already logged for this conversation.
func Extract(ctx context.Context, deps app.Deps, conversationID string) ([]model.Feedback, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, nil
	}
	history, err := deps.Store.ListMessages(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	turns := trimHistory(history, 1200)
	if len(turns) == 0 {
		return nil, nil
	}
	var transcript strings.Builder
	for _, m := range turns {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		fmt.Fprintf(&transcript, "%s: %s\n", m.Role, m.Content)
	}

	conv, _ := deps.Store.GetConversation(ctx, conversationID)

	// A single user turn, not a leading system message: this model's chat
	// template garbles output when a system message sandwiches a user
	// message before llm.Client.JSON appends its own trailing system
	// instruction (verified directly against the live endpoint).
	prompt := []llm.Message{
		{Role: "user", Content: "You mine app feedback out of a support chat transcript. Only extract feedback the USER actually expressed about the app itself: bugs, confusion, feature requests or praise. Never invent feedback the user did not give — if there is none, answer with an empty array. Each item's verbatim field must be the user's own words, not a paraphrase.\n\nTranscript:\n" + transcript.String()},
	}
	var extracted []extractedFeedback
	if err := deps.LLM.JSON(ctx, llm.Request{Messages: prompt, MaxTokens: 768, Temperature: 0.1}, extractionSchema, &extracted); err != nil {
		return nil, err
	}
	if len(extracted) == 0 {
		return nil, nil
	}

	existing, _ := deps.Store.ListFeedback(ctx, store.FeedbackFilter{Limit: 200})
	seen := map[string]bool{}
	for _, f := range existing {
		if f.ConversationID == conversationID {
			seen[dedupKey(f.Kind, f.Theme, f.Verbatim)] = true
		}
	}

	stored := make([]model.Feedback, 0, len(extracted))
	for _, e := range extracted {
		verbatim := strings.TrimSpace(e.Verbatim)
		if verbatim == "" {
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(e.Kind))
		switch kind {
		case "bug", "confusion", "request", "praise":
		default:
			kind = "request"
		}
		theme := strings.ToLower(strings.TrimSpace(e.Theme))
		key := dedupKey(kind, theme, verbatim)
		if seen[key] {
			continue
		}
		seen[key] = true

		severity := e.Severity
		if severity < 1 || severity > 5 {
			severity = 3
		}

		created, err := deps.Store.CreateFeedback(ctx, model.Feedback{
			ConversationID: conversationID,
			Kind:           kind,
			Theme:          theme,
			Severity:       severity,
			Verbatim:       verbatim,
			Requested:      strings.TrimSpace(e.Requested),
			Route:          conv.Route,
			Role:           conv.Role,
			Source:         "chat",
			Status:         "new",
		})
		if err != nil {
			return stored, err
		}
		stored = append(stored, created)
	}
	return stored, nil
}

func dedupKey(kind, theme, verbatim string) string {
	norm := strings.ToLower(strings.Join(strings.Fields(verbatim), " "))
	return kind + "|" + theme + "|" + norm
}
