package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
