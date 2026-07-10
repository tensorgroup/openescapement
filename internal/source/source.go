// Package source resolves and fetches pack sources. Remote sources are
// fetched with the system git binary so the user's existing auth (SSH keys,
// credential helpers) applies to private pack repos.
package source

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/esc"
)

// Parsed is a decomposed source string.
type Parsed struct {
	Local  bool   // plain directory on disk, no git involved
	URL    string // git URL for remote sources; original path for local
	Subdir string // pack subdirectory inside the repo ("" = repo root)
}

// FetchResult is a pack checkout on disk.
type FetchResult struct {
	Dir     string // pack directory (subdir applied)
	RepoDir string // git checkout root ("" for plain local dirs)
	Commit  string // resolved commit SHA ("" for plain local dirs)
}

// ParseSource decomposes a config source string.
//
//	github.com/org/repo//subdir   -> https clone URL + subdir
//	git@host:path//subdir         -> scp-style URL + subdir
//	https://... | file://...      -> used as-is
//	./x ../x /abs/x               -> local plain directory
func ParseSource(s string) (*Parsed, error) {
	if s == "" {
		return nil, fmt.Errorf("empty pack source")
	}
	if strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") || strings.HasPrefix(s, "/") {
		return &Parsed{Local: true, URL: s}, nil
	}
	// Split off //subdir, ignoring the scheme's own "//".
	rest := s
	scheme := ""
	if idx := strings.Index(s, "://"); idx >= 0 {
		scheme = s[:idx+3]
		rest = s[idx+3:]
	}
	url, subdir := rest, ""
	if idx := strings.Index(rest, "//"); idx >= 0 {
		url, subdir = rest[:idx], rest[idx+2:]
	}
	full := scheme + url
	if scheme == "" && !strings.Contains(url, "@") {
		full = "https://" + url
	}
	return &Parsed{URL: full, Subdir: subdir}, nil
}

var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// validRef constrains git refs to a conservative charset. The leading
// character excludes "-" so a ref can never be parsed as a git option.
var validRef = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]*$`)

func cacheKey(url string) string {
	sum := sha256.Sum256([]byte(url))
	base := unsafeChars.ReplaceAllString(strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "git@"), "-")
	if len(base) > 60 {
		base = base[len(base)-60:]
	}
	return base + "-" + hex.EncodeToString(sum[:6])
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// Fetch materializes a pack source on disk at its pinned ref. Local plain
// directories resolve relative to repoRoot and return Commit="".
func Fetch(ctx context.Context, ref config.PackRef, cacheDir, repoRoot string) (*FetchResult, error) {
	parsed, err := ParseSource(ref.Source)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", esc.ErrFetch, err)
	}
	if parsed.Local {
		dir := parsed.URL
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(repoRoot, dir)
		}
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("%w: local pack %s: not a directory", esc.ErrFetch, ref.Source)
		}
		return &FetchResult{Dir: dir}, nil
	}
	if ref.Ref == "" {
		return nil, fmt.Errorf("%w: source %s: ref is required for git sources", esc.ErrFetch, ref.Source)
	}
	// Reject option-shaped values before anything reaches git's argv.
	if !validRef.MatchString(ref.Ref) {
		return nil, fmt.Errorf("%w: source %s: ref %q contains disallowed characters", esc.ErrFetch, ref.Source, ref.Ref)
	}
	if strings.HasPrefix(parsed.URL, "-") {
		return nil, fmt.Errorf("%w: source %q looks like a command-line option", esc.ErrFetch, ref.Source)
	}
	checkout := filepath.Join(cacheDir, cacheKey(parsed.URL))
	if _, statErr := os.Stat(filepath.Join(checkout, ".git")); statErr != nil {
		if err := os.MkdirAll(cacheDir, 0o755); err != nil {
			return nil, fmt.Errorf("%w: %v", esc.ErrFetch, err)
		}
		if _, err := git(ctx, "", "clone", "--quiet", "--", parsed.URL, checkout); err != nil {
			return nil, fmt.Errorf("%w: cloning %s: %v", esc.ErrFetch, parsed.URL, err)
		}
	} else {
		if _, err := git(ctx, checkout, "fetch", "--quiet", "--tags", "--force", "origin"); err != nil {
			return nil, fmt.Errorf("%w: fetching %s: %v", esc.ErrFetch, parsed.URL, err)
		}
	}
	if _, err := git(ctx, checkout, "checkout", "--quiet", "--detach", ref.Ref); err != nil {
		return nil, fmt.Errorf("%w: source %s: ref %q not found: %v", esc.ErrFetch, ref.Source, ref.Ref, err)
	}
	commit, err := git(ctx, checkout, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", esc.ErrFetch, err)
	}
	dir := checkout
	if parsed.Subdir != "" {
		dir = filepath.Join(checkout, filepath.FromSlash(parsed.Subdir))
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%w: source %s: subdir %q not found at %s", esc.ErrFetch, ref.Source, parsed.Subdir, ref.Ref)
	}
	return &FetchResult{Dir: dir, RepoDir: checkout, Commit: commit}, nil
}

// DefaultCacheDir returns the per-user pack cache directory.
func DefaultCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "escapement"), nil
}

// LsRemoteTags returns tag name -> commit hash for all tags at url, without
// fetching any content. It reuses the same arg-injection hygiene as Fetch.
func LsRemoteTags(ctx context.Context, url string) (map[string]string, error) {
	if strings.HasPrefix(url, "-") {
		return nil, fmt.Errorf("%w: source %q looks like a command-line option", esc.ErrFetch, url)
	}
	out, err := git(ctx, "", "ls-remote", "--tags", "--", url)
	if err != nil {
		return nil, fmt.Errorf("%w: ls-remote %s: %v", esc.ErrFetch, url, err)
	}
	tags := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		hash, ref := fields[0], fields[1]
		name := strings.TrimPrefix(ref, "refs/tags/")
		if name == ref {
			continue // not a tag ref
		}
		// Annotated tags emit both refs/tags/X (tag object) and
		// refs/tags/X^{} (the commit). Prefer the dereferenced commit.
		deref := strings.HasSuffix(name, "^{}")
		name = strings.TrimSuffix(name, "^{}")
		if _, seen := tags[name]; !seen || deref {
			tags[name] = hash
		}
	}
	return tags, nil
}

// LsRemoteHash returns the commit hash url resolves ref to, or "" if absent.
// Annotated tags are peeled to their commit so the result agrees with
// LsRemoteTags.
func LsRemoteHash(ctx context.Context, url, ref string) (string, error) {
	if strings.HasPrefix(url, "-") {
		return "", fmt.Errorf("%w: source %q looks like a command-line option", esc.ErrFetch, url)
	}
	if !validRef.MatchString(ref) {
		return "", fmt.Errorf("%w: ref %q contains disallowed characters", esc.ErrFetch, ref)
	}
	// A name-pattern query does not emit the ^{} peeled line, so ask for
	// both patterns explicitly. The ^{} suffix is appended to the
	// already-validated ref, so the injection guard above still holds.
	out, err := git(ctx, "", "ls-remote", "--", url, ref, ref+"^{}")
	if err != nil {
		return "", fmt.Errorf("%w: ls-remote %s %s: %v", esc.ErrFetch, url, ref, err)
	}
	hash := ""
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if strings.HasSuffix(fields[1], "^{}") {
			return fields[0], nil // peeled commit of an annotated tag wins
		}
		if hash == "" {
			hash = fields[0]
		}
	}
	return hash, nil
}
