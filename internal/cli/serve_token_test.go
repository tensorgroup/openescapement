package cli

import (
	"net/http/httptest"
	"testing"

	"github.com/tensorgroup/openescapement/internal/portal/store"
	"github.com/tensorgroup/openescapement/internal/portal/web"
)

// TestResolveToken pins cmdServe's --token contract: an explicit flag value
// wins, demo mode always disables auth regardless of the flag (demo's
// contract is unchanged), and an empty flag outside demo keeps today's
// random-token-per-start behavior.
func TestResolveToken(t *testing.T) {
	if tok, err := resolveToken(false, "my-flag-token"); err != nil || tok != "my-flag-token" {
		t.Fatalf("resolveToken(false, flag) = %q, %v, want flag value verbatim", tok, err)
	}
	if tok, err := resolveToken(true, "my-flag-token"); err != nil || tok != "" {
		t.Fatalf("resolveToken(true, flag) = %q, %v, want empty (demo disables auth)", tok, err)
	}
	tok1, err := resolveToken(false, "")
	if err != nil {
		t.Fatal(err)
	}
	if tok1 == "" || len(tok1) != 32 {
		t.Fatalf("resolveToken(false, \"\") = %q, want a 32-char random hex token", tok1)
	}
	tok2, err := resolveToken(false, "")
	if err != nil {
		t.Fatal(err)
	}
	if tok1 == tok2 {
		t.Fatalf("resolveToken(false, \"\") returned the same token twice: %q", tok1)
	}
}

// TestServeTokenFlagGatesRoute verifies the --token value actually reaches
// the server's auth gate: wired the same way cmdServe wires it (resolveToken
// then web.New), a request with no credentials is unauthorized and one
// bearing the flag's token succeeds.
func TestServeTokenFlagGatesRoute(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tok, err := resolveToken(false, "secret-flag-token")
	if err != nil {
		t.Fatal(err)
	}
	h := web.New(st, nil, tok, "test").Handler()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != 401 {
		t.Fatalf("no credentials: code=%d, want 401", rr.Code)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer secret-flag-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("flag token: code=%d, want 200", rr.Code)
	}
}
