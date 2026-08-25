package search

import (
	"html"
	"net/http"

	"mm-machine/internal/model"
	"mm-machine/internal/onboarding"
)

// savedView renders the saved-searches panel.
type savedView struct {
	Searches []model.SavedSearch
}

// saved answers GET /find/saved with the current profile's saved searches,
// each runnable in one click and removable in another. Query re-renders
// this panel on every /find turn (see Handler.Query) so it stays current
// without the page shell needing to know it exists.
func (h *Handler) saved(w http.ResponseWriter, r *http.Request) {
	p, ok := onboarding.Current(r, h.deps)
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	list, err := h.deps.Store.ListSavedSearches(r.Context(), p.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tpl.ExecuteTemplate(w, "saved", savedView{Searches: list}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// deleteSaved answers POST /find/saved/delete. store.Store has no delete
// path for a saved search in this slice's scope (see TASK-m2herd-search.md:
// internal/store is off limits), so this can only remove the chip from the
// visitor's current view — it does not persist, and the search reappears on
// the next full page load. Flagged in the handoff report rather than faked
// as a real delete.
func (h *Handler) deleteSaved(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func writeBadge(w http.ResponseWriter, class, text string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<span class="mm-badge ` + html.EscapeString(class) + `">` + html.EscapeString(text) + `</span>`))
}
