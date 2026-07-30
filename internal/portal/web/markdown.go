package web

import (
	"html/template"
	"regexp"
	"strings"
)

// mdHTML renders a minimal, safe subset of markdown to HTML for displaying
// pack rule fragments. Every input line is HTML-escaped first, so no raw
// HTML, links, or images from the source ever reach the page — this is by
// design: it renders policy text, not a general-purpose markdown document.
// Supported after escaping: '#'/'##'/'###' headers, "- " list runs, ```
// fences, blank-line-separated paragraphs, and inline **bold** / `code`.
func mdHTML(src []byte) template.HTML {
	lines := strings.Split(string(src), "\n")
	for i, l := range lines {
		lines[i] = template.HTMLEscapeString(l)
	}

	var b strings.Builder
	i := 0
	for i < len(lines) {
		line := lines[i]
		switch {
		case line == "":
			i++
		case line == "```":
			i++
			start := i
			for i < len(lines) && lines[i] != "```" {
				i++
			}
			b.WriteString("<pre><code>")
			b.WriteString(strings.Join(lines[start:i], "\n"))
			b.WriteString("</code></pre>\n")
			if i < len(lines) {
				i++ // skip closing fence
			}
		case strings.HasPrefix(line, "### "):
			b.WriteString("<h4>" + inlineMd(strings.TrimPrefix(line, "### ")) + "</h4>\n")
			i++
		case strings.HasPrefix(line, "## "):
			b.WriteString("<h3>" + inlineMd(strings.TrimPrefix(line, "## ")) + "</h3>\n")
			i++
		case strings.HasPrefix(line, "# "):
			b.WriteString("<h2>" + inlineMd(strings.TrimPrefix(line, "# ")) + "</h2>\n")
			i++
		case strings.HasPrefix(line, "- "):
			b.WriteString("<ul>\n")
			for i < len(lines) && strings.HasPrefix(lines[i], "- ") {
				b.WriteString("<li>" + inlineMd(strings.TrimPrefix(lines[i], "- ")) + "</li>\n")
				i++
			}
			b.WriteString("</ul>\n")
		default:
			start := i
			for i < len(lines) && lines[i] != "" && !isMdBlockStart(lines[i]) {
				i++
			}
			b.WriteString("<p>" + inlineMd(strings.Join(lines[start:i], " ")) + "</p>\n")
		}
	}
	return template.HTML(b.String())
}

// isMdBlockStart reports whether line begins a header, list, or fence
// block, so a paragraph run stops there instead of swallowing it.
func isMdBlockStart(line string) bool {
	return line == "```" ||
		strings.HasPrefix(line, "# ") ||
		strings.HasPrefix(line, "## ") ||
		strings.HasPrefix(line, "### ") ||
		strings.HasPrefix(line, "- ")
}

var (
	mdCodeRE = regexp.MustCompile("`([^`]+)`")
	mdBoldRE = regexp.MustCompile(`\*\*(.+?)\*\*`)
)

// inlineMd applies the two supported inline spans to an already-escaped
// line: `code` before **bold**, so bold markers inside a code span aren't
// mistaken for emphasis.
func inlineMd(s string) string {
	s = mdCodeRE.ReplaceAllString(s, "<code>$1</code>")
	s = mdBoldRE.ReplaceAllString(s, "<strong>$1</strong>")
	return s
}
