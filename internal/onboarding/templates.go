package onboarding

const threadHTML = `{{define "thread"}}<div class="mm-msg mm">
  <span class="mm-who">mm</span>
  {{if .Done}}
  <p>That is enough to work with — {{.Profile.Role}}{{if .Profile.Trades}}, {{range $i, $t := .Profile.Trades}}{{if $i}}, {{end}}{{$t}}{{end}}{{end}}{{if .Profile.Regions}} in {{index .Profile.Regions 0}}{{end}}. Ask me for what you need and I will search against it.</p>
  {{else}}
  <p>{{.Question}}</p>
  {{end}}
  <div class="mm-meter" role="progressbar" aria-valuenow="{{.Progress}}" aria-valuemin="0" aria-valuemax="100"><i style="width: {{.Progress}}%"></i></div>
  <span class="mm-muted mm-mono">profile {{.Progress}}% known</span>
</div>{{end}}`

const profileHTML = `{{define "profile"}}<div class="mm-panel" id="mm-profile">
  <div class="mm-card-head"><strong>Your profile</strong><span class="mm-fit">{{.Completeness}}%</span></div>
  <div class="mm-suggest">
    {{if .Role}}<span class="mm-chip on">{{.Role}}</span>{{end}}
    {{range .Trades}}<span class="mm-chip">{{.}}</span>{{end}}
    {{range .Regions}}<span class="mm-chip">{{.}}</span>{{end}}
    {{if .CrewSize}}<span class="mm-chip">{{.CrewSize}} people</span>{{end}}
    {{range .Documents}}<span class="mm-chip">{{.}}</span>{{end}}
    {{if .Availability}}<span class="mm-chip">{{.Availability}}</span>{{end}}
  </div>
  <a class="mm-btn quiet" href="/start/reset">start over</a>
</div>{{end}}`
