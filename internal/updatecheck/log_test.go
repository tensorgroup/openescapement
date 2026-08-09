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

func TestLoadLogSkipsCorruptLines(t *testing.T) {
	root := t.TempDir()
	valid := Entry{Time: time.Now().UTC(), Outcome: OutcomeOKCurrent, Cadence: "7d", Prompt: "none"}
	if err := appendLog(root, valid); err != nil {
		t.Fatal(err)
	}
	// Inject a garbage line between two valid entries by hand-editing the file.
	p := filepath.Join(root, ".escapement", logFileName)
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("{truncated\n")...)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := appendLog(root, valid); err != nil {
		t.Fatal(err)
	}
	es, err := LoadLog(root)
	if err != nil {
		t.Fatalf("LoadLog with corrupt line: %v", err)
	}
	if len(es) != 2 {
		t.Fatalf("got %d entries, want 2 (corrupt line skipped)", len(es))
	}
	for i, e := range es {
		if e.Outcome != OutcomeOKCurrent {
			t.Errorf("entry %d outcome = %q, want %q", i, e.Outcome, OutcomeOKCurrent)
		}
	}
}

func TestEnsureGitignoreIdempotent(t *testing.T) {
	root := t.TempDir()
	ensureGitignore(root)
	ensureGitignore(root)
	gi, err := os.ReadFile(filepath.Join(root, ".escapement", ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(gi), logFileName); n != 1 {
		t.Errorf("gitignore contains %d occurrences of %q, want exactly 1: %q", n, logFileName, gi)
	}
	if n := strings.Count(string(gi), outboxFileName); n != 1 {
		t.Errorf("gitignore contains %d occurrences of %q, want exactly 1: %q", n, outboxFileName, gi)
	}
}

func TestEnsureGitignoreAddsMissingEntryToExistingFile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".escapement")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Simulate a .gitignore that only ever saw the update log (e.g. written
	// before the outbox existed): ensureGitignore must add the missing
	// entry without disturbing the one already present.
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(logFileName+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ensureGitignore(root)
	gi, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(gi), logFileName); n != 1 {
		t.Errorf("gitignore contains %d occurrences of %q, want exactly 1: %q", n, logFileName, gi)
	}
	if n := strings.Count(string(gi), outboxFileName); n != 1 {
		t.Errorf("gitignore contains %d occurrences of %q, want exactly 1: %q", n, outboxFileName, gi)
	}
}
