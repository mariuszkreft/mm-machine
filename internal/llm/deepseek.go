// Package llm — concrete client for the fleet's vLLM endpoint.
//
// The served model (deepseek-v4-flash-0731) puts its chain of thought in a
// separate "reasoning" field and spends ~33 tokens on it before emitting any
// content, so short max_tokens values come back with content:null and
// finish_reason:"length". DefaultMaxTokens guards against that.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	return &client{cfg: cfg, http: &http.Client{Timeout: 180 * time.Second}}
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

func (c *client) post(ctx context.Context, body any, stream bool) (*http.Response, error) {
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
	if stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		resp.Body.Close()
		return nil, fmt.Errorf("llm: %s: %s", resp.Status, strings.TrimSpace(string(snippet)))
	}
	return resp, nil
}

func (c *client) Chat(ctx context.Context, req Request) (Response, error) {
	resp, err := c.post(ctx, c.body(req, false), false)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	var parsed chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return Response{}, fmt.Errorf("llm: decode: %w", err)
	}
	if parsed.Error != nil {
		return Response{}, fmt.Errorf("llm: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return Response{}, fmt.Errorf("llm: empty choices")
	}
	ch := parsed.Choices[0]
	return Response{
		Content:      ch.Message.Content,
		Reasoning:    ch.Message.Reasoning,
		FinishReason: ch.FinishReason,
		Tokens:       parsed.Usage.CompletionTokens,
	}, nil
}

// Stream is a minimal SSE reader; it degrades to a single Delta when the
// endpoint answers without streaming.
func (c *client) Stream(ctx context.Context, req Request, fn func(Delta) error) error {
	resp, err := c.post(ctx, c.body(req, true), true)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	dec := newSSEReader(resp.Body)
	for {
		payload, err := dec.next()
		if err == io.EOF {
			return fn(Delta{Done: true})
		}
		if err != nil {
			return err
		}
		if payload == "[DONE]" {
			return fn(Delta{Done: true})
		}
		var parsed chatResponse
		if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
			continue
		}
		if len(parsed.Choices) == 0 {
			continue
		}
		d := parsed.Choices[0].Delta
		if d.Content == "" && d.Reasoning == "" {
			continue
		}
		if err := fn(Delta{Content: d.Content, Reasoning: d.Reasoning}); err != nil {
			return err
		}
	}
}

// JSON asks for strict JSON and tolerates fenced or prose-wrapped answers.
func (c *client) JSON(ctx context.Context, req Request, schemaHint string, out any) error {
	msgs := append([]Message(nil), req.Messages...)
	msgs = append(msgs, Message{
		Role:    "system",
		Content: "Answer with a single JSON value and nothing else. No prose, no code fence. Shape:\n" + schemaHint,
	})
	req.Messages = msgs
	resp, err := c.Chat(ctx, req)
	if err != nil {
		return err
	}
	raw := ExtractJSON(resp.Content)
	if raw == "" {
		return fmt.Errorf("llm: no JSON in answer (finish_reason=%s)", resp.FinishReason)
	}
	return json.Unmarshal([]byte(raw), out)
}

// ExtractJSON pulls the first balanced JSON object or array out of s.
func ExtractJSON(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "```"); i >= 0 {
		rest := s[i+3:]
		if j := strings.Index(rest, "\n"); j >= 0 {
			rest = rest[j+1:]
		}
		if j := strings.Index(rest, "```"); j >= 0 {
			rest = rest[:j]
		}
		s = strings.TrimSpace(rest)
	}
	start := strings.IndexAny(s, "{[")
	if start < 0 {
		return ""
	}
	open := s[start]
	close := byte('}')
	if open == '[' {
		close = ']'
	}
	depth, inStr, esc := 0, false, false
	for i := start; i < len(s); i++ {
		ch := s[i]
		switch {
		case esc:
			esc = false
		case ch == '\\' && inStr:
			esc = true
		case ch == '"':
			inStr = !inStr
		case inStr:
		case ch == open:
			depth++
		case ch == close:
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

func (c *client) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	_, err := c.Chat(ctx, Request{
		Messages:  []Message{{Role: "user", Content: "ping"}},
		MaxTokens: 512,
	})
	return err
}
