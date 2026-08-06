package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runEsc runs a command and fails the test unless it exits 0.
func runEsc(t *testing.T, root string, args ...string) {
	t.Helper()
	code, out := run(t, root, args...)
	if code != 0 {
		t.Fatalf("esc %s exited %d:\n%s", strings.Join(args, " "), code, out)
	}
}

// runEscOut returns output first, matching how the assertions below read.
func runEscOut(t *testing.T, root string, args ...string) (string, int) {
	t.Helper()
	code, out := run(t, root, args...)
	return out, code
}

func setupGovernedRepo(t *testing.T) string {
	t.Helper()
	return newGoverned(t, newPackRepo(t, "1.0.0"), "v1.0.0")
}

// skillFiles is the pack-side fixture for a skill directory. Kept separate so
// dropSkillFileFromPack can publish a version without one of these entries.
func skillFiles() map[string]string {
	return map[string]string{
		"skills/esc-security/SKILL.md": "---\nname: esc-security\ndescription: demo\n---\n\nRules.\n",
		"skills/esc-security/EXTRA.md": "Extra pack content.\n",
	}
}

// setupGovernedRepoWithSkills publishes a pack version declaring a skills
// dir, packRepoFiles/newPackRepo nest all pack content under "org/" (the
// governed repo's source URL is file://<packRepo>//org), so every
// pack-relative path here is written under that prefix. packRepoFiles
// already emits a "skills:" key (for vault-usage), so this extends that list
// in place rather than appending a second "skills:" mapping, which yaml.v3
// rejects as a duplicate key.
func setupGovernedRepoWithSkills(t *testing.T) string {
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
	return newGoverned(t, packRepo, "v1.0.0")
}

// setupGovernedRepoWithReporting activates in Task 8, when pack.Manifest
// gains a Reporting field; until then the produced pack.yaml has an unknown
// "reporting" key and fails to parse under KnownFields(true).
func setupGovernedRepoWithReporting(t *testing.T, level string) string {
	t.Helper()
	packRepo := newPackRepo(t, "1.0.0")
	writeFiles(t, packRepo, map[string]string{
		"org/pack.yaml": withManifestLines(t, packRepo, "reporting:\n  amendments: "+level+"\n"),
	})
	gitIn(t, packRepo, "add", ".")
	gitIn(t, packRepo, "commit", "-m", "declare reporting")
	gitIn(t, packRepo, "tag", "-f", "v1.0.0")
	return newGoverned(t, packRepo, "v1.0.0")
}

// withManifestLines appends lines to the pack manifest already on disk, at
// org/pack.yaml (see setupGovernedRepoWithSkills for why "org/").
func withManifestLines(t *testing.T, packRepo, extra string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(packRepo, "org", "pack.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s + extra
}

// withExtraSkill returns the pack manifest with rel appended to the existing
// "skills:" list (packRepoFiles always declares one entry, "skills/vault-usage").
func withExtraSkill(t *testing.T, packRepo, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(packRepo, "org", "pack.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	const anchor = "skills:\n  - skills/vault-usage\n"
	s := string(raw)
	if !strings.Contains(s, anchor) {
		t.Fatalf("pack.yaml missing expected skills: entry to extend:\n%s", s)
	}
	return strings.Replace(s, anchor, anchor+"  - "+rel+"\n", 1)
}

// dropSkillFileFromPack publishes a new pack version without rel (a
// pack-relative path, e.g. "skills/esc-security/EXTRA.md"), retagging so the
// governed repo picks it up on the next sync.
func dropSkillFileFromPack(t *testing.T, packRepo, rel string) {
	t.Helper()
	if err := os.Remove(filepath.Join(packRepo, "org", filepath.FromSlash(rel))); err != nil {
		t.Fatal(err)
	}
	gitIn(t, packRepo, "add", "-A")
	gitIn(t, packRepo, "commit", "-m", "drop "+rel)
	gitIn(t, packRepo, "tag", "-f", "v1.0.0")
}

// TestFixtureSkillsRepoSyncs is a smoke test for setupGovernedRepoWithSkills.
// The artifact path is .claude/skills/esc-<pack-name>-<skill-dir-base>, not
// .claude/skills/<skill-dir-base>: see render.TargetSkills handling in
// internal/engine/engine.go, which builds the name as
// "esc-" + p.Manifest.Name + "-" + filepath.Base(rel). newPackRepo's fixture
// manifest (internal/cli/cli_test.go) names the pack "acme-org", so the skill
// declared at "skills/esc-security" lands at
// .claude/skills/esc-acme-org-esc-security/SKILL.md.
func TestFixtureSkillsRepoSyncs(t *testing.T) {
	repo := setupGovernedRepoWithSkills(t)
	runEsc(t, repo, "sync")
	if _, err := os.Stat(filepath.Join(repo, ".claude", "skills", "esc-acme-org-esc-security", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}
