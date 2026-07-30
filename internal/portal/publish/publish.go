// Package publish manages server-side working git clones of pack repos: it
// validates and publishes new rule-fragment content through a
// validate -> commit -> tag flow. Nothing is ever half-published — any
// failure after the working tree is touched restores it before returning.
package publish

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/pack"
)

// Manager works on working git clones under Dir: Dir/<name>/ is a git repo
// whose root is an esc pack (pack.yaml at top level).
type Manager struct{ Dir string }

// PackInfo summarizes one pack clone.
type PackInfo struct {
	Name, Version, Description, Dir string
	Fragments                       []string // sorted relative paths from manifest Rules+Skills
	Tags                            []Tag    // newest first
}

// Tag is one git tag on a pack clone.
type Tag struct {
	Name string // e.g. "v1.2.0"
	Date string // "2026-07-30"
}

// NewManager returns a Manager rooted at dir.
func NewManager(dir string) *Manager {
	return &Manager{Dir: dir}
}

// versionRE constrains publish versions to strict semver-shaped x.y.z.
var versionRE = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// gitRun runs git in dir, capturing combined output into the error.
func gitRun(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// List returns every pack clone under Dir, sorted by name. Subdirectories
// that are not both an esc pack (pack.yaml present) and a git repo (.git
// present) are skipped.
func (m *Manager) List(ctx context.Context) ([]PackInfo, error) {
	entries, err := os.ReadDir(m.Dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(m.Dir, e.Name())
		if _, err := os.Stat(filepath.Join(dir, "pack.yaml")); err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	infos := make([]PackInfo, 0, len(names))
	for _, name := range names {
		info, err := m.Get(ctx, name)
		if err != nil {
			return nil, err
		}
		infos = append(infos, *info)
	}
	return infos, nil
}

// Get loads and returns the pack clone named name.
func (m *Manager) Get(ctx context.Context, name string) (*PackInfo, error) {
	dir := filepath.Join(m.Dir, name)
	p, err := pack.Load(dir)
	if err != nil {
		return nil, err
	}
	frags := make([]string, 0, len(p.Manifest.Rules)+len(p.Manifest.Skills))
	frags = append(frags, p.Manifest.Rules...)
	frags = append(frags, p.Manifest.Skills...)
	sort.Strings(frags)
	tags, err := listTags(ctx, dir)
	if err != nil {
		return nil, err
	}
	return &PackInfo{
		Name:        p.Manifest.Name,
		Version:     p.Manifest.Version,
		Description: p.Manifest.Description,
		Dir:         dir,
		Fragments:   frags,
		Tags:        tags,
	}, nil
}

// listTags returns dir's git tags, newest first.
func listTags(ctx context.Context, dir string) ([]Tag, error) {
	// creatordate has one-second resolution: publishing two versions within
	// the same second (routine for the commit+tag pair Publish issues) ties
	// on the primary key. -version:refname breaks the tie deterministically;
	// git treats the last --sort flag as primary, so creatordate still
	// governs whenever timestamps actually differ.
	out, err := gitRun(ctx, dir, "for-each-ref", "--sort=-version:refname", "--sort=-creatordate", "--format=%(refname:short) %(creatordate:short)", "refs/tags")
	if err != nil {
		return nil, err
	}
	var tags []Tag
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		tags = append(tags, Tag{Name: fields[0], Date: fields[1]})
	}
	return tags, nil
}

// safeFragPath resolves frag against cloneDir, rejecting absolute paths,
// ".." escapes, and symlink targets that resolve outside cloneDir. It
// mirrors pack.safeRel's hardening but also checks the resolved filesystem
// target, since a fragment path may point through a symlink.
func safeFragPath(cloneDir, frag string) (string, error) {
	if filepath.IsAbs(frag) {
		return "", fmt.Errorf("fragment path %q: absolute paths not allowed", frag)
	}
	clean := filepath.Clean(filepath.FromSlash(frag))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("fragment path %q escapes the pack clone", frag)
	}
	full := filepath.Join(cloneDir, clean)
	if resolved, err := filepath.EvalSymlinks(full); err == nil {
		cloneResolved, err2 := filepath.EvalSymlinks(cloneDir)
		if err2 != nil {
			cloneResolved = cloneDir
		}
		rel, err3 := filepath.Rel(cloneResolved, resolved)
		if err3 != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("fragment path %q resolves outside the pack clone", frag)
		}
	}
	return full, nil
}

// ReadFragment returns the current on-disk content of frag within the named
// pack clone.
func (m *Manager) ReadFragment(name, frag string) ([]byte, error) {
	dir := filepath.Join(m.Dir, name)
	full, err := safeFragPath(dir, frag)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(full)
}

