package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/tensorgroup/openescapement/internal/portal/seed"
	"github.com/tensorgroup/openescapement/internal/portal/store"
	"github.com/tensorgroup/openescapement/internal/portal/web"
)

// cmdServe launches the esc admin portal HTTP server. It does not use the
// caller's context: Run bounds every other command to a 10-minute timeout to
// stop a hostile remote from hanging CI, but that timeout would kill a
// long-running server. cmdServe instead builds its own context tied to
// SIGINT/SIGTERM for a clean shutdown.
func cmdServe(root string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:8484", "listen address")
	dataDir := fs.String("data-dir", "", "server data directory (default: ~/.escapement/server)")
	demo := fs.Bool("demo", false, "seed demo data; disable auth (localhost only)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *demo && !isLoopback(*addr) {
		fmt.Fprintln(stderr, "esc: demo mode binds localhost only")
		return 2
	}

	dir := *dataDir
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(stderr, "esc: %v\n", err)
			return 4
		}
		dir = filepath.Join(home, ".escapement", "server")
	}

	token := ""
	if *demo {
		if err := seed.Demo(dir, time.Now().UTC()); err != nil {
			fmt.Fprintf(stderr, "esc: %v\n", err)
			return 4
		}
	} else {
		buf := make([]byte, 16)
		if _, err := rand.Read(buf); err != nil {
			fmt.Fprintf(stderr, "esc: %v\n", err)
			return 4
		}
		token = hex.EncodeToString(buf)
	}

	st, err := store.Open(dir)
	if err != nil {
		fmt.Fprintf(stderr, "esc: %v\n", err)
		return 4
	}

	// Packs (the publish manager) is nil until Task 10 wires a packs
	// directory into cmdServe; the portal's packs pages treat that as "no
	// pack repos configured".
	srv := web.New(st, nil, token, Version)
	httpServer := &http.Server{Addr: *addr, Handler: srv.Handler()}

	if token == "" {
		fmt.Fprintf(stdout, "esc portal: http://%s/\n", *addr)
	} else {
		fmt.Fprintf(stdout, "esc portal: http://%s/?token=%s\n", *addr, token)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	serveErr := make(chan error, 1)
	go func() { serveErr <- httpServer.ListenAndServe() }()

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(stderr, "esc: %v\n", err)
			return 4
		}
		return 0
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintf(stderr, "esc: %v\n", err)
			return 4
		}
		return 0
	}
}

// isLoopback reports whether addr's host resolves to a loopback address or
// "localhost". Used to keep --demo (auth disabled) off non-local interfaces.
func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
