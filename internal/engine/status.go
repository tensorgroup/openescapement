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
	Altered            State = "altered"
	Missing            State = "missing"
	Stale              State = "stale"
	ConstraintViolated State = "constraint-violated"
	PackStale          State = "pack-stale"
	CheckOverdue       State = "check-overdue"
	Orphan             State = "orphan"
)

// Finding is one classified artifact (or pack pin) in a status report.
//
// Managed and local are orthogonal axes: State/Detail/Alteration describe
// content escapement owns, Local/Amendment describe content sharing the
// artifact that it does not own. A team can both append its own rules and
// carry an inadvertent edit inside a managed block at the same time, and one
// enum would force dropping one of those facts.
type Finding struct {
	Path       string      `json:"path"`
	Kind       string      `json:"kind,omitempty"`
	State      State       `json:"managed"`
	Local      LocalState  `json:"local"`
	Detail     string      `json:"detail,omitempty"`
	Amendment  *Amendment  `json:"amendment,omitempty"`
	Alteration *Alteration `json:"alteration,omitempty"`
}

// StatusResult is the full drift report for a governed repo.
type StatusResult struct {
	Findings []Finding
	Plan     *PlanResult
}

// Clean reports whether every artifact is in sync and no constraint is
// violated. It consults f.State only: f.Local is a separate axis, and a
// local amendment alone is expected behavior, not drift. A team that
// appended its own rules must not flip this to unclean or change any exit
// code.
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

	// Orphan managed blocks: files that carry an esc block for a target no
	// longer in the effective set (pack dropped it, repo un-acknowledged it, or
	// a filter excludes it). Reported here and removed by the next sync.
	desiredBlocks := map[string]bool{}
	for _, a := range plan.Artifacts {
		if a.Kind == KindBlock {
			desiredBlocks[a.Path] = true
		}
	}
	if lock != nil {
		for _, la := range lock.Artifacts {
			if la.Kind != KindBlock || desiredBlocks[la.Path] {
				continue
			}
			content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(la.Path)))
			if err != nil {
				continue // already gone
			}
			if block, err := render.Extract(content); err == nil && block != nil {
				res.Findings = append(res.Findings, Finding{
					Path: la.Path, State: Orphan,
					Detail: "carries an esc block for a target no longer in the effective set — run `esc sync` to remove it",
				})
			}
		}
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
// from "human edited the managed content" (altered). It also reports the
// orthogonal local axis for kinds where escapement shares the artifact with
// content it does not own (KindBlock, KindJSONKeys); KindFile and KindDir
// never carry a local amendment here (KindDir is Task 6's job, once the
// per-file manifest lands).
func classify(root string, a Artifact, lock *lockfile.Lock) Finding {
	abs := filepath.Join(root, filepath.FromSlash(a.Path))
	locked := lock.Artifact(a.Path)
	staleOrAltered := func(actualHash, detail string) (State, string) {
		if locked != nil && locked.Hash == actualHash {
			return Stale, "rendered from an older pack state — run `esc sync`"
		}
		return Altered, detail
	}

	switch a.Kind {
	case KindBlock:
		content, err := os.ReadFile(abs)
		if err != nil {
			return Finding{Path: a.Path, Kind: a.Kind, State: Missing, Detail: "file does not exist — run `esc sync`"}
		}
		block, err := render.Extract(content)
		if err != nil {
			return Finding{Path: a.Path, Kind: a.Kind, State: Altered, Detail: err.Error()}
		}
		if block == nil {
			return Finding{Path: a.Path, Kind: a.Kind, State: Missing, Detail: "no managed block — run `esc sync`"}
		}
		f := Finding{Path: a.Path, Kind: a.Kind, Local: LocalNone}
		if surround, serr := render.BlockSurround(content); serr == nil {
			if am := newAmendment(surround, nil); am != nil {
				f.Local, f.Amendment = LocalAmended, am
			}
		}
		actual := render.BodyHash(block.Body)
		if actual == a.Hash {
			f.State = InSync
			return f
		}
		f.State, f.Detail = staleOrAltered(actual, "managed block was hand-edited (hash mismatch)")
		if f.State == Altered {
			f.Alteration = &Alteration{ExpectedHash: a.Hash, ActualHash: actual}
		}
		return f
	case KindFile:
		content, err := os.ReadFile(abs)
		if err != nil {
			return Finding{Path: a.Path, Kind: a.Kind, State: Missing, Detail: "file does not exist — run `esc sync`"}
		}
		f := Finding{Path: a.Path, Kind: a.Kind, Local: LocalNone}
		actual := esc.HashBytes(content)
		if actual == a.Hash {
			f.State = InSync
			return f
		}
		f.State, f.Detail = staleOrAltered(actual, "file was hand-edited (hash mismatch)")
		if f.State == Altered {
			f.Alteration = &Alteration{ExpectedHash: a.Hash, ActualHash: actual}
		}
		return f
	case KindDir:
		if info, err := os.Stat(abs); err != nil || !info.IsDir() {
			return Finding{Path: a.Path, Kind: a.Kind, State: Missing, Detail: "skill directory missing — run `esc sync`"}
		}
		// Local is deliberately LocalNone here: Task 6 adds directory
		// amendment detection once Task 5's per-file manifest exists to
		// distinguish escapement's files from ones a team added.
		f := Finding{Path: a.Path, Kind: a.Kind, Local: LocalNone}
		actual, err := pack.DirHash(abs)
		if err != nil {
			f.State, f.Detail = Altered, err.Error()
			return f
		}
		if actual == a.Hash {
			f.State = InSync
			return f
		}
		f.State, f.Detail = staleOrAltered(actual, "skill directory was modified")
		if f.State == Altered {
			f.Alteration = &Alteration{ExpectedHash: a.Hash, ActualHash: actual}
		}
		return f
	case KindJSONKeys:
		content, err := os.ReadFile(abs)
		if err != nil {
			return Finding{Path: a.Path, Kind: a.Kind, State: Missing, Detail: "file does not exist — run `esc sync`"}
		}
		f := Finding{Path: a.Path, Kind: a.Kind, Local: LocalNone}
		if names, nerr := render.UnownedMCPServers(content, a.Keys); nerr == nil {
			if am := newAmendment("", names); am != nil {
				f.Local, f.Amendment = LocalAmended, am
			}
		}
		actual, err := render.OwnedMCPHash(content, a.Keys)
		if err != nil {
			f.State, f.Detail = Altered, err.Error()
			return f
		}
		if actual == a.Hash {
			f.State = InSync
			return f
		}
		f.State, f.Detail = staleOrAltered(actual, "managed mcp server entries were modified")
		if f.State == Altered {
			f.Alteration = &Alteration{ExpectedHash: a.Hash, ActualHash: actual}
		}
		return f
	}
	return Finding{Path: a.Path, Kind: a.Kind, State: Altered, Detail: "unknown artifact kind " + a.Kind}
}
