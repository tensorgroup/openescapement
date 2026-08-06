package engine

import (
	"context"

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
//
// Deliberately excluded: a generated-at timestamp and an esc version field.
// Both were considered and rejected — the global contract requires
// deterministic output and the test strategy is golden files, so either
// field would either break every golden or force a normalization step that
// weakens what the goldens actually prove. A publisher that needs
// generation-time provenance can stamp its own receipt time; that is a
// property of ingestion, not of this document.
type Report struct {
	Schema int `json:"schema"`
	// Command names which esc command produced this document: "status" or
	// "sync". Without it, a sync that skipped nothing is byte-identical to
	// a status document, and any consumer that persists, batches, or
	// forwards these documents — exactly what a contract meant to travel
	// has to survive — loses that distinction the moment the document
	// leaves the process that generated it.
	Command    string       `json:"command"`
	Packs      []ReportPack `json:"packs"`
	Findings   []Finding    `json:"findings"`
	Collection Collection   `json:"collection"`
	// Skipped is a pointer so status (sync == nil) omits the key entirely
	// while sync always emits the array, even empty ([]), rather than
	// omitting it when nothing was skipped. json's omitempty treats a nil
	// slice and an empty non-nil slice identically (both "empty", both
	// dropped), so a plain []Skipped could not tell those two cases apart;
	// a nil *[]Skipped is empty (omitted) and a non-nil pointer to an empty
	// slice is not (renders "[]"). Command already carries this
	// distinction too — this is belt-and-suspenders, not the only signal.
	Skipped *[]Skipped `json:"skipped,omitempty"`
}

// NewReport assembles the document. sync is nil for a status report;
// non-nil marks this as a sync report (Command: "sync") and its Skipped
// entries are copied in as an always-present (possibly empty) array.
func NewReport(st *StatusResult, coll Collection, sync *SyncResult) *Report {
	r := &Report{
		Schema:     1,
		Command:    "status",
		Collection: coll,
		Packs:      []ReportPack{},
		Findings:   append([]Finding{}, st.Findings...), // never nil: see Report.Findings' contract
	}
	if st.Plan != nil {
		for i, lp := range st.Plan.Packs {
			r.Packs = append(r.Packs, ReportPack{
				Source: lp.Source, Ref: lp.Ref, Pinned: lp.Hash,
				Latest: st.LatestBySource[lp.Source],
				Signed: packSigned(st.Plan.Config, i),
			})
		}
	}
	if sync != nil {
		r.Command = "sync"
		skipped := append([]Skipped{}, sync.Skipped...) // never nil: see Report.Skipped's contract
		r.Skipped = &skipped
	}
	return r
}

// PopulateDiffs fills Alteration.Diff on every Altered block or file finding
// in rep.Findings, via the shared artifactDiff helper (diff.go) that
// DriftDiff also uses. It is a separate pass from NewReport — which stays a
// pure function of its three arguments — because it needs ctx and root to
// do that shell-out, and because human output does not want to pay for it
// (see cmdStatus/cmdSync's --json-only call site).
//
// Diff is populated for KindBlock and KindFile only:
//
//   - KindDir is skipped because its ExpectedHash/ActualHash pair is a
//     whole-tree hash (pack.DirHashOf over the artifact's Files), not a
//     single file's content hash. A `git diff` between two directory path
//     strings would not be an empty diff, it would be a meaningless one —
//     exactly the trap this function must not fall into.
//   - KindJSONKeys is also skipped, and for a sharper reason than "out of
//     scope": a whole-file .mcp.json diff would ship the user's own
//     UNOWNED MCP server entries — arbitrary local configuration escapement
//     has no claim over — into a document a publisher consumes. That is a
//     privacy leak, not a nicety to add later, and it is why this case
//     must stay skipped even though computing *a* diff here would be easy.
//     (For completeness: the owned-only hash is resolved from the union of
//     the plan's desired keys and the lockfile's last-synced keys — the
//     same reconciliation classify's KindJSONKeys case uses so a
//     pack-dropped server isn't misattributed to the team before the next
//     sync removes it — so even a "diff just the owned subset" version
//     would have to duplicate that union outside classify to stay
//     accurate. Nobody should build that here; if a future task wants an
//     owned-subset diff, it belongs next to classify, not as a workaround
//     in this function.) ExpectedHash/ActualHash still identify the
//     alteration; only the human-readable Diff is left empty.
func PopulateDiffs(ctx context.Context, root string, plan *PlanResult, rep *Report) error {
	if plan == nil || rep == nil {
		return nil
	}
	byPath := make(map[string]Artifact, len(plan.Artifacts))
	for _, a := range plan.Artifacts {
		byPath[a.Path] = a
	}
	for i := range rep.Findings {
		f := &rep.Findings[i]
		if f.State != Altered || f.Alteration == nil {
			continue
		}
		if f.Kind != KindBlock && f.Kind != KindFile {
			continue
		}
		a, ok := byPath[f.Subject]
		if !ok {
			continue
		}
		d, err := artifactDiff(ctx, root, a)
		if err != nil {
			return err
		}
		f.Alteration.Diff = d
	}
	return nil
}

// packSigned reports whether the i'th pack pin's configured trust setting
// is the signed default. Paired with st.Plan.Config.Packs by index, not by
// matching Source: planFromConfig appends res.Packs in cfg.Packs order
// (engine.go), so the indices already correspond 1:1, and two pack refs
// that happen to share a Source (e.g. the same source pinned at two
// different refs is not possible today, but nothing enforces uniqueness)
// would otherwise make a Source-keyed lookup silently pair the wrong
// Trust. Fails closed: an out-of-range index (a defensive case that should
// never happen if the two slices are really parallel) reports false
// (unsigned), never true — matching the security posture of "assume the
// most cautious answer when the data doesn't line up" that the rest of
// this package uses (see verifyTrust's fail-closed default in engine.go).
// Trust == "unsigned" is the only value that skips signature verification
// (see verifyTrust); every other value, including the empty default, means
// signed.
func packSigned(cfg *config.Config, i int) bool {
	if cfg == nil || i < 0 || i >= len(cfg.Packs) {
		return false
	}
	return cfg.Packs[i].Trust != "unsigned"
}
