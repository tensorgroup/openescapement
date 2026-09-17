// Package render composes pack fragments into per-target policy text and
// manages the output mechanisms: managed blocks, whole files, and JSON merge.
package render

import (
	"fmt"
	"path"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/tensorgroup/openescapement/internal/pack"
	"github.com/tensorgroup/openescapement/internal/targets"
)

const (
	TargetClaude       = targets.NameClaude
	TargetAgents       = targets.NameAgents
	TargetGemini       = targets.NameGemini
	TargetGovernance   = targets.NameGovernance
	TargetSkills       = targets.NameSkills
	TargetMCP          = targets.NameMCP
	TargetRules        = targets.NameRules
	TargetAgentsSkills = targets.NameAgentsSkills
)

// TargetFile maps agent-file targets to the file they render into.
var TargetFile = map[string]string{
	TargetClaude:     "CLAUDE.md",
	TargetAgents:     "AGENTS.md",
	TargetGemini:     "GEMINI.md",
	TargetGovernance: "GOVERNANCE.md",
}

const notice = "> Managed by escapement. Do not edit. Run `esc diff` to see source. Team content goes outside this block."

var statusOrder = []string{"preferred", "allowed", "review-required", "banned"}

var statusLabel = map[string]string{
	"preferred":       "Preferred",
	"allowed":         "Allowed",
	"review-required": "Review required",
	"banned":          "Banned",
}

// fragmentApplies reports whether a fragment renders into target. Fragments
// without explicit targets apply everywhere.
func fragmentApplies(f pack.Fragment, target string) bool {
	if len(f.Targets) == 0 {
		return true
	}
	for _, t := range f.Targets {
		if t == target {
			return true
		}
	}
	return false
}

// Compose renders the managed-block body for one agent-file target from all
// packs in order. Output is deterministic.
func Compose(packs []*pack.Pack, target string) string {
	var b strings.Builder
	b.WriteString(notice)
	b.WriteString("\n")
	for _, p := range packs {
		for _, f := range p.Fragments {
			if !fragmentApplies(f, target) {
				continue
			}
			b.WriteString("\n")
			b.WriteString(strings.TrimSuffix(f.Body, "\n"))
			b.WriteString("\n")
		}
	}
	if cat := composeCatalog(packs); cat != "" {
		b.WriteString("\n")
		b.WriteString(cat)
	}
	return b.String()
}

// ComposeCustom renders the managed-block body for one pack-defined custom
// target. Only the owning pack's fragments that explicitly name the target are
// included; fragments with empty targets are never included. No catalog
// section is rendered (the catalog carve-out, §3). Output is deterministic.
func ComposeCustom(p *pack.Pack, target string) string {
	var b strings.Builder
	b.WriteString(notice)
	b.WriteString("\n")
	for _, f := range p.Fragments {
		if !fragmentNamesTarget(f, target) {
			continue
		}
		b.WriteString("\n")
		b.WriteString(strings.TrimSuffix(f.Body, "\n"))
		b.WriteString("\n")
	}
	return b.String()
}

// fragmentNamesTarget reports whether f explicitly names target. Unlike
// fragmentApplies, an empty target list never matches — custom targets require
// explicit opt-in.
func fragmentNamesTarget(f pack.Fragment, target string) bool {
	for _, t := range f.Targets {
		if t == target {
			return true
		}
	}
	return false
}

// FragmentNamesTarget reports whether f explicitly names target. An empty
// target list never matches: rule and custom targets require explicit opt-in.
func FragmentNamesTarget(f pack.Fragment, target string) bool {
	return fragmentNamesTarget(f, target)
}

// composeCatalog renders all packs' catalog entries as concise directive
// text, grouped by status in fixed severity order.
func composeCatalog(packs []*pack.Pack) string {
	var entries []pack.CatalogEntry
	for _, p := range packs {
		entries = append(entries, p.Manifest.Catalog...)
	}
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Tool & service policy\n")
	for _, status := range statusOrder {
		for _, e := range entries {
			if e.Status != status {
				continue
			}
			b.WriteString(fmt.Sprintf("- **%s:** %s (%s)", statusLabel[status], e.Name, e.Category))
			if e.Notes != "" {
				b.WriteString(" - " + e.Notes)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

// RuleFileStem is a rules fragment's filename without the .md suffix.
func RuleFileStem(f pack.Fragment) string {
	return strings.TrimSuffix(path.Base(f.Path), ".md")
}

// RuleFileName is the on-disk filename (no directory) a rules fragment
// renders to.
func RuleFileName(packName string, f pack.Fragment) string {
	return "esc-" + packName + "-" + RuleFileStem(f) + ".md"
}

// RuleFilePath is the repo-relative path a rules fragment renders to.
func RuleFilePath(packName string, f pack.Fragment) string {
	return path.Join(".claude", "rules", RuleFileName(packName, f))
}

// RenderRule renders one rules fragment to a whole .claude/rules file:
// optional paths frontmatter (only when path-scoped, so frontmatter sits at
// byte 0 as Claude Code's rules loader requires), a one-line ownership notice
// (deliberately without the pack version, so a version bump does not rewrite
// every rule file), a blank line, then the endings-normalized body run
// through strings.TrimSpace (which trims leading as well as trailing
// whitespace — a body whose first line is indented loses that indent, which
// is deliberate, not just a trailing trim) plus exactly one newline.
// Deterministic. The loader (internal/pack) rejects a rules fragment whose
// body is empty after trimming, so this formula never has to special-case a
// notice-only file with no body.
func RenderRule(f pack.Fragment, packName string) string {
	var b strings.Builder
	if len(f.Paths) > 0 {
		// yaml.Marshal cannot fail here: the value is a plain struct with a
		// single []string field, no cyclic references and no custom
		// MarshalYAML — the only error paths in yaml.v3's encoder.
		fm, _ := yaml.Marshal(struct {
			Paths []string `yaml:"paths"`
		}{f.Paths})
		b.WriteString("---\n")
		b.Write(fm)
		b.WriteString("---\n")
	}
	b.WriteString("<!-- Managed by escapement (pack " + packName + "). Do not edit. -->\n\n")
	b.WriteString(strings.TrimSpace(string(NormalizeEndings([]byte(f.Body)))))
	b.WriteString("\n")
	return b.String()
}

// PackLabels returns "name@version" for each pack, for block metadata.
func PackLabels(packs []*pack.Pack) []string {
	labels := make([]string, len(packs))
	for i, p := range packs {
		labels[i] = p.Manifest.Name + "@" + p.Manifest.Version
	}
	return labels
}
