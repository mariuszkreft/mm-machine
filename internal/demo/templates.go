package demo

const pageHTML = `{{define "demo"}}<!doctype html>
<html lang="{{.T.Code}}">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.T.T "demo.title"}} · Montage Manager</title>
  <script src="https://unpkg.com/htmx.org@2.0.3"></script>
  <link rel="stylesheet" href="/static/base.css?v=2">
  <link rel="stylesheet" href="/static/demo.css?v=1">
</head>
<body>
<div class="mm-shell">
  <header class="mm-top">
    <a class="mm-brand" href="/"><span class="mm-brand-mark">MM</span><span>Montage Manager</span></a>
    <nav>
      <a href="/offers">{{.T.T "nav.pipeline"}}</a>
      <a href="/about">{{.T.T "nav.about"}}</a>
      <a href="/dev">{{.T.T "nav.dev"}}</a>
    </nav>
  </header>
  <main class="mm-main">
    <section class="mm-ask">
      <h1>{{.T.T "demo.title"}}</h1>
      <p class="mm-lede">{{.T.T "demo.explain"}}</p>
    </section>

    <div class="mm-cards demo-grid">
      {{range .Personas}}
      <article class="mm-card demo-card">
        <div class="mm-card-head">
          <span class="mm-badge">{{.Role}}</span>
          {{if .CrewSize}}<span class="mm-mono">{{.CrewSize}}</span>{{end}}
        </div>
        <h3>{{.Label}}</h3>
        <p>{{.Summary}}</p>
        <div class="mm-suggest">
          {{range .Trades}}<span class="mm-chip">{{.}}</span>{{end}}
          {{range .Regions}}<span class="mm-chip">{{.}}</span>{{end}}
          {{range .Documents}}<span class="mm-chip">{{.}}</span>{{end}}
        </div>
        <ul class="mm-why">{{range .SampleAsks}}<li>{{.}}</li>{{end}}</ul>
        <form method="post" action="/demo/enter">
          <input type="hidden" name="persona" value="{{.Key}}">
          <button class="mm-btn" type="submit">{{$.T.T "demo.enter"}}</button>
        </form>
      </article>
      {{end}}
    </div>

    {{if .Active}}
    <form method="post" action="/demo/leave" class="demo-leave">
      <button class="mm-btn ghost" type="submit">{{.T.T "demo.leave"}}</button>
    </form>
    {{end}}
  </main>
  <footer class="mm-foot"><a href="/">Montage Manager</a></footer>
</div>
</body>
</html>{{end}}`
