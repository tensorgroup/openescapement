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

// TestPackUpdateSkillSourcesHashMatchesVendoredDir pins the ordering
// invariant: once a skill's directory is swapped in, sources.yaml's
// recorded hash must already agree with what's on disk. Computing the
// diffstat from the live directory (and saving sources.yaml only
// afterward) leaves a window where a failure between the swap and the
// save would strand the new content on disk under the OLD hash — this
// assertion is what that self-contradicting state would violate.
func TestPackUpdateSkillSourcesHashMatchesVendoredDir(t *testing.T) {
	root, up := vendoredAuthorPack(t)
	publishSkillV2(t, up)
	runEsc(t, root, "pack", "update-skill", "brainstorming")
	srcs, err := pack.LoadSources(root)
	if err != nil {
		t.Fatal(err)
	}
	e := srcs.Skill("brainstorming")
	if e == nil {
		t.Fatal("brainstorming not recorded")
	}
	got, err := pack.DirHash(filepath.Join(root, "skills", "brainstorming"))
	if err != nil {
		t.Fatal(err)
	}
	if got != e.Hash {
		t.Fatalf("sources.yaml hash %q does not match the vendored directory's hash %q", e.Hash, got)
	}
}

// TestPackUpdateSkillUnknownNameAbortsBeforeAnyWrite confirms the whole
// requested name set is validated against sources.yaml before the first
// re-vendor, so an unknown name in a multi-name invocation cannot leave an
// earlier, valid skill already re-vendored while the command still fails.
func TestPackUpdateSkillUnknownNameAbortsBeforeAnyWrite(t *testing.T) {
	root, up := vendoredAuthorPack(t)
	publishSkillV2(t, up)
	before, err := pack.LoadSources(root)
	if err != nil {
		t.Fatal(err)
	}
	beforeContent, err := os.ReadFile(filepath.Join(root, "skills", "brainstorming", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	// "brainstorming" sorts before "nope", so a loop that validates names as
	// it goes would already have re-vendored brainstorming by the time it
	// hits the unknown name.
	if _, code := runEscOut(t, root, "pack", "update-skill", "brainstorming", "nope"); code == 0 {
		t.Fatal("an unknown name among valid ones must still fail")
	}
	after, err := pack.LoadSources(root)
	if err != nil {
		t.Fatal(err)
	}
	if after.Skill("brainstorming").Commit != before.Skill("brainstorming").Commit {
		t.Fatal("an unknown name must abort before any earlier valid skill is re-vendored")
	}
	afterContent, err := os.ReadFile(filepath.Join(root, "skills", "brainstorming", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(afterContent) != string(beforeContent) {
		t.Fatal("vendored copy must be untouched when the batch aborts on an unknown name")
	}
}
