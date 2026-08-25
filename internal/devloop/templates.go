package devloop

const devHTML = `{{define "dev"}}<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Montage Manager · dev loop</title>
  <script src="https://unpkg.com/htmx.org@2.0.3"></script>
  <link rel="stylesheet" href="/static/app.css?v=4">
  <link rel="stylesheet" href="/static/dev.css?v=2">
</head>
<body class="dev">
  <header class="topbar">
    <a class="brand" href="/"><span class="brand-mark">MM</span><span>Montage Manager</span></a>
    <nav class="nav"><a href="/">App</a><a href="/dev">Dev loop</a><a href="/dev/backlog.json">backlog.json</a></nav>
  </header>
  <main>
    <section class="workspace">
      <div class="section-title">
        <p class="eyebrow">Generated {{.Generated}} · {{.Model}} · refresh every {{.Interval}}</p>
        <h2>What users told the app about itself</h2>
      </div>
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
        <button type="button" class="primary" hx-post="/dev/refresh" hx-include="#filter-form" hx-target="#workspace-body" hx-swap="outerHTML">Regenerate backlog</button>
      </form>
      {{if .LastError}}<div class="dev-error">Last regeneration failed: {{.LastError}}</div>{{end}}
      {{template "workspace" .}}
    </section>
  </main>
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

const backlogHTML = `{{define "backlog"}}<div class="section-title"><h2>Backlog</h2></div>
<div class="backlog">
  {{range .Backlog}}
  <article class="backlog-item">
    <div class="backlog-top">
      <strong>{{.Title}}</strong>
      <span class="status status-{{.Status}}">{{.Status}}</span>
      <span class="score">{{printf "%.1f" .Score}}</span>
    </div>
    <div class="score-bar"><span style="width:{{printf "%.0f" .ScorePct}}%"></span></div>
    <p>{{.Rationale}}</p>
    <span class="meta">{{.Count}} report(s) · avg severity {{printf "%.1f" .AvgSeverity}} · {{.Kind}} · {{.Theme}}</span>
    <ul>{{range .Evidence}}<li>{{.}}</li>{{end}}</ul>
    <div class="backlog-actions">
      <button hx-post="/dev/backlog/{{.ID}}/status" hx-vals='{"status":"accepted"}' hx-include="#filter-form" hx-target="#workspace-body" hx-swap="outerHTML">Accept</button>
      <button hx-post="/dev/backlog/{{.ID}}/status" hx-vals='{"status":"shipped"}' hx-include="#filter-form" hx-target="#workspace-body" hx-swap="outerHTML">Ship</button>
      <button hx-post="/dev/backlog/{{.ID}}/status" hx-vals='{"status":"rejected"}' hx-include="#filter-form" hx-target="#workspace-body" hx-swap="outerHTML">Reject</button>
    </div>
  </article>
  {{else}}
  <div class="empty">Backlog is empty. Collect feedback, then regenerate.</div>
  {{end}}
</div>{{end}}`

const feedbackHTML = `{{define "feedback"}}<div class="section-title"><h2>Raw feedback</h2></div>
<div class="feedback-list">
  {{range .Feedback}}
  <article class="feedback-item">
    <span class="badge {{.Kind}}">{{.Kind}}</span>
    <p>{{.Verbatim}}</p>
    <span class="meta">cluster: {{.Theme}} · severity {{.Severity}} · {{.Source}} · {{.Role}} · {{.Status}}</span>
  </article>
  {{else}}
  <div class="empty">No feedback yet. Talk to the assistant on the home page.</div>
  {{end}}
</div>{{end}}`
