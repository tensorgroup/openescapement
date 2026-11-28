package pack

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	Paths   []string // glob scopes for a rules fragment; nil for every other fragment
	Body    string
}

// ruleStem constrains a rules fragment's filename stem, which becomes part of
// an on-disk filename under .claude/rules/.
var ruleStem = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

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
	// Normalize CRLF to LF at load: the splitter recognizes only LF fences, so
	// a CRLF fragment would otherwise lose its frontmatter (and its scope)
	// silently. Same one-conversion rule as render.NormalizeEndings, inlined
	// because pack must not import render (cycle).
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if fm, body, ok := splitFrontmatter(content); ok {
		// The frontmatter is parsed twice — once into a typed struct for
		// targets, once into a yaml.Node (in parsePaths) for the strict
		// paths: shape check a struct decode can't express. Frontmatter is a
		// few lines, so the extra parse is not worth the coupling a single
		// shared decode would add between two independently-varying checks.
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
		paths, hasPaths, perr := parsePaths([]byte(fm), packName, rel)
		if perr != nil {
			return nil, perr
		}
		if err := validateRulePaths(packName, rel, meta.Targets, hasPaths, body); err != nil {
			return nil, err
		}
		frag.Targets = meta.Targets
		frag.Paths = paths
		frag.Body = body
	} else {
		frag.Body = content
	}
	return frag, nil
}

// parsePaths extracts the paths: field strictly from frontmatter via a
// yaml.Node, so absent (nil, present=false) is distinguishable from null, an
// empty sequence, a scalar, and non-string elements — all of which are
// errors. present reports whether the key appeared at all.
func parsePaths(fm []byte, packName, rel string) (paths []string, present bool, err error) {
	var doc yaml.Node
	if uerr := yaml.Unmarshal(fm, &doc); uerr != nil {
		return nil, false, fmt.Errorf("%w: rule %s frontmatter: %v", esc.ErrManifest, rel, uerr)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, false, nil
	}
	m := doc.Content[0]
	fail := func(msg string) error {
		return fmt.Errorf("%w: pack %q: rule %s: %s", esc.ErrManifest, packName, rel, msg)
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Tag == "!!merge" {
			return nil, false, fail("merge keys are not supported in fragment frontmatter, because a merged key cannot be validated")
		}
		if m.Content[i].Value != "paths" {
			continue
		}
		v := m.Content[i+1]
		if v.Tag == "!!null" {
			return nil, true, fail("paths: is present but null; give a non-empty list of glob strings or remove the key")
		}
		if v.Kind != yaml.SequenceNode {
			return nil, true, fail("paths: must be a list of glob strings")
		}
		if len(v.Content) == 0 {
			return nil, true, fail("paths: when present must be a non-empty list")
		}
		for _, e := range v.Content {
			if e.Kind != yaml.ScalarNode || e.Tag != "!!str" {
				return nil, true, fail("paths: every entry must be a string")
			}
			if e.Value == "" || strings.ContainsAny(e.Value, "\r\n") {
				return nil, true, fail("paths: every entry must be a non-empty string with no newlines")
			}
			paths = append(paths, e.Value)
		}
		return paths, true, nil
	}
	return nil, false, nil
}

// validateRulePaths enforces the targets-relationship, stem, and non-empty
// body rules for a rules-targeting fragment; paths content itself was
// already validated by parsePaths. Rejecting an empty body here (rather than
// in the renderer) keeps RenderRule's output formula simple: it can assume a
// non-empty body instead of special-casing the notice-only file that an
// empty body would otherwise produce.
func validateRulePaths(packName, rel string, tgts []string, hasPaths bool, body string) error {
	namesRules := false
	for _, t := range tgts {
		if t == targets.NameRules {
			namesRules = true
		}
	}
	if !namesRules {
		if hasPaths {
			return fmt.Errorf("%w: pack %q: rule %s: paths: is only allowed on a fragment whose targets include %q", esc.ErrManifest, packName, rel, targets.NameRules)
		}
		return nil
	}
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("%w: pack %q: rule %s: rules fragment has no body", esc.ErrManifest, packName, rel)
	}
	stem := strings.TrimSuffix(filepath.Base(rel), ".md")
	if !ruleStem.MatchString(stem) {
		return fmt.Errorf("%w: pack %q: rule %s: filename stem %q must match %s", esc.ErrManifest, packName, rel, stem, ruleStem)
	}
	return nil
}

// splitFrontmatter splits "---\n<yaml>\n---\n<body>". A closing fence at the
// very end of the input (no trailing newline after it) is also accepted,
// with the body normalized to empty: requiring "\n---\n" alone would treat a
// fragment ending immediately after its closing fence as having no
// frontmatter at all, silently skipping targets/paths parsing and dumping
// the raw frontmatter text into the body. Returns ok=false when the file has
// no frontmatter.
func splitFrontmatter(content string) (fm, body string, ok bool) {
	if !strings.HasPrefix(content, "---\n") {
		return "", "", false
	}
	rest := content[len("---\n"):]
	if idx := strings.Index(rest, "\n---\n"); idx >= 0 {
		return rest[:idx], rest[idx+len("\n---\n"):], true
	}
	if strings.HasSuffix(rest, "\n---") {
		return rest[:len(rest)-len("\n---")], "", true
	}
	return "", "", false
}
