package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPackUpdateSkillRefusesTraversalName is the CRITICAL fix's regression
// test. sources.yaml is committed and travels inside the pack repo, exactly
// like a lockfile's Files entries — untrusted input that must be contained
// BEFORE anything reads it (AGENTS.md). Before the fix, SourceSkill.Name was
// never validated: a hand-edited (or maliciously PR'd) `name: ../../victim`
// entry made vendoredDir's fallback (filepath.Join(root, "skills", name))
// resolve OUTSIDE the pack repo, and `esc pack update-skill --all --force`
// would os.Rename the victim directory aside and replace it with fetched
// content — skipping the divergence gate entirely, since --force bypasses
// it, so no knowledge of the victim's hash is even needed.
//
// This test builds exactly that attack: a real, fetchable upstream skill
// (so the command gets past resolveSkillRef/fetchSkillSource and reaches
// the rename/delete), a sources.yaml entry named "../../victim" pointing at
// it, and a "victim" directory one level above the pack repo root — exactly
// where root/skills/../../victim resolves once filepath.Join cleans it.
func TestPackUpdateSkillRefusesTraversalName(t *testing.T) {
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	up := newUpstreamSkillRepo(t)
	root := newAuthorPack(t)

	// root/skills/../../victim cleans to filepath.Dir(root)/victim: exactly
	// one level above the pack repo, a sibling directory escapement never
	// owns.
	victim := filepath.Join(filepath.Dir(root), "victim")
	if err := os.MkdirAll(victim, 0o755); err != nil {
		t.Fatal(err)
	}
	const victimContent = "do not touch\n"
	if err := os.WriteFile(filepath.Join(victim, "original.txt"), []byte(victimContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// A hand-edited (or hostile-PR'd) sources.yaml: a valid, fetchable
	// source/subdir/ref so the command reaches the rename/delete, but a
	// traversal name. Commit/Hash are provenance metadata only — never used
	// to decide what gets fetched — so dummy values are fine, and --force
	// means the hash is never even consulted for the divergence gate.
	sourcesYAML := "schema: 1\nskills:\n" +
		"  - name: ../../victim\n" +
		"    source: file://" + up + "\n" +
		"    subdir: skills/brainstorming\n" +
		"    ref: v1.0.0\n" +
		"    commit: 0000000000000000000000000000000000000000\n" +
		"    hash: \"sha256:0000000000000000000000000000000000000000000000000000000000000000\"\n"
	writeFiles(t, root, map[string]string{"sources.yaml": sourcesYAML})

	out, code := runEscOut(t, root, "pack", "update-skill", "--all", "--force")
	if code == 0 {
		t.Fatalf("a traversal name in sources.yaml must be refused, not succeed:\n%s", out)
	}

	got, err := os.ReadFile(filepath.Join(victim, "original.txt"))
	if err != nil {
		t.Fatalf("victim directory was removed or replaced: %v", err)
	}
	if string(got) != victimContent {
		t.Fatalf("victim content changed: got %q, want %q", got, victimContent)
	}
	if fi, err := os.Stat(victim); err != nil || !fi.IsDir() {
		t.Fatalf("victim directory itself was replaced or removed: %v", err)
	}
}
