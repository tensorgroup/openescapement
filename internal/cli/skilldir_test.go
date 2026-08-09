package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/engine"
)

// TestSkillDirPreservesAddedFiles is the regression test for the data-loss
// defect: stageDir used to remove the whole destination tree on every sync,
// silently destroying any file a team added under an escapement-owned skill
// directory. See TestFixtureSkillsRepoSyncs for why the synced path is
// esc-acme-org-esc-security, not esc-security.
func TestSkillDirPreservesAddedFiles(t *testing.T) {
	repo := setupGovernedRepoWithSkills(t)
	runEsc(t, repo, "sync")

	dir := filepath.Join(repo, ".claude", "skills", "esc-acme-org-esc-security")
	added := filepath.Join(dir, "team-notes.md")
	if err := os.WriteFile(added, []byte("our own notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	runEsc(t, repo, "sync")

	got, err := os.ReadFile(added)
	if err != nil {
		t.Fatalf("added file destroyed by sync: %v", err)
	}
	if string(got) != "our own notes\n" {
		t.Errorf("added file mutated: %q", got)
	}
}

// TestSkillDirRemovesPackDroppedFiles verifies the other half of the
// three-way merge: a file the previous manifest recorded but the pack no
// longer provides must be removed, not kept as if it were a local amendment.
//
// Built inline rather than via setupGovernedRepoWithSkills, because this
// test needs the pack repo path in order to publish a version without
// EXTRA.md. Pack content nests under "org/" (see setupGovernedRepoWithSkills
// and newGoverned in cli_test.go), and the manifest already declares a
// "skills:" list for skills/vault-usage, so withExtraSkill extends that list
// in place rather than writing a second "skills:" key.
//
// This does not use the fixtures_test.go dropSkillFileFromPack helper: that
// helper force-retags the same v1.0.0 tag, which is indistinguishable from
// an attacker moving a tag (see TestMovedTagFailsClosed in cli_test.go) and
// is rejected by the lock-integrity check on the next sync regardless of
// intent — a check this task does not touch. A legitimate content change
// instead goes through a new version and the supported `update` flow, same
// as TestUpdateAndDiffAgainst.
func TestSkillDirRemovesPackDroppedFiles(t *testing.T) {
	packRepo := newPackRepo(t, "1.0.0")
	files := map[string]string{}
	for rel, content := range skillFiles() {
		files["org/"+rel] = content
	}
	files["org/pack.yaml"] = withExtraSkill(t, packRepo, "skills/esc-security")
	writeFiles(t, packRepo, files)
	gitIn(t, packRepo, "add", ".")
	gitIn(t, packRepo, "commit", "-m", "add skills")
	gitIn(t, packRepo, "tag", "-f", "v1.0.0")
	repo := newGoverned(t, packRepo, "v1.0.0")

	runEsc(t, repo, "sync")
	dropped := filepath.Join(repo, ".claude", "skills", "esc-acme-org-esc-security", "EXTRA.md")
	if _, err := os.Stat(dropped); err != nil {
		t.Fatalf("precondition: %v", err)
	}

	// Publish v1.0.1 without EXTRA.md, under a new tag.
	if err := os.Remove(filepath.Join(packRepo, "org", "skills", "esc-security", "EXTRA.md")); err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile(filepath.Join(packRepo, "org", "pack.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	bumped := strings.Replace(string(manifest), "version: 1.0.0\n", "version: 1.0.1\n", 1)
	if bumped == string(manifest) {
		t.Fatal("precondition: expected pack.yaml to declare version: 1.0.0")
	}
	writeFiles(t, packRepo, map[string]string{"org/pack.yaml": bumped})
	gitIn(t, packRepo, "add", "-A")
	gitIn(t, packRepo, "commit", "-m", "drop EXTRA.md")
	gitIn(t, packRepo, "tag", "-a", "v1.0.1", "-m", "v1.0.1")

	runEsc(t, repo, "update", "--ref", "v1.0.1")
	runEsc(t, repo, "sync")

	if _, err := os.Stat(dropped); !os.IsNotExist(err) {
		t.Error("a file the pack dropped must be removed, not kept as an amendment")
	}
}

// TestSkillDirUpgradePreservesUnknownFiles covers the upgrade path: a
// lockfile written before this change has no "files" entry for a dir
// artifact. On the first sync after upgrade, prevFiles is empty, so nothing
// unknown is removed even if the pack has in fact dropped a file — the
// conservative choice, since deleting a team's real work is worse than
// leaving one pack-dropped file to be reported later as an amendment.
func TestSkillDirUpgradePreservesUnknownFiles(t *testing.T) {
	repo := setupGovernedRepoWithSkills(t)
	runEsc(t, repo, "sync")

	lockPath := filepath.Join(repo, ".escapement", "escapement.lock")
	raw, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	var lock map[string]any
	if err := json.Unmarshal(raw, &lock); err != nil {
		t.Fatal(err)
	}
	arts, _ := lock["artifacts"].([]any)
	found := false
	for _, a := range arts {
		art, _ := a.(map[string]any)
		if art["kind"] == "dir" {
			delete(art, "files") // simulate a pre-Task-5 lock: no files key at all
			found = true
		}
	}
	if !found {
		t.Fatal("precondition: expected a dir artifact in the lock")
	}
	out, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, append(out, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	extra := filepath.Join(repo, ".claude", "skills", "esc-acme-org-esc-security", "EXTRA.md")
	if _, err := os.Stat(extra); err != nil {
		t.Fatalf("precondition: %v", err)
	}

	runEsc(t, repo, "sync")

	if _, err := os.Stat(extra); err != nil {
		t.Errorf("pre-manifest lock upgrade must not delete unknown files: %v", err)
	}
}

// TestSkillDirLockTraversalFailsClosed is the fix-round regression test for
// finding 1 (CRITICAL): escapement.lock is a committed, PR-reachable
// artifact, so its "files" entries for a dir artifact are not trusted input.
// Before the fix, mergeDir's removal loop joined a prevFiles entry straight
// onto dst with filepath.Join, which cleans ".." segments instead of
// rejecting them, so a lockfile entry like "../../../../outside.txt" could
// make `esc sync` delete a file outside the repo entirely. Every removal
// path now goes through containedPath first, mirroring the existing
// prevLock.Path check in Apply (apply.go:132), so this must fail the whole
// sync rather than reach the filesystem outside the repo.
func TestSkillDirLockTraversalFailsClosed(t *testing.T) {
	repo := setupGovernedRepoWithSkills(t)
	runEsc(t, repo, "sync")

	// One level above the repo root: .claude/skills/esc-acme-org-esc-security
	// is 3 path segments under repo, so 4 ".." collapses to exactly 1 level
	// above repo (path.Clean keeps the excess "..").
	outside := filepath.Join(filepath.Dir(repo), "outside-"+filepath.Base(repo)+".txt")
	if err := os.WriteFile(outside, []byte("do not touch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(outside)

	lockPath := filepath.Join(repo, ".escapement", "escapement.lock")
	raw, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	var lock map[string]any
	if err := json.Unmarshal(raw, &lock); err != nil {
		t.Fatal(err)
	}
	arts, _ := lock["artifacts"].([]any)
	found := false
	for _, a := range arts {
		art, _ := a.(map[string]any)
		if art["kind"] == "dir" {
			files, _ := art["files"].([]any)
			files = append(files, "../../../../outside-"+filepath.Base(repo)+".txt")
			art["files"] = files
			found = true
		}
	}
	if !found {
		t.Fatal("precondition: expected a dir artifact in the lock")
	}
	out, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, append(out, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, code := runEscOut(t, repo, "sync"); code == 0 {
		t.Fatal("sync with a traversal entry in the lockfile should fail closed, not exit 0")
	}

	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("a lockfile traversal entry deleted a file outside the repo: %v", err)
	}
}

// TestSkillDirLockTraversalWithinRepoFailsClosed is the second fix-round
// regression test for finding 1: the first fix round contained removal
// paths against root, not dst. Joining a prevFiles entry onto artPath and
// only then containing the result against root lets ".." collapse before
// containment is ever checked, so an entry with just enough ".." to land
// back inside the repo — but still outside the skill directory — sails
// through: root still contains it, only dst doesn't. This is exactly that
// case: .claude/skills/esc-acme-org-esc-security is 3 path segments under
// the repo root, and "../../../VICTIM.md" collapses to exactly the repo
// root, an ordinary in-repo file with no ownership relationship to this
// skill directory at all. Removal paths are now contained against dst
// itself (containedPath(dst, prev)), so this must still fail closed.
func TestSkillDirLockTraversalWithinRepoFailsClosed(t *testing.T) {
	repo := setupGovernedRepoWithSkills(t)
	runEsc(t, repo, "sync")

	victim := filepath.Join(repo, "VICTIM.md")
	if err := os.WriteFile(victim, []byte("do not touch\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	lockPath := filepath.Join(repo, ".escapement", "escapement.lock")
	raw, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	var lock map[string]any
	if err := json.Unmarshal(raw, &lock); err != nil {
		t.Fatal(err)
	}
	arts, _ := lock["artifacts"].([]any)
	found := false
	for _, a := range arts {
		art, _ := a.(map[string]any)
		if art["kind"] == "dir" {
			files, _ := art["files"].([]any)
			files = append(files, "../../../VICTIM.md")
			art["files"] = files
			found = true
		}
	}
	if !found {
		t.Fatal("precondition: expected a dir artifact in the lock")
	}
	out, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, append(out, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, code := runEscOut(t, repo, "sync"); code == 0 {
		t.Fatal("sync with an in-repo traversal entry in the lockfile should fail closed, not exit 0")
	}

	got, err := os.ReadFile(victim)
	if err != nil {
		t.Fatalf("an in-repo traversal entry deleted VICTIM.md: %v", err)
	}
	if string(got) != "do not touch\n" {
		t.Errorf("in-repo victim file was modified: %q", got)
	}
}

// TestSkillDirNestedSymlinkFailsClosed is the fix-round regression test for
// finding 2 (Important): refuseSymlinks(root, a.Path) in Apply only checks
// path components down to the skill directory itself, not the files
// mergeDir discovers underneath it. The old stageDir destroyed any nested
// symlink wholesale via os.RemoveAll, so it never mattered; mergeDir's
// per-file atomicWrite instead calls os.MkdirAll/os.CreateTemp on the
// file's parent directory, which the OS resolves through a symlinked
// intermediate component — so a symlinked subdirectory committed inside an
// escapement-owned skill dir became a write primitive into wherever it
// points. Every file mergeDir touches is now re-checked with refuseSymlinks
// immediately before the filesystem call, so this must fail closed and
// never touch the symlink's target.
func TestSkillDirNestedSymlinkFailsClosed(t *testing.T) {
	packRepo := newPackRepo(t, "1.0.0")
	files := map[string]string{
		"org/skills/esc-security/SKILL.md":    "---\nname: esc-security\ndescription: demo\n---\n\nRules.\n",
		"org/skills/esc-security/sub/note.md": "Nested pack content.\n",
	}
	files["org/pack.yaml"] = withExtraSkill(t, packRepo, "skills/esc-security")
	writeFiles(t, packRepo, files)
	gitIn(t, packRepo, "add", ".")
	gitIn(t, packRepo, "commit", "-m", "add nested skill file")
	gitIn(t, packRepo, "tag", "-f", "v1.0.0")
	repo := newGoverned(t, packRepo, "v1.0.0")

	runEsc(t, repo, "sync")
	dir := filepath.Join(repo, ".claude", "skills", "esc-acme-org-esc-security")
	if _, err := os.Stat(filepath.Join(dir, "sub", "note.md")); err != nil {
		t.Fatalf("precondition: %v", err)
	}

	// A team member (or a hostile PR) replaces the synced "sub" directory
	// with a symlink pointing outside the repo entirely.
	outside := t.TempDir()
	marker := filepath.Join(outside, "marker.txt")
	if err := os.WriteFile(marker, []byte("do not touch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, "sub")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "sub")); err != nil {
		t.Fatal(err)
	}

	// Same pack, same version: mergeDir re-copies and re-writes every synced
	// file on every sync, so no version bump is needed to exercise the
	// write loop's per-file symlink check.
	if _, code := runEscOut(t, repo, "sync"); code == 0 {
		t.Fatal("sync through a symlinked subdirectory should fail closed, not exit 0")
	}

	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("marker file outside the repo was removed: %v", err)
	}
	if string(got) != "do not touch\n" {
		t.Errorf("marker file outside the repo was modified: %q", got)
	}
	if entries, err := os.ReadDir(outside); err != nil {
		t.Fatal(err)
	} else if len(entries) != 1 {
		t.Errorf("sync wrote through the symlink into the outside directory: %v", entries)
	}
}

// TestSkillDirAmendmentReported is Task 6: a file a team adds under an
// escapement-owned skill directory must report as a local amendment on
// status, not be silently ignored (Task 5 already made sync preserve it) and
// not flip the directory's managed axis to altered (the managed hash covers
// only pack-provided files, per DirHashOf). See TestFixtureSkillsRepoSyncs
// for why the synced path is esc-acme-org-esc-security, not esc-security.
func TestSkillDirAmendmentReported(t *testing.T) {
	repo := setupGovernedRepoWithSkills(t)
	runEsc(t, repo, "sync")
	dir := filepath.Join(repo, ".claude", "skills", "esc-acme-org-esc-security")
	if err := os.WriteFile(filepath.Join(dir, "team-notes.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := engine.Status(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	// setupGovernedRepoWithSkills's fixture pack declares two skills
	// (vault-usage from packRepoFiles, plus esc-security added here), so
	// Findings carries two KindDir entries. Only esc-security got the added
	// file, so the check must be scoped to its path — checking every KindDir
	// finding indiscriminately fails on vault-usage's unrelated finding.
	const skillPath = ".claude/skills/esc-acme-org-esc-security"
	var found bool
	for _, f := range st.Findings {
		if f.Kind != engine.KindDir || f.Subject != skillPath {
			continue
		}
		found = true
		if f.State != engine.InSync {
			t.Errorf("managed axis = %q, want in-sync", f.State)
		}
		if f.Local != engine.LocalAmended {
			t.Errorf("local axis = %q, want amended", f.Local)
		}
		if f.Amendment == nil || len(f.Amendment.Items) != 1 || f.Amendment.Items[0] != "team-notes.md" {
			t.Errorf("amendment = %+v", f.Amendment)
		}
	}
	if !found {
		t.Fatal("no finding for " + skillPath)
	}
	if !st.Clean() {
		t.Error("an amendment alone must not make status unclean")
	}
}

// TestSkillDirPristineSyncReportsInSync locks in the false-positive
// direction this task exists to eliminate: a skill directory with no team
// additions must report in-sync/none on both axes right after `esc sync`,
// with no fix-round regression (see
// TestDirFilesAgreesWithDirHashAcrossGitDir in internal/pack for the unit
// case this guards: plan-time Files and the managed Hash used to come from
// two independently filtered walks, and a tree containing a nested .git
// directory made them disagree even with nothing a team touched).
func TestSkillDirPristineSyncReportsInSync(t *testing.T) {
	repo := setupGovernedRepoWithSkills(t)
	runEsc(t, repo, "sync")

	st, err := engine.Status(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	var dirFindings int
	for _, f := range st.Findings {
		if f.Kind != engine.KindDir {
			continue
		}
		dirFindings++
		if f.State != engine.InSync {
			t.Errorf("%s: managed axis = %q, want in-sync", f.Subject, f.State)
		}
		if f.Local != engine.LocalNone {
			t.Errorf("%s: local axis = %q, want none", f.Subject, f.Local)
		}
		if f.Amendment != nil {
			t.Errorf("%s: amendment = %+v, want nil", f.Subject, f.Amendment)
		}
	}
	if dirFindings != 2 {
		t.Fatalf("expected 2 dir findings (vault-usage, esc-security), got %d", dirFindings)
	}
	if !st.Clean() {
		t.Error("a pristine sync must report clean")
	}
}

// TestSkillDirAlteredIsNotSweptUpAsOrphan is the non-vacuity test for the
// desiredDirs assignment sitting BEFORE the skip gate in Apply, whose own
// comment calls that placement load-bearing but which nothing exercised.
//
// A hand-edited skill directory is declined by the skip gate, which
// `continue`s past the rest of the artifact loop. If desiredDirs were
// recorded after the gate instead of before it, that declined directory
// would be absent from the effective set as far as the orphaned-dir pass
// below is concerned, and the pass would then delete every pack-provided
// file in it — turning "we declined to touch your edit" into "we deleted the
// file your edit was in."
func TestSkillDirAlteredIsNotSweptUpAsOrphan(t *testing.T) {
	repo := setupGovernedRepoWithSkills(t)
	runEsc(t, repo, "sync")

	dir := filepath.Join(repo, ".claude", "skills", "esc-acme-org-esc-security")
	skill := filepath.Join(dir, "SKILL.md")
	edited := "---\nname: esc-security\ndescription: demo\n---\n\nRules, hand-edited.\n"
	if err := os.WriteFile(skill, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	// Precondition: this really is the declined path, not just an in-sync one.
	st, err := engine.Status(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	const skillPath = ".claude/skills/esc-acme-org-esc-security"
	var altered bool
	for _, f := range st.Findings {
		if f.Kind == engine.KindDir && f.Subject == skillPath && f.State == engine.Altered {
			altered = true
		}
	}
	if !altered {
		t.Fatal("precondition: the hand-edited skill dir should classify as altered")
	}

	runEsc(t, repo, "sync")

	got, err := os.ReadFile(skill)
	if err != nil {
		t.Fatalf("a declined skill dir was swept up by the orphaned-dir pass: %v", err)
	}
	if string(got) != edited {
		t.Errorf("declined skill dir was overwritten: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "EXTRA.md")); err != nil {
		t.Errorf("orphan pass removed a pack file from a dir still in the effective set: %v", err)
	}
}

// TestSkillDirRemovalThroughNestedSymlinkFailsClosed is the non-vacuity test
// for the refuseSymlinks call in mergeDir's REMOVAL loop. The existing
// nested-symlink test covers the write loop's call; deleting the removal
// loop's left the suite green, because that loop runs first and its
// containedPath check is purely lexical.
//
// Setup: the pack ships skills/esc-security/sub/note.md, then drops it in
// v1.0.1, making it a removal target. Between the update and the sync, the
// synced "sub" directory is swapped for a symlink out of the repo. Without
// the check, os.Remove resolves through that symlinked component and deletes
// the file it points at.
func TestSkillDirRemovalThroughNestedSymlinkFailsClosed(t *testing.T) {
	packRepo := newPackRepo(t, "1.0.0")
	files := map[string]string{
		"org/skills/esc-security/SKILL.md":    "---\nname: esc-security\ndescription: demo\n---\n\nRules.\n",
		"org/skills/esc-security/sub/note.md": "Nested pack content.\n",
	}
	files["org/pack.yaml"] = withExtraSkill(t, packRepo, "skills/esc-security")
	writeFiles(t, packRepo, files)
	gitIn(t, packRepo, "add", ".")
	gitIn(t, packRepo, "commit", "-m", "add nested skill file")
	gitIn(t, packRepo, "tag", "-f", "v1.0.0")
	repo := newGoverned(t, packRepo, "v1.0.0")
	runEsc(t, repo, "sync")

	dir := filepath.Join(repo, ".claude", "skills", "esc-acme-org-esc-security")
	if _, err := os.Stat(filepath.Join(dir, "sub", "note.md")); err != nil {
		t.Fatalf("precondition: %v", err)
	}

	// Publish v1.0.1 without sub/note.md, under a new tag (see
	// TestSkillDirRemovesPackDroppedFiles for why a new tag, not a retag).
	if err := os.RemoveAll(filepath.Join(packRepo, "org", "skills", "esc-security", "sub")); err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile(filepath.Join(packRepo, "org", "pack.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	bumped := strings.Replace(string(manifest), "version: 1.0.0\n", "version: 1.0.1\n", 1)
	if bumped == string(manifest) {
		t.Fatal("precondition: expected pack.yaml to declare version: 1.0.0")
	}
	writeFiles(t, packRepo, map[string]string{"org/pack.yaml": bumped})
	gitIn(t, packRepo, "add", "-A")
	gitIn(t, packRepo, "commit", "-m", "drop sub/note.md")
	gitIn(t, packRepo, "tag", "-a", "v1.0.1", "-m", "v1.0.1")
	runEsc(t, repo, "update", "--ref", "v1.0.1")

	// Only now swap "sub" for a symlink, so the pending removal of
	// sub/note.md is the thing that walks through it.
	outside := t.TempDir()
	victim := filepath.Join(outside, "note.md")
	if err := os.WriteFile(victim, []byte("do not touch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, "sub")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "sub")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	if _, code := runEscOut(t, repo, "sync"); code == 0 {
		t.Fatal("removing a pack-dropped file through a symlinked subdirectory should fail closed, not exit 0")
	}
	got, err := os.ReadFile(victim)
	if err != nil {
		t.Fatalf("the removal loop deleted a file outside the repo: %v", err)
	}
	if string(got) != "do not touch\n" {
		t.Errorf("file outside the repo was modified: %q", got)
	}
}

// retiredSkillDirRepo publishes a pack at v1.0.0 declaring skills/esc-security,
// governs a repo against it, syncs, and then publishes v1.0.1 with that skill
// dropped from the manifest. It returns the governed repo, left pinned at
// v1.0.0 so the caller can hand-edit the synced directory before running
// `esc update --ref v1.0.1`.
//
// Built inline rather than via setupGovernedRepoWithSkills for the reason
// TestSkillDirRemovesPackDroppedFiles gives: the caller needs the pack repo
// path in order to publish a second version, and the drop must go out under a
// NEW tag, since force-retagging is indistinguishable from an attacker moving
// a tag and is rejected by the lock-integrity check.
func retiredSkillDirRepo(t *testing.T) string {
	t.Helper()
	packRepo := newPackRepo(t, "1.0.0")
	files := map[string]string{}
	for rel, content := range skillFiles() {
		files["org/"+rel] = content
	}
	files["org/pack.yaml"] = withExtraSkill(t, packRepo, "skills/esc-security")
	writeFiles(t, packRepo, files)
	gitIn(t, packRepo, "add", ".")
	gitIn(t, packRepo, "commit", "-m", "add skills")
	gitIn(t, packRepo, "tag", "-f", "v1.0.0")
	repo := newGoverned(t, packRepo, "v1.0.0")
	runEsc(t, repo, "sync")

	manifest, err := os.ReadFile(filepath.Join(packRepo, "org", "pack.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	retired := strings.Replace(string(manifest), "  - skills/esc-security\n", "", 1)
	if retired == string(manifest) {
		t.Fatal("precondition: pack.yaml does not declare skills/esc-security")
	}
	bumped := strings.Replace(retired, "version: 1.0.0\n", "version: 1.0.1\n", 1)
	if bumped == retired {
		t.Fatal("precondition: expected pack.yaml to declare version: 1.0.0")
	}
	writeFiles(t, packRepo, map[string]string{"org/pack.yaml": bumped})
	gitIn(t, packRepo, "add", "-A")
	gitIn(t, packRepo, "commit", "-m", "retire esc-security")
	gitIn(t, packRepo, "tag", "-a", "v1.0.1", "-m", "v1.0.1")
	return repo
}

// TestOrphanSkillDirDeclinedRetirementIsVisibleToCheck is the fix-round
// regression test for the compliance-gate hole. Orphan detection filtered
// dir-kind lock entries out entirely, so no orphan-dir finding existed in any
// state: sync would decline to retire a skill directory holding a hand-edited
// pack file, park the lock entry, and re-report the decline on every run,
// while `esc status` said nothing and `esc status --check` — where AGENTS.md
// puts compliance gating — exited 0 forever. The orphan-block path has warned
// on status first since it was written; the dir path now matches it.
func TestOrphanSkillDirDeclinedRetirementIsVisibleToCheck(t *testing.T) {
	repo := retiredSkillDirRepo(t)
	dir := filepath.Join(repo, ".claude", "skills", "esc-acme-org-esc-security")
	skill := filepath.Join(dir, "SKILL.md")
	original, err := os.ReadFile(skill)
	if err != nil {
		t.Fatalf("precondition: %v", err)
	}
	edited := append(append([]byte(nil), original...), []byte("\nOur own addition.\n")...)
	if err := os.WriteFile(skill, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	runEsc(t, repo, "update", "--ref", "v1.0.1")

	// Status says so BEFORE sync is ever asked to delete it.
	out, code := runEscOut(t, repo, "status")
	if code != 0 {
		t.Fatalf("bare status on an orphan should exit 0, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "esc-acme-org-esc-security") || !strings.Contains(out, "orphan") {
		t.Errorf("status must report the retired skill directory as an orphan:\n%s", out)
	}
	if !strings.Contains(out, "hand-edited") {
		t.Errorf("status must report the hand-edit sync is about to decline:\n%s", out)
	}
	if !strings.Contains(out, "`esc sync --force` removes it") {
		t.Errorf("status must name the remedy, matching the orphan-block wording:\n%s", out)
	}

	if _, code := runEscOut(t, repo, "status", "--check"); code != 1 {
		t.Errorf("`status --check` must fail on a retired skill dir sync will decline, got %d", code)
	}

	// Sync declines, exits 0, and preserves the edit.
	syncOut, code := runEscOut(t, repo, "sync")
	if code != 0 {
		t.Fatalf("a declined retirement must not fail the rollout, exit=%d:\n%s", code, syncOut)
	}
	if got, rerr := os.ReadFile(skill); rerr != nil {
		t.Fatalf("the hand-edited pack file was destroyed: %v", rerr)
	} else if !strings.Contains(string(got), "Our own addition.") {
		t.Errorf("the hand-edit was overwritten: %q", got)
	}

	// And the gate stays closed afterwards: the parked state is exactly the
	// one that used to be invisible forever.
	if _, code := runEscOut(t, repo, "status", "--check"); code != 1 {
		t.Errorf("`status --check` must still fail while the retirement stays declined, got %d", code)
	}

	// Converged by --force: the directory goes, and the gate reopens.
	runEsc(t, repo, "sync", "--force")
	if _, serr := os.Stat(dir); !os.IsNotExist(serr) {
		t.Errorf("--force must retire the directory, stat err = %v", serr)
	}
	if out, code := runEscOut(t, repo, "status", "--check"); code != 0 {
		t.Errorf("`status --check` must pass once the retirement completes, got %d:\n%s", code, out)
	}
}

// TestOrphanSkillDirUnmanagedFilesReportedByStatus is the other orphan-dir
// status shape: no hand-edit, but files the team added. Sync will remove the
// pack files and keep the directory, so status must say that rather than the
// bare "run `esc sync` to remove it", and must report the added files on the
// local axis like every other amendment.
func TestOrphanSkillDirUnmanagedFilesReportedByStatus(t *testing.T) {
	repo := retiredSkillDirRepo(t)
	dir := filepath.Join(repo, ".claude", "skills", "esc-acme-org-esc-security")
	if err := os.WriteFile(filepath.Join(dir, "team-notes.md"), []byte("ours\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runEsc(t, repo, "update", "--ref", "v1.0.1")

	out, code := runEscOut(t, repo, "status")
	if code != 0 {
		t.Fatalf("bare status on an orphan should exit 0, got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "keeps the directory for the files you added") {
		t.Errorf("status must say the directory survives for the team's files:\n%s", out)
	}
	if !strings.Contains(out, "1 unmanaged file preserved") {
		t.Errorf("status must report the added file on the local axis:\n%s", out)
	}
	if _, code := runEscOut(t, repo, "status", "--check"); code != 1 {
		t.Errorf("an orphaned skill directory is not in sync, so --check must fail, got %d", code)
	}
}
