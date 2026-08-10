package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/pack"
)

// publishSkillV2 rewrites a skill upstream and tags v2.0.0.
func publishSkillV2(t *testing.T, up string) {
	t.Helper()
	writeFiles(t, up, map[string]string{
		"skills/brainstorming/SKILL.md": "---\nname: brainstorming\n---\n\nExplore first, v2.\n",
	})
	gitIn(t, up, "add", ".")
	gitIn(t, up, "commit", "-q", "-m", "v2")
	gitIn(t, up, "tag", "v2.0.0")
}

func vendoredAuthorPack(t *testing.T) (root, up string) {
	t.Helper()
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	up = newUpstreamSkillRepo(t)
	root = newAuthorPack(t)
	runEsc(t, root, "pack", "add-skill", "file://"+up+"#skills")
	return root, up
}

func TestPackUpdateSkillPullsNewVersionAndPrintsDiffstat(t *testing.T) {
	root, up := vendoredAuthorPack(t)
	publishSkillV2(t, up)
	out, code := runEscOut(t, root, "pack", "update-skill", "brainstorming")
	if code != 0 {
		t.Fatalf("update-skill exited %d:\n%s", code, out)
	}
	got, _ := os.ReadFile(filepath.Join(root, "skills", "brainstorming", "SKILL.md"))
	if !strings.Contains(string(got), "v2") {
		t.Fatalf("vendored copy not updated:\n%s", got)
	}
	if !strings.Contains(out, "changed") { // diffstat line
		t.Fatalf("no diffstat printed:\n%s", out)
	}
	srcs, _ := pack.LoadSources(root)
	if e := srcs.Skill("brainstorming"); e == nil || e.Ref != "v2.0.0" {
		t.Fatalf("provenance not advanced: %+v", e)
	}
}

func TestPackUpdateSkillDivergenceGateMirrorsSync(t *testing.T) {
	root, up := vendoredAuthorPack(t)
	// The author edits the vendored copy after vendoring.
	writeFiles(t, root, map[string]string{
		"skills/brainstorming/SKILL.md": "locally tweaked\n",
	})
	publishSkillV2(t, up)
	before, _ := pack.LoadSources(root)
	out, code := runEscOut(t, root, "pack", "update-skill", "brainstorming")
	if code != 0 {
		t.Fatalf("a skipped update is exit 0 (it keeps reporting), got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "skipped") {
		t.Fatalf("no skip warning:\n%s", out)
	}
	got, _ := os.ReadFile(filepath.Join(root, "skills", "brainstorming", "SKILL.md"))
	if string(got) != "locally tweaked\n" {
		t.Fatal("divergence gate overwrote the author's edit")
	}
	after, _ := pack.LoadSources(root)
	if after.Skill("brainstorming").Commit != before.Skill("brainstorming").Commit {
		t.Fatal("skip must leave sources.yaml unchanged so the divergence keeps reporting")
	}
	// --force converges, exactly like sync --force on a managed region.
	runEsc(t, root, "pack", "update-skill", "brainstorming", "--force")
	got, _ = os.ReadFile(filepath.Join(root, "skills", "brainstorming", "SKILL.md"))
	if !strings.Contains(string(got), "v2") {
		t.Fatal("--force did not converge the vendored copy")
	}
}

func TestPackUpdateSkillAllAndUnknown(t *testing.T) {
	root, up := vendoredAuthorPack(t)
	publishSkillV2(t, up)
	runEsc(t, root, "pack", "update-skill", "--all")
	srcs, _ := pack.LoadSources(root)
	for _, e := range srcs.Skills {
		if e.Ref != "v2.0.0" {
			t.Errorf("%s not advanced by --all: %+v", e.Name, e)
		}
	}
	if _, code := runEscOut(t, root, "pack", "update-skill", "nope"); code == 0 {
		t.Fatal("unknown skill name must fail")
	}
	if _, code := runEscOut(t, root, "pack", "update-skill"); code != 2 {
		t.Fatal("no names and no --all is a usage error")
	}
}
