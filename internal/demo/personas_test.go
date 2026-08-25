package demo

import (
	"testing"

	"mm-machine/internal/store"
)

func TestPersonasCount(t *testing.T) {
	if got := len(Personas()); got < 8 {
		t.Fatalf("want at least 8 personas, got %d", got)
	}
}

func TestPersonasComplete(t *testing.T) {
	for _, p := range Personas() {
		if got := store.Completeness(p.Profile); got < 80 {
			t.Errorf("persona %s: completeness %d < 80", p.Key, got)
		}
	}
}

func TestPersonasHaveSampleAsksInBothLanguages(t *testing.T) {
	for _, p := range Personas() {
		for _, lang := range []string{"de", "en"} {
			if len(pickList(p.SampleAsks, lang)) == 0 {
				t.Errorf("persona %s: no sample questions for %q", p.Key, lang)
			}
		}
		if p.Summary["de"] == "" || p.Summary["en"] == "" {
			t.Errorf("persona %s: summary missing in one language", p.Key)
		}
	}
}

func TestPersonaLookup(t *testing.T) {
	for _, p := range Personas() {
		got, ok := Persona(p.Key)
		if !ok || got.Key != p.Key {
			t.Errorf("Persona(%q) = %+v, %v", p.Key, got, ok)
		}
	}
	if _, ok := Persona("does-not-exist"); ok {
		t.Error("Persona(unknown) reported ok")
	}
}
