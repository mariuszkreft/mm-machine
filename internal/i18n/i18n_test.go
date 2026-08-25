package i18n

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Every catalog key must exist in both languages: a half-translated string is
// how a German page ends up with an English sentence in the middle of it.
func TestCatalogComplete(t *testing.T) {
	for _, key := range Keys() {
		entry := Entry(key)
		for _, lang := range Supported {
			if strings.TrimSpace(entry[lang]) == "" {
				t.Errorf("key %q missing %s", key, lang)
			}
		}
	}
}

func TestDetect(t *testing.T) {
	cases := []struct {
		name   string
		header string
		cookie string
		want   Lang
	}{
		{"default is German", "", "", DE},
		{"English browser", "en-GB,en;q=0.9", "", EN},
		{"German browser", "de-AT,de;q=0.9,en;q=0.5", "", DE},
		{"unknown language falls back", "fr-FR,fr;q=0.9", "", DE},
		{"cookie beats header", "en-GB", "de", DE},
		{"cookie beats header the other way", "de-DE", "en", EN},
		{"invalid cookie ignored", "en", "xx", EN},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				r.Header.Set("Accept-Language", tc.header)
			}
			if tc.cookie != "" {
				r.AddCookie(&http.Cookie{Name: CookieName, Value: tc.cookie})
			}
			if got := Detect(r); got != tc.want {
				t.Fatalf("Detect = %q, want %q", got, tc.want)
			}
		})
	}
}

// A missing key renders as the key, so the gap is visible rather than blank.
func TestMissingKeyIsVisible(t *testing.T) {
	if got := T(DE, "does.not.exist"); got != "does.not.exist" {
		t.Fatalf("T = %q, want the key back", got)
	}
}

func TestAnswerInIsLanguageSpecific(t *testing.T) {
	if !strings.Contains(AnswerIn(DE), "Deutsch") {
		t.Fatal("German instruction does not name the language")
	}
	if !strings.Contains(AnswerIn(EN), "English") {
		t.Fatal("English instruction does not name the language")
	}
}

// German dates are day.month.year; English keeps the day-month-year word form.
// This is the format contract every template must go through instead of
// formatting a time.Time itself.
func TestDateIsLocaleSpecific(t *testing.T) {
	when := time.Date(2026, time.October, 5, 10, 0, 0, 0, time.UTC)
	if got := Date(DE, when); got != "05.10.2026" {
		t.Fatalf("Date(DE) = %q, want 05.10.2026", got)
	}
	if got := Date(EN, when); got != "5 Oct 2026" {
		t.Fatalf("Date(EN) = %q, want 5 Oct 2026", got)
	}
	if got := Date(DE, time.Time{}); got != "" {
		t.Fatalf("Date(DE, zero) = %q, want empty", got)
	}
}

// The Printer methods are what templates actually call; they must not
// reintroduce the language-blind formatting they wrap.
func TestPrinterDateAndAgo(t *testing.T) {
	when := time.Date(2026, time.October, 5, 10, 0, 0, 0, time.UTC)
	de := Printer{Lang: DE}
	if got := de.Date(when); got != "05.10.2026" {
		t.Fatalf("Printer.Date(DE) = %q, want 05.10.2026", got)
	}
	if got := de.Ago(time.Time{}); got != T(DE, "time.justNow") {
		t.Fatalf("Printer.Ago(DE, zero) = %q, want %q", got, T(DE, "time.justNow"))
	}
	en := Printer{Lang: EN}
	if got := en.Ago(time.Time{}); got != T(EN, "time.justNow") {
		t.Fatalf("Printer.Ago(EN, zero) = %q, want %q", got, T(EN, "time.justNow"))
	}
}
