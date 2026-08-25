package main

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

//go:embed static/*
var staticFS embed.FS

type Offer struct {
	ID        string
	Title     string
	Location  string
	Category  string
	Amount    string
	Budget    string
	Status    string
	Signal    string
	Supplier  string
	Updated   string
	Progress  int
	Attention string
}

type Dashboard struct {
	Now       string
	View      string
	Query     string
	Role      string
	Offers    []Offer
	Counts    map[string]int
	Spotlight Offer
}

var page = template.Must(template.New("page").Funcs(template.FuncMap{
	"lower": strings.ToLower,
}).Parse(pageHTML + offersHTML))

var partial = template.Must(template.New("partial").Funcs(template.FuncMap{
	"lower": strings.ToLower,
}).Parse(offersHTML))

func main() {
	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServer(http.FS(staticFS)))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/", handleHome)
	mux.HandleFunc("/offers", handleOffers)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           withSecurityHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("mm-machine listening on :%s", port)
	log.Fatal(srv.ListenAndServe())
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	data := dashboard(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := page.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleOffers(w http.ResponseWriter, r *http.Request) {
	data := dashboard(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := partial.ExecuteTemplate(w, "offers", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func dashboard(r *http.Request) Dashboard {
	view := r.URL.Query().Get("view")
	if view == "" {
		view = "all"
	}
	role := r.URL.Query().Get("role")
	if role == "" {
		role = "customer"
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	offers := seedOffers()
	filtered := make([]Offer, 0, len(offers))
	for _, offer := range offers {
		if view != "all" && strings.ToLower(offer.Status) != view {
			continue
		}
		if query != "" {
			haystack := strings.ToLower(offer.Title + " " + offer.Location + " " + offer.Category + " " + offer.Supplier)
			if !strings.Contains(haystack, strings.ToLower(query)) {
				continue
			}
		}
		filtered = append(filtered, offer)
	}

	return Dashboard{
		Now:       time.Now().Format("02 Jan 2006 15:04"),
		View:      view,
		Query:     query,
		Role:      role,
		Offers:    filtered,
		Counts:    counts(offers),
		Spotlight: offers[0],
	}
}

func seedOffers() []Offer {
	return []Offer{
		{ID: "MM-1842", Title: "Photovoltaic roof installation", Location: "Munich, DE", Category: "Energy", Amount: "420 panels", Budget: "EUR 146k", Status: "process", Signal: "Attention", Supplier: "Voltwerk GmbH", Updated: "12 min ago", Progress: 68, Attention: "3 requests need document expiry checks"},
		{ID: "MM-1841", Title: "Retail floor refit", Location: "Zurich, CH", Category: "Interior", Amount: "1,800 m2", Budget: "EUR 82k", Status: "requested", Signal: "OK", Supplier: "Alpine Montage", Updated: "38 min ago", Progress: 36, Attention: "5 supplier answers ready"},
		{ID: "MM-1838", Title: "Warehouse steel assembly", Location: "Rotterdam, NL", Category: "Industrial", Amount: "96 tons", Budget: "EUR 310k", Status: "open", Signal: "OK", Supplier: "Nordline Build", Updated: "1 h ago", Progress: 22, Attention: "Hardware list confirmed"},
		{ID: "MM-1832", Title: "Hotel bathroom modernization", Location: "Vienna, AT", Category: "Sanitary", Amount: "74 rooms", Budget: "EUR 228k", Status: "done", Signal: "Review", Supplier: "Prime Install", Updated: "Yesterday", Progress: 100, Attention: "Review window open"},
	}
}

func counts(offers []Offer) map[string]int {
	result := map[string]int{"all": len(offers)}
	for _, offer := range offers {
		result[strings.ToLower(offer.Status)]++
	}
	return result
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

const pageHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Montage Manager</title>
  <script src="https://unpkg.com/htmx.org@2.0.3"></script>
  <link rel="stylesheet" href="/static/app.css">
</head>
<body>
  <header class="topbar">
    <a class="brand" href="/" aria-label="Montage Manager home"><span class="brand-mark">MM</span><span>Montage Manager</span></a>
    <nav class="nav" aria-label="Primary">
      <a href="#pipeline">Pipeline</a>
      <a href="#workflow">Workflow</a>
      <a href="#trust">Trust</a>
    </nav>
    <a class="command" href="#pipeline">Create offer</a>
  </header>

  <main>
    <section class="hero">
      <div class="hero-copy">
        <p class="eyebrow">mm.MachineMachine.ai</p>
        <h1>Project offers, supplier trust, and delivery control in one operating surface.</h1>
        <p class="lede">Montage Manager turns messy installation work into searchable offers, structured requests, live status, document checks, mail, calendars, and reviews.</p>
        <div class="hero-actions">
          <a class="primary" href="#pipeline">Open pipeline</a>
          <a class="secondary" href="#workflow">See structure</a>
        </div>
      </div>
      <div class="hero-panel" aria-label="Live offer command panel">
        <div class="panel-head">
          <span>Live command</span>
          <strong>{{.Now}}</strong>
        </div>
        <div class="signal-grid">
          <div><span>Requests</span><strong>18</strong></div>
          <div><span>In process</span><strong>7</strong></div>
          <div><span>Attention</span><strong>3</strong></div>
        </div>
        <div class="focus-card">
          <span>{{.Spotlight.ID}}</span>
          <h2>{{.Spotlight.Title}}</h2>
          <p>{{.Spotlight.Attention}}</p>
          <div class="progress"><i style="width: {{.Spotlight.Progress}}%"></i></div>
        </div>
        <img class="map-preview" src="/static/reference-map.jpg" alt="Montage Manager structure map">
      </div>
    </section>

    <section class="metrics" aria-label="Platform modules">
      <article><span>01</span><strong>Trial</strong><p>Restricted discovery without contact data or pictures.</p></article>
      <article><span>02</span><strong>Trust member</strong><p>Verified business details, documents, hardware, contracts, and portfolio proof.</p></article>
      <article><span>03</span><strong>Role dashboards</strong><p>Customer and supplier routes with news, mail, offers, calendar, profile, community.</p></article>
      <article><span>04</span><strong>Review loop</strong><p>Structured ratings, pros, cons, pictures, and text after completion.</p></article>
    </section>

    <section class="workspace" id="pipeline">
      <div class="section-title">
        <p class="eyebrow">Pipeline</p>
        <h2>Offer control surface</h2>
      </div>
      <div class="toolbar" hx-boost="true" hx-target="#offers" hx-swap="outerHTML">
        <a class="tab {{if eq .View "all"}}active{{end}}" href="/offers?view=all&role={{.Role}}">All <span>{{index .Counts "all"}}</span></a>
        <a class="tab {{if eq .View "open"}}active{{end}}" href="/offers?view=open&role={{.Role}}">Open <span>{{index .Counts "open"}}</span></a>
        <a class="tab {{if eq .View "requested"}}active{{end}}" href="/offers?view=requested&role={{.Role}}">Requested <span>{{index .Counts "requested"}}</span></a>
        <a class="tab {{if eq .View "process"}}active{{end}}" href="/offers?view=process&role={{.Role}}">Process <span>{{index .Counts "process"}}</span></a>
        <a class="tab {{if eq .View "done"}}active{{end}}" href="/offers?view=done&role={{.Role}}">Done <span>{{index .Counts "done"}}</span></a>
        <form class="search" hx-get="/offers" hx-target="#offers" hx-trigger="input changed delay:220ms, submit" hx-swap="outerHTML">
          <input type="hidden" name="view" value="{{.View}}">
          <input type="hidden" name="role" value="{{.Role}}">
          <input name="q" value="{{.Query}}" placeholder="Search offers, suppliers, cities" autocomplete="off">
        </form>
      </div>
      {{template "offers" .}}
    </section>

    <section class="workflow" id="workflow">
      <div class="section-title">
        <p class="eyebrow">Structure</p>
        <h2>From discovery to review</h2>
      </div>
      <div class="lanes">
        <article><span>Trial</span><h3>Browse limited market</h3><p>No pictures or contact details until registration.</p></article>
        <article><span>Registration</span><h3>Build trust profile</h3><p>Documents, licenses, tax data, hardware proof, AGB, portfolio.</p></article>
        <article><span>Customer</span><h3>Search and request</h3><p>Filters by address, time, category, amount, license, hardware, price.</p></article>
        <article><span>Supplier</span><h3>Create and manage</h3><p>Open offers, requests, status changes, mail, updates, logbook.</p></article>
        <article><span>Completion</span><h3>Close the loop</h3><p>Done offers collect structured ratings and proof-rich reviews.</p></article>
      </div>
    </section>

    <section class="trust" id="trust">
      <div class="section-title">
        <p class="eyebrow">Trust layer</p>
        <h2>Verification made operational</h2>
      </div>
      <div class="trust-grid">
        <article><strong>Expiry intelligence</strong><p>News feed flags expired documents, missing certification pictures, and blocked trust fields.</p></article>
        <article><strong>Unified mail</strong><p>Open offers, active projects, community requests, and status actions stay in one inbox.</p></article>
        <article><strong>Calendar availability</strong><p>Supplier capacity and busy windows become part of offer routing.</p></article>
      </div>
    </section>
  </main>
</body>
</html>`

const offersHTML = `{{define "offers"}}<div id="offers" class="offers" aria-live="polite">
  {{range .Offers}}
  <article class="offer">
    <div class="offer-top">
      <span class="id">{{.ID}}</span>
      <span class="badge {{lower .Status}}">{{.Status}}</span>
    </div>
    <h3>{{.Title}}</h3>
    <p>{{.Location}} · {{.Category}} · {{.Amount}}</p>
    <div class="offer-meta">
      <span>{{.Supplier}}</span>
      <strong>{{.Budget}}</strong>
    </div>
    <div class="progress"><i style="width: {{.Progress}}%"></i></div>
    <div class="offer-foot">
      <span>{{.Signal}}</span>
      <span>Updated {{.Updated}}</span>
    </div>
  </article>
  {{else}}
  <div class="empty">No offers match this view.</div>
  {{end}}
</div>{{end}}`
