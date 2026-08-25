package search

import (
	"testing"
	"time"
)

func TestParseWindowAtMonthAndDuration(t *testing.T) {
	now := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)
	start, end, ok := parseWindowAt("looking for work from October for three weeks", now)
	if !ok {
		t.Fatal("expected a window to parse")
	}
	wantStart := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	wantEnd := wantStart.AddDate(0, 0, 21)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", end, wantEnd)
	}
}

func TestParseWindowAtMonthAlreadyPassedRollsToNextYear(t *testing.T) {
	now := time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)
	start, _, ok := parseWindowAt("work in March", now)
	if !ok {
		t.Fatal("expected a window to parse")
	}
	if start.Year() != 2027 || start.Month() != time.March {
		t.Errorf("start = %v, want March 2027", start)
	}
}

func TestParseWindowAtNextNWeeks(t *testing.T) {
	now := time.Date(2026, time.August, 25, 0, 0, 0, 0, time.UTC)
	start, end, ok := parseWindowAt("crew for the next 2 weeks", now)
	if !ok {
		t.Fatal("expected a window to parse")
	}
	if !start.Equal(now) {
		t.Errorf("start = %v, want %v", start, now)
	}
	wantEnd := now.AddDate(0, 0, 14)
	if !end.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", end, wantEnd)
	}
}

func TestParseWindowAtNoDate(t *testing.T) {
	if _, _, ok := parseWindowAt("electrical crew in Munich", time.Now()); ok {
		t.Error("expected no window for a sentence without one")
	}
}

func TestParseNegations(t *testing.T) {
	trades, regions := parseNegations("steel work, not Vienna")
	if len(trades) != 0 {
		t.Errorf("excludeTrades = %v, want none", trades)
	}
	if len(regions) != 1 || regions[0] != "Vienna" {
		t.Errorf("excludeRegions = %v, want [Vienna]", regions)
	}

	trades, regions = parseNegations("any trade except electrical")
	if len(trades) != 1 || trades[0] != "electrical" {
		t.Errorf("excludeTrades = %v, want [electrical]", trades)
	}
	if len(regions) != 0 {
		t.Errorf("excludeRegions = %v, want none", regions)
	}
}

func TestStripNegatedRemovesFromPositiveList(t *testing.T) {
	got := stripNegated([]string{"Vienna", "Munich"}, []string{"vienna"})
	if len(got) != 1 || got[0] != "Munich" {
		t.Errorf("stripNegated = %v, want [Munich]", got)
	}
}

func TestParseBudgetBounds(t *testing.T) {
	min, hasMin, max, hasMax := parseBudgetBounds("steel work under EUR 100k")
	if hasMin {
		t.Errorf("hasMin = true, want false")
	}
	if !hasMax || max != 100_000 {
		t.Errorf("max = %d, hasMax = %v, want 100000, true", max, hasMax)
	}

	min, hasMin, max, hasMax = parseBudgetBounds("budget over 50k")
	if hasMax {
		t.Errorf("hasMax = true, want false")
	}
	if !hasMin || min != 50_000 {
		t.Errorf("min = %d, hasMin = %v, want 50000, true", min, hasMin)
	}
}

func TestParseOfferBudget(t *testing.T) {
	cases := map[string]int{
		"EUR 146k": 146_000,
		"€82k":     82_000,
		"EUR 1.5m": 1_500_000,
	}
	for in, want := range cases {
		got, ok := parseOfferBudget(in)
		if !ok || got != want {
			t.Errorf("parseOfferBudget(%q) = %d, %v, want %d, true", in, got, ok, want)
		}
	}
	if _, ok := parseOfferBudget("negotiable"); ok {
		t.Error("expected an unparseable budget to report ok=false")
	}
}
