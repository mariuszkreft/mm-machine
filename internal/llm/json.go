package llm

import (
	"encoding/json"
	"strings"
)

// ExtractJSON pulls the first syntactically valid JSON object or array out
// of s, tolerating prose before and after it and markdown code fences
// (including multiple/nested fences where only one block holds real JSON).
func ExtractJSON(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if raw := extractFromFences(s); raw != "" {
		return raw
	}
	return extractBalanced(s)
}

// extractFromFences looks inside every ```-fenced block of s, in order, for
// a validated JSON value. A block whose language hint is "json" wins
// outright; otherwise the first block that yields valid JSON is used. This
// lets a decorative or unrelated fenced block earlier in the answer be
// skipped in favor of the real one.
func extractFromFences(s string) string {
	type fence struct {
		lang string
		body string
	}
	var blocks []fence
	rest := s
	for {
		i := strings.Index(rest, "```")
		if i < 0 {
			break
		}
		rest = rest[i+3:]
		j := strings.Index(rest, "```")
		if j < 0 {
			break
		}
		content := rest[:j]
		rest = rest[j+3:]
		lang := ""
		if nl := strings.IndexByte(content, '\n'); nl >= 0 {
			head := strings.TrimSpace(content[:nl])
			if head != "" && !strings.ContainsAny(head, "{[\"") {
				lang = head
				content = content[nl+1:]
			}
		}
		blocks = append(blocks, fence{lang: lang, body: content})
	}
	fallback := ""
	for _, b := range blocks {
		raw := extractBalanced(b.body)
		if raw == "" {
			continue
		}
		if b.lang == "json" {
			return raw
		}
		if fallback == "" {
			fallback = raw
		}
	}
	return fallback
}

// extractBalanced scans s for the first balanced {...} or [...] value that
// is itself valid JSON, skipping over false starts such as a stray "{" in
// prose that never closes or doesn't parse.
func extractBalanced(s string) string {
	from := 0
	for {
		rel := strings.IndexAny(s[from:], "{[")
		if rel < 0 {
			return ""
		}
		start := from + rel
		end := balancedEnd(s, start)
		if end >= 0 {
			candidate := s[start : end+1]
			if json.Valid([]byte(candidate)) {
				return candidate
			}
		}
		from = start + 1
	}
}

// balancedEnd returns the index of the character that closes the bracket
// opened at s[start] (honoring JSON string/escape rules), or -1 if s ends
// before the bracket closes.
func balancedEnd(s string, start int) int {
	open := s[start]
	closeCh := byte('}')
	if open == '[' {
		closeCh = ']'
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
		case ch == closeCh:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
