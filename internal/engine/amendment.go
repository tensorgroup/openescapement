package engine

import (
	"sort"
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
	// The hash must cover Items as well as Content: for an items-only
	// amendment (kind=json-keys today, kind=dir in Task 6) Content is always
	// "", so hashing Content alone would give every distinct set of dropped
	// items the same constant hash and the portal could never tell one
	// team amendment from another across reports. Items are sorted into a
	// local copy before hashing (never mutating the caller's slice) so the
	// hash is independent of call-site ordering: producers already sort,
	// but the hash must not depend on that silently. Content and the
	// canonicalized items are joined with a NUL separator, which cannot
	// appear in either, so no pair of distinct (content, items) inputs can
	// collide by shifting a boundary between them.
	sortedItems := append([]string(nil), items...)
	sort.Strings(sortedItems)
	a.Hash = esc.HashBytes([]byte(content + "\x00" + strings.Join(sortedItems, "\x00")))
	a.Content = content
	return a
}
