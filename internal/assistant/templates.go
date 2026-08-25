package assistant

const panelHTML = `{{define "panel"}}<div id="assistant-panel" class="assistant-panel" data-conversation="{{.ConversationID}}">
  <div class="assistant-head">
    <span class="dot"></span>
    <strong>Montage assistant</strong>
    <span class="model">{{.Model}}</span>
  </div>
  <div id="assistant-log" class="assistant-log">
    {{if .History}}{{range .History}}<div class="bubble {{.Role}}"><span class="who">{{bubbleLabel $.Lang .Role}}</span><p>{{.Content}}</p></div>{{end}}{{else}}<div class="bubble assistant"><span class="who">{{bubbleLabel .Lang "assistant"}}</span><p>{{.Greeting}}</p></div>{{end}}
  </div>
  <div id="assistant-typing" class="typing-indicator" aria-live="polite"><span></span><span></span><span></span></div>
  <form class="assistant-form"
        method="post" action="/assistant/message"
        hx-get="/assistant/turn"
        hx-target="#assistant-log"
        hx-swap="beforeend"
        hx-on::after-request="this.reset()"
        hx-indicator="#assistant-busy">
    <input type="hidden" name="conversation" value="{{.ConversationID}}">
    <input type="hidden" name="role" value="{{.Role}}">
    <input type="hidden" name="route" value="{{.Route}}">
    <input name="message" placeholder="{{.PlaceholderLabel}}" autocomplete="off" required>
    <button type="submit">{{.SendLabel}}</button>
    <span id="assistant-busy" class="htmx-indicator">{{.BusyLabel}}</span>
  </form>
  <form class="feedback-form" hx-post="/feedback" hx-target="this" hx-swap="outerHTML">
    <input type="hidden" name="conversation" value="{{.ConversationID}}">
    <input type="hidden" name="role" value="{{.Role}}">
    <input type="hidden" name="route" value="{{.Route}}">
    <select name="kind">
      {{range .KindOptions}}<option value="{{.Value}}">{{.Label}}</option>{{end}}
    </select>
    <input name="verbatim" placeholder="{{.FeedbackPlaceholder}}" required>
    <button type="submit">{{.FeedbackButtonLabel}}</button>
  </form>
  <script>
  (function () {
    if (window.__mmAssistantWired) return;
    window.__mmAssistantWired = true;
    function panelOf(el) { return el && el.closest ? el.closest('.assistant-panel') : null; }
    function scrollLog() {
      var log = document.getElementById('assistant-log');
      if (log) log.scrollTop = log.scrollHeight;
    }
    document.body.addEventListener('htmx:sseOpen', function (evt) {
      var panel = panelOf(evt.target);
      if (!panel) return;
      panel.classList.add('streaming');
      var btn = panel.querySelector('.assistant-form button[type=submit]');
      if (btn) btn.disabled = true;
    });
    function stopStreaming(evt) {
      var panel = panelOf(evt.target);
      if (!panel) return;
      panel.classList.remove('streaming');
      var btn = panel.querySelector('.assistant-form button[type=submit]');
      if (btn) btn.disabled = false;
      scrollLog();
    }
    document.body.addEventListener('htmx:sseClose', stopStreaming);
    document.body.addEventListener('htmx:sseError', stopStreaming);
    document.body.addEventListener('htmx:afterSwap', function (evt) {
      if (evt.target && evt.target.id === 'assistant-log') scrollLog();
    });
    document.body.addEventListener('htmx:sseMessage', scrollLog);
    scrollLog();
  })();
  </script>
</div>{{end}}`

const bubbleHTML = `{{define "bubble"}}<div class="bubble {{.Role}}"><span class="who">{{.Role}}</span><p>{{.Content}}</p></div>{{end}}`
