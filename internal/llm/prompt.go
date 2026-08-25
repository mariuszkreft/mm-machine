package llm

// EstimateTokens gives a rough token count for s. It uses the common
// heuristic of ~4 characters per token; good enough for budget trimming,
// not for billing.
func EstimateTokens(s string) int {
	n := len(s)
	if n == 0 {
		return 0
	}
	if t := n / 4; t > 0 {
		return t
	}
	return 1
}

// BuildMessages assembles a system + history + user message slice, trimming
// the oldest history turns first until the total fits within budgetTokens
// (as measured by EstimateTokens over each message's content). The system
// and user messages are never dropped; if they alone exceed the budget, all
// history is dropped but they are still included.
func BuildMessages(system string, history []Message, user string, budgetTokens int) []Message {
	var sys, usr []Message
	if system != "" {
		sys = []Message{{Role: "system", Content: system}}
	}
	if user != "" {
		usr = []Message{{Role: "user", Content: user}}
	}

	fixed := EstimateTokens(system) + EstimateTokens(user)
	trimmed := TrimHistory(history, budgetTokens-fixed)

	out := make([]Message, 0, len(sys)+len(trimmed)+len(usr))
	out = append(out, sys...)
	out = append(out, trimmed...)
	out = append(out, usr...)
	return out
}

// TrimHistory drops the oldest messages from history until the remaining
// messages' estimated token count fits within budgetTokens. If budgetTokens
// is <= 0, it returns an empty slice.
func TrimHistory(history []Message, budgetTokens int) []Message {
	if budgetTokens <= 0 || len(history) == 0 {
		return nil
	}
	total := 0
	for _, m := range history {
		total += EstimateTokens(m.Content)
	}
	start := 0
	for total > budgetTokens && start < len(history) {
		total -= EstimateTokens(history[start].Content)
		start++
	}
	if start >= len(history) {
		return nil
	}
	return append([]Message(nil), history[start:]...)
}
