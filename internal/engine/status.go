package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/lockfile"
	"github.com/tensorgroup/openescapement/internal/pack"
	"github.com/tensorgroup/openescapement/internal/render"
	"github.com/tensorgroup/openescapement/internal/updatecheck"
)

// State classifies one artifact's condition relative to the plan.
type State string

const (
	InSync             State = "in-sync"
	Modified           State = "modified"
	Missing            State = "missing"
	Stale              State = "stale"
	ConstraintViolated State = "constraint-violated"
	PackStale          State = "pack-stale"
	CheckOverdue       State = "check-overdue"
)

// Finding is one classified artifact (or pack pin) in a status report.
type Finding struct {
	Path   string
	State  State
	Detail string
}

// StatusResult is the full drift report for a governed repo.
type StatusResult struct {
	Findings []Finding
	Plan     *PlanResult
}

// Clean reports whether every artifact is in sync and no constraint is violated.
func (s *StatusResult) Clean() bool {
	for _, f := range s.Findings {
		if f.State != InSync {
			return false
		}
	}
	return true
}

// Status runs the plan pipeline read-only and classifies every artifact.
func Status(ctx context.Context, root string) (*StatusResult, error) {
	plan, err := Plan(ctx, root)
	if err != nil {
		return nil, err
	}
	lock, err := lockfile.Load(root)
	if err != nil {
		return nil, err
	}
	res := &StatusResult{Plan: plan}

	// Config pin present but pack absent from lock → never synced at this pin.
	for _, lp := range plan.Packs {
		if lock.Pack(lp.Source, lp.Ref) == nil {
			res.Findings = append(res.Findings, Finding{
				Path: lp.Source, State: Stale,
				Detail: fmt.Sprintf("pack pin %s not in lockfile — run `esc sync`", lp.Ref),
			})
		}
	}

	for _, a := range plan.Artifacts {
		res.Findings = append(res.Findings, classify(root, a, lock))
	}
	for _, v := range plan.Violations {
		res.Findings = append(res.Findings, Finding{Path: v.Path, State: ConstraintViolated, Detail: v.Rule})
	}

	// Update-freshness findings (inert unless a pack declares update_check).
	if cadence, _ := updatecheck.Cadence(plan.PackObjs); cadence > 0 {
		entries, _ := updatecheck.LoadLog(root)
		ls := updatecheck.LastSuccess(entries)
		if ls != nil {
			for _, ps := range ls.Packs {
				if ps.Updates {
					res.Findings = append(res.Findings, Finding{
						Path:  ps.Source,
						State: PackStale,
						Detail: fmt.Sprintf("update available: %s -> %s (run `esc update` then `esc sync`)",
							ps.Pinned, ps.Latest),
					})
				}
			}
		}
		last := updatecheck.LastEntry(entries)
		overdue := ls == nil || time.Since(ls.Time) > cadence
		attemptFailed := last != nil && last.Outcome == updatecheck.OutcomeError
		if overdue && attemptFailed {
			res.Findings = append(res.Findings, Finding{
				Path:   "update-check",
				State:  CheckOverdue,
				Detail: "no successful update check within cadence and the latest attempt failed",
			})
		}
	}
	return res, nil
}

// classify compares one artifact's desired state against disk, using the
// lockfile to distinguish "pack moved on, file matches old sync" (stale)
// from "human edited the managed content" (modified).
func classify(root string, a Artifact, lock *lockfile.Lock) Finding {
	abs := filepath.Join(root, filepath.FromSlash(a.Path))
	locked := lock.Artifact(a.Path)
	staleOrModified := func(actualHash, detail string) Finding {
		if locked != nil && locked.Hash == actualHash {
			return Finding{a.Path, Stale, "rendered from an older pack state — run `esc sync`"}
		}
		return Finding{a.Path, Modified, detail}
	}

	switch a.Kind {
	case KindBlock:
		content, err := os.ReadFile(abs)
		if err != nil {
			return Finding{a.Path, Missing, "file does not exist — run `esc sync`"}
		}
		block, err := render.Extract(content)
		if err != nil {
			return Finding{a.Path, Modified, err.Error()}
		}
		if block == nil {
			return Finding{a.Path, Missing, "no managed block — run `esc sync`"}
		}
		actual := render.BodyHash(block.Body)
		if actual == a.Hash {
			return Finding{a.Path, InSync, ""}
		}
		return staleOrModified(actual, "managed block was hand-edited (hash mismatch)")
	case KindFile:
		content, err := os.ReadFile(abs)
		if err != nil {
			return Finding{a.Path, Missing, "file does not exist — run `esc sync`"}
		}
		actual := esc.HashBytes(content)
		if actual == a.Hash {
			return Finding{a.Path, InSync, ""}
		}
		return staleOrModified(actual, "file was hand-edited (hash mismatch)")
	case KindDir:
		if info, err := os.Stat(abs); err != nil || !info.IsDir() {
			return Finding{a.Path, Missing, "skill directory missing — run `esc sync`"}
		}
		actual, err := pack.DirHash(abs)
		if err != nil {
			return Finding{a.Path, Modified, err.Error()}
		}
		if actual == a.Hash {
			return Finding{a.Path, InSync, ""}
		}
		return staleOrModified(actual, "skill directory was modified")
	case KindJSONKeys:
		content, err := os.ReadFile(abs)
		if err != nil {
			return Finding{a.Path, Missing, "file does not exist — run `esc sync`"}
		}
		actual, err := render.OwnedMCPHash(content, a.Keys)
		if err != nil {
			return Finding{a.Path, Modified, err.Error()}
		}
		if actual == a.Hash {
			return Finding{a.Path, InSync, ""}
		}
		return staleOrModified(actual, "managed mcp server entries were modified")
	}
	return Finding{a.Path, Modified, "unknown artifact kind " + a.Kind}
}
