// Package render composes pack fragments into per-target policy text and
// manages the output mechanisms: managed blocks, whole files, and JSON merge.
package render

import (
	"fmt"
	"strings"

	"github.com/tensorgroup/openescapement/internal/pack"
)

const (
	TargetClaude     = "claude"
	TargetAgents     = "agents"
	TargetGemini     = "gemini"
	TargetGovernance = "governance"
	TargetSkills     = "skills"
	TargetMCP        = "mcp"
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
				b.WriteString(" — " + e.Notes)
			}
			b.WriteString("\n")
		}
	}
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
