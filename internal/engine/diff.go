package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/render"
)

// DriftDiff returns a unified diff of expected vs actual for every planned
// artifact that differs (block/file kinds). Empty string means no drift.
func DriftDiff(ctx context.Context, root string, plan *PlanResult) (string, error) {
	var out strings.Builder
	for _, a := range plan.Artifacts {
		if a.Kind != KindBlock && a.Kind != KindFile {
			continue
		}
		expected, err := prospectiveContent(root, a, plan.PackObjs)
		if err != nil {
			return "", err
		}
		actual, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(a.Path)))
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if string(actual) == string(expected) {
			continue
		}
		d, err := gitDiff(ctx, actual, expected, a.Path)
		if err != nil {
			return "", err
		}
		out.WriteString(d)
	}
	return out.String(), nil
}

// PolicyDiff renders the policy text of the configured agent-file targets
// from two plans (current pins vs alternate ref) and diffs them.
func PolicyDiff(ctx context.Context, cur, next *PlanResult) (string, error) {
	var out strings.Builder
	targets := cur.Config.Targets
	if len(targets) == 0 {
		targets = []string{render.TargetClaude, render.TargetAgents, render.TargetGemini, render.TargetGovernance}
	}
	for _, t := range targets {
		if render.TargetFile[t] == "" {
			continue // skills/mcp targets have no single policy text to diff
		}
		var a, b string
		if t == render.TargetGovernance {
			a, b = render.Governance(cur.PackObjs), render.Governance(next.PackObjs)
		} else {
			a, b = render.Compose(cur.PackObjs, t), render.Compose(next.PackObjs, t)
		}
		if a == b {
			continue
		}
		d, err := gitDiff(ctx, []byte(a), []byte(b), render.TargetFile[t])
		if err != nil {
			return "", err
		}
		out.WriteString(d)
	}
	return out.String(), nil
}

// PlanWithRef re-plans with one source pinned to a different ref.
func PlanWithRef(ctx context.Context, root, sourceFilter, ref string) (*PlanResult, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return nil, err
	}
	idx, err := SelectSource(cfg, sourceFilter)
	if err != nil {
		return nil, err
	}
	// In-memory copy only — the on-disk config is never touched by diff.
	alt := *cfg
	alt.Packs = append([]config.PackRef(nil), cfg.Packs...)
	alt.Packs[idx].Ref = ref
	return planFromConfig(ctx, root, &alt)
}

// SelectSource picks the pack index matching filter; with one pack and no
// filter it picks that pack.
func SelectSource(cfg *config.Config, filter string) (int, error) {
	if filter == "" {
		if len(cfg.Packs) == 1 {
			return 0, nil
		}
		return 0, fmt.Errorf("multiple packs configured; use --source to pick one")
	}
	for i, p := range cfg.Packs {
		if p.Source == filter || strings.Contains(p.Source, filter) {
			return i, nil
		}
	}
	return 0, fmt.Errorf("no configured pack matches source %q", filter)
}

// gitDiff shells out to `git diff --no-index` for readable unified output.
func gitDiff(ctx context.Context, a, b []byte, label string) (string, error) {
	dir, err := os.MkdirTemp("", "esc-diff-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	fa := filepath.Join(dir, "actual", filepath.FromSlash(label))
	fb := filepath.Join(dir, "expected", filepath.FromSlash(label))
	for f, content := range map[string][]byte{fa: a, fb: b} {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(f, content, 0o644); err != nil {
			return "", err
		}
	}
	cmd := exec.CommandContext(ctx, "git", "-c", "core.pager=cat", "diff", "--no-index", "--src-prefix=actual/", "--dst-prefix=expected/", "--", fa, fb)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			err = nil // exit 1 = files differ, which is the point
		}
		if err != nil {
			return "", fmt.Errorf("git diff: %v", err)
		}
	}
	return string(out), nil
}
