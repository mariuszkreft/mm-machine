package llm

import (
	"reflect"
	"strings"
	"testing"
)

func TestTrimHistory(t *testing.T) {
	history := []Message{
		{Role: "user", Content: strings.Repeat("a", 40)},      // ~10 tokens
		{Role: "assistant", Content: strings.Repeat("b", 40)}, // ~10 tokens
		{Role: "user", Content: strings.Repeat("c", 40)},      // ~10 tokens
		{Role: "assistant", Content: strings.Repeat("d", 40)}, // ~10 tokens (newest)
	}

	t.Run("drops oldest first when over budget", func(t *testing.T) {
		got := TrimHistory(history, 25) // fits roughly the newest two
		if len(got) == 0 {
			t.Fatalf("expected some history to survive, got none")
		}
		if got[len(got)-1].Content != history[len(history)-1].Content {
			t.Fatalf("newest message must survive trimming")
		}
		if reflect.DeepEqual(got, history) {
			t.Fatalf("expected trimming to drop something from a tight budget")
		}
		// oldest entries must be the ones gone, not newest.
		for i, m := range got {
			if m.Content != history[len(history)-len(got)+i].Content {
				t.Fatalf("trimmed history is not a suffix of the original: %v", got)
			}
		}
	})

	t.Run("everything fits", func(t *testing.T) {
		got := TrimHistory(history, 10_000)
		if !reflect.DeepEqual(got, history) {
			t.Fatalf("expected all history to survive a huge budget")
		}
	})

	t.Run("zero or negative budget drops everything", func(t *testing.T) {
		if got := TrimHistory(history, 0); got != nil {
			t.Fatalf("want nil, got %v", got)
		}
		if got := TrimHistory(history, -5); got != nil {
			t.Fatalf("want nil, got %v", got)
		}
	})

	t.Run("empty history", func(t *testing.T) {
		if got := TrimHistory(nil, 1000); got != nil {
			t.Fatalf("want nil, got %v", got)
		}
	})
}

func TestBuildMessages(t *testing.T) {
	history := []Message{
		{Role: "user", Content: strings.Repeat("a", 40)},
		{Role: "assistant", Content: strings.Repeat("b", 40)},
		{Role: "user", Content: strings.Repeat("c", 40)},
	}

	t.Run("system and user always present, history trimmed", func(t *testing.T) {
		out := BuildMessages("be terse", history, "what now?", 20)
		if len(out) == 0 || out[0].Role != "system" || out[0].Content != "be terse" {
			t.Fatalf("system message missing or wrong: %v", out)
		}
		last := out[len(out)-1]
		if last.Role != "user" || last.Content != "what now?" {
			t.Fatalf("user message missing or wrong: %v", out)
		}
		if len(out) > 2+len(history) {
			t.Fatalf("unexpected message count: %v", out)
		}
	})

	t.Run("tiny budget still keeps system and user", func(t *testing.T) {
		out := BuildMessages("sys", history, "usr", 1)
		if len(out) != 2 {
			t.Fatalf("expected history fully dropped, got %v", out)
		}
		if out[0].Content != "sys" || out[1].Content != "usr" {
			t.Fatalf("unexpected messages: %v", out)
		}
	})

	t.Run("empty system or user omitted", func(t *testing.T) {
		out := BuildMessages("", nil, "usr", 100)
		if len(out) != 1 || out[0].Role != "user" {
			t.Fatalf("expected only the user message, got %v", out)
		}
	})
}
