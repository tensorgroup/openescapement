package render

import (
	"strings"
	"testing"
)

func meta() BlockMeta { return BlockMeta{Packs: []string{"acme-org@1.4.0", "eng-dept@2.1.0"}} }

func TestSpliceIntoEmptyFile(t *testing.T) {
	out, err := Splice(nil, "policy body\n", meta())
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "<!-- escapement:begin packs=acme-org@1.4.0,eng-dept@2.1.0 hash=sha256:") {
		t.Errorf("begin marker missing/wrong:\n%s", s)
	}
	if !strings.Contains(s, "policy body\n") || !strings.Contains(s, "<!-- escapement:end -->") {
		t.Errorf("body or end missing:\n%s", s)
	}
}

func TestSpliceAppendPreservesOutside(t *testing.T) {
	team := "# CLAUDE.md\n\nOur build uses pnpm.\n"
	out, err := Splice([]byte(team), "body\n", meta())
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	// New blocks land after the leading H1, not at the end of the file, so
	// the heading is a prefix and the rest of the team content is a suffix.
	if !strings.HasPrefix(s, "# CLAUDE.md\n\n") {
		t.Errorf("heading not preserved as prefix:\n%s", out)
	}
	if !strings.HasSuffix(s, "Our build uses pnpm.\n") {
		t.Errorf("team content not preserved as suffix:\n%s", out)
	}
}

func TestSplicePlaceholder(t *testing.T) {
	team := "# Top\n\n<!-- escapement:block -->\n\n# Bottom\n"
	out, err := Splice([]byte(team), "body\n", meta())
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "# Top") || !strings.Contains(s, "# Bottom") {
		t.Errorf("surroundings lost:\n%s", s)
	}
	if strings.Index(s, "escapement:begin") > strings.Index(s, "# Bottom") {
		t.Errorf("block not at placeholder position:\n%s", s)
	}
	if strings.Contains(s, "escapement:block") {
		t.Errorf("placeholder not consumed:\n%s", s)
	}
}

func TestSpliceReplacesExistingBlockOnly(t *testing.T) {
	first, err := Splice([]byte("before\n"), "v1 body\n", meta())
	if err != nil {
		t.Fatal(err)
	}
	withSuffix := append([]byte{}, first...)
	withSuffix = append(withSuffix, []byte("\nafter\n")...)
	out, err := Splice(withSuffix, "v2 body\n", meta())
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	// "before\n" has no frontmatter or H1 to anchor on, so the block landed
	// at the top of the file (before "before") on the first Splice; the
	// second Splice must replace it in place, leaving "before"/"after" as a
	// suffix in their original order.
	if !strings.HasSuffix(s, "before\n\nafter\n") {
		t.Errorf("outside bytes modified:\n%q", s)
	}
	if strings.Contains(s, "v1 body") || !strings.Contains(s, "v2 body") {
		t.Errorf("body not replaced:\n%s", s)
	}
	if strings.Count(s, "escapement:begin") != 1 {
		t.Errorf("expected exactly one block:\n%s", s)
	}
}

func TestSpliceCorruptBlock(t *testing.T) {
	if _, err := Splice([]byte("x\n<!-- escapement:begin packs=a@1 hash=sha256:00 -->\nbody\n"), "b\n", meta()); err == nil {
		t.Error("begin without end: want error")
	}
	two := "<!-- escapement:begin packs=a@1 hash=h -->\nb\n<!-- escapement:end -->\n<!-- escapement:begin packs=a@1 hash=h -->\nb\n<!-- escapement:end -->\n"
	if _, err := Splice([]byte(two), "b\n", meta()); err == nil {
		t.Error("two blocks: want error")
	}
}

func TestExtractRoundTrip(t *testing.T) {
	body := "the body\nwith two lines\n"
	out, err := Splice(nil, body, meta())
	if err != nil {
		t.Fatal(err)
	}
	b, err := Extract(out)
	if err != nil {
		t.Fatal(err)
	}
	if b == nil {
		t.Fatal("Extract returned nil for file with block")
	}
	if b.Body != body {
		t.Errorf("body round-trip: %q", b.Body)
	}
	if b.Hash != BodyHash(body) {
		t.Errorf("hash mismatch: %s vs %s", b.Hash, BodyHash(body))
	}
	if len(b.Packs) != 2 || b.Packs[0] != "acme-org@1.4.0" {
		t.Errorf("packs: %v", b.Packs)
	}
	none, err := Extract([]byte("no block here\n"))
	if err != nil || none != nil {
		t.Errorf("no block: want nil,nil got %v,%v", none, err)
	}
}

