package engine

import (
	"fmt"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/pack"
)

// Reporting levels, ordered.
const (
	ReportOff     = "off"
	ReportMetrics = "metrics"
	ReportContent = "content"
)

var reportRank = map[string]int{ReportOff: 0, ReportMetrics: 1, ReportContent: 2}

// Collection is the resolved reporting level and where it came from. Spec 1
// resolves and reports it but never acts on it: local surfaces always show
// complete local truth, and redaction lives in the publisher.
type Collection struct {
	Amendments string `json:"amendments"`
	Source     string `json:"source"` // default | pack | repo-override
}

// ResolveReporting returns the effective level. The highest level any pack
// requests wins, mirroring the update-check rule that the strictest cadence
// wins: a pack can make policy stricter but never weaker, so adding a second
// pack can never silently reduce what an admin sees. The repo may then clamp
// down, never up.
func ResolveReporting(packs []*pack.Pack, cfg *config.Config) (Collection, error) {
	level, source := ReportOff, "default"
	for _, p := range packs {
		if p == nil || p.Manifest.Reporting == nil {
			continue
		}
		want := p.Manifest.Reporting.Amendments
		rank, ok := reportRank[want]
		if !ok || want == ReportOff {
			return Collection{}, fmt.Errorf("%w: pack %s declares reporting.amendments %q (want metrics or content)",
				esc.ErrManifest, p.Manifest.Name, want)
		}
		if rank > reportRank[level] {
			level, source = want, "pack"
		}
	}
	if cfg == nil || cfg.ReportAmendments == "" {
		return Collection{Amendments: level, Source: source}, nil
	}
	want := cfg.ReportAmendments
	rank, ok := reportRank[want]
	if !ok || want == ReportContent {
		return Collection{}, fmt.Errorf("%w: report_amendments %q (want metrics or off)", esc.ErrConfig, want)
	}
	if rank >= reportRank[level] {
		return Collection{}, fmt.Errorf("%w: report_amendments %q cannot raise the level above the pack-declared %q",
			esc.ErrConfig, want, level)
	}
	return Collection{Amendments: want, Source: "repo-override"}, nil
}
