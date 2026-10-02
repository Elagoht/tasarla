package markup

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// ==text== marks text, as <mark>. It is parsed like GFM's ~~text~~: a run of
// exactly two "=" that can open or close by the flanking rules, so "a == b"
// stays text.

var kindMark = ast.NewNodeKind("Mark")

type markNode struct{ ast.BaseInline }

func (n *markNode) Kind() ast.NodeKind { return kindMark }

func (n *markNode) Dump(source []byte, level int) { ast.DumpHelper(n, source, level, nil, nil) }

type markDelimiters struct{}

func (markDelimiters) IsDelimiter(b byte) bool { return b == '=' }

func (markDelimiters) CanOpenCloser(opener, closer *parser.Delimiter) bool {
	return opener.Char == closer.Char
}

func (markDelimiters) OnMatch(int) ast.Node { return &markNode{} }

type markParser struct{}

func (markParser) Trigger() []byte { return []byte{'='} }

func (markParser) Parse(_ ast.Node, block text.Reader, pc parser.Context) ast.Node {
	before := block.PrecendingCharacter()
	line, segment := block.PeekLine()
	node := parser.ScanDelimiter(line, before, 2, markDelimiters{})
	if node == nil || node.OriginalLength != 2 || before == '=' {
		return nil
	}
	node.Segment = segment.WithStop(segment.Start + node.OriginalLength)
	block.Advance(node.OriginalLength)
	pc.PushDelimiter(node)
	return node
}

func (markParser) CloseBlock(ast.Node, parser.Context) {}

type markRenderer struct{}

func (markRenderer) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	r.Register(kindMark, func(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			_, _ = w.WriteString("<mark>")
		} else {
			_, _ = w.WriteString("</mark>")
		}
		return ast.WalkContinue, nil
	})
}

type markExtension struct{}

func (markExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithInlineParsers(util.Prioritized(markParser{}, 500)))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(markRenderer{}, 500)))
}
