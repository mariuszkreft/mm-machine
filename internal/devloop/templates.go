package devloop

const devHTML = `{{define "dev"}}<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Montage Manager · dev loop</title>
  <script src="https://unpkg.com/htmx.org@2.0.3"></script>
  <link rel="stylesheet" href="/static/app.css?v=3">
  <link rel="stylesheet" href="/static/dev.css?v=1">
</head>
<body class="dev">
  <header class="topbar">
    <a class="brand" href="/"><span class="brand-mark">MM</span><span>Montage Manager</span></a>
    <nav class="nav"><a href="/">App</a><a href="/dev">Dev loop</a><a href="/dev/backlog.json">backlog.json</a></nav>
  </header>
  <main>
    <section class="workspace">
      <div class="section-title">
        <p class="eyebrow">Generated {{.Generated}} · {{.Model}}</p>
        <h2>What users told the app about itself</h2>
      </div>
      <button class="primary" hx-post="/dev/refresh" hx-target="#backlog" hx-swap="outerHTML">Regenerate backlog</button>
      {{template "backlog" .}}
      <div class="section-title"><h2>Raw feedback</h2></div>
      <div class="feedback-list">
        {{range .Feedback}}
        <article class="feedback-item">
          <span class="badge {{.Kind}}">{{.Kind}}</span>
          <p>{{.Verbatim}}</p>
          <span class="meta">{{.Theme}} · severity {{.Severity}} · {{.Source}} · {{.Role}}</span>
        </article>
        {{else}}
        <div class="empty">No feedback yet. Talk to the assistant on the home page.</div>
        {{end}}
      </div>
    </section>
  </main>
</body>
</html>{{end}}`

const backlogHTML = `{{define "backlog"}}<div id="backlog" class="backlog">
  {{range .Backlog}}
  <article class="backlog-item">
    <div class="backlog-top"><strong>{{.Title}}</strong><span class="score">{{printf "%.1f" .Score}}</span></div>
    <p>{{.Rationale}}</p>
    <span class="meta">{{.Count}} reports · avg severity {{printf "%.1f" .AvgSeverity}} · {{.Status}}</span>
    <ul>{{range .Evidence}}<li>{{.}}</li>{{end}}</ul>
  </article>
  {{else}}
  <div class="empty">Backlog is empty. Collect feedback, then regenerate.</div>
  {{end}}
</div>{{end}}`
