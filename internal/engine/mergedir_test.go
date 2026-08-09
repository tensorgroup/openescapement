package engine

import (
	"os"
	"path/filepath"
	"slices"
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

// TestMergeDirCopiesShapesVerbatim runs awkward file contents through
// mergeDir twice (write, then the subsequent sync) and asserts every byte
// lands verbatim: mergeDir owns whole files, so its structural-validity
// obligation is to never reinterpret content, and to converge.
func TestMergeDirCopiesShapesVerbatim(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "pack-src")
	files := map[string]string{
		"SKILL.md":           "---\r\nname: s\r\n---\r\n\r\nCRLF body\r\n",
		"no-newline.md":      "last line without newline",
		"nested/deep/ref.md": "# nested\n\ncontent\n",
		"whitespace-only.md": "\n  \n",
	}
	for rel, content := range files {
		p := filepath.Join(src, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const artPath = ".claude/skills/esc-acme-org-s"
	dst := filepath.Join(root, filepath.FromSlash(artPath))

	written, err := mergeDir(root, artPath, src, dst, nil)
	if err != nil {
		t.Fatalf("mergeDir: %v", err)
	}
	if len(written) != len(files) {
		t.Fatalf("wrote %d files, want %d: %v", len(written), len(files), written)
	}
	for rel, content := range files {
		got, err := os.ReadFile(filepath.Join(dst, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if string(got) != content {
			t.Errorf("%s not copied verbatim:\nwant %q\ngot  %q", rel, content, got)
		}
	}

	// The subsequent sync: same pack state, prevFiles from the first pass.
	again, err := mergeDir(root, artPath, src, dst, written)
	if err != nil {
		t.Fatalf("second mergeDir (sync): %v", err)
	}
	if !slices.Equal(again, written) {
		t.Errorf("file list not stable across sync: %v vs %v", again, written)
	}
	for rel, content := range files {
		got, _ := os.ReadFile(filepath.Join(dst, filepath.FromSlash(rel)))
		if string(got) != content {
			t.Errorf("%s changed on the second sync:\nwant %q\ngot  %q", rel, content, got)
		}
	}
}
