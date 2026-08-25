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

type Perspective struct {
	Key        string
	Label      string
	Title      string
	Subtitle   string
	Quote      string
	Primary    string
	Secondary  string
	Stats      []Metric
	Workflow   []string
	Pain       []string
	ActionName string
}

type Metric struct {
	Label string
	Value string
	Note  string
}

type Module struct {
	Name   string
	Body   string
	Impact string
}

type RoadmapItem struct {
	Phase string
	Title string
	Body  string
}

type Dashboard struct {
	Now         string
	View        string
	Query       string
	Role        string
	Offers      []Offer
	Counts      map[string]int
	Spotlight   Offer
	Perspective Perspective
	Perspectives []Perspective
	Modules     []Module
	Roadmap      []RoadmapItem
}

var page = template.Must(template.New("page").Funcs(template.FuncMap{
	"lower": strings.ToLower,
}).Parse(pageHTML + offersHTML + perspectiveHTML))

var partial = template.Must(template.New("partial").Funcs(template.FuncMap{
	"lower": strings.ToLower,
}).Parse(offersHTML + perspectiveHTML))

func main() {
	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServer(http.FS(staticFS)))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/", handleHome)
	mux.HandleFunc("/offers", handleOffers)
	mux.HandleFunc("/perspective", handlePerspective)

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

