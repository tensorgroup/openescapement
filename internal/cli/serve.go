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

	"github.com/tensorgroup/openescapement/internal/guidance"
	"github.com/tensorgroup/openescapement/internal/portal/publish"
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
	tokenFlag := fs.String("token", "", "auth token (default: random, printed on start); ignored with --demo")
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

	if *demo {
		if err := resetDemo(dir, stdout); err != nil {
			fmt.Fprintf(stderr, "esc: %v\n", err)
			return 4
		}
	}

	if err := guidance.Seed(dir); err != nil {
		fmt.Fprintf(stderr, "esc: %v\n", err)
		return 4
	}

	demoRepo := ""
	if *demo {
		// resetDemo above already wiped every demo-owned path, so this
		// rebuilds registry, events, the demo pack, and the governed repo
		// from scratch on every start — no stale state carries over.
		if err := seed.Demo(dir, time.Now().UTC()); err != nil {
			fmt.Fprintf(stderr, "esc: %v\n", err)
			return 4
		}
		_, repo, err := seed.Repos(dir)
		if err != nil {
			fmt.Fprintf(stderr, "esc: %v\n", err)
			return 4
		}
		demoRepo = repo
	}
	token, err := resolveToken(*demo, *tokenFlag)
	if err != nil {
		fmt.Fprintf(stderr, "esc: %v\n", err)
		return 4
	}

	st, err := store.Open(dir)
	if err != nil {
		fmt.Fprintf(stderr, "esc: %v\n", err)
		return 4
	}

	mgr := publish.NewManager(filepath.Join(dir, "packs"))
	srv := web.New(st, mgr, token, Version)
	srv.GuidanceDir = filepath.Join(dir, "guidance")
	httpServer := &http.Server{Addr: *addr, Handler: srv.Handler()}

	switch {
	case *demo:
		fmt.Fprintf(stdout, "esc portal (demo): http://%s/\n", *addr)
	case token == "":
		fmt.Fprintf(stdout, "esc portal: http://%s/\n", *addr)
	default:
		fmt.Fprintf(stdout, "esc portal: http://%s/?token=%s\n", *addr, token)
	}
	if *demo {
		fmt.Fprintf(stdout, "demo governed repo: %s   (cd there and run `esc sync` after publishing)\n", demoRepo)
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

// resetDemo removes the demo-owned paths under dir so `esc serve --demo`
// always boots pristine example data. It deletes only these specific paths,
// never the data dir itself, so the reset is bounded even when --demo is
// pointed at a directory. Non-demo mode never calls this.
func resetDemo(dir string, stdout io.Writer) error {
	for _, p := range []string{"registry.json", "events.jsonl", "packs", "demo-repo", "guidance"} {
		if err := os.RemoveAll(filepath.Join(dir, p)); err != nil {
			return err
		}
	}
	fmt.Fprintln(stdout, "esc: demo data reset")
	return nil
}

// resolveToken picks cmdServe's auth token. Demo mode always disables auth
// (""), regardless of tokenFlag: "Demo mode unchanged" is the deliberate
// contract, since --demo already binds localhost only and seeds throwaway
// data. Otherwise an explicit --token value is used verbatim; an empty flag
// keeps today's behavior of a fresh random hex token per start.
func resolveToken(demo bool, tokenFlag string) (string, error) {
	if demo {
		return "", nil
	}
	if tokenFlag != "" {
		return tokenFlag, nil
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
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
