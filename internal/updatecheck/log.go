package updatecheck

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tensorgroup/openescapement/internal/config"
)

const (
	logFileName = "update-log.jsonl"
	maxEntries  = 50
)

// Entry is one line of .escapement/update-log.jsonl.
type Entry struct {
	Time    time.Time    `json:"time"`
	Outcome Outcome      `json:"outcome"`
	Cadence string       `json:"cadence,omitempty"`
	Packs   []PackStatus `json:"packs,omitempty"`
	Prompt  string       `json:"prompt"` // none | accepted | declined
}

func logPath(root string) string {
	return filepath.Join(root, config.Dir, logFileName)
}

// LoadLog reads the JSONL log; returns (nil, nil) if the file does not exist.
// Lines that fail to parse (a truncated tail or a hand-edited line) are
// silently skipped so one corrupt line never makes the whole history
// unreadable. Errors are returned only for real I/O failures.
func LoadLog(root string) ([]Entry, error) {
	data, err := os.ReadFile(logPath(root))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Entry
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			continue // corrupt line: drop the entry, keep the rest
		}
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// LastEntry returns the newest entry, or nil.
func LastEntry(es []Entry) *Entry {
	if len(es) == 0 {
		return nil
	}
	return &es[len(es)-1]
}

// LastSuccess returns the newest ok_current/ok_updates entry, or nil.
func LastSuccess(es []Entry) *Entry {
	for i := len(es) - 1; i >= 0; i-- {
		if es[i].Outcome == OutcomeOKCurrent || es[i].Outcome == OutcomeOKUpdates {
			return &es[i]
		}
	}
	return nil
}

// appendLog appends e, trims to the last maxEntries, and rewrites the file
// atomically. It also ensures the gitignore entry so the per-clone log is
// never committed regardless of when the repo was initialized.
func appendLog(root string, e Entry) error {
	ensureGitignore(root)
	entries, _ := LoadLog(root)
	entries = append(entries, e)
	if len(entries) > maxEntries {
		entries = entries[len(entries)-maxEntries:]
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, en := range entries {
		if err := enc.Encode(en); err != nil {
			return err
		}
	}
	return atomicWrite(logPath(root), buf.Bytes())
}

// atomicWrite writes via a temp file + rename in the destination directory.
func atomicWrite(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".esc-log-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// ensureGitignore makes .escapement/.gitignore ignore the update log. It is
// best-effort: a failure here must never fail the invoking command.
func ensureGitignore(root string) {
	p := filepath.Join(root, config.Dir, ".gitignore")
	data, err := os.ReadFile(p)
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) == logFileName {
				return
			}
		}
		f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return
		}
		defer f.Close()
		if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
			f.WriteString("\n")
		}
		f.WriteString(logFileName + "\n")
		return
	}
	if !os.IsNotExist(err) {
		return // real read error: don't clobber an existing .gitignore
	}
	if os.MkdirAll(filepath.Join(root, config.Dir), 0o755) == nil {
		os.WriteFile(p, []byte(logFileName+"\n"), 0o644)
	}
}
