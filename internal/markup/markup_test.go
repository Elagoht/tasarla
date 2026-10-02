package markup

import (
	"regexp"
	"strings"
	"testing"
)

func TestMarkdownIsSafe(t *testing.T) {
	attacks := []string{
		"<script>alert(1)</script>",
		`<img src=x onerror=alert(1)>`,
		"[x](javascript:alert(1))",
		"[x](JaVaScRiPt:alert(1))",
		"[x](&#106;avascript:alert(1))",
		"[x](data:text/html;base64,PHNjcmlwdD4=)",
		"<javascript:alert(1)>",
		"<div onclick=\"alert(1)\">x</div>",
		"a <b onmouseover=alert(1)>b</b>",
		"![x](https://x.io/a.png)",
		"<style>body{display:none}</style>",
		"[x](https://x.io \"t\" onclick=alert(1))",
	}
	for _, in := range attacks {
		out := strings.ToLower(string(Markdown(in)))
		for name, re := range unsafeOutput {
			if re.MatchString(out) {
				t.Errorf("%q → %q matches %s", in, out, name)
			}
		}
	}
	// Raw HTML is left out: no tag of the input survives.
	for in, tag := range map[string]*regexp.Regexp{
		"<script>alert(1)</script>":         regexp.MustCompile(`<script\b`),
		"<style>body{display:none}</style>": regexp.MustCompile(`<style\b`),
		"<div onclick=\"alert(1)\">x</div>": regexp.MustCompile(`<div\b`),
		"a <b onmouseover=alert(1)>b</b>":   regexp.MustCompile(`<b\b`),
	} {
		if out := string(Markdown(in)); tag.MatchString(strings.ToLower(out)) {
			t.Errorf("%q → %q kept its tag", in, out)
		}
	}
}

// unsafeOutput describes output that could run: text a user typed, such as
// "onclick" outside a tag, is inert and stays.
var unsafeOutput = map[string]*regexp.Regexp{
	"a dangerous tag":         regexp.MustCompile(`<(script|img|style|div|iframe|object|embed|svg)\b`),
	"an event attribute":      regexp.MustCompile(`<[^>]*\son[a-z]+\s*=`),
	"a dangerous URL in href": regexp.MustCompile(`(href|src)\s*=\s*"?\s*(javascript|data|vbscript):`),
}

// Raw HTML is never markup, but the text the user typed stays visible.
func TestMarkdownShowsRawHTMLAsText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"inline generic", "Use Array<string> here", []string{"Use Array&lt;string&gt; here"}},
		{"inline br", "the <br> tag breaks lines", []string{"the &lt;br&gt; tag breaks lines"}},
		{"block div", "<div>\nimportant note\n</div>\nafter", []string{"&lt;div&gt;<br>", "important note<br>", "&lt;/div&gt;", "after"}},
		{"comment", "<!-- c -->x", []string{"&lt;!-- c --&gt;x"}},
		{"unclosed details", "<details>\nhidden reason\n\nnext para", []string{"&lt;details&gt;<br>", "hidden reason", "<p>next para</p>"}},
		{"image alt", "![logo](https://x.io/a.png)", []string{"<p>logo</p>"}},
	}
	for _, c := range cases {
		out := string(Markdown(c.in))
		for _, w := range c.want {
			if !strings.Contains(out, w) {
				t.Errorf("%s: %q → %q, want it to contain %q", c.name, c.in, out, w)
			}
		}
		if strings.Contains(out, "<img") || strings.Contains(out, "<div") || strings.Contains(out, "<details") || strings.Contains(out, "<!--") {
			t.Errorf("%s: %q → %q became markup", c.name, c.in, out)
		}
	}
}

func TestMarkdownRenders(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"**kalın** *eğik* ~~sil~~ `kod`", []string{"<strong>kalın</strong>", "<em>eğik</em>", "<del>sil</del>", "<code>kod</code>"}},
		{"a\nb", []string{"a<br>"}},
		{"a\n\nb", []string{"<p>a</p>", "<p>b</p>"}},
		{"# Başlık", []string{"<h2>Başlık</h2>"}},
		{"###### Altı", []string{"<h6>Altı</h6>"}},
		{"- bir\n- iki", []string{"<ul>", "<li>bir</li>"}},
		{"3. üç\n4. dört", []string{`<ol start="3">`}},
		{"- [x] bitti\n- [ ] kaldı", []string{`<input checked="" disabled="" type="checkbox"`, `<input disabled="" type="checkbox"`}},
		{"> alıntı", []string{"<blockquote>"}},
		{"| a | b |\n|---|---|\n| 1 | 2 |", []string{"<table>", "<th>a</th>", "<td>1</td>"}},
		{"see https://x.io/a.", []string{`<a href="https://x.io/a" rel="nofollow noreferrer">https://x.io/a</a>.`}},
		{"[site](https://x.io/q?a=1&b=2)", []string{`href="https://x.io/q?a=1&amp;b=2"`}},
		{"mail@ornek.com", []string{`href="mailto:mail@ornek.com"`}},
		{"@ayse bakar mısın", []string{"@ayse bakar mısın"}},
		{"<b>bold</b>", []string{"bold"}},
	}
	for _, c := range cases {
		out := string(Markdown(c.in))
		for _, w := range c.want {
			if !strings.Contains(out, w) {
				t.Errorf("%q → %q, want it to contain %q", c.in, out, w)
			}
		}
	}
	if strings.Contains(string(Markdown("# h")), "<h1") {
		t.Error("an h1 survived")
	}
	if strings.Contains(string(Markdown("<b>bold</b>")), "<b") {
		t.Error("raw <b> survived")
	}
}

func TestMarkdownMarksText(t *testing.T) {
	cases := map[string]string{
		"ama ==hepsini== kapsıyor": "ama <mark>hepsini</mark> kapsıyor",
		"==**kalın** işaret==":     "<mark><strong>kalın</strong> işaret</mark>",
		"a == b":                   "a == b",
		"=tek=":                    "=tek=",
		"===üç===":                 "===üç===",
		"`==kod==`":                "<code>==kod==</code>",
	}
	for in, want := range cases {
		if out := string(Markdown(in)); !strings.Contains(out, want) {
			t.Errorf("%q → %q, want it to contain %q", in, out, want)
		}
	}
	if out := string(Markdown("==<b onclick=x>a</b>==")); strings.Contains(out, "onclick=") && strings.Contains(out, "<b") {
		t.Errorf("raw HTML inside a mark became markup: %q", out)
	}
}
