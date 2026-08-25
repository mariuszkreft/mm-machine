package web

// aboutHTML is the story surface: what the product is and where it is going.
// It is deliberately not the home page — the working surface is — so it
// carries pitch content but none of the pipeline's mutating controls.
const aboutHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Montage Manager — what it is</title>
  <script src="https://unpkg.com/htmx.org@2.0.3"></script>
  <link rel="stylesheet" href="/static/base.css?v=2">
  <link rel="stylesheet" href="/static/app.css?v=5">
  <link rel="stylesheet" href="/static/assistant.css?v=1">
</head>
<body>
<div class="mm-shell">
  <header class="mm-top">
    <a class="mm-brand" href="/"><span class="mm-brand-mark">MM</span><span>Montage Manager</span></a>
    <nav aria-label="Primary">
      <a href="/">Home</a>
      <a href="/offers">Pipeline</a>
      <a href="/dev">Dev loop</a>
    </nav>
  </header>

  <main class="mm-main">
    <section class="mm-hero-grid">
      <div class="mm-ask">
        <h1>The operating system for direct subcontractor work.</h1>
        <p class="mm-lede">Montage Manager connects GUs and SUs without opaque broker layers: structured projects, verified teams, document safes, progress proof, payment signals, and dispute evidence.</p>
        <div class="mm-suggest">
          <a class="mm-btn" href="#assistant">Ask the assistant</a>
          <a class="mm-btn ghost" href="/offers">Open pipeline</a>
        </div>
      </div>
      <div class="mm-panel" aria-label="Live command panel">
        <div class="mm-card-head">
          <span>Live command</span>
          <strong class="mm-mono">{{.Now}}</strong>
        </div>
        <div class="mm-stats">
          <div class="mm-stat"><span>All</span><strong>{{index .Counts "all"}}</strong></div>
          <div class="mm-stat"><span>In process</span><strong>{{index .Counts "process"}}</strong></div>
          <div class="mm-stat"><span>Open</span><strong>{{index .Counts "open"}}</strong></div>
        </div>
        <div class="mm-panel">
          <span class="mm-mono">{{.Spotlight.ID}}</span>
          <h3>{{.Spotlight.Title}}</h3>
          <p class="mm-muted">{{.Spotlight.Attention}}</p>
          <div class="mm-meter" role="progressbar" aria-valuenow="{{.Spotlight.Progress}}" aria-valuemin="0" aria-valuemax="100" aria-label="Spotlight progress"><i style="width: {{.Spotlight.Progress}}%"></i></div>
        </div>
        <img class="mm-map" src="/static/reference-map.jpg" alt="Montage Manager structure map">
      </div>
    </section>

    <section class="mm-section" id="assistant">
      <p class="mm-eyebrow">Local AI · {{.LLMModel}}</p>
      <h2>Talk to the app about the app</h2>
      <div id="assistant-panel" hx-get="/assistant/panel?role={{.Role}}&route=home" hx-trigger="load" hx-swap="outerHTML">
        <div class="mm-skel" style="width: 40%; height: 1.5em"></div>
      </div>
    </section>

    <section class="mm-section" id="perspectives">
      <p class="mm-eyebrow">Two-sided product</p>
      <h2>One platform, two operating realities</h2>
      <div class="mm-steps" role="tablist" aria-label="Perspective" hx-target="#perspective-panel" hx-swap="outerHTML">
        {{range .Perspectives}}
        <button class="mm-chip {{if eq $.Role .Key}}on{{end}}" type="button" role="tab" aria-selected="{{if eq $.Role .Key}}true{{else}}false{{end}}" hx-get="/perspective?role={{.Key}}&view={{$.View}}&q={{$.Query}}">{{.Label}}</button>
        {{end}}
      </div>
      {{template "perspective" .}}
    </section>

    <section class="mm-section" id="modules">
      <p class="mm-eyebrow">Product depth</p>
      <h2>Modules that remove the broker bottleneck</h2>
      <div class="mm-cards">
        {{range .Modules}}
        <article class="mm-card">
          <h3>{{.Name}}</h3>
          <p class="mm-muted">{{.Body}}</p>
          <span class="mm-mono">{{.Impact}}</span>
        </article>
        {{end}}
      </div>
    </section>

    <section class="mm-section" id="roadmap">
      <p class="mm-eyebrow">Roadmap</p>
      <h2>Start narrow, then own the transaction</h2>
      <div class="mm-cards">
        {{range .Roadmap}}
        <article class="mm-card">
          <span class="mm-badge">{{.Phase}}</span>
          <h3>{{.Title}}</h3>
          <p class="mm-muted">{{.Body}}</p>
        </article>
        {{end}}
      </div>
    </section>
  </main>

  <footer class="mm-foot">
    <span>Montage Manager {{.Version}}</span>
    <span class="mm-mono">local model: {{.LLMModel}}</span>
    <a href="/">home</a>
    <a href="/dev">dev loop</a>
  </footer>
