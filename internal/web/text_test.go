package web

import "testing"

func TestRichText(t *testing.T) {
	cases := map[string]string{
		"plain":                    "plain",
		"<b>bold</b>":              "&lt;b&gt;bold&lt;/b&gt;",
		"a\nb":                     "a<br>\nb",
		"see https://x.io/a.":      `see <a href="https://x.io/a" rel="noopener noreferrer nofollow">https://x.io/a</a>.`,
		`javascript:alert(1)`:      `javascript:alert(1)`,
		`"https://x.io/q?a=1&b=2"`: `&#34;<a href="https://x.io/q?a=1&amp;b=2" rel="noopener noreferrer nofollow">https://x.io/q?a=1&amp;b=2</a>&#34;`,
	}
	for in, want := range cases {
		if got := string(richText(in)); got != want {
			t.Errorf("richText(%q) = %q, want %q", in, got, want)
		}
	}
}
