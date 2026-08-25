// Package llm — concrete client for the fleet's vLLM endpoint.
//
// The served model (deepseek-v4-flash-0731) puts its chain of thought in a
// separate "reasoning" field and spends ~33 tokens on it before emitting any
// content, so short max_tokens values come back with content:null and
// finish_reason:"length". DefaultMaxTokens guards against that; ErrEmptyAnswer
// is the sentinel for when a caller's own MaxTokens still isn't enough.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the LAN address of the cluster endpoint. The GPU-fabric
	// address (192.168.100.11) is not routable from m2 or the gateway.
	DefaultBaseURL = "http://192.168.31.90:8000/v1"
	// DefaultModel is what vLLM serves there.
	DefaultModel = "deepseek-v4-flash-0731"
	// DefaultMaxTokens must stay well above the model's hidden preamble.
	DefaultMaxTokens = 1024
	// maxTokenBudget caps the doubling an empty answer triggers.
	maxTokenBudget = 4096

	maxAttempts   = 3
	baseBackoff   = 200 * time.Millisecond
	maxBackoffCap = 2 * time.Second

	defaultChatTimeout   = 60 * time.Second
	defaultStreamTimeout = 180 * time.Second
	defaultHealthTimeout = 20 * time.Second
)

type client struct {
	cfg  Config
	http *http.Client
}

// New returns a Client for an OpenAI-compatible endpoint, filling in the fleet
// defaults for any zero field.
func New(cfg Config) Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	if cfg.APIKey == "" {
		cfg.APIKey = "local" // vLLM is served open; any placeholder works
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = DefaultMaxTokens
	}
	// No client-wide Timeout: request lifetime is governed by per-call
	// context deadlines (see ensureDeadline) so long-lived streams aren't
	// cut off by a blanket transport timeout.
	return &client{cfg: cfg, http: &http.Client{}}
}

func (c *client) Model() string { return c.cfg.Model }

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

