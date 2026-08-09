// Package publisher will hold the local telemetry surface (repo identity,
// event shaping, transport). This file is its first piece: repo identity.
package publisher

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

// gitCommand builds one git invocation scoped to root (`-C root`), with
// LC_ALL=C forced so the caller's locale never leaks into git's output.
// Mirrors internal/cli's helper of the same name and shape; this package
// keeps its own unexported copy rather than importing internal/cli, since
// the CLI package must not be a dependency of this one.
func gitCommand(ctx context.Context, root string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	return cmd
}

// remoteSchemes lists the URL schemes NormalizeRemote strips. Closed set:
// nothing outside these four forms is recognized.
var remoteSchemes = []string{"ssh://", "https://", "http://", "git://"}

// NormalizeRemote reduces a git remote URL to a comparable "host/org/repo"
// form, so the SSH and HTTPS forms of one remote bucket identically and
// credentials embedded in a URL never survive normalization. Rules (closed
// set, applied in order): trim space; strip a leading ssh://, https://,
// http://, or git:// scheme; strip userinfo (user@ or user:pass@); convert
// scp-like shorthand (git@host:org/repo) to host/org/repo; strip one
// trailing ".git" and any trailing "/"; lowercase the host segment only,
// leaving the path's case untouched. Empty input returns "". Input with no
// recognizable host/path shape passes through with only the generic steps
// (trim, scheme/suffix strip) applied, rather than erroring.
func NormalizeRemote(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}

	for _, scheme := range remoteSchemes {
		if strings.HasPrefix(s, scheme) {
			s = s[len(scheme):]
			break
		}
	}

	// Userinfo can only appear in the authority segment (before the first
	// "/"), never in the path: an "@" past that point (e.g. a literal "@"
	// in a path segment) is not credentials and must survive. Within that
	// segment, take the LAST "@" so a password containing its own literal
	// "@" (user:p@ss@host) still strips as a whole.
	authority := s
	if firstSlash := strings.Index(s, "/"); firstSlash >= 0 {
		authority = s[:firstSlash]
	}
	if at := strings.LastIndex(authority, "@"); at >= 0 {
		rest := s[at+1:]
		colon := strings.Index(rest, ":")
		slash := strings.Index(rest, "/")
		if colon >= 0 && (slash == -1 || colon < slash) {
			// scp-like shorthand: host:path -> host/path.
			s = rest[:colon] + "/" + rest[colon+1:]
		} else {
			// Strip userinfo (user@ or user:pass@).
			s = rest
		}
	}

	s = strings.TrimSuffix(s, "/")
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimSuffix(s, "/")

	if slash := strings.Index(s, "/"); slash >= 0 {
		s = strings.ToLower(s[:slash]) + s[slash:]
	}

	return s
}

// RepoRemote reads origin's remote URL for the git repo rooted at root, via
// `git -C root config --get remote.origin.url`. Any failure (root is not a
// git repo, no origin remote configured, git missing) returns "" rather
// than an error: identity is best-effort by design, and a repo without a
// readable remote simply lands in the unregistered bucket. The result is
// git's raw output, not normalized; pass it through NormalizeRemote.
func RepoRemote(ctx context.Context, root string) string {
	out, err := gitCommand(ctx, root, "config", "--get", "remote.origin.url").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
