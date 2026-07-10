// Package updatecheck implements the client-side update-freshness check:
// cadence math, a git ls-remote staleness probe, a per-clone JSONL log, the
// invocation throttle, and the sync-time recorder. It is inert unless an
// installed pack declares update_check.
package updatecheck

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/lockfile"
	"github.com/tensorgroup/openescapement/internal/pack"
	"github.com/tensorgroup/openescapement/internal/source"
)

// checkTimeout bounds the ls-remote probe so a dead remote can never stall an
// interactive command.
const checkTimeout = 10 * time.Second

// Outcome classifies one check attempt.
type Outcome string

const (
	OutcomeOKCurrent Outcome = "ok_current"
	OutcomeOKUpdates Outcome = "ok_updates"
	OutcomeError     Outcome = "error"
)

// PackStatus is the pinned-vs-latest comparison for one pack.
type PackStatus struct {
	Source  string `json:"source"`
	Kind    string `json:"kind"` // "tag" | "branch"
	Pinned  string `json:"pinned"`
	Latest  string `json:"latest"`
	Updates bool   `json:"updates"`
}

// Cadence returns the strictest (shortest) declared cadence across packs and
// its raw string. Returns (0, "") when no pack declares update_check.
func Cadence(packs []*pack.Pack) (time.Duration, string) {
	var min time.Duration
	var str string
	for _, p := range packs {
		if p.Manifest.UpdateCheck == nil {
			continue
		}
		d, err := pack.ParseEvery(p.Manifest.UpdateCheck.Every)
		if err != nil || d <= 0 {
			continue
		}
		if min == 0 || d < min {
			min, str = d, p.Manifest.UpdateCheck.Every
		}
	}
	return min, str
}

// check probes each non-local pack's git source for newer versions. It fetches
// no content. A declared update_check.endpoint is intentionally ignored in
// v0.1 — the probe always uses git ls-remote.
func check(ctx context.Context, cfg *config.Config, lock *lockfile.Lock) ([]PackStatus, error) {
	var out []PackStatus
	for _, ref := range cfg.Packs {
		parsed, err := source.ParseSource(ref.Source)
		if err != nil {
			return nil, err
		}
		if parsed.Local {
			continue // dev-mode local dirs surface as ordinary drift
		}
		st := PackStatus{Source: ref.Source, Pinned: ref.Ref}
		if _, ok := parseSemver(ref.Ref); ok {
			st.Kind = "tag"
			tags, err := source.LsRemoteTags(ctx, parsed.URL)
			if err != nil {
				return nil, err
			}
			if latest, ok := maxStableSemver(tags); ok {
				st.Latest = latest
				pv, _ := parseSemver(ref.Ref)
				lv, _ := parseSemver(latest)
				st.Updates = semverLess(pv, lv)
			} else {
				st.Latest = ref.Ref
			}
		} else {
			st.Kind = "branch"
			hash, err := source.LsRemoteHash(ctx, parsed.URL, ref.Ref)
			if err != nil {
				return nil, err
			}
			st.Latest = short(hash)
			if lp := lock.Pack(ref.Source, ref.Ref); lp != nil && lp.Commit != "" && hash != "" {
				st.Updates = hash != lp.Commit
			}
		}
		out = append(out, st)
	}
	return out, nil
}

func short(hash string) string {
	if len(hash) > 12 {
		return hash[:12]
	}
	return hash
}

type semver struct {
	major, minor, patch int
	pre                 string
}

// parseSemver accepts v?MAJOR.MINOR.PATCH[-prerelease]. Anything else is not a
// tag pin (branch/SHA) and returns ok=false.
func parseSemver(s string) (semver, bool) {
	s = strings.TrimPrefix(s, "v")
	core, pre := s, ""
	if i := strings.IndexByte(s, '-'); i >= 0 {
		core, pre = s[:i], s[i+1:]
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return semver{}, false
	}
	var n [3]int
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil || v < 0 {
			return semver{}, false
		}
		n[i] = v
	}
	return semver{n[0], n[1], n[2], pre}, true
}

// semverLess reports a < b. For equal cores, a prerelease is lower than the
// bare release; two prereleases compare lexically (documented approximation).
func semverLess(a, b semver) bool {
	if a.major != b.major {
		return a.major < b.major
	}
	if a.minor != b.minor {
		return a.minor < b.minor
	}
	if a.patch != b.patch {
		return a.patch < b.patch
	}
	if a.pre == b.pre {
		return false
	}
	if a.pre == "" {
		return false // a is release; not less than b
	}
	if b.pre == "" {
		return true // a is prerelease, b is release
	}
	return a.pre < b.pre
}

// maxStableSemver returns the greatest non-prerelease tag name, or ok=false.
// Map iteration order is randomized by Go, so ties (e.g. "v2.0.0" and
// "2.0.0" both parsing to the same semver) are broken deterministically via
// preferName rather than by whichever name the iteration visits last.
func maxStableSemver(tags map[string]string) (string, bool) {
	var best semver
	var bestName string
	found := false
	for name := range tags {
		v, ok := parseSemver(name)
		if !ok || v.pre != "" {
			continue // skip non-semver and prerelease tags
		}
		switch {
		case !found:
			best, bestName, found = v, name, true
		case semverLess(best, v):
			best, bestName = v, name
		case semverLess(v, best):
			// current name is strictly greater; keep it
		case preferName(name, bestName):
			// equal semver: deterministic tie-break
			best, bestName = v, name
		}
	}
	return bestName, found
}

// preferName reports whether candidate should win a tie over current for two
// tag names that parsed to the same semver: a "v"-prefixed name is preferred,
// and otherwise the lexicographically smaller name wins. Either rule alone
// makes the winner a pure function of the tag names, independent of map
// iteration order.
func preferName(candidate, current string) bool {
	cv, cu := strings.HasPrefix(candidate, "v"), strings.HasPrefix(current, "v")
	if cv != cu {
		return cv
	}
	return candidate < current
}
