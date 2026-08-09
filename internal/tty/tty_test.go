package tty

import (
	"os"
	"testing"
)

func TestIsInteractiveNil(t *testing.T) {
	if IsInteractive(nil) {
		t.Error("a nil file must not be treated as interactive")
	}
}

// go test always substitutes /dev/null for the test binary's stdin, so this
// doubles as a check that os.Stdin itself is not misdetected inside the
// test suite (see the package doc comment for why that matters).
func TestIsInteractiveStdinUnderGoTest(t *testing.T) {
	if IsInteractive(os.Stdin) {
		t.Error("os.Stdin under `go test` is /dev/null and must not be treated as interactive")
	}
}

func TestIsInteractiveExcludesDevNull(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if IsInteractive(f) {
		t.Error("os.DevNull must not be treated as an interactive terminal")
	}
}

func TestIsInteractiveRegularFileIsNotInteractive(t *testing.T) {
	f, err := os.Open(os.Args[0]) // any regular file on disk
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if IsInteractive(f) {
		t.Error("a regular file is not a character device and must not be treated as interactive")
	}
}
