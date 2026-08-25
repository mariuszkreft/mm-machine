package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testClient(t *testing.T, ts *httptest.Server) *client {
	t.Helper()
	c := New(Config{BaseURL: ts.URL, Model: "test-model", APIKey: "x"}).(*client)
	return c
}

// --- Stream: wire-format assembly -------------------------------------------------

// sseBody writes each of frames as a `data: ...\n\n` event, splitting every
// frame across two writes (with a flush and tiny sleep in between) to
// exercise chunk-boundary reassembly, and sprinkling in blank keep-alive
// lines.
func sseBody(w http.ResponseWriter, frames []string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	fl, _ := w.(http.Flusher)
	write := func(s string) {
		io.WriteString(w, s)
		if fl != nil {
			fl.Flush()
		}
	}
	write("\n") // leading keep-alive
	for _, f := range frames {
		line := "data: " + f + "\n\n"
		mid := len(line) / 2
		write(line[:mid])
		time.Sleep(time.Millisecond)
		write(line[mid:])
		write("\n") // keep-alive between events
	}
}

func deltaFrame(content, reasoning, finish string) string {
	type delta struct {
		Content   string `json:"content,omitempty"`
		Reasoning string `json:"reasoning,omitempty"`
	}
	type choice struct {
		Delta        delta  `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	}
	b, _ := json.Marshal(struct {
		Choices []choice `json:"choices"`
	}{Choices: []choice{{Delta: delta{Content: content, Reasoning: reasoning}, FinishReason: finish}}})
	return string(b)
}

func TestStream_ReasoningThenContentAssembly(t *testing.T) {
	frames := []string{
		deltaFrame("", "Let", ""),
		deltaFrame("", " me think", ""),
		deltaFrame("Hello", "", ""),
		deltaFrame(" there", "", ""),
		deltaFrame("!", "", "stop"),
		"[DONE]",
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sseBody(w, frames)
	}))
	defer ts.Close()
	c := testClient(t, ts)

	var content, reasoning strings.Builder
	doneCount := 0
	err := c.Stream(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}}, func(d Delta) error {
		content.WriteString(d.Content)
		reasoning.WriteString(d.Reasoning)
		if d.Done {
			doneCount++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}
	if content.String() != "Hello there!" {
		t.Fatalf("content = %q, want %q", content.String(), "Hello there!")
	}
	if reasoning.String() != "Let me think" {
		t.Fatalf("reasoning = %q, want %q", reasoning.String(), "Let me think")
	}
	if doneCount != 1 {
		t.Fatalf("expected exactly one Done delta, got %d", doneCount)
	}
}

func TestStream_FallbackWhenServerIgnoresStreaming(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Answers as a normal JSON completion regardless of stream:true.
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"choices":[{"message":{"content":"full answer","reasoning":"thinking"},"finish_reason":"stop"}]}`)
	}))
	defer ts.Close()
	c := testClient(t, ts)

	var deltas []Delta
	err := c.Stream(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}}, func(d Delta) error {
		deltas = append(deltas, d)
		return nil
	})
	if err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}
	if len(deltas) != 2 {
		t.Fatalf("expected content delta + done delta, got %d: %+v", len(deltas), deltas)
	}
	if deltas[0].Content != "full answer" || deltas[0].Reasoning != "thinking" {
		t.Fatalf("unexpected first delta: %+v", deltas[0])
	}
	if !deltas[1].Done {
		t.Fatalf("expected second delta to be Done, got %+v", deltas[1])
	}
}

