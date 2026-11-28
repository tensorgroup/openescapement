package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/config"
)

func planWithRuleFiles(files map[string]string) *PlanResult {
	res := &PlanResult{Config: &config.Config{Targets: []string{"rules"}}}
	for p, body := range files {
		res.Artifacts = append(res.Artifacts, Artifact{Path: p, Kind: KindFile, Body: body})
	}
	return res
}

func TestPolicyDiffRuleFileAddedRemovedChanged(t *testing.T) {
	ctx := context.Background()
	cur := planWithRuleFiles(map[string]string{
		".claude/rules/esc-acme-keep.md":   "keep body\n",
		".claude/rules/esc-acme-change.md": "old\n",
		".claude/rules/esc-acme-remove.md": "gone soon\n",
	})
	next := planWithRuleFiles(map[string]string{
		".claude/rules/esc-acme-keep.md":   "keep body\n",
		".claude/rules/esc-acme-change.md": "new\n",
		".claude/rules/esc-acme-add.md":    "brand new\n",
	})
	out, err := PolicyDiff(ctx, cur, next)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	for _, want := range []string{"esc-acme-change.md", "esc-acme-remove.md", "esc-acme-add.md"} {
		if !strings.Contains(out, want) {
			t.Errorf("diff missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "esc-acme-keep.md") {
		t.Errorf("unchanged rule file must not appear:\n%s", out)
	}
}

func TestPolicyDiffRulePathsChangeShows(t *testing.T) {
	ctx := context.Background()
	cur := planWithRuleFiles(map[string]string{".claude/rules/esc-acme-api.md": "---\npaths:\n    - src/**\n---\nb\n"})
	next := planWithRuleFiles(map[string]string{".claude/rules/esc-acme-api.md": "---\npaths:\n    - cmd/**\n---\nb\n"})
	out, err := PolicyDiff(ctx, cur, next)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if !strings.Contains(out, "esc-acme-api.md") {
		t.Errorf("a paths change must appear:\n%s", out)
	}
}
