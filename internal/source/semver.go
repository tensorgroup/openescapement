package source

import (
	"strconv"
	"strings"
)

// semver is the parsed core of a semantic version tag. Build metadata is
// discarded at parse time: semver §10 says it must be ignored for ordering.
type semver struct {
	major, minor, patch int
	pre                 []string // empty = a release, which sorts above any prerelease
}

// parseSemver parses "v1.2.3", "1.2.3", "1.2.3-rc.1+meta". Anything that is
// not exactly major.minor.patch (all numeric) with an optional prerelease is
// not semver and reports false — outdated/update-skill must simply ignore
// such tags rather than guess.
func parseSemver(s string) (semver, bool) {
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	var pre []string
	if i := strings.IndexByte(s, '-'); i >= 0 {
		pre = strings.Split(s[i+1:], ".")
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return semver{}, false
	}
	var nums [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || (len(p) > 1 && p[0] == '0') {
			return semver{}, false
		}
		nums[i] = n
	}
	return semver{nums[0], nums[1], nums[2], pre}, true
}

// compareSemver returns -1, 0, or 1, per semver §11: numeric core first;
// a release outranks any prerelease of the same core; prerelease segments
// compare numerically when both numeric, lexically otherwise, and a numeric
// segment is always lower than an alphanumeric one.
func compareSemver(a, b semver) int {
	for _, d := range [3]int{a.major - b.major, a.minor - b.minor, a.patch - b.patch} {
		if d < 0 {
			return -1
		}
		if d > 0 {
			return 1
		}
	}
	switch {
	case len(a.pre) == 0 && len(b.pre) == 0:
		return 0
	case len(a.pre) == 0:
		return 1
	case len(b.pre) == 0:
		return -1
	}
	for i := 0; i < len(a.pre) && i < len(b.pre); i++ {
		an, aErr := strconv.Atoi(a.pre[i])
		bn, bErr := strconv.Atoi(b.pre[i])
		aNum, bNum := aErr == nil, bErr == nil
		switch {
		case aNum && bNum:
			if an != bn {
				if an < bn {
					return -1
				}
				return 1
			}
		case aNum:
			return -1 // numeric identifiers sort below alphanumeric
		case bNum:
			return 1
		default:
			if c := strings.Compare(a.pre[i], b.pre[i]); c != 0 {
				return c
			}
		}
	}
	switch {
	case len(a.pre) < len(b.pre):
		return -1
	case len(a.pre) > len(b.pre):
		return 1
	}
	return 0
}

// HighestSemverTag returns the highest semantic-version tag in tags
// (name -> commit, as returned by LsRemoteTags), ignoring non-semver names.
// Ties (e.g. "1.0.0" and "v1.0.0") break on the lexicographically larger
// tag name so the result is deterministic regardless of map order.
func HighestSemverTag(tags map[string]string) (name, commit string, ok bool) {
	var best semver
	for n, c := range tags {
		v, isSemver := parseSemver(n)
		if !isSemver {
			continue
		}
		cmp := 1
		if ok {
			cmp = compareSemver(v, best)
		}
		if cmp > 0 || (cmp == 0 && n > name) {
			best, name, commit, ok = v, n, c, true
		}
	}
	return name, commit, ok
}
