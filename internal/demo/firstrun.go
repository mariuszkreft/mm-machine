package demo

import (
	"fmt"
	"strconv"
	"strings"

	"mm-machine/internal/i18n"
	"mm-machine/internal/model"
)

// FirstRun returns the worked example a visitor sees the moment they step
// into a persona: the persona's first sample question, already answered from
// the seeded market. It is data only — it never renders HTML and never calls
// into internal/web or internal/search — so whoever assembles the initial
// thread can drop it in with one call.
func FirstRun(persona model.Persona, lang string) []model.ChatMessage {
	asks := pickList(persona.SampleAsks, lang)
	if len(asks) == 0 {
		return nil
	}
	return []model.ChatMessage{
		{Role: "user", Content: asks[0]},
		{Role: "assistant", Content: firstRunAnswer(persona, lang)},
	}
}

// firstRunAnswer picks the demand or supply side of the market depending on
// which side of the market the persona sits on, and reports the top matches
// for their trades and regions the same way search would, without depending
// on the search package.
func firstRunAnswer(persona model.Persona, lang string) string {
	if persona.Profile.Role == "owner" {
		return crewAnswer(persona.Profile, lang)
	}
	return offerAnswer(persona.Profile, lang)
}

func offerAnswer(p model.Profile, lang string) string {
	var lines []string
	for _, o := range Offers() {
		if !tradeMatch(p.Trades, o.Trade) || !regionMatch(p.Regions, o.Region) {
			continue
		}
		lines = append(lines, offerLine(o, lang))
		if len(lines) == 3 {
			break
		}
	}
	if len(lines) == 0 {
		return noMatchText(lang)
	}
	return strings.Join(append([]string{introText(lang, len(lines), "offer")}, lines...), "\n")
}

func crewAnswer(p model.Profile, lang string) string {
	var lines []string
	for _, c := range Crews() {
		if !tradesOverlap(p.Trades, c.Trades) || !regionsOverlap(p.Regions, c.Regions) {
			continue
		}
		lines = append(lines, crewLine(c, lang))
		if len(lines) == 3 {
			break
		}
	}
	if len(lines) == 0 {
		return noMatchText(lang)
	}
	return strings.Join(append([]string{introText(lang, len(lines), "crew")}, lines...), "\n")
}

func offerLine(o model.Offer, lang string) string {
	date := i18n.Date(i18n.Lang(lang), o.Start)
	if lang == "en" {
		return fmt.Sprintf("- %s (%s) — %s, %s, starts %s", o.Title, o.ID, o.Region, o.Budget, date)
	}
	return fmt.Sprintf("- %s (%s) — %s, %s, Start %s", o.Title, o.ID, o.Region, o.Budget, date)
}

func crewLine(c model.Crew, lang string) string {
	date := i18n.Date(i18n.Lang(lang), c.AvailableFrom)
	if lang == "en" {
		return fmt.Sprintf("- %s, %s (%s) — %s people, %s, from %s", c.Name, c.Company, c.ID, strconv.Itoa(c.Size), c.Rate, date)
	}
	return fmt.Sprintf("- %s, %s (%s) — %s Leute, %s, ab %s", c.Name, c.Company, c.ID, strconv.Itoa(c.Size), c.Rate, date)
}

func introText(lang string, n int, kind string) string {
	if lang == "en" {
		switch kind {
		case "crew":
			return fmt.Sprintf("I found %d matching crew(s):", n)
		default:
			return fmt.Sprintf("I found %d matching offer(s):", n)
		}
	}
	switch kind {
	case "crew":
		return fmt.Sprintf("Ich habe %d passende Kolonne(n) gefunden:", n)
	default:
		return fmt.Sprintf("Ich habe %d passende(s) Angebot(e) gefunden:", n)
	}
}

func noMatchText(lang string) string {
	if lang == "en" {
		return "Nothing in the demo market matches your profile exactly right now — try one of the other sample questions."
	}
	return "Aktuell passt nichts im Beispielmarkt exakt zu Ihrem Profil — probieren Sie eine der anderen Beispielfragen."
}

func tradeMatch(want []string, got string) bool {
	if len(want) == 0 {
		return true
	}
	for _, w := range want {
		if strings.EqualFold(strings.TrimSpace(w), strings.TrimSpace(got)) {
			return true
		}
	}
	return false
}

func tradesOverlap(want, have []string) bool {
	if len(want) == 0 {
		return true
	}
	for _, h := range have {
		if tradeMatch(want, h) {
			return true
		}
	}
	return false
}

func regionMatch(want []string, got string) bool {
	if len(want) == 0 {
		return true
	}
	g := strings.ToLower(strings.TrimSpace(got))
	for _, w := range want {
		w = strings.ToLower(strings.TrimSpace(w))
		if w == "" {
			continue
		}
		if strings.Contains(g, w) || strings.Contains(w, g) {
			return true
		}
	}
	return false
}

func regionsOverlap(want, have []string) bool {
	if len(want) == 0 {
		return true
	}
	for _, h := range have {
		if regionMatch(want, h) {
			return true
		}
	}
	return false
}