// Diff compares the current on-disk fragment against proposed content using
// `git diff --no-index`. It returns "" when the contents are identical.
func (m *Manager) Diff(ctx context.Context, name, frag string, proposed []byte) (string, error) {
	dir := filepath.Join(m.Dir, name)
	full, err := safeFragPath(dir, frag)
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp("", "esc-portal-diff-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(proposed); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, "git", "diff", "--no-index", "--", full, tmp.Name())
	out, err := cmd.CombinedOutput()
	if err == nil {
		return "", nil // identical
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return string(out), nil // files differ; this is success for Diff
	}
	return "", fmt.Errorf("git diff --no-index: %v\n%s", err, out)
}

// restore discards any working-tree changes in dir via `git checkout -- .`
// and `git clean -fd`.
func restore(ctx context.Context, dir string) error {
	if _, err := gitRun(ctx, dir, "checkout", "--", "."); err != nil {
		return err
	}
	if _, err := gitRun(ctx, dir, "clean", "-fd"); err != nil {
		return err
	}
	return nil
}

// restoreAndErr restores dir's working tree and returns cause, folding in
// any restore failure so it is never silently swallowed.
func restoreAndErr(ctx context.Context, dir string, cause error) error {
	if err := restore(ctx, dir); err != nil {
		return fmt.Errorf("%v (restore also failed: %v)", cause, err)
	}
	return cause
}

// rewriteVersion rewrites the top-level "version" key in the pack.yaml at
// path to newVersion, preserving comments and key order by round-tripping
// through yaml.Node instead of the typed Manifest struct.
func rewriteVersion(path, newVersion string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("pack.yaml: expected a top-level mapping")
	}
	mapping := doc.Content[0]
	found := false
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == "version" {
			mapping.Content[i+1].Value = newVersion
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("pack.yaml: version key not found")
	}
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

// Publish writes content to frag, bumps pack.yaml's version to newVersion,
// validates the result, and commits + tags it as v<newVersion>. Nothing is
// half-published: any failure restores the working tree to its
// pre-Publish state before returning.
//
// Steps, in order:
//  1. Guards: newVersion is well-formed, frag stays inside the clone, and
//     the target tag does not already exist.
//  2. Write the fragment and rewrite the manifest version.
//  3. Validate via pack.Load; restore and return on failure.
//  4. git add -A, commit, and annotated-tag; restore and return on any git
//     failure.
func (m *Manager) Publish(ctx context.Context, name, frag string, content []byte, newVersion string) error {
	dir := filepath.Join(m.Dir, name)

	// Step 1: guards.
	if !versionRE.MatchString(newVersion) {
		return fmt.Errorf("invalid version %q: must match %s", newVersion, versionRE.String())
	}
	fragPath, err := safeFragPath(dir, frag)
	if err != nil {
		return err
	}
	tagName := "v" + newVersion
	existing, err := gitRun(ctx, dir, "tag", "-l", tagName)
	if err != nil {
		return err
	}
	if strings.TrimSpace(existing) != "" {
		return fmt.Errorf("tag %s already exists", tagName)
	}

	// Step 2: write fragment + rewrite manifest version.
	if err := os.WriteFile(fragPath, content, 0o644); err != nil {
		return restoreAndErr(ctx, dir, err)
	}
	if err := rewriteVersion(filepath.Join(dir, "pack.yaml"), newVersion); err != nil {
		return restoreAndErr(ctx, dir, err)
	}

	// Step 3: validate.
	if _, err := pack.Load(dir); err != nil {
		if rerr := restore(ctx, dir); rerr != nil {
			return fmt.Errorf("%w: pack validation failed: %v (restore also failed: %v)", esc.ErrManifest, err, rerr)
		}
		return fmt.Errorf("%w: pack validation failed: %v", esc.ErrManifest, err)
	}

	// Step 4: commit + tag.
	if _, err := gitRun(ctx, dir, "add", "-A"); err != nil {
		return restoreAndErr(ctx, dir, err)
	}
	commitMsg := fmt.Sprintf("portal: publish %s v%s", name, newVersion)
	if _, err := gitRun(ctx, dir,
		"-c", "user.name=esc portal", "-c", "user.email=portal@escapement.local", "-c", "commit.gpgsign=false",
		"commit", "-m", commitMsg,
	); err != nil {
		return restoreAndErr(ctx, dir, err)
	}
	tagMsg := fmt.Sprintf("publish v%s", newVersion)
	if _, err := gitRun(ctx, dir,
		"-c", "user.name=esc portal", "-c", "user.email=portal@escapement.local", "-c", "tag.gpgsign=false",
		"tag", "-a", tagName, "-m", tagMsg,
	); err != nil {
		return restoreAndErr(ctx, dir, err)
	}
	return nil
}
