package engine

import (
	"context"
	"os"
	"path/filepath"

	"github.com/tensorgroup/openescapement/internal/config"
)

// ReportPack is one pack pin as reported to a consumer.
//
// Pinned carries the pack's content hash (lockfile.LockPack.Hash), not its
// Commit: Commit is empty for plain local dev packs (see LockPack's doc
// comment), so it cannot identify every pin, while Hash is always populated
// and is exactly the value Plan's lock-integrity check compares to detect a
// moved tag or tampered source — the resolved identity of what is actually
// on disk, for both git and local sources alike. Ref stays the separate,
// human-chosen pin (a tag, branch, or empty for local); Pinned is what that
// ref actually resolved to.
type ReportPack struct {
	Source string `json:"source"`
	Ref    string `json:"ref"`
	Pinned string `json:"pinned"`
	Latest string `json:"latest,omitempty"`
	Signed bool   `json:"signed"`
}

// Report is the machine-readable status document and the contract the
// telemetry publisher consumes. It always carries complete local truth; the
// reporting level is reported here but applied downstream — a local surface
// is the user's own machine and their own files, and an open-source user
// whose pack declares no reporting block at all still gets a fully useful
// local report.
type Report struct {
	Schema     int          `json:"schema"`
	Packs      []ReportPack `json:"packs"`
	Artifacts  []Finding    `json:"artifacts"`
	Collection Collection   `json:"collection"`
	Skipped    []Skipped    `json:"skipped,omitempty"`
}

// NewReport assembles the document. sync may be nil for status runs.
func NewReport(st *StatusResult, coll Collection, sync *SyncResult) *Report {
	r := &Report{Schema: 1, Collection: coll, Artifacts: st.Findings}
	if st.Plan != nil {
		for _, lp := range st.Plan.Packs {
			r.Packs = append(r.Packs, ReportPack{
				Source: lp.Source, Ref: lp.Ref, Pinned: lp.Hash,
				Latest: st.LatestBySource[lp.Source],
				Signed: packSigned(st.Plan.Config, lp.Source),
			})
		}
	}
	if sync != nil {
		r.Skipped = sync.Skipped
	}
	return r
}

// PopulateDiffs fills Alteration.Diff on every Altered block or file finding
// in rep.Artifacts, re-deriving the artifact's prospective (expected)
// content and diffing it against what is on disk now, via the same gitDiff
// shell-out DriftDiff already uses. It is a separate pass from NewReport —
// which stays a pure function of its three arguments — because it needs ctx
// and root to do that shell-out, and because human output does not want to
// pay for it (see cmdStatus/cmdSync's --json-only call site).
//
// Diff is populated for KindBlock and KindFile only:
//
//   - KindDir is skipped because its ExpectedHash/ActualHash pair is a
//     whole-tree hash (pack.DirHashOf over the artifact's Files), not a
//     single file's content hash. A `git diff` between two directory path
//     strings would not be an empty diff, it would be a meaningless one —
//     exactly the trap this function must not fall into.
//   - KindJSONKeys is also skipped. Its hash covers only the owned-key
//     subset (render.OwnedMCPHash), resolved from the union of the plan's
//     desired keys and the lockfile's last-synced keys — the same
//     locked-vs-desired reconciliation classify uses so a pack-dropped
//     server isn't misattributed to the team before the next sync removes
//     it (see classify's KindJSONKeys case). Reproducing that union here,
//     outside classify, to build an accurate "expected" document was judged
//     out of scope for this task: a naive whole-file diff would render
//     entries the team owns nothing to do with as part of the alteration,
//     the same class of misleading output the KindDir case above is called
//     out to avoid. ExpectedHash/ActualHash still identify the alteration;
//     only the human-readable Diff is left empty.
func PopulateDiffs(ctx context.Context, root string, plan *PlanResult, rep *Report) error {
	if plan == nil || rep == nil {
		return nil
	}
	byPath := make(map[string]Artifact, len(plan.Artifacts))
	for _, a := range plan.Artifacts {
		byPath[a.Path] = a
	}
	for i := range rep.Artifacts {
		f := &rep.Artifacts[i]
		if f.State != Altered || f.Alteration == nil {
			continue
		}
		if f.Kind != KindBlock && f.Kind != KindFile {
			continue
		}
		a, ok := byPath[f.Path]
		if !ok {
			continue
		}
		expected, err := prospectiveContent(root, a)
		if err != nil {
			return err
		}
		actual, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.Path)))
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if string(actual) == string(expected) {
			continue
		}
		d, err := gitDiff(ctx, actual, expected, f.Path)
		if err != nil {
			return err
		}
		f.Alteration.Diff = d
	}
	return nil
}

// packSigned reports whether source's configured trust setting is the
// signed default. Trust == "unsigned" is the only way a user explicitly
// accepts an unsigned source (see verifyTrust); every other value, including
// the empty default, means signed.
func packSigned(cfg *config.Config, source string) bool {
	if cfg == nil {
		return true
	}
	for _, ref := range cfg.Packs {
		if ref.Source == source {
			return ref.Trust != "unsigned"
		}
	}
	return true
}
