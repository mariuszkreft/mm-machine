package devloop

const devHTML = `{{define "dev"}}<!doctype html>
<html lang="{{.T.Code}}">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.T.T "dev.pageTitle"}}</title>
  <script src="https://unpkg.com/htmx.org@2.0.3"></script>
  <link rel="stylesheet" href="/static/base.css?v=2">
  <link rel="stylesheet" href="/static/dev.css?v=3">
</head>
<body>
<div class="mm-shell">
  <header class="mm-top">
    <a class="mm-brand" href="/"><span class="mm-brand-mark">MM</span><span>Montage Manager</span></a>
    <nav>
      <a href="/">{{.T.T "dev.navApp"}}</a>
      <a href="/offers">{{.T.T "nav.pipeline"}}</a>
      <a href="/dev/backlog.json">backlog.json</a>
      <a class="mm-lang" href="/lang?to={{if .T.Is "de"}}en{{else}}de{{end}}" rel="nofollow" aria-label="{{if .T.Is "de"}}{{.T.T "lang.switchTo.en"}}{{else}}{{.T.T "lang.switchTo.de"}}{{end}}">{{if .T.Is "de"}}English{{else}}Deutsch{{end}}</a>
    </nav>
  </header>
  <main class="mm-main">
    <section class="mm-ask">
      <h1>{{.T.T "dev.title"}}</h1>
      <p class="mm-lede">{{.T.T "dev.explain"}}</p>
      <p class="mm-lede mm-mono">{{printf (.T.T "dev.generatedLine") .Generated .Model .Interval}}</p>
      <form id="filter-form" class="filter-bar">
        <label>{{.T.T "dev.filterKind"}}
          <select name="kind" hx-get="/dev/filter" hx-trigger="change" hx-include="#filter-form" hx-target="#workspace-body" hx-swap="outerHTML">
            <option value="" {{if eq .Filters.Kind ""}}selected{{end}}>{{.T.T "dev.filterAll"}}</option>
            {{range $k, $n := .Counts.ByKind}}<option value="{{$k}}" {{if eq $.Filters.Kind $k}}selected{{end}}>{{$.T.T (print "feedback.kind." $k)}} ({{$n}})</option>{{end}}
          </select>
        </label>
        <label>{{.T.T "dev.filterStatus"}}
          <select name="status" hx-get="/dev/filter" hx-trigger="change" hx-include="#filter-form" hx-target="#workspace-body" hx-swap="outerHTML">
            <option value="" {{if eq .Filters.Status ""}}selected{{end}}>{{.T.T "dev.filterAll"}}</option>
            {{range $s := (slice "proposed" "accepted" "shipped" "rejected")}}<option value="{{$s}}" {{if eq $.Filters.Status $s}}selected{{end}}>{{$.T.T (print "backlog.status." $s)}}</option>{{end}}
          </select>
        </label>
        <button type="button" class="mm-btn" hx-post="/dev/refresh" hx-include="#filter-form" hx-target="#workspace-body" hx-swap="outerHTML">{{.T.T "dev.regenerate"}}</button>
      </form>
      {{if .LastError}}<div class="dev-error">{{printf (.T.T "dev.regenFailed") .LastError}}</div>{{end}}
      {{template "workspace" .}}
    </section>
  </main>
  <footer class="mm-foot">
    <span>{{.T.T "dev.footerTagline"}}</span>
    <a href="/">{{.T.T "dev.backToApp"}}</a>
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
  <div class="count-tile"><strong>{{.Counts.Total}}</strong><span>{{.T.T "dev.countTotal"}}</span></div>
  <div class="count-tile"><strong>{{.Counts.New}}</strong><span>{{.T.T "dev.countNew"}}</span></div>
  <div class="count-tile"><strong>{{.Counts.Triaged}}</strong><span>{{.T.T "dev.countTriaged"}}</span></div>
  {{range $k, $n := .Counts.ByKind}}<div class="count-tile"><strong>{{$n}}</strong><span>{{$.T.T (print "feedback.kind." $k)}}</span></div>{{end}}
  <div class="count-tile"><strong>{{.LastRun}}</strong><span>{{.T.T "dev.countLastRun"}}</span></div>
</div>{{end}}`

const backlogHTML = `{{define "backlog"}}<h2 class="dev-heading">{{.T.T "dev.backlogHeading"}}</h2>
<div class="backlog">
  {{range .Backlog}}
  <article class="mm-card backlog-item">
    <div class="mm-card-head">
      <strong>{{.Title}}</strong>
      <span class="mm-badge status-{{.Status}}">{{$.T.T (print "backlog.status." .Status)}}</span>
      <span class="mm-fit">{{printf "%.1f" .Score}}</span>
    </div>
    <div class="mm-meter"><i style="width:{{printf "%.0f" .ScorePct}}%"></i></div>
    <p>{{.Rationale}}</p>
    <span class="mm-muted mm-mono">{{printf ($.T.T "dev.reportCount") .Count .AvgSeverity ($.T.T (print "feedback.kind." .Kind)) .Theme}}</span>
    <ul class="mm-why">{{range .Evidence}}<li>{{.}}</li>{{end}}</ul>
    <div class="backlog-actions">
      <button class="mm-chip" hx-post="/dev/backlog/{{.ID}}/status" hx-vals='{"status":"accepted"}' hx-include="#filter-form" hx-target="#workspace-body" hx-swap="outerHTML">{{$.T.T "dev.accept"}}</button>
      <button class="mm-chip" hx-post="/dev/backlog/{{.ID}}/status" hx-vals='{"status":"shipped"}' hx-include="#filter-form" hx-target="#workspace-body" hx-swap="outerHTML">{{$.T.T "dev.ship"}}</button>
      <button class="mm-chip" hx-post="/dev/backlog/{{.ID}}/status" hx-vals='{"status":"rejected"}' hx-include="#filter-form" hx-target="#workspace-body" hx-swap="outerHTML">{{$.T.T "dev.reject"}}</button>
    </div>
  </article>
  {{else}}
  <div class="mm-empty">{{.T.T "dev.backlogEmpty"}}</div>
  {{end}}
</div>{{end}}`

const feedbackHTML = `{{define "feedback"}}<h2 class="dev-heading">{{.T.T "dev.feedbackHeading"}}</h2>
<div class="feedback-list">
  {{range .Feedback}}
  {{$status := .Status}}{{if not $status}}{{$status = "new"}}{{end}}
  {{$role := .Role}}{{if not $role}}{{$role = "unknown"}}{{end}}
  <article class="mm-card feedback-item">
    <span class="mm-badge kind-{{.Kind}}">{{$.T.T (print "feedback.kind." .Kind)}}</span>
    <p>{{.Verbatim}}</p>
    <span class="mm-muted mm-mono">{{printf ($.T.T "dev.feedbackMeta") .Theme .Severity ($.T.T (print "feedback.source." .Source)) ($.T.T (print "role." $role)) ($.T.T (print "feedback.status." $status))}}</span>
  </article>
  {{else}}
  <div class="mm-empty">{{.T.T "dev.feedbackEmpty"}}</div>
  {{end}}
</div>{{end}}`
