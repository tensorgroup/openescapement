package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tensorgroup/openescapement/internal/pack"
)

// TestPackAddSkillRefusesInvalidName is Important-1's regression test.
// cmdPackAddSkill's doc comment claims "all refusals happen before anything
// is written", but the discovered skill name (an upstream directory
// basename, or the URL's #subdir basename) was never checked against
// pack.ValidName before being written to skills/<name>/ and appended to
// pack.yaml — so a name like "my skill" (a space, which ValidName forbids)
// landed on disk and in the manifest, and only pack.Load's own validation
// caught it afterwards, by which point the author already has a pack that
// no longer loads. The name must be refused before any write, exactly like
// every other "taken name" refusal in this same pre-write loop.
func TestPackAddSkillRefusesInvalidName(t *testing.T) {
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	up := t.TempDir()
	gitIn(t, up, "init", "-q")
	gitIdentity(t, up)
	writeFiles(t, up, map[string]string{"my skill/SKILL.md": "---\nname: x\n---\n\nBody.\n"})
	gitIn(t, up, "add", ".")
	gitIn(t, up, "commit", "-q", "-m", "v1")
	gitIn(t, up, "tag", "v1.0.0")

	root := newAuthorPack(t)
	out, code := runEscOut(t, root, "pack", "add-skill", "file://"+up+"#my skill")
	if code == 0 {
		t.Fatalf("an invalid skill name must be refused, not vendored:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(root, "skills")); !os.IsNotExist(err) {
		t.Fatalf("nothing may be written to disk when the discovered name is invalid")
	}
	p, err := pack.Load(root)
	if err != nil {
		t.Fatalf("pack.yaml must still load after a refused add-skill: %v", err)
	}
	if len(p.Manifest.Skills) != 0 {
		t.Fatalf("pack.yaml gained an entry despite the refusal: %+v", p.Manifest.Skills)
	}
	if srcs, err := pack.LoadSources(root); err != nil || srcs != nil {
		t.Fatalf("sources.yaml written despite the refusal: %+v, %v", srcs, err)
	}
}
