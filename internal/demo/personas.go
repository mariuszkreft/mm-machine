package demo

import "mm-machine/internal/model"

// Personas are prefilled example identities. Stepping into one sets a profile
// and a language; it never touches the visitor's own profile, which is put
// back when they leave the example.
func Personas() []model.Persona {
	return []model.Persona{
		{
			Key:   "gu-muenchen",
			Label: "Generalunternehmer · München",
			Lang:  "de",
			Summary: map[string]string{
				"de": "Bauleiter eines GU in München, sucht kurzfristig Elektro- und Trockenbaukolonnen für einen Bürobau.",
				"en": "Site manager at a Munich general contractor, needs electrical and drywall crews for an office build at short notice.",
			},
			Profile: model.Profile{
				Role: "owner", Company: "Stadtwerk Bau GmbH", Contact: "Bauleitung",
				Trades: []string{"electrical", "drywall"}, Regions: []string{"München, DE"},
				CrewSize: 6, Languages: []string{"de"}, Documents: []string{"a1", "insurance"},
				Availability: "ab 12. Oktober, 8 Wochen",
				Notes:        "Neubau Bürogebäude, neun Etagen, Kolonne muss eigene Werkzeuge stellen.",
			},
			SampleAsks: map[string][]string{
				"de": {
					"Wer kann ab Oktober sechs Elektriker in München stellen?",
					"Trockenbaukolonne mit Brandschutznachweis, 10 Leute",
					"Zeig mir meine offenen Aufträge",
				},
				"en": {
					"Who can field six electricians in Munich from October?",
					"Drywall crew with fire-safety certificates, 10 people",
					"Show me my open offers",
				},
			},
		},
		{
			Key:   "su-elektro-pl",
			Label: "Nachunternehmer · Elektro, PL",
			Lang:  "de",
			Summary: map[string]string{
				"de": "Polnischer Elektrobetrieb mit acht Monteuren, A1 und Versicherung liegen vor, sucht Anschlussauftrag in Süddeutschland.",
				"en": "Polish electrical company with eight fitters, A1 and insurance in place, looking for follow-up work in southern Germany.",
			},
			Profile: model.Profile{
				Role: "executor", Company: "Nowak Montage Sp. z o.o.", Contact: "Kolonnenführer",
				Trades: []string{"electrical", "drywall"}, Regions: []string{"München, DE", "Stuttgart, DE"},
				CrewSize: 8, Languages: []string{"pl", "de"}, Documents: []string{"a1", "insurance"},
				Availability: "ab 1. Oktober, 6 Wochen",
				Notes:        "Eigener Transporter und Werkzeug, Unterkunft muss gestellt werden.",
			},
			SampleAsks: map[string][]string{
				"de": {
					"Welche Aufträge passen zu meiner Kolonne?",
					"Offene Elektroarbeiten in Bayern ab Oktober",
					"Welche Papiere fehlen mir noch für Deutschland?",
				},
				"en": {
					"Which jobs fit my crew?",
					"Open electrical work in Bavaria from October",
					"Which papers am I still missing for Germany?",
				},
			},
		},
		{
			Key:   "su-stahl-nl",
			Label: "Subcontractor · steel, NL",
			Lang:  "en",
			Summary: map[string]string{
				"de": "Niederländischer Stahlbaubetrieb, zwölf Leute, EN-1090-zertifizierte Schweißer, sucht Hallenbau ab November.",
				"en": "Dutch steel company, twelve people, EN 1090 certified welders, looking for hall construction from November.",
			},
			Profile: model.Profile{
				Role: "executor", Company: "Van Dijk Staalbouw BV", Contact: "Planning",
				Trades: []string{"steel"}, Regions: []string{"Rotterdam, NL", "Antwerpen, BE"},
				CrewSize: 12, Languages: []string{"nl", "en"}, Documents: []string{"a1", "insurance", "certificates"},
				Availability: "from November, at least 8 weeks",
				Notes:        "Hall construction and steel stairs, certified welders to EN 1090.",
			},
			SampleAsks: map[string][]string{
				"de": {
					"Stahlbauaufträge in den Niederlanden ab November",
					"Wer sucht eine Kolonne mit zwölf Leuten?",
					"Welche Aufträge verlangen EN 1090?",
				},
				"en": {
					"Steel jobs in the Netherlands from November",
					"Who needs a crew of twelve?",
					"Which offers require EN 1090?",
				},
			},
		},
		{
			Key:   "gu-zurich",
			Label: "Generalunternehmer · Zürich",
			Lang:  "de",
			Summary: map[string]string{
				"de": "Projektleiterin für Ladenbau in Zürich, braucht Innenausbau-Kolonnen mit Nachtarbeitserfahrung.",
				"en": "Retail fit-out project lead in Zurich, needs interior crews able to work nights.",
			},
			Profile: model.Profile{
				Role: "owner", Company: "Alpine Montage AG", Contact: "Projektleitung",
				Trades: []string{"interior"}, Regions: []string{"Zürich, CH"},
				CrewSize: 6, Languages: []string{"de"}, Documents: []string{"a1"},
				Availability: "ab 21. September, 4 Wochen",
				Notes:        "Ladenbau im laufenden Betrieb, Anlieferung nur nachts möglich.",
			},
			SampleAsks: map[string][]string{
				"de": {
					"Innenausbau-Kolonne für Zürich, Nachtarbeit möglich",
					"Wer hat ab September frei?",
					"Was kostet eine Kolonne mit sechs Leuten üblicherweise?",
				},
				"en": {
					"Interior crew for Zurich, able to work nights",
					"Who is free from September?",
					"What does a crew of six usually cost?",
				},
			},
		},
	}
}

// Persona returns one persona by key.
func Persona(key string) (model.Persona, bool) {
	for _, p := range Personas() {
		if p.Key == key {
			return p, true
		}
	}
	return model.Persona{}, false
}
