package demo

import "testing"

func TestFirstRunCoherent(t *testing.T) {
	for _, p := range Personas() {
		for _, lang := range []string{"de", "en"} {
			msgs := FirstRun(p, lang)
			if len(msgs) != 2 {
				t.Fatalf("persona %s/%s: want 2 messages, got %d", p.Key, lang, len(msgs))
			}
			if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
				t.Errorf("persona %s/%s: want roles user,assistant, got %s,%s", p.Key, lang, msgs[0].Role, msgs[1].Role)
			}
			if msgs[0].Content == "" || msgs[1].Content == "" {
				t.Errorf("persona %s/%s: empty message content", p.Key, lang)
			}
			asks := pickList(p.SampleAsks, lang)
			if msgs[0].Content != asks[0] {
				t.Errorf("persona %s/%s: first message %q is not the persona's first sample question %q", p.Key, lang, msgs[0].Content, asks[0])
			}
		}
	}
}
