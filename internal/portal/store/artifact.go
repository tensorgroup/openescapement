package store

import "github.com/tensorgroup/openescapement/internal/engine"

// EventArtifact is one artifact's governance posture within an event.
// Managed mirrors escapement's managed axis (in-sync|altered|stale|missing|
// orphan); Local mirrors the local axis (none|amended). Amendment and
// Alteration reuse engine's own payload types rather than redefining them,
// since store already lives server-side where engine is importable.
type EventArtifact struct {
	Path       string             `json:"path"`
	Kind       string             `json:"kind"`
	Managed    string             `json:"managed"`
	Local      string             `json:"local"`
	Amendment  *engine.Amendment  `json:"amendment,omitempty"`
	Alteration *engine.Alteration `json:"alteration,omitempty"`
}

// DeriveDrift computes Event.Drift's value from artifacts. The sender no
// longer chooses drift directly; this is the one place that decides it.
// Three-valued with precedence, so the stale distinction survives
// derivation instead of collapsing into drifted: rollup.FleetRow.Status
// renders "stale" as its own state (FleetRows reads e.Drift verbatim), and
// the spec's migration promise is that existing rollups continue to work —
// a two-valued derivation that folded stale into drifted would silently
// break that contract.
//
//  1. any artifact's Managed in {altered, missing, orphan, occupied} -> "drifted"
//  2. else any artifact's Managed == "stale" -> "stale"
//  3. else -> "in-sync"
//
// occupied joins this set (not a fourth precedence level) because it is
// exit-1 territory on `esc status --check` exactly like the rest of the set:
// an unmanaged directory sitting at a KindDir artifact's target with no
// prior lock entry. The CLI's --check verdict and this rollup must agree on
// what counts as compliant, or a repo failing CI would still publish as
// in-sync to the fleet view.
func DeriveDrift(arts []EventArtifact) string {
	stale := false
	for _, a := range arts {
		switch a.Managed {
		case "altered", "missing", "orphan", "occupied":
			return "drifted"
		case "stale":
			stale = true
		}
	}
	if stale {
		return "stale"
	}
	return "in-sync"
}

// FleetState is the cross product of artifact axes for a repo's event, per
// spec: altered if any artifact's Managed is altered; else augmented if any
// artifact carries a local amendment; else unadulterated. altered wins over
// augmented when both axes are present. ungoverned is rollup's no-events
// case, not decided here.
func FleetState(arts []EventArtifact) string {
	amended := false
	for _, a := range arts {
		if a.Managed == "altered" {
			return "altered"
		}
		if a.Local == "amended" {
			amended = true
		}
	}
	if amended {
		return "augmented"
	}
	return "unadulterated"
}
