// Package i18n carries the app's two languages.
//
// The market is DACH, so German is the default and English is the fallback —
// not the other way round. Every user-visible string lives in the catalog with
// both, and the local model is told which language to answer in, so a visitor
// never gets a German page with English answers inside it.
package i18n

import (
	"net/http"
	"strings"
	"time"
)

// Lang is a supported language.
type Lang string

const (
	DE Lang = "de"
	EN Lang = "en"

	// CookieName remembers an explicit choice; without it the language comes
	// from Accept-Language, and without that from Default.
	CookieName = "mm_lang"
	// Default is German: the app is built for the DACH construction market.
	Default = DE
)

// Supported lists the languages in menu order.
var Supported = []Lang{DE, EN}

// Valid reports whether l is a language the app actually has strings for.
func Valid(l Lang) bool {
	for _, s := range Supported {
		if s == l {
			return true
		}
	}
	return false
}

// Name is the language's own name, for the switcher.
func (l Lang) Name() string {
	switch l {
	case DE:
		return "Deutsch"
	default:
		return "English"
	}
}

// Detect resolves the language for a request: explicit cookie first, then
// Accept-Language, then the default.
func Detect(r *http.Request) Lang {
	if c, err := r.Cookie(CookieName); err == nil {
		if l := Lang(strings.ToLower(c.Value)); Valid(l) {
			return l
		}
	}
	for _, tag := range strings.Split(r.Header.Get("Accept-Language"), ",") {
		tag = strings.TrimSpace(strings.SplitN(tag, ";", 2)[0])
		if tag == "" {
			continue
		}
		primary := Lang(strings.ToLower(strings.SplitN(tag, "-", 2)[0]))
		if Valid(primary) {
			return primary
		}
	}
	return Default
}

// Set writes the language cookie.
func Set(w http.ResponseWriter, l Lang) {
	if !Valid(l) {
		l = Default
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    string(l),
		Path:     "/",
		MaxAge:   int((365 * 24 * time.Hour).Seconds()),
		HttpOnly: false, // the switcher may read it client-side
		SameSite: http.SameSiteLaxMode,
	})
}

// T looks a key up in the catalog. A missing key returns the key itself, which
// makes the gap visible in the UI instead of rendering an empty page.
func T(l Lang, key string) string {
	entry, ok := catalog[key]
	if !ok {
		return key
	}
	if s := entry[l]; s != "" {
		return s
	}
	if s := entry[Default]; s != "" {
		return s
	}
	return key
}

// Printer binds a language so templates can call one-argument helpers.
type Printer struct{ Lang Lang }

// NewPrinter returns a Printer for the request's language.
func NewPrinter(r *http.Request) Printer { return Printer{Lang: Detect(r)} }

// T is the template-facing lookup.
func (p Printer) T(key string) string { return T(p.Lang, key) }

// Ago is the template-facing relative-time helper, bound to the printer's
// language so templates never call the model's language-blind Ago directly.
func (p Printer) Ago(t time.Time) string { return Ago(p.Lang, t) }

// Date is the template-facing date formatter, bound to the printer's language.
func (p Printer) Date(t time.Time) string { return Date(p.Lang, t) }

// Is reports whether the printer is set to a language, for template branches.
func (p Printer) Is(l string) bool { return string(p.Lang) == l }

// Code returns the language code, for the html lang attribute.
func (p Printer) Code() string { return string(p.Lang) }

// AnswerIn is the instruction appended to every model prompt, so answers come
// back in the visitor's language.
func AnswerIn(l Lang) string {
	switch l {
	case DE:
		return "Antworte auf Deutsch, in der Sie-Form, knapp und konkret. Verwende die deutschen Fachbegriffe der Baubranche (Gewerk, Monteur, Nachunternehmer, A1-Bescheinigung)."
	default:
		return "Answer in English, short and concrete, using the construction industry's own terms."
	}
}

// Date formats a date the way each locale expects.
func Date(l Lang, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	if l == DE {
		return t.Format("02.01.2006")
	}
	return t.Format("2 Jan 2006")
}

// Ago renders a coarse relative time in the given language.
func Ago(l Lang, t time.Time) string {
	if t.IsZero() {
		return T(l, "time.justNow")
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return T(l, "time.justNow")
	case d < time.Hour:
		return itoa(int(d.Minutes())) + " " + T(l, "time.minutesAgo")
	case d < 24*time.Hour:
		return itoa(int(d.Hours())) + " " + T(l, "time.hoursAgo")
	case d < 48*time.Hour:
		return T(l, "time.yesterday")
	default:
		return itoa(int(d.Hours()/24)) + " " + T(l, "time.daysAgo")
	}
}

func itoa(n int) string {
	if n <= 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