func TestSpliceTopPlacement(t *testing.T) {
	meta := BlockMeta{Packs: []string{"p@1"}}
	cases := []struct {
		name, existing, wantPrefix string
	}{
		{"plain", "team rules\n", "<!-- escapement:begin "},
		{"h1", "# Project\n\nteam rules\n", "# Project\n\n<!-- escapement:begin "},
		{"h1 no blank", "# Project\nteam rules\n", "# Project\n<!-- escapement:begin "},
		{"frontmatter", "---\ntitle: x\n---\nteam rules\n", "---\ntitle: x\n---\n<!-- escapement:begin "},
		{"frontmatter and h1", "---\ntitle: x\n---\n# P\n\nteam\n", "---\ntitle: x\n---\n# P\n\n<!-- escapement:begin "},
		{"hashtag not h1", "#hashtag\n", "<!-- escapement:begin "},
		{"subheading not h1", "## Sub\n", "<!-- escapement:begin "},
		{"unterminated frontmatter", "---\ntitle: x\n", "<!-- escapement:begin "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Splice([]byte(tc.existing), "body\n", meta)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(out), tc.wantPrefix) {
				t.Errorf("got:\n%s\nwant prefix:\n%s", out, tc.wantPrefix)
			}
			// Every original byte survives, in order.
			if !strings.Contains(string(out), strings.TrimPrefix(tc.existing, tc.wantPrefix)) &&
				!strings.Contains(string(out), "team") && tc.existing != "" {
				t.Errorf("original content lost:\n%s", out)
			}
		})
	}
}

func TestSpliceExistingBlockDoesNotMove(t *testing.T) {
	meta := BlockMeta{Packs: []string{"p@1"}}
	first, err := Splice([]byte("# P\n\nteam rules\n"), "body\n", meta)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a block that lives at the bottom already.
	bottom := "# P\n\nteam rules\n\n" + string(first[strings.Index(string(first), "<!-- escapement:begin "):])
	out, err := Splice([]byte(bottom), "body2\n", meta)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(out), "# P\n\nteam rules\n") {
		t.Errorf("existing block moved:\n%s", out)
	}
}

func TestSplicePlaceholderWinsOverTopPlacement(t *testing.T) {
	meta := BlockMeta{Packs: []string{"p@1"}}
	out, err := Splice([]byte("# P\n\nteam\n"+Placeholder+"\n"), "body\n", meta)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(out), "# P\n\nteam\n") {
		t.Errorf("placeholder ignored:\n%s", out)
	}
}

func TestBlockSurround(t *testing.T) {
	meta := BlockMeta{Packs: []string{"p@1"}}
	withBlock, err := Splice([]byte("# P\n\nteam rules\n"), "body\n", meta)
	if err != nil {
		t.Fatal(err)
	}
	got, err := BlockSurround(withBlock)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "team rules") || strings.Contains(got, "escapement:begin") {
		t.Errorf("surround = %q", got)
	}

	only, err := Splice(nil, "body\n", meta)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := BlockSurround(only); err != nil || got != "" {
		t.Errorf("block-only file: got %q, err %v; want empty", got, err)
	}

	if got, err := BlockSurround([]byte("no block here\n")); err != nil || got != "" {
		t.Errorf("no block: got %q, err %v; want empty", got, err)
	}
}

// FrontmatterEnd is exported for esc init's "above the title" placement,
// which must land above everything a human wrote but still below
// frontmatter, since frontmatter is only frontmatter at byte 0. insertAt is
// built on the same function, so the two can never disagree about where
// frontmatter ends.
func TestFrontmatterEnd(t *testing.T) {
	cases := []struct {
		name, in string
		want     int
	}{
		{"none", "# Rules\n\nteam\n", 0},
		{"frontmatter", "---\ntitle: x\n---\nteam\n", len("---\ntitle: x\n---\n")},
		{"frontmatter then blank line", "---\ntitle: x\n---\n\n# R\n", len("---\ntitle: x\n---\n")},
		{"no trailing newline after fence", "---\ntitle: x\n---", len("---\ntitle: x\n---")},
		{"unterminated fence", "---\ntitle: x\n# R\n", 0},
		{"fence not at byte 0", "intro\n---\ntitle: x\n---\n", 0},
		{"close fence with trailing text", "---\ntitle: x\n---x\nteam\n", 0},
		{"empty", "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FrontmatterEnd([]byte(tc.in)); got != tc.want {
				t.Errorf("FrontmatterEnd(%q) = %d, want %d", tc.in, got, tc.want)
			}
			// Whatever the offset, it must be a valid line boundary or the
			// end of input: never mid-line inside the frontmatter itself.
			at := FrontmatterEnd([]byte(tc.in))
			if at > 0 && at < len(tc.in) && tc.in[at-1] != '\n' {
				t.Errorf("offset %d is mid-line in %q", at, tc.in)
			}
		})
	}
}
