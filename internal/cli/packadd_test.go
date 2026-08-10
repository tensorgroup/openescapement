package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/pack"
)

// newUpstreamSkillRepo builds a real git repo carrying a two-skill suite
// under skills/, tagged v1.0.0, and returns its path.
func newUpstreamSkillRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitIn(t, repo, "init", "-q")
	writeFiles(t, repo, map[string]string{
		"README.md":                       "a skill suite\n",
		"skills/brainstorming/SKILL.md":   "---\nname: brainstorming\n---\n\nExplore first.\n",
		"skills/writing-plans/SKILL.md":   "---\nname: writing-plans\n---\n\nPlan second.\n",
		"skills/writing-plans/example.md": "an example\n",
	})
	gitIn(t, repo, "add", ".")
	gitIn(t, repo, "commit", "-q", "-m", "v1")
	gitIn(t, repo, "tag", "v1.0.0")
	return repo
}

// newAuthorPack scaffolds a pack repo (the author's cwd for esc pack ...).
func newAuthorPack(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"pack.yaml": "schema: 1\nname: acme\nversion: 1.0.0\n",
	})
	return root
}

func TestPackAddSkillVendorsSuiteWithProvenance(t *testing.T) {
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	up := newUpstreamSkillRepo(t)
	root := newAuthorPack(t)
	out, code := runEscOut(t, root, "pack", "add-skill", "file://"+up+"#skills")
	if code != 0 {
		t.Fatalf("add-skill exited %d:\n%s", code, out)
	}
	for _, rel := range []string{
		"skills/brainstorming/SKILL.md",
		"skills/writing-plans/SKILL.md",
		"skills/writing-plans/example.md",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Errorf("vendored file missing: %s (%v)", rel, err)
		}
	}
	// pack.yaml gained object entries and still loads.
	p, err := pack.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, e := range p.Manifest.Skills {
		names[e.Name] = true
	}
	if !names["brainstorming"] || !names["writing-plans"] {
		t.Fatalf("pack.yaml entries wrong: %+v", p.Manifest.Skills)
	}
	// sources.yaml records provenance with an exact resolved commit and the
	// dir hash of each vendored copy.
	srcs, err := pack.LoadSources(root)
	if err != nil || srcs == nil || len(srcs.Skills) != 2 {
		t.Fatalf("sources.yaml: %+v, %v", srcs, err)
	}
	hex40 := regexp.MustCompile(`^[0-9a-f]{40}$`)
	for _, s := range srcs.Skills {
		if s.Ref != "v1.0.0" || !hex40.MatchString(s.Commit) {
			t.Errorf("entry %s: ref=%q commit=%q", s.Name, s.Ref, s.Commit)
		}
		wantHash, err := pack.DirHash(filepath.Join(root, "skills", s.Name))
		if err != nil || s.Hash != wantHash {
			t.Errorf("entry %s: recorded hash %q, dir hashes to %q (%v)", s.Name, s.Hash, wantHash, err)
		}
	}
	// The command told the author what happened at which commit.
	if !strings.Contains(out, "brainstorming") || !strings.Contains(out, "v1.0.0") {
		t.Errorf("summary output missing vendored names/ref:\n%s", out)
	}
}

func TestPackAddSkillOnlyAndRefusals(t *testing.T) {
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	up := newUpstreamSkillRepo(t)
	root := newAuthorPack(t)
	runEsc(t, root, "pack", "add-skill", "file://"+up+"#skills", "--only", "brainstorming")
	if _, err := os.Stat(filepath.Join(root, "skills", "writing-plans")); !os.IsNotExist(err) {
		t.Fatal("--only did not narrow the vendored set")
	}
	// A taken name is refused, and nothing is partially written.
	out, code := runEscOut(t, root, "pack", "add-skill", "file://"+up+"#skills", "--only", "brainstorming")
	if code == 0 {
		t.Fatalf("re-vendoring a taken name must fail:\n%s", out)
	}
	// Unknown --only name is an error, not a silent no-op.
	if _, code := runEscOut(t, root, "pack", "add-skill", "file://"+up+"#skills", "--only", "nope"); code == 0 {
		t.Fatal("--only with an unknown name must fail")
	}
}

func TestPackAddSkillNoTagsFallsBackToHeadWithWarning(t *testing.T) {
	t.Setenv("ESC_CACHE_DIR", t.TempDir())
	up := t.TempDir()
	gitIn(t, up, "init", "-q")
	writeFiles(t, up, map[string]string{"SKILL.md": "---\nname: solo\n---\n\nOne skill, no tags.\n"})
	gitIn(t, up, "add", ".")
	gitIn(t, up, "commit", "-q", "-m", "head only")
	root := newAuthorPack(t)
	out, code := runEscOut(t, root, "pack", "add-skill", "file://"+up)
	if code != 0 {
		t.Fatalf("add-skill exited %d:\n%s", code, out)
	}
	if !strings.Contains(out, "no semver tags") {
		t.Fatalf("missing fallback warning:\n%s", out)
	}
	srcs, _ := pack.LoadSources(root)
	if srcs == nil || len(srcs.Skills) != 1 || srcs.Skills[0].Ref != "HEAD" || srcs.Skills[0].Commit == "" {
		t.Fatalf("HEAD fallback not recorded exactly: %+v", srcs)
	}
	// Repo-root-is-a-skill: named after the repo directory.
	if _, err := os.Stat(filepath.Join(root, "skills", filepath.Base(up), "SKILL.md")); err != nil {
		t.Fatalf("root skill not vendored under repo name: %v", err)
	}
}
