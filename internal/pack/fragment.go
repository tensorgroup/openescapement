package pack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/targets"
)

// Fragment is one markdown rule file. Empty Targets means the fragment applies
// to every built-in target (never a custom target).
type Fragment struct {
	Path    string
	Targets []string
	Body    string
}

// loadFragment parses one rule file. A fragment may name built-in fragment
// targets or one of its own pack's declared custom targets (customNames);
// naming anything else — including another pack's custom target — is a
// constraint failure (§3 unknown/foreign reference). packName is the owning
// pack's manifest name, included in the error so the operator can tell which
// pack authored the offending rule without cross-referencing the path.
func loadFragment(packDir, packName, rel string, customNames map[string]bool) (*Fragment, error) {
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
			if !targets.IsFragmentTarget(tgt) && !customNames[tgt] {
				return nil, fmt.Errorf("%w: pack %q: rule %s: unknown or foreign target %q (not a built-in target or a custom target defined by this pack)", esc.ErrConstraint, packName, rel, tgt)
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
