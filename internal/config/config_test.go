package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestLoadAllowCustomTargetFiles(t *testing.T) {
	root := writeConfig(t, `schema: 1
packs:
  - source: file:///tmp/p
    ref: v1.0.0
    trust: unsigned
allow_custom_target_files:
  - .github/copilot-instructions.md
  - PAYMENTS-AGENTS.md
`)
	c, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.AllowCustomTargetFiles) != 2 || c.AllowCustomTargetFiles[0] != ".github/copilot-instructions.md" {
		t.Errorf("allow_custom_target_files: %v", c.AllowCustomTargetFiles)
	}
}

func TestLoadUnknownFieldRejected(t *testing.T) {
	root := writeConfig(t, "schema: 1\npacks: []\nbogus: true\n")
	if _, err := Load(root); err == nil {
		t.Fatal("want error for unknown config field")
	}
}

// TestLoadAbsentAllowCustomTargetFilesBackwardCompat covers configs written
// before allow_custom_target_files existed: the key is entirely absent, not
// just empty, and Load must still succeed (backward compat, §2.3 is opt-in).
func TestLoadAbsentAllowCustomTargetFilesBackwardCompat(t *testing.T) {
	root := writeConfig(t, `schema: 1
packs:
  - source: file:///tmp/p
    ref: v1.0.0
    trust: unsigned
`)
	c, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.AllowCustomTargetFiles) != 0 {
		t.Errorf("AllowCustomTargetFiles should be empty when the key is absent, got %v", c.AllowCustomTargetFiles)
	}
}

func TestSaveAllowCustomTargetFilesRoundTrip(t *testing.T) {
	root := t.TempDir()
	c := &Config{
		Schema:                 1,
		Packs:                  []PackRef{{Source: "file:///tmp/p", Ref: "v1.0.0", Trust: "unsigned"}},
		AllowCustomTargetFiles: []string{".github/copilot-instructions.md", "PAYMENTS-AGENTS.md"},
	}
	if err := c.Save(root); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(root)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if len(got.AllowCustomTargetFiles) != 2 ||
		got.AllowCustomTargetFiles[0] != ".github/copilot-instructions.md" ||
		got.AllowCustomTargetFiles[1] != "PAYMENTS-AGENTS.md" {
		t.Errorf("AllowCustomTargetFiles round trip mismatch: %v", got.AllowCustomTargetFiles)
	}
}
