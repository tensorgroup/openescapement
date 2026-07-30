package web

import (
	"strings"
	"testing"
)

func TestMdHTML(t *testing.T) {
	src := "# Title\n\nPara with **bold** and `code`.\n\n- a\n- b\n\n```\nx < y\n```\n\n<script>alert(1)</script>\n"
	got := string(mdHTML([]byte(src)))
	for _, want := range []string{"<h2>Title</h2>", "<strong>bold</strong>", "<code>code</code>",
		"<li>a</li>", "<pre><code>x &lt; y", "&lt;script&gt;"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "<script>") {
		t.Fatal("raw HTML leaked")
	}
}
