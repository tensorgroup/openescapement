package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newCustomPackRepo builds a git pack repo whose pack.yaml declares the given
// custom targets and whose one fragment explicitly names fragTargets.
func newCustomPackRepo(t *testing.T, name, version, packYAMLBody, fragment string) string {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"pack.yaml":     packYAMLBody,
		"rules/main.md": fragment,
	})
	gitIn(t, dir, "init", "-q", "-b", "main")
	gitIn(t, dir, "config", "user.email", "t@e.com")
	gitIn(t, dir, "config", "user.name", "T")
	gitIn(t, dir, "config", "commit.gpgsign", "false")
	gitIn(t, dir, "config", "tag.gpgsign", "false")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "v"+version)
	gitIn(t, dir, "tag", "-a", "v"+version, "-m", "v"+version)
	return dir
}

const copilotPackYAML = `schema: 1
name: acme-org
version: 1.0.0
custom_targets:
  - name: copilot
    file: .github/copilot-instructions.md
    doc: https://docs.github.com/copilot
    description: Copilot instructions.
rules:
  - rules/main.md
`

const copilotFragment = "---\ntargets: [claude, copilot]\n---\n## Copilot rule\nUse Vault.\n"

// governedWith writes a governed repo pinning packRepo (root of the git repo,
// pack.yaml at repo root -> no //subdir), with the given extra config lines.
func governedWith(t *testing.T, packRepo, ref, extra string) string {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		".escapement/config.yaml": "schema: 1\npacks:\n  - source: file://" + packRepo + "\n    ref: " + ref + "\n    trust: unsigned\n" + extra,
		"CLAUDE.md":               "# Team notes\n",
	})
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	return dir
}

