package render

import (
	"fmt"
	"strings"

	"github.com/tensorgroup/openescapement/internal/esc"
)

const (
	beginPrefix = "<!-- escapement:begin "
	endMarker   = "<!-- escapement:end -->"
	Placeholder = "<!-- escapement:block -->"
)

// BlockMeta identifies the packs a managed block was rendered from.
type BlockMeta struct {
	Packs []string // "name@version" in render order
}

// Block is a managed block found in a file.
type Block struct {
	Packs []string
	Hash  string
	Body  string
	start int // byte offset of begin marker
	end   int // byte offset one past end marker line
}

// BodyHash returns the canonical hash of a block body.
func BodyHash(body string) string { return esc.HashBytes([]byte(body)) }

func renderBlock(body string, meta BlockMeta) string {
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	begin := fmt.Sprintf("%spacks=%s hash=%s -->", beginPrefix, strings.Join(meta.Packs, ","), BodyHash(body))
	return begin + "\n" + body + endMarker + "\n"
}

// Splice returns existing with the managed block set to body. The block is
// replaced in place if present, substituted for the placeholder comment if
// present, and appended otherwise. Bytes outside the block are never modified.
func Splice(existing []byte, body string, meta BlockMeta) ([]byte, error) {
	block := renderBlock(body, meta)
	s := string(existing)
	found, err := Extract(existing)
	if err != nil {
		return nil, err
	}
	if found != nil {
		return []byte(s[:found.start] + block + s[found.end:]), nil
	}
	if idx := strings.Index(s, Placeholder); idx >= 0 {
		return []byte(s[:idx] + strings.TrimSuffix(block, "\n") + s[idx+len(Placeholder):]), nil
	}
	if len(s) == 0 {
		return []byte(block), nil
	}
	sep := "\n"
	if !strings.HasSuffix(s, "\n") {
		sep = "\n\n"
	}
	return []byte(s + sep + block), nil
}

// RemoveBlock returns file with its managed block removed, preserving every
// byte outside the block. removed reports whether a block was present. An
// error is returned only when the block structure is corrupt.
func RemoveBlock(file []byte) ([]byte, bool, error) {
	b, err := Extract(file)
	if err != nil {
		return nil, false, err
	}
	if b == nil {
		return file, false, nil
	}
	s := string(file)
	return []byte(s[:b.start] + s[b.end:]), true, nil
}

// Extract finds the managed block in file. Returns (nil, nil) when absent and
// an error when the block structure is corrupt (unterminated or duplicated).
func Extract(file []byte) (*Block, error) {
	s := string(file)
	begin := strings.Index(s, beginPrefix)
	if begin < 0 {
		if strings.Contains(s, endMarker) {
			return nil, fmt.Errorf("managed block corrupt: end marker without begin")
		}
		return nil, nil
	}
	if strings.Index(s[begin+len(beginPrefix):], beginPrefix) >= 0 {
		return nil, fmt.Errorf("managed block corrupt: multiple begin markers")
	}
	headerEnd := strings.Index(s[begin:], "-->")
	if headerEnd < 0 {
		return nil, fmt.Errorf("managed block corrupt: unterminated begin marker")
	}
	header := s[begin+len(beginPrefix) : begin+headerEnd]
	bodyStart := begin + headerEnd + len("-->")
	if strings.HasPrefix(s[bodyStart:], "\n") {
		bodyStart++
	}
	endIdx := strings.Index(s[bodyStart:], endMarker)
	if endIdx < 0 {
		return nil, fmt.Errorf("managed block corrupt: begin without end marker")
	}
	b := &Block{
		Body:  s[bodyStart : bodyStart+endIdx],
		start: begin,
		end:   bodyStart + endIdx + len(endMarker),
	}
	if strings.HasPrefix(s[b.end:], "\n") {
		b.end++
	}
	for _, field := range strings.Fields(strings.TrimSpace(header)) {
		switch {
		case strings.HasPrefix(field, "packs="):
			b.Packs = strings.Split(strings.TrimPrefix(field, "packs="), ",")
		case strings.HasPrefix(field, "hash="):
			b.Hash = strings.TrimPrefix(field, "hash=")
		}
	}
	return b, nil
}
