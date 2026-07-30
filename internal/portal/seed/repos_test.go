package seed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReposIdempotent(t *testing.T) {
	dir := t.TempDir()
	p1, r1, err := Repos(dir)
	if err != nil {
		t.Fatal(err)
	}
	p2, r2, err := Repos(dir)
	if err != nil || p1 != p2 || r1 != r2 {
		t.Fatalf("not idempotent: %v", err)
	}
	cfg, err := os.ReadFile(filepath.Join(r1, ".escapement", "config.yaml"))
	if err != nil || !strings.Contains(string(cfg), "trust: unsigned") {
		t.Fatalf("config: %s err=%v", cfg, err)
	}
}
