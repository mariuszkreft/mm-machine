package onboarding

const threadHTML = `{{define "thread"}}<div class="mm-msg mm">
  <span class="mm-who">mm</span>
  {{if .Done}}
  <p>{{.DoneSummary}}</p>
  {{else}}
  {{if .Why}}<p class="mm-muted onb-why">{{.Why}}</p>{{end}}
  <p>{{.Question}}</p>
  {{if .OfferFinish}}<p class="mm-muted">{{.NudgeText}}</p>
  <button type="button" class="mm-btn ghost" hx-post="/start/finish" hx-target="#mm-thread" hx-swap="beforeend">{{.NudgeButton}}</button>{{end}}
  <button type="button" class="mm-btn quiet onb-skip" hx-post="/start/finish" hx-target="#mm-thread" hx-swap="beforeend">{{.SkipLabel}}</button>
  {{end}}
  <div class="mm-meter" role="progressbar" aria-valuenow="{{.Progress}}" aria-valuemin="0" aria-valuemax="100"><i style="width: {{.Progress}}%"></i></div>
  <span class="mm-muted mm-mono">{{.MeterLabel}}</span>
</div>
{{if .Learned}}{{template "profile" .ProfileView}}<div class="mm-msg mm onb-ack" hx-ext="sse" sse-connect="{{.StreamURL}}" sse-swap="message" sse-close="done" hx-target="find p" hx-swap="beforeend"><span class="mm-who">mm</span><p></p></div>{{end}}
{{end}}`

const profileHTML = `{{define "profile"}}<div class="mm-panel onb-profile" id="mm-profile">
  <div class="mm-card-head"><strong>{{.KnownLabel}}</strong><span class="mm-fit">{{.Completeness}}%</span></div>
  <div class="mm-meter" role="progressbar" aria-valuenow="{{.Completeness}}" aria-valuemin="0" aria-valuemax="100"><i style="width: {{.Completeness}}%"></i></div>
  <div class="mm-suggest">
    {{if .RoleLabel}}<span class="mm-chip on">{{.RoleLabel}}<button type="button" class="onb-chip-x" hx-post="/start/profile/edit" hx-vals='{"field":"role","op":"remove"}' hx-target="closest .mm-panel" hx-swap="outerHTML" aria-label="{{.RoleLabel}}">×</button></span>{{end}}
    {{range .TradesView}}<span class="mm-chip">{{.Label}}<button type="button" class="onb-chip-x" hx-post="/start/profile/edit" hx-vals='{"field":"trades","op":"remove","value":"{{.Value}}"}' hx-target="closest .mm-panel" hx-swap="outerHTML" aria-label="{{.Aria}}">×</button></span>{{end}}
    {{range .Regions}}<span class="mm-chip">{{.}}<button type="button" class="onb-chip-x" hx-post="/start/profile/edit" hx-vals='{"field":"regions","op":"remove","value":"{{.}}"}' hx-target="closest .mm-panel" hx-swap="outerHTML" aria-label="{{.}}">×</button></span>{{end}}
    {{if .CrewSizeLabel}}<span class="mm-chip">{{.CrewSizeLabel}}<button type="button" class="onb-chip-x" hx-post="/start/profile/edit" hx-vals='{"field":"crewSize","op":"remove"}' hx-target="closest .mm-panel" hx-swap="outerHTML" aria-label="{{.CrewSizeAria}}">×</button></span>{{end}}
    {{range .DocsView}}<span class="mm-chip">{{.Label}}<button type="button" class="onb-chip-x" hx-post="/start/profile/edit" hx-vals='{"field":"documents","op":"remove","value":"{{.Value}}"}' hx-target="closest .mm-panel" hx-swap="outerHTML" aria-label="{{.Aria}}">×</button></span>{{end}}
    {{if .Availability}}<span class="mm-chip">{{.Availability}}<button type="button" class="onb-chip-x" hx-post="/start/profile/edit" hx-vals='{"field":"availability","op":"remove"}' hx-target="closest .mm-panel" hx-swap="outerHTML" aria-label="{{.AvailabilityAria}}">×</button></span>{{end}}
  </div>
  <div class="onb-edit-rows">
    <form class="onb-edit-row" hx-post="/start/profile/edit" hx-target="closest .mm-panel" hx-swap="outerHTML">
      <input type="hidden" name="field" value="regions">
      <input type="hidden" name="op" value="set">
      <input class="mm-field" name="value" placeholder="{{.RegionPlaceholder}}" aria-label="{{.RegionPlaceholder}}">
      <button class="mm-btn ghost" type="submit">{{.UpdateLabel}}</button>
    </form>
    <form class="onb-edit-row" hx-post="/start/profile/edit" hx-target="closest .mm-panel" hx-swap="outerHTML">
      <input type="hidden" name="field" value="crewSize">
      <input type="hidden" name="op" value="set">
      <input class="mm-field" name="value" type="number" min="1" placeholder="{{.CrewSizePlaceholder}}" aria-label="{{.CrewSizePlaceholder}}">
      <button class="mm-btn ghost" type="submit">{{.UpdateLabel}}</button>
    </form>
  </div>
  {{if .Missing}}<p class="mm-muted mm-mono">{{.StillMissingLabel}} {{range $i, $m := .Missing}}{{if $i}}, {{end}}{{$m}}{{end}}</p>{{end}}
  <a class="mm-btn quiet" href="/start/reset">{{.ResetLabel}}</a>
</div>{{end}}`
