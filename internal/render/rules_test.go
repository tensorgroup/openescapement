package render_test

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/tensorgroup/openescapement/internal/pack"
	"github.com/tensorgroup/openescapement/internal/render"
)

func TestRenderRuleNoPathsExact(t *testing.T) {
	f := pack.Fragment{Path: "rules/api.md", Targets: []string{"rules"}, Body: "# API\n\ndo the thing\n"}
	got := render.RenderRule(f, "acme")
	want := "<!-- Managed by escapement (pack acme). Do not edit. -->\n\n# API\n\ndo the thing\n"
	if got != want {
		t.Errorf("no-paths render:\n got %q\nwant %q", got, want)
	}
}

func TestRenderRulePathsFrontmatterDeterministicAndAtByte0(t *testing.T) {
	paths := []string{"src/api/**", "cmd/*.go"}
	f := pack.Fragment{Path: "rules/api.md", Targets: []string{"rules"}, Paths: paths, Body: "body\n"}
	got := render.RenderRule(f, "acme")
	if !strings.HasPrefix(got, "---\n") {
		t.Fatalf("frontmatter must be at byte 0:\n%q", got)
	}
	fm, _ := yaml.Marshal(struct {
		Paths []string `yaml:"paths"`
	}{paths})
	want := "---\n" + string(fm) + "---\n<!-- Managed by escapement (pack acme). Do not edit. -->\n\nbody\n"
	if got != want {
		t.Errorf("paths render:\n got %q\nwant %q", got, want)
	}
	if render.RenderRule(f, "acme") != got {
		t.Error("RenderRule not deterministic")
	}
}

func TestRenderRuleQuotesRoundTrip(t *testing.T) {
	paths := []string{`a"quote`, `a\backslash`}
	f := pack.Fragment{Path: "rules/x.md", Targets: []string{"rules"}, Paths: paths, Body: "b\n"}
	got := render.RenderRule(f, "acme")
	fmEnd := strings.Index(got[4:], "\n---\n")
	var parsed struct {
		Paths []string `yaml:"paths"`
	}
	if err := yaml.Unmarshal([]byte(got[4:4+fmEnd]), &parsed); err != nil {
		t.Fatalf("frontmatter did not parse: %v\n%q", err, got)
	}
	if len(parsed.Paths) != 2 || parsed.Paths[0] != `a"quote` || parsed.Paths[1] != `a\backslash` {
		t.Errorf("paths did not round-trip: %v", parsed.Paths)
	}
}

func TestRenderRuleCRLFBodyNormalized(t *testing.T) {
	f := pack.Fragment{Path: "rules/x.md", Targets: []string{"rules"}, Body: "a\r\nb\r\n"}
	got := render.RenderRule(f, "acme")
	if strings.Contains(got, "\r") || !strings.HasSuffix(got, "a\nb\n") {
		t.Errorf("CRLF body must normalize: %q", got)
	}
}

func TestRenderRuleNoTrailingNewlineBodyGetsOne(t *testing.T) {
	f := pack.Fragment{Path: "rules/x.md", Targets: []string{"rules"}, Body: "no newline"}
	if got := render.RenderRule(f, "acme"); !strings.HasSuffix(got, "no newline\n") {
		t.Errorf("must end with exactly one newline: %q", got)
	}
}

func TestRuleFilePath(t *testing.T) {
	f := pack.Fragment{Path: "rules/api-handlers.md"}
	if p := render.RuleFilePath("acme-org", f); p != ".claude/rules/esc-acme-org-api-handlers.md" {
		t.Errorf("path = %q", p)
	}
}
