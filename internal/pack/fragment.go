package pack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/tensorgroup/openescapement/internal/esc"
)

// Fragment is one markdown rule file. Empty Targets means the fragment
// applies to every target.
type Fragment struct {
	Path    string
	Targets []string
	Body    string
}

// ValidTargets are the render targets a fragment may name in frontmatter.
var ValidTargets = map[string]bool{
	"claude": true, "agents": true, "gemini": true, "governance": true,
}

func loadFragment(packDir, rel string) (*Fragment, error) {
	raw, err := os.ReadFile(filepath.Join(packDir, rel))
	if err != nil {
		return nil, fmt.Errorf("%w: rule %s: %v", esc.ErrManifest, rel, err)
	}
	frag := &Fragment{Path: rel}
	content := string(raw)
	if fm, body, ok := splitFrontmatter(content); ok {
		var meta struct {
			Targets []string `yaml:"targets"`
		}
		if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
			return nil, fmt.Errorf("%w: rule %s frontmatter: %v", esc.ErrManifest, rel, err)
		}
		for _, tgt := range meta.Targets {
			if !ValidTargets[tgt] {
				return nil, fmt.Errorf("%w: rule %s: unknown target %q", esc.ErrManifest, rel, tgt)
			}
		}
		frag.Targets = meta.Targets
		frag.Body = body
	} else {
		frag.Body = content
	}
	return frag, nil
}

// splitFrontmatter splits "---\n<yaml>\n---\n<body>". Returns ok=false when
// the file has no frontmatter.
func splitFrontmatter(content string) (fm, body string, ok bool) {
	if !strings.HasPrefix(content, "---\n") {
		return "", "", false
	}
	rest := content[len("---\n"):]
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return "", "", false
	}
	return rest[:idx], rest[idx+len("\n---\n"):], true
}
