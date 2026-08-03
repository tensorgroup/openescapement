package web

import (
	"strings"
	"testing"
)

func TestMdHTML(t *testing.T) {
	src := "# Title\n\nPara with **bold** and `code`.\n\n- a\n- b\n\n```\nx < y\n```\n\n<script>alert(1)</script>\n"
	got := string(mdHTML([]byte(src)))
	for _, want := range []string{"<h2>Title</h2>", "<strong>bold</strong>", "<code>code</code>",
		"<li>a</li>", "<pre><code>x &lt; y", "<!-- raw HTML omitted -->"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "<script>") {
		t.Fatal("raw HTML leaked")
	}
}

func TestMdHTMLLinks(t *testing.T) {
	cases := []struct {
		name, in      string
		want, notWant []string
	}{
		{"markdown link", "See [docs](https://a.com/x) here.",
			[]string{`href="https://a.com/x"`, `>docs</a>`, `rel="noreferrer"`}, nil},
		{"bare url autolink", "Visit https://a.com/y now",
			[]string{`href="https://a.com/y"`}, nil},
		{"ampersand url", "Query https://a.com/s?x=1&y=2 end",
			[]string{`href="https://a.com/s?x=1&amp;y=2"`}, nil},
		{"hostile scheme not clickable", "Click [x](javascript:alert(1)) please",
			nil, []string{"javascript"}},
		{"url in code span not linkified", "Run `https://a.com/z` today",
			[]string{"<code>https://a.com/z</code>"}, []string{"<a "}},
		{"bold and link", "**note** https://a.com/w",
			[]string{"<strong>note</strong>", `href="https://a.com/w"`}, nil},
		{"link in list item", "- see [docs](https://a.com/l)",
			[]string{"<li>", `href="https://a.com/l"`}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := string(mdHTML([]byte(c.in)))
			for _, w := range c.want {
				if !strings.Contains(got, w) {
					t.Fatalf("missing %q in:\n%s", w, got)
				}
			}
			for _, nw := range c.notWant {
				if strings.Contains(got, nw) {
					t.Fatalf("unexpected %q in:\n%s", nw, got)
				}
			}
		})
	}
}
