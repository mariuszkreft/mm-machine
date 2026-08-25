package devloop

const devHTML = `{{define "dev"}}<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Montage Manager · dev loop</title>
  <script src="https://unpkg.com/htmx.org@2.0.3"></script>
  <link rel="stylesheet" href="/static/base.css?v=2">
  <link rel="stylesheet" href="/static/dev.css?v=3">
</head>
<body>
<div class="mm-shell">
  <header class="mm-top">
    <a class="mm-brand" href="/"><span class="mm-brand-mark">MM</span><span>Montage Manager</span></a>
    <nav><a href="/">App</a><a href="/offers">Pipeline</a><a href="/dev/backlog.json">backlog.json</a></nav>
  </header>
  <main class="mm-main">
    <section class="mm-ask">
      <h1>What users told the app about itself</h1>
      <p class="mm-lede mm-mono">Generated {{.Generated}} · {{.Model}} · refresh every {{.Interval}}</p>
      <form id="filter-form" class="filter-bar">
        <label>Kind
          <select name="kind" hx-get="/dev/filter" hx-trigger="change" hx-include="#filter-form" hx-target="#workspace-body" hx-swap="outerHTML">
            <option value="" {{if eq .Filters.Kind ""}}selected{{end}}>all</option>
            {{range $k, $n := .Counts.ByKind}}<option value="{{$k}}" {{if eq $.Filters.Kind $k}}selected{{end}}>{{$k}} ({{$n}})</option>{{end}}
          </select>
        </label>
        <label>Status
          <select name="status" hx-get="/dev/filter" hx-trigger="change" hx-include="#filter-form" hx-target="#workspace-body" hx-swap="outerHTML">
            <option value="" {{if eq .Filters.Status ""}}selected{{end}}>all</option>
            {{range $s := (slice "proposed" "accepted" "shipped" "rejected")}}<option value="{{$s}}" {{if eq $.Filters.Status $s}}selected{{end}}>{{$s}}</option>{{end}}
          </select>
        </label>
        <button type="button" class="mm-btn" hx-post="/dev/refresh" hx-include="#filter-form" hx-target="#workspace-body" hx-swap="outerHTML">Regenerate backlog</button>
      </form>
      {{if .LastError}}<div class="dev-error">Last regeneration failed: {{.LastError}}</div>{{end}}
      {{template "workspace" .}}
    </section>
  </main>
  <footer class="mm-foot">
    <span>dev loop · the app's own backlog, written by its users</span>
    <a href="/">back to the app</a>
  </footer>
</div>
</body>
</html>{{end}}`

const workspaceHTML = `{{define "workspace"}}<div id="workspace-body">
  {{template "counts" .}}
  {{template "backlog" .}}
  {{template "feedback" .}}
</div>{{end}}`

const countsHTML = `{{define "counts"}}<div class="counts-row">
  <div class="count-tile"><strong>{{.Counts.Total}}</strong><span>feedback total</span></div>
  <div class="count-tile"><strong>{{.Counts.New}}</strong><span>new</span></div>
  <div class="count-tile"><strong>{{.Counts.Triaged}}</strong><span>triaged</span></div>
  {{range $k, $n := .Counts.ByKind}}<div class="count-tile"><strong>{{$n}}</strong><span>{{$k}}</span></div>{{end}}
  <div class="count-tile"><strong>{{.LastRun}}</strong><span>last regenerated</span></div>
</div>{{end}}`

const backlogHTML = `{{define "backlog"}}<h2 class="dev-heading">Backlog</h2>
<div class="backlog">
  {{range .Backlog}}
  <article class="mm-card backlog-item">
    <div class="mm-card-head">
      <strong>{{.Title}}</strong>
      <span class="mm-badge status-{{.Status}}">{{.Status}}</span>
      <span class="mm-fit">{{printf "%.1f" .Score}}</span>
    </div>
    <div class="mm-meter"><i style="width:{{printf "%.0f" .ScorePct}}%"></i></div>
    <p>{{.Rationale}}</p>
    <span class="mm-muted mm-mono">{{.Count}} report(s) · avg severity {{printf "%.1f" .AvgSeverity}} · {{.Kind}} · {{.Theme}}</span>
    <ul class="mm-why">{{range .Evidence}}<li>{{.}}</li>{{end}}</ul>
    <div class="backlog-actions">
      <button class="mm-chip" hx-post="/dev/backlog/{{.ID}}/status" hx-vals='{"status":"accepted"}' hx-include="#filter-form" hx-target="#workspace-body" hx-swap="outerHTML">Accept</button>
      <button class="mm-chip" hx-post="/dev/backlog/{{.ID}}/status" hx-vals='{"status":"shipped"}' hx-include="#filter-form" hx-target="#workspace-body" hx-swap="outerHTML">Ship</button>
      <button class="mm-chip" hx-post="/dev/backlog/{{.ID}}/status" hx-vals='{"status":"rejected"}' hx-include="#filter-form" hx-target="#workspace-body" hx-swap="outerHTML">Reject</button>
    </div>
  </article>
  {{else}}
  <div class="mm-empty">Backlog is empty. Collect feedback, then regenerate.</div>
  {{end}}
</div>{{end}}`

const feedbackHTML = `{{define "feedback"}}<h2 class="dev-heading">Raw feedback</h2>
<div class="feedback-list">
  {{range .Feedback}}
  <article class="mm-card feedback-item">
    <span class="mm-badge kind-{{.Kind}}">{{.Kind}}</span>
    <p>{{.Verbatim}}</p>
    <span class="mm-muted mm-mono">cluster: {{.Theme}} · severity {{.Severity}} · {{.Source}} · {{.Role}} · {{.Status}}</span>
  </article>
  {{else}}
  <div class="mm-empty">No feedback yet. Ask the app a question on the home page and tell it what is wrong.</div>
  {{end}}
</div>{{end}}`