func TestCustomTargetAcknowledgmentGate(t *testing.T) {
	repo := newCustomPackRepo(t, "acme-org", "1.0.0", copilotPackYAML, copilotFragment)

	// Without acknowledgment: sync fails closed, exit 1, names pack + target + file.
	root := governedWith(t, repo, "v1.0.0", "")
	code, out := run(t, root, "sync")
	if code != 1 {
		t.Fatalf("unacknowledged sync exit %d, want 1:\n%s", code, out)
	}
	for _, want := range []string{
		"copilot", "acme-org", ".github/copilot-instructions.md", "allow_custom_target_files",
		"  - .github/copilot-instructions.md", // copy-pasteable YAML list line (§2.3)
	} {
		if !strings.Contains(out, want) {
			t.Errorf("error missing %q:\n%s", want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".github", "copilot-instructions.md")); !os.IsNotExist(err) {
		t.Error("file must not be written when unacknowledged (fail closed, not skip)")
	}

	// status reports the unacknowledged target.
	if code, out := run(t, root, "status"); code == 0 || !strings.Contains(out, "copilot") {
		t.Errorf("status should report unacknowledged target, exit=%d:\n%s", code, out)
	}

	// With acknowledgment: sync succeeds and lands the block beside seeded content.
	root2 := governedWith(t, repo, "v1.0.0", "allow_custom_target_files:\n  - .github/copilot-instructions.md\n")
	writeFiles(t, root2, map[string]string{".github/copilot-instructions.md": "# Existing user content\n"})
	if code, out := run(t, root2, "sync"); code != 0 {
		t.Fatalf("acknowledged sync exit %d:\n%s", code, out)
	}
	got, err := os.ReadFile(filepath.Join(root2, ".github", "copilot-instructions.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if !strings.Contains(s, "# Existing user content") {
		t.Error("seeded user content must be preserved")
	}
	if !strings.Contains(s, "## Copilot rule") || !strings.Contains(s, "escapement:begin") {
		t.Errorf("managed block missing from custom file:\n%s", s)
	}
	if strings.Contains(s, "Tool & service policy") {
		t.Error("custom file must not carry a catalog section")
	}
	// CLAUDE.md also got the copilot fragment (it names claude).
	claude, _ := os.ReadFile(filepath.Join(root2, "CLAUDE.md"))
	if !strings.Contains(string(claude), "## Copilot rule") {
		t.Error("claude target should include the fragment")
	}
}

func TestCustomTargetOwnershipIsolation(t *testing.T) {
	// A second pack whose fragment names copilot but does NOT declare it: fails
	// at load as a foreign reference (exit 1).
	foreignYAML := `schema: 1
name: team-pack
version: 1.0.0
rules:
  - rules/main.md
`
	repo := newCustomPackRepo(t, "team-pack", "1.0.0", foreignYAML, "---\ntargets: [copilot]\n---\nbody\n")
	root := governedWith(t, repo, "v1.0.0", "allow_custom_target_files:\n  - .github/copilot-instructions.md\n")
	if code, out := run(t, root, "sync"); code != 1 || !strings.Contains(out, "copilot") {
		t.Fatalf("foreign fragment reference should fail exit 1, got %d:\n%s", code, out)
	}
}

func TestCustomTargetEmptyFrontmatterNeverRenders(t *testing.T) {
	// Fragment with no targets: applies to built-ins only, never the custom file.
	repo := newCustomPackRepo(t, "acme-org", "1.0.0", copilotPackYAML, "## Plain rule\nno frontmatter\n")
	root := governedWith(t, repo, "v1.0.0", "allow_custom_target_files:\n  - .github/copilot-instructions.md\n")
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("sync exit %d:\n%s", code, out)
	}
	// The custom file is either absent or has an empty (notice-only) block; it
	// must never contain the fragment body.
	if b, err := os.ReadFile(filepath.Join(root, ".github", "copilot-instructions.md")); err == nil {
		if strings.Contains(string(b), "Plain rule") {
			t.Errorf("empty-frontmatter fragment leaked into custom target:\n%s", b)
		}
	}
	claude, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if !strings.Contains(string(claude), "Plain rule") {
		t.Error("empty-frontmatter fragment should reach built-in claude")
	}
}

func TestCustomTargetCollisions(t *testing.T) {
	// Two packs declaring the same custom file: collision, exit 1, names both.
	p1YAML := `schema: 1
name: org-pack
version: 1.0.0
custom_targets:
  - name: copilot
    file: SHARED.md
rules:
  - rules/main.md
`
	p2YAML := `schema: 1
name: team-pack
version: 1.0.0
custom_targets:
  - name: helper
    file: SHARED.md
rules:
  - rules/main.md
`
	r1 := newCustomPackRepo(t, "org-pack", "1.0.0", p1YAML, "---\ntargets: [copilot]\n---\nbody\n")
	r2 := newCustomPackRepo(t, "team-pack", "1.0.0", p2YAML, "---\ntargets: [helper]\n---\nbody\n")
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		".escapement/config.yaml": "schema: 1\npacks:\n" +
			"  - source: file://" + r1 + "\n    ref: v1.0.0\n    trust: unsigned\n" +
			"  - source: file://" + r2 + "\n    ref: v1.0.0\n    trust: unsigned\n" +
			"allow_custom_target_files:\n  - SHARED.md\n",
		"CLAUDE.md": "# t\n",
	})
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	code, out := run(t, dir, "sync")
	if code != 1 {
		t.Fatalf("file collision should exit 1, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "org-pack") || !strings.Contains(out, "team-pack") {
		t.Errorf("collision error should name both packs:\n%s", out)
	}
}

func TestCustomTargetInvalidDefinition(t *testing.T) {
	badYAML := `schema: 1
name: acme-org
version: 1.0.0
custom_targets:
  - name: copilot
    file: .claude/commands/x.md
rules:
  - rules/main.md
`
	repo := newCustomPackRepo(t, "acme-org", "1.0.0", badYAML, "---\ntargets: [copilot]\n---\nbody\n")
	root := governedWith(t, repo, "v1.0.0", "allow_custom_target_files:\n  - .claude/commands/x.md\n")
	if code, out := run(t, root, "sync"); code != 1 {
		t.Fatalf("invalid custom-target definition should exit 1, got %d:\n%s", code, out)
	}
}

