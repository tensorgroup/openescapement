package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPackAddSkillRefusesSymlinkedSkillsParent is the residual fix's
// regression test for cmdPackAddSkill's own gap: skills/ itself is a
// symlink into a directory outside the pack repo, present before add-skill
// ever runs (e.g. a committed symlink, or one left behind by prior local
// tooling). Pre-fix, the pre-write "already exists" guard
// (os.Lstat(root/skills/<name>)) resolves through the symlinked parent and
// reports NotExist for a name the victim directory doesn't already
// contain, so the refusal never fires — and vendorCopy then happily
// os.MkdirAll/os.WriteFile's the fetched skill straight into the victim,
// outside root, at exit 0.
func TestPackAddSkillRefusesSymlinkedSkillsParent(t *testing.T) {
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	up := newUpstreamSkillRepo(t)
	root := newAuthorPack(t)

	victimParent := t.TempDir()
	skillsDir := filepath.Join(root, "skills")
	if err := os.Symlink(victimParent, skillsDir); err != nil {
		t.Fatal(err)
	}

	out, code := runEscOut(t, root, "pack", "add-skill", "file://"+up+"#skills")
	if code == 0 {
		t.Fatalf("a symlinked skills/ parent must be refused, not succeed:\n%s", out)
	}
	if code != 4 {
		t.Fatalf("containment refusal must exit 4 (plain error, not ErrConstraint), got %d:\n%s", code, out)
	}

	// Nothing may have been written into the victim: it must still be
	// empty, exactly as it started.
	entries, err := os.ReadDir(victimParent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("victim directory outside the pack repo gained content: %v", entries)
	}
	// The symlink itself must survive untouched.
	if got, err := os.Readlink(skillsDir); err != nil || got != victimParent {
		t.Fatalf("skills/ symlink was disturbed: readlink=%q err=%v", got, err)
	}
}
