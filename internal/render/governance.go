package render

import (
	"fmt"
	"strings"

	"github.com/tensorgroup/openescapement/internal/pack"
)

// Governance renders the human-readable GOVERNANCE.md: provenance, per-pack
// rules prose (untargeted + governance-targeted fragments), the tool catalog
// as a table, and — in their own "Rule files" section per pack — every
// fragment that targets TargetRules, listed by rendered filename and, when
// path-scoped, its paths. A fragment that names both governance and rules
// targets appears in both sections; that duplication is deliberate, not a
// bug, since the two sections answer different questions (what governance
// prose applies vs. what file a rules fragment renders to).
func Governance(packs []*pack.Pack) string {
	var b strings.Builder
	b.WriteString("# Governance\n\n")
	b.WriteString("This document is rendered by [escapement](https://github.com/tensorgroup/openescapement) — do not edit by hand.\n\nActive policy packs:\n\n")
	for _, p := range packs {
		b.WriteString(fmt.Sprintf("- **%s** %s", p.Manifest.Name, p.Manifest.Version))
		if p.Manifest.Description != "" {
			b.WriteString(" — " + p.Manifest.Description)
		}
		b.WriteString("\n")
	}

	if table := catalogTable(packs); table != "" {
		b.WriteString("\n## Tool & service catalog\n\n")
		b.WriteString(table)
	}

	for _, p := range packs {
		var frags []pack.Fragment
		for _, f := range p.Fragments {
			if fragmentApplies(f, TargetGovernance) {
				frags = append(frags, f)
			}
		}
		if len(frags) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("\n## Policy: %s %s\n", p.Manifest.Name, p.Manifest.Version))
		for _, f := range frags {
			b.WriteString("\n")
			b.WriteString(strings.TrimSuffix(f.Body, "\n"))
			b.WriteString("\n")
		}
	}

	for _, p := range packs {
		var rules []pack.Fragment
		for _, f := range p.Fragments {
			if fragmentNamesTarget(f, TargetRules) {
				rules = append(rules, f)
			}
		}
		if len(rules) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("\n## Rule files: %s %s\n", p.Manifest.Name, p.Manifest.Version))
		for _, f := range rules {
			b.WriteString(fmt.Sprintf("\n**%s**", RuleFileName(p.Manifest.Name, f)))
			if len(f.Paths) > 0 {
				b.WriteString(" — paths: " + strings.Join(f.Paths, ", "))
			}
			b.WriteString("\n\n")
			b.WriteString(strings.TrimSuffix(f.Body, "\n"))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func catalogTable(packs []*pack.Pack) string {
	type row struct {
		e    pack.CatalogEntry
		from string
	}
	var rows []row
	for _, p := range packs {
		for _, e := range p.Manifest.Catalog {
			rows = append(rows, row{e, p.Manifest.Name})
		}
	}
	if len(rows) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("| Tool / service | Category | Status | Notes | Pack |\n")
	b.WriteString("|---|---|---|---|---|\n")
	for _, status := range statusOrder {
		for _, r := range rows {
			if r.e.Status != status {
				continue
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
				r.e.Name, r.e.Category, r.e.Status, r.e.Notes, r.from))
		}
	}
	return b.String()
}
