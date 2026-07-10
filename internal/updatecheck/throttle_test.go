// internal/updatecheck/throttle_test.go
package updatecheck

import (
	"context"
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
