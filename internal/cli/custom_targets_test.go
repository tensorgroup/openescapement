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
	// Exit 4, not 1. A symlink refusal is a containment failure, not drift:
	// exit 1 is the class a CI gate reads as routine and self-healing, and no
	// amount of syncing clears a symlink planted where escapement writes. This
	// asserted exit 1 only because refuseSymlinks wrapped esc.ErrConstraint;
	// it now matches its containment siblings in mergeDir. See the exit-class
	// note on refuseSymlinks (internal/engine/apply.go) for why not exit 3.
	code, out := run(t, root, "sync")
	if code != 4 {
		t.Fatalf("writing through a symlinked parent should fail exit 4, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "refusing to write through symlink") {
		t.Errorf("refusal must name the symlink:\n%s", out)
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

// TestCustomTargetDiffAgainst covers spec §3: PolicyDiff must diff the
// effective custom-target set, not just the four built-ins. A content change
// to a custom target's owning fragment between the current pin and an
// alternate ref must appear in `esc diff --against`.
func TestCustomTargetDiffAgainst(t *testing.T) {
	// Fragment names copilot only (not claude), so a content change is
	// visible exclusively through the custom target's diff — isolating the
	// assertion from the built-in claude diff path.
	v1Fragment := "---\ntargets: [copilot]\n---\n## Copilot rule\nUse Vault.\n"
	repo := newCustomPackRepo(t, "acme-org", "1.0.0", copilotPackYAML, v1Fragment)
	root := governedWith(t, repo, "v1.0.0", "allow_custom_target_files:\n  - .github/copilot-instructions.md\n")
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("sync: %d\n%s", code, out)
	}

	// Publish v2.0.0 with changed custom-target content.
	v2YAML := strings.Replace(copilotPackYAML, "version: 1.0.0", "version: 2.0.0", 1)
	v2Fragment := "---\ntargets: [copilot]\n---\n## Copilot rule\nUse LastPass.\n"
	writeFiles(t, repo, map[string]string{"pack.yaml": v2YAML, "rules/main.md": v2Fragment})
	gitIn(t, repo, "add", "-A")
	gitIn(t, repo, "commit", "-q", "-m", "v2")
	gitIn(t, repo, "tag", "-a", "v2.0.0", "-m", "v2")

	code, out := run(t, root, "diff", "--against", "v2.0.0")
	if code != 1 {
		t.Fatalf("diff --against: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, ".github/copilot-instructions.md") {
		t.Errorf("custom target file missing from diff:\n%s", out)
	}
	if !strings.Contains(out, "Vault") || !strings.Contains(out, "LastPass") {
		t.Errorf("custom target content change missing from diff:\n%s", out)
	}
}

// TestCustomTargetDiffAgainstFilteredOut covers spec §3: a custom target
// excluded by the repo's targets: filter must be absent from both sides of
// the diff, even though its content changed between refs — exactly as Plan
// treats it (never rendered, never considered).
func TestCustomTargetDiffAgainstFilteredOut(t *testing.T) {
	// Fragment names copilot only, and the repo's targets: filter excludes
	// it, so claude never sees this content either — the whole diff must be
	// empty even though the custom target's content changed between refs.
	v1Fragment := "---\ntargets: [copilot]\n---\n## Copilot rule\nUse Vault.\n"
	repo := newCustomPackRepo(t, "acme-org", "1.0.0", copilotPackYAML, v1Fragment)
	root := governedWith(t, repo, "v1.0.0", "targets: [claude]\n")
	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("sync: %d\n%s", code, out)
	}

	v2YAML := strings.Replace(copilotPackYAML, "version: 1.0.0", "version: 2.0.0", 1)
	v2Fragment := "---\ntargets: [copilot]\n---\n## Copilot rule\nUse LastPass.\n"
	writeFiles(t, repo, map[string]string{"pack.yaml": v2YAML, "rules/main.md": v2Fragment})
	gitIn(t, repo, "add", "-A")
	gitIn(t, repo, "commit", "-q", "-m", "v2")
	gitIn(t, repo, "tag", "-a", "v2.0.0", "-m", "v2")

	code, out := run(t, root, "diff", "--against", "v2.0.0")
	if code != 0 {
		t.Errorf("filtered-out custom target: diff should be empty, exit %d:\n%s", code, out)
	}
	if strings.Contains(out, ".github/copilot-instructions.md") {
		t.Errorf("filtered-out custom target must not appear in diff:\n%s", out)
	}
	if strings.Contains(out, "LastPass") || strings.Contains(out, "Vault") {
		t.Errorf("filtered-out custom target content must not appear in diff:\n%s", out)
	}
}

// TestOrphanBlockHandEditIsNotSilentlyDeleted covers the skip gate on the
// orphan-block removal path. The main sync path declines to overwrite a
// hand-edited managed region; deleting that same region outright is strictly
// more destructive, so it cannot be the one write that bypasses the gate.
// Before the fix, a hand-edit inside a block whose target had left the
// effective set was deleted with no warning, no skip entry, and exit 0 — and
// status could not warn first either, since orphan findings hard-coded
// Local: LocalNone and never compared the block against the lockfile.
//
// Bytes outside the block must still be preserved exactly, in every branch.
func TestOrphanBlockHandEditIsNotSilentlyDeleted(t *testing.T) {
	repo := newCustomPackRepo(t, "acme-org", "1.0.0", copilotPackYAML, copilotFragment)
	root := governedWith(t, repo, "v1.0.0", "allow_custom_target_files:\n  - .github/copilot-instructions.md\n")
	writeFiles(t, root, map[string]string{".github/copilot-instructions.md": "# Existing user content\n"})
	runEsc(t, root, "sync")

	// Hand-edit inside the managed block.
	p := filepath.Join(root, ".github", "copilot-instructions.md")
	content, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	edited := replaceOnce(t, string(content), "Use Vault.", "Use whatever, we edited this.")
	if err := os.WriteFile(p, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	// Orphan the target: repin to a pack version that does not declare it.
	repo2 := newCustomPackRepo(t, "acme-org", "2.0.0",
		"schema: 1\nname: acme-org\nversion: 2.0.0\nrules:\n  - rules/main.md\n",
		"## Claude rule\nbody\n")
	writeFiles(t, root, map[string]string{
		".escapement/config.yaml": "schema: 1\npacks:\n  - source: file://" + repo2 + "\n    ref: v2.0.0\n    trust: unsigned\n",
	})

	// Status must say so before sync is ever asked to delete it.
	out, code := runEscOut(t, root, "status")
	if code != 0 {
		t.Fatalf("status on an orphan should exit 0, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "hand-edited") {
		t.Errorf("status must report the orphaned block's hand-edit, got:\n%s", out)
	}

	if code, out := run(t, root, "sync"); code != 0 {
		t.Fatalf("a declined orphan must not fail the rollout, exit=%d:\n%s", code, out)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "Use whatever, we edited this.") {
		t.Errorf("sync silently deleted a hand-edited orphaned block:\n%s", got)
	}
	if !strings.Contains(string(got), "# Existing user content") {
		t.Errorf("bytes outside the block must be preserved:\n%s", got)
	}

	// The orphan must still be reported after the skip: dropping the lock
	// entry would leave an undeleted managed block in a repo status calls clean.
	if out, _ := runEscOut(t, root, "status"); !strings.Contains(strings.ToLower(out), "orphan") {
		t.Errorf("a skipped orphan must still be reported on the next status:\n%s", out)
	}

	// --force is the documented way through, and still preserves the outside.
	if code, out := run(t, root, "sync", "--force"); code != 0 {
		t.Fatalf("sync --force: %d\n%s", code, out)
	}
	got, err = os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "escapement:begin") {
		t.Errorf("sync --force must remove the orphaned block:\n%s", got)
	}
	if !strings.Contains(string(got), "# Existing user content") {
		t.Errorf("--force must still preserve bytes outside the block:\n%s", got)
	}
}
