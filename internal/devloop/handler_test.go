package devloop

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"mm-machine/internal/app"
	"mm-machine/internal/i18n"
	"mm-machine/internal/model"
	"mm-machine/internal/store"
)

func newTestHandler(t *testing.T, mem *store.Memory) (*http.ServeMux, app.Deps) {
	t.Helper()
	deps := app.Deps{Store: mem, LLM: nil, LLMModel: "none", Version: "test"}
	mux := http.NewServeMux()
	Register(mux, deps)
	return mux, deps
}

func seedBacklogItem(t *testing.T, mem *store.Memory, item model.BacklogItem) model.BacklogItem {
	t.Helper()
	if err := mem.ReplaceBacklog(context.Background(), []model.BacklogItem{item}); err != nil {
		t.Fatalf("seed backlog: %v", err)
	}
	items, err := mem.ListBacklog(context.Background())
	if err != nil || len(items) != 1 {
		t.Fatalf("ListBacklog after seed: %v (%d items)", err, len(items))
	}
	return items[0]
}

func TestSetStatusEndpoint(t *testing.T) {
	mem := store.NewMemory()
	item := seedBacklogItem(t, mem, model.BacklogItem{Title: "Do the thing", Theme: "thing", Status: "proposed"})

	mux, _ := newTestHandler(t, mem)

	form := url.Values{"status": {"accepted"}}
	req := httptest.NewRequest(http.MethodPost, "/dev/backlog/"+strconv.FormatInt(item.ID, 10)+"/status", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	got, err := mem.ListBacklog(context.Background())
	if err != nil {
		t.Fatalf("ListBacklog: %v", err)
	}
	if len(got) != 1 || got[0].Status != "accepted" {
		t.Fatalf("backlog item not updated: %+v", got)
	}
	if !strings.Contains(rec.Body.String(), "accepted") {
		t.Errorf("response should render the new status; body = %s", rec.Body.String())
	}
}

func TestSetStatusEndpoint_RejectsUnknownStatus(t *testing.T) {
	mem := store.NewMemory()
	item := seedBacklogItem(t, mem, model.BacklogItem{Title: "x", Theme: "x", Status: "proposed"})
	mux, _ := newTestHandler(t, mem)

	form := url.Values{"status": {"deleted"}}
	req := httptest.NewRequest(http.MethodPost, "/dev/backlog/"+strconv.FormatInt(item.ID, 10)+"/status", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestSetStatusEndpoint_UnknownID(t *testing.T) {
	mem := store.NewMemory()
	mux, _ := newTestHandler(t, mem)

	form := url.Values{"status": {"accepted"}}
	req := httptest.NewRequest(http.MethodPost, "/dev/backlog/999/status", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body = %s", rec.Code, rec.Body.String())
	}
}

func TestDevPage_EscapesFeedback(t *testing.T) {
	mem := store.NewMemory()
	seedFeedback(t, mem, []model.Feedback{
		newFeedback("bug", "xss", "<script>alert(1)</script>", 3),
	})
	mux, _ := newTestHandler(t, mem)

	req := httptest.NewRequest(http.MethodGet, "/dev", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Errorf("feedback verbatim was not escaped: %s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Errorf("expected escaped script tag in output")
	}
}

// /dev is German by default, and the browser lang attribute must say so.
func TestDevPage_GermanByDefault(t *testing.T) {
	mux, _ := newTestHandler(t, store.NewMemory())
	req := httptest.NewRequest(http.MethodGet, "/dev", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, `lang="de"`) {
		t.Fatalf("/dev not marked as German: %s", body)
	}
	if !strings.Contains(body, i18n.T(i18n.DE, "dev.title")) {
		t.Fatalf("/dev missing German title: %s", body)
	}
}

// The language cookie must carry through /dev exactly like every other page.
func TestDevPage_EnglishViaCookie(t *testing.T) {
	mux, _ := newTestHandler(t, store.NewMemory())
	req := httptest.NewRequest(http.MethodGet, "/dev", nil)
	req.AddCookie(&http.Cookie{Name: i18n.CookieName, Value: "en"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, `lang="en"`) {
		t.Fatalf("/dev not marked as English: %s", body)
	}
	if !strings.Contains(body, i18n.T(i18n.EN, "dev.title")) {
		t.Fatalf("/dev missing English title: %s", body)
	}
}

// Backlog status and feedback kind are a fixed vocabulary (proposed/accepted/
// shipped/rejected, bug/confusion/request/praise); the badges and filters
// must show the localized label, not the raw internal value.
func TestDevPage_LocalizesBacklogAndFeedbackVocabulary(t *testing.T) {
	mem := store.NewMemory()
	seedBacklogItem(t, mem, model.BacklogItem{Title: "Ship the thing", Theme: "thing", Kind: "bug", Status: "accepted"})
	seedFeedback(t, mem, []model.Feedback{
		newFeedback("praise", "nice", "this is great", 1),
	})
	mux, _ := newTestHandler(t, mem)

	de := httptest.NewRecorder()
	mux.ServeHTTP(de, httptest.NewRequest(http.MethodGet, "/dev", nil))
	deBody := de.Body.String()
	if !strings.Contains(deBody, i18n.T(i18n.DE, "backlog.status.accepted")) {
		t.Errorf("German /dev missing localized backlog status: %s", deBody)
	}
	if !strings.Contains(deBody, i18n.T(i18n.DE, "feedback.kind.praise")) {
		t.Errorf("German /dev missing localized feedback kind: %s", deBody)
	}

	enReq := httptest.NewRequest(http.MethodGet, "/dev", nil)
	enReq.AddCookie(&http.Cookie{Name: i18n.CookieName, Value: "en"})
	en := httptest.NewRecorder()
	mux.ServeHTTP(en, enReq)
	enBody := en.Body.String()
	if !strings.Contains(enBody, "accepted") {
		t.Errorf("English /dev missing English backlog status: %s", enBody)
	}
	if !strings.Contains(enBody, "praise") {
		t.Errorf("English /dev missing English feedback kind: %s", enBody)
	}
}

func TestDevPage_FilterByKind(t *testing.T) {
	mem := store.NewMemory()
	seedFeedback(t, mem, []model.Feedback{
		newFeedback("bug", "a", "bug report", 3),
		newFeedback("praise", "b", "nice job", 1),
	})
	mux, _ := newTestHandler(t, mem)

	req := httptest.NewRequest(http.MethodGet, "/dev?kind=bug", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "bug report") {
		t.Errorf("expected matching feedback to be present")
	}
	if strings.Contains(body, "nice job") {
		t.Errorf("filter by kind=bug should exclude praise feedback")
	}
}
