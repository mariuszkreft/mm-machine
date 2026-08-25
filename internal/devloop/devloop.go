// Package devloop closes the loop: it reads the feedback the assistant
// collected, clusters it into ranked backlog items with the local model, and
// exposes them at /dev plus docs/backlog.md so the next development iteration
// starts from what users actually said.
//
// Clustering prefers the local LLM (see cluster.go) and always falls back to
// a mechanical grouping-by-theme when the model is unreachable or returns
// garbage, so /dev never breaks because the cluster is down.
package devloop

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
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
		tpl:  template.Must(template.New("dev").Funcs(funcs).Parse(devHTML + workspaceHTML + countsHTML + backlogHTML + feedbackHTML)),
	}
	mux.HandleFunc("/dev", h.page)
	mux.HandleFunc("/dev/filter", h.filter)
	mux.HandleFunc("/dev/refresh", h.refresh)
	mux.HandleFunc("POST /dev/backlog/{id}/status", h.setStatus)
	mux.HandleFunc("/dev/backlog.json", h.backlogJSON)
	return h
}

var funcs = template.FuncMap{
	"slice": func(items ...string) []string { return items },
}

// filters narrows the backlog and feedback lists shown on /dev.
type filters struct {
	Kind   string
	Status string
}

func filtersFromRequest(r *http.Request) filters {
	_ = r.ParseForm()
	return filters{
		Kind:   strings.TrimSpace(r.Form.Get("kind")),
		Status: strings.TrimSpace(r.Form.Get("status")),
	}
}

// counts summarises the feedback pool for the /dev header.
type counts struct {
	Total   int
	New     int
	Triaged int
	ByKind  map[string]int
}

func computeCounts(fb []model.Feedback) counts {
	c := counts{ByKind: map[string]int{}}
	for _, f := range fb {
		c.Total++
		if f.Status == "" || f.Status == "new" {
			c.New++
		} else {
			c.Triaged++
		}
		c.ByKind[f.Kind]++
	}
	return c
}

// backlogView adds the display-only score bar percentage to a backlog item.
type backlogView struct {
	model.BacklogItem
	ScorePct float64
}

func toBacklogView(items []model.BacklogItem) []backlogView {
	max := 0.0
	for _, it := range items {
		if it.Score > max {
			max = it.Score
		}
	}
	out := make([]backlogView, len(items))
	for i, it := range items {
		pct := 0.0
		if max > 0 {
			pct = it.Score / max * 100
		}
		out[i] = backlogView{BacklogItem: it, ScorePct: pct}
	}
	return out
}

func filterBacklog(items []model.BacklogItem, f filters) []model.BacklogItem {
	if f.Kind == "" && f.Status == "" {
		return items
	}
	out := make([]model.BacklogItem, 0, len(items))
	for _, it := range items {
		if f.Kind != "" && !strings.EqualFold(it.Kind, f.Kind) {
			continue
		}
		if f.Status != "" && !strings.EqualFold(it.Status, f.Status) {
			continue
		}
		out = append(out, it)
	}
	return out
}

func filterFeedback(items []model.Feedback, f filters) []model.Feedback {
	if f.Kind == "" {
		return items
	}
	out := make([]model.Feedback, 0, len(items))
	for _, it := range items {
		if !strings.EqualFold(it.Kind, f.Kind) {
			continue
		}
		out = append(out, it)
	}
	return out
}

type devView struct {
	Generated string
	Model     string
	Version   string
	Filters   filters
	Counts    counts
	Feedback  []model.Feedback
	Backlog   []backlogView
	LastRun   string
	LastError string
	Interval  string
}

func (h *Handler) view(ctx context.Context, f filters) (devView, error) {
	all, err := h.deps.Store.ListFeedback(ctx, store.FeedbackFilter{})
	if err != nil {
		return devView{}, err
	}
	recent, err := h.deps.Store.ListFeedback(ctx, store.FeedbackFilter{Limit: 200})
	if err != nil {
		return devView{}, err
	}
	bl, err := h.deps.Store.ListBacklog(ctx)
	if err != nil {
		return devView{}, err
	}

	lastRun := "never"
	if t := LastRun(); !t.IsZero() {
		lastRun = t.Format("02 Jan 2006 15:04:05")
	}
	lastErr := ""
	if err := LastError(); err != nil {
		lastErr = err.Error()
	}

	return devView{
		Generated: time.Now().Format("02 Jan 2006 15:04"),
		Model:     h.deps.LLMModel,
		Version:   h.deps.Version,
		Filters:   f,
		Counts:    computeCounts(all),
		Feedback:  filterFeedback(recent, f),
		Backlog:   toBacklogView(filterBacklog(bl, f)),
		LastRun:   lastRun,
		LastError: lastErr,
		Interval:  backlogInterval().String(),
	}, nil
}