type chatChoice struct {
	Message struct {
		Content   string `json:"content"`
		Reasoning string `json:"reasoning"`
	} `json:"message"`
	Delta struct {
		Content   string `json:"content"`
		Reasoning string `json:"reasoning"`
	} `json:"delta"`
	FinishReason string `json:"finish_reason"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
	Usage   struct {
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *client) body(req Request, stream bool) chatRequest {
	max := req.MaxTokens
	if max <= 0 {
		max = c.cfg.MaxTokens
	}
	if max < 512 {
		max = 512
	}
	return chatRequest{
		Model:       c.cfg.Model,
		Messages:    req.Messages,
		MaxTokens:   max,
		Temperature: req.Temperature,
		Stop:        req.Stop,
		Stream:      stream,
	}
}

// ensureDeadline layers a default timeout onto ctx if the caller didn't
// already set one, so a single attempt can never hang forever.
func ensureDeadline(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}

// backoffDuration returns a capped exponential delay with jitter for the
// given attempt (1-based).
func backoffDuration(attempt int) time.Duration {
	d := baseBackoff * time.Duration(1<<uint(attempt-1))
	if d > maxBackoffCap {
		d = maxBackoffCap
	}
	jitter := time.Duration(rand.Int63n(int64(d)/2 + 1))
	return d/2 + jitter
}

// sleepBackoff waits out the backoff for attempt, or returns false early if
// ctx is done.
func sleepBackoff(ctx context.Context, attempt int) bool {
	t := time.NewTimer(backoffDuration(attempt))
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// isRetryable reports whether err is worth another attempt: connection
// failures, timeouts and 5xx responses are; caller aborts, empty-answer and
// 4xx client errors are not.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	var ca *callerAbort
	if errors.As(err, &ca) {
		return false
	}
	if errors.Is(err, ErrEmptyAnswer) {
		// The served model occasionally spends a whole completion on its
		// reasoning channel and returns no content at all. Asking again is
		// idempotent and usually enough.
		return true
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	var se *statusError
	if errors.As(err, &se) {
		return se.Status >= 500
	}
	return true // network errors, timeouts, mid-stream EOF, decode errors
}

func (c *client) post(ctx context.Context, body chatRequest) (*http.Response, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	if body.Stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm: %s [%s]: %w", c.cfg.BaseURL, c.cfg.Model, err)
	}
	if resp.StatusCode >= 400 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		resp.Body.Close()
		return nil, &statusError{Endpoint: c.cfg.BaseURL, Model: c.cfg.Model, Status: resp.StatusCode, Snippet: strings.TrimSpace(string(snippet))}
	}
	return resp, nil
}

// Chat blocks until the whole answer is ready, retrying idempotent failures
// (connection errors, timeouts, 5xx) up to maxAttempts with capped
// exponential backoff and jitter.
func (c *client) Chat(ctx context.Context, req Request) (Response, error) {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		resp, err := c.chatOnce(ctx, req)
		if errors.Is(err, ErrEmptyAnswer) {
			// Give the next attempt more room: an empty answer is usually the
			// reasoning channel eating the whole budget.
			req = withDoubledBudget(req, c.cfg.MaxTokens)
		}
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !isRetryable(err) || attempt == maxAttempts {
			return Response{}, err
		}
		if !sleepBackoff(ctx, attempt) {
			return Response{}, ctx.Err()
		}
	}
	return Response{}, lastErr
}

func (c *client) chatOnce(ctx context.Context, req Request) (Response, error) {
	attemptCtx, cancel := ensureDeadline(ctx, defaultChatTimeout)
	defer cancel()
	resp, err := c.post(attemptCtx, c.body(req, false))
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	var parsed chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return Response{}, fmt.Errorf("llm: %s [%s]: decode: %w", c.cfg.BaseURL, c.cfg.Model, err)
	}
	if parsed.Error != nil {
		return Response{}, fmt.Errorf("llm: %s [%s]: %s", c.cfg.BaseURL, c.cfg.Model, parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return Response{}, fmt.Errorf("llm: %s [%s]: empty choices", c.cfg.BaseURL, c.cfg.Model)
	}
	ch := parsed.Choices[0]
	if strings.TrimSpace(ch.Message.Content) == "" {
		return Response{}, newEmptyAnswerErr(c.cfg.BaseURL, c.cfg.Model)
	}
	return Response{
		Content:      ch.Message.Content,
		Reasoning:    ch.Message.Reasoning,
		FinishReason: ch.FinishReason,
		Tokens:       parsed.Usage.CompletionTokens,
	}, nil
}

// callFn invokes fn and, on error, marks it as a caller abort so the retry
// loop never retries after the caller has already asked to stop.
func callFn(fn func(Delta) error, d Delta) error {
	if err := fn(d); err != nil {
		return &callerAbort{err}
	}
	return nil
}

// Stream calls fn for every chunk, retrying idempotent connection failures
// (refused connections, 5xx, EOF before any content) up to maxAttempts —
// but never once a content delta has reached the caller, since that data
// can't be un-delivered. If the server ignores stream:true and answers as a
// normal completion, Stream falls back to emitting the whole answer as one
// delta instead of failing.
func (c *client) Stream(ctx context.Context, req Request, fn func(Delta) error) error {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		delivered, err := c.streamOnce(ctx, req, fn)
		if err == nil {
			return nil
		}
		lastErr = err
		if errors.Is(err, ErrEmptyAnswer) {
			req = withDoubledBudget(req, c.cfg.MaxTokens)
		}
		if delivered || !isRetryable(err) || attempt == maxAttempts {
			return err
		}
		if !sleepBackoff(ctx, attempt) {
			return ctx.Err()
		}
	}
	return lastErr
}

// withDoubledBudget returns req with twice the token budget it effectively had.
func withDoubledBudget(req Request, fallback int) Request {
	budget := req.MaxTokens
	if budget <= 0 {
		budget = fallback
	}
	if budget <= 0 {
		budget = DefaultMaxTokens
	}
	if budget < maxTokenBudget {
		req.MaxTokens = min(budget*2, maxTokenBudget)
	}
	return req
}

// streamOnce makes a single attempt. It reports whether any content delta
// reached fn, so the caller knows whether a retry is still safe.
func (c *client) streamOnce(ctx context.Context, req Request, fn func(Delta) error) (delivered bool, err error) {
	attemptCtx, cancel := ensureDeadline(ctx, defaultStreamTimeout)
	defer cancel()
	resp, err := c.post(attemptCtx, c.body(req, true))
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		return c.streamFallback(resp, fn)
	}

	dec := newSSEReader(resp.Body)
	// finish closes a stream that ended cleanly. A stream that ended without a
	// single content delta is the model's empty-answer hiccup, not an answer:
	// report it so the caller can retry (nothing was delivered yet).
	finish := func() (bool, error) {
		if !delivered {
			return false, newEmptyAnswerErr(c.cfg.BaseURL, c.cfg.Model)
		}
		return delivered, callFn(fn, Delta{Done: true})
	}
	for {
		payload, err := dec.next()
		if err == io.EOF {
			return finish()
		}
		if err != nil {
			return delivered, fmt.Errorf("llm: %s [%s]: stream read: %w", c.cfg.BaseURL, c.cfg.Model, err)
		}
		if payload == "[DONE]" {
			return finish()
		}
		var parsed chatResponse
		if json.Unmarshal([]byte(payload), &parsed) != nil || len(parsed.Choices) == 0 {
			continue
		}
		d := parsed.Choices[0].Delta
		if d.Content == "" && d.Reasoning == "" {
			continue
		}
		if d.Content != "" {
			delivered = true
		}
		if err := callFn(fn, Delta{Content: d.Content, Reasoning: d.Reasoning}); err != nil {
			return delivered, err
		}
	}
}

// streamFallback handles a server that answered a stream:true request as a
// normal, non-SSE completion: it decodes the whole body and emits it as a
// single delta rather than failing.
func (c *client) streamFallback(resp *http.Response, fn func(Delta) error) (bool, error) {
	var parsed chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return false, fmt.Errorf("llm: %s [%s]: decode: %w", c.cfg.BaseURL, c.cfg.Model, err)
	}
	if len(parsed.Choices) == 0 {
		return false, fmt.Errorf("llm: %s [%s]: empty choices", c.cfg.BaseURL, c.cfg.Model)
	}
	ch := parsed.Choices[0]
	if ch.Message.Content == "" && ch.FinishReason == "length" {
		return false, newEmptyAnswerErr(c.cfg.BaseURL, c.cfg.Model)
	}
	if err := callFn(fn, Delta{Content: ch.Message.Content, Reasoning: ch.Message.Reasoning}); err != nil {
		return false, err
	}
	if err := callFn(fn, Delta{Done: true}); err != nil {
		return true, err
	}
	return true, nil
}

// JSON asks for strict JSON and tolerates fenced or prose-wrapped answers.
// If the first answer doesn't contain parseable JSON, it retries once with
// a stricter instruction before giving up.
func (c *client) JSON(ctx context.Context, req Request, schemaHint string, out any) error {
	resp, err := c.askJSON(ctx, req, schemaHint, false)
	if err != nil {
		return err // network/API failure already retried inside Chat
	}
	if raw := ExtractJSON(resp.Content); raw != "" {
		if json.Unmarshal([]byte(raw), out) == nil {
			return nil
		}
	}

	resp2, err := c.askJSON(ctx, req, schemaHint, true)
	if err != nil {
		return err
	}
	raw2 := ExtractJSON(resp2.Content)
	if raw2 == "" {
		return fmt.Errorf("llm: %s [%s]: no JSON in answer after retry (finish_reason=%s)", c.cfg.BaseURL, c.cfg.Model, resp2.FinishReason)
	}
	if err := json.Unmarshal([]byte(raw2), out); err != nil {
		return fmt.Errorf("llm: %s [%s]: answer is not valid JSON after retry (finish_reason=%s): %w", c.cfg.BaseURL, c.cfg.Model, resp2.FinishReason, err)
	}
	return nil
}

func (c *client) askJSON(ctx context.Context, req Request, schemaHint string, strict bool) (Response, error) {
	instruction := "Answer with a single JSON value and nothing else. No prose, no code fence. Shape:\n" + schemaHint
	if strict {
		instruction = "Your previous answer was not valid JSON. Respond again with ONLY one JSON value: no prose, no explanation, no markdown code fences, nothing before or after it. Shape:\n" + schemaHint
	}
	// Where the instruction goes matters. A trailing system turn AFTER the user
	// message makes deepseek-v4-flash answer with empty content and
	// finish_reason "stop" — verified against the live endpoint. Folding the
	// same words into the leading system turn (or, failing that, into the last
	// user turn) answers reliably.
	msgs := append([]Message(nil), req.Messages...)
	switch {
	case len(msgs) > 0 && msgs[0].Role == "system":
		msgs[0].Content = msgs[0].Content + "\n\n" + instruction
	case len(msgs) > 0 && msgs[len(msgs)-1].Role == "user":
		msgs[len(msgs)-1].Content = msgs[len(msgs)-1].Content + "\n\n" + instruction
	default:
		msgs = append([]Message{{Role: "system", Content: instruction}}, msgs...)
	}
	r2 := req
	r2.Messages = msgs
	// JSON answers are short, but the hidden reasoning preamble in front of
	// them is not: give the budget room so the content is never truncated away.
	if r2.MaxTokens < 1536 {
		r2.MaxTokens = 1536
	}
	return c.Chat(ctx, r2)
}

// Health hits /models when the endpoint offers it, falling back to a tiny
// chat completion for servers that don't.
func (c *client) Health(ctx context.Context) error {
	ctx, cancel := ensureDeadline(ctx, defaultHealthTimeout)
	defer cancel()
	if err := c.pingModels(ctx); err == nil {
		return nil
	}
	_, err := c.Chat(ctx, Request{
		Messages:  []Message{{Role: "user", Content: "ping"}},
		MaxTokens: 512,
	})
	return err
}

func (c *client) pingModels(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.BaseURL+"/models", nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return &statusError{Endpoint: c.cfg.BaseURL, Model: c.cfg.Model, Status: resp.StatusCode, Snippet: strings.TrimSpace(string(snippet))}
	}
	io.Copy(io.Discard, resp.Body)
	return nil
}
