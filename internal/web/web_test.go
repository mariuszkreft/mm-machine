package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"mm-machine/internal/app"
	"mm-machine/internal/i18n"
	"mm-machine/internal/model"
	"mm-machine/internal/store"
)

func testDeps(t *testing.T) app.Deps {
	t.Helper()
	return app.Deps{Store: store.NewMemory(), Version: "test", LLMModel: "test-model"}
}

func newMux(deps app.Deps) *http.ServeMux {
	mux := http.NewServeMux()
	Register(mux, deps)
	return mux
}

func do(mux *http.ServeMux, method, target string, body string, hx bool, cookie *http.Cookie) *httptest.ResponseRecorder {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestHomeUnknownProfile(t *testing.T) {
	mux := newMux(testDeps(t))
	rec := do(mux, http.MethodGet, "/", "", false, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="mm-input"`) {
		t.Fatalf("home missing prompt input: %s", body)
	}
	// German is the default language for the DACH market.
	if !strings.Contains(body, i18n.T(i18n.DE, "greeting.new")) {
		t.Fatalf("home missing German unknown-profile greeting: %s", body)
	}
	if !strings.Contains(body, `lang="de"`) {
		t.Fatalf("home not marked as German: %s", body)
	}
	if !strings.Contains(body, `id="mm-thread"`) {
		t.Fatalf("home missing thread: %s", body)
	}
}

func TestHomeKnownProfile(t *testing.T) {
	deps := testDeps(t)
	profile, err := deps.Store.UpsertProfile(context.Background(), model.Profile{
		ID:           "known-1",
		Role:         "executor",
		Trades:       []string{"electrical"},
		Regions:      []string{"Munich, DE"},
		Completeness: 60,
		UpdatedAt:    time.Now(),
	})
	if err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	mux := newMux(deps)
	rec := do(mux, http.MethodGet, "/", "", false, &http.Cookie{Name: "mm_profile", Value: profile.ID})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Willkommen zurück") {
		t.Fatalf("home missing known-profile greeting: %s", body)
	}
	// The trade is shown with its German label, not the internal slug.
	if !strings.Contains(body, i18n.T(i18n.DE, "trade.electrical")) {
		t.Fatalf("home missing profile trade: %s", body)
	}
}

// A visitor whose browser asks for English gets English, and the language
// switcher is what makes that choice sticky.
func TestHomeEnglishViaAcceptLanguage(t *testing.T) {
	mux := newMux(testDeps(t))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "en-GB,en;q=0.9")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, i18n.T(i18n.EN, "greeting.new")) {
		t.Fatalf("home did not honour Accept-Language: %s", body)
	}
	if !strings.Contains(body, `lang="en"`) {
		t.Fatalf("home not marked as English: %s", body)
	}
}

// An explicit cookie beats the browser header.
func TestLanguageCookieWins(t *testing.T) {
	mux := newMux(testDeps(t))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "en-GB,en;q=0.9")
	req.AddCookie(&http.Cookie{Name: i18n.CookieName, Value: "de"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), i18n.T(i18n.DE, "greeting.new")) {
		t.Fatalf("language cookie ignored")
	}
}

func TestAboutRenders(t *testing.T) {
	mux := newMux(testDeps(t))
	rec := do(mux, http.MethodGet, "/about", "", false, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Montage Manager") {
		t.Fatalf("about missing title: %s", body)
	}
	if !strings.Contains(body, `id="perspective-panel"`) {
		t.Fatalf("about missing perspective panel: %s", body)
	}
}

func TestOffersFiltersByStatus(t *testing.T) {
	mux := newMux(testDeps(t))
	rec := do(mux, http.MethodGet, "/offers?view=open", "", true, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Warehouse steel assembly") {
		t.Fatalf("open view missing its offer: %s", body)
	}
	if strings.Contains(body, "Photovoltaic roof installation") {
		t.Fatalf("open view leaked a process offer: %s", body)
	}
}

func TestOffersFiltersByQuery(t *testing.T) {
	mux := newMux(testDeps(t))
	rec := do(mux, http.MethodGet, "/offers?view=all&q=Zurich", "", true, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Retail floor refit") {
		t.Fatalf("query missing matching offer: %s", body)
	}
	if strings.Contains(body, "Warehouse steel assembly") {
		t.Fatalf("query leaked a non-matching offer: %s", body)
	}
}

func TestOffersDirectVisitRendersPage(t *testing.T) {
	mux := newMux(testDeps(t))
	rec := do(mux, http.MethodGet, "/offers", "", false, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<!doctype html>") {
		t.Fatalf("direct /offers visit did not get a full page: %s", body)
	}
	if !strings.Contains(body, `id="offers"`) {
		t.Fatalf("pipeline page missing the offers grid: %s", body)
	}
}

func TestOffersNewCreatesAndRerenders(t *testing.T) {
	mux := newMux(testDeps(t))
	rec := do(mux, http.MethodPost, "/offers/new", "title=Test+Offer&location=Testville&status=open", true, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Test Offer") {
		t.Fatalf("create response missing new offer: %s", rec.Body.String())
	}

	follow := do(mux, http.MethodGet, "/offers?view=open", "", true, nil)
	if !strings.Contains(follow.Body.String(), "Test Offer") {
		t.Fatalf("open view missing created offer: %s", follow.Body.String())
	}
}

func TestOffersStatusMovesOffer(t *testing.T) {
	mux := newMux(testDeps(t))
	rec := do(mux, http.MethodPost, "/offers/status", "id=MM-1838&status=process&view=all", true, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Warehouse steel assembly") {
		t.Fatalf("moved offer missing from re-render: %s", body)
	}
}

func TestOffersStatusRejectsUnknown(t *testing.T) {
	mux := newMux(testDeps(t))
	rec := do(mux, http.MethodPost, "/offers/status", "id=MM-1838&status=archived&view=all", true, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown status, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestNoUnescapedUserInput guards the design contract's escaping rule: no
// route may echo user- or model-produced text as raw HTML.
func TestNoUnescapedUserInput(t *testing.T) {
	const payload = `<script>alert(1)</script>`
	escaped := url.QueryEscape(payload)
	mux := newMux(testDeps(t))

	pages := []*httptest.ResponseRecorder{
		do(mux, http.MethodGet, "/", "", false, nil),
		do(mux, http.MethodGet, "/about", "", false, nil),
		do(mux, http.MethodGet, "/offers", "", false, nil),
		do(mux, http.MethodGet, "/offers?view=all&q="+escaped, "", true, nil),
		do(mux, http.MethodPost, "/offers/new", "title="+escaped+"&status=open", true, nil),
	}
	for i, rec := range pages {
		if strings.Contains(rec.Body.String(), payload) {
			t.Fatalf("page %d rendered unescaped user input: %s", i, rec.Body.String())
		}
	}
}
