package source

import "testing"

func TestHighestSemverTag(t *testing.T) {
	cases := []struct {
		name string
		tags map[string]string
		want string
		ok   bool
	}{
		{"empty", map[string]string{}, "", false},
		{"no semver tags", map[string]string{"latest": "a", "release-1": "b"}, "", false},
		{"basic ordering", map[string]string{"v1.2.0": "a", "v1.10.0": "b", "v1.9.9": "c"}, "v1.10.0", true},
		{"v prefix optional", map[string]string{"1.0.0": "a", "v2.0.0": "b"}, "v2.0.0", true},
		{"prerelease below release", map[string]string{"v2.0.0-rc.1": "a", "v2.0.0": "b"}, "v2.0.0", true},
		{"prerelease ordering", map[string]string{"v2.0.0-alpha": "a", "v2.0.0-beta": "b"}, "v2.0.0-beta", true},
		{"numeric prerelease below alnum", map[string]string{"v1.0.0-1": "a", "v1.0.0-alpha": "b"}, "v1.0.0-alpha", true},
		{"build metadata ignored", map[string]string{"v1.0.0+build5": "a"}, "v1.0.0+build5", true},
		{"non-semver ignored beside semver", map[string]string{"nightly": "a", "v0.1.0": "b"}, "v0.1.0", true},
		{"equal versions tie-break on name", map[string]string{"1.0.0": "a", "v1.0.0": "b"}, "v1.0.0", true},
	}
	for _, c := range cases {
		name, commit, ok := HighestSemverTag(c.tags)
		if ok != c.ok || name != c.want {
			t.Errorf("%s: got (%q, ok=%v), want (%q, ok=%v)", c.name, name, ok, c.want, c.ok)
		}
		if ok && commit != c.tags[name] {
			t.Errorf("%s: commit %q does not match tags[%q]", c.name, commit, name)
		}
	}
}
