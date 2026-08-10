package pack

import (
	"strings"
	"testing"
)

func servers(s map[string]map[string]any) MCPSpec { return MCPSpec{Servers: s} }

func TestLintMCPFlagsFloatingVersions(t *testing.T) {
	warns := LintMCP(servers(map[string]map[string]any{
		"b-latest": {"command": "npx", "args": []any{"-y", "@scope/server@latest"}},
		"a-nopin":  {"command": "npx", "args": []any{"-y", "@scope/server"}},
		"pinned":   {"command": "npx", "args": []any{"-y", "@scope/server@1.2.3"}},
		"binary":   {"command": "/usr/local/bin/my-server", "args": []any{"--port", "3000"}},
	}))
	if len(warns) != 2 {
		t.Fatalf("want 2 warnings, got %d: %v", len(warns), warns)
	}
	// Deterministic: sorted by server name.
	if !strings.Contains(warns[0], "a-nopin") || !strings.Contains(warns[1], "b-latest") {
		t.Fatalf("warnings unsorted or misattributed: %v", warns)
	}
	if !strings.Contains(warns[1], "@latest") {
		t.Fatalf("@latest warning must name the float: %v", warns[1])
	}
}

func TestLintMCPUvxAndEmpty(t *testing.T) {
	if w := LintMCP(servers(nil)); len(w) != 0 {
		t.Fatalf("no servers, no warnings: %v", w)
	}
	w := LintMCP(servers(map[string]map[string]any{
		"u": {"command": "uvx", "args": []any{"some-tool"}},
	}))
	if len(w) != 1 {
		t.Fatalf("uvx without a pin must warn: %v", w)
	}
}
