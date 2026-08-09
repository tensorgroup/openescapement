package publisher

import (
	"encoding/json"

	"github.com/tensorgroup/openescapement/internal/engine"
)

// Redact returns a deep copy of rep with its human-prose fields stripped
// unless level is engine.ReportContent. It never mutates rep. engine.ReportOff
// is not level's concern here: a report is only ever handed to Redact
// because some endpoint is about to receive it, and that decision belongs
// to the caller, not to this function.
//
// At engine.ReportContent the copy is returned whole. At every other
// level — engine.ReportMetrics, and fail-closed, any level Redact does not
// recognize (an off-by-typo level, or engine.ReportOff reaching here despite
// the caller's own gate) — four fields are emptied:
//
//   - Finding.Amendment.Content, Finding.Alteration.Diff: free-text file
//     content and diffs, the reason this function exists at all.
//   - Finding.Detail: also human prose, and in several classify.go paths
//     (status.go's KindBlock/KindDir cases) it is populated directly from
//     err.Error() — an unbounded channel for whatever an underlying
//     filesystem or git error happens to say, which can embed a path
//     fragment or other local detail never meant to leave the machine.
//   - Skipped.Reason: the same free-text-from-error shape as Detail
//     (apply.go's Skipped construction sites), for the same reason.
//
// All four are prose whose structural meaning already ships in
// machine-readable form: a consumer that needs to know what happened reads
// Finding.State/Local (managed/local axes) or Skipped.Cause, not prose
// generated for a human terminal. Everything else on Finding and Skipped —
// Subject, Kind, Amendment.Items (dir/json-keys unmanaged names, not free
// text), Amendment.Bytes/Lines/Hash, Alteration.ExpectedHash/ActualHash,
// Collection, and every ReportPack pin — is exactly the metrics grade the
// amendment model promises and MUST keep surviving below content; do not
// fold any of those into this redaction pass.
//
// Content and Diff carry `omitempty` (engine.Amendment, engine.Alteration),
// as does Detail (engine.Finding), so emptying those three drops the key
// from the wire rather than shipping an empty one. Skipped.Reason
// (engine.apply.go) has no `omitempty`; an emptied Reason still ships as
// "reason":"" — cosmetic wire noise, not a leak, since the value itself is
// gone.
//
// The deep copy is a JSON marshal/unmarshal round trip through
// engine.Report: the simplest implementation that cannot alias any of
// rep's slices or pointers, so mutating the returned copy — or Redact's
// own redaction pass — can never reach the caller's original Report.
func Redact(rep *engine.Report, level string) *engine.Report {
	out := deepCopyReport(rep)
	if out == nil || level == engine.ReportContent {
		return out
	}
	for i := range out.Findings {
		f := &out.Findings[i]
		if f.Amendment != nil {
			f.Amendment.Content = ""
		}
		if f.Alteration != nil {
			f.Alteration.Diff = ""
		}
		f.Detail = ""
	}
	if out.Skipped != nil {
		for i := range *out.Skipped {
			(*out.Skipped)[i].Reason = ""
		}
	}
	return out
}

// deepCopyReport marshals and unmarshals rep through engine.Report's own
// JSON contract. engine.Report and everything it embeds is plain
// JSON-safe data (strings, ints, bools, slices, and struct pointers) with
// no channel, func, or unsafe field that could make either step fail, so
// an error here means the type has grown a field this copy no longer
// safely handles — a programmer error, not a runtime condition a caller
// can react to, hence the panic rather than a threaded error return.
func deepCopyReport(rep *engine.Report) *engine.Report {
	if rep == nil {
		return nil
	}
	b, err := json.Marshal(rep)
	if err != nil {
		panic("publisher: report failed to marshal for deep copy: " + err.Error())
	}
	out := new(engine.Report)
	if err := json.Unmarshal(b, out); err != nil {
		panic("publisher: report failed to unmarshal for deep copy: " + err.Error())
	}
	return out
}
