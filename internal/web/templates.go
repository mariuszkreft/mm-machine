package web

// pageHTML is the full public page. Partials it references live below.
const pageHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Montage Manager</title>
  <script src="https://unpkg.com/htmx.org@2.0.3"></script>
  <script src="https://unpkg.com/htmx-ext-sse@2.2.2/sse.js"></script>
  <link rel="stylesheet" href="/static/app.css?v=4">
  <link rel="stylesheet" href="/static/assistant.css?v=1">
</head>
<body>
  <header class="topbar">
    <a class="brand" href="/" aria-label="Montage Manager home"><span class="brand-mark">MM</span><span>Montage Manager</span></a>
    <nav class="nav" aria-label="Primary">
      <a href="#pipeline">Pipeline</a>
      <a href="#perspectives">Roles</a>
      <a href="#modules">Modules</a>
      <a href="#assistant">Assistant</a>
      <a href="/dev">Dev loop</a>
    </nav>
    <a class="command" href="#assistant">Talk to the app</a>
  </header>

  <main>
    <section class="hero">
      <div class="hero-copy">
        <p class="eyebrow">mm.MachineMachine.ai</p>
        <h1>The operating system for direct subcontractor work.</h1>
        <p class="lede">Montage Manager connects GUs and SUs without opaque broker layers: structured projects, verified teams, document safes, progress proof, payment signals, and dispute evidence.</p>
        <div class="hero-actions">
          <a class="primary" href="#assistant">Ask the assistant</a>
          <a class="secondary" href="#pipeline">Open pipeline</a>
        </div>
      </div>
      <div class="hero-panel" aria-label="Live offer command panel">
        <div class="panel-head">
          <span>Live command</span>
          <strong>{{.Now}}</strong>
        </div>
        <div class="signal-grid">
          <div><span>All</span><strong>{{index .Counts "all"}}</strong></div>
          <div><span>In process</span><strong>{{index .Counts "process"}}</strong></div>
          <div><span>Open</span><strong>{{index .Counts "open"}}</strong></div>
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

    <section class="workspace assistant-section" id="assistant">
      <div class="section-title">
        <p class="eyebrow">Local AI · {{.LLMModel}}</p>
        <h2>Talk to the app about the app</h2>
      </div>
      <div id="assistant-panel" hx-get="/assistant/panel?role={{.Role}}&route=home" hx-trigger="load" hx-swap="outerHTML">
        <div class="assistant-loading">Waking the local model…</div>
      </div>
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
      <form class="offer-form" hx-post="/offers/new" hx-target="#offers" hx-swap="outerHTML">
        <input name="title" placeholder="New offer title" required>
        <input name="location" placeholder="City, country">
        <input name="category" placeholder="Category">
        <input name="budget" placeholder="Budget">
        <input name="supplier" placeholder="Supplier">
        <select name="status">
          <option value="open">open</option>
          <option value="requested">requested</option>
          <option value="process">process</option>
          <option value="done">done</option>
        </select>
        <button type="submit">Create offer</button>
      </form>
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

    <section class="trust" id="trust">
      <div class="section-title">
        <p class="eyebrow">Roadmap</p>
        <h2>Start narrow, then own the transaction</h2>
      </div>
      <div class="trust-grid roadmap-grid">
        {{range .Roadmap}}
        <article><span>{{.Phase}}</span><strong>{{.Title}}</strong><p>{{.Body}}</p></article>
        {{end}}
      </div>
    </section>
  </main>

  <footer class="site-foot">
    <span>Montage Manager {{.Version}}</span>
    <span>local model: {{.LLMModel}}</span>
    <a href="/dev">dev loop</a>
  </footer>
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
    <div class="offer-actions" hx-target="#offers" hx-swap="outerHTML">
      {{$id := .ID}}{{$cur := lower .Status}}
      {{range $next := $.Statuses}}
        {{if ne $next $cur}}
        <button hx-post="/offers/status" hx-vals='{"id": "{{$id}}", "status": "{{$next}}", "view": "{{$.View}}", "role": "{{$.Role}}", "q": "{{$.Query}}"}'>{{$next}}</button>
        {{end}}
      {{end}}
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
