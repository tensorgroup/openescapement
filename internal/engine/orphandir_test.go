package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/lockfile"
	"github.com/tensorgroup/openescapement/internal/pack"
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
// manifest, i.e. they are local amendments.
//
// The recorded Hash is the real DirHashOf over the manifest, exactly what a
// prior sync would have written. It used to be a placeholder, on the reasoning
// that the hash was irrelevant to this pass; the pass now compares against it
// to detect a hand-edited pack-provided file, so a placeholder would make
// every test here take the skip branch.
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
	hash, err := pack.DirHashOf(dir, names)
	if err != nil {
		t.Fatal(err)
	}
	lock := &lockfile.Lock{Schema: 1, Artifacts: []lockfile.LockArtifact{
		{Path: artPath, Kind: KindDir, Hash: hash, Files: names},
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
// (ownedSkillPath) to the repo-relative artifact path. It used to test the
// ABSOLUTE path for "/esc-", which makes it vacuous for any repo that merely
// happens to live under a directory containing "esc-" — the whole guard
// evaporates and the removal reaches whatever in-repo path a lockfile entry
// names. The repo here sits under "esc-tools" for exactly that reason: with
// an absolute-path guard, this passes and deletes a directory escapement
// never owned.
//
// The artifact path is nested two levels below .claude/skills. One level
// below is always ownable now (Task 3 replaced the "esc-" name-prefix
// marker with "exactly one path element below .claude/skills", so a
// name-overridden skill with no prefix can still retire); a dir nested
// deeper than that — a team directory sitting inside what looks like an
// owned skill directory — never is, which is the case this test drives.
func TestApplyOrphanDirOwnershipGuardUsesRelativePath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "esc-tools", "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	const artPath = ".claude/skills/esc-acme-org/team-owned"
	orphanDirRepo(t, root, artPath, map[string]string{"notes.md": "do not touch\n"}, nil)

	if _, err := Apply(root, &PlanResult{}, false); err == nil {
		t.Fatal("Apply must refuse a lockfile dir entry that is not escapement-owned, even nested inside an owned skill directory")
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

// TestApplyOrphanDirPreservesEditedPackFile is the last hole in "we never
// destroy your work". The pass above learned to keep team-ADDED files when a
// pack retires a skill directory, but a pack-provided file the team EDITED
// was still removed with no warning, at exit 0, and status says nothing
// about a retired directory beforehand. The lockfile's dir hash covers
// exactly Files, so a mismatch against it is proof that a pack-provided file
// changed since escapement wrote it, and the whole retirement is declined.
func TestApplyOrphanDirPreservesEditedPackFile(t *testing.T) {
	root := t.TempDir()
	const artPath = ".claude/skills/esc-acme-org-esc-security"
	orphanDirRepo(t, root, artPath,
		map[string]string{"SKILL.md": "pack content\n", "EXTRA.md": "more pack content\n"}, nil)
	// The team edits a pack-provided file after that sync.
	writeUnder(t, filepath.Join(root, filepath.FromSlash(artPath)), "SKILL.md", "pack content\nour own addition\n")

	res, err := Apply(root, &PlanResult{}, false)
	if err != nil {
		t.Fatalf("a declined retirement must not fail the rollout: %v", err)
	}

	dir := filepath.Join(root, filepath.FromSlash(artPath))
	got, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		t.Fatalf("retiring a skill dir destroyed a hand-edited pack file: %v", err)
	}
	if string(got) != "pack content\nour own addition\n" {
		t.Errorf("hand-edited pack file mutated: %q", got)
	}
	// Nothing is removed, not just the edited file: the removal loop is
	// declined wholesale, so the files the edit sits beside survive too.
	if _, err := os.Stat(filepath.Join(dir, "EXTRA.md")); err != nil {
		t.Errorf("declining the retirement must leave the whole directory alone: %v", err)
	}

	if len(res.Skipped) != 1 || res.Skipped[0].Subject != artPath {
		t.Fatalf("want one Skipped entry for %s, got %+v", artPath, res.Skipped)
	}
	if r := res.Skipped[0].Reason; !strings.Contains(r, "was edited since the last sync") {
		t.Errorf("skip reason must say a pack-provided file was edited, got %q", r)
	}
	if res.Skipped[0].ExpectedHash == "" || res.Skipped[0].ActualHash == "" ||
		res.Skipped[0].ExpectedHash == res.Skipped[0].ActualHash {
		t.Errorf("skip must carry both hashes and they must differ: %+v", res.Skipped[0])
	}

	// The lock entry is carried forward unchanged. Dropping it would lose the
	// manifest a later --force needs and stop the condition from being
	// reported ever again, leaving an undeleted owned directory nothing
	// mentions.
	lock, err := lockfile.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	prev := lock.Artifact(artPath)
	if prev == nil {
		t.Fatal("lock entry for a declined retirement must be carried forward")
	}
	if len(prev.Files) != 2 {
		t.Errorf("carried-forward manifest must be unchanged, got %v", prev.Files)
	}

	// Convergence, direction 1: the condition keeps reporting rather than
	// silently vanishing, and the second run is identical to the first.
	res2, err := Apply(root, &PlanResult{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res2.Skipped) != 1 || res2.Skipped[0] != res.Skipped[0] {
		t.Errorf("a declined retirement must re-report identically on the next sync, got %+v", res2.Skipped)
	}
}

// TestApplyOrphanDirForceRemovesEditedPackFile is the convergence exit. A
// declined retirement is reported on every sync, so there must be a way out
// of it that is not "edit files until esc agrees": --force is consent to
// overwrite escapement's own content, and an edited pack-provided file is
// escapement's content. Team-added files are still not force's to delete.
func TestApplyOrphanDirForceRemovesEditedPackFile(t *testing.T) {
	root := t.TempDir()
	const artPath = ".claude/skills/esc-acme-org-esc-security"
	orphanDirRepo(t, root, artPath,
		map[string]string{"SKILL.md": "pack content\n"},
		map[string]string{"team-notes.md": "our own notes\n"})
	writeUnder(t, filepath.Join(root, filepath.FromSlash(artPath)), "SKILL.md", "pack content\nedited\n")

	res, err := Apply(root, &PlanResult{}, true)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, filepath.FromSlash(artPath))
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); !os.IsNotExist(err) {
		t.Errorf("--force must remove an edited pack-provided file, stat err = %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "team-notes.md")); err != nil {
		t.Errorf("--force is about our content, not theirs: %v", err)
	} else if string(got) != "our own notes\n" {
		t.Errorf("team file mutated by --force: %q", got)
	}
	if len(res.Skipped) != 1 || !strings.Contains(res.Skipped[0].Reason, "unmanaged file") {
		t.Errorf("want the ordinary unmanaged-file skip after --force, got %+v", res.Skipped)
	}
}

// TestApplyOrphanDirRevertConverges is the second convergence exit: put the
// pack-provided file back the way escapement wrote it and the retirement
// proceeds on the next ordinary sync, with no --force and no skip.
func TestApplyOrphanDirRevertConverges(t *testing.T) {
	root := t.TempDir()
	const artPath = ".claude/skills/esc-acme-org-esc-security"
	orphanDirRepo(t, root, artPath, map[string]string{"SKILL.md": "pack content\n"}, nil)
	dir := filepath.Join(root, filepath.FromSlash(artPath))
	writeUnder(t, dir, "SKILL.md", "pack content\nedited\n")

	if res, err := Apply(root, &PlanResult{}, false); err != nil {
		t.Fatal(err)
	} else if len(res.Skipped) != 1 {
		t.Fatalf("precondition: want the edit declined, got %+v", res.Skipped)
	}

	writeUnder(t, dir, "SKILL.md", "pack content\n")
	res, err := Apply(root, &PlanResult{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Skipped) != 0 {
		t.Errorf("reverting the edit must converge with no skip, got %+v", res.Skipped)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("the retired directory must be gone once nothing local is in the way, stat err = %v", err)
	}
	lock, err := lockfile.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if lock.Artifact(artPath) != nil {
		t.Error("the lock entry must drop once the directory is actually removed")
	}
}

// TestApplyOrphanDirMissingPackFileFailsClosed covers the read-error branch
// of the same gate. A pack-provided file the team deleted makes the hash
// uncomputable, which means the files beside it cannot be shown to be
// unedited either. Deleting a file is not itself work worth preserving, but
// guessing in the destructive direction on an unverifiable directory is
// exactly what this whole pass exists to stop, so it declines and points at
// --force like every other skip.
func TestApplyOrphanDirMissingPackFileFailsClosed(t *testing.T) {
	root := t.TempDir()
	const artPath = ".claude/skills/esc-acme-org-esc-security"
	orphanDirRepo(t, root, artPath,
		map[string]string{"SKILL.md": "pack content\n", "EXTRA.md": "more pack content\n"}, nil)
	dir := filepath.Join(root, filepath.FromSlash(artPath))
	if err := os.Remove(filepath.Join(dir, "EXTRA.md")); err != nil {
		t.Fatal(err)
	}

	res, err := Apply(root, &PlanResult{}, false)
	if err != nil {
		t.Fatalf("an unverifiable retirement must be declined, not an error: %v", err)
	}
	if len(res.Skipped) != 1 {
		t.Fatalf("want one Skipped entry, got %+v", res.Skipped)
	}
	if res.Skipped[0].ActualHash != "" {
		t.Errorf("no hash can be computed, so none should be reported: %q", res.Skipped[0].ActualHash)
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Errorf("nothing may be removed from an unverifiable directory: %v", err)
	}
}

// TestApplyOrphanDirPreManifestLockUnaffected pins the len(prev.Files) guard.
// A lockfile written before the manifest change records a hash covering a
// file list it never stored, so comparing against it would be meaningless and
// would take every pre-manifest retirement down the edited-file branch with
// the wrong reason attached. That path already removes nothing and reports
// every on-disk file as unmanaged, which is the conservative answer it was
// designed to give.
func TestApplyOrphanDirPreManifestLockUnaffected(t *testing.T) {
	root := t.TempDir()
	const artPath = ".claude/skills/esc-acme-org-esc-security"
	writeUnder(t, filepath.Join(root, filepath.FromSlash(artPath)), "SKILL.md", "pack content\n")
	lock := &lockfile.Lock{Schema: 1, Artifacts: []lockfile.LockArtifact{
		{Path: artPath, Kind: KindDir, Hash: "sha256:whatever-the-old-scheme-recorded"},
	}}
	if err := lock.Save(root); err != nil {
		t.Fatal(err)
	}

	res, err := Apply(root, &PlanResult{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Skipped) != 1 || !strings.Contains(res.Skipped[0].Reason, "unmanaged file") {
		t.Fatalf("want the unmanaged-file skip on a pre-manifest lockfile, got %+v", res.Skipped)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(artPath), "SKILL.md")); err != nil {
		t.Errorf("a pre-manifest retirement must remove nothing: %v", err)
	}
}

// TestApplyOrphanDirSymlinkedPackFileFailsClosed is the fix-round regression
// test for the ordering defect in the hash gate. pack.DirHashOf READS every
// manifest entry, and the gate ran before the removal loop's containment
// checks, so a manifest entry replaced by a symlink was read THROUGH to
// out-of-repo content: the hash mismatched, the gate reported a routine
// exit-0 "orphan-dir-edited" skip, and refuseSymlinks never fired at all.
// That downgraded a containment failure to routine drift, the exact
// inversion this sweep's exit-code change argues against, and the skip's own
// remedy line then sent the user to `esc sync --force`, which bypasses the
// gate, reaches refuseSymlinks, and hard-errors. Containment now runs over
// the whole manifest before anything reads it.
func TestApplyOrphanDirSymlinkedPackFileFailsClosed(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	const artPath = ".claude/skills/esc-acme-org-esc-security"
	orphanDirRepo(t, root, artPath, map[string]string{"SKILL.md": "pack content\n"}, nil)

	victim := filepath.Join(outside, "victim.txt")
	if err := os.WriteFile(victim, []byte("do not touch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, filepath.FromSlash(artPath))
	if err := os.Remove(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(dir, "SKILL.md")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	res, err := Apply(root, &PlanResult{}, false)
	if err == nil {
		t.Fatalf("a symlinked manifest entry must fail closed, not become an exit-0 skip: %+v", res)
	}
	if !strings.Contains(err.Error(), "refusing to write through symlink") {
		t.Errorf("error must come from the symlink check, got %v", err)
	}
	if res != nil {
		t.Errorf("nothing may be reported after a containment failure, got %+v", res)
	}
	got, rerr := os.ReadFile(victim)
	if rerr != nil {
		t.Fatalf("the file outside the repo was removed: %v", rerr)
	}
	if string(got) != "do not touch\n" {
		t.Errorf("file outside the repo was modified: %q", got)
	}
}

// TestApplyOrphanDirTraversalEntryIsNotHashed is the other half of the same
// ordering defect. A manifest entry like "../../../../secret.txt" was read
// and sha256'd by the gate before containedPath ever rejected it, and because
// the gate then `continue`d, the removal loop that carries containedPath was
// never reached: the whole sync exited 0 having read a file outside the
// repository and published its hash as actual_hash. Removal was never at
// risk, but the read was real.
func TestApplyOrphanDirTraversalEntryIsNotHashed(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "a", "b", "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(parent, "secret.txt")
	if err := os.WriteFile(secret, []byte("out-of-repo content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The skill dir sits 3 segments under root, so 6 "..": 3 to reach root,
	// 3 more to clear the repo entirely and land on parent/secret.txt.
	const artPath = ".claude/skills/esc-acme-org-esc-security"
	const traversal = "../../../../../../secret.txt"
	writeUnder(t, filepath.Join(root, filepath.FromSlash(artPath)), "SKILL.md", "pack content\n")
	lock := &lockfile.Lock{Schema: 1, Artifacts: []lockfile.LockArtifact{
		{Path: artPath, Kind: KindDir, Hash: "sha256:stale", Files: []string{"SKILL.md", traversal}},
	}}
	if err := lock.Save(root); err != nil {
		t.Fatal(err)
	}
	// Precondition: the entry really does resolve onto the out-of-repo file,
	// so a gate that hashed before containing would in fact read it.
	if filepath.Clean(filepath.Join(root, filepath.FromSlash(artPath), traversal)) != filepath.Clean(secret) {
		t.Fatalf("precondition: traversal entry does not land on %s", secret)
	}

	res, err := Apply(root, &PlanResult{}, false)
	if err == nil {
		t.Fatalf("a traversing manifest entry must fail closed, not become an exit-0 skip: %+v", res)
	}
	if !strings.Contains(err.Error(), "escapes the repository root") {
		t.Errorf("error must come from the containment check, got %v", err)
	}
	if res != nil {
		t.Errorf("nothing may be reported after a containment failure, got %+v", res)
	}
	if _, serr := os.Stat(secret); serr != nil {
		t.Errorf("the out-of-repo file was removed: %v", serr)
	}
}

// TestApplyOrphanDirManifestPathIsNowADirectory covers the read-error branch
// for a path that is neither present-and-readable nor absent: os.ReadFile on
// a directory returns EISDIR, which is not os.IsNotExist, so the gate used to
// hard-error exit 4 on it instead of taking the fail-closed skip it intends
// for every unverifiable directory. Every read error is now the skip.
func TestApplyOrphanDirManifestPathIsNowADirectory(t *testing.T) {
	root := t.TempDir()
	const artPath = ".claude/skills/esc-acme-org-esc-security"
	orphanDirRepo(t, root, artPath, map[string]string{"SKILL.md": "pack content\n"}, nil)
	dir := filepath.Join(root, filepath.FromSlash(artPath))
	if err := os.Remove(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "SKILL.md"), 0o755); err != nil {
		t.Fatal(err)
	}

	res, err := Apply(root, &PlanResult{}, false)
	if err != nil {
		t.Fatalf("an unverifiable retirement must be declined, not fail the rollout: %v", err)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Cause != SkipOrphanDirEdited {
		t.Fatalf("want one orphan-dir-edited skip, got %+v", res.Skipped)
	}
	if r := res.Skipped[0].Reason; !strings.Contains(r, "unreadable") || !strings.Contains(r, "SKILL.md") {
		t.Errorf("skip reason must say what is unreadable and name it, got %q", r)
	}
	if _, serr := os.Stat(filepath.Join(dir, "SKILL.md")); serr != nil {
		t.Errorf("nothing may be removed from an unverifiable directory: %v", serr)
	}
}
