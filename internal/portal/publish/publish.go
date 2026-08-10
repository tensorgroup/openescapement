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
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/pack"
)

// Manager works on working git clones under Dir: Dir/<name>/ is a git repo
// whose root is an esc pack (pack.yaml at top level).
type Manager struct {
	Dir string

	// locks holds one *sync.Mutex per pack name, created on first use, so
	// concurrent Publish calls for the same clone serialize (closing the
	// TOCTOU window between the tag-exists guard and tag creation, and
	// preventing one call's `git add -A` from sweeping up another's
	// in-progress write) while different clones still proceed independently.
	locks sync.Map
}

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

// ErrFragmentExists reports that AddFragment's destination path is already
// present in the pack on disk. The portal maps it to a 422 directing the user
// to edit that fragment instead; v1 never overwrites.
var ErrFragmentExists = errors.New("fragment already exists in pack")

// ErrBadVersion reports that a user-supplied version string is malformed
// (fails versionRE) or already published as a git tag. Both Publish and
// AddFragment wrap it with descriptive text; the portal maps it to a 422
// with the submitted form state preserved, since the version field is
// user-editable free text a typo can easily break.
var ErrBadVersion = errors.New("invalid or already-published version")

// gitRun runs git in dir, capturing combined output into the error. It is a
// package-level var (not a plain func) so tests can substitute a wrapper to
// simulate a failure at a specific point in the Publish sequence without
// touching the real git plumbing everywhere else.
var gitRun = func(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// lockFor returns the mutex guarding Publish calls for the named clone.
func (m *Manager) lockFor(name string) *sync.Mutex {
	v, _ := m.locks.LoadOrStore(name, &sync.Mutex{})
	return v.(*sync.Mutex)
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
	for _, e := range p.Manifest.Skills {
		// A skill entry's Path is a directory, not a fragment: os.ReadFile
		// on it (ReadFragment, and every page/handler that iterates
		// Fragments) fails with EISDIR. The skill's displayable content is
		// its SKILL.md, so the fragment is the file within the directory,
		// joined with "/" rather than filepath.Join to keep the string
		// forward-slash on every platform, matching every other fragment
		// path here and what safeFragPath expects. Manifest validation only
		// checks that the declared path is a directory, not that a
		// SKILL.md lives inside it, so a missing SKILL.md is skipped
		// rather than added: contributing a fragment that would also 500
		// is worse than a pack rendering with one less fragment.
		skillFrag := e.Path + "/SKILL.md"
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(skillFrag))); err != nil {
			continue
		}
		frags = append(frags, skillFrag)
	}
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
// ".." escapes, paths reaching into the clone's own .git directory, and
// symlink targets (including via a not-yet-existent leaf under a
// symlinked ancestor directory) that resolve outside cloneDir. It mirrors
// pack.safeRel's hardening but also checks the resolved filesystem target,
// since a fragment path may point through a symlink.
func safeFragPath(cloneDir, frag string) (string, error) {
	if filepath.IsAbs(frag) {
		return "", fmt.Errorf("fragment path %q: absolute paths not allowed", frag)
	}
	clean := filepath.Clean(filepath.FromSlash(frag))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("fragment path %q escapes the pack clone", frag)
	}
	first := clean
	if idx := strings.IndexRune(clean, filepath.Separator); idx >= 0 {
		first = clean[:idx]
	}
	// EqualFold, not ==: on case-insensitive filesystems (macOS default,
	// Windows) ".Git/config" resolves to the same real .git directory as
	// ".git/config", so an exact-case compare alone would reopen the
	// vulnerability this guard exists to close.
	if strings.EqualFold(first, ".git") {
		return "", fmt.Errorf("fragment path %q: writes under .git are not allowed", frag)
	}
	full := filepath.Join(cloneDir, clean)

	// Resolve symlinks against the deepest existing ancestor of full, not
	// full itself: for a fragment that doesn't exist yet (a brand-new file
	// being published), EvalSymlinks(full) just errors "no such file",
	// which would otherwise skip this check entirely and let a symlinked
	// *parent* directory smuggle the write outside the clone.
	resolved, err := resolveExistingAncestor(full)
	if err != nil {
		return "", fmt.Errorf("fragment path %q: %v", frag, err)
	}
	cloneResolved, err := filepath.EvalSymlinks(cloneDir)
	if err != nil {
		cloneResolved = cloneDir
	}
	rel, err := filepath.Rel(cloneResolved, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("fragment path %q resolves outside the pack clone", frag)
	}
	return full, nil
}

