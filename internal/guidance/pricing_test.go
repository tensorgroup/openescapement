package guidance

import (
	"regexp"
	"testing"
)

// Example fragments are adopted verbatim into user rule packs, where absolute
// prices go stale. Dollar figures belong in the vendor note pages; fragments
// keep only relative cost guidance.
func TestExampleFragmentsCarryNoAbsolutePricing(t *testing.T) {
	g := Load("")
	priced := regexp.MustCompile(`\$\d`)
	for _, vendor := range VendorOrder {
		for _, rel := range g.ExampleFiles(vendor) {
			content, _, err := g.ReadFile(rel)
			if err != nil {
				t.Fatalf("%s: %v", rel, err)
			}
			if priced.Match(content) {
				t.Errorf("%s contains absolute pricing; keep dollar figures in the vendor note page", rel)
			}
		}
	}
}
