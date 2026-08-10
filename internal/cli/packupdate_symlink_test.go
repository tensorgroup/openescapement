package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPackUpdateSkillRefusesSymlinkAtSkillDir and
// TestPackUpdateSkillRefusesSymlinkAtSkillsParent are the residual fix's
// regression tests. containedSkillDir (skillvendor.go) validates a skill
// name and checks lexical containment, but that alone is not containment: a
// lexically-contained path is still reached by following whatever symlink
// sits at any of its path components, exactly as any other path-based
// syscall would. A pack repo that carries a committed symlink at
// skills/<name> — or at skills/ itself — must not let `esc pack
// update-skill --force`'s os.Rename/os.RemoveAll act through it, outside
// the pack repo.

// victimDirWithFile builds a directory OUTSIDE root holding one file, so a
// test can assert byte-identical survival after the attempted attack.
func victimDirWithFile(t *testing.T, name, content string) string {
	t.Helper()
	victim := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(victim, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(victim, "original.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return victim
}

func assertVictimIntact(t *testing.T, victim, content string) {
	t.Helper()
	fi, err := os.Stat(victim)
	if err != nil || !fi.IsDir() {
		t.Fatalf("victim directory itself was replaced or removed: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(victim, "original.txt"))
	if err != nil {
		t.Fatalf("victim file was removed: %v", err)
	}
	if string(got) != content {
		t.Fatalf("victim content changed: got %q, want %q", got, content)
	}
}

// TestPackUpdateSkillRefusesSymlinkAtSkillDir: skills/brainstorming itself
// is a symlink into a victim directory living outside the pack repo.
//
// Before the fix this already exits non-zero and leaves the victim
// untouched, but only by accident: pack.DirHash's underlying
// filepath.WalkDir Lstats its root argument, sees the symlink, and
// pack.DirFiles refuses it as "symlinks are not allowed in packs" — a rule
// about pack CONTENT, not a containment check, and one that a future
// reordering of cmdPackUpdateSkill (e.g. moving the force-skip hash check
// before the read) could silently stop relying on. The message assertion
// below is what actually pins this down as a deliberate containment
// refusal rather than a coincidence: it fails against the pre-fix code,
// which reports the DirFiles wording instead.
func TestPackUpdateSkillRefusesSymlinkAtSkillDir(t *testing.T) {
	root, up := vendoredAuthorPack(t)
	publishSkillV2(t, up)

	const victimContent = "do not touch (leaf)\n"
	victim := victimDirWithFile(t, "victim-leaf", victimContent)

	skillDir := filepath.Join(root, "skills", "brainstorming")
	if err := os.RemoveAll(skillDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, skillDir); err != nil {
		t.Fatal(err)
	}

	out, code := runEscOut(t, root, "pack", "update-skill", "brainstorming", "--force")
	if code == 0 {
		t.Fatalf("a symlinked skill directory must be refused, not succeed:\n%s", out)
	}
	if code != 4 {
		t.Fatalf("containment refusal must exit 4 (plain error, not ErrConstraint), got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "refusing to write through symlink") {
		t.Fatalf("expected an explicit symlink-containment refusal, got:\n%s", out)
	}
	assertVictimIntact(t, victim, victimContent)
	// The symlink itself must survive untouched too: refusal happens before
	// any rename, so skills/brainstorming is still the same symlink.
	if got, err := os.Readlink(skillDir); err != nil || got != victim {
		t.Fatalf("skills/brainstorming symlink was disturbed: readlink=%q err=%v", got, err)
	}
}

// TestPackUpdateSkillRefusesSymlinkAtSkillsParent: skills/ itself is a
// symlink into a directory outside the pack repo that happens to contain a
// "brainstorming" subdirectory — the case where the leaf path component
// (skills/brainstorming) resolves, via the OS transparently following the
// symlinked parent, to a real directory rather than a symlink, so a check
// that only Lstats the leaf never sees anything wrong.
func TestPackUpdateSkillRefusesSymlinkAtSkillsParent(t *testing.T) {
	root, up := vendoredAuthorPack(t)
	publishSkillV2(t, up)

	const victimContent = "do not touch (parent)\n"
	victimParent := t.TempDir()
	victimSkill := filepath.Join(victimParent, "brainstorming")
	if err := os.MkdirAll(victimSkill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(victimSkill, "original.txt"), []byte(victimContent), 0o644); err != nil {
		t.Fatal(err)
	}
	// The manifest (pack.yaml) also declares "writing-plans" (vendoredAuthorPack
	// vendors the whole two-skill upstream suite). pack.Load's manifest
	// validation Stats every declared skill path, and once skills/ is a
	// symlink that Stat resolves through it too — so a placeholder directory
	// must exist at victimParent/writing-plans or the command fails at
	// manifest load, before ever reaching the code this test targets.
	if err := os.MkdirAll(filepath.Join(victimParent, "writing-plans"), 0o755); err != nil {
		t.Fatal(err)
	}

	skillsDir := filepath.Join(root, "skills")
	if err := os.RemoveAll(skillsDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victimParent, skillsDir); err != nil {
		t.Fatal(err)
	}

	out, code := runEscOut(t, root, "pack", "update-skill", "brainstorming", "--force")
	if code == 0 {
		t.Fatalf("a symlinked skills/ parent must be refused, not succeed:\n%s", out)
	}
	if code != 4 {
		t.Fatalf("containment refusal must exit 4 (plain error, not ErrConstraint), got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "refusing to write through symlink") {
		t.Fatalf("expected an explicit symlink-containment refusal, got:\n%s", out)
	}
	assertVictimIntact(t, victimSkill, victimContent)
	if got, err := os.Readlink(skillsDir); err != nil || got != victimParent {
		t.Fatalf("skills/ symlink was disturbed: readlink=%q err=%v", got, err)
	}
}
