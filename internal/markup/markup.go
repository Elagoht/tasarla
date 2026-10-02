// Package markup renders user text written in Markdown as safe HTML.
package markup

import (
	"bytes"
	"html/template"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// shiftHeadings moves every heading one level down, so a "#" in a card does
// not compete with the page's own titles.
type shiftHeadings struct{}

func (shiftHeadings) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if h, ok := n.(*ast.Heading); ok && entering {
			h.Level = min(h.Level+1, 6)
		}
		return ast.WalkContinue, nil
	})
}

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithASTTransformers(util.Prioritized(shiftHeadings{}, 100))),
	goldmark.WithRendererOptions(html.WithHardWraps()),
)

var policy = func() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "br", "strong", "em", "del", "code", "pre", "blockquote", "ul", "ol", "li",
		"h2", "h3", "h4", "h5", "h6", "table", "thead", "tbody", "tr", "th", "td", "hr")
	p.AllowAttrs("href").OnElements("a")
	p.AllowURLSchemes("http", "https", "mailto")
	p.RequireParseableURLs(true)
	p.RequireNoFollowOnLinks(true)
	p.RequireNoReferrerOnLinks(true)
	p.AllowAttrs("start").Matching(regexp.MustCompile(`^[0-9]{1,6}$`)).OnElements("ol")
	p.AllowAttrs("type").Matching(regexp.MustCompile(`^checkbox$`)).OnElements("input")
	p.AllowAttrs("checked", "disabled").Matching(regexp.MustCompile(`^(|checked|disabled)$`)).OnElements("input")
	return p
}()

var inputTag = regexp.MustCompile(`<input\b[^>]*>`)

// onlyTaskCheckboxes drops every <input> that is not a disabled checkbox.
// bluemonday matches attributes one at a time and cannot require a pair, so
// this enforces it: the sole <input> goldmark makes is a task-list checkbox,
// and a form control a user could use must never reach a card.
func onlyTaskCheckboxes(s string) string {
	return inputTag.ReplaceAllStringFunc(s, func(tag string) string {
		if strings.Contains(tag, ` type="checkbox"`) && strings.Contains(tag, ` disabled=""`) {
			return tag
		}
		return ""
	})
}

// clean is the last step of Markdown: the policy, then the checkbox rule.
func clean(rendered []byte) string {
	return onlyTaskCheckboxes(string(policy.SanitizeBytes(rendered)))
}

// Markdown renders s, GitHub-flavoured, line breaks kept and raw HTML left
// out, and keeps only the elements and links a card may show.
func Markdown(s string) template.HTML {
	var buf bytes.Buffer
	if err := md.Convert([]byte(s), &buf); err != nil {
		return template.HTML(template.HTMLEscapeString(s))
	}
	return template.HTML(clean(buf.Bytes()))
}
