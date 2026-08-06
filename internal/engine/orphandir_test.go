package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/lockfile"
)

// The tests here drive Apply's orphaned-skill-dir pass directly, with a
// hand-built lockfile and an empty plan, because that is exactly the shape
// the pass reacts to: a dir artifact the lockfile records and the effective
// set no longer contains. Building the same situation through a real pack
// fetch costs two published pack versions per case and buries the one line
// under test; the CLI suite already covers the end-to-end path.

// writeUnder writes content at rel under dir, creating parents.
func writeUnder(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// orphanDirRepo lays out a repo at root holding one dir artifact at artPath,
// and writes a lockfile recording packFiles as the pack-provided manifest for
// it. extraFiles are written into the same directory but left out of the
// manifest, i.e. they are local amendments. Hash is irrelevant to this pass
// and is left as a placeholder.
func orphanDirRepo(t *testing.T, root, artPath string, packFiles, extraFiles map[string]string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(artPath))
	names := make([]string, 0, len(packFiles))
	for rel, content := range packFiles {
		writeUnder(t, dir, rel, content)
		names = append(names, rel)
	}
	for rel, content := range extraFiles {
		writeUnder(t, dir, rel, content)
	}
	lock := &lockfile.Lock{Schema: 1, Artifacts: []lockfile.LockArtifact{
		{Path: artPath, Kind: KindDir, Hash: "placeholder", Files: names},
	}}
	if err := lock.Save(root); err != nil {
		t.Fatal(err)
	}
}

// TestApplyOrphanDirPreservesLocalAmendments is the regression test for the
// data-loss defect this pass carried: when a pack dropped a skill directory,
// Apply called os.RemoveAll on the whole tree, deleting every file the team
// had added under it, with no skip, no report, and exit 0. That is precisely
// what mergeDir refuses to do on every ordinary sync, undone at the one
// moment nobody is watching. Only the paths the manifest recorded may be
// removed; the directory itself survives as long as anything unmanaged is
// still in it.
func TestApplyOrphanDirPreservesLocalAmendments(t *testing.T) {
	root := t.TempDir()
	const artPath = ".claude/skills/esc-acme-org-esc-security"
	orphanDirRepo(t, root, artPath,
		map[string]string{"SKILL.md": "pack content\n"},
		map[string]string{"team-notes.md": "our own notes\n", "runbooks/oncall.md": "ours too\n"})

	res, err := Apply(root, &PlanResult{}, false)
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(root, filepath.FromSlash(artPath))
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); !os.IsNotExist(err) {
		t.Errorf("a pack-provided file in a retired skill dir must be removed, stat err = %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "team-notes.md"))
	if err != nil {
		t.Fatalf("retiring a skill dir destroyed a local amendment: %v", err)
	}
	if string(got) != "our own notes\n" {
		t.Errorf("local amendment mutated: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "runbooks", "oncall.md")); err != nil {
		t.Errorf("nested local amendment destroyed: %v", err)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Errorf("a directory still holding unmanaged files must be left in place: %v", err)
	}

	// Reported, not merely left behind: without this the user has no way to
	// learn why a directory they expected to disappear is still there.
	if len(res.Skipped) != 1 || res.Skipped[0].Subject != artPath {
		t.Fatalf("want one Skipped entry for %s, got %+v", artPath, res.Skipped)
	}
	if r := res.Skipped[0].Reason; !strings.Contains(r, "team-notes.md") || !strings.Contains(r, "runbooks/oncall.md") {
		t.Errorf("skip reason must name what was preserved, got %q", r)
	}
}

// TestApplyOrphanDirRemovedWhenFullyManaged is the other half: the fix must
// not over-correct into never cleaning up. A retired skill dir holding
// nothing but pack-provided files still goes away entirely, silently, with
// no skip entry.
func TestApplyOrphanDirRemovedWhenFullyManaged(t *testing.T) {
	root := t.TempDir()
	const artPath = ".claude/skills/esc-acme-org-esc-security"
	orphanDirRepo(t, root, artPath,
		map[string]string{"SKILL.md": "pack content\n", "EXTRA.md": "more pack content\n"}, nil)

	res, err := Apply(root, &PlanResult{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(artPath))); !os.IsNotExist(err) {
		t.Errorf("a fully pack-owned retired skill dir must be removed, stat err = %v", err)
	}
	if len(res.Skipped) != 0 {
		t.Errorf("nothing was preserved, so nothing should be reported: %+v", res.Skipped)
	}
}

// TestApplyOrphanDirOwnershipGuardUsesRelativePath pins the ownership guard
// to the repo-relative artifact path. It used to test the ABSOLUTE path for
// "/esc-", which makes it vacuous for any repo that merely happens to live
// under a directory containing "esc-" — the whole guard evaporates and the
// removal reaches whatever in-repo path a lockfile entry names. The repo here
// sits under "esc-tools" for exactly that reason: with an absolute-path
// guard, this passes and deletes a directory escapement never owned.
func TestApplyOrphanDirOwnershipGuardUsesRelativePath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "esc-tools", "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	const artPath = ".claude/skills/team-owned"
	orphanDirRepo(t, root, artPath, map[string]string{"notes.md": "do not touch\n"}, nil)

	if _, err := Apply(root, &PlanResult{}, false); err == nil {
		t.Fatal("Apply must refuse a lockfile dir entry that is not escapement-owned, even under an esc- parent directory")
	}
	got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(artPath), "notes.md"))
	if err != nil {
		t.Fatalf("a directory escapement does not own was deleted: %v", err)
	}
	if string(got) != "do not touch\n" {
		t.Errorf("non-owned file mutated: %q", got)
	}
}

// TestApplyOrphanDirRefusesSymlinkedParent covers the second half of the same
// line's defect: unlike the sibling block-removal loop, this pass never
// called refuseSymlinks. containedPath is purely lexical, so it happily
// approves ".claude/skills/esc-x" while ".claude" is a symlink pointing
// anywhere at all, and the removal lands outside the repo entirely.
func TestApplyOrphanDirRefusesSymlinkedParent(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	const artPath = ".claude/skills/esc-acme-org-esc-security"
	// Laid out so the lexically contained removal path lands squarely on the
	// victim once the symlink is followed.
	writeUnder(t, filepath.Join(outside, "skills", "esc-acme-org-esc-security"), "SKILL.md", "do not touch\n")
	if err := os.Symlink(outside, filepath.Join(root, ".claude")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	lock := &lockfile.Lock{Schema: 1, Artifacts: []lockfile.LockArtifact{
		{Path: artPath, Kind: KindDir, Hash: "placeholder", Files: []string{"SKILL.md"}},
	}}
	if err := lock.Save(root); err != nil {
		t.Fatal(err)
	}

	if _, err := Apply(root, &PlanResult{}, false); err == nil {
		t.Fatal("Apply must fail closed on an orphaned dir reached through a symlinked parent, not exit 0")
	}
	got, err := os.ReadFile(filepath.Join(outside, "skills", "esc-acme-org-esc-security", "SKILL.md"))
	if err != nil {
		t.Fatalf("the orphan-dir removal followed a symlinked parent out of the repo: %v", err)
	}
	if string(got) != "do not touch\n" {
		t.Errorf("file outside the repo was modified: %q", got)
	}
}
