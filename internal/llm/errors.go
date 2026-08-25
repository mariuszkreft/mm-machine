package llm

import (
	"errors"
	"fmt"
)

// ErrEmptyAnswer is returned when the model comes back with content == ""
// and finish_reason == "length". deepseek-v4-flash spends ~33 output tokens
// on a hidden reasoning preamble before it emits any content, so this almost
// always means max_tokens was set too low; callers should retry with a
// bigger budget rather than treat it as a transient failure.
var ErrEmptyAnswer = errors.New("llm: empty answer, finish_reason=length (max_tokens too low: the model burns tokens on a hidden reasoning preamble before content; use >= 512)")

func newEmptyAnswerErr(endpoint, model string) error {
	return fmt.Errorf("llm: %s [%s]: %w", endpoint, model, ErrEmptyAnswer)
}

// statusError is an HTTP-level failure. It carries enough context (endpoint,
// model, status, and a short response snippet) to debug without re-running
// the request.
type statusError struct {
	Endpoint string
	Model    string
	Status   int
	Snippet  string
}

func (e *statusError) Error() string {
	snippet := e.Snippet
	if len(snippet) > 200 {
		snippet = snippet[:200] + "..."
	}
	return fmt.Sprintf("llm: %s [%s]: http %d: %s", e.Endpoint, e.Model, e.Status, snippet)
}

// callerAbort wraps an error returned by a Stream callback, marking it as
// "do not retry" while staying transparent to errors.Is/As via Unwrap.
type callerAbort struct{ err error }

func (c *callerAbort) Error() string { return c.err.Error() }
func (c *callerAbort) Unwrap() error { return c.err }
