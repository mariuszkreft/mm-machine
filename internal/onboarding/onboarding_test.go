package onboarding

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

func newTestHandler(l *fakeLLM) (*Handler, *http.ServeMux, store.Store) {
	db := store.NewMemory()
	deps := app.Deps{Store: db, LLM: l, Version: "test-1", LLMModel: "fake-model"}
	mux := http.NewServeMux()
	h := Register(mux, deps)
	return h, mux, db
}

func doForm(mux *http.ServeMux, method, path string, form url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	var req *http.Request
	if method == http.MethodGet {
		req = httptest.NewRequest(method, path+"?"+form.Encode(), nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func profileCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == CookieName {
			return c
		}
	}
	return nil
}

// --- 1. dense-sentence extraction fills multiple fields ---------------------

func TestExtractDenseSentenceFillsMultipleFields(t *testing.T) {
	fake := &fakeLLM{JSONFunc: func(ctx context.Context, req llm.Request, schemaHint string, out any) error {
		if !strings.Contains(req.Messages[1].Content, "electrical") {
			t.Fatalf("expected the raw message forwarded to the model, got: %s", req.Messages[1].Content)
		}
		return json.Unmarshal([]byte(`{
			"role": "executor",
			"trades": ["electrical", "drywall"],
			"regions": ["Munich, DE"],
			"crewSize": 8,
			"documents": ["a1", "insurance"],
			"availability": "free from October"
		}`), out)
	}}
	deps := app.Deps{LLM: fake}

	p := model.Profile{ID: "p1", Role: "unknown"}
	got := Extract(context.Background(), deps, p,
		"we're 8 guys doing electrical and drywall around Munich, A1 and insurance done, free from October")

	if got.Role != "executor" {
		t.Fatalf("role = %q, want executor", got.Role)
	}
	if len(got.Trades) != 2 || got.Trades[0] != "electrical" || got.Trades[1] != "drywall" {
		t.Fatalf("trades = %v", got.Trades)
	}
	if len(got.Regions) != 1 || got.Regions[0] != "munich, de" {
		t.Fatalf("regions = %v", got.Regions)
	}
	if got.CrewSize != 8 {
		t.Fatalf("crewSize = %d, want 8", got.CrewSize)
	}
	if len(got.Documents) != 2 {
		t.Fatalf("documents = %v", got.Documents)
	}
	if got.Availability != "free from October" {
		t.Fatalf("availability = %q", got.Availability)
	}

	// One sentence should be enough: every field the sentence supported is
	// now known, so there is nothing left to ask.
	if _, done := NextQuestion(got); !done {
		t.Fatalf("expected onboarding done once every field is filled from one sentence: %+v", got)
	}
}

func TestTurnFillsProfileFromOneMessageAndSetsCookieOnce(t *testing.T) {
	fake := &fakeLLM{JSONFunc: func(ctx context.Context, req llm.Request, schemaHint string, out any) error {
		return json.Unmarshal([]byte(`{
			"role": "executor",
			"trades": ["electrical", "drywall"],
			"regions": ["Munich, DE"],
			"crewSize": 8,
			"documents": ["a1", "insurance"],
			"availability": "free from October"
		}`), out)
	}}
	_, mux, db := newTestHandler(fake)

	first := doForm(mux, http.MethodPost, "/start/turn", url.Values{
		"message": {"we're 8 guys doing electrical and drywall around Munich, A1 and insurance done, free from October"},
	})
	if first.Code != http.StatusOK {
		t.Fatalf("turn status = %d, body=%s", first.Code, first.Body.String())
	}
	cookie := profileCookie(first)
	if cookie == nil {
		t.Fatalf("expected the profile cookie to be set on first contact")
	}

	// A second turn, replaying the cookie, must not set it again (reused, not
	// re-issued) and must build on the same profile.
	second := doForm(mux, http.MethodPost, "/start/turn", url.Values{"message": {"hello again"}}, cookie)
	if c := profileCookie(second); c != nil {
		t.Fatalf("cookie was re-set on a request that already carried it: %+v", c)
	}

	req := httptest.NewRequest(http.MethodGet, "/start/profile", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "electrical") || !strings.Contains(body, "8 people") {
		t.Fatalf("profile panel missing learned facts: %s", body)
	}

	p, err := db.GetProfile(context.Background(), cookie.Value)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if p.CrewSize != 8 || p.Role != "executor" {
		t.Fatalf("stored profile = %+v", p)
	}
}

// --- 2. correction replaces rather than appends ------------------------------

func TestCorrectionReplacesRatherThanAppends(t *testing.T) {
	calls := 0
	fake := &fakeLLM{JSONFunc: func(ctx context.Context, req llm.Request, schemaHint string, out any) error {
		calls++
		if calls == 1 {
			return json.Unmarshal([]byte(`{"role":"executor","trades":["electrical"]}`), out)
		}
		// The correction pass: remove electrical, add sanitary.
		if !strings.Contains(schemaHint, `"remove"`) {
			t.Fatalf("expected the correction schema on the second call, got %q", schemaHint)
		}
		return json.Unmarshal([]byte(`{"remove":{"trades":["electrical"]},"add":{"trades":["sanitary"]}}`), out)
	}}
	_, mux, db := newTestHandler(fake)

	first := doForm(mux, http.MethodPost, "/start/turn", url.Values{"message": {"we do electrical work"}})
	cookie := profileCookie(first)
	if cookie == nil {
		t.Fatalf("expected cookie after first turn")
	}

	doForm(mux, http.MethodPost, "/start/turn", url.Values{"message": {"no, we do sanitary, not electrical"}}, cookie)

	p, err := db.GetProfile(context.Background(), cookie.Value)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if len(p.Trades) != 1 || p.Trades[0] != "sanitary" {
		t.Fatalf("trades after correction = %v, want exactly [sanitary]", p.Trades)
	}
}

func TestExtractCorrectionFunctionRemovesAndReplacesDirectly(t *testing.T) {
	fake := &fakeLLM{JSONFunc: func(ctx context.Context, req llm.Request, schemaHint string, out any) error {
		return json.Unmarshal([]byte(`{"remove":{"trades":["electrical"],"documents":[]},"add":{"trades":["sanitary"]}}`), out)
	}}
	deps := app.Deps{LLM: fake}
	p := model.Profile{ID: "p1", Role: "executor", Trades: []string{"electrical", "drywall"}}

	got := ExtractCorrection(context.Background(), deps, p, "no, we do sanitary, not electrical")

	want := map[string]bool{"drywall": true, "sanitary": true}
	if len(got.Trades) != 2 {
		t.Fatalf("trades = %v, want 2 entries", got.Trades)
	}
	for _, tr := range got.Trades {
		if !want[tr] {
			t.Fatalf("unexpected trade %q survived correction: %v", tr, got.Trades)
		}
	}
	for _, tr := range got.Trades {
		if tr == "electrical" {
			t.Fatalf("electrical should have been removed, got %v", got.Trades)
		}
	}
}

func TestIsCorrectionDetectsCorrectionLanguage(t *testing.T) {
	cases := map[string]bool{
		"no, we do sanitary, not electrical": true,
		"actually it's Berlin not Munich":    true,
		"we also do hvac":                    false,
		"we're 8 guys doing electrical":      false,
	}
	for msg, want := range cases {
		if got := isCorrection(msg); got != want {
			t.Errorf("isCorrection(%q) = %v, want %v", msg, got, want)
		}
	}
}

// --- 3. NextQuestion never repeats a known field -----------------------------

func TestNextQuestionNeverRepeatsAKnownField(t *testing.T) {
	p := model.Profile{
		Role:         "executor",
		Trades:       []string{"electrical"},
		Regions:      []string{"munich, de"},
		CrewSize:     8,
		Documents:    []string{"a1"},
		Availability: "from October",
	}
	q, done := NextQuestion(p)
	if !done {
		t.Fatalf("expected done once every field is filled, got question %q", q)
	}

	p.Availability = ""
	q, done = NextQuestion(p)
	if done || !strings.Contains(strings.ToLower(q), "when") {
		t.Fatalf("expected only the availability question, got done=%v q=%q", done, q)
	}

	// Filled fields must never resurface as a question, however NextQuestion
	// is walked.
	for _, field := range []string{"role", "trades", "regions", "crewSize", "documents"} {
		_ = field
		if strings.Contains(strings.ToLower(q), "region") || strings.Contains(strings.ToLower(q), "trade") {
			t.Fatalf("question re-asked a known field: %q", q)
		}
	}
}

func TestNextQuestionOffersFinishAfterTwoSkippedTurns(t *testing.T) {
	fake := &fakeLLM{JSONFunc: func(ctx context.Context, req llm.Request, schemaHint string, out any) error {
		// Never extracts anything new, however earnestly the visitor answers.
		return json.Unmarshal([]byte(`{}`), out)
	}}
	_, mux, _ := newTestHandler(fake)

	first := doForm(mux, http.MethodPost, "/start/turn", url.Values{"message": {"hard to say really"}})
	cookie := profileCookie(first)
	if strings.Contains(first.Body.String(), "that's enough") {
		t.Fatalf("should not offer to finish after only one skipped turn: %s", first.Body.String())
	}

	second := doForm(mux, http.MethodPost, "/start/turn", url.Values{"message": {"not sure honestly"}}, cookie)
	if !strings.Contains(second.Body.String(), "that's enough") {
		t.Fatalf("expected an offer to finish after two skipped turns: %s", second.Body.String())
	}
}

// --- SSE endpoint emits and terminates ---------------------------------------

func TestStreamEmitsChunksAndTerminates(t *testing.T) {
	fake := &fakeLLM{StreamFunc: func(ctx context.Context, req llm.Request, fn func(llm.Delta) error) error {
		for _, chunk := range []string{"Got ", "it, ", "8 electricians in Munich."} {
			if err := fn(llm.Delta{Content: chunk}); err != nil {
				return err
			}
		}
		return nil
	}}
	_, mux, _ := newTestHandler(fake)

	rec := doForm(mux, http.MethodGet, "/start/stream", url.Values{"hint": {"trades: electrical; crew size: 8"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("stream status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content-type = %q", ct)
	}
	body := rec.Body.String()
	if strings.Count(body, "event: message") != 3 {
		t.Fatalf("expected 3 message events, got: %s", body)
	}
	if !strings.HasSuffix(strings.TrimRight(body, "\n"), "event: done\ndata: ") {
		t.Fatalf("stream did not terminate with a done event: %s", body)
	}
}

func TestStreamHonoursClientDisconnect(t *testing.T) {
	fake := &fakeLLM{StreamFunc: func(ctx context.Context, req llm.Request, fn func(llm.Delta) error) error {
		return fn(llm.Delta{Content: "should not be sent"})
	}}
	_, mux, _ := newTestHandler(fake)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/start/stream?hint=trades%3A+electrical", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if fake.streamCalls != 0 {
		t.Fatalf("expected LLM.Stream not to be called for a disconnected client, got %d calls", fake.streamCalls)
	}
}

func TestStreamFallsBackWithoutHint(t *testing.T) {
	fake := &fakeLLM{}
	_, mux, _ := newTestHandler(fake)

	rec := doForm(mux, http.MethodGet, "/start/stream", url.Values{})
	if rec.Code != http.StatusOK {
		t.Fatalf("stream status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "event: done") {
		t.Fatalf("expected the stream to still close cleanly: %s", rec.Body.String())
	}
	if fake.streamCalls != 0 {
		t.Fatalf("expected no model call with an empty hint, got %d", fake.streamCalls)
	}
}

// --- HTML escaping ------------------------------------------------------------

func TestHTMLEscapingOfVisitorInput(t *testing.T) {
	fake := &fakeLLM{JSONFunc: func(ctx context.Context, req llm.Request, schemaHint string, out any) error {
		return json.Unmarshal([]byte(`{"company":"<b>Acme</b>"}`), out)
	}}
	_, mux, _ := newTestHandler(fake)

	message := `<script>alert(1)</script> we do electrical`
	rec := doForm(mux, http.MethodPost, "/start/turn", url.Values{"message": {message}})
	body := rec.Body.String()
	if strings.Contains(body, "<script>") {
		t.Fatalf("visitor message was not escaped: %s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Fatalf("expected the escaped echo of the message: %s", body)
	}

	cookie := profileCookie(rec)
	req := httptest.NewRequest(http.MethodGet, "/start/profile", nil)
	req.AddCookie(cookie)
	prec := httptest.NewRecorder()
	mux.ServeHTTP(prec, req)
	if strings.Contains(prec.Body.String(), "<b>Acme</b>") {
		t.Fatalf("stored company field rendered unescaped: %s", prec.Body.String())
	}
}

func TestProfileEditEscapesInput(t *testing.T) {
	_, mux, _ := newTestHandler(&fakeLLM{})
	first := doForm(mux, http.MethodPost, "/start/turn", url.Values{"message": {""}})
	cookie := profileCookie(first)

	rec := doForm(mux, http.MethodPost, "/start/profile/edit", url.Values{
		"field": {"availability"},
		"op":    {"set"},
		"value": {`<img src=x onerror=alert(1)>`},
	}, cookie)
	if strings.Contains(rec.Body.String(), "<img") {
		t.Fatalf("edited value was not escaped: %s", rec.Body.String())
	}
}

// --- ApplyEdit ----------------------------------------------------------------

func TestApplyEditRemovesAndSets(t *testing.T) {
	p := model.Profile{Role: "executor", Trades: []string{"electrical", "drywall"}, Regions: []string{"munich, de"}, CrewSize: 4}

	p = ApplyEdit(p, "trades", "remove", "electrical")
	if len(p.Trades) != 1 || p.Trades[0] != "drywall" {
		t.Fatalf("trades after remove = %v", p.Trades)
	}

	p = ApplyEdit(p, "regions", "set", "Berlin, DE")
	if len(p.Regions) != 1 || p.Regions[0] != "Berlin, DE" {
		t.Fatalf("regions after set = %v", p.Regions)
	}

	p = ApplyEdit(p, "crewSize", "set", "12")
	if p.CrewSize != 12 {
		t.Fatalf("crewSize after set = %d", p.CrewSize)
	}

	p = ApplyEdit(p, "crewSize", "remove", "")
	if p.CrewSize != 0 {
		t.Fatalf("crewSize after remove = %d", p.CrewSize)
	}
}

// --- mechanical fallback, no live model ---------------------------------------

func TestMechanicalFallbackWhenLLMUnavailable(t *testing.T) {
	deps := app.Deps{LLM: nil}
	p := model.Profile{ID: "p1", Role: "unknown"}
	got := Extract(context.Background(), deps, p, "we offer electrical and drywall work, A1 and insurance ready")
	if got.Role != "executor" {
		t.Fatalf("mechanical role = %q", got.Role)
	}
	if len(got.Trades) != 2 {
		t.Fatalf("mechanical trades = %v", got.Trades)
	}
	if len(got.Documents) != 2 {
		t.Fatalf("mechanical documents = %v", got.Documents)
	}
}

// --- live cluster smoke test (guarded, not run in -short) ---------------------

func TestLiveExtraction(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live-cluster test in -short mode")
	}
	client := llm.New(llm.Config{
		BaseURL:   "http://192.168.31.90:8000/v1",
		Model:     "deepseek-v4-flash-0731",
		APIKey:    "local",
		MaxTokens: 1024,
	})
	deps := app.Deps{LLM: client}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := client.Health(ctx); err != nil {
		t.Skipf("live cluster unreachable: %v", err)
	}
	p := model.Profile{ID: "live", Role: "unknown"}
	got := Extract(ctx, deps, p, "we're 8 guys doing electrical and drywall around Munich, A1 and insurance done, free from October")
	t.Logf("live extraction result: %+v", got)
	if got.Role != "owner" && got.Role != "executor" {
		t.Fatalf("live model did not infer a role: %+v", got)
	}
}
