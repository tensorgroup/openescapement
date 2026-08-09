package publisher

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
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

// FlushOutbox sends every queued envelope via send, grouped BY ENDPOINT:
// within each endpoint's group entries go out oldest-first, and a failure
// stops only that endpoint's group — every other endpoint's group still
// runs. This is deliberate: the outbox is one shared queue across every
// configured endpoint (AppendOutbox is called once per endpoint per
// Publish call), and a single dead endpoint must not starve every other
// endpoint's backlog behind it. Before this fix, a global oldest-first scan
// stopped at the very first failure regardless of which endpoint it
// belonged to, so a healthy endpoint's entries would queue forever behind a
// dead one — and eventually be dropped silently by the age eviction above,
// never once reaching an endpoint that was ready to receive them the whole
// time. Groups are visited in order of each endpoint's oldest entry, and
// survivors (retained across a failure) are written back in their original
// relative order, not regrouped, so the on-disk queue keeps reading as one
// chronological log.
//
// Before sending, any entry older than OutboxMaxAge relative to now is
// dropped rather than sent: the portal stamps each event's timestamp at
// ingest, not from the envelope, so a stale entry that happened to flush
// successfully weeks late would masquerade as fresh fleet data. A corrupt
// line encountered on load is likewise never sent. dropped folds all three
// cases, aged out, corrupt on load, and (see next paragraph) permanently
// rejected by send, into one count, "not sent and not kept", since a
// caller reporting loss on stderr does not need to distinguish why an entry
// never went out.
//
// Within a group, a send error matching errors.Is(err, errPermanent) is
// this function's third outcome, alongside success and an ordinary
// (transient) failure: the entry is dropped (counted in dropped, never
// retained) and the SAME endpoint's next entry is still attempted. A
// permanently-rejected entry, such as a payload over the server's size
// limit, will never succeed no matter how many times it is retried, so
// unlike a transient failure it carries no useful "stop here" signal about
// the entries queued behind it; retaining it would only let one poisoned
// entry stall every later entry on that endpoint forever. An ordinary error
// still stops the group and retains it plus everything queued behind it,
// exactly as before.
//
// sent counts successful sends; remaining counts entries left queued
// afterward. err is the first ordinary (non-permanent) error encountered,
// in group-visit order, or nil if every group fully drained without one:
// a permanent failure is fully resolved by being dropped, so it is not
// reflected in err, only in dropped. A missing or empty outbox is a no-op.
func FlushOutbox(root string, send func(endpoint string, e Envelope) error, now time.Time) (sent, dropped, remaining int, err error) {
	entries, corrupt, err := loadOutbox(root)
	if err != nil {
		return 0, 0, 0, err
	}
	dropped = corrupt

	var fresh []outboxEntry
	for _, e := range entries {
		if now.Sub(e.Time) > OutboxMaxAge {
			dropped++
			continue
		}
		fresh = append(fresh, e)
	}

	if len(fresh) == 0 {
		if dropped > 0 {
			if werr := atomicWriteOutbox(root, nil); werr != nil {
				return 0, dropped, 0, werr
			}
		}
		return 0, dropped, 0, nil
	}

	// Partition fresh into per-endpoint index groups. fresh is already
	// oldest-first overall, and a stable partition preserves that within
	// each group; order records each endpoint's first (oldest) appearance,
	// so groups are visited oldest-backlog-first too.
	order := make([]string, 0, len(fresh))
	groups := make(map[string][]int, len(fresh))
	for i, e := range fresh {
		if _, ok := groups[e.Endpoint]; !ok {
			order = append(order, e.Endpoint)
		}
		groups[e.Endpoint] = append(groups[e.Endpoint], i)
	}

	retained := make([]bool, len(fresh))
	var sendErr error
	for _, endpoint := range order {
		stopped := false
		for _, i := range groups[endpoint] {
			if stopped {
				retained[i] = true
				continue
			}
			serr := send(fresh[i].Endpoint, fresh[i].Envelope)
			if serr == nil {
				sent++
				continue
			}
			if errors.Is(serr, errPermanent) {
				// Never accepted, never will be: drop it and keep going
				// within this same endpoint's group. Unlike an ordinary
				// failure, a permanent one says nothing about whether the
				// entries queued behind it would succeed.
				dropped++
				continue
			}
			if sendErr == nil {
				sendErr = serr
			}
			stopped = true
			retained[i] = true
		}
	}

	survivors := make([]outboxEntry, 0, len(fresh)-sent)
	for i, e := range fresh {
		if retained[i] {
			survivors = append(survivors, e)
		}
	}
	remaining = len(survivors)

	if werr := atomicWriteOutbox(root, survivors); werr != nil {
		return sent, dropped, remaining, werr
	}
	return sent, dropped, remaining, sendErr
}
