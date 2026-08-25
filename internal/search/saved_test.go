package search

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"mm-machine/internal/model"
	"mm-machine/internal/onboarding"
)

func withProfile(t *testing.T, db interface {
	UpsertProfile(context.Context, model.Profile) (model.Profile, error)
}) *http.Cookie {
	t.Helper()
	p, err := db.UpsertProfile(context.Background(), model.Profile{ID: "p1", Role: "owner", Completeness: 100})
	if err != nil {
		t.Fatalf("UpsertProfile: %v", err)
	}
	return &http.Cookie{Name: onboarding.CookieName, Value: p.ID}
}

func TestSavedSearchRoundTrip(t *testing.T) {
	_, mux, db := newTestHandler(t, nil)
	cookie := withProfile(t, db)

	save := httptest.NewRequest(http.MethodPost, "/find/save", strings.NewReader(url.Values{"q": {"steel work"}}.Encode()))
	save.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	save.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, save)
	if !strings.Contains(rec.Body.String(), "saved") {
		t.Fatalf("expected a saved confirmation, got %q", rec.Body.String())
	}

	list := httptest.NewRequest(http.MethodGet, "/find/saved", nil)
	list.AddCookie(cookie)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, list)
	if !strings.Contains(rec.Body.String(), "steel work") {
		t.Fatalf("expected the saved search to be listed, got %q", rec.Body.String())
	}

	del := httptest.NewRequest(http.MethodPost, "/find/saved/delete", strings.NewReader(url.Values{"id": {"1"}}.Encode()))
	del.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, del)
	if rec.Code != http.StatusOK {
		t.Errorf("delete status = %d, want 200", rec.Code)
	}
}

func TestQueryRefreshesSavedList(t *testing.T) {
	_, mux, db := newTestHandler(t, nil)
	cookie := withProfile(t, db)
	if _, err := db.SaveSearch(context.Background(), model.SavedSearch{ProfileID: "p1", Label: "hvac work", Query: "hvac work"}); err != nil {
		t.Fatalf("SaveSearch: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/find", strings.NewReader(url.Values{"q": {"steel"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), "hvac work") {
		t.Errorf("expected the saved-searches panel in the /find response, got %q", rec.Body.String())
	}
}
