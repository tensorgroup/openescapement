package engine

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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

// Non-artifact finding kinds. Status reports on things that are not on-disk
// artifacts at all — a pack pin, the update-check subsystem, or a
// constraint violation — and Kind is a total discriminator (see Finding's
// doc comment): every Finding, artifact or not, carries one of these or one
// of the Kind* artifact constants in engine.go. Defined here rather than
// alongside KindBlock etc. because these three have no corresponding
// Artifact — they exist only as Finding.Kind values.
const (
	KindPack        = "pack"         // a pack pin, not a file on disk (Subject is the pack source)
	KindUpdateCheck = "update-check" // the update-check subsystem itself (Subject is the literal "update-check")
	KindConstraint  = "constraint"   // a constraint violation (Subject is the violated path, which may or may not be a tracked artifact)
)

// Finding is one classified artifact, pack pin, or subsystem-level signal in
// a status report.
//
// Kind is a total discriminator: every Finding carries one of the Kind*
// artifact constants (block/file/dir/json-keys) when Subject names a real
// on-disk artifact, or one of KindPack/KindUpdateCheck/KindConstraint above
// when it does not. Splitting artifact and non-artifact findings into two
// separate arrays was considered and rejected: a consumer's core question
// ("is this repo compliant?") would then require merging two arrays
// forever, and the boundary is arbitrary anyway — a constraint-violated
// finding can point at a real artifact file. One array, always kinded, is
// the simpler contract.
//
// Managed and local are orthogonal axes: State/Detail/Alteration describe
// content escapement owns, Local/Amendment describe content sharing the
// artifact that it does not own. A team can both append its own rules and
// carry an inadvertent edit inside a managed block at the same time, and one
// enum would force dropping one of those facts.
type Finding struct {
	// Subject is what this finding is about; interpretation is determined
	// by Kind. For an artifact Kind (block/file/dir/json-keys) it is the
	// artifact's repo-relative path. For KindPack it is the pack source
	// URL. For KindUpdateCheck it is the literal string "update-check" —
	// there is no on-disk file to name, the finding is about the
	// subsystem itself. For KindConstraint it is the violated path, which
	// may or may not coincide with a tracked artifact.
	Subject    string      `json:"subject"`
	Kind       string      `json:"kind"`
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
	// LatestBySource carries the newest successful update-check's per-pack
	// latest-version string, keyed by pack source. Populated only when some
	// pack declares update_check (the same gate that produces PackStale
	// findings below); nil otherwise. NewReport reads this to fill
	// ReportPack.Latest without a second disk read or any network probe.
	LatestBySource map[string]string
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
				Subject: lp.Source, Kind: KindPack, State: Stale, Local: LocalNone,
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
				// la.Kind is guaranteed KindBlock by the filter above:
				// orphan detection only ever walks block-kind lock entries.
				f := Finding{
					Subject: la.Path, Kind: la.Kind, State: Orphan, Local: LocalNone,
					Detail: "carries an esc block for a target no longer in the effective set — run `esc sync` to remove it",
				}
				// Orphans get both axes, like every other finding. Local was
				// hard-coded to LocalNone here, which made status claim a
				// file had no local content in the one report a user reads
				// before letting sync delete part of it.
				if surround, serr := render.BlockSurround(content); serr == nil {
					if am := newAmendment(surround, nil); am != nil {
						f.Local, f.Amendment = LocalAmended, am
					}
				}
				// A hand-edit inside an orphaned block is what Apply's skip
				// gate declines to delete, so status has to say so first:
				// otherwise the user's only signal is a directory-style
				// "run sync to remove it" for something sync will then
				// refuse to remove.
				if actual := render.BodyHash(block.Body); actual != la.Hash {
					f.Alteration = &Alteration{ExpectedHash: la.Hash, ActualHash: actual}
					f.Detail = "carries a hand-edited esc block for a target no longer in the effective set; `esc sync` leaves it in place, `esc sync --force` removes it"
				}
				res.Findings = append(res.Findings, f)
			}
		}
	}

	// Orphan skill directories: an escapement-owned dir the lockfile records
	// that the effective set no longer contains. Same shape and vocabulary as
	// the orphan-block pass above, and for the same reason spelled out there:
	// Apply's orphan-dir pass can DECLINE to retire one of these (it still
	// holds files the team added, or a pack-provided file in it was
	// hand-edited), and status has to say so first. Until this existed, dir
	// entries were filtered out of orphan detection entirely, so no orphan-dir
	// finding was produced in any state — a repo parked in a declined
	// retirement showed nothing on `esc status` and passed `esc status
	// --check` at exit 0 forever, which is precisely where AGENTS.md puts
	// compliance gating.
	desiredDirs := map[string]bool{}
	for _, a := range plan.Artifacts {
		if a.Kind == KindDir {
			desiredDirs[a.Path] = true
		}
	}
	if lock != nil {
		for _, la := range lock.Artifacts {
			if la.Kind != KindDir || desiredDirs[la.Path] {
				continue
			}
			abs, cerr := containedPath(root, la.Path)
			if cerr != nil {
				continue
			}
			// Read-only surface: a lockfile path reaching out of the repo
			// through a symlink is refused rather than followed to build a
			// report. Apply fails closed and loudly on the same entry, which
			// is the signal that case gets.
			if refuseSymlinks(root, la.Path) != nil {
				continue
			}
			if info, serr := os.Stat(abs); serr != nil || !info.IsDir() {
				continue // already gone
			}
			f := Finding{
				Subject: la.Path, Kind: la.Kind, State: Orphan, Local: LocalNone,
				Detail: "an esc skill directory for a target no longer in the effective set — run `esc sync` to remove it",
			}
			if unmanaged, uerr := unmanagedDirFiles(abs, la.Files); uerr == nil {
				if am := newAmendment("", unmanaged); am != nil {
					f.Local, f.Amendment = LocalAmended, am
					f.Detail = "an esc skill directory for a target no longer in the effective set; `esc sync` removes the pack-provided files and keeps the directory for the files you added"
				}
			}
			// la.Files is lockfile data, so containment comes before the read
			// here for the same reason it does in Apply's orphan-dir pass:
			// DirHashOf reads every entry, and an unvalidated entry would be
			// read through a symlink or out of the root to produce a hash this
			// report then publishes.
			if len(la.Files) > 0 {
				if verr := validateDirEntries(abs, la.Path, la.Files); verr != nil {
					f.Detail = "an esc skill directory for a target no longer in the effective set, with an unusable lockfile manifest (" + verr.Error() + "); `esc sync` fails closed on it"
				} else if actual, herr := pack.DirHashOf(abs, la.Files); herr != nil || actual != la.Hash {
					f.Alteration = &Alteration{ExpectedHash: la.Hash, ActualHash: actual}
					f.Detail = "an esc skill directory for a target no longer in the effective set, holding a pack-provided file that was hand-edited or is no longer readable; `esc sync` leaves it in place, `esc sync --force` removes it"
				}
			}
			res.Findings = append(res.Findings, f)
		}
	}

	for _, v := range plan.Violations {
		res.Findings = append(res.Findings, Finding{Subject: v.Path, Kind: KindConstraint, State: ConstraintViolated, Local: LocalNone, Detail: v.Rule})
	}

	// Update-freshness findings (inert unless a pack declares update_check).
	if cadence, _ := updatecheck.Cadence(plan.PackObjs); cadence > 0 {
		entries, _ := updatecheck.LoadLog(root)
		ls := updatecheck.LastSuccess(entries)
		if ls != nil {
			res.LatestBySource = make(map[string]string, len(ls.Packs))
			for _, ps := range ls.Packs {
				res.LatestBySource[ps.Source] = ps.Latest
				if ps.Updates {
					res.Findings = append(res.Findings, Finding{
						Subject: ps.Source,
						Kind:    KindPack,
						State:   PackStale,
						Local:   LocalNone,
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
				Subject: "update-check",
				Kind:    KindUpdateCheck,
				State:   CheckOverdue,
				Local:   LocalNone,
				Detail:  "no successful update check within cadence and the latest attempt failed",
			})
		}
	}
	return res, nil
}

// classify compares one artifact's desired state against disk, using the
// lockfile to distinguish "pack moved on, file matches old sync" (stale)
// from "human edited the managed content" (altered). It also reports the
// orthogonal local axis for kinds where escapement shares the artifact with
// content it does not own (KindBlock, KindJSONKeys, KindDir); KindFile never
// carries a local amendment here since it has no notion of a shared
// container.
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
			return Finding{Subject: a.Path, Kind: a.Kind, State: Missing, Local: LocalNone, Detail: "file does not exist — run `esc sync`"}
		}
		block, err := render.Extract(content)
		if err != nil {
			return Finding{Subject: a.Path, Kind: a.Kind, State: Altered, Local: LocalNone, Detail: err.Error()}
		}
		if block == nil {
			return Finding{Subject: a.Path, Kind: a.Kind, State: Missing, Local: LocalNone, Detail: "no managed block — run `esc sync`"}
		}
		f := Finding{Subject: a.Path, Kind: a.Kind, Local: LocalNone}
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
			return Finding{Subject: a.Path, Kind: a.Kind, State: Missing, Local: LocalNone, Detail: "file does not exist — run `esc sync`"}
		}
		f := Finding{Subject: a.Path, Kind: a.Kind, Local: LocalNone}
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
			return Finding{Subject: a.Path, Kind: a.Kind, State: Missing, Local: LocalNone, Detail: "skill directory missing — run `esc sync`"}
		}
		f := Finding{Subject: a.Path, Kind: a.Kind, Local: LocalNone}
		unmanaged, err := unmanagedDirFiles(abs, a.Files)
		if err != nil {
			return Finding{Subject: a.Path, Kind: a.Kind, State: Altered, Local: LocalNone, Detail: err.Error()}
		}
		if am := newAmendment("", unmanaged); am != nil {
			f.Local, f.Amendment = LocalAmended, am
		}
		// The managed hash covers only a.Files (the pack-provided paths), not
		// the whole tree: a file the team added under this directory must
		// report as an amendment above, not flip the entire directory to
		// altered.
		actual, err := pack.DirHashOf(abs, a.Files)
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
			return Finding{Subject: a.Path, Kind: a.Kind, State: Missing, Local: LocalNone, Detail: "file does not exist — run `esc sync`"}
		}
		f := Finding{Subject: a.Path, Kind: a.Kind, Local: LocalNone}
		// A server the pack no longer declares is still escapement's own
		// content until the next sync removes it (that gap shows up as
		// Stale, below, not Altered). Union the plan's desired keys with the
		// lockfile's last-synced keys so a dropped-but-not-yet-reconciled
		// server is never misattributed to the team as a local amendment.
		owned := a.Keys
		if locked != nil && len(locked.Keys) > 0 {
			seen := make(map[string]bool, len(a.Keys)+len(locked.Keys))
			owned = make([]string, 0, len(a.Keys)+len(locked.Keys))
			for _, k := range a.Keys {
				if !seen[k] {
					seen[k] = true
					owned = append(owned, k)
				}
			}
			for _, k := range locked.Keys {
				if !seen[k] {
					seen[k] = true
					owned = append(owned, k)
				}
			}
		}
		if names, nerr := render.UnownedMCPServers(content, owned); nerr == nil {
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
	return Finding{Subject: a.Path, Kind: a.Kind, State: Altered, Local: LocalNone, Detail: "unknown artifact kind " + a.Kind}
}

// unmanagedDirFiles walks dir and returns the sorted, slash-separated paths
// relative to dir that are not in provided (the pack-provided file list for
// this artifact). These are files a team added under an escapement-owned
// skill directory: sync preserves them (Task 5's mergeDir), and this is what
// lets status report them as a local amendment instead of silently ignoring
// them or, worse, folding them into the managed hash and reporting the whole
// directory as altered.
func unmanagedDirFiles(dir string, provided []string) ([]string, error) {
	owned := make(map[string]bool, len(provided))
	for _, p := range provided {
		owned[p] = true
	}
	var unmanaged []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !owned[rel] {
			unmanaged = append(unmanaged, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(unmanaged)
	return unmanaged, nil
}
