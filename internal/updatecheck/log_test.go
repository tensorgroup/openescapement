// internal/updatecheck/log_test.go
package updatecheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogRoundTripTrimAndGitignore(t *testing.T) {
	root := t.TempDir()
	if es, err := LoadLog(root); err != nil || es != nil {
		t.Fatalf("empty log = %v, %v; want nil,nil", es, err)
	}
	for i := 0; i < 60; i++ {
		e := Entry{Time: time.Now().UTC(), Outcome: OutcomeOKCurrent, Cadence: "7d", Prompt: "none"}
		if i == 59 {
			e.Outcome = OutcomeError // most recent is an error
		}
		if err := appendLog(root, e); err != nil {
			t.Fatal(err)
		}
	}
	es, err := LoadLog(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != maxEntries {
		t.Fatalf("trim: got %d entries, want %d", len(es), maxEntries)
	}
	if LastEntry(es).Outcome != OutcomeError {
		t.Error("LastEntry should be the error entry")
	}
	if LastSuccess(es) == nil || LastSuccess(es).Outcome != OutcomeOKCurrent {
		t.Error("LastSuccess should skip the trailing error")
	}
	// Gitignore scaffolded with the log filename.
	gi, err := os.ReadFile(filepath.Join(root, ".escapement", ".gitignore"))
	if err != nil || !strings.Contains(string(gi), "update-log.jsonl") {
		t.Errorf(".gitignore not ensured: %q err %v", gi, err)
	}
}
