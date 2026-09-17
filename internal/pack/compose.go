package pack

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/tensorgroup/openescapement/internal/esc"
)

// ComposeFragments merges rule fragments into one: a single leading
// frontmatter carrying the union of the parts' targets (first-appearance
// order), then the bodies joined by blank lines. A sole part passes through
// as-is (after CRLF normalization) so single-fragment adoption keeps its
// historical output. A part with no frontmatter contributes no targets;
// callers are expected to compose fragments that share a targets posture.
func ComposeFragments(parts [][]byte) ([]byte, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("%w: compose: no fragments selected", esc.ErrManifest)
	}
	// Normalize endings up front so the LF-only splitter never lets a CRLF
	// scoped part slip through unparsed.
	norm := make([][]byte, len(parts))
	for i, p := range parts {
		norm[i] = []byte(strings.ReplaceAll(string(p), "\r\n", "\n"))
	}
	for i, p := range norm {
		has, err := partHasPaths(p, i)
		if err != nil {
			return nil, err
		}
		if has {
			return nil, fmt.Errorf("%w: compose part %d: a path-scoped rule fragment cannot be composed", esc.ErrManifest, i)
		}
	}
	if len(norm) == 1 {
		return norm[0], nil
	}
	var union []string
	seen := map[string]bool{}
	bodies := make([]string, 0, len(norm))
	for i, p := range norm {
		content := string(p)
		if fm, body, ok := splitFrontmatter(content); ok {
			var meta struct {
				Targets []string `yaml:"targets"`
			}
			if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
				return nil, fmt.Errorf("%w: compose part %d frontmatter: %v", esc.ErrManifest, i, err)
			}
			for _, t := range meta.Targets {
				if !seen[t] {
					seen[t] = true
					union = append(union, t)
				}
			}
			content = body
		}
		bodies = append(bodies, strings.TrimSpace(content))
	}
	var b strings.Builder
	if len(union) > 0 {
		b.WriteString("---\ntargets: [" + strings.Join(union, ", ") + "]\n---\n\n")
	}
	b.WriteString(strings.Join(bodies, "\n\n"))
	b.WriteString("\n")
	return []byte(b.String()), nil
}

// partHasPaths reports whether a compose part's frontmatter carries a paths:
// key at all, regardless of the value's shape. Composing must fail closed
// the same way the loader does: a scalar, an empty sequence, or a null
// paths: is a malformed rules fragment, not an unscoped one, and letting any
// of those shapes through here would have the rebuilt frontmatter silently
// drop the scope instead of refusing to compose it.
func partHasPaths(part []byte, i int) (bool, error) {
	fm, _, ok := splitFrontmatter(string(part))
	if !ok {
		return false, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(fm), &doc); err != nil {
		return false, fmt.Errorf("%w: compose part %d frontmatter: %v", esc.ErrManifest, i, err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return false, nil
	}
	m := doc.Content[0]
	for j := 0; j+1 < len(m.Content); j += 2 {
		if m.Content[j].Value == "paths" {
			return true, nil
		}
	}
	return false, nil
}
