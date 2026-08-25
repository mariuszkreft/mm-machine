package llm

import "testing"

func TestExtractJSON(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "bare object",
			in:   `{"a":1}`,
			want: `{"a":1}`,
		},
		{
			name: "prose on both sides, no fence",
			in:   `Sure, here is the answer: {"a":1} -- hope that helps!`,
			want: `{"a":1}`,
		},
		{
			name: "fenced with json hint and trailing prose",
			in:   "Here you go:\n```json\n{\"a\":1,\"b\":[1,2,3]}\n```\nLet me know if you need more.",
			want: `{"a":1,"b":[1,2,3]}`,
		},
		{
			name: "fenced with no language hint",
			in:   "```\n{\"a\":1}\n```",
			want: `{"a":1}`,
		},
		{
			name: "array instead of object",
			in:   `prefix [1,2,3] suffix`,
			want: `[1,2,3]`,
		},
		{
			name: "decorative fence before the real json fence",
			in: "First, some pseudo-code: ```text\nif x { do_thing() }\n```\n" +
				"Now the answer: ```json\n{\"ok\":true}\n```",
			want: `{"ok":true}`,
		},
		{
			name: "braces inside a string value don't confuse balancing",
			in:   `{"msg":"contains a { stray brace"}`,
			want: `{"msg":"contains a { stray brace"}`,
		},
		{
			name: "escaped quote inside string",
			in:   `{"msg":"she said \"hi\""}`,
			want: `{"msg":"she said \"hi\""}`,
		},
		{
			name: "stray unmatched brace before the real object",
			in:   `oops { not json ... anyway: {"a":1}`,
			want: `{"a":1}`,
		},
		{
			name: "no json at all",
			in:   "just some prose, nothing to see here",
			want: "",
		},
		{
			name: "empty string",
			in:   "",
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractJSON(tc.in)
			if got != tc.want {
				t.Errorf("ExtractJSON(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