func handlePerspective(w http.ResponseWriter, r *http.Request) {
	data := dashboard(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := partial.ExecuteTemplate(w, "perspective", data); err != nil {
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
		role = "owner"
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

	perspectives := seedPerspectives()
	perspective := perspectives[0]
	for _, item := range perspectives {
		if item.Key == role {
			perspective = item
			break
		}
	}

	return Dashboard{
		Now:          time.Now().Format("02 Jan 2006 15:04"),
		View:         view,
		Query:        query,
		Role:         perspective.Key,
		Offers:       filtered,
		Counts:       counts(offers),
		Spotlight:    offers[0],
		Perspective:  perspective,
		Perspectives: perspectives,
		Modules:      seedModules(),
		Roadmap:      seedRoadmap(),
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

func seedPerspectives() []Perspective {
	return []Perspective{
		{
			Key:       "owner",
			Label:     "Generalunternehmer",
			Title:     "Mobilize verified teams without broker margin leakage.",
			Subtitle:  "For project owners and GUs who need reliable capacity fast, with proof, clean communication, and payment control.",
			Quote:     "27 Euro paid, 18 Euro reaches the worker. Montage Manager makes the hidden 9 Euro visible and negotiable.",
			Primary:   "Post structured project",
			Secondary: "Compare supplier answers",
			Stats: []Metric{
				{Label: "Broker leakage", Value: "30-50%", Note: "margin often captured without operational value"},
				{Label: "Rework driver", Value: "48%", Note: "caused by miscommunication"},
				{Label: "Mobilization window", Value: "14d", Note: "typical pressure for international teams"},
			},
			Workflow: []string{"Post chaotic brief", "AI normalizes scope", "Invite verified teams", "Compare answers and rates", "Track logbook and approvals", "Release payment and review"},
			Pain:     []string{"Middlemen hide true labor economics", "Site reality differs from the description", "No objective basis for partial acceptance", "Team capacity is hard to verify quickly"},
			ActionName: "Open GU cockpit",
		},
		{
			Key:       "executor",
			Label:     "Subunternehmer",
			Title:     "Win direct work and keep documents ready before the project starts.",
			Subtitle:  "For subcontractors who need direct access to serious projects, predictable payment, team formation, and EU compliance support.",
			Quote:     "A1 can take three weeks for a four-week job. The platform turns document chaos into a reusable trust profile.",
			Primary:   "Find direct projects",
			Secondary: "Prepare compliance safe",
			Stats: []Metric{
				{Label: "Late payments", Value: "77%", Note: "subcontractor projects affected"},
				{Label: "Avg. delay", Value: "57d", Note: "cash-flow risk after work is done"},
				{Label: "Penalty risk", Value: "500k", Note: "possible compliance fines in severe cases"},
			},
			Workflow: []string{"Build trust profile", "Upload A1 and insurance", "Join or form team", "Answer projects directly", "Document progress", "Get accepted and paid"},
			Pain:     []string{"Documents are scattered and expire silently", "GU payment reputation is invisible", "Small teams cannot prove capacity", "Disputes drag because proof is weak"},
			ActionName: "Open SU cockpit",
		},
	}
}

func seedModules() []Module {
	return []Module{
		{Name: "AI Job Assistant", Body: "Turns photos, voice notes, drawings, and rough text into a normalized project package.", Impact: "Cuts bidder questions and scope conflict."},
		{Name: "Team Builder", Body: "Lets subcontractors combine crews, skills, languages, documents, and hardware into deployable teams.", Impact: "Makes large projects accessible without a broker."},
		{Name: "Document Safe", Body: "Stores A1, insurance, certificates, tax data, expiry dates, and trust-member proof.", Impact: "Speeds EU compliance checks."},
		{Name: "Status Documentation", Body: "Photo, video, timestamp, creator, location, request, update, and logbook history.", Impact: "Creates the payment and acceptance record."},
		{Name: "Dispute Desk", Body: "Structured evidence and neutral review flow for defects, delays, and payment disagreements.", Impact: "Avoids slow court escalation."},
	}
}

func seedRoadmap() []RoadmapItem {
	return []RoadmapItem{
		{Phase: "Months 1-3", Title: "MVP: DACH marketplace spine", Body: "Job posting, team formation, basic verification, offer lists, and two role dashboards."},
		{Phase: "Months 4-6", Title: "AI and dispute layer", Body: "AI brief normalization, bidder Q&A, photo history, and first mediation workflow."},
		{Phase: "Months 7-12", Title: "Payments and bureaucracy", Body: "Payment rails, A1/SIPSI/VOB/B modules, enterprise compliance APIs, and premium settlement flows."},
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
  <link rel="stylesheet" href="/static/app.css?v=2">
</head>
<body>
  <header class="topbar">
    <a class="brand" href="/" aria-label="Montage Manager home"><span class="brand-mark">MM</span><span>Montage Manager</span></a>
    <nav class="nav" aria-label="Primary">
      <a href="#pipeline">Pipeline</a>
      <a href="#perspectives">Roles</a>
      <a href="#modules">Modules</a>
      <a href="#workflow">Workflow</a>
    </nav>
    <a class="command" href="#pipeline">Create offer</a>
  </header>

  <main>
    <section class="hero">
      <div class="hero-copy">
        <p class="eyebrow">mm.MachineMachine.ai</p>
        <h1>The operating system for direct subcontractor work.</h1>
        <p class="lede">Montage Manager connects GUs and SUs without opaque broker layers: structured projects, verified teams, document safes, progress proof, payment signals, and dispute evidence.</p>
        <div class="hero-actions">
          <a class="primary" href="#perspectives">Choose perspective</a>
          <a class="secondary" href="#pipeline">Open pipeline</a>
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
      <article><span>Market</span><strong>EUR 2.75T</strong><p>EU construction volume, still fragmented and under-digitized.</p></article>
      <article><span>AI adoption</span><strong>4%</strong><p>Low digital maturity leaves room for workflow-native intelligence.</p></article>
      <article><span>Payment delay</span><strong>57 days</strong><p>Average late-payment drag for subcontractors.</p></article>
      <article><span>Waste</span><strong>EUR 177.5B</strong><p>Productivity losses tied to communication and coordination failure.</p></article>
    </section>

    <section class="workspace perspectives" id="perspectives">
      <div class="section-title">
        <p class="eyebrow">Two-sided product</p>
        <h2>One platform, two operating realities</h2>
      </div>
      <div class="role-switch" hx-target="#perspective-panel" hx-swap="outerHTML">
        {{range .Perspectives}}
        <button class="role-tab {{if eq $.Role .Key}}active{{end}}" hx-get="/perspective?role={{.Key}}&view={{$.View}}&q={{$.Query}}">{{.Label}}</button>
        {{end}}
      </div>
      {{template "perspective" .}}
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

    <section class="modules" id="modules">
      <div class="section-title">
        <p class="eyebrow">Product depth</p>
        <h2>Modules that remove the broker bottleneck</h2>
      </div>
      <div class="module-grid">
        {{range .Modules}}
        <article>
          <h3>{{.Name}}</h3>
          <p>{{.Body}}</p>
          <span>{{.Impact}}</span>
        </article>
        {{end}}
      </div>
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
        <p class="eyebrow">Roadmap and money</p>
        <h2>Start narrow, then own the transaction</h2>
      </div>
      <div class="trust-grid roadmap-grid">
        {{range .Roadmap}}
        <article><span>{{.Phase}}</span><strong>{{.Title}}</strong><p>{{.Body}}</p></article>
        {{end}}
      </div>
      <div class="pricing-strip">
        <div><span>Pro</span><strong>49 EUR/mo</strong><p>Search, organize, verify, communicate.</p></div>
        <div><span>Dispute</span><strong>100 EUR/case</strong><p>Evidence-backed mediation workflow.</p></div>
        <div><span>Enterprise</span><strong>299 EUR/mo</strong><p>Compliance tooling and API access.</p></div>
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

const perspectiveHTML = `{{define "perspective"}}<div id="perspective-panel" class="perspective-panel">
  <div class="persona-main">
    <p class="eyebrow">{{.Perspective.Label}}</p>
    <h3>{{.Perspective.Title}}</h3>
    <p>{{.Perspective.Subtitle}}</p>
    <blockquote>{{.Perspective.Quote}}</blockquote>
    <div class="hero-actions">
      <a class="primary" href="#pipeline">{{.Perspective.Primary}}</a>
      <a class="secondary" href="#modules">{{.Perspective.Secondary}}</a>
    </div>
  </div>
  <div class="persona-stats">
    {{range .Perspective.Stats}}
    <article><span>{{.Label}}</span><strong>{{.Value}}</strong><p>{{.Note}}</p></article>
    {{end}}
  </div>
  <div class="path-card">
    <strong>{{.Perspective.ActionName}}</strong>
    <ol>
      {{range .Perspective.Workflow}}<li>{{.}}</li>{{end}}
    </ol>
  </div>
  <div class="pain-card">
    <strong>Decision pressure</strong>
    <ul>
      {{range .Perspective.Pain}}<li>{{.}}</li>{{end}}
    </ul>
  </div>
</div>{{end}}`