func (h *Handler) page(w http.ResponseWriter, r *http.Request) {
	v, err := h.view(r.Context(), filtersFromRequest(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.tpl.ExecuteTemplate(w, "dev", v)
}

// filter re-renders the workspace body for a kind/status change.
func (h *Handler) filter(w http.ResponseWriter, r *http.Request) {
	v, err := h.view(r.Context(), filtersFromRequest(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.tpl.ExecuteTemplate(w, "workspace", v)
}

// refresh regenerates the backlog from the current feedback.
func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f := filtersFromRequest(r)
	if _, err := regenerateGuarded(ctx, h.deps); err != nil && !errors.Is(err, errRegenInProgress) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	v, err := h.view(ctx, f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.tpl.ExecuteTemplate(w, "workspace", v)
}

// setStatus accepts/rejects/ships a backlog item.
func (h *Handler) setStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	f := filtersFromRequest(r)
	status := strings.ToLower(strings.TrimSpace(r.Form.Get("status")))
	switch status {
	case "accepted", "rejected", "shipped", "proposed":
	default:
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}
	if err := h.deps.Store.SetBacklogStatus(r.Context(), id, status); err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, store.ErrNotFound) {
			code = http.StatusNotFound
		}
		http.Error(w, err.Error(), code)
		return
	}
	v, err := h.view(r.Context(), f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.tpl.ExecuteTemplate(w, "workspace", v)
}

func (h *Handler) backlogJSON(w http.ResponseWriter, r *http.Request) {
	items, err := h.deps.Store.ListBacklog(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if items == nil {
		// A nil slice encodes as null, which breaks JSON consumers expecting a list.
		items = []model.BacklogItem{}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(items)
}

// --- regeneration state: last run time / error, and an overlap guard -------

var (
	stateMu      sync.Mutex
	regenRunning bool
	lastRunAt    time.Time
	lastRunErr   error
)

var errRegenInProgress = errors.New("devloop: regeneration already in progress")

// LastRun returns when Regenerate last finished (zero if it never ran).
func LastRun() time.Time {
	stateMu.Lock()
	defer stateMu.Unlock()
	return lastRunAt
}

// LastError returns the error from the last regeneration, if any.
func LastError() error {
	stateMu.Lock()
	defer stateMu.Unlock()
	return lastRunErr
}

// regenerateGuarded runs Regenerate, refusing to overlap a run already in
// flight and recording the outcome for LastRun/LastError.
func regenerateGuarded(ctx context.Context, deps app.Deps) ([]model.BacklogItem, error) {
	stateMu.Lock()
	if regenRunning {
		stateMu.Unlock()
		return nil, errRegenInProgress
	}
	regenRunning = true
	stateMu.Unlock()

	items, err := Regenerate(ctx, deps)

	stateMu.Lock()
	regenRunning = false
	lastRunAt = time.Now()
	lastRunErr = err
	stateMu.Unlock()

	return items, err
}

// --- background refresh -----------------------------------------------------

// Start regenerates the backlog on a fixed interval (BACKLOG_INTERVAL, default
// 15m, "0" disables) until ctx is cancelled or the returned stop func is
// called. Wire it from main.go with:
//
//	stop := devloop.Start(ctx, deps)
//	defer stop()
func Start(ctx context.Context, deps app.Deps) func() {
	interval := backlogInterval()
	ctx, cancel := context.WithCancel(ctx)
	if interval <= 0 {
		return cancel
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := regenerateGuarded(ctx, deps); err != nil && !errors.Is(err, errRegenInProgress) {
					log.Printf("devloop: background regeneration failed: %v", err)
				}
			}
		}
	}()
	return cancel
}

func backlogInterval() time.Duration {
	raw := strings.TrimSpace(os.Getenv("BACKLOG_INTERVAL"))
	if raw == "" {
		return 15 * time.Minute
	}
	if raw == "0" {
		return 0
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		log.Printf("devloop: bad BACKLOG_INTERVAL %q, using 15m", raw)
		return 15 * time.Minute
	}
	return d
}

func backlogPath() string {
	if v := strings.TrimSpace(os.Getenv("BACKLOG_PATH")); v != "" {
		return v
	}
	return "docs/backlog.md"
}
