package assistant

const panelHTML = `{{define "panel"}}<div id="assistant-panel" class="assistant-panel" data-conversation="{{.ConversationID}}">
  <div class="assistant-head">
    <span class="dot"></span>
    <strong>Montage assistant</strong>
    <span class="model">{{.Model}}</span>
  </div>
  <div id="assistant-log" class="assistant-log">
    <div class="bubble assistant"><span class="who">assistant</span><p>{{.Greeting}}</p></div>
  </div>
  <form class="assistant-form"
        hx-post="/assistant/message"
        hx-target="#assistant-log"
        hx-swap="beforeend"
        hx-on::after-request="this.reset()"
        hx-indicator="#assistant-busy">
    <input type="hidden" name="conversation" value="{{.ConversationID}}">
    <input type="hidden" name="role" value="{{.Role}}">
    <input type="hidden" name="route" value="{{.Route}}">
    <input name="message" placeholder="Ask about the app, or tell it what is broken" autocomplete="off" required>
    <button type="submit">Send</button>
    <span id="assistant-busy" class="htmx-indicator">thinking…</span>
  </form>
  <form class="feedback-form" hx-post="/feedback" hx-target="this" hx-swap="outerHTML">
    <input type="hidden" name="conversation" value="{{.ConversationID}}">
    <input type="hidden" name="role" value="{{.Role}}">
    <input type="hidden" name="route" value="{{.Route}}">
    <select name="kind">
      <option value="bug">bug</option>
      <option value="confusion">confusion</option>
      <option value="request">request</option>
      <option value="praise">praise</option>
    </select>
    <input name="verbatim" placeholder="Direct feedback about this app" required>
    <button type="submit">Log feedback</button>
  </form>
</div>{{end}}`

const bubbleHTML = `{{define "bubble"}}<div class="bubble {{.Role}}"><span class="who">{{.Role}}</span><p>{{.Content}}</p></div>{{end}}`
