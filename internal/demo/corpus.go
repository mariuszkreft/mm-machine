// Package demo fills the app with a believable DACH construction market and
// with example profiles a visitor can step into.
//
// Why this exists: an empty marketplace explains nothing. Someone opening
// mm.m-2.cc for the first time should be able to type one sentence and get a
// real ranked answer, and should be able to look at the product from a
// Generalunternehmer's and a Subunternehmer's side without registering.
package demo

import (
	"time"

	"mm-machine/internal/model"
)

func day(year int, month time.Month, d int) time.Time {
	return time.Date(year, month, d, 8, 0, 0, 0, time.UTC)
}

// Offers is the demand side of the demo market.
func Offers() []model.Offer {
	now := time.Now()
	return []model.Offer{
		{ID: "MM-2101", Title: "Photovoltaik-Dachmontage Gewerbehalle", Location: "München, DE", Category: "Energietechnik", Amount: "420 Module", Budget: "EUR 146k", Status: "process", Signal: "Attention", Supplier: "Voltwerk GmbH", Progress: 68, Attention: "Drei Anfragen brauchen eine Prüfung der Dokumenten-Laufzeit", Trade: "energy", Region: "München, DE", CrewSize: 8, Start: day(2026, time.October, 5), Requirements: []string{"a1", "insurance"}, Languages: []string{"de", "en"}, CreatedAt: now.Add(-72 * time.Hour), UpdatedAt: now.Add(-12 * time.Minute)},
		{ID: "MM-2102", Title: "Ladenbau Umbau Innenstadt", Location: "Zürich, CH", Category: "Innenausbau", Amount: "1.800 m²", Budget: "EUR 82k", Status: "requested", Signal: "OK", Supplier: "Alpine Montage", Progress: 36, Attention: "Fünf Rückmeldungen von Nachunternehmern liegen vor", Trade: "interior", Region: "Zürich, CH", CrewSize: 6, Start: day(2026, time.September, 21), Requirements: []string{"a1"}, Languages: []string{"de"}, CreatedAt: now.Add(-96 * time.Hour), UpdatedAt: now.Add(-38 * time.Minute)},
		{ID: "MM-2103", Title: "Stahlbau Lagerhalle", Location: "Rotterdam, NL", Category: "Stahlbau", Amount: "96 t", Budget: "EUR 310k", Status: "open", Signal: "OK", Supplier: "Nordline Build", Progress: 22, Attention: "Materialliste bestätigt, Kolonne gesucht", Trade: "steel", Region: "Rotterdam, NL", CrewSize: 12, Start: day(2026, time.November, 2), Requirements: []string{"a1", "insurance", "certificates"}, Languages: []string{"nl", "en"}, CreatedAt: now.Add(-120 * time.Hour), UpdatedAt: now.Add(-time.Hour)},
		{ID: "MM-2104", Title: "Hotelbäder Sanierung", Location: "Wien, AT", Category: "Sanitär", Amount: "74 Zimmer", Budget: "EUR 228k", Status: "done", Signal: "Review", Supplier: "Prime Install", Progress: 100, Attention: "Abnahmefenster offen, Bewertung ausstehend", Trade: "sanitary", Region: "Wien, AT", CrewSize: 5, Start: day(2026, time.June, 3), Requirements: []string{"a1"}, Languages: []string{"de"}, CreatedAt: now.Add(-400 * time.Hour), UpdatedAt: now.Add(-26 * time.Hour)},
		{ID: "MM-2105", Title: "Elektroinstallation Neubau Bürogebäude", Location: "München, DE", Category: "Elektro", Amount: "9 Etagen", Budget: "EUR 410k", Status: "open", Signal: "OK", Supplier: "Stadtwerk Bau", Progress: 8, Attention: "Kolonne mit mindestens sechs Elektrikern gesucht", Trade: "electrical", Region: "München, DE", CrewSize: 6, Start: day(2026, time.October, 12), Requirements: []string{"a1", "insurance"}, Languages: []string{"de"}, CreatedAt: now.Add(-40 * time.Hour), UpdatedAt: now.Add(-3 * time.Hour)},
		{ID: "MM-2106", Title: "Trockenbau Klinik-Erweiterung", Location: "Stuttgart, DE", Category: "Trockenbau", Amount: "4.200 m²", Budget: "EUR 190k", Status: "requested", Signal: "Attention", Supplier: "Südbau GmbH", Progress: 30, Attention: "Nachweis Brandschutzqualifikation fehlt bei zwei Bietern", Trade: "drywall", Region: "Stuttgart, DE", CrewSize: 10, Start: day(2026, time.September, 28), Requirements: []string{"a1", "certificates"}, Languages: []string{"de", "pl"}, CreatedAt: now.Add(-60 * time.Hour), UpdatedAt: now.Add(-5 * time.Hour)},
		{ID: "MM-2107", Title: "Lüftungsanlagen Rechenzentrum", Location: "Frankfurt, DE", Category: "Heizung/Klima", Amount: "12 Anlagen", Budget: "EUR 520k", Status: "open", Signal: "OK", Supplier: "Rhein Klima", Progress: 5, Attention: "Zutrittsprüfung für Rechenzentrum notwendig", Trade: "hvac", Region: "Frankfurt, DE", CrewSize: 9, Start: day(2026, time.November, 16), Requirements: []string{"a1", "insurance", "certificates"}, Languages: []string{"de", "en"}, CreatedAt: now.Add(-30 * time.Hour), UpdatedAt: now.Add(-90 * time.Minute)},
		{ID: "MM-2108", Title: "Sanitärinstallation Wohnanlage", Location: "Berlin, DE", Category: "Sanitär", Amount: "120 Wohnungen", Budget: "EUR 340k", Status: "process", Signal: "OK", Supplier: "Nordbau Berlin", Progress: 55, Attention: "Zweite Kolonne für Bauabschnitt B gesucht", Trade: "sanitary", Region: "Berlin, DE", CrewSize: 8, Start: day(2026, time.August, 18), Requirements: []string{"a1"}, Languages: []string{"de", "pl"}, CreatedAt: now.Add(-200 * time.Hour), UpdatedAt: now.Add(-7 * time.Hour)},
		{ID: "MM-2109", Title: "Stahltreppen Montage Parkhaus", Location: "Linz, AT", Category: "Stahlbau", Amount: "34 Läufe", Budget: "EUR 95k", Status: "open", Signal: "OK", Supplier: "Donau Stahl", Progress: 12, Attention: "Kranstellung durch Auftraggeber", Trade: "steel", Region: "Linz, AT", CrewSize: 4, Start: day(2026, time.October, 20), Requirements: []string{"a1", "insurance"}, Languages: []string{"de"}, CreatedAt: now.Add(-52 * time.Hour), UpdatedAt: now.Add(-4 * time.Hour)},
		{ID: "MM-2110", Title: "Innenausbau Flagship Store", Location: "Hamburg, DE", Category: "Innenausbau", Amount: "900 m²", Budget: "EUR 210k", Status: "requested", Signal: "OK", Supplier: "Elbwerk Interior", Progress: 42, Attention: "Termin steht, Kolonne muss Nachtarbeit leisten können", Trade: "interior", Region: "Hamburg, DE", CrewSize: 7, Start: day(2026, time.September, 14), Requirements: []string{"a1", "insurance"}, Languages: []string{"de", "en"}, CreatedAt: now.Add(-88 * time.Hour), UpdatedAt: now.Add(-9 * time.Hour)},
	}
}

