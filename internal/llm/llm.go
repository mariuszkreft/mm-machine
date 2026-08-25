// Package llm talks to the fleet's local OpenAI-compatible endpoint.
//
// This file is the CONTRACT. The concrete vLLM/deepseek implementation lives
// in deepseek.go.
package llm

import "context"

// Message is one OpenAI-style chat message.
type Message struct {
	Role    string `json:"role"` // system | user | assistant
	Content string `json:"content"`
}

// Request is a chat completion request.
type Request struct {
	Messages    []Message
	MaxTokens   int     // callers should stay >= 512: the model spends ~33 tokens on a hidden preamble
	Temperature float64 // 0 means use the client default
	Stop        []string
}

// Response is a completed answer. Reasoning holds the model's separate
// reasoning channel when it emits one (deepseek-v4-flash does).
type Response struct {
	Content      string
	Reasoning    string
	FinishReason string
	Tokens       int
}

// Delta is one streamed chunk.
type Delta struct {
	Content   string
	Reasoning string
	Done      bool
}

// Client is the LLM port used by the assistant and dev-loop packages.
type Client interface {
	// Chat blocks until the whole answer is ready.
	Chat(ctx context.Context, req Request) (Response, error)
	// Stream calls fn for every chunk; returning an error from fn aborts.
	Stream(ctx context.Context, req Request, fn func(Delta) error) error
	// JSON asks for a strict JSON answer matching schemaHint and unmarshals it
	// into out. Implementations must tolerate models that wrap JSON in prose or
	// code fences.
	JSON(ctx context.Context, req Request, schemaHint string, out any) error
	// Health reports whether the endpoint answers.
	Health(ctx context.Context) error
	// Model returns the served model id, for display.
	Model() string
}

// Config describes the endpoint. Defaults target the fleet cluster.
type Config struct {
	BaseURL   string // e.g. http://192.168.31.90:8000/v1
	Model     string // e.g. deepseek-v4-flash-0731
	APIKey    string // vLLM is open; any placeholder works
	MaxTokens int    // default cap when a Request leaves it at 0
}
