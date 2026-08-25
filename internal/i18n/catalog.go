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
	"offer.crew":             {DE: "Mannstärke", EN: "crew"},
	"offer.papers":           {DE: "Papiere", EN: "papers"},
	"offer.start":            {DE: "Beginn", EN: "start"},
	"offer.create":           {DE: "Auftrag anlegen", EN: "Create offer"},

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

	// --- feedback and dev loop ----------------------------------------------
	"feedback.ask":    {DE: "Was war unklar, was fehlt, was ist kaputt?", EN: "What was unclear, what is missing, what broke?"},
	"feedback.thanks": {DE: "Notiert. Es erscheint im Feedback-Bereich.", EN: "Logged. It will show up in the feedback area."},
	"dev.title":       {DE: "Was Nutzer über diese App gesagt haben", EN: "What users told the app about itself"},
	"dev.explain": {
		DE: "Jede Rückmeldung aus dem Gespräch landet hier, wird vom lokalen Modell zu Themen gebündelt und nach Häufigkeit mal Schwere sortiert. Daraus entsteht die Reihenfolge der nächsten Entwicklungsschritte.",
		EN: "Every piece of feedback from a conversation lands here, gets clustered into themes by the local model, and is ranked by frequency times severity. That ranking is what the next development iteration works from.",
	},

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