</div>
</body>
</html>`

// offersHTML is the pipeline grid: an htmx fragment swapped into both the
// standalone /offers page and any partial re-render (filter, create, status
// change). Every card carries the same explanation discipline as a search
// result — status, signal and what needs attention are never implicit.
const offersHTML = `{{define "offers"}}<div id="offers" class="mm-cards" role="region" aria-live="polite" aria-label="Pipeline">
  {{range .Offers}}
  <article class="mm-card">
    <div class="mm-card-head">
      <span class="mm-mono">{{.ID}}</span>
      <span class="mm-badge {{signalTone .Signal}}">{{.Signal}}</span>
      <span class="mm-badge">{{.Status}}</span>
    </div>
    <h3>{{.Title}}</h3>
    <p class="mm-muted">{{.Location}} · {{.Category}} · {{.Budget}}</p>
    <div class="mm-meter" role="progressbar" aria-valuenow="{{.Progress}}" aria-valuemin="0" aria-valuemax="100" aria-label="{{.Title}} progress"><i style="width: {{.Progress}}%"></i></div>
    <ul class="mm-why">
      <li>Status: {{.Status}}</li>
      <li>Signal: {{.Signal}}</li>
      {{if .Attention}}<li>{{.Attention}}</li>{{end}}
      <li>Updated {{.Updated}}</li>
    </ul>
    {{$id := .ID}}{{$cur := lower .Status}}
    <div class="mm-steps" role="group" aria-label="Move {{.Title}} to" hx-target="#offers" hx-swap="outerHTML">
      {{range $next := $.Statuses}}
        {{if eq $next $cur}}
        <button class="mm-chip on" type="button" disabled aria-current="step">{{$next}}</button>
        {{else}}
        <button class="mm-chip" type="button" hx-post="/offers/status"
                hx-vals='{"id": "{{$id}}", "status": "{{$next}}", "view": "{{$.View}}", "role": "{{$.Role}}", "q": "{{$.Query}}"}'>{{$next}}</button>
        {{end}}
      {{end}}
    </div>
  </article>
  {{else}}
  <div class="mm-empty">No offers match this view.</div>
  {{end}}
</div>{{end}}`

// pipelineToolbarHTML is the filter/search/create bar above the offers grid.
// It is boosted so plain links and forms work without htmx, and degrades to
// the standalone pipeline page on direct navigation.
const pipelineToolbarHTML = `{{define "pipeline-toolbar"}}<div class="mm-toolbar">
  <div class="mm-steps" role="tablist" aria-label="Pipeline view" hx-boost="true" hx-target="#offers" hx-swap="outerHTML">
    <a class="mm-chip {{if eq .View "all"}}on{{end}}" role="tab" aria-selected="{{if eq .View "all"}}true{{else}}false{{end}}" href="/offers?view=all&role={{.Role}}">All <span class="mm-mono">{{index .Counts "all"}}</span></a>
    <a class="mm-chip {{if eq .View "open"}}on{{end}}" role="tab" aria-selected="{{if eq .View "open"}}true{{else}}false{{end}}" href="/offers?view=open&role={{.Role}}">Open <span class="mm-mono">{{index .Counts "open"}}</span></a>
    <a class="mm-chip {{if eq .View "requested"}}on{{end}}" role="tab" aria-selected="{{if eq .View "requested"}}true{{else}}false{{end}}" href="/offers?view=requested&role={{.Role}}">Requested <span class="mm-mono">{{index .Counts "requested"}}</span></a>
    <a class="mm-chip {{if eq .View "process"}}on{{end}}" role="tab" aria-selected="{{if eq .View "process"}}true{{else}}false{{end}}" href="/offers?view=process&role={{.Role}}">Process <span class="mm-mono">{{index .Counts "process"}}</span></a>
    <a class="mm-chip {{if eq .View "done"}}on{{end}}" role="tab" aria-selected="{{if eq .View "done"}}true{{else}}false{{end}}" href="/offers?view=done&role={{.Role}}">Done <span class="mm-mono">{{index .Counts "done"}}</span></a>
  </div>
  <form role="search" hx-get="/offers" hx-target="#offers" hx-trigger="input changed delay:220ms, submit" hx-swap="outerHTML">
    <input type="hidden" name="view" value="{{.View}}">
    <input type="hidden" name="role" value="{{.Role}}">
    <label class="mm-sr" for="mm-offer-search">Search offers, suppliers, cities</label>
    <input id="mm-offer-search" class="mm-field" name="q" value="{{.Query}}" placeholder="Search offers, suppliers, cities" autocomplete="off">
  </form>