// resolveExistingAncestor returns the symlink-resolved form of the deepest
// existing ancestor of path (path itself, if it already exists). cloneDir
// is always among path's ancestors and always exists, so the walk is
// guaranteed to terminate.
func resolveExistingAncestor(path string) (string, error) {
	for {
		if _, err := os.Lstat(path); err == nil {
			return filepath.EvalSymlinks(path)
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return path, nil
		}
		path = parent
	}
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

// restore discards any working-tree AND index changes in dir, resetting it
// back to ref — "HEAD" before anything has been committed, or a captured
// pre-commit SHA to unwind a commit whose tag step failed — then removes
// any untracked files the reset left behind. `git checkout -- .` only
// restores tracked files from the index, not the index itself from HEAD:
// after `git add -A`, that would leave staged changes (or, if the commit
// already landed, the commit itself) in place — a half-published state.
// `git reset --hard` undoes both.
//
// restore always runs on a context with cancellation/deadline stripped: the
// caller's ctx may already be cancelled (e.g. a request that triggered
// Publish timed out), but restore must still run to completion so the
// clone is never left half-published.
func restore(ctx context.Context, dir, ref string) error {
	ctx = context.WithoutCancel(ctx)
	if _, err := gitRun(ctx, dir, "reset", "--hard", ref); err != nil {
		return err
	}
	if _, err := gitRun(ctx, dir, "clean", "-fd"); err != nil {
		return err
	}
	return nil
}

// restoreAndErr restores dir's working tree to ref and returns cause,
// folding in any restore failure so it is never silently swallowed.
func restoreAndErr(ctx context.Context, dir, ref string, cause error) error {
	if err := restore(ctx, dir, ref); err != nil {
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
// half-published: any failure restores the clone to its pre-Publish state
// before returning, including unwinding a commit whose tag step failed.
//
// Steps, in order:
//  1. Guards: newVersion is well-formed, frag stays inside the clone, and
//     the target tag does not already exist.
//  2. Write the fragment and rewrite the manifest version.
//  3. Validate via pack.Load; restore and return on failure.
//  4. git add -A, commit, and annotated-tag; restore and return on any git
//     failure. A failure after the commit lands is unwound back to the
//     exact pre-commit SHA (not a relative HEAD~1), and only when that
//     commit is still HEAD — if something else moved HEAD in the meantime,
//     Publish refuses to guess and surfaces both failures instead.
//
// Publish for a given pack name serializes against other Publish calls for
// the same name (see Manager.locks): without that, the tag-exists guard in
// step 1 and the tag creation in step 4 race, and one call's step 4
// `git add -A` can sweep up another's in-progress write.
func (m *Manager) Publish(ctx context.Context, name, frag string, content []byte, newVersion string) error {
	lock := m.lockFor(name)
	lock.Lock()
	defer lock.Unlock()

	dir := filepath.Join(m.Dir, name)

	// Step 1: guards.
	if !versionRE.MatchString(newVersion) {
		return fmt.Errorf("%w: invalid version %q: must match %s", ErrBadVersion, newVersion, versionRE.String())
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
		return fmt.Errorf("%w: tag %s already exists", ErrBadVersion, tagName)
	}

	// Step 2: write fragment + rewrite manifest version.
	if err := os.WriteFile(fragPath, content, 0o644); err != nil {
		return restoreAndErr(ctx, dir, "HEAD", err)
	}
	if err := rewriteVersion(filepath.Join(dir, "pack.yaml"), newVersion); err != nil {
		return restoreAndErr(ctx, dir, "HEAD", err)
	}

	// Step 3: validate.
	if _, err := pack.Load(dir); err != nil {
		if rerr := restore(ctx, dir, "HEAD"); rerr != nil {
			return fmt.Errorf("%w: pack validation failed: %v (restore also failed: %v)", esc.ErrManifest, err, rerr)
		}
		return fmt.Errorf("%w: pack validation failed: %v", esc.ErrManifest, err)
	}

	// Step 4: commit + tag.
	return commitAndTag(ctx, dir, name, newVersion)
}

// commitAndTag stages, commits, and annotated-tags the pack clone at dir as
// v<newVersion>. It captures the pre-commit SHA so a failed tag step unwinds
// exactly that commit, and only while it is still HEAD; any failure restores
// the clone before returning. Shared by Publish (edit an existing fragment)
// and AddFragment (adopt a new one).
func commitAndTag(ctx context.Context, dir, name, newVersion string) error {
	tagName := "v" + newVersion
	preCommitHead, err := gitRun(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return restoreAndErr(ctx, dir, "HEAD", err)
	}
	preCommitHead = strings.TrimSpace(preCommitHead)

	if _, err := gitRun(ctx, dir, "add", "-A"); err != nil {
		return restoreAndErr(ctx, dir, "HEAD", err)
	}
	commitMsg := fmt.Sprintf("portal: publish %s v%s", name, newVersion)
	if _, err := gitRun(ctx, dir,
		"-c", "user.name=esc portal", "-c", "user.email=portal@escapement.local", "-c", "commit.gpgsign=false",
		"commit", "-m", commitMsg,
	); err != nil {
		return restoreAndErr(ctx, dir, "HEAD", err)
	}

	newHead, err := gitRun(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return restoreAndErr(ctx, dir, preCommitHead, err)
	}
	newHead = strings.TrimSpace(newHead)

	tagMsg := fmt.Sprintf("publish v%s", newVersion)
	if _, err := gitRun(ctx, dir,
		"-c", "user.name=esc portal", "-c", "user.email=portal@escapement.local", "-c", "tag.gpgsign=false",
		"tag", "-a", tagName, "-m", tagMsg,
	); err != nil {
		if curHead, herr := gitRun(ctx, dir, "rev-parse", "HEAD"); herr == nil && strings.TrimSpace(curHead) == newHead {
			return restoreAndErr(ctx, dir, preCommitHead, err)
		}
		return fmt.Errorf("tag failed and the commit could not be safely unwound (HEAD moved): %v", err)
	}
	return nil
}

// addRuleToManifest appends rel to the top-level "rules" sequence of the
// pack.yaml at path, creating the key if absent. It round-trips through
// yaml.Node so comments and key order survive, mirroring rewriteVersion.
func addRuleToManifest(path, rel string) error {
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
	var rules *yaml.Node
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == "rules" {
			rules = mapping.Content[i+1]
			break
		}
	}
	if rules == nil {
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "rules"},
			&yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"})
		rules = mapping.Content[len(mapping.Content)-1]
	}
	if rules.Kind != yaml.SequenceNode {
		rules.Kind = yaml.SequenceNode
		rules.Tag = "!!seq"
		rules.Value = ""
	}
	rules.Content = append(rules.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: rel})
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

// AddFragment adopts a brand-new fragment into the named pack: it writes
// content to frag, adds frag to pack.yaml's rules list, bumps the version to
// newVersion, validates, and commits + tags v<newVersion>. It refuses to
// overwrite: if frag already exists on disk it returns ErrFragmentExists and
// touches nothing. Like Publish, any failure after the working tree is
// modified restores the clone before returning, and calls for the same pack
// serialize.
func (m *Manager) AddFragment(ctx context.Context, name, frag string, content []byte, newVersion string) error {
	lock := m.lockFor(name)
	lock.Lock()
	defer lock.Unlock()

	dir := filepath.Join(m.Dir, name)

	if !versionRE.MatchString(newVersion) {
		return fmt.Errorf("%w: invalid version %q: must match %s", ErrBadVersion, newVersion, versionRE.String())
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
		return fmt.Errorf("%w: tag %s already exists", ErrBadVersion, tagName)
	}
	// Collision guard: never overwrite an existing fragment. This runs before
	// anything is written, so no restore is needed on this path.
	if _, err := os.Stat(fragPath); err == nil {
		return ErrFragmentExists
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(fragPath), 0o755); err != nil {
		return restoreAndErr(ctx, dir, "HEAD", err)
	}
	if err := os.WriteFile(fragPath, content, 0o644); err != nil {
		return restoreAndErr(ctx, dir, "HEAD", err)
	}
	manifestPath := filepath.Join(dir, "pack.yaml")
	if err := addRuleToManifest(manifestPath, frag); err != nil {
		return restoreAndErr(ctx, dir, "HEAD", err)
	}
	if err := rewriteVersion(manifestPath, newVersion); err != nil {
		return restoreAndErr(ctx, dir, "HEAD", err)
	}

	if _, err := pack.Load(dir); err != nil {
		if rerr := restore(ctx, dir, "HEAD"); rerr != nil {
			return fmt.Errorf("%w: pack validation failed: %v (restore also failed: %v)", esc.ErrManifest, err, rerr)
		}
		return fmt.Errorf("%w: pack validation failed: %v", esc.ErrManifest, err)
	}

	return commitAndTag(ctx, dir, name, newVersion)
}
