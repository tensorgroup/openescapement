package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tensorgroup/openescapement/internal/pack"
)

func TestDiscoverSkillsSuiteAndRoot(t *testing.T) {
	suite := t.TempDir()
	writeFiles(t, suite, map[string]string{
		"skills/brainstorming/SKILL.md": "a\n",
		"skills/writing-plans/SKILL.md": "b\n",
		"skills/writing-plans/ref.md":   "c\n",
		"README.md":                     "not a skill\n",
	})
	got, err := discoverSkills(suite, "suite-root")
	if err != nil || len(got) != 2 {
		t.Fatalf("discover: %+v, %v", got, err)
	}
	if got[0].Name != "brainstorming" || got[1].Name != "writing-plans" {
		t.Fatalf("wrong names/order: %+v", got)
	}

	single := t.TempDir()
	writeFiles(t, single, map[string]string{"SKILL.md": "root skill\n"})
	got, err = discoverSkills(single, "my-skill")
	if err != nil || len(got) != 1 || got[0].Name != "my-skill" || got[0].Dir != single {
		t.Fatalf("root-is-a-skill: %+v, %v", got, err)
	}
}

func TestDiscoverSkillsRejectsNestedAndEmpty(t *testing.T) {
	nested := t.TempDir()
	writeFiles(t, nested, map[string]string{
		"outer/SKILL.md":       "a\n",
		"outer/inner/SKILL.md": "b\n",
	})
	if _, err := discoverSkills(nested, "x"); err == nil {
		t.Fatal("nested skill dirs must be an error")
	}
	if _, err := discoverSkills(t.TempDir(), "x"); err == nil {
		t.Fatal("no skills found must be an error")
	}
}

func TestVendorCopyMatchesDirFilesAndRefusesExisting(t *testing.T) {
	src := t.TempDir()
	writeFiles(t, src, map[string]string{"SKILL.md": "s\n", "scripts/run.sh": "#!/bin/sh\n"})
	if err := os.Chmod(filepath.Join(src, "scripts", "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	dstRoot := t.TempDir()
	dst := filepath.Join(dstRoot, "skills", "demo")
	files, hash, err := vendorCopy(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	wantHash, err := pack.DirHash(dst)
	if err != nil || hash != wantHash {
		t.Fatalf("recorded hash %q != DirHash(dst) %q (%v)", hash, wantHash, err)
	}
	srcHash, _ := pack.DirHash(src)
	if hash != srcHash {
		t.Fatalf("vendored copy not byte-identical to upstream: %q != %q", hash, srcHash)
	}
	if len(files) != 2 {
		t.Fatalf("files: %v", files)
	}
	if info, _ := os.Stat(filepath.Join(dst, "scripts", "run.sh")); info.Mode()&0o111 == 0 {
		t.Fatal("executable bit not preserved")
	}
	if _, _, err := vendorCopy(src, dst); err == nil {
		t.Fatal("existing destination must be refused")
	}
}

func TestDiffstat(t *testing.T) {
	old := map[string]string{"a": "1", "b": "2", "c": "3"}
	new := map[string]string{"a": "1", "b": "9", "d": "4"}
	added, removed, changed := diffstat(old, new)
	if added != 1 || removed != 1 || changed != 1 {
		t.Fatalf("got %d/%d/%d, want 1/1/1", added, removed, changed)
	}
}
