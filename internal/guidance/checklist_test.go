package guidance

import (
	"strings"
	"testing"
)

// seededTelemetryModels mirrors internal/portal/seed/seed.go modelOrder. Keep
// in sync when the demo seed's models change (spec §6 content acceptance).
var seededTelemetryModels = []string{
	"claude-sonnet-5", "claude-opus-5", "claude-haiku-4-5", "gpt-5.2", "gemini-3-pro",
}

func TestEverySeededModelHasRegistryEntry(t *testing.T) {
	s := Load("") // embedded
	for _, id := range seededTelemetryModels {
		if _, ok := s.VendorForModel(id); !ok {
			t.Errorf("seeded telemetry model %q missing from registry", id)
		}
	}
}

func TestEveryVendorNoteHasSources(t *testing.T) {
	s := Load("")
	for _, key := range VendorOrder {
		note, _, err := s.ReadFile(key + ".md")
		if err != nil {
			t.Fatalf("note %s.md: %v", key, err)
		}
		if !strings.Contains(string(note), "## Sources") {
			t.Errorf("%s.md missing a Sources section", key)
		}
	}
}
