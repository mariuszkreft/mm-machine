// Package devloop closes the loop: it reads the feedback the assistant
// collected, clusters it into ranked backlog items with the local model, and
// exposes them at /dev plus docs/backlog.md so the next development iteration
// starts from what users actually said.
//
// This is the phase-0 baseline: a mechanical clusterer and the /dev surface.
// LLM clustering, the markdown export and status transitions are the dev-loop
// worker's job.
package devloop

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"time"

	"mm-machine/internal/app"
	"mm-machine/internal/model"
	"mm-machine/internal/store"
)

// Handler serves the development-loop surface.
type Handler struct {
	deps app.Deps
	tpl  *template.Template
}

// Register wires the dev-loop routes onto mux.
func Register(mux *http.ServeMux, deps app.Deps) *Handler {
	h := &Handler{
		deps: deps,
		tpl:  template.Must(template.New("dev").Parse(devHTML + backlogHTML)),
	}
	mux.HandleFunc("/dev", h.page)
	mux.HandleFunc("/dev/refresh", h.refresh)
	mux.HandleFunc("/dev/backlog.json", h.backlogJSON)
	return h
}

type devView struct {
	Generated string
	Model     string
	Version   string
	Feedback  []model.Feedback
	Backlog   []model.BacklogItem
}

func (h *Handler) view(ctx context.Context) (devView, error) {
	fb, err := h.deps.Store.ListFeedback(ctx, store.FeedbackFilter{Limit: 200})
	if err != nil {
		return devView{}, err
	}
	bl, err := h.deps.Store.ListBacklog(ctx)
	if err != nil {
		return devView{}, err
	}
	return devView{
		Generated: time.Now().Format("02 Jan 2006 15:04"),
		Model:     h.deps.LLMModel,
		Version:   h.deps.Version,
		Feedback:  fb,
		Backlog:   bl,
	}, nil
}

func (h *Handler) page(w http.ResponseWriter, r *http.Request) {
	v, err := h.view(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.tpl.ExecuteTemplate(w, "dev", v)
}

// refresh regenerates the backlog from the current feedback.
func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if _, err := Regenerate(ctx, h.deps); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	v, err := h.view(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.tpl.ExecuteTemplate(w, "backlog", v)
}

func (h *Handler) backlogJSON(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Store.ListBacklog(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(items)
}

// Regenerate clusters all feedback into ranked backlog items and stores them.
// The baseline groups by theme (or kind when no theme is set) and ranks by
// frequency times average severity.
func Regenerate(ctx context.Context, deps app.Deps) ([]model.BacklogItem, error) {
	fb, err := deps.Store.ListFeedback(ctx, store.FeedbackFilter{Limit: 500})
	if err != nil {
		return nil, err
	}
	groups := map[string][]model.Feedback{}
	for _, f := range fb {
		key := strings.ToLower(strings.TrimSpace(f.Theme))
		if key == "" {
			key = strings.ToLower(f.Kind)
		}
		if key == "" {
			key = "unsorted"
		}
		groups[key] = append(groups[key], f)
	}
	items := make([]model.BacklogItem, 0, len(groups))
	for key, group := range groups {
		sum := 0
		evidence := make([]string, 0, len(group))
		for _, f := range group {
			sev := f.Severity
			if sev == 0 {
				sev = 3
			}
			sum += sev
			if len(evidence) < 5 {
				evidence = append(evidence, f.Verbatim)
			}
		}
		avg := float64(sum) / float64(len(group))
		items = append(items, model.BacklogItem{
			Title:       strings.ToUpper(key[:1]) + key[1:],
			Rationale:   "Grouped from " + itoa(len(group)) + " pieces of user feedback.",
			Theme:       key,
			Kind:        group[0].Kind,
			Count:       len(group),
			AvgSeverity: avg,
			Score:       float64(len(group)) * avg,
			Evidence:    evidence,
			Status:      "proposed",
			UpdatedAt:   time.Now(),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Score > items[j].Score })
	if err := deps.Store.ReplaceBacklog(ctx, items); err != nil {
		return nil, err
	}
	return items, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
