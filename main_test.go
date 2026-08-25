package main

import (
	"testing"

	"mm-machine/internal/model"
)

// The prompt is the only entry point, so where it routes decides whether a
// visitor gets onboarded, searched or answered. German is the default
// language, so German phrasings have to route as well as English ones.
func TestRoute(t *testing.T) {
	known := model.Profile{Role: "owner", Trades: []string{"electrical"}, Regions: []string{"München, DE"}, CrewSize: 6, Documents: []string{"a1"}, Availability: "ab Oktober", Completeness: 100}
	fresh := model.Profile{}

	cases := []struct {
		name    string
		message string
		profile model.Profile
		want    string
	}{
		{"German question about the app", "Was passiert mit meinen Daten?", known, routeAssist},
		{"German how-does-it-work", "Wie funktioniert das hier?", known, routeAssist},
		{"German complaint", "Die Suche funktioniert nicht auf dem Handy", known, routeAssist},
		{"German wish", "Ich wünsche mir einen Export", known, routeAssist},
		{"English question about the app", "how does this work?", known, routeAssist},
		{"German search", "6 Monteure in München ab Oktober", known, routeSearch},
		{"English search", "steel crew in Rotterdam", known, routeSearch},
		{"unknown visitor is onboarded", "wir sind 8 Leute aus Krakau", fresh, routeOnboard},
		{"a product question beats onboarding", "was ist das hier eigentlich?", fresh, routeAssist},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := route(tc.message, tc.profile); got != tc.want {
				t.Fatalf("route(%q) = %q, want %q", tc.message, got, tc.want)
			}
		})
	}
}
