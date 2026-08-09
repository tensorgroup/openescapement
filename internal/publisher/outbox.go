package publisher

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/tensorgroup/openescapement/internal/config"
)

const (
	// outboxFileName names the per-clone outbox file. Kept as its own
	// unexported copy here rather than importing internal/updatecheck for
	// the constant — see identity.go for why this package keeps its own
	// copies instead of reaching into a sibling package for small pieces.
	outboxFileName = "outbox.jsonl"

	// OutboxCap is the maximum number of queued envelopes AppendOutbox
	// keeps. Beyond it, the oldest entries are evicted first.
	OutboxCap = 200

	// OutboxMaxAge is how long a queued envelope is kept before AppendOutbox
	// evicts it regardless of count.
	OutboxMaxAge = 14 * 24 * time.Hour
)

// outboxEntry is one line of .escapement/outbox.jsonl: the envelope plus
// the endpoint it is bound for (so a later flush knows where to send it
// without re-deriving it) and the time it was queued (for age eviction and
// oldest-first ordering).
type outboxEntry struct {
	Time     time.Time `json:"ts"`
	Endpoint string    `json:"endpoint"`
	Envelope Envelope  `json:"envelope"`
}

func outboxPath(root string) string {
	return filepath.Join(root, config.Dir, outboxFileName)
}

// loadOutbox reads the JSONL outbox; returns (nil, 0, nil) if the file does
// not exist. A line that fails to parse (a truncated tail, a hand-edited
// line) is skipped and counted rather than failing the whole read, matching
// updatecheck.LoadLog's discipline: one corrupt line never makes the rest
// of the queue unreadable. Errors are returned only for real I/O failures.
func loadOutbox(root string) (entries []outboxEntry, corrupt int, err error) {
	data, err := os.ReadFile(outboxPath(root))
	if os.IsNotExist(err) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var e outboxEntry
		if jsonErr := json.Unmarshal(line, &e); jsonErr != nil {
			corrupt++
			continue
		}
		entries = append(entries, e)
	}
	if scErr := sc.Err(); scErr != nil {
		return nil, corrupt, scErr
	}
	return entries, corrupt, nil
}

// appendOutboxLine appends one entry to the outbox with O_APPEND: the hot
// path for the common case where no eviction is needed, so a normal append
// never pays for rewriting the whole file.
func appendOutboxLine(root string, e outboxEntry) error {
	dir := filepath.Join(root, config.Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	f, err := os.OpenFile(outboxPath(root), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

// atomicWriteOutbox rewrites the outbox file via temp+rename, mirroring
// updatecheck's atomicWrite (this package keeps its own unexported copy;
// see identity.go for why this package doesn't reach into a sibling
// package for a small helper). Used only when evicting or flushing —
// never on the hot append path.
func atomicWriteOutbox(root string, entries []outboxEntry) error {
	dir := filepath.Join(root, config.Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	tmp, err := os.CreateTemp(dir, ".esc-outbox-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), outboxPath(root))
}

// evictOutbox drops the oldest entries in entries (assumed oldest-first)
// that are either older than OutboxMaxAge (relative to now) or beyond
// OutboxCap once the age eviction is applied. It returns the surviving
// entries and how many were dropped.
func evictOutbox(entries []outboxEntry, now time.Time) (kept []outboxEntry, dropped int) {
	kept = entries
	i := 0
	for i < len(kept) && now.Sub(kept[i].Time) > OutboxMaxAge {
		i++
	}
	kept = kept[i:]
	if len(kept) > OutboxCap {
		kept = kept[len(kept)-OutboxCap:]
	}
	return kept, len(entries) - len(kept)
}

// AppendOutbox appends one envelope, bound for endpoint, to
// .escapement/outbox.jsonl, then enforces retention: a count over
// OutboxCap or an age over OutboxMaxAge drops the oldest entries first.
// dropped is every entry evicted this call, so a caller can report the
// loss on stderr — a silently truncating queue reads as healthy telemetry
// while losing data. The common case (no eviction needed) appends with
// O_APPEND; eviction rewrites the file via temp+rename.
func AppendOutbox(root, endpoint string, e Envelope, now time.Time) (dropped int, err error) {
	entries, _, err := loadOutbox(root)
	if err != nil {
		return 0, err
	}
	newEntry := outboxEntry{Time: now, Endpoint: endpoint, Envelope: e}
	entries = append(entries, newEntry)

	kept, dropped := evictOutbox(entries, now)
	if dropped == 0 {
		if err := appendOutboxLine(root, newEntry); err != nil {
			return 0, err
		}
		return 0, nil
	}
	if err := atomicWriteOutbox(root, kept); err != nil {
		return dropped, err
	}
	return dropped, nil
}

// FlushOutbox sends every queued envelope oldest-first via send, stopping
// at the first failure and leaving the failed entry and everything after
// it queued for the next attempt. A corrupt line encountered on load is
// skipped (never fatal) and is dropped from the file once any entries are
// rewritten. sent counts successful sends; remaining counts entries left
// in the outbox afterward. A missing or empty outbox is a no-op.
func FlushOutbox(root string, send func(endpoint string, e Envelope) error, now time.Time) (sent, remaining int, err error) {
	entries, _, err := loadOutbox(root)
	if err != nil {
		return 0, 0, err
	}
	if len(entries) == 0 {
		return 0, 0, nil
	}

	var sendErr error
	i := 0
	for ; i < len(entries); i++ {
		if sendErr = send(entries[i].Endpoint, entries[i].Envelope); sendErr != nil {
			break
		}
	}
	sent = i
	remaining = len(entries) - sent

	if werr := atomicWriteOutbox(root, entries[sent:]); werr != nil {
		return sent, remaining, werr
	}
	return sent, remaining, sendErr
}
