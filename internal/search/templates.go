package search

import (
	"fmt"
	"html/template"

	"mm-machine/internal/i18n"
)

// templateFuncs exposes the shared and package-local translation lookups to
// the templates below: t is the shared catalog (internal/i18n), tl is this
// package's own (see lang.go), so a card's per-item strings (status badge,
// crew headcount, kind badge) can be localized inline without threading a
// precomputed field through model.Match, which this package does not own.
var templateFuncs = template.FuncMap{
	"t":  func(lang i18n.Lang, key string) string { return i18n.T(lang, key) },
	"tl": tr,
	"offerStatusKey": func(status string) string {
		return fmt.Sprintf("offer.status.%s", status)
	},
}

// parseTemplates builds the one *template.Template every handler executes
// named templates from.
func parseTemplates() *template.Template {
	return template.Must(template.Must(template.New("search").Funcs(templateFuncs).Parse(resultsHTML)).Parse(savedHTML))
}

const resultsHTML = `{{define "results"}}{{$lang := .Lang}}<div class="mm-msg mm mm-summary" id="mm-summary">
  <span class="mm-who">mm</span>
  <p class="mm-summary-fallback">{{.Fallback}}</p>
  <p class="mm-summary-live" hx-ext="sse" sse-connect="{{.StreamURL}}" sse-swap="message" sse-close="done" hx-swap="beforeend"></p>
  {{if .Degraded}}<span class="mm-badge warn">{{t $lang "search.degraded"}}</span>{{end}}
  {{if .Widen.Applied}}<span class="mm-badge warn">{{printf (t $lang "search.widened") .Widen.Dropped}}</span>{{end}}
  <script>
  (function () {
    if (window.__mmSearchWired) return;
    window.__mmSearchWired = true;
    document.body.addEventListener('htmx:sseMessage', function (evt) {
      var msg = evt.target.closest ? evt.target.closest('.mm-summary') : null;
      if (msg) msg.classList.add('mm-live');
    });
    document.body.addEventListener('htmx:afterSwap', function () {
      var saved = document.querySelectorAll('.mm-saved');
      for (var i = 0; i < saved.length - 1; i++) saved[i].remove();
    });
  })();
  </script>
</div>
<div class="mm-cards" id="mm-results">
  {{range .Matches}}
  <article class="mm-card">
    <div class="mm-card-head">
      <span class="mm-mono">{{.Ref}}</span>
      <span class="mm-badge kind-{{.Kind}}">{{tl $lang (printf "search.kind.%s" .Kind)}}</span>
      {{if eq .Kind "crew"}}<span class="mm-badge good">{{tl $lang "search.crewCount" .Crew.Size}}</span>{{else}}<span class="mm-badge">{{t $lang (offerStatusKey .Offer.Status)}}</span>{{end}}
      <span class="mm-fit">{{.Fit}}%</span>
    </div>
    <h3>{{.Title}}</h3>
    {{if eq .Kind "crew"}}
    <div class="mm-card-head">{{.Crew.Company}}{{if .Crew.Regions}} · {{index .Crew.Regions 0}}{{end}}{{if .Crew.Rate}} · {{.Crew.Rate}}{{end}}</div>
    {{if .Crew.AvailableNote}}<div class="mm-muted mm-crew-availability">{{.Crew.AvailableNote}}</div>{{end}}
    {{else}}
    <div class="mm-card-head">{{.Offer.Location}} · {{.Offer.Category}} · {{.Offer.Budget}}</div>
    {{end}}
    <div class="mm-meter"><i style="width: {{.Fit}}%"></i></div>
    <ul class="mm-why">{{range .Why}}<li>{{.}}</li>{{end}}</ul>
  </article>
  {{else}}
  <div class="mm-empty">{{t $lang "search.nothing"}} {{t $lang "search.nothingHelp"}}</div>
  {{end}}
</div>
{{if .Chips}}
<div class="mm-refine" id="mm-refine">
  {{$q := .Query}}
  <span class="mm-refine-label">{{t $lang "search.refine"}}</span>
  {{range .Chips}}
  <form class="mm-refine-form" hx-post="/find" hx-target="#mm-thread" hx-swap="beforeend">
    <input type="hidden" name="q" value="{{$q}}">
    <input type="hidden" name="refine" value="{{.Refine}}">
    <button class="mm-chip" type="submit">{{.Label}}</button>
  </form>
  {{end}}
</div>
{{end}}
<form class="mm-suggest" hx-post="/find/save" hx-swap="outerHTML">
  <input type="hidden" name="q" value="{{.Query}}">
  <button class="mm-chip" type="submit">{{t $lang "search.save"}}</button>
</form>{{end}}`

const savedHTML = `{{define "saved"}}<div class="mm-saved" id="mm-saved">
  {{if .Searches}}
  {{range .Searches}}
  <span class="mm-saved-item">
    <form class="mm-saved-run" hx-post="/find" hx-target="#mm-thread" hx-swap="beforeend">
      <input type="hidden" name="q" value="{{.Query}}">
      <button class="mm-chip" type="submit">{{.Label}}</button>
    </form>
    <form class="mm-saved-del" hx-post="/find/saved/delete" hx-vals='{"id":"{{.ID}}"}' hx-target="closest .mm-saved-item" hx-swap="outerHTML">
      <button class="mm-btn quiet" type="submit" aria-label="remove saved search">&times;</button>
    </form>
  </span>
  {{end}}
  {{end}}
</div>{{end}}`
