// Package assistant is the app's self-aware chat surface: it answers questions
// about Montage Manager using the local model, and mines every conversation for
// feedback about the app itself, which the dev loop then turns into a backlog.
//
// This is the phase-0 baseline: synchronous chat plus explicit feedback capture.
// Streaming, conversation persistence and LLM feedback extraction are the
// assistant worker's job.
package assistant

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"strings"
	"time"

	"mm-machine/internal/app"
	"mm-machine/internal/llm"
	"mm-machine/internal/model"
)

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
}

func newID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (h *Handler) panel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	role := r.URL.Query().Get("role")
	if role == "" {
		role = "owner"
	}
	route := r.URL.Query().Get("route")
	conv := model.Conversation{ID: newID(), Role: role, Route: route, CreatedAt: time.Now()}
	if _, err := h.deps.Store.CreateConversation(ctx, conv); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	view := panelView{
		ConversationID: conv.ID,
		Role:           role,
		Route:          route,
		Model:          h.deps.LLMModel,
		Greeting:       "I am the Montage Manager assistant, running on the local cluster model. Ask me anything about this app — and tell me what is wrong with it. Every complaint becomes a backlog item.",
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.tpl.ExecuteTemplate(w, "panel", view)
}

// SystemPrompt describes the app to itself.
func SystemPrompt(role, route string) string {
	return strings.Join([]string{
		"You are the in-app assistant of Montage Manager (mm.machinemachine.ai), a marketplace that connects Generalunternehmer (GU, project owners) with Subunternehmer (SU, subcontractors) for assembly and installation work in the DACH region, without broker margin leakage.",
		"The app is a Go + htmx server. Public routes: / (hero, assistant, perspectives, pipeline, modules, roadmap), /offers (pipeline partial, filter by status and free text), /perspective (role switch), /offers/new (create an offer), /assistant/* (this chat), /feedback (feedback capture), /dev (the development loop backlog), /healthz.",
		"Modules: AI Job Assistant, Team Builder, Document Safe, Status Documentation, Dispute Desk.",
		"You run on a local vLLM cluster; no user data leaves the fleet.",
		"Your second job matters as much as the first: collect honest feedback about this app. Ask what confused the user, what is missing, what broke. Be short, concrete and never salesy. Two or three sentences per answer.",
		fmt.Sprintf("The visitor is currently in the %q perspective on route %q.", role, route),
	}, "\n")
}

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

	msgs := []llm.Message{{Role: "system", Content: SystemPrompt(role, route)}}
	history, _ := h.deps.Store.ListMessages(ctx, convID)
	for _, m := range history {
		if m.Role == "user" || m.Role == "assistant" {
			msgs = append(msgs, llm.Message{Role: m.Role, Content: m.Content})
		}
	}

	answer := ""
	resp, err := h.deps.LLM.Chat(ctx, llm.Request{Messages: msgs, MaxTokens: 1024, Temperature: 0.4})
	switch {
	case err != nil:
		answer = "The local model did not answer: " + err.Error()
	case strings.TrimSpace(resp.Content) == "":
		answer = "The local model returned an empty answer (finish reason: " + resp.FinishReason + ")."
	default:
		answer = strings.TrimSpace(resp.Content)
		_, _ = h.deps.Store.AppendMessage(ctx, model.ChatMessage{ConversationID: convID, Role: "assistant", Content: answer, Reasoning: resp.Reasoning})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writeBubble(w, "user", text)
	writeBubble(w, "assistant", answer)
}

func writeBubble(w http.ResponseWriter, role, text string) {
	fmt.Fprintf(w, `<div class="bubble %s"><span class="who">%s</span><p>%s</p></div>`,
		html.EscapeString(role), html.EscapeString(role), html.EscapeString(text))
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

// Extract is the hook the dev loop uses to mine a conversation for feedback.
// The assistant worker replaces this with a real LLM extraction pass.
func Extract(ctx context.Context, deps app.Deps, conversationID string) ([]model.Feedback, error) {
	_ = ctx
	_ = deps
	_ = conversationID
	return nil, nil
}
