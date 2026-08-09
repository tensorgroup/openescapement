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

// DeriveDrift computes Event.Drift's value from artifacts: "drifted" if any
// artifact's Managed state is not in-sync, else "in-sync". The sender no
// longer chooses drift directly; this is the one place that decides it.
func DeriveDrift(arts []EventArtifact) string {
	for _, a := range arts {
		if a.Managed != "in-sync" {
			return "drifted"
		}
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
