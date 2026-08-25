package search

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"mm-machine/internal/app"
	"mm-machine/internal/llm"
	"mm-machine/internal/model"
	"mm-machine/internal/store"
)

// fakeLLM is a controllable llm.Client that never touches the network,
// matching the pattern used by the other packages' tests.
type fakeLLM struct {
	JSONFunc   func(ctx context.Context, req llm.Request, schemaHint string, out any) error
	StreamFunc func(ctx context.Context, req llm.Request, fn func(llm.Delta) error) error
}

func (f *fakeLLM) Chat(context.Context, llm.Request) (llm.Response, error) {
	return llm.Response{}, errors.New("fakeLLM: Chat not configured")
}

func (f *fakeLLM) Stream(ctx context.Context, req llm.Request, fn func(llm.Delta) error) error {
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

func (f *fakeLLM) Health(context.Context) error { return nil }
func (f *fakeLLM) Model() string                { return "fake-model" }

var _ llm.Client = (*fakeLLM)(nil)

func newTestHandler(t *testing.T, l llm.Client) (*Handler, *http.ServeMux, store.Store) {
	t.Helper()
	db := store.NewMemory()
	deps := app.Deps{Store: db, LLM: l, Version: "test", LLMModel: "fake-model"}
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

// --- intent parsing -----------------------------------------------------------

func TestParseUsesModelDatesAndNegation(t *testing.T) {
	fake := &fakeLLM{JSONFunc: func(_ context.Context, _ llm.Request, _ string, out any) error {
		// The model heard "Vienna" even though the sentence negates it —
		// finishParse must strip it back out regardless of what came back.
		raw := `{"kind":"find_offers","trades":["energy"],"regions":["Vienna"],"confidence":0.9}`
		return json.Unmarshal([]byte(raw), out)
	}}
	deps := app.Deps{LLM: fake}
	intent, fc := Parse(context.Background(), deps, "energy work from October for three weeks, not Vienna", model.Profile{})

	if intent.Fallback {
		t.Error("expected the model path, not the mechanical fallback")
	}
	if len(intent.Trades) != 1 || intent.Trades[0] != "energy" {
		t.Errorf("intent.Trades = %v, want [energy]", intent.Trades)
	}
	if len(intent.Regions) != 0 {
		t.Errorf("intent.Regions = %v, want none: the negated region must be stripped", intent.Regions)
	}
	if !fc.HasWindow {
		t.Fatal("expected a parsed date window")
	}
	if fc.Start.Month() != 10 {
		t.Errorf("window start month = %v, want October", fc.Start.Month())
	}
	if len(fc.ExcludeRegions) != 1 || fc.ExcludeRegions[0] != "Vienna" {
		t.Errorf("fc.ExcludeRegions = %v, want [Vienna]", fc.ExcludeRegions)
	}
}

func TestParseFallsBackMechanicallyOnModelError(t *testing.T) {
	fake := &fakeLLM{JSONFunc: func(context.Context, llm.Request, string, any) error {
		return errors.New("boom: model returned unparseable garbage")
	}}
	deps := app.Deps{LLM: fake}
	intent, _ := Parse(context.Background(), deps, "electrical crew in Munich", model.Profile{})

	if !intent.Fallback {
		t.Error("expected Fallback=true when the model errors")
	}
	if len(intent.Trades) != 1 || intent.Trades[0] != "electrical" {
		t.Errorf("intent.Trades = %v, want [electrical] from keyword spotting", intent.Trades)
	}
}

func TestParseNoLLMConfigured(t *testing.T) {
	intent, _ := Parse(context.Background(), app.Deps{}, "sanitary work in Zurich", model.Profile{})
	if !intent.Fallback {
		t.Error("expected Fallback=true with no LLM configured")
	}
}

// --- HTTP handler ---------------------------------------------------------------

func TestQueryEscapesTheQuery(t *testing.T) {
	_, mux, _ := newTestHandler(t, nil)
	rec := doForm(mux, http.MethodPost, "/find", url.Values{"q": {`<script>alert(1)</script> steel`}})

	body := rec.Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("raw script tag leaked into the response unescaped")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Error("expected the query to appear HTML-escaped")
	}
}

func TestQueryDegradedBadgeWithoutModel(t *testing.T) {
	_, mux, _ := newTestHandler(t, nil)
	rec := doForm(mux, http.MethodPost, "/find", url.Values{"q": {"steel work"}})
	if !strings.Contains(rec.Body.String(), "matched without the model") {
		t.Error("expected the degraded badge when no model is configured")
	}
}

func TestQueryEmptyReturnsNoContent(t *testing.T) {
	_, mux, _ := newTestHandler(t, nil)
	rec := doForm(mux, http.MethodPost, "/find", url.Values{"q": {"  "}})
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

func TestQueryRefineOverridesFacetWithoutReparsing(t *testing.T) {
	calls := 0
	fake := &fakeLLM{JSONFunc: func(_ context.Context, _ llm.Request, _ string, out any) error {
		calls++
		return errors.New("force mechanical path so we know exactly what trades come back")
	}}
	_, mux, _ := newTestHandler(t, fake)
	rec := doForm(mux, http.MethodPost, "/find", url.Values{"q": {"steel work"}, "refine": {"set:trade:energy"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if calls != 1 {
		t.Errorf("Parse called the model %d times, want exactly 1 (refine must not re-parse)", calls)
	}
	if strings.Contains(rec.Body.String(), "trade matches steel") {
		t.Error("refine to energy should have dropped the steel trade filter")
	}
}
