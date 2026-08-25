// Package app carries the dependencies every handler package needs.
package app

import (
	"mm-machine/internal/llm"
	"mm-machine/internal/store"
)

// Deps is passed to each package's Register function.
type Deps struct {
	Store store.Store
	LLM   llm.Client
	// Version is shown in the footer and given to the assistant as context.
	Version string
	// LLMModel is the served model id, for display.
	LLMModel string
}
