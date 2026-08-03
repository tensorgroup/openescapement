package web

import (
	"bytes"
	"html/template"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// md is the shared, safe markdown renderer for portal display. It uses
// goldmark's default renderer/html with Unsafe left off, so raw source HTML is
// escaped and dangerous link schemes (javascript:, data:, ...) are filtered by
// html.IsDangerousURL. Linkify autolinks bare URLs, restricted to http and
// https. A heading offset maps '#' to <h2> so the layout's <h1> stays the
// page's only top-level heading.
var md = goldmark.New(
	goldmark.WithExtensions(
		extension.NewLinkify(
			extension.WithLinkifyAllowedProtocols([][]byte{[]byte("http"), []byte("https")}),
		),
	),
	goldmark.WithParserOptions(
		parser.WithASTTransformers(util.Prioritized(headingOffset{}, 100)),
	),
)

// headingOffset bumps every heading one level deeper (# -> h2, ## -> h3, ...)
// so rendered guidance and fragment content never emits a competing <h1>.
type headingOffset struct{}

func (headingOffset) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if h, ok := n.(*ast.Heading); ok && h.Level < 6 {
				h.Level++
			}
		}
		return ast.WalkContinue, nil
	})
}

// mdHTML renders a safe subset of markdown to HTML for portal display. Raw
// source HTML is escaped; bare http/https URLs and [text](url) links become
// anchors carrying rel="noreferrer"; dangerous-scheme link targets are
// filtered by goldmark's default (safe) renderer. A leading YAML front-matter
// block is stripped so fragment metadata is not rendered as body.
func mdHTML(src []byte) template.HTML {
	var buf bytes.Buffer
	if err := md.Convert(stripFrontmatter(src), &buf); err != nil {
		// Convert only errors on a writer failure; fall back to escaped source.
		return template.HTML(template.HTMLEscapeString(string(src)))
	}
	// goldmark does not emit rel on anchors; add it uniformly. The safe
	// renderer always writes `<a href="` first and has already escaped and
	// scheme-filtered the destination, so this replace cannot inject markup.
	out := strings.ReplaceAll(buf.String(), `<a href=`, `<a rel="noreferrer" href=`)
	return template.HTML(out)
}

// stripFrontmatter removes a leading "---\n...\n---\n" YAML block, matching how
// pack fragments and guidance examples carry their targets metadata. Content
// without a leading front-matter block is returned unchanged.
func stripFrontmatter(src []byte) []byte {
	s := string(src)
	if !strings.HasPrefix(s, "---\n") {
		return src
	}
	rest := s[len("---\n"):]
	if i := strings.Index(rest, "\n---\n"); i >= 0 {
		return []byte(rest[i+len("\n---\n"):])
	}
	return src
}
