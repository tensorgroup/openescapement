package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMergeDirRejectsSelfTargetingLockEntries covers the mergeDir removal
// loop directly, at the package level, because the case under test needs
// dst to be genuinely empty when the removal runs — awkward to force
// through a full CLI sync, trivial to construct here.
//
// A lockfile "files" entry of "." or "" resolves (via containedPath(dst,
// prev)) to dst itself, not to anything under it. Without an explicit
// check, that target would reach os.Remove(dst): on a directory, Remove
// only fails with ENOTEMPTY, so an empty dst would be deleted outright,
// and the accident of non-empty directories being merely coincidental
// protection. mergeDir rejects target == dst explicitly instead, and this
// test proves it against a dst with nothing in it — the case where the
// accidental ENOTEMPTY protection would not have helped.
func TestMergeDirRejectsSelfTargetingLockEntries(t *testing.T) {
	for _, prev := range []string{".", ""} {
		t.Run("entry_"+prev, func(t *testing.T) {
			root := t.TempDir()
			src := filepath.Join(root, "pack-src") // deliberately empty: no pack files
			if err := os.MkdirAll(src, 0o755); err != nil {
				t.Fatal(err)
			}
			artPath := ".claude/skills/esc-acme-org-esc-empty"
			// filepath.ToSlash/FromSlash round trip keeps this platform-safe
			// without hardcoding a separator.
			dst := filepath.Join(root, filepath.FromSlash(artPath))
			if err := os.MkdirAll(dst, 0o755); err != nil {
				t.Fatal(err)
			}

			_, err := mergeDir(root, artPath, src, dst, []string{prev})
			if err == nil {
				t.Fatal("expected mergeDir to refuse a lockfile entry that resolves to the skill directory itself")
			}
			if fi, statErr := os.Stat(dst); statErr != nil || !fi.IsDir() {
				t.Errorf("skill directory was removed instead of the removal being refused: stat err=%v", statErr)
			}
		})
	}
}