// TestStream_NoRetryAfterContentDelivered simulates a connection that dies
// mid-stream (via a handler panic, which makes the Go http server close the
// connection without a clean chunked-encoding terminator) right after
// delivering one content delta. Stream must surface the error but MUST NOT
// retry, because the caller already received real content.
func TestStream_NoRetryAfterContentDelivered(t *testing.T) {
	var requests int32
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "data: "+deltaFrame("partial", "", "")+"\n\n")
		if fl, ok := w.(http.Flusher); ok {
			fl.Flush()
		}
		panic("simulated mid-stream connection drop")
	}))
	ts.Config.ErrorLog = log.New(io.Discard, "", 0) // silence the panic-recovery log line
	ts.Start()
	defer ts.Close()
	c := testClient(t, ts)

	var got []Delta
	err := c.Stream(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}}, func(d Delta) error {
		got = append(got, d)
		return nil
	})
	if err == nil {
		t.Fatalf("expected an error from the dropped connection")
	}
	if len(got) != 1 || got[0].Content != "partial" {
		t.Fatalf("expected the partial content to have reached the caller, got %+v", got)
	}
	if n := atomic.LoadInt32(&requests); n != 1 {
		t.Fatalf("expected exactly 1 request (no retry after content delivered), got %d", n)
	}
}

// TestStream_RetriesBeforeAnyContent simulates a 500 on the first attempt
// (nothing delivered) followed by a working stream on the second attempt.
func TestStream_RetriesBeforeAnyContent(t *testing.T) {
	var requests int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requests, 1)
		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, "server hiccup")
			return
		}
		sseBody(w, []string{deltaFrame("ok", "", "stop"), "[DONE]"})
	}))
	defer ts.Close()
	c := testClient(t, ts)

	var content strings.Builder
	err := c.Stream(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}}, func(d Delta) error {
		content.WriteString(d.Content)
		return nil
	})
	if err != nil {
		t.Fatalf("Stream returned error after retry: %v", err)
	}
	if content.String() != "ok" {
		t.Fatalf("content = %q, want %q", content.String(), "ok")
	}
	if n := atomic.LoadInt32(&requests); n != 2 {
		t.Fatalf("expected 2 requests (1 failure + 1 retry), got %d", n)
	}
}

func TestStream_CallerAbortIsNotRetried(t *testing.T) {
	var requests int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		sseBody(w, []string{deltaFrame("x", "", ""), deltaFrame("y", "", "stop"), "[DONE]"})
	}))
	defer ts.Close()
	c := testClient(t, ts)

	abortErr := errors.New("caller stop")
	err := c.Stream(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}}, func(d Delta) error {
		return abortErr
	})
	if !errors.Is(err, abortErr) {
		t.Fatalf("expected caller's error to surface via errors.Is, got %v", err)
	}
	if n := atomic.LoadInt32(&requests); n != 1 {
		t.Fatalf("expected exactly 1 request (caller abort must not retry), got %d", n)
	}
}

// --- Chat: retries, status errors, ErrEmptyAnswer ---------------------------------

func TestChat_RetriesOn5xxThenSucceeds(t *testing.T) {
	var requests int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requests, 1)
		if n < 3 {
			w.WriteHeader(http.StatusBadGateway)
			io.WriteString(w, "bad gateway")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`)
	}))
	defer ts.Close()
	c := testClient(t, ts)

	resp, err := c.Chat(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("Chat returned error after retries: %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("Content = %q, want %q", resp.Content, "ok")
	}
	if n := atomic.LoadInt32(&requests); n != 3 {
		t.Fatalf("expected 3 requests, got %d", n)
	}
}

func TestChat_NoRetryOn4xx(t *testing.T) {
	var requests int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, "bad request")
	}))
	defer ts.Close()
	c := testClient(t, ts)

	_, err := c.Chat(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}})
	if err == nil {
		t.Fatalf("expected an error")
	}
	var se *statusError
	if !errors.As(err, &se) || se.Status != http.StatusBadRequest {
		t.Fatalf("expected a 400 statusError, got %v", err)
	}
	if n := atomic.LoadInt32(&requests); n != 1 {
		t.Fatalf("expected exactly 1 request (4xx must not retry), got %d", n)
	}
}

func TestChat_ErrEmptyAnswer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"content":"","reasoning":"burned it all here"},"finish_reason":"length"}]}`)
	}))
	defer ts.Close()
	c := testClient(t, ts)

	_, err := c.Chat(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}, MaxTokens: 64})
	if !errors.Is(err, ErrEmptyAnswer) {
		t.Fatalf("expected ErrEmptyAnswer, got %v", err)
	}
	if !strings.Contains(err.Error(), "max_tokens") {
		t.Fatalf("expected error to mention max_tokens, got %q", err.Error())
	}
}