func TestCustomTargetFilterInteraction(t *testing.T) {
	repo := newCustomPackRepo(t, "acme-org", "1.0.0", copilotPackYAML, copilotFragment)
	// targets filter excludes copilot: the custom file is not rendered.
	root := governedWith(t, repo, "v1.0.0",
		"targets: [claude]\nallow_custom_target_files:\n  - .github/copilot-instructions.md\n")
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("sync exit %d:\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(root, ".github", "copilot-instructions.md")); !os.IsNotExist(err) {
		t.Error("filtered-out custom target must not be rendered")
	}
}

// TestCustomTargetFilterExemptsAcknowledgment covers spec §3: filtering a
// custom target out by name is a complete opt-out. A repo that never
// acknowledges the file must still sync cleanly as long as the target is
// excluded by the targets filter — the acknowledgment gate must not even
// consider it.
func TestCustomTargetFilterExemptsAcknowledgment(t *testing.T) {
	repo := newCustomPackRepo(t, "acme-org", "1.0.0", copilotPackYAML, copilotFragment)
	// targets filter excludes copilot; no allow_custom_target_files entry at all.
	root := governedWith(t, repo, "v1.0.0", "targets: [claude]\n")
	code, out := run(t, root, "sync")
	if code != 0 {
		t.Fatalf("filtered-out custom target should need no acknowledgment, exit %d:\n%s", code, out)
	}
	if strings.Contains(out, "not acknowledged") {
		t.Errorf("filtered-out custom target must not raise an acknowledgment violation:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(root, ".github", "copilot-instructions.md")); !os.IsNotExist(err) {
		t.Error("filtered-out custom target must not be rendered")
	}
}

func TestCustomTargetDeterministic(t *testing.T) {
	repo := newCustomPackRepo(t, "acme-org", "1.0.0", copilotPackYAML, copilotFragment)
	root := governedWith(t, repo, "v1.0.0", "allow_custom_target_files:\n  - .github/copilot-instructions.md\n")
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("sync: %d\n%s", code, out)
	}
	first, _ := os.ReadFile(filepath.Join(root, ".github", "copilot-instructions.md"))
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("re-sync: %d\n%s", code, out)
	}
	second, _ := os.ReadFile(filepath.Join(root, ".github", "copilot-instructions.md"))
	if string(first) != string(second) {
		t.Error("custom target output is not deterministic across syncs")
	}
	if code, _ := run(t, root, "status"); code != 0 {
		t.Error("status should be clean after sync")
	}
}

func TestCustomTargetSymlinkRefusal(t *testing.T) {
	repo := newCustomPackRepo(t, "acme-org", "1.0.0", copilotPackYAML, copilotFragment)
	root := governedWith(t, repo, "v1.0.0", "allow_custom_target_files:\n  - .github/copilot-instructions.md\n")
	// Make .github a symlink to an out-of-repo directory.
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".github")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	code, out := run(t, root, "sync")
	if code != 1 {
		t.Fatalf("writing through a symlinked parent should fail exit 1, got %d:\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(outside, "copilot-instructions.md")); !os.IsNotExist(err) {
		t.Error("must not have written through the symlink")
	}
}

