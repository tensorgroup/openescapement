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
	"github.com/tensorgroup/openescapement/internal/targets"
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

// TestDetectExistingSkipsSymlinkedAgentsAncestor: os.Lstat(".agents/skills")
// reports a real directory even when ".agents" itself (not "skills") is a
// symlink, since Lstat only refuses to follow the final path component. A
// pre-fill here would make the first `esc sync` refuse the ancestor at exit
// 4 (refuseSymlinks walks every component). The "skills" leaf is real; only
// the ".agents" ancestor is symlinked.
func TestDetectExistingSkipsSymlinkedAgentsAncestor(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real-agents-dir")
	if err := os.MkdirAll(filepath.Join(real, "skills", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "skills", "x", "SKILL.md"), []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(root, ".agents")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	got, err := detectExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range got {
		if d.Target == render.TargetAgentsSkills {
			t.Errorf("must not detect agents-skills through a symlinked .agents ancestor: %+v", got)
		}
	}
}

// TestDetectExistingSkipsSymlinkedClaudeAncestor is the .claude/skills parity
// gap: detectExisting used plain os.ReadDir with no symlink check at all, so
// it followed a symlinked ".claude" (or a symlinked "skills" leaf) straight
// through to a real directory beyond it.
func TestDetectExistingSkipsSymlinkedClaudeAncestor(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real-claude-dir")
	if err := os.MkdirAll(filepath.Join(real, "skills", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "skills", "x", "SKILL.md"), []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(root, ".claude")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	got, err := detectExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range got {
		if d.Target == render.TargetSkills {
			t.Errorf("must not detect skills through a symlinked .claude ancestor: %+v", got)
		}
	}
}

// TestTargetOrderCoversAllTargets pins targetOrder to exactly the built-in
// table order. It reads targets.BuiltIns() directly rather than through
// builtInTargetOrder() — the very expression that initializes targetOrder —
// so the assertion is not tautological: a stray or dropped entry in either
// targetOrder or builtInTargetOrder would fail it. It also checks that
// orderDetected preserves every target detectExisting can actually emit
// (rules is never detected from disk, so it is intentionally excluded from
// that half of the check).
func TestTargetOrderCoversAllTargets(t *testing.T) {
	ins := targets.BuiltIns()
	want := make([]string, len(ins))
	for i, in := range ins {
		want[i] = in.Name
	}
	if len(targetOrder) != len(want) {
		t.Fatalf("targetOrder length %d, want %d (%v vs %v)", len(targetOrder), len(want), targetOrder, want)
	}
	for i := range want {
		if targetOrder[i] != want[i] {
			t.Errorf("targetOrder[%d] = %q, want %q", i, targetOrder[i], want[i])
		}
	}

	emittable := []string{
		render.TargetClaude, render.TargetAgents, render.TargetGemini,
		render.TargetGovernance, render.TargetMCP, render.TargetSkills,
		render.TargetAgentsSkills,
	}
	detected := make([]Detected, len(emittable))
	for i, target := range emittable {
		detected[i] = Detected{Target: target}
	}
	ordered := orderDetected(detected)
	if len(ordered) != len(emittable) {
		t.Fatalf("orderDetected dropped entries: got %d want %d (%+v)", len(ordered), len(emittable), ordered)
	}
	seen := map[string]bool{}
	for _, d := range ordered {
		seen[d.Target] = true
	}
	for _, target := range emittable {
		if !seen[target] {
			t.Errorf("orderDetected dropped target %q emitted by detectExisting", target)
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
				"`esc sync` will:\n"+
				"  - insert a managed block at the top of CLAUDE.md\n"+
				"  - add pack MCP servers alongside your existing ones\n\n"+
				"Your current content is preserved byte for byte and reported as a local amendment.\n\n"+
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
				"`esc sync` will:\n"+
				"  - insert a managed block at the placeholder already in AGENTS.md\n"+
				"  - update the managed block already in GEMINI.md\n\n"+
				"Your current content is preserved byte for byte and reported as a local amendment.\n",
			config.Path(root), geminiLines)
		if got := out.String(); got != want {
			t.Errorf("output =\n%q\nwant\n%q", got, want)
		}
	})
}

// One detected item still reads as prose; the list form is only for two or
// more. This is the boundary the bulleted form introduced, so pin both sides
// of it (the two-item side is pinned by the cases above).
func TestInitExplainsSingleDetectionAsProse(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"CLAUDE.md": "line1\nline2\nline3\n"})
	var out bytes.Buffer
	if code := Run(root, []string{"init"}, &out, &out); code != 0 {
		t.Fatalf("init: %d\n%s", code, out.String())
	}
	want := fmt.Sprintf(
		"Initialized %s\n\n"+
			"Found CLAUDE.md (3 lines).\n\n"+
			"`esc sync` will insert a managed block at the top of CLAUDE.md. Your current content is preserved byte for byte and reported as a local amendment.\n\n"+
			"To place the block somewhere else, put %s where you want it before syncing.\n",
		config.Path(root), render.Placeholder)
	if got := out.String(); got != want {
		t.Errorf("output =\n%q\nwant\n%q", got, want)
	}
}

// The full-detection case, which no test previously reached: before the list
// form, all six targets produced one ~60 word sentence that said "insert a
// managed block at the top of" four consecutive times. Full-string equality,
// so the shape of the worst case stays pinned.
func TestInitExplainsAllSixDetectedTargets(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"CLAUDE.md":                      "line1\nline2\nline3\n",
		"AGENTS.md":                      "line1\nline2\nline3\n",
		"GEMINI.md":                      "line1\nline2\nline3\n",
		"GOVERNANCE.md":                  "line1\nline2\nline3\n",
		".mcp.json":                      `{"mcpServers":{}}`,
		".claude/skills/team-x/SKILL.md": "---\nname: team-x\n---\n",
	})
	var out bytes.Buffer
	if code := Run(root, []string{"init"}, &out, &out); code != 0 {
		t.Fatalf("init: %d\n%s", code, out.String())
	}
	want := fmt.Sprintf(
		"Initialized %s\n\n"+
			"Found CLAUDE.md (3 lines), AGENTS.md (3 lines), GEMINI.md (3 lines), GOVERNANCE.md (3 lines), .mcp.json and .claude/skills.\n\n"+
			"`esc sync` will:\n"+
			"  - insert a managed block at the top of CLAUDE.md\n"+
			"  - insert a managed block at the top of AGENTS.md\n"+
			"  - insert a managed block at the top of GEMINI.md\n"+
			"  - report GOVERNANCE.md as occupied and not overwrite it — move or delete it so escapement can own it\n"+
			"  - add pack MCP servers alongside your existing ones\n"+
			"  - add pack skills alongside yours in .claude/skills\n\n"+
			"Your current content is preserved byte for byte and reported as a local amendment.\n"+
			"A detected GOVERNANCE.md is a whole file escapement owns: it is reported occupied and left untouched until you move or delete it.\n\n"+
			"To place the block somewhere else, put %s where you want it before syncing.\n",
		config.Path(root), render.Placeholder)
	if got := out.String(); got != want {
		t.Errorf("output =\n%q\nwant\n%q", got, want)
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
