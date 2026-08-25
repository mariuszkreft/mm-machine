package demo

import (
	"context"
	"strings"
	"testing"

	"mm-machine/internal/model"
	"mm-machine/internal/store"
)

// countDemoOffers counts offers with a demo-market id ("MM-2..."), ignoring
// whatever base seed content the store started with.
func countDemoOffers(offers []model.Offer) int {
	n := 0
	for _, o := range offers {
		if strings.HasPrefix(o.ID, "MM-2") {
			n++
		}
	}
	return n
}

func TestSeedIdempotent(t *testing.T) {
	s := store.NewMemory()
	defer s.Close()
	ctx := context.Background()

	if err := Seed(ctx, s); err != nil {
		t.Fatal(err)
	}
	offers1, _ := s.ListOffers(ctx, store.OfferFilter{})
	crews1, _ := s.ListCrews(ctx, store.CrewFilter{})

	if err := Seed(ctx, s); err != nil {
		t.Fatal(err)
	}
	offers2, _ := s.ListOffers(ctx, store.OfferFilter{})
	crews2, _ := s.ListCrews(ctx, store.CrewFilter{})

	if len(offers1) != len(offers2) {
		t.Errorf("offer count changed across a second seed: %d -> %d", len(offers1), len(offers2))
	}
	if len(crews1) != len(crews2) {
		t.Errorf("crew count changed across a second seed: %d -> %d", len(crews1), len(crews2))
	}
	if want := len(Offers()); countDemoOffers(offers1) != want {
		t.Errorf("want %d demo offers seeded, got %d", want, countDemoOffers(offers1))
	}
	if want := len(Crews()); len(crews1) != want {
		t.Errorf("want %d demo crews seeded, got %d", want, len(crews1))
	}
}

func TestSeedPreservesOfferEdit(t *testing.T) {
	s := store.NewMemory()
	defer s.Close()
	ctx := context.Background()

	if err := Seed(ctx, s); err != nil {
		t.Fatal(err)
	}
	edited, err := s.GetOffer(ctx, "MM-2101")
	if err != nil {
		t.Fatal(err)
	}
	edited.Status = "done"
	edited.Budget = "EUR 999k"
	if _, err := s.UpdateOffer(ctx, edited); err != nil {
		t.Fatal(err)
	}

	if err := Seed(ctx, s); err != nil {
		t.Fatal(err)
	}
	after, err := s.GetOffer(ctx, "MM-2101")
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != "done" || after.Budget != "EUR 999k" {
		t.Errorf("seed overwrote an operator's offer edit: %+v", after)
	}
}

func TestSeedPreservesCrewEdit(t *testing.T) {
	s := store.NewMemory()
	defer s.Close()
	ctx := context.Background()

	if err := Seed(ctx, s); err != nil {
		t.Fatal(err)
	}
	edited, err := s.GetCrew(ctx, "CR-01")
	if err != nil {
		t.Fatal(err)
	}
	edited.Rate = "999 EUR/h"
	if _, err := s.UpsertCrew(ctx, edited); err != nil {
		t.Fatal(err)
	}

	if err := Seed(ctx, s); err != nil {
		t.Fatal(err)
	}
	after, err := s.GetCrew(ctx, "CR-01")
	if err != nil {
		t.Fatal(err)
	}
	if after.Rate != "999 EUR/h" {
		t.Errorf("seed overwrote an operator's crew edit: rate = %q", after.Rate)
	}
	want := Crews()[0]
	if after.Name != want.Name {
		t.Errorf("seed stopped syncing an untouched crew field: name = %q, want %q", after.Name, want.Name)
	}
}

func TestSeedOffSkipsWithoutDeleting(t *testing.T) {
	s := store.NewMemory()
	defer s.Close()
	ctx := context.Background()

	if err := Seed(ctx, s); err != nil {
		t.Fatal(err)
	}
	before, _ := s.ListOffers(ctx, store.OfferFilter{})

	t.Setenv("MM_DEMO", "off")
	if err := Seed(ctx, s); err != nil {
		t.Fatal(err)
	}
	after, _ := s.ListOffers(ctx, store.OfferFilter{})
	if len(after) != len(before) {
		t.Errorf("MM_DEMO=off changed offer count on an already-seeded store: %d -> %d", len(before), len(after))
	}
}

func TestSeedOffSkipsInitialSeed(t *testing.T) {
	s := store.NewMemory()
	defer s.Close()
	ctx := context.Background()

	t.Setenv("MM_DEMO", "off")
	if err := Seed(ctx, s); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetOffer(ctx, "MM-2101"); err == nil {
		t.Fatal("MM_DEMO=off still seeded the demo market")
	}
}
