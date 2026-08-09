package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/lockfile"
	"github.com/tensorgroup/openescapement/internal/render"
)

// TestExamplesLockMatchesShippedFiles guards against the lockfile shipped
// with examples/governed-service drifting from the artifacts it describes.
// The lock's per-artifact hash is only consulted by `esc status`/`esc sync`
// as the Stale-vs-Altered tiebreaker once rendered content has already
// diverged from disk — so a wrong value here does not fail a plain sync,
// it silently misclassifies the next real drift. This test pins the lock
// directly against the shipped files, independent of the sync/status path,
// so the example never ships with a lock nobody re-derived.
func TestExamplesLockMatchesShippedFiles(t *testing.T) {
	root, err := filepath.Abs("../../examples/governed-service")
	if err != nil {
		t.Fatal(err)
	}

	lock, err := lockfile.Load(root)
	if err != nil {
		t.Fatalf("loading example lock: %v", err)
	}
	if lock == nil {
		t.Fatal("example lock not found")
	}

	for _, a := range lock.Artifacts {
		// Only block and file kinds are single regular files whose content
		// hashes directly to the lock's stored value the way this test
		// checks; dir (a whole skill directory) and json-keys (a subset of
		// a shared file) need different comparisons this test does not
		// perform, so they are left alone rather than mis-checked.
		if a.Kind != "block" && a.Kind != "file" {
			continue
		}
		abs := filepath.Join(root, filepath.FromSlash(a.Path))
		content, err := os.ReadFile(abs)
		if os.IsNotExist(err) {
			continue // artifact not shipped as a file in this example; nothing to pin
		}
		if err != nil {
			t.Fatalf("reading %s: %v", a.Path, err)
		}

		switch a.Kind {
		case "block":
			block, err := render.Extract(content)
			if err != nil {
				t.Fatalf("extracting managed block from %s: %v", a.Path, err)
			}
			if block == nil {
				t.Fatalf("%s: lock has a block-kind entry but the file has no managed block", a.Path)
			}
			if got := render.BodyHash(block.Body); got != a.Hash {
				t.Errorf("%s: lock hash %s does not match render.BodyHash(Extract(file).Body) %s; regenerate by running `esc sync` inside examples/governed-service (see examples/README.md)", a.Path, a.Hash, got)
			}
		case "file":
			// Same semantics as internal/engine/status.go's KindFile check:
			// the lock is written from the LF render path, so the on-disk
			// bytes hash directly (NormalizeEndings is a no-op on an
			// already-LF checkout, which the shipped example is).
			if got := esc.HashBytes(content); got != a.Hash {
				t.Errorf("%s: lock hash %s does not match esc.HashBytes(file bytes) %s; regenerate by running `esc sync` inside examples/governed-service (see examples/README.md)", a.Path, a.Hash, got)
			}
		}
	}
}
