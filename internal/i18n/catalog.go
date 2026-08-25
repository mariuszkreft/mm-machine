package i18n

// catalog holds every user-visible string in both languages.
//
// Keys are dotted and grouped by surface. Copy here is deliberately
// explanatory: this app asks people for real commercial information, so every
// prompt says what it will do with the answer, and every machine-produced
// number is followed by the reason it came out that way.
var catalog = map[string]map[Lang]string{
	// --- shell ---------------------------------------------------------------
	"app.name":     {DE: "Montage Manager", EN: "Montage Manager"},
	"nav.pipeline": {DE: "Aufträge", EN: "Pipeline"},
	"nav.about":    {DE: "Was ist das", EN: "What it is"},
	"nav.dev":      {DE: "Feedback", EN: "Dev loop"},
	"nav.demo":     {DE: "Beispiel-Profile", EN: "Example profiles"},
	"nav.home":     {DE: "Start", EN: "Home"},

	"lang.switchTo.de": {DE: "Auf Deutsch umschalten", EN: "Switch to German"},
	"lang.switchTo.en": {DE: "Auf Englisch umschalten", EN: "Switch to English"},

	"aria.primaryNav":      {DE: "Hauptnavigation", EN: "Primary"},
	"aria.perspectiveTabs": {DE: "Perspektive", EN: "Perspective"},
	"aria.conversation":    {DE: "Gespräch", EN: "Conversation"},

	"home.headline": {
		DE: "Was brauchen Sie?",
		EN: "What do you need?",
	},
	"home.lede": {
		DE: "Beschreiben Sie Auftrag, Kolonne oder Papiere in einem Satz — so wie Sie es einem Kollegen sagen würden. Montage Manager erkennt Gewerk, Region, Mannstärke und Termin daraus, sucht passende Treffer und zeigt zu jedem, warum er passt.",
		EN: "Describe the job, the crew or the paperwork in one sentence — the way you would tell a colleague. Montage Manager reads the trade, region, crew size and timing out of it, finds the matches, and shows you why each one fits.",
	},
	"home.placeholder": {
		DE: "6 Monteure in München, 3 Wochen, A1 vorhanden",
		EN: "6 fitters in Munich, 3 weeks, A1 ready",
	},
	"home.send":    {DE: "Fragen", EN: "Ask"},
	"home.busy":    {DE: "denkt nach…", EN: "thinking…"},
	"home.tryThis": {DE: "Zum Ausprobieren:", EN: "Try:"},

	"greeting.new": {
		DE: "Sagen Sie mir in Ihren Worten, was Sie brauchen — eine Kolonne, einen Auftrag, oder fehlende Papiere. Ich lese so viel wie möglich aus Ihrem Satz heraus und frage nur nach, was ich nicht erschließen kann. Nichts davon verlässt unseren eigenen Server.",
		EN: "Tell me what you need in your own words — a crew, a job, papers you are missing. I read as much as I can out of your sentence and ask only about what I cannot infer. None of it leaves our own server.",
	},
	"greeting.known": {
		DE: "Willkommen zurück. Ich habe Sie als %s gespeichert. Fragen Sie nach dem, was Sie brauchen — ich gewichte die Treffer danach.",
		EN: "Welcome back. I have you saved as %s. Ask for what you need and I will weight the matches against it.",
	},

	"role.owner":    {DE: "Generalunternehmer", EN: "general contractor"},
	"role.executor": {DE: "Nachunternehmer", EN: "subcontractor"},
	"role.unknown":  {DE: "unbekannt", EN: "unknown"},

	// --- onboarding ----------------------------------------------------------
	"onboarding.why": {
		DE: "Je mehr ich über Ihren Betrieb weiß, desto weniger müssen Sie bei jeder Suche wiederholen. Sie können jede Angabe später ändern oder das Profil ganz löschen.",
		EN: "The more I know about your company, the less you have to repeat with every search. You can change any of it later, or delete the profile entirely.",
	},
	"onboarding.role":         {DE: "Suchen Sie eine Kolonne, oder suchen Sie Arbeit?", EN: "Are you looking for a crew, or looking for work?"},
	"onboarding.tradesSU":     {DE: "Welche Gewerke deckt Ihre Kolonne ab?", EN: "What trades does your crew cover?"},
	"onboarding.tradesGU":     {DE: "Welche Arbeiten sollen ausgeführt werden?", EN: "What kind of work do you need done?"},
	"onboarding.regions":      {DE: "In welcher Region — Stadt oder Land?", EN: "Which region — city or country?"},
	"onboarding.crewSizeSU":   {DE: "Wie viele Leute können Sie stellen?", EN: "How many people can you field?"},
	"onboarding.crewSizeGU":   {DE: "Wie viele Leute brauchen Sie?", EN: "How many people do you need?"},
	"onboarding.documents":    {DE: "Welche Papiere liegen vor — A1, Versicherung, Nachweise?", EN: "Which papers are ready — A1, insurance, certificates?"},
	"onboarding.availability": {DE: "Wann geht es los, und für wie lange?", EN: "When does it start, and for how long?"},
	"onboarding.stillMissing": {DE: "Es fehlt noch:", EN: "Still missing:"},
	"onboarding.known":        {DE: "So habe ich Sie verstanden — bitte korrigieren Sie, was nicht stimmt:", EN: "This is how I understood you — correct anything that is wrong:"},
	"onboarding.reset":        {DE: "Profil löschen", EN: "Start over"},

	// --- search --------------------------------------------------------------
	"search.matches":     {DE: "Treffer", EN: "matches"},
	"search.bestFit":     {DE: "Beste Übereinstimmung", EN: "Best fit"},
	"search.why":         {DE: "Warum dieser Treffer", EN: "Why this matched"},
	"search.nothing":     {DE: "Dazu habe ich noch nichts gefunden.", EN: "Nothing matches that yet."},
	"search.nothingHelp": {DE: "Versuchen Sie ein anderes Gewerk, eine größere Region oder einen späteren Termin — oder beschreiben Sie den Auftrag ausführlicher.", EN: "Try another trade, a wider region or a later date — or describe the job in more detail."},
	"search.widened":     {DE: "Ich habe %s aus der Suche genommen, um überhaupt etwas zu finden.", EN: "I dropped %s from the search to find anything at all."},
	"search.degraded":    {DE: "Ohne Sprachmodell gefunden — nur Stichwortsuche.", EN: "Matched without the model — keyword search only."},
	"search.refine":      {DE: "Eingrenzen:", EN: "Refine:"},
	"search.save":        {DE: "Suche merken", EN: "Save this search"},
	"search.saved":       {DE: "gemerkt", EN: "saved"},

	// --- pipeline ------------------------------------------------------------
	"offer.status.open":      {DE: "offen", EN: "open"},
	"offer.status.requested": {DE: "angefragt", EN: "requested"},
	"offer.status.process":   {DE: "in Arbeit", EN: "process"},
	"offer.status.done":      {DE: "abgeschlossen", EN: "done"},
	"offer.signal.ok":        {DE: "OK", EN: "OK"},
	"offer.signal.review":    {DE: "Prüfen", EN: "Review"},
	"offer.signal.attention": {DE: "Achtung", EN: "Attention"},
	"offer.crew":             {DE: "Mannstärke", EN: "crew"},
	"offer.papers":           {DE: "Papiere", EN: "papers"},
	"offer.start":            {DE: "Beginn", EN: "start"},
	"offer.create":           {DE: "Auftrag anlegen", EN: "Create offer"},
	"offer.progressLabel":    {DE: "%s Fortschritt", EN: "%s progress"},

	"pipeline.title": {DE: "Montage Manager — Aufträge", EN: "Montage Manager — pipeline"},
	"pipeline.lede": {
		DE: "Jeder Auftrag, eine Steuerungsfläche: Status, Signal und was Aufmerksamkeit braucht, an einem Ort.",
		EN: "Every offer, one control surface: status, signal and what needs attention, together.",
	},
	"pipeline.viewLabel":                 {DE: "Ansicht", EN: "Pipeline view"},
	"pipeline.viewAll":                   {DE: "Alle", EN: "All"},
	"pipeline.searchLabel":               {DE: "Aufträge, Anbieter, Städte durchsuchen", EN: "Search offers, suppliers, cities"},
	"pipeline.newOffer":                  {DE: "Neuer Auftrag", EN: "New offer"},
	"pipeline.field.title":               {DE: "Titel", EN: "Title"},
	"pipeline.field.location":            {DE: "Ort", EN: "Location"},
	"pipeline.field.locationPlaceholder": {DE: "Stadt, Land", EN: "City, country"},
	"pipeline.field.category":            {DE: "Kategorie", EN: "Category"},
	"pipeline.field.budget":              {DE: "Budget", EN: "Budget"},
	"pipeline.field.supplier":            {DE: "Anbieter", EN: "Supplier"},
	"pipeline.field.status":              {DE: "Status", EN: "Status"},
	"pipeline.moveTo":                    {DE: "%s verschieben nach", EN: "Move %s to"},
	"pipeline.updated":                   {DE: "Aktualisiert %s", EN: "Updated %s"},
	"pipeline.statusLine":                {DE: "Status: %s", EN: "Status: %s"},
	"pipeline.signalLine":                {DE: "Signal: %s", EN: "Signal: %s"},
	"pipeline.empty":                     {DE: "Für diese Ansicht gibt es keine Treffer.", EN: "No offers match this view."},

	// --- trades and papers ---------------------------------------------------
	"trade.electrical": {DE: "Elektro", EN: "electrical"},
	"trade.sanitary":   {DE: "Sanitär", EN: "sanitary"},
	"trade.steel":      {DE: "Stahlbau", EN: "steel"},
	"trade.interior":   {DE: "Innenausbau", EN: "interior"},
	"trade.energy":     {DE: "Energietechnik", EN: "energy"},
	"trade.drywall":    {DE: "Trockenbau", EN: "drywall"},
	"trade.hvac":       {DE: "Heizung/Klima", EN: "hvac"},

	"doc.a1":           {DE: "A1-Bescheinigung", EN: "A1 certificate"},
	"doc.insurance":    {DE: "Betriebshaftpflicht", EN: "liability insurance"},
	"doc.certificates": {DE: "Fachnachweise", EN: "certificates"},
	"doc.tax":          {DE: "Steuerunterlagen", EN: "tax documents"},

	// --- about -----------------------------------------------------------------
	"about.title": {DE: "Montage Manager — was es ist", EN: "Montage Manager — what it is"},
	"about.h1": {
		DE: "Das Betriebssystem für die direkte Zusammenarbeit mit Nachunternehmern.",
		EN: "The operating system for direct subcontractor work.",
	},
	"about.lede": {
		DE: "Montage Manager verbindet Generalunternehmer und Nachunternehmer ohne intransparente Vermittlerebene: strukturierte Projekte, geprüfte Kolonnen, ein Dokumentensafe, Fortschrittsnachweise, Zahlungssignale und Belege für Streitfälle.",
		EN: "Montage Manager connects GUs and SUs without opaque broker layers: structured projects, verified teams, document safes, progress proof, payment signals, and dispute evidence.",
	},
	"about.askAssistant":           {DE: "Assistent fragen", EN: "Ask the assistant"},
	"about.openPipeline":           {DE: "Aufträge öffnen", EN: "Open pipeline"},
	"about.liveCommand":            {DE: "Live-Übersicht", EN: "Live command panel"},
	"about.statAll":                {DE: "Alle", EN: "All"},
	"about.statProcess":            {DE: "In Arbeit", EN: "In process"},
	"about.statOpen":               {DE: "Offen", EN: "Open"},
	"about.spotlightLabel":         {DE: "Im Fokus", EN: "Spotlight progress"},
	"about.spotlightNoneTitle":     {DE: "Noch keine Aufträge", EN: "No offers yet"},
	"about.spotlightNoneAttention": {DE: "Legen Sie den ersten Auftrag im Auftragscockpit an.", EN: "Create the first offer in the pipeline."},
	"about.localAI":                {DE: "Lokale KI · %s", EN: "Local AI · %s"},
	"about.talkHeadline":           {DE: "Sprechen Sie mit der App über die App", EN: "Talk to the app about the app"},
	"about.perspectivesEyebrow":    {DE: "Zwei Seiten, eine Plattform", EN: "Two-sided product"},
	"about.perspectivesHeadline": {
		DE: "Eine Plattform, zwei Betriebsrealitäten",
		EN: "One platform, two operating realities",
	},
	"about.decisionPressure": {DE: "Entscheidungsdruck", EN: "Decision pressure"},
	"about.modulesEyebrow":   {DE: "Funktionsumfang", EN: "Product depth"},
	"about.modulesHeadline": {
		DE: "Module, die den Makler-Engpass auflösen",
		EN: "Modules that remove the broker bottleneck",
	},
	"about.roadmapEyebrow": {DE: "Roadmap", EN: "Roadmap"},
	"about.roadmapHeadline": {
		DE: "Klein anfangen, dann die ganze Transaktion abbilden",
		EN: "Start narrow, then own the transaction",
	},
	"about.localModel":     {DE: "lokales Modell:", EN: "local model:"},
	"about.roles.headline": {DE: "Was die beiden Seiten bekommen", EN: "What the two roles get"},
	"about.roles.owner": {
		DE: "Als Generalunternehmer: eine Suche in einem Satz statt einer Ausschreibung, sofortige Treffer mit Begründung, und ein Auftrags-Cockpit, das jeden Status auf einen Blick zeigt.",
		EN: "As general contractor: a one-sentence search instead of a tender, instant matches with their reasoning, and an offer cockpit that shows every status at a glance.",
	},
	"about.roles.executor": {
		DE: "Als Nachunternehmer: passende Aufträge, sortiert nach Eignung Ihrer Kolonne, und ein Profil, das Ihre Papiere und Verfügbarkeit einmal festhält statt bei jeder Anfrage neu.",
		EN: "As subcontractor: matching jobs ranked against your crew's fit, and a profile that records your papers and availability once instead of at every inquiry.",
	},
	"about.privacy.eyebrow": {DE: "Datenschutz", EN: "Privacy"},
	"about.privacy.headline": {
		DE: "Was das lokale Modell tut — und was den Server verlässt",
		EN: "What the local model does — and what leaves the server",
	},
	"about.privacy.body": {
		DE: "Montage Manager liest Ihre Anfrage mit einem Sprachmodell, das auf diesem Server läuft. Es leitet daraus Gewerk, Region, Mannstärke und Termin ab und schlägt passende Treffer vor. Nichts von dem, was Sie eintippen, verlässt diesen Server — kein externer Anbieter, keine Cloud-API, keine Weitergabe an Dritte.",
		EN: "Montage Manager reads your request with a language model that runs on this server. It infers the trade, region, crew size and timing from it, and proposes matching results. Nothing you type leaves this server — no external provider, no cloud API, no sharing with third parties.",
	},
	"about.feedback.headline": {DE: "Wie der Feedback-Kreislauf funktioniert", EN: "How the feedback loop works"},
	"about.feedback.body": {
		DE: "Wenn Sie im Gespräch sagen, was unklar war, was fehlt oder was kaputt ist, liest das lokale Modell diese Rückmeldung mit, bündelt sie mit ähnlichen Meldungen zu Themen und ordnet sie nach Häufigkeit und Schwere. Das Ergebnis steht offen unter /dev — das ist der nächste Entwicklungsschritt, direkt aus dem, was Nutzer gesagt haben.",
		EN: "When you tell the app in conversation what was unclear, missing or broken, the local model reads that feedback, clusters it with similar reports into themes, and ranks them by frequency and severity. The result is open at /dev — that is the next development step, straight from what users said.",
	},

	// --- feedback and dev loop ----------------------------------------------
	"feedback.ask":    {DE: "Was war unklar, was fehlt, was ist kaputt?", EN: "What was unclear, what is missing, what broke?"},
	"feedback.thanks": {DE: "Notiert. Es erscheint im Feedback-Bereich.", EN: "Logged. It will show up in the feedback area."},
	"dev.title":       {DE: "Was Nutzer über diese App gesagt haben", EN: "What users told the app about itself"},
	"dev.explain": {
		DE: "Jede Rückmeldung aus dem Gespräch landet hier, wird vom lokalen Modell zu Themen gebündelt und nach Häufigkeit mal Schwere sortiert. Daraus entsteht die Reihenfolge der nächsten Entwicklungsschritte.",
		EN: "Every piece of feedback from a conversation lands here, gets clustered into themes by the local model, and is ranked by frequency times severity. That ranking is what the next development iteration works from.",
	},
	"dev.pageTitle": {DE: "Montage Manager · Feedback-Kreislauf", EN: "Montage Manager · dev loop"},
	"dev.navApp":    {DE: "App", EN: "App"},
	"dev.generatedLine": {
		DE: "Erstellt %s · %s · Aktualisierung alle %s",
		EN: "Generated %s · %s · refresh every %s",
	},
	"dev.filterKind":   {DE: "Art", EN: "Kind"},
	"dev.filterStatus": {DE: "Status", EN: "Status"},
	"dev.filterAll":    {DE: "alle", EN: "all"},
	"dev.regenerate":   {DE: "Backlog neu erzeugen", EN: "Regenerate backlog"},
	"dev.regenFailed":  {DE: "Letzte Regenerierung fehlgeschlagen: %s", EN: "Last regeneration failed: %s"},
	"dev.footerTagline": {
		DE: "Feedback-Kreislauf · der eigene Rückstand der App, geschrieben von ihren Nutzern",
		EN: "dev loop · the app's own backlog, written by its users",
	},
	"dev.backToApp":      {DE: "zurück zur App", EN: "back to the app"},
	"dev.countTotal":     {DE: "Rückmeldungen gesamt", EN: "feedback total"},
	"dev.countNew":       {DE: "neu", EN: "new"},
	"dev.countTriaged":   {DE: "gesichtet", EN: "triaged"},
	"dev.countLastRun":   {DE: "zuletzt erzeugt", EN: "last regenerated"},
	"dev.backlogHeading": {DE: "Rückstand", EN: "Backlog"},
	"dev.reportCount": {
		DE: "%d Meldung(en) · durchschnittliche Schwere %.1f · %s · %s",
		EN: "%d report(s) · avg severity %.1f · %s · %s",
	},
	"dev.accept": {DE: "Annehmen", EN: "Accept"},
	"dev.ship":   {DE: "Ausliefern", EN: "Ship"},
	"dev.reject": {DE: "Ablehnen", EN: "Reject"},
	"dev.backlogEmpty": {
		DE: "Der Rückstand ist leer. Sammeln Sie Feedback und erzeugen Sie ihn dann neu.",
		EN: "Backlog is empty. Collect feedback, then regenerate.",
	},
	"dev.feedbackHeading": {DE: "Rohes Feedback", EN: "Raw feedback"},
	"dev.feedbackMeta": {
		DE: "Thema: %s · Schwere %d · %s · %s · %s",
		EN: "cluster: %s · severity %d · %s · %s · %s",
	},
	"dev.feedbackEmpty": {
		DE: "Noch kein Feedback. Stellen Sie der App auf der Startseite eine Frage und sagen Sie ihr, was nicht stimmt.",
		EN: "No feedback yet. Ask the app a question on the home page and tell it what is wrong.",
	},

	"feedback.kind.bug":       {DE: "Fehler", EN: "bug"},
	"feedback.kind.confusion": {DE: "Verwirrung", EN: "confusion"},
	"feedback.kind.request":   {DE: "Wunsch", EN: "request"},
	"feedback.kind.praise":    {DE: "Lob", EN: "praise"},

	"feedback.status.new":      {DE: "neu", EN: "new"},
	"feedback.status.triaged":  {DE: "gesichtet", EN: "triaged"},
	"feedback.status.shipped":  {DE: "ausgeliefert", EN: "shipped"},
	"feedback.status.rejected": {DE: "abgelehnt", EN: "rejected"},

	"feedback.source.chat":   {DE: "Chat", EN: "chat"},
	"feedback.source.widget": {DE: "Widget", EN: "widget"},

	"backlog.status.proposed": {DE: "vorgeschlagen", EN: "proposed"},
	"backlog.status.accepted": {DE: "angenommen", EN: "accepted"},
	"backlog.status.shipped":  {DE: "ausgeliefert", EN: "shipped"},
	"backlog.status.rejected": {DE: "abgelehnt", EN: "rejected"},

	// --- time ----------------------------------------------------------------
	"time.justNow":    {DE: "gerade eben", EN: "just now"},
	"time.minutesAgo": {DE: "Min. her", EN: "min ago"},
	"time.hoursAgo":   {DE: "Std. her", EN: "h ago"},
	"time.yesterday":  {DE: "gestern", EN: "yesterday"},
	"time.daysAgo":    {DE: "Tage her", EN: "d ago"},

	// --- demo ----------------------------------------------------------------
	"demo.title": {DE: "Beispiel-Profile", EN: "Example profiles"},
	"demo.explain": {
		DE: "Um die App mit echten Daten zu sehen, können Sie in eines dieser Beispiel-Profile schlüpfen. Es setzt Profil und Sprache, nichts wird dauerhaft verändert — Ihr eigenes Profil bleibt erhalten.",
		EN: "To see the app working with real data, step into one of these example profiles. It sets a profile and language, nothing is changed permanently — your own profile is kept.",
	},
	"demo.enter": {DE: "Als dieses Profil ansehen", EN: "View as this profile"},
	"demo.leave": {DE: "Beispiel verlassen", EN: "Leave the example"},
	"demo.active": {
		DE: "Sie sehen die App gerade als Beispiel-Profil %s.",
		EN: "You are viewing the app as the example profile %s.",
	},
}

// Keys returns every catalog key, for tests that assert both languages exist.
func Keys() []string {
	out := make([]string, 0, len(catalog))
	for k := range catalog {
		out = append(out, k)
	}
	return out
}

// Entry exposes one catalog entry, for tests.
func Entry(key string) map[Lang]string { return catalog[key] }
