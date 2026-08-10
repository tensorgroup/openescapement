package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/pack"
)

// discoveredSkill is one skill directory found in a fetched source tree.
type discoveredSkill struct {
	Name string // on-disk name: the directory's base, or fallbackName at the root
	Dir  string // absolute path of the skill directory
}

// discoverSkills finds skill directories by the presence of SKILL.md (spec
// §4). If dir itself directly contains SKILL.md, the whole tree is one skill
// named fallbackName (the subdir's base, or the repo name). Otherwise every
// directory directly containing SKILL.md is a skill named after its base.
// Nested skills are ambiguous and refused. Symlinked entries are not
// followed during discovery — the copy path (pack.DirFiles) fails closed on
// any symlink inside a selected skill, which is the actual trust boundary.
func discoverSkills(dir, fallbackName string) ([]discoveredSkill, error) {
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err == nil {
		return []discoveredSkill{{Name: fallbackName, Dir: dir}}, nil
	}
	var found []discoveredSkill
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil // never follow; if it's inside a chosen skill, DirFiles refuses later
		}
		if !d.IsDir() {
			return nil
		}
		if d.Name() == ".git" {
			return filepath.SkipDir
		}
		if p == dir {
			return nil
		}
		// Deliberately keep walking into every directory that has SKILL.md,
		// rather than SkipDir once found: a SKILL.md nested deeper inside
		// this directory must still be discovered below, so it can be
		// reported as an ambiguous nesting instead of silently dropped.
		if _, serr := os.Stat(filepath.Join(p, "SKILL.md")); serr == nil {
			found = append(found, discoveredSkill{Name: d.Name(), Dir: p})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("no SKILL.md found under %s: nothing to vendor", dir)
	}
	for i := range found {
		for j := range found {
			if i == j {
				continue
			}
			if strings.HasPrefix(found[j].Dir, found[i].Dir+string(filepath.Separator)) {
				return nil, fmt.Errorf("skill directory %s contains a nested skill directory %s; ambiguous — vendor from a narrower #subdir", found[i].Dir, found[j].Dir)
			}
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Name < found[j].Name })
	for i := 1; i < len(found); i++ {
		if found[i].Name == found[i-1].Name {
			return nil, fmt.Errorf("two skill directories are both named %q; use --only after moving one aside upstream, or vendor from a narrower #subdir", found[i].Name)
		}
	}
	return found, nil
}

// vendorCopy copies srcDir into dstDir, enumerating files with the SAME
// pack.DirFiles walk hashing uses — never a second, divergent walk (the
// hardening cycle documented exactly that bug class). Symlinks anywhere in
// srcDir fail the copy closed. dstDir must not exist: overwriting is the
// caller's explicit, separately-gated decision. The executable bit is
// preserved (skills ship scripts); everything else lands 0o644.
func vendorCopy(srcDir, dstDir string) (files []string, hash string, err error) {
	if _, serr := os.Lstat(dstDir); serr == nil {
		return nil, "", fmt.Errorf("destination %s already exists", dstDir)
	}
	files, err = pack.DirFiles(srcDir)
	if err != nil {
		return nil, "", err
	}
	for _, rel := range files {
		src := filepath.Join(srcDir, filepath.FromSlash(rel))
		dst := filepath.Join(dstDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, "", err
		}
		content, err := os.ReadFile(src)
		if err != nil {
			return nil, "", err
		}
		mode := os.FileMode(0o644)
		if info, err := os.Stat(src); err == nil && info.Mode()&0o111 != 0 {
			mode = 0o755
		}
		if err := os.WriteFile(dst, content, mode); err != nil {
			return nil, "", err
		}
	}
	// Hash the DESTINATION: what sources.yaml records must be the hash of
	// what is actually in the pack repo, not an assumption about the copy.
	hash, err = pack.DirHashOf(dstDir, files)
	if err != nil {
		return nil, "", err
	}
	return files, hash, nil
}

// fileHashes returns rel path -> content hash for every file DirFiles sees.
func fileHashes(dir string) (map[string]string, error) {
	files, err := pack.DirFiles(dir)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(files))
	for _, rel := range files {
		content, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		out[rel] = esc.HashBytes(content)
	}
	return out, nil
}

// diffstat compares two fileHashes maps.
func diffstat(old, new map[string]string) (added, removed, changed int) {
	for rel, h := range new {
		oh, ok := old[rel]
		switch {
		case !ok:
			added++
		case oh != h:
			changed++
		}
	}
	for rel := range old {
		if _, ok := new[rel]; !ok {
			removed++
		}
	}
	return added, removed, changed
}

// splitSkillURL splits the CLI's URL[#subdir] form. The spec's authoring
// commands use '#' (not the pack-source '//' convention) so a URL can be
// pasted verbatim; it is translated to internal/source's form by the caller.
func splitSkillURL(arg string) (url, subdir string) {
	if i := strings.LastIndex(arg, "#"); i >= 0 {
		return arg[:i], arg[i+1:]
	}
	return arg, ""
}

// repoBaseName derives a fallback skill name from a git URL: the last path
// segment with any .git suffix trimmed.
func repoBaseName(url string) string {
	s := strings.TrimSuffix(strings.TrimRight(url, "/"), ".git")
	if i := strings.LastIndexAny(s, "/:"); i >= 0 {
		s = s[i+1:]
	}
	return s
}
