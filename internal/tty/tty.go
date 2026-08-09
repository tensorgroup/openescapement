// Package tty provides a single, dependency-free heuristic for deciding
// whether a stream is a real interactive terminal. Shared by every package
// that must decide whether it is safe to print a prompt and read an answer
// from stdin (internal/updatecheck's update-check throttle, internal/cli's
// `esc init` placement offer): two independent implementations of the same
// four-line check is exactly the kind of divergence that lets one of them
// silently drift and reintroduce a bug the other already fixed.
package tty

import "os"

// IsInteractive reports whether f is a real terminal.
//
// The obvious check — f.Stat().Mode()&os.ModeCharDevice != 0 — is not
// enough on its own: os.DevNull is itself a character device. `go test`
// always substitutes /dev/null for the test binary's stdin, even when
// invoked from a real terminal (verified directly: a test binary's own
// stdin stats as /dev/null), and so does any real, non-test invocation with
// stdin redirected from /dev/null, a common CI pattern (`esc sync
// < /dev/null`). Without excluding it, both would be misdetected as
// interactive and print a prompt no one is there to answer. So this checks
// the char-device bit and then rules out the null device specifically.
//
// This is still only a heuristic, not a real terminal test — that needs an
// ioctl (TIOCGETA / GetConsoleMode), which is what golang.org/x/term
// provides, and this project's single-external-dependency policy rules
// that out. In particular, /dev/zero also stats as a character device and
// is not excluded here, so it still passes this check. Any caller that
// reads from the stream after seeing true must bound how much it reads
// rather than assume a real, cooperative terminal is on the other end.
func IsInteractive(f *os.File) bool {
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	if fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	if null, err := os.Stat(os.DevNull); err == nil && os.SameFile(fi, null) {
		return false
	}
	return true
}
