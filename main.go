// Command mm-machine serves Montage Manager: the public marketplace surface,
// a self-aware assistant backed by the fleet's local model, and the development
// loop that turns user feedback into a ranked backlog.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"mm-machine/internal/app"
	"mm-machine/internal/assistant"
	"mm-machine/internal/demo"
	"mm-machine/internal/devloop"
	"mm-machine/internal/i18n"
	"mm-machine/internal/llm"
	"mm-machine/internal/model"
	"mm-machine/internal/onboarding"
	"mm-machine/internal/search"
	"mm-machine/internal/store"
	"mm-machine/internal/web"
)

// Version is stamped into the footer and the assistant's context.
const Version = "0.5.0"

func main() {
	db, err := openStore()
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer db.Close()

	client := llm.New(llm.Config{
		BaseURL: env("LLM_BASE_URL", llm.DefaultBaseURL),
		Model:   env("LLM_MODEL", llm.DefaultModel),
		APIKey:  env("LLM_API_KEY", "local"),
	})

	deps := app.Deps{Store: db, LLM: client, Version: Version, LLMModel: client.Model()}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
		defer cancel()
		if err := client.Health(ctx); err != nil {
			http.Error(w, "llm: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ready"))
	})
	web.Register(mux, deps)
	devloop.Register(mux, deps)
	demo.Register(mux, deps)
	onboard := onboarding.Register(mux, deps)
	finder := search.Register(mux, deps)
	helper := assistant.Register(mux, deps)

	// /ask is the single entry point behind the prompt. Routing lives here
	// rather than in any of the three packages so none of them owns the others.
	// /lang switches language and returns where the visitor came from.
	mux.HandleFunc("/lang", func(w http.ResponseWriter, r *http.Request) {
		i18n.Set(w, i18n.Lang(r.FormValue("to")+r.URL.Query().Get("to")))
		back := r.Referer()
		if back == "" {
			back = "/"
		}
		http.Redirect(w, r, back, http.StatusSeeOther)
	})

	mux.HandleFunc("/ask", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		profile, err := onboarding.Ensure(w, r, deps)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		switch route(r.FormValue("message"), profile) {
		case routeAssist:
			helper.Ask(w, r)
		case routeOnboard:
			onboard.Turn(w, r)
		default:
			finder.Query(w, r)
		}
	})

	// The dev loop re-clusters feedback into the backlog on an interval
	// (BACKLOG_INTERVAL, 0 disables).
	ctx, stopLoop := context.WithCancel(context.Background())
	defer stopLoop()
	stop := devloop.Start(ctx, deps)
	defer stop()

	srv := &http.Server{
		Addr:              ":" + env("PORT", "8080"),
		Handler:           withSecurityHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Minute, // the assistant waits on the local model
	}
	// Fill an empty install with the demo market so the app is understandable
	// the moment it is opened, and never overwrite what is already there.
	seedCtx, cancelSeed := context.WithTimeout(context.Background(), 30*time.Second)
	if err := demo.Seed(seedCtx, db); err != nil {
		log.Printf("demo: seed: %v", err)
	}
	cancelSeed()

	log.Printf("mm-machine %s listening on %s (llm %s)", Version, srv.Addr, client.Model())
	log.Fatal(srv.ListenAndServe())
}

// openStore uses SQLite when a path is configured and falls back to the
// in-memory store otherwise.
func openStore() (store.Store, error) {
	path := env("DB_PATH", "data/mm.db")
	if path == "" || path == ":memory:" {
		return store.NewMemory(), nil
	}
	db, err := store.Open(path)
	if err != nil {
		log.Printf("store: sqlite unavailable (%v) — falling back to memory", err)
		return store.NewMemory(), nil
	}
	return db, nil
}

// Where a prompt goes. Three destinations, one rule each:
//   - a visitor the app cannot describe yet is onboarding,
//   - a question about the product itself belongs to the assistant (which is
//     also what keeps the feedback loop fed),
//   - everything else is a search.
const (
	routeOnboard = "onboard"
	routeAssist  = "assist"
	routeSearch  = "search"
)

// metaMarkers are phrasings that are about the app rather than about work.
// Kept deliberately literal: a cheap wrong guess here costs one redirect, an
// LLM classifier would cost a round-trip on every single prompt.
var metaMarkers = []string{
	// English
	"how do i", "how does", "how can i", "what is this", "what can i", "what does this",
	"who are you", "help", "explain", "why do", "why does", "bug", "broken", "not working",
	"doesn't work", "does not work", "confus", "feedback", "suggest", "you should", "i wish",
	// German — the default language, so these matter more than the English ones
	"wie funktioniert", "wie geht", "wie macht ihr", "wie sucht", "was ist das", "was ist mm",
	"was kann ich", "was macht", "was passiert mit", "wer seid ihr", "wozu", "warum",
	"hilfe", "erklär", "erklar", "kaputt", "geht nicht", "funktioniert nicht", "fehler",
	"verwirr", "unklar", "vorschlag", "ihr solltet", "ich wünsche", "ich wuensche",
	"datenschutz", "meine daten", "meinen daten",
}

func route(message string, profile model.Profile) string {
	m := strings.ToLower(strings.TrimSpace(message))
	for _, marker := range metaMarkers {
		if strings.Contains(m, marker) {
			return routeAssist
		}
	}
	if !profile.Known() {
		return routeOnboard
	}
	return routeSearch
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}
