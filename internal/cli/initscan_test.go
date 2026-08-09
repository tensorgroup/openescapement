package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/render"
)

func TestDetectExisting(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"CLAUDE.md": "# Project\n\nour rules\nline three\n",
		".mcp.json": `{"mcpServers":{"ours":{"command":"a"}}}`,
	})
	if err := os.MkdirAll(filepath.Join(root, ".claude", "skills", "team-thing"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := detectExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]Detected{}
	for _, d := range got {
		byPath[d.Path] = d
	}

	claude, ok := byPath["CLAUDE.md"]
	if !ok {
		t.Fatal("CLAUDE.md not detected")
	}
	if claude.Target != "claude" || claude.Lines != 4 {
		t.Errorf("claude = %+v, want target=claude lines=4", claude)
	}
	if claude.HasBlock || claude.HasPlaceholder {
		t.Errorf("plain file must report no block and no placeholder: %+v", claude)
	}
	if _, ok := byPath[".mcp.json"]; !ok {
		t.Error(".mcp.json not detected")
	}
	if _, ok := byPath["AGENTS.md"]; ok {
		t.Error("absent file must not be detected")
	}
}

func TestDetectExistingEmptyRepo(t *testing.T) {
	got, err := detectExisting(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Error("must return an empty slice, never nil")
	}
	if len(got) != 0 {
		t.Errorf("got %+v, want none", got)
	}
}

func TestDetectExistingFindsBlockAndPlaceholder(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"AGENTS.md": "before\n" + render.Placeholder + "\nafter\n",
	})
	got, err := detectExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].HasPlaceholder || got[0].HasBlock {
		t.Errorf("got %+v, want one entry with HasPlaceholder", got)
	}
}

func TestConfigTemplateTargets(t *testing.T) {
	with := configTemplate([]string{"claude", "mcp"})
	if !strings.Contains(with, "targets:\n  - claude\n  - mcp\n") {
		t.Errorf("targets not rendered:\n%s", with)
	}
	without := configTemplate(nil)
	if strings.Contains(without, "targets:") {
		t.Errorf("empty targets must be omitted entirely:\n%s", without)
	}
	if !strings.Contains(without, "schema: 1") {
		t.Errorf("base template lost:\n%s", without)
	}
}
