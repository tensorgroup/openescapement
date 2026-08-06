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
