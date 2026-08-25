package store

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"mm-machine/internal/model"
)

func openTestStore(t *testing.T) *SQLite {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sub", "store.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s.(*SQLite)
}

func TestOfferRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	o := model.Offer{
		ID: "T-1", Title: "Test offer", Location: "Berlin", Category: "Energy",
		Amount: "1", Budget: "1k", Status: "open", Signal: "OK", Supplier: "Acme",
		Progress: 10, Attention: "none",
	}
	created, err := s.CreateOffer(ctx, o)
	if err != nil {
		t.Fatalf("CreateOffer: %v", err)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("expected non-zero timestamps, got %+v", created)
	}
	if created.CreatedAt.Location() != time.UTC {
		t.Fatalf("expected UTC location, got %v", created.CreatedAt.Location())
	}

	got, err := s.GetOffer(ctx, "T-1")
	if err != nil {
		t.Fatalf("GetOffer: %v", err)
	}
	if got.Title != "Test offer" {
		t.Fatalf("got title %q", got.Title)
	}

	got.Status = "process"
	got.Progress = 55
	updated, err := s.UpdateOffer(ctx, got)
	if err != nil {
		t.Fatalf("UpdateOffer: %v", err)
	}
	if updated.Status != "process" || updated.Progress != 55 {
		t.Fatalf("update did not apply: %+v", updated)
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Fatalf("CreatedAt should be preserved across update: got %v want %v", updated.CreatedAt, created.CreatedAt)
	}

	if _, err := s.GetOffer(ctx, "missing"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if _, err := s.UpdateOffer(ctx, model.Offer{ID: "missing"}); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestOfferFiltering(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	all, err := s.ListOffers(ctx, OfferFilter{})
	if err != nil {
		t.Fatalf("ListOffers: %v", err)
	}
	if len(all) != len(SeedOffers()) {
		t.Fatalf("expected %d seeded offers, got %d", len(SeedOffers()), len(all))
	}

	open, err := s.ListOffers(ctx, OfferFilter{Status: "open"})
	if err != nil {
		t.Fatalf("ListOffers status: %v", err)
	}
	for _, o := range open {
		if o.Status != "open" {
			t.Fatalf("expected only open offers, got %+v", o)
		}
	}
	if len(open) == 0 {
		t.Fatalf("expected at least one open offer")
	}

	byQuery, err := s.ListOffers(ctx, OfferFilter{Query: "munich"})
	if err != nil {
		t.Fatalf("ListOffers query: %v", err)
	}
	if len(byQuery) != 1 || byQuery[0].ID != "MM-1842" {
		t.Fatalf("expected MM-1842 for query munich, got %+v", byQuery)
	}

	limited, err := s.ListOffers(ctx, OfferFilter{Limit: 1})
	if err != nil {
		t.Fatalf("ListOffers limit: %v", err)
	}
	if len(limited) != 1 {
		t.Fatalf("expected 1 offer, got %d", len(limited))
	}

	counts, err := s.CountOffersByStatus(ctx)
	if err != nil {
		t.Fatalf("CountOffersByStatus: %v", err)
	}
	if counts["all"] != len(SeedOffers()) {
		t.Fatalf("expected all=%d, got %d", len(SeedOffers()), counts["all"])
	}
}

func TestStaticContentRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	persp, err := s.ListPerspectives(ctx)
	if err != nil {
		t.Fatalf("ListPerspectives: %v", err)
	}
	if len(persp) != len(SeedPerspectives()) {
		t.Fatalf("expected %d perspectives, got %d", len(SeedPerspectives()), len(persp))
	}
	if len(persp) > 0 && len(persp[0].Stats) == 0 {
		t.Fatalf("expected stats to round-trip, got %+v", persp[0])
	}

	mods, err := s.ListModules(ctx)
	if err != nil {
		t.Fatalf("ListModules: %v", err)
	}
	if len(mods) != len(SeedModules()) {
		t.Fatalf("expected %d modules, got %d", len(SeedModules()), len(mods))
	}

	roadmap, err := s.ListRoadmap(ctx)
	if err != nil {
		t.Fatalf("ListRoadmap: %v", err)
	}
	if len(roadmap) != len(SeedRoadmap()) {
		t.Fatalf("expected %d roadmap items, got %d", len(SeedRoadmap()), len(roadmap))
	}
}

func TestConversationAndMessageRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	c, err := s.CreateConversation(ctx, model.Conversation{ID: "conv-1", Role: "owner", Route: "/pipeline"})
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if c.CreatedAt.IsZero() {
		t.Fatalf("expected non-zero CreatedAt")
	}

	got, err := s.GetConversation(ctx, "conv-1")
	if err != nil {
		t.Fatalf("GetConversation: %v", err)
	}
	if got.Role != "owner" {
		t.Fatalf("got role %q", got.Role)
	}

	if _, err := s.GetConversation(ctx, "missing"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	msg1, err := s.AppendMessage(ctx, model.ChatMessage{ConversationID: "conv-1", Role: "user", Content: "hi"})
	if err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if msg1.ID == 0 {
		t.Fatalf("expected non-zero message ID")
	}
	msg2, err := s.AppendMessage(ctx, model.ChatMessage{ConversationID: "conv-1", Role: "assistant", Content: "hello"})
	if err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if msg2.ID == msg1.ID {
		t.Fatalf("expected distinct message IDs")
	}

	msgs, err := s.ListMessages(ctx, "conv-1")
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(msgs) != 2 || msgs[0].Content != "hi" || msgs[1].Content != "hello" {
		t.Fatalf("unexpected messages: %+v", msgs)
	}

	updatedConv, err := s.GetConversation(ctx, "conv-1")
	if err != nil {
		t.Fatalf("GetConversation: %v", err)
	}
	if !updatedConv.UpdatedAt.Equal(msg2.CreatedAt) {
		t.Fatalf("expected conversation UpdatedAt to track last message: got %v want %v", updatedConv.UpdatedAt, msg2.CreatedAt)
	}
}

