// internal/updatecheck/throttle_test.go
package updatecheck

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadPromptDecision(t *testing.T) {
	var w strings.Builder
	if !readPromptDecision(strings.NewReader("\n"), &w) {
		t.Error("empty line should accept")
	}
	if readPromptDecision(strings.NewReader("n\n"), &w) {
		t.Error("'n' should decline")
	}
	if readPromptDecision(strings.NewReader("\x1b\n"), &w) {
		t.Error("ESC sequence should decline")
	}
	if readPromptDecision(strings.NewReader(""), &w) {
		t.Error("bare EOF (Ctrl-D) should decline, not accept")
	}
	if !strings.Contains(w.String(), "Update now?") {
		t.Error("prompt text not written")
	}
}

func TestMaybeInertWithoutLog(t *testing.T) {
	root := t.TempDir()
	// No log at all → inert, returns nil, writes nothing.
	if d := Maybe(context.Background(), root, nil, &strings.Builder{}); d != nil {
		t.Fatalf("no log should be inert, got %+v", d)
	}
	if es, _ := LoadLog(root); es != nil {
		t.Error("Maybe wrote a log when inert")
	}
}

func TestMaybeNotOverdue(t *testing.T) {
	root := t.TempDir()
	// A fresh successful check within cadence → not overdue.
	appendLog(root, Entry{Time: time.Now().UTC(), Outcome: OutcomeOKCurrent, Cadence: "7d", Prompt: "none"})
	before, _ := LoadLog(root)
	if d := Maybe(context.Background(), root, nil, &strings.Builder{}); d != nil {
		t.Fatalf("within cadence should be no-op, got %+v", d)
	}
	after, _ := LoadLog(root)
	if len(after) != len(before) {
		t.Error("Maybe wrote a log entry while within cadence")
	}
}

func TestMaybeConfigLoadErrorLogged(t *testing.T) {
	root := t.TempDir()
	// Overdue active log, but a corrupt config.yaml: the failure must be
	// visible as an error entry, not a silent return.
	old := time.Now().UTC().Add(-8 * 24 * time.Hour)
	appendLog(root, Entry{Time: old, Outcome: OutcomeOKCurrent, Cadence: "7d", Prompt: "none"})
	if err := os.WriteFile(filepath.Join(root, ".escapement", "config.yaml"), []byte("{{{not yaml"), 0o644); err != nil {
		t.Fatal(err)
	}
	var errbuf strings.Builder
	if d := Maybe(context.Background(), root, nil, &errbuf); d != nil {
		t.Fatalf("config failure should return nil, got %+v", d)
	}
	es, _ := LoadLog(root)
	last := LastEntry(es)
	if len(es) != 2 || last.Outcome != OutcomeError {
		t.Fatalf("want appended error entry, got %d entries, last %+v", len(es), last)
	}
	if last.Cadence != "7d" {
		t.Errorf("error entry cadence = %q, want 7d (throttle stays live)", last.Cadence)
	}
	if !strings.Contains(errbuf.String(), "update check skipped") {
		t.Errorf("stderr notice missing, got %q", errbuf.String())
	}
}

func TestRecordSyncInertMarkerOnce(t *testing.T) {
	root := t.TempDir()
	// Feature was active; packs no longer declare update_check. Only the
	// transition to inert writes a marker — not every sync thereafter.
	appendLog(root, Entry{Time: time.Now().UTC(), Outcome: OutcomeOKCurrent, Cadence: "7d", Prompt: "none"})
	RecordSync(context.Background(), root, nil)
	RecordSync(context.Background(), root, nil)
	es, _ := LoadLog(root)
	if len(es) != 2 {
		t.Fatalf("want exactly one inert marker appended, got %d entries", len(es))
	}
	if LastEntry(es).Cadence != "" {
		t.Errorf("marker entry should have empty cadence, got %q", LastEntry(es).Cadence)
	}
}
