package search

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"mm-machine/internal/llm"
)

// sseText reconstructs the plain text an SSE body's "message" events spell
// out, undoing the word-by-word "data: " framing emitWords writes.
func sseText(body string) string {
	var out strings.Builder
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "data: ") {
			out.WriteString(strings.TrimPrefix(line, "data: "))
		}
	}
	return out.String()
}

// TestStreamNeverNamesAnUnlistedOffer feeds a fake client that tries to
// introduce a result outside the computed match set. The guard must catch
// it and fall back to the mechanical summary instead of forwarding it.
func TestStreamNeverNamesAnUnlistedOffer(t *testing.T) {
	fake := &fakeLLM{StreamFunc: func(_ context.Context, _ llm.Request, fn func(llm.Delta) error) error {
		return fn(llm.Delta{Content: "Also check out MM-9999, a fantastic unrelated deal!"})
	}}
	_, mux, _ := newTestHandler(t, fake)

	matchesJSON := `[{"id":"MM-1842","title":"Photovoltaic roof installation","fit":88,"reason":"trade matches energy"}]`
	rec := doForm(mux, http.MethodGet, "/find/stream", url.Values{
		"q":       {"energy work"},
		"matches": {matchesJSON},
	})

	body := rec.Body.String()
	if strings.Contains(body, "MM-9999") {
		t.Fatalf("hallucinated offer id leaked into the stream:\n%s", body)
	}
	if !strings.Contains(sseText(body), "Photovoltaic roof installation") {
		t.Errorf("expected the mechanical fallback naming the real match, got:\n%s", body)
	}
}

// TestStreamNarratesListedMatches confirms a well-behaved model's sentence
// does reach the client when it only talks about what's in the list.
func TestStreamNarratesListedMatches(t *testing.T) {
	fake := &fakeLLM{StreamFunc: func(_ context.Context, _ llm.Request, fn func(llm.Delta) error) error {
		return fn(llm.Delta{Content: "One strong match: MM-1842, a photovoltaic roof job."})
	}}
	_, mux, _ := newTestHandler(t, fake)

	matchesJSON := `[{"id":"MM-1842","title":"Photovoltaic roof installation","fit":88,"reason":"trade matches energy"}]`
	rec := doForm(mux, http.MethodGet, "/find/stream", url.Values{
		"q":       {"energy work"},
		"matches": {matchesJSON},
	})

	body := rec.Body.String()
	if !strings.Contains(body, "MM-1842") {
		t.Errorf("expected the validated sentence to reach the client, got:\n%s", body)
	}
}

// TestStreamNoModelUsesMechanicalSummary covers the no-LLM path: it must
// still answer, honestly, without opening a connection to nothing.
func TestStreamNoModelUsesMechanicalSummary(t *testing.T) {
	_, mux, _ := newTestHandler(t, nil)
	matchesJSON := `[{"id":"MM-1842","title":"Photovoltaic roof installation","fit":88,"reason":"trade matches energy"}]`
	rec := doForm(mux, http.MethodGet, "/find/stream", url.Values{
		"q":       {"energy work"},
		"matches": {matchesJSON},
	})
	if !strings.Contains(sseText(rec.Body.String()), "Photovoltaic roof installation") {
		t.Errorf("expected the mechanical summary, got:\n%s", rec.Body.String())
	}
}

func TestMentionsUnlisted(t *testing.T) {
	allowed := map[string]bool{"MM-1842": true}
	if mentionsUnlisted("great match: MM-1842", allowed) {
		t.Error("MM-1842 is allowed, should not be flagged")
	}
	if !mentionsUnlisted("also see MM-9999", allowed) {
		t.Error("MM-9999 is not allowed, should be flagged")
	}
}
