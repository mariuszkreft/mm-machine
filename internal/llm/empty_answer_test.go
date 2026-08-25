package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The served model sometimes spends a whole completion on its reasoning
// channel and answers with empty content. That is a hiccup, not a verdict:
// the client must ask again with a bigger budget.
func TestChatRetriesEmptyContent(t *testing.T) {
	var calls int
	var budgets []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body chatRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		budgets = append(budgets, body.MaxTokens)
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			// Empty content with a benign finish reason — the failure mode
			// seen in production.
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"","reasoning":"thinking…"},"finish_reason":"stop"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Here is the answer."},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Model: "test", MaxTokens: 1024})
	resp, err := c.Chat(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}, MaxTokens: 1024})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content != "Here is the answer." {
		t.Fatalf("content = %q", resp.Content)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if len(budgets) != 2 || budgets[1] <= budgets[0] {
		t.Fatalf("budgets = %v, want the retry to ask for more room", budgets)
	}
}

// A stream that closes without a single content delta is the same hiccup, and
// nothing was delivered yet, so it is safe to retry.
func TestStreamRetriesEmptyStream(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "text/event-stream")
		if calls == 1 {
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"reasoning\":\"hmm\"}}]}\n\ndata: [DONE]\n\n"))
			return
		}
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Model: "test"})
	var got string
	err := c.Stream(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}}, func(d Delta) error {
		got += d.Content
		return nil
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if got != "ok" {
		t.Fatalf("streamed %q, want %q", got, "ok")
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}