func TestFeedbackRoundTripAndFilters(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	f1, err := s.CreateFeedback(ctx, model.Feedback{ConversationID: "c1", Kind: "bug", Theme: "login", Severity: 3, Verbatim: "broken", Route: "/", Role: "owner", Source: "chat"})
	if err != nil {
		t.Fatalf("CreateFeedback: %v", err)
	}
	if f1.Status != "new" {
		t.Fatalf("expected default status new, got %q", f1.Status)
	}
	if f1.ID == 0 {
		t.Fatalf("expected non-zero feedback ID")
	}

	if _, err := s.CreateFeedback(ctx, model.Feedback{ConversationID: "c1", Kind: "request", Theme: "export", Severity: 1, Verbatim: "want csv", Route: "/", Role: "owner", Source: "chat"}); err != nil {
		t.Fatalf("CreateFeedback: %v", err)
	}

	byKind, err := s.ListFeedback(ctx, FeedbackFilter{Kind: "bug"})
	if err != nil {
		t.Fatalf("ListFeedback kind: %v", err)
	}
	if len(byKind) != 1 || byKind[0].Theme != "login" {
		t.Fatalf("unexpected filter result: %+v", byKind)
	}

	byStatus, err := s.ListFeedback(ctx, FeedbackFilter{Status: "new"})
	if err != nil {
		t.Fatalf("ListFeedback status: %v", err)
	}
	if len(byStatus) != 2 {
		t.Fatalf("expected 2 new feedback items, got %d", len(byStatus))
	}

	if err := s.SetFeedbackStatus(ctx, f1.ID, "triaged"); err != nil {
		t.Fatalf("SetFeedbackStatus: %v", err)
	}
	afterStatus, err := s.ListFeedback(ctx, FeedbackFilter{Status: "triaged"})
	if err != nil {
		t.Fatalf("ListFeedback: %v", err)
	}
	if len(afterStatus) != 1 || afterStatus[0].ID != f1.ID {
		t.Fatalf("expected triaged feedback to be f1, got %+v", afterStatus)
	}

	if err := s.SetFeedbackStatus(ctx, 9999, "triaged"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestReplaceBacklogPreservesStatusByTheme(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	first := []model.BacklogItem{
		{Title: "Fix login", Theme: "login", Kind: "bug", Count: 3, AvgSeverity: 4, Score: 9.5, Evidence: []string{"a"}},
		{Title: "Add export", Theme: "export", Kind: "request", Count: 1, AvgSeverity: 2, Score: 3.1, Evidence: []string{"b"}},
	}
	if err := s.ReplaceBacklog(ctx, first); err != nil {
		t.Fatalf("ReplaceBacklog: %v", err)
	}

	items, err := s.ListBacklog(ctx)
	if err != nil {
		t.Fatalf("ListBacklog: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 backlog items, got %d", len(items))
	}
	var loginID int64
	for _, it := range items {
		if it.Theme == "login" {
			loginID = it.ID
			if it.Status != "proposed" {
				t.Fatalf("expected default status proposed, got %q", it.Status)
			}
		}
	}

	if err := s.SetBacklogStatus(ctx, loginID, "shipped"); err != nil {
		t.Fatalf("SetBacklogStatus: %v", err)
	}

	second := []model.BacklogItem{
		{Title: "Fix login (regen)", Theme: "login", Kind: "bug", Count: 5, AvgSeverity: 4.2, Score: 11.0, Evidence: []string{"a", "c"}},
		{Title: "New theme", Theme: "onboarding", Kind: "confusion", Count: 2, AvgSeverity: 2.5, Score: 4.0, Evidence: []string{"d"}},
	}
	if err := s.ReplaceBacklog(ctx, second); err != nil {
		t.Fatalf("ReplaceBacklog: %v", err)
	}

	after, err := s.ListBacklog(ctx)
	if err != nil {
		t.Fatalf("ListBacklog: %v", err)
	}
	if len(after) != 2 {
		t.Fatalf("expected 2 backlog items after replace, got %d", len(after))
	}
	found := false
	for _, it := range after {
		if it.Theme == "login" {
			found = true
			if it.Status != "shipped" {
				t.Fatalf("expected shipped status to survive regeneration, got %q", it.Status)
			}
			if it.Title != "Fix login (regen)" {
				t.Fatalf("expected refreshed title, got %q", it.Title)
			}
		}
		if it.Theme == "export" {
			t.Fatalf("export theme should have been removed by replace")
		}
	}
	if !found {
		t.Fatalf("expected login theme to survive replace")
	}

	if err := s.SetBacklogStatus(ctx, 9999, "shipped"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestReopenPersistsWithoutReseeding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.db")
	s1, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	ctx := context.Background()
	if _, err := s1.CreateOffer(ctx, model.Offer{ID: "EXTRA-1", Title: "Extra", Location: "X", Category: "Y", Amount: "1", Budget: "1", Status: "open", Signal: "OK", Supplier: "S", Progress: 0, Attention: "n"}); err != nil {
		t.Fatalf("CreateOffer: %v", err)
	}
	countsBefore, err := s1.ListOffers(ctx, OfferFilter{})
	if err != nil {
		t.Fatalf("ListOffers: %v", err)
	}
	wantCount := len(countsBefore)
	if err := s1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen Open: %v", err)
	}
	defer s2.Close()

	after, err := s2.ListOffers(ctx, OfferFilter{})
	if err != nil {
		t.Fatalf("ListOffers after reopen: %v", err)
	}
	if len(after) != wantCount {
		t.Fatalf("expected %d offers after reopen (no re-seed), got %d", wantCount, len(after))
	}

	got, err := s2.GetOffer(ctx, "EXTRA-1")
	if err != nil {
		t.Fatalf("GetOffer after reopen: %v", err)
	}
	if got.Title != "Extra" {
		t.Fatalf("data lost across reopen: %+v", got)
	}
}

func TestConcurrentAccessDoesNotLock(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			if _, err := s.ListOffers(ctx, OfferFilter{}); err != nil {
				errs <- err
			}
		}(i)
		go func(i int) {
			defer wg.Done()
			_, err := s.CreateFeedback(ctx, model.Feedback{ConversationID: "c", Kind: "bug", Theme: fmt.Sprintf("t%d", i), Severity: 1, Verbatim: "x", Route: "/", Role: "owner", Source: "chat"})
			if err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent access error: %v", err)
	}
}
