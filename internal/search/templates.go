package search

const resultsHTML = `{{define "results"}}<div class="mm-msg mm">
  <span class="mm-who">mm</span>
  <p>{{.Summary}}</p>
  {{if .Degraded}}<span class="mm-badge warn">matched without the model</span>{{end}}
</div>
<div class="mm-cards" id="mm-results">
  {{range .Matches}}
  <article class="mm-card">
    <div class="mm-card-head">
      <span class="mm-mono">{{.Offer.ID}}</span>
      <span class="mm-badge">{{.Offer.Status}}</span>
      <span class="mm-fit">{{.Fit}}%</span>
    </div>
    <h3>{{.Offer.Title}}</h3>
    <div class="mm-card-head">{{.Offer.Location}} · {{.Offer.Category}} · {{.Offer.Budget}}</div>
    <div class="mm-meter"><i style="width: {{.Fit}}%"></i></div>
    <ul class="mm-why">{{range .Why}}<li>{{.}}</li>{{end}}</ul>
  </article>
  {{else}}
  <div class="mm-empty">Nothing matches that yet. Try a different trade, region or timeframe.</div>
  {{end}}
</div>
<form class="mm-suggest" hx-post="/find/save" hx-swap="outerHTML">
  <input type="hidden" name="q" value="{{.Query}}">
  <button class="mm-chip" type="submit">save this search</button>
</form>{{end}}`
