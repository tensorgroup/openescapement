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

// h2Index returns the byte offset of the first line that is exactly the
// level-2 heading "## title", outside HTML comments, or -1 if there is none.
func h2Index(text, title string) int {
	inComment := false
	offset := 0
	for _, line := range strings.SplitAfter(text, "\n") {
		start := offset
		offset += len(line)
		trimmed := strings.TrimRight(line, "\r\n")
		visible := ""
		for rest := trimmed; ; {
			if inComment {
				end := strings.Index(rest, "-->")
				if end < 0 {
					break
				}
				inComment = false
				rest = rest[end+len("-->"):]
				continue
			}
			open := strings.Index(rest, "<!--")
			if open < 0 {
				visible += rest
				break
			}
			visible += rest[:open]
			inComment = true
			rest = rest[open+len("<!--"):]
		}
		if visible == trimmed && trimmed == "## "+title {
			return start
		}
	}
	return -1
}

func TestH2Index(t *testing.T) {
	cases := []struct {
		name, text string
		want       int
	}{
		{"level two at start", "## Access routes\nx", 0},
		{"level two after line", "intro\n## Access routes\n", 6},
		{"level three only", "### Access routes\n", -1},
		{"inside comment", "<!-- ## Access routes -->\n", -1},
		{"inside multiline comment", "<!--\n## Access routes\n-->\n", -1},
		{"mid-line", "see ## Access routes\n", -1},
		{"trailing text", "## Access routes and more\n", -1},
		{"after comment", "<!-- ## Access routes -->\n## Access routes\n", 26},
		{"crlf", "a\r\n## Access routes\r\n", 3},
		{"absent", "## Sources\n", -1},
	}
	for _, c := range cases {
		if got := h2Index(c.text, "Access routes"); got != c.want {
			t.Errorf("%s: h2Index = %d, want %d", c.name, got, c.want)
		}
	}
}

// checkSectionOrder reports a problem when the note lacks a level-2
// Governance, Access routes, or Sources heading, or has them out of order.
func checkSectionOrder(text string) string {
	gov, routes, src := h2Index(text, "Governance"), h2Index(text, "Access routes"), h2Index(text, "Sources")
	switch {
	case gov < 0:
		return "missing a ## Governance heading"
	case routes < 0:
		return "missing a ## Access routes heading"
	case src < 0:
		return "missing a ## Sources heading"
	case !(gov < routes && routes < src):
		return "sections must run Governance, Access routes, Sources"
	}
	return ""
}

func TestCheckSectionOrder(t *testing.T) {
	good := "## Governance\n\n## Access routes\n\n## Sources\n"
	if msg := checkSectionOrder(good); msg != "" {
		t.Errorf("well-formed note rejected: %s", msg)
	}
	bad := map[string]string{
		"level three":       "## Governance\n### Access routes\n## Sources\n",
		"commented out":     "## Governance\n<!-- ## Access routes -->\n## Sources\n",
		"before governance": "## Access routes\n## Governance\n## Sources\n",
		"after sources":     "## Governance\n## Sources\n## Access routes\n",
		"no governance":     "## Access routes\n## Sources\n",
	}
	for name, text := range bad {
		if checkSectionOrder(text) == "" {
			t.Errorf("%s: broken note accepted", name)
		}
	}
}

func TestEveryVendorNoteHasAccessRoutes(t *testing.T) {
	s := Load("")
	for _, key := range VendorOrder {
		note, _, err := s.ReadFile(key + ".md")
		if err != nil {
			t.Fatalf("note %s.md: %v", key, err)
		}
		if msg := checkSectionOrder(string(note)); msg != "" {
			t.Errorf("%s.md: %s", key, msg)
		}
	}
}

func TestEveryCurrentFullDepthModelHasStarter(t *testing.T) {
	s := Load("") // embedded
	fullDepth := map[string]bool{"anthropic": true, "openai": true, "google": true}
	for _, v := range s.Registry.Vendors {
		if !fullDepth[v.Key] {
			continue
		}
		for _, m := range v.Models {
			if m.Status != "current" {
				continue
			}
			if m.Starter == "" {
				t.Errorf("current model %s (%s) has no starter", m.ID, v.Key)
				continue
			}
			if _, _, err := s.ReadFile(m.Starter); err != nil {
				t.Errorf("starter %q for %s not readable: %v", m.Starter, m.ID, err)
			}
		}
	}
}
