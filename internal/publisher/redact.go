package publisher

import (
	"encoding/json"

	"github.com/tensorgroup/openescapement/internal/engine"
)

// Redact returns a deep copy of rep, its Findings' Amendment.Content and
// Alteration.Diff stripped unless level is engine.ReportContent. It never
// mutates rep. engine.ReportOff is not level's concern here: a report is
// only ever handed to Redact because some endpoint is about to receive it,
// and that decision belongs to the caller, not to this function.
//
// At engine.ReportContent the copy is returned whole. At every other
// level — engine.ReportMetrics, and fail-closed, any level Redact does not
// recognize — every Content and Diff is emptied. Both fields carry
// `omitempty` (engine.Amendment, engine.Alteration), so emptying the
// string drops the key from the wire rather than shipping an empty one.
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
