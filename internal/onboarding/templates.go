package onboarding

const threadHTML = `{{define "thread"}}<div class="mm-msg mm">
  <span class="mm-who">mm</span>
  {{if .Done}}
  <p>That is enough to work with — {{.Profile.Role}}{{if .Profile.Trades}}, {{range $i, $t := .Profile.Trades}}{{if $i}}, {{end}}{{$t}}{{end}}{{end}}{{if .Profile.Regions}} in {{index .Profile.Regions 0}}{{end}}. Ask me for what you need and I will search against it.</p>
  {{else}}
  <p>{{.Question}}</p>
  {{if .OfferFinish}}<p class="mm-muted">Nothing new caught my ear the last couple of times — happy to keep going, or:</p>
  <button type="button" class="mm-btn ghost" hx-post="/start/finish" hx-target="#mm-thread" hx-swap="beforeend">that's enough, let's go</button>{{end}}
  {{end}}
  <div class="mm-meter" role="progressbar" aria-valuenow="{{.Progress}}" aria-valuemin="0" aria-valuemax="100"><i style="width: {{.Progress}}%"></i></div>
  <span class="mm-muted mm-mono">profile {{.Progress}}% known</span>
</div>
{{if .Learned}}{{template "profile" .ProfileView}}<div class="mm-msg mm onb-ack" hx-ext="sse" sse-connect="{{.StreamURL}}" sse-swap="message" sse-close="done" hx-target="find p" hx-swap="beforeend"><span class="mm-who">mm</span><p></p></div>{{end}}
{{end}}`

const profileHTML = `{{define "profile"}}<div class="mm-panel onb-profile" id="mm-profile">
  <div class="mm-card-head"><strong>Your profile</strong><span class="mm-fit">{{.Completeness}}%</span></div>
  <div class="mm-meter" role="progressbar" aria-valuenow="{{.Completeness}}" aria-valuemin="0" aria-valuemax="100"><i style="width: {{.Completeness}}%"></i></div>
  <div class="mm-suggest">
    {{if .Role}}{{if ne .Role "unknown"}}<span class="mm-chip on">{{.Role}}<button type="button" class="onb-chip-x" hx-post="/start/profile/edit" hx-vals='{"field":"role","op":"remove"}' hx-target="closest .mm-panel" hx-swap="outerHTML" aria-label="clear role">×</button></span>{{end}}{{end}}
    {{range .Trades}}<span class="mm-chip">{{.}}<button type="button" class="onb-chip-x" hx-post="/start/profile/edit" hx-vals='{"field":"trades","op":"remove","value":"{{.}}"}' hx-target="closest .mm-panel" hx-swap="outerHTML" aria-label="remove {{.}} trade">×</button></span>{{end}}
    {{range .Regions}}<span class="mm-chip">{{.}}<button type="button" class="onb-chip-x" hx-post="/start/profile/edit" hx-vals='{"field":"regions","op":"remove","value":"{{.}}"}' hx-target="closest .mm-panel" hx-swap="outerHTML" aria-label="remove {{.}} region">×</button></span>{{end}}
    {{if .CrewSize}}<span class="mm-chip">{{.CrewSize}} people<button type="button" class="onb-chip-x" hx-post="/start/profile/edit" hx-vals='{"field":"crewSize","op":"remove"}' hx-target="closest .mm-panel" hx-swap="outerHTML" aria-label="clear crew size">×</button></span>{{end}}
    {{range .Documents}}<span class="mm-chip">{{.}}<button type="button" class="onb-chip-x" hx-post="/start/profile/edit" hx-vals='{"field":"documents","op":"remove","value":"{{.}}"}' hx-target="closest .mm-panel" hx-swap="outerHTML" aria-label="remove {{.}} document">×</button></span>{{end}}
    {{if .Availability}}<span class="mm-chip">{{.Availability}}<button type="button" class="onb-chip-x" hx-post="/start/profile/edit" hx-vals='{"field":"availability","op":"remove"}' hx-target="closest .mm-panel" hx-swap="outerHTML" aria-label="clear availability">×</button></span>{{end}}
  </div>
  <div class="onb-edit-rows">
    <form class="onb-edit-row" hx-post="/start/profile/edit" hx-target="closest .mm-panel" hx-swap="outerHTML">
      <input type="hidden" name="field" value="regions">
      <input type="hidden" name="op" value="set">
      <input class="mm-field" name="value" placeholder="change region" aria-label="change region">
      <button class="mm-btn ghost" type="submit">update</button>
    </form>
    <form class="onb-edit-row" hx-post="/start/profile/edit" hx-target="closest .mm-panel" hx-swap="outerHTML">
      <input type="hidden" name="field" value="crewSize">
      <input type="hidden" name="op" value="set">
      <input class="mm-field" name="value" type="number" min="1" placeholder="crew size" aria-label="set crew size">
      <button class="mm-btn ghost" type="submit">update</button>
    </form>
  </div>
  {{if .Missing}}<p class="mm-muted mm-mono">still missing: {{range $i, $m := .Missing}}{{if $i}}, {{end}}{{$m}}{{end}}</p>{{end}}
  <a class="mm-btn quiet" href="/start/reset">start over</a>
</div>{{end}}`