</div>
<details class="mm-panel">
  <summary>New offer</summary>
  <form class="mm-form" hx-post="/offers/new" hx-target="#offers" hx-swap="outerHTML">
    <label>Title <input class="mm-field" name="title" required></label>
    <label>Location <input class="mm-field" name="location" placeholder="City, country"></label>
    <label>Category <input class="mm-field" name="category"></label>
    <label>Budget <input class="mm-field" name="budget"></label>
    <label>Supplier <input class="mm-field" name="supplier"></label>
    <label>Status
      <select class="mm-field" name="status">
        <option value="open">open</option>
        <option value="requested">requested</option>
        <option value="process">process</option>
        <option value="done">done</option>
      </select>
    </label>
    <button class="mm-btn" type="submit">Create offer</button>
  </form>
</details>{{end}}`

// pipelineHTML is the standalone pipeline page: the offer control surface as
// its own working page rather than a fragment (previously only reachable as
// an htmx partial, so a direct visit to /offers rendered no page at all).
const pipelineHTML = `{{define "pipeline"}}<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Montage Manager — pipeline</title>
  <script src="https://unpkg.com/htmx.org@2.0.3"></script>
  <link rel="stylesheet" href="/static/base.css?v=2">
</head>
<body>
<div class="mm-shell">
  <header class="mm-top">
    <a class="mm-brand" href="/"><span class="mm-brand-mark">MM</span><span>Montage Manager</span></a>
    <nav aria-label="Primary">
      <a href="/">Home</a>
      <a href="/about">What it is</a>
      <a href="/dev">Dev loop</a>
    </nav>
  </header>

  <main class="mm-main">
    <div class="mm-ask">
      <h1>Pipeline</h1>
      <p class="mm-lede">Every offer, one control surface: status, signal and what needs attention, together.</p>
    </div>
    {{template "pipeline-toolbar" .}}
    {{template "offers" .}}
  </main>

  <footer class="mm-foot">
    <span>Montage Manager {{.Version}}</span>
    <a href="/">home</a>
    <a href="/about">what it is</a>
  </footer>
</div>
</body>
</html>{{end}}`

// perspectiveHTML is the role-switch fragment: the same panel renders inline
// on /about and is swapped in place when a visitor picks a different role.
const perspectiveHTML = `{{define "perspective"}}<div id="perspective-panel">
  <div class="mm-panel">
    <p class="mm-eyebrow">{{.Perspective.Label}}</p>
    <h3>{{.Perspective.Title}}</h3>
    <p class="mm-lede">{{.Perspective.Subtitle}}</p>
    <p class="mm-lede">{{.Perspective.Quote}}</p>
    <div class="mm-suggest">
      <a class="mm-btn" href="/offers">{{.Perspective.Primary}}</a>
      <a class="mm-btn ghost" href="/about#modules">{{.Perspective.Secondary}}</a>
    </div>
  </div>
  <div class="mm-cards">
    {{range .Perspective.Stats}}
    <article class="mm-card">
      <div class="mm-stat"><span>{{.Label}}</span><strong>{{.Value}}</strong></div>
      <p class="mm-muted">{{.Note}}</p>
    </article>
    {{end}}
  </div>
  <div class="mm-cards">
    <div class="mm-panel">
      <strong>{{.Perspective.ActionName}}</strong>
      <ol>{{range .Perspective.Workflow}}<li>{{.}}</li>{{end}}</ol>
    </div>
    <div class="mm-panel">
      <strong>Decision pressure</strong>
      <ul>{{range .Perspective.Pain}}<li>{{.}}</li>{{end}}</ul>
    </div>
  </div>
</div>{{end}}`

// shellHTML is the home surface: one prompt, one thread, nothing else above
// the fold. Everything a visitor can do starts as a sentence. Header and
// prompt are pinned together so the prompt stays reachable as the thread
// grows; an in-flight skeleton reserves the shape of the answer that is
// coming so nothing jumps when it lands.
const shellHTML = `{{define "shell"}}<!doctype html>
<html lang="{{.T.Code}}">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Montage Manager</title>
  <script src="https://unpkg.com/htmx.org@2.0.3"></script>
  <script src="https://unpkg.com/htmx-ext-sse@2.2.2/sse.js"></script>
  <link rel="stylesheet" href="/static/base.css?v=2">
  <link rel="stylesheet" href="/static/onboarding.css?v=1">
  <link rel="stylesheet" href="/static/search.css?v=1">
