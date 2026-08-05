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
// byte-identical so single-fragment adoption keeps its historical output.
// A part with no frontmatter contributes no targets; callers are expected
// to compose fragments that share a targets posture.
func ComposeFragments(parts [][]byte) ([]byte, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("%w: compose: no fragments selected", esc.ErrManifest)
	}
	if len(parts) == 1 {
		return parts[0], nil
	}
	var union []string
	seen := map[string]bool{}
	bodies := make([]string, 0, len(parts))
	for i, p := range parts {
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
