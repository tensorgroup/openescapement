package pack

import (
	"fmt"
	"sort"
	"strings"
)

// runnerCommands are launchers that fetch a package at invocation time, where
// an unpinned package spec means every consuming machine may run different
// code (spec §8: version discipline, not vendoring).
var runnerCommands = map[string]bool{"npx": true, "pnpx": true, "bunx": true, "uvx": true}

// LintMCP returns advisory warnings for MCP server definitions that float:
// an explicit @latest anywhere, or a runner (npx/uvx/...) whose package
// argument carries no version pin. Warnings, never errors — some servers are
// legitimately unversioned (a local binary, a checked-in script). Sorted by
// server name for deterministic output.
func LintMCP(m MCPSpec) []string {
	names := make([]string, 0, len(m.Servers))
	for name := range m.Servers {
		names = append(names, name)
	}
	sort.Strings(names)
	var warns []string
	for _, name := range names {
		def := m.Servers[name]
		cmd, _ := def["command"].(string)
		var args []string
		if raw, ok := def["args"].([]any); ok {
			for _, a := range raw {
				if s, ok := a.(string); ok {
					args = append(args, s)
				}
			}
		}
		all := append([]string{cmd}, args...)
		floated := ""
		for _, s := range all {
			if strings.Contains(s, "@latest") {
				floated = fmt.Sprintf("uses %q", s)
				break
			}
		}
		if floated == "" && runnerCommands[cmd] {
			pinned := false
			for _, a := range args {
				if strings.HasPrefix(a, "-") {
					continue
				}
				// "@scope/pkg@1.2.3" or "pkg@1.2.3": a version pin is an "@"
				// after the first character (a leading @ is a scope, not a pin).
				if strings.LastIndex(a, "@") > 0 {
					pinned = true
					break
				}
			}
			if !pinned {
				floated = fmt.Sprintf("runs %s without a version pin", cmd)
			}
		}
		if floated != "" {
			warns = append(warns, fmt.Sprintf("mcp server %q %s; consuming machines may run different code — pin a version", name, floated))
		}
	}
	return warns
}