</head>
<body>
<div class="mm-shell">
  <div class="mm-topzone">
    <header class="mm-top">
      <a class="mm-brand" href="/"><span class="mm-brand-mark">MM</span><span>Montage Manager</span></a>
      <nav aria-label="Primary">
        <a href="/offers">{{.T.T "nav.pipeline"}}</a>
        <a href="/demo">{{.T.T "nav.demo"}}</a>
        <a href="/about">{{.T.T "nav.about"}}</a>
        <a href="/dev">{{.T.T "nav.dev"}}</a>
        <a class="mm-lang" href="/lang?to={{if .T.Is "de"}}en{{else}}de{{end}}" rel="nofollow">{{if .T.Is "de"}}EN{{else}}DE{{end}}</a>
      </nav>
    </header>

    {{if .PersonaLabel}}
    <div class="mm-demo-banner" role="status">
      <span>{{.PersonaLabel}}</span>
      <form method="post" action="/demo/leave"><button class="mm-btn quiet" type="submit">{{.T.T "demo.leave"}}</button></form>
    </div>
    {{end}}

    <section class="mm-ask">
      <h1>{{.Headline}}</h1>
      <p class="mm-lede">{{.Lede}}</p>
      <form class="mm-prompt"
            hx-post="/ask"
            hx-target="#mm-thread"
            hx-swap="beforeend"
            hx-indicator="#mm-busy"
            hx-on::before-request="mmThread.pending()"
            hx-on::after-swap="mmThread.settle()"
            hx-on::after-request="this.reset(); this.querySelector('input').focus()">
        <input id="mm-input" name="message" autocomplete="off" autofocus
               placeholder="{{.Placeholder}}" aria-label="{{.Headline}}" aria-keyshortcuts="/">
        <button class="mm-btn" type="submit">{{.T.T "home.send"}}</button>
      </form>
      <div class="mm-suggest">
        <span class="mm-muted">{{.T.T "home.tryThis"}}</span>
        {{range .Suggestions}}
        <button class="mm-chip" type="button" hx-post="/ask" hx-target="#mm-thread" hx-swap="beforeend"
                hx-vals='{"message": "{{.}}"}' hx-indicator="#mm-busy"
                hx-on::before-request="mmThread.pending()" hx-on::after-swap="mmThread.settle()">{{.}}</button>
        {{end}}
        <span id="mm-busy" class="htmx-indicator mm-muted" role="status">{{.T.T "home.busy"}}</span>
      </div>
    </section>
  </div>

  <main class="mm-main">
    <div class="mm-thread" id="mm-thread" role="log" aria-live="polite" aria-label="Conversation">
      {{template "greeting" .}}
    </div>
  </main>

  <footer class="mm-foot">
    <span>Montage Manager {{.Version}}</span>
    <span class="mm-mono">{{.LLMModel}}</span>
    <a href="/about">{{.T.T "nav.about"}}</a>
    <a href="/demo">{{.T.T "nav.demo"}}</a>
    <a href="/dev">{{.T.T "nav.dev"}}</a>
  </footer>
</div>

<template id="mm-pending-tpl">
  <div class="mm-msg mm" id="mm-pending">
    <span class="mm-who">mm</span>
    <span class="mm-sr">Thinking…</span>
    <div class="mm-skel" style="width: 70%"></div>
    <div class="mm-skel" style="width: 45%"></div>
  </div>
</template>

<script>
  window.mmThread = {
    pending: function () {
      var thread = document.getElementById('mm-thread');
      var tpl = document.getElementById('mm-pending-tpl');
      if (thread && tpl) thread.appendChild(tpl.content.cloneNode(true));
    },
    settle: function () {
      var pending = document.getElementById('mm-pending');
      if (pending) pending.remove();
    }
  };
  document.addEventListener('keydown', function (e) {
    var el = document.activeElement;
    var typing = el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.isContentEditable);
    if (e.key === '/' && !typing) {
      e.preventDefault();
      var input = document.getElementById('mm-input');
      if (input) input.focus();
    } else if (e.key === 'Escape' && el && el.id === 'mm-input') {
      el.value = '';
    }
  });
</script>
</body>
</html>{{end}}`

// greetingHTML is the thread's first message: it differs for someone the app
// already knows, which is the whole point of onboarding. A returning visitor
// leads with the concrete facts the app already has, not just a sentence.
const greetingHTML = `{{define "greeting"}}<div class="mm-msg mm">
  <span class="mm-who">mm</span>
  {{if .Profile.Known}}
  <p>{{printf (.T.T "greeting.known") .ProfileLine}}</p>
  {{else}}
  <p>{{.T.T "greeting.new"}}</p>
  {{end}}
</div>{{end}}`
