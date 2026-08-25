// Command mm-machine serves Montage Manager: the public marketplace surface,
// a self-aware assistant backed by the fleet's local model, and the development
// loop that turns user feedback into a ranked backlog.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"mm-machine/internal/app"
	"mm-machine/internal/assistant"
	"mm-machine/internal/devloop"
	"mm-machine/internal/llm"
	"mm-machine/internal/store"
	"mm-machine/internal/web"
)

// Version is stamped into the footer and the assistant's context.
const Version = "0.3.0"

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
	assistant.Register(mux, deps)
	devloop.Register(mux, deps)

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
