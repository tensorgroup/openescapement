package targets

import "testing"

func TestBuiltIns(t *testing.T) {
	want := map[string]struct {
		file string
		kind string
	}{
		"claude":     {"CLAUDE.md", KindManagedBlock},
		"agents":     {"AGENTS.md", KindManagedBlock},
		"gemini":     {"GEMINI.md", KindManagedBlock},
		"governance": {"GOVERNANCE.md", KindWholeFile},
		"skills":     {"", KindSkillsDir},
		"mcp":        {"", KindMCPConfig},
	}
	got := BuiltIns()
	if len(got) != len(want) {
		t.Fatalf("BuiltIns count: got %d want %d", len(got), len(want))
	}
	for _, in := range got {
		w, ok := want[in.Name]
		if !ok {
			t.Errorf("unexpected built-in %q", in.Name)
			continue
		}
		if in.File != w.file || in.Kind != w.kind || !in.BuiltIn || in.OwnerPack != "" {
			t.Errorf("built-in %q wrong: %+v", in.Name, in)
		}
	}
	if !IsFragmentTarget("claude") || !IsFragmentTarget("governance") {
		t.Error("claude/governance must be fragment targets")
	}
	if IsFragmentTarget("skills") || IsFragmentTarget("mcp") || IsFragmentTarget("copilot") {
		t.Error("skills/mcp/custom must not be fragment targets")
	}
}

func TestValidateCustom(t *testing.T) {
	cases := []struct {
		desc string
		name string
		file string
		ok   bool
	}{
		{"valid single segment", "qwen", "QWEN.md", true},
		{"valid github carve-out", "copilot", ".github/copilot-instructions.md", true},
		{"valid dashed name", "my-target", "docs/policy.md", true},

		{"name uppercase", "Copilot", "x.md", false},
		{"name leading digit", "1copilot", "x.md", false},
		{"name empty", "", "x.md", false},
		{"name too long", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "x.md", false}, // 33 chars
		{"name underscore", "co_pilot", "x.md", false},
		{"name collides builtin", "claude", "x.md", false},

		{"file empty", "copilot", "", false},
		{"file traversal parent", "copilot", "../x.md", false},
		{"file traversal middle", "copilot", "a/../../x.md", false},
		{"file absolute", "copilot", "/etc/x.md", false},
		{"file backslash", "copilot", `a\b.md`, false},
		{"file non-md", "copilot", "foo.txt", false},
		{"file depth 3", "copilot", "a/b/c.md", false},
		{"file trailing dot segment", "copilot", "foo./bar.md", false},
		{"file trailing space segment", "copilot", "foo /bar.md", false},
		{"file leading slash empty seg", "copilot", "/x.md", false},
		{"file double slash", "copilot", "a//b.md", false},
		{"file non-ascii", "copilot", "café.md", false},
		{"denylist .claude", "copilot", ".claude/commands/x.md", false},
		{"denylist .git", "copilot", ".git/x.md", false},
		{"denylist .escapement", "copilot", ".escapement/x.md", false},
		{"denylist github workflows", "copilot", ".github/workflows/x.md", false},
		{"denylist .vscode", "copilot", ".vscode/x.md", false},
		{"denylist .idea", "copilot", ".idea/x.md", false},
		{"file collides builtin claude", "copilot", "claude.md", false},
		{"file collides builtin case", "copilot", "Claude.MD", false},
		{"file collides builtin agents", "copilot", "AGENTS.md", false},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			err := ValidateCustom(tc.name, tc.file)
			if tc.ok && err != nil {
				t.Errorf("ValidateCustom(%q,%q) = %v, want nil", tc.name, tc.file, err)
			}
			if !tc.ok && err == nil {
				t.Errorf("ValidateCustom(%q,%q) = nil, want error", tc.name, tc.file)
			}
		})
	}
}