// Crews is the supply side of the demo market.
func Crews() []model.Crew {
	return []model.Crew{
		{ID: "CR-01", Name: "Kolonne Nowak", Company: "Nowak Montage Sp. z o.o.", Trades: []string{"electrical", "drywall"}, Regions: []string{"München, DE", "Stuttgart, DE", "DACH"}, Size: 8, Languages: []string{"pl", "de"}, Documents: []string{"a1", "insurance"}, AvailableFrom: day(2026, time.October, 1), AvailableNote: "ab 1. Oktober, 6 Wochen am Stück", Rate: "46-54 EUR/h", Rating: 4.6, JobsDone: 23, Note: "Feste Kolonne seit vier Jahren, eigener Transporter und Werkzeug, Erfahrung mit Bürogebäuden."},
		{ID: "CR-02", Name: "Steel Crew Rotterdam", Company: "Van Dijk Staalbouw BV", Trades: []string{"steel"}, Regions: []string{"Rotterdam, NL", "NL", "Antwerpen, BE"}, Size: 12, Languages: []string{"nl", "en"}, Documents: []string{"a1", "insurance", "certificates"}, AvailableFrom: day(2026, time.November, 1), AvailableNote: "ab November, mindestens 8 Wochen", Rate: "52-60 EUR/h", Rating: 4.8, JobsDone: 41, Note: "Hallenbau und Stahltreppen, zertifizierte Schweißer nach EN 1090."},
		{ID: "CR-03", Name: "Sanitärteam Alpen", Company: "Berger Haustechnik GmbH", Trades: []string{"sanitary", "hvac"}, Regions: []string{"Wien, AT", "Linz, AT", "AT"}, Size: 5, Languages: []string{"de"}, Documents: []string{"a1", "certificates"}, AvailableFrom: day(2026, time.September, 15), AvailableNote: "kurzfristig verfügbar, auch Wochenende", Rate: "48-58 EUR/h", Rating: 4.4, JobsDone: 17, Note: "Hotel- und Klinikbäder, Erfahrung mit laufendem Betrieb."},
		{ID: "CR-04", Name: "Elektro Kolonne Süd", Company: "Kraft Elektrotechnik", Trades: []string{"electrical", "energy"}, Regions: []string{"München, DE", "Augsburg, DE"}, Size: 6, Languages: []string{"de", "tr"}, Documents: []string{"a1", "insurance", "certificates", "tax"}, AvailableFrom: day(2026, time.October, 6), AvailableNote: "ab Oktober frei", Rate: "50-58 EUR/h", Rating: 4.7, JobsDone: 31, Note: "Photovoltaik und Gebäudeinstallation, eigene Hubarbeitsbühne."},
		{ID: "CR-05", Name: "Trockenbau Team Ost", Company: "Kowalski Ausbau", Trades: []string{"drywall", "interior"}, Regions: []string{"Berlin, DE", "Stuttgart, DE", "DE"}, Size: 10, Languages: []string{"pl", "de"}, Documents: []string{"a1"}, AvailableFrom: day(2026, time.September, 22), AvailableNote: "ab Ende September, 10 Wochen", Rate: "42-48 EUR/h", Rating: 4.1, JobsDone: 12, Note: "Großflächen und Brandschutzwände, Nachweise in Arbeit."},
		{ID: "CR-06", Name: "Klima Crew Rhein", Company: "Rheinluft Service", Trades: []string{"hvac"}, Regions: []string{"Frankfurt, DE", "Köln, DE"}, Size: 9, Languages: []string{"de", "en"}, Documents: []string{"a1", "insurance", "certificates"}, AvailableFrom: day(2026, time.November, 10), AvailableNote: "ab Mitte November", Rate: "55-65 EUR/h", Rating: 4.9, JobsDone: 28, Note: "Rechenzentren und Reinräume, Sicherheitsüberprüfung vorhanden."},
	}
}
