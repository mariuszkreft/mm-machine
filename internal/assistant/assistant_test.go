package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"mm-machine/internal/app"
	"mm-machine/internal/i18n"
	"mm-machine/internal/llm"
	"mm-machine/internal/model"
	"mm-machine/internal/store"
)

// fakeLLM is a controllable llm.Client that never touches the network.
type fakeLLM struct {
	ChatFunc   func(ctx context.Context, req llm.Request) (llm.Response, error)
	StreamFunc func(ctx context.Context, req llm.Request, fn func(llm.Delta) error) error
	JSONFunc   func(ctx context.Context, req llm.Request, schemaHint string, out any) error

	streamCalls int
}

func (f *fakeLLM) Chat(ctx context.Context, req llm.Request) (llm.Response, error) {
	if f.ChatFunc != nil {
		return f.ChatFunc(ctx, req)
	}
	return llm.Response{}, errors.New("fakeLLM: Chat not configured")
}

func (f *fakeLLM) Stream(ctx context.Context, req llm.Request, fn func(llm.Delta) error) error {
	f.streamCalls++
	if f.StreamFunc != nil {
		return f.StreamFunc(ctx, req, fn)
	}
	return errors.New("fakeLLM: Stream not configured")
}

func (f *fakeLLM) JSON(ctx context.Context, req llm.Request, schemaHint string, out any) error {
	if f.JSONFunc != nil {
		return f.JSONFunc(ctx, req, schemaHint, out)
	}
	return errors.New("fakeLLM: JSON not configured")
}

func (f *fakeLLM) Health(ctx context.Context) error { return nil }
func (f *fakeLLM) Model() string                    { return "fake-model" }

var _ llm.Client = (*fakeLLM)(nil)

func newTestHandler(t *testing.T, l *fakeLLM) (*Handler, *http.ServeMux, store.Store) {
	t.Helper()
	db := store.NewMemory()
	deps := app.Deps{Store: db, LLM: l, Version: "test-1", LLMModel: "fake-model"}
	mux := http.NewServeMux()
	h := Register(mux, deps)
	return h, mux, db
}

