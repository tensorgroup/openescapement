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
	if !strings.HasPrefix(string(out), team) {
		t.Errorf("team content not preserved as prefix:\n%s", out)
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
	if !strings.HasPrefix(s, "before\n") || !strings.HasSuffix(s, "\nafter\n") {
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