// --- JSON: retry with a stricter instruction ---------------------------------------

func TestJSON_RetriesWithStricterInstructionOnBadParse(t *testing.T) {
	var requests int32
	var sawStrictInstruction bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requests, 1)
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "Your previous answer was not valid JSON") {
			sawStrictInstruction = true
		}
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			io.WriteString(w, `{"choices":[{"message":{"content":"sorry, I can't help with that"},"finish_reason":"stop"}]}`)
			return
		}
		io.WriteString(w, `{"choices":[{"message":{"content":"noise before {\"answer\":42} noise after"},"finish_reason":"stop"}]}`)
	}))
	defer ts.Close()
	c := testClient(t, ts)

	var out struct {
		Answer int `json:"answer"`
	}
	err := c.JSON(context.Background(), Request{Messages: []Message{{Role: "user", Content: "give me the answer"}}}, `{"answer": number}`, &out)
	if err != nil {
		t.Fatalf("JSON returned error: %v", err)
	}
	if out.Answer != 42 {
		t.Fatalf("Answer = %d, want 42", out.Answer)
	}
	if n := atomic.LoadInt32(&requests); n != 2 {
		t.Fatalf("expected 2 requests (initial + stricter retry), got %d", n)
	}
	if !sawStrictInstruction {
		t.Fatalf("expected the retry request to carry the stricter instruction")
	}
}

func TestJSON_FailsAfterStrictRetryStillUnparseable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"content":"still not json"},"finish_reason":"stop"}]}`)
	}))
	defer ts.Close()
	c := testClient(t, ts)

	var out map[string]any
	err := c.JSON(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}}, `{}`, &out)
	if err == nil {
		t.Fatalf("expected an error")
	}
}

// --- Health --------------------------------------------------------------------------

func TestHealth_UsesModelsEndpoint(t *testing.T) {
	var hitModels bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			hitModels = true
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"data":[]}`)
			return
		}
		t.Fatalf("unexpected request to %s", r.URL.Path)
	}))
	defer ts.Close()
	c := testClient(t, ts)

	if err := c.Health(context.Background()); err != nil {
		t.Fatalf("Health returned error: %v", err)
	}
	if !hitModels {
		t.Fatalf("expected Health to hit /models")
	}
}

func TestHealth_FallsBackToChatWhenModelsMissing(t *testing.T) {
	var hitChat bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/models":
			w.WriteHeader(http.StatusNotFound)
		case "/chat/completions":
			hitChat = true
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"choices":[{"message":{"content":"pong"},"finish_reason":"stop"}]}`)
		default:
			t.Fatalf("unexpected request to %s", r.URL.Path)
		}
	}))
	defer ts.Close()
	c := testClient(t, ts)

	if err := c.Health(context.Background()); err != nil {
		t.Fatalf("Health returned error: %v", err)
	}
	if !hitChat {
		t.Fatalf("expected Health to fall back to a chat completion")
	}
}

// --- Live sanity check (skipped in -short) ------------------------------------------

// TestLive_Stream hits the real fleet endpoint and confirms content deltas
// arrive incrementally. It is guarded by testing.Short() so `go test -short`
// never touches the network; a live cluster is not required for the rest of
// the suite.
func TestLive_Stream(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live check in -short mode")
	}
	c := New(Config{})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var chunks int
	var content strings.Builder
	err := c.Stream(ctx, Request{
		Messages:  []Message{{Role: "user", Content: "Say hello in exactly three words."}},
		MaxTokens: 512,
	}, func(d Delta) error {
		if d.Content != "" {
			chunks++
			content.WriteString(d.Content)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("live Stream failed: %v", err)
	}
	if chunks < 2 {
		t.Fatalf("expected content to arrive as multiple deltas, got %d", chunks)
	}
	if strings.TrimSpace(content.String()) == "" {
		t.Fatalf("expected non-empty streamed content")
	}
	t.Logf("live content: %q", content.String())
}