func doForm(mux *http.ServeMux, method, path string, form url.Values) *httptest.ResponseRecorder {
	var req *http.Request
	if method == http.MethodGet {
		req = httptest.NewRequest(method, path+"?"+form.Encode(), nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestPanelRenders(t *testing.T) {
	_, mux, _ := newTestHandler(t, &fakeLLM{})
	req := httptest.NewRequest(http.MethodGet, "/assistant/panel?role=owner&route=%2F", nil)
	req.AddCookie(&http.Cookie{Name: i18n.CookieName, Value: "en"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("panel status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "assistant-panel") {
		t.Fatalf("panel body missing assistant-panel: %s", body)
	}
	if !strings.Contains(body, "Montage Manager assistant") {
		t.Fatalf("panel body missing greeting: %s", body)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != cookieName || cookies[0].Value == "" {
		t.Fatalf("expected a conversation cookie to be set, got %+v", cookies)
	}
}

func TestPanelRendersExistingHistory(t *testing.T) {
	_, mux, db := newTestHandler(t, &fakeLLM{})
	conv, err := db.CreateConversation(context.Background(), model.Conversation{ID: "conv-1", Role: "owner", Route: "/", CreatedAt: time.Now()})
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if _, err := db.AppendMessage(context.Background(), model.ChatMessage{ConversationID: conv.ID, Role: "user", Content: "hello there"}); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if _, err := db.AppendMessage(context.Background(), model.ChatMessage{ConversationID: conv.ID, Role: "assistant", Content: "hi, how can I help"}); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/assistant/panel", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: conv.ID})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "hello there") || !strings.Contains(body, "hi, how can I help") {
		t.Fatalf("panel did not render existing history: %s", body)
	}
	if strings.Contains(body, "running on the local cluster model") {
		t.Fatalf("panel showed the greeting even though history exists: %s", body)
	}
}

func TestMessageRoundTripPersistsBothTurns(t *testing.T) {
	fake := &fakeLLM{ChatFunc: func(ctx context.Context, req llm.Request) (llm.Response, error) {
		return llm.Response{Content: "Sure, here is how offers work.", FinishReason: "stop"}, nil
	}}
	_, mux, db := newTestHandler(t, fake)

	rec := doForm(mux, http.MethodPost, "/assistant/message", url.Values{
		"conversation": {"conv-x"},
		"role":         {"owner"},
		"route":        {"/"},
		"message":      {"how do offers work?"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("message status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Sure, here is how offers work.") {
		t.Fatalf("response missing assistant answer: %s", rec.Body.String())
	}

	msgs, err := db.ListMessages(context.Background(), "conv-x")
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 persisted turns, got %d: %+v", len(msgs), msgs)
	}
	if msgs[0].Role != "user" || msgs[0].Content != "how do offers work?" {
		t.Fatalf("unexpected user turn: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "Sure, here is how offers work." {
		t.Fatalf("unexpected assistant turn: %+v", msgs[1])
	}
}

func TestStreamEmitsChunksAndTerminates(t *testing.T) {
	fake := &fakeLLM{StreamFunc: func(ctx context.Context, req llm.Request, fn func(llm.Delta) error) error {
		for _, chunk := range []string{"Hel", "lo, ", "world."} {
			if err := fn(llm.Delta{Content: chunk}); err != nil {
				return err
			}
		}
		return fn(llm.Delta{Done: true})
	}}
	_, mux, db := newTestHandler(t, fake)

	turn := doForm(mux, http.MethodGet, "/assistant/turn", url.Values{
		"conversation": {"conv-s"},
		"role":         {"owner"},
		"route":        {"/"},
		"message":      {"stream please"},
	})
	if turn.Code != http.StatusOK || !strings.Contains(turn.Body.String(), "sse-connect") {
		t.Fatalf("turn response missing sse wiring: status=%d body=%s", turn.Code, turn.Body.String())
	}

	rec := doForm(mux, http.MethodGet, "/assistant/stream", url.Values{
		"conversation": {"conv-s"},
		"role":         {"owner"},
		"route":        {"/"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("stream status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("stream content-type = %q", ct)
	}
	body := rec.Body.String()
	if strings.Count(body, "event: message") != 3 {
		t.Fatalf("expected 3 message events, got body: %s", body)
	}
	if !strings.Contains(body, "Hel") || !strings.Contains(body, "lo, ") || !strings.Contains(body, "world.") {
		t.Fatalf("stream body missing chunks: %s", body)
	}
	if !strings.HasSuffix(strings.TrimRight(body, "\n"), "event: done\ndata: ") {
		t.Fatalf("stream did not terminate with a done event: %s", body)
	}

	msgs, err := db.ListMessages(context.Background(), "conv-s")
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(msgs) != 2 || msgs[1].Role != "assistant" || msgs[1].Content != "Hello, world." {
		t.Fatalf("assistant turn not persisted after stream completed: %+v", msgs)
	}
}

func TestStreamHonoursClientDisconnect(t *testing.T) {
	fake := &fakeLLM{StreamFunc: func(ctx context.Context, req llm.Request, fn func(llm.Delta) error) error {
		return fn(llm.Delta{Content: "should not be sent"})
	}}
	_, mux, db := newTestHandler(t, fake)

	// Persist the user turn as /assistant/turn would, then request the
	// stream with an already-cancelled context, simulating a client that
	// left before the answer started.
	if _, err := db.AppendMessage(context.Background(), model.ChatMessage{ConversationID: "conv-gone", Role: "user", Content: "hi"}); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/assistant/stream?conversation=conv-gone&role=owner&route=%2F", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if fake.streamCalls != 0 {
		t.Fatalf("expected LLM.Stream not to be called for a disconnected client, got %d calls", fake.streamCalls)
	}
	msgs, _ := db.ListMessages(context.Background(), "conv-gone")
	if len(msgs) != 1 {
		t.Fatalf("expected only the user turn to be persisted, got %+v", msgs)
	}
}

func TestHTMLEscaping(t *testing.T) {
	fake := &fakeLLM{ChatFunc: func(ctx context.Context, req llm.Request) (llm.Response, error) {
		return llm.Response{Content: "<b>bold</b> answer", FinishReason: "stop"}, nil
	}}
	_, mux, _ := newTestHandler(t, fake)

	rec := doForm(mux, http.MethodPost, "/assistant/message", url.Values{
		"conversation": {"conv-esc"},
		"role":         {"owner"},
		"route":        {"/"},
		"message":      {"<script>alert(1)</script>"},
	})
	body := rec.Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatalf("user input was not escaped: %s", body)
	}
	if strings.Contains(body, "<b>bold</b>") {
		t.Fatalf("assistant answer was not escaped: %s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") || !strings.Contains(body, "&lt;b&gt;bold&lt;/b&gt;") {
		t.Fatalf("expected escaped entities in body: %s", body)
	}
}

func TestExtractStoresWhatTheLLMReturns(t *testing.T) {
	fake := &fakeLLM{JSONFunc: func(ctx context.Context, req llm.Request, schemaHint string, out any) error {
		return json.Unmarshal([]byte(`[{"kind":"bug","theme":"upload-fails","severity":4,"verbatim":"the upload button does nothing","requested":"make it submit the file"}]`), out)
	}}
	db := store.NewMemory()
	deps := app.Deps{Store: db, LLM: fake, Version: "test-1", LLMModel: "fake-model"}
	ctx := context.Background()

	if _, err := db.CreateConversation(ctx, model.Conversation{ID: "conv-fb", Role: "owner", Route: "/", CreatedAt: time.Now()}); err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if _, err := db.AppendMessage(ctx, model.ChatMessage{ConversationID: "conv-fb", Role: "user", Content: "the upload button does nothing"}); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if _, err := db.AppendMessage(ctx, model.ChatMessage{ConversationID: "conv-fb", Role: "assistant", Content: "sorry to hear that, can you say more?"}); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}

	got, err := Extract(ctx, deps, "conv-fb")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 extracted feedback, got %d: %+v", len(got), got)
	}
	if got[0].Kind != "bug" || got[0].Verbatim != "the upload button does nothing" || got[0].Source != "chat" {
		t.Fatalf("unexpected feedback record: %+v", got[0])
	}

	stored, err := db.ListFeedback(ctx, store.FeedbackFilter{})
	if err != nil {
		t.Fatalf("ListFeedback: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("expected 1 stored feedback record, got %d", len(stored))
	}
}

func TestExtractStoresNothingForEmptyArray(t *testing.T) {
	fake := &fakeLLM{JSONFunc: func(ctx context.Context, req llm.Request, schemaHint string, out any) error {
		return json.Unmarshal([]byte(`[]`), out)
	}}
	db := store.NewMemory()
	deps := app.Deps{Store: db, LLM: fake, Version: "test-1", LLMModel: "fake-model"}
	ctx := context.Background()

	if _, err := db.CreateConversation(ctx, model.Conversation{ID: "conv-empty", Role: "owner", Route: "/", CreatedAt: time.Now()}); err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if _, err := db.AppendMessage(ctx, model.ChatMessage{ConversationID: "conv-empty", Role: "user", Content: "this app is great"}); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}

	got, err := Extract(ctx, deps, "conv-empty")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no extracted feedback, got %+v", got)
	}
	stored, err := db.ListFeedback(ctx, store.FeedbackFilter{})
	if err != nil {
		t.Fatalf("ListFeedback: %v", err)
	}
	if len(stored) != 0 {
		t.Fatalf("expected no stored feedback, got %d", len(stored))
	}
}

func TestExtractDeduplicatesAgainstRecentFeedback(t *testing.T) {
	fake := &fakeLLM{JSONFunc: func(ctx context.Context, req llm.Request, schemaHint string, out any) error {
		return json.Unmarshal([]byte(`[{"kind":"bug","theme":"upload-fails","severity":4,"verbatim":"the upload button does nothing","requested":"fix it"}]`), out)
	}}
	db := store.NewMemory()
	deps := app.Deps{Store: db, LLM: fake, Version: "test-1", LLMModel: "fake-model"}
	ctx := context.Background()

	if _, err := db.CreateConversation(ctx, model.Conversation{ID: "conv-dup", Role: "owner", Route: "/", CreatedAt: time.Now()}); err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if _, err := db.AppendMessage(ctx, model.ChatMessage{ConversationID: "conv-dup", Role: "user", Content: "the upload button does nothing"}); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if _, err := db.CreateFeedback(ctx, model.Feedback{ConversationID: "conv-dup", Kind: "bug", Theme: "upload-fails", Verbatim: "the upload button does nothing", Source: "chat", Status: "new"}); err != nil {
		t.Fatalf("CreateFeedback: %v", err)
	}

	got, err := Extract(ctx, deps, "conv-dup")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected duplicate feedback to be skipped, got %+v", got)
	}
}

// --- required: German feedback extraction stores a German verbatim ----------

func TestGermanFeedbackExtractionStoresGermanVerbatim(t *testing.T) {
	germanVerbatim := "der Upload-Knopf tut gar nichts"
	fake := &fakeLLM{JSONFunc: func(ctx context.Context, req llm.Request, schemaHint string, out any) error {
		if !strings.Contains(req.Messages[0].Content, "German") {
			t.Fatalf("expected the extraction prompt to call out German transcripts, got: %s", req.Messages[0].Content)
		}
		return json.Unmarshal([]byte(`[{"kind":"bug","theme":"upload-fails","severity":4,"verbatim":"`+germanVerbatim+`","requested":"den Upload-Knopf reparieren"}]`), out)
	}}
	db := store.NewMemory()
	deps := app.Deps{Store: db, LLM: fake, Version: "test-1", LLMModel: "fake-model"}
	ctx := context.Background()

	if _, err := db.CreateConversation(ctx, model.Conversation{ID: "conv-de", Role: "owner", Route: "/", CreatedAt: time.Now()}); err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if _, err := db.AppendMessage(ctx, model.ChatMessage{ConversationID: "conv-de", Role: "user", Content: germanVerbatim}); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}

	got, err := Extract(ctx, deps, "conv-de")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(got) != 1 || got[0].Verbatim != germanVerbatim {
		t.Fatalf("expected the German verbatim to be stored unchanged, got %+v", got)
	}

	stored, err := db.ListFeedback(ctx, store.FeedbackFilter{})
	if err != nil {
		t.Fatalf("ListFeedback: %v", err)
	}
	if len(stored) != 1 || stored[0].Verbatim != germanVerbatim {
		t.Fatalf("expected 1 stored German feedback record, got %+v", stored)
	}
}

// --- required: AnswerIn reaches the model request ----------------------------

func TestAnswerInReachesTheModelRequest(t *testing.T) {
	var captured llm.Request
	fake := &fakeLLM{ChatFunc: func(ctx context.Context, req llm.Request) (llm.Response, error) {
		captured = req
		return llm.Response{Content: "Verstanden.", FinishReason: "stop"}, nil
	}}
	_, mux, _ := newTestHandler(t, fake)

	req := httptest.NewRequest(http.MethodPost, "/assistant/message", strings.NewReader(url.Values{
		"conversation": {"conv-lang"},
		"role":         {"owner"},
		"route":        {"/"},
		"message":      {"was kann ich hier machen?"},
	}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: i18n.CookieName, Value: "de"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("message status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(captured.Messages) == 0 || captured.Messages[0].Role != "system" {
		t.Fatalf("expected a leading system message, got: %+v", captured.Messages)
	}
	if !strings.Contains(captured.Messages[0].Content, i18n.AnswerIn(i18n.DE)) {
		t.Fatalf("expected i18n.AnswerIn(de) in the system prompt, got: %s", captured.Messages[0].Content)
	}
}

// --- SystemPrompt describes the app to itself --------------------------------

func TestSystemPromptIncludesCorpusPersonasAndLanguage(t *testing.T) {
	db := store.NewMemory()
	deps := app.Deps{Store: db, Version: "test-1", LLMModel: "fake-model"}
	ctx := context.Background()

	prompt := SystemPrompt(ctx, deps, "owner", "/", "", i18n.DE)

	if !strings.Contains(prompt, "München") {
		t.Fatalf("expected an example persona (Munich GU) in the system prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, "offers") || !strings.Contains(prompt, "crews") {
		t.Fatalf("expected the corpus size (offers/crews) in the system prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, "/dev") {
		t.Fatalf("expected the feedback loop to mention the /dev backlog, got: %s", prompt)
	}
	if !strings.Contains(prompt, i18n.AnswerIn(i18n.DE)) {
		t.Fatalf("expected i18n.AnswerIn(de) at the end of the system prompt, got: %s", prompt)
	}
}
