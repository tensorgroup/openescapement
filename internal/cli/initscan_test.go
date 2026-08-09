package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/config"
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

func TestDetectExistingFindsPlaceholder(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"AGENTS.md": "before\n" + render.Placeholder + "\nafter\n",
	})
	got, err := detectExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].HasPlaceholder || got[0].HasBlock {
		t.Errorf("got %+v, want one entry with HasPlaceholder only", got)
	}
}

// TestDetectExistingFindsBlock covers a real managed block, built with
// render.Splice so the fixture cannot drift from the actual block format.
func TestDetectExistingFindsBlock(t *testing.T) {
	root := t.TempDir()
	block, err := render.Splice(nil, "## Secrets\nUse Vault.\n", render.BlockMeta{Packs: []string{"acme-org@1.0.0"}})
	if err != nil {
		t.Fatal(err)
	}
	writeFiles(t, root, map[string]string{
		"CLAUDE.md": string(block),
	})
	got, err := detectExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].HasBlock || got[0].HasPlaceholder {
		t.Errorf("got %+v, want one entry with HasBlock only", got)
	}
}

// TestDetectExistingFindsCorruptBlock covers the "still counts as present"
// behavior: a begin marker with no matching end marker makes render.Extract
// return an error, and detectExisting must still report HasBlock so `esc
// init` warns about it instead of silently promising an insert that `esc
// sync` cannot honor either.
func TestDetectExistingFindsCorruptBlock(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"CLAUDE.md": "<!-- escapement:begin packs=acme-org@1.0.0 -->\nbody without an end marker\n",
	})
	got, err := detectExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].HasBlock {
		t.Errorf("got %+v, want one entry with HasBlock (corrupt block still counts as present)", got)
	}
}

// TestTargetOrderCoversAllTargets guards against a target being added to
// render.TargetFile (or a new non-file target constant) without updating
// targetOrder: without this, orderDetected and detectedTargets would
// silently drop it from both the generated targets: list and the
// explanation even though detectExisting still finds it.
func TestTargetOrderCoversAllTargets(t *testing.T) {
	want := map[string]bool{render.TargetMCP: true, render.TargetSkills: true}
	for target := range render.TargetFile {
		want[target] = true
	}
	seen := map[string]bool{}
	for _, target := range targetOrder {
		if seen[target] {
			t.Errorf("targetOrder has duplicate entry %q", target)
		}
		seen[target] = true
	}
	for target := range want {
		if !seen[target] {
			t.Errorf("targetOrder missing %q (present in render.TargetFile/TargetMCP/TargetSkills)", target)
		}
	}
	for target := range seen {
		if !want[target] {
			t.Errorf("targetOrder has unknown entry %q not in render.TargetFile/TargetMCP/TargetSkills", target)
		}
	}
}

// TestInitExplainsDetection pins the exact wording of esc init's explanation
// text (the amendment model's first-contact vocabulary, per the plan brief)
// across the three cases it must distinguish: nothing detected, clean files
// detected, and a file that already carries a block or a placeholder.
// Asserting full-string equality, not substrings, so the singular/plural
// boundary in detectedItemLabel, the one/two/three-item grammar in
// joinList, and the needsPlacementHint gating in explainDetection all stay
// pinned against wording drift.
func TestInitExplainsDetection(t *testing.T) {
	t.Run("nothing detected", func(t *testing.T) {
		root := t.TempDir()
		var out bytes.Buffer
		if code := Run(root, []string{"init"}, &out, &out); code != 0 {
			t.Fatalf("init: %d\n%s", code, out.String())
		}
		want := fmt.Sprintf("Initialized %s\nAdd pack sources to the config, then run `esc sync`.\n", config.Path(root))
		if got := out.String(); got != want {
			t.Errorf("output =\n%q\nwant\n%q", got, want)
		}
	})

	t.Run("clean files detected", func(t *testing.T) {
		root := t.TempDir()
		writeFiles(t, root, map[string]string{
			"CLAUDE.md": "line1\nline2\nline3\n",
			".mcp.json": `{"mcpServers":{}}`,
		})
		var out bytes.Buffer
		if code := Run(root, []string{"init"}, &out, &out); code != 0 {
			t.Fatalf("init: %d\n%s", code, out.String())
		}
		want := fmt.Sprintf(
			"Initialized %s\n\n"+
				"Found CLAUDE.md (3 lines) and .mcp.json.\n\n"+
				"`esc sync` will insert a managed block at the top of CLAUDE.md and add pack MCP servers alongside your existing ones. Your current content is preserved byte for byte and reported as a local amendment.\n\n"+
				"To place the block somewhere else, put %s where you want it before syncing.\n",
			config.Path(root), render.Placeholder)
		if got := out.String(); got != want {
			t.Errorf("output =\n%q\nwant\n%q", got, want)
		}
	})

	t.Run("detected file already has a block or placeholder", func(t *testing.T) {
		root := t.TempDir()
		gemini, err := render.Splice(nil, "body\n", render.BlockMeta{Packs: []string{"acme-org@1.0.0"}})
		if err != nil {
			t.Fatal(err)
		}
		geminiLines := bytes.Count(gemini, []byte("\n"))
		writeFiles(t, root, map[string]string{
			"AGENTS.md": "before\n" + render.Placeholder + "\nafter\n",
		})
		if err := os.WriteFile(filepath.Join(root, "GEMINI.md"), gemini, 0o644); err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if code := Run(root, []string{"init"}, &out, &out); code != 0 {
			t.Fatalf("init: %d\n%s", code, out.String())
		}
		// No placement-hint paragraph: neither file needs one, and promising
		// one would be a promise `esc sync` cannot honor.
		want := fmt.Sprintf(
			"Initialized %s\n\n"+
				"Found AGENTS.md (3 lines) and GEMINI.md (%d lines).\n\n"+
				"`esc sync` will insert a managed block at the placeholder already in AGENTS.md and update the managed block already in GEMINI.md. Your current content is preserved byte for byte and reported as a local amendment.\n",
			config.Path(root), geminiLines)
		if got := out.String(); got != want {
			t.Errorf("output =\n%q\nwant\n%q", got, want)
		}
	})
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