func TestOrphanBlockRemoval(t *testing.T) {
	// v1 defines copilot; sync writes the block into a file with user content.
	repo := newCustomPackRepo(t, "acme-org", "1.0.0", copilotPackYAML, copilotFragment)
	root := governedWith(t, repo, "v1.0.0", "allow_custom_target_files:\n  - .github/copilot-instructions.md\n")
	writeFiles(t, root, map[string]string{".github/copilot-instructions.md": "# Existing user content\n"})
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("initial sync: %d\n%s", code, out)
	}

	// Un-acknowledge the target (drop it from the effective set) by rewriting
	// config with no allow list, and remove the pack's custom target so it is
	// no longer even declared. Simplest: point config at a v2 pack with no
	// custom_targets. Here we just drop acknowledgment AND the filter so the
	// block is orphaned. Repin to a pack version that no longer defines it.
	repo2 := newCustomPackRepo(t, "acme-org", "2.0.0",
		"schema: 1\nname: acme-org\nversion: 2.0.0\nrules:\n  - rules/main.md\n",
		"## Claude rule\nbody\n")
	writeFiles(t, root, map[string]string{
		".escapement/config.yaml": "schema: 1\npacks:\n  - source: file://" + repo2 + "\n    ref: v2.0.0\n    trust: unsigned\n",
	})
	_ = repo // v1 repo no longer referenced

	// Plain status reports the orphan but exits 0 — orphans are self-healing
	// drift (the next sync removes them), not a fail-closed violation.
	if code, out := run(t, root, "status"); code != 0 || !strings.Contains(strings.ToLower(out), "orphan") {
		t.Errorf("status should report orphan and exit 0, exit=%d:\n%s", code, out)
	}
	// status --check exits 1 on any drift, including orphans.
	if code, out := run(t, root, "status", "--check"); code != 1 {
		t.Errorf("status --check should exit 1 on orphan drift, exit=%d:\n%s", code, out)
	}

	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("re-sync: %d\n%s", code, out)
	}
	got, err := os.ReadFile(filepath.Join(root, ".github", "copilot-instructions.md"))
	if err != nil {
		t.Fatalf("file should still exist (had user content): %v", err)
	}
	if strings.Contains(string(got), "escapement:begin") {
		t.Errorf("orphaned managed block should be removed:\n%s", got)
	}
	if !strings.Contains(string(got), "# Existing user content") {
		t.Errorf("user content must be preserved:\n%s", got)
	}
}

func TestOrphanBlockByteEmptyDeletion(t *testing.T) {
	// Same as above but the custom file had NO user content, so removing the
	// block leaves it byte-empty and the file is deleted.
	repo := newCustomPackRepo(t, "acme-org", "1.0.0", copilotPackYAML, copilotFragment)
	root := governedWith(t, repo, "v1.0.0", "allow_custom_target_files:\n  - .github/copilot-instructions.md\n")
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("initial sync: %d\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(root, ".github", "copilot-instructions.md")); err != nil {
		t.Fatalf("expected custom file after first sync: %v", err)
	}
	repo2 := newCustomPackRepo(t, "acme-org", "2.0.0",
		"schema: 1\nname: acme-org\nversion: 2.0.0\nrules:\n  - rules/main.md\n",
		"## Claude rule\nbody\n")
	writeFiles(t, root, map[string]string{
		".escapement/config.yaml": "schema: 1\npacks:\n  - source: file://" + repo2 + "\n    ref: v2.0.0\n    trust: unsigned\n",
	})
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("re-sync: %d\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(root, ".github", "copilot-instructions.md")); !os.IsNotExist(err) {
		t.Error("byte-empty orphaned custom file should be deleted")
	}
}

// TestCustomTargetFilterCaseInsensitive covers the config targets: filter
// naming a declared custom target in non-canonical case. Custom target names
// are validated lowercase-only, and the filter-selection step already
// case-folds for the allDeclared check, so a non-canonical filter entry must
// render the target rather than silently drop it (built-ins, by contrast,
// are matched by exact case and hard-reject an unknown-case name; the fix
// here makes the final dispatch loop consistent with the earlier,
// intentionally case-insensitive selection step instead of silently no-oping
// between the two).
func TestCustomTargetFilterCaseInsensitive(t *testing.T) {
	repo := newCustomPackRepo(t, "acme-org", "1.0.0", copilotPackYAML, copilotFragment)
	root := governedWith(t, repo, "v1.0.0",
		"targets: [COPILOT]\nallow_custom_target_files:\n  - .github/copilot-instructions.md\n")
	code, out := run(t, root, "sync")
	if code != 0 {
		t.Fatalf("sync exit %d:\n%s", code, out)
	}
	got, err := os.ReadFile(filepath.Join(root, ".github", "copilot-instructions.md"))
	if err != nil {
		t.Fatalf("custom target must render despite non-canonical filter case: %v", err)
	}
	if !strings.Contains(string(got), "## Copilot rule") {
		t.Errorf("custom target content missing:\n%s", got)
	}
}
