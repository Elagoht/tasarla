package web

import (
	"html"
	"html/template"
	"regexp"
	"strings"
)

var urlPattern = regexp.MustCompile(`https?://[^\s<>"']+`)

// richText shows user text as written: escaped, line breaks kept, and http(s)
// links made clickable (spec §12.3). Nothing the user wrote becomes markup.
func richText(s string) template.HTML {
	var b strings.Builder
	plain := func(t string) {
		b.WriteString(strings.ReplaceAll(html.EscapeString(t), "\n", "<br>\n"))
	}
	last := 0
	for _, m := range urlPattern.FindAllStringIndex(s, -1) {
		start, end := m[0], m[1]
		// Punctuation closing a sentence is not part of the link.
		for end > start && strings.ContainsRune(".,;:!?)]}", rune(s[end-1])) {
			end--
		}
		plain(s[last:start])
		link := html.EscapeString(s[start:end])
		b.WriteString(`<a href="` + link + `" rel="noopener noreferrer nofollow">` + link + `</a>`)
		last = end
	}
	plain(s[last:])
	return template.HTML(b.String())
}
