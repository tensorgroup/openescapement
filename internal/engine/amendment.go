package engine

import (
	"strings"

	"github.com/tensorgroup/openescapement/internal/esc"
)

// LocalState classifies content escapement does not own that shares an
// artifact with content it does. It is orthogonal to State: a file can carry
// both a local amendment and an altered managed region, and collapsing the
// two would force dropping one of those facts.
type LocalState string

const (
	LocalNone    LocalState = "none"
	LocalAmended LocalState = "amended"
)

// Amendment describes unmanaged content sharing an artifact with managed
// content. Content carries surrounding text (kind=block); Items carries
// discrete unmanaged names: file paths for kind=dir, server names for
// kind=json-keys.
type Amendment struct {
	Bytes   int      `json:"bytes"`
	Lines   int      `json:"lines"`
	Hash    string   `json:"hash"`
	Items   []string `json:"items,omitempty"`
	Content string   `json:"content,omitempty"`
}

// Alteration describes a hand-edited managed region. Diff is populated only
// by the JSON report surfaces, which can afford the git shell-out; human
// output points at `esc diff` instead.
type Alteration struct {
	ExpectedHash string `json:"expected_hash"`
	ActualHash   string `json:"actual_hash"`
	Diff         string `json:"diff,omitempty"`
}

// newAmendment returns nil when there is nothing unmanaged to report.
func newAmendment(content string, items []string) *Amendment {
	if strings.TrimSpace(content) == "" && len(items) == 0 {
		return nil
	}
	a := &Amendment{
		Bytes: len(content),
		Lines: strings.Count(content, "\n"),
		Items: items,
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		a.Lines++
	}
	a.Hash = esc.HashBytes([]byte(content))
	a.Content = content
	return a
}
