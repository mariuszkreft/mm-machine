package store

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"mm-machine/internal/model"
)

// Memory is an in-process Store used as a fallback when no database path is
// configured, and by tests. It is safe for concurrent use.
type Memory struct {
	mu            sync.RWMutex
	offers        []model.Offer
	conversations map[string]model.Conversation
	messages      []model.ChatMessage
	feedback      []model.Feedback
	backlog       []model.BacklogItem
	nextID        int64
}

// NewMemory returns a seeded in-memory store.
func NewMemory() *Memory {
	return &Memory{
		offers:        SeedOffers(),
		conversations: map[string]model.Conversation{},
		nextID:        1,
	}
}

var _ Store = (*Memory)(nil)

func (m *Memory) id() int64 { m.nextID++; return m.nextID - 1 }

func matches(o model.Offer, f OfferFilter) bool {
	if f.Status != "" && f.Status != "all" && !strings.EqualFold(o.Status, f.Status) {
		return false
	}
	if f.Query != "" {
		hay := strings.ToLower(o.Title + " " + o.Location + " " + o.Category + " " + o.Supplier)
		if !strings.Contains(hay, strings.ToLower(f.Query)) {
			return false
		}
	}
	return true
}

func (m *Memory) ListOffers(_ context.Context, f OfferFilter) ([]model.Offer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.Offer, 0, len(m.offers))
	for _, o := range m.offers {
		if matches(o, f) {
			out = append(out, o)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (m *Memory) GetOffer(_ context.Context, id string) (model.Offer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, o := range m.offers {
		if o.ID == id {
			return o, nil
		}
	}
	return model.Offer{}, ErrNotFound
}

func (m *Memory) CreateOffer(_ context.Context, o model.Offer) (model.Offer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if o.CreatedAt.IsZero() {
		o.CreatedAt = time.Now()
	}
	o.UpdatedAt = time.Now()
	m.offers = append(m.offers, o)
	return o, nil
}

func (m *Memory) UpdateOffer(_ context.Context, o model.Offer) (model.Offer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, cur := range m.offers {
		if cur.ID == o.ID {
			o.CreatedAt = cur.CreatedAt
			o.UpdatedAt = time.Now()
			m.offers[i] = o
			return o, nil
		}
	}
	return model.Offer{}, ErrNotFound
}

func (m *Memory) CountOffersByStatus(_ context.Context) (map[string]int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	counts := map[string]int{"all": len(m.offers)}
	for _, o := range m.offers {
		counts[strings.ToLower(o.Status)]++
	}
	return counts, nil
}

func (m *Memory) ListPerspectives(context.Context) ([]model.Perspective, error) {
	return SeedPerspectives(), nil
}
func (m *Memory) ListModules(context.Context) ([]model.Module, error) { return SeedModules(), nil }
func (m *Memory) ListRoadmap(context.Context) ([]model.RoadmapItem, error) {
	return SeedRoadmap(), nil
}

func (m *Memory) CreateConversation(_ context.Context, c model.Conversation) (model.Conversation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	c.UpdatedAt = time.Now()
	m.conversations[c.ID] = c
	return c, nil
}

func (m *Memory) GetConversation(_ context.Context, id string) (model.Conversation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.conversations[id]
	if !ok {
		return model.Conversation{}, ErrNotFound
	}
	return c, nil
}

func (m *Memory) AppendMessage(_ context.Context, msg model.ChatMessage) (model.ChatMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	msg.ID = m.id()
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	m.messages = append(m.messages, msg)
	if c, ok := m.conversations[msg.ConversationID]; ok {
		c.UpdatedAt = msg.CreatedAt
		m.conversations[msg.ConversationID] = c
	}
	return msg, nil
}

func (m *Memory) ListMessages(_ context.Context, conversationID string) ([]model.ChatMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []model.ChatMessage{}
	for _, msg := range m.messages {
		if msg.ConversationID == conversationID {
			out = append(out, msg)
		}
	}
	return out, nil
}

func (m *Memory) CreateFeedback(_ context.Context, f model.Feedback) (model.Feedback, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f.ID = m.id()
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now()
	}
	if f.Status == "" {
		f.Status = "new"
	}
	m.feedback = append(m.feedback, f)
	return f, nil
}

func (m *Memory) ListFeedback(_ context.Context, f FeedbackFilter) ([]model.Feedback, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []model.Feedback{}
	for _, item := range m.feedback {
		if f.Status != "" && item.Status != f.Status {
			continue
		}
		if f.Kind != "" && item.Kind != f.Kind {
			continue
		}
		if f.Since > 0 && item.CreatedAt.Unix() < f.Since {
			continue
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (m *Memory) SetFeedbackStatus(_ context.Context, id int64, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.feedback {
		if m.feedback[i].ID == id {
			m.feedback[i].Status = status
			return nil
		}
	}
	return ErrNotFound
}

func (m *Memory) ReplaceBacklog(_ context.Context, items []model.BacklogItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range items {
		if items[i].ID == 0 {
			items[i].ID = m.id()
		}
		items[i].UpdatedAt = time.Now()
	}
	m.backlog = items
	return nil
}

func (m *Memory) ListBacklog(context.Context) ([]model.BacklogItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := append([]model.BacklogItem(nil), m.backlog...)
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out, nil
}

func (m *Memory) SetBacklogStatus(_ context.Context, id int64, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.backlog {
		if m.backlog[i].ID == id {
			m.backlog[i].Status = status
			return nil
		}
	}
	return ErrNotFound
}

func (m *Memory) Close() error { return nil }
