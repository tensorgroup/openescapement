package source

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/esc"
)

// sshKeygen skips the test when ssh-keygen (with -Y support) is unavailable.
func sshKeygen(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not available")
	}
}

// signedRepo builds a git repo whose tag is SSH-signed, returning the repo
// path and an allowed_signers file trusting the signing key.
func signedRepo(t *testing.T, signTag bool) (repo, signers string) {
	t.Helper()
	sshDir := t.TempDir()
	key := filepath.Join(sshDir, "id_ed25519")
	if out, err := exec.Command("ssh-keygen", "-t", "ed25519", "-N", "", "-q", "-f", key).CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen: %v\n%s", err, out)
	}
	pub, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	signers = filepath.Join(sshDir, "allowed_signers")
	// allowed_signers line: "<principal> <keytype> <key>"
	fields := strings.Fields(string(pub))
	os.WriteFile(signers, []byte("policy@test.example "+fields[0]+" "+fields[1]+"\n"), 0o644)

	repo = initGitRepo(t, packFiles(), "unsigned-tag")
	cfgs := [][]string{
		{"config", "gpg.format", "ssh"},
		{"config", "user.signingkey", key},
	}
	for _, args := range cfgs {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	tagArgs := []string{"tag", "-a", "v1.0.0", "-m", "v1.0.0"}
	if signTag {
		tagArgs = []string{"tag", "-s", "v1.0.0", "-m", "v1.0.0"}
	}
	cmd := exec.Command("git", tagArgs...)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git tag: %v\n%s", err, out)
	}
	return repo, signers
}

func TestVerifySignedTag(t *testing.T) {
	sshKeygen(t)
	repo, signers := signedRepo(t, true)
	if err := VerifyRef(context.Background(), repo, "v1.0.0", signers); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
}

func TestVerifyUnsignedTag(t *testing.T) {
	sshKeygen(t)
	repo, signers := signedRepo(t, false)
	err := VerifyRef(context.Background(), repo, "v1.0.0", signers)
	if !errors.Is(err, esc.ErrSignature) {
		t.Fatalf("unsigned tag accepted: %v", err)
	}
}

func TestVerifyWrongSigner(t *testing.T) {
	sshKeygen(t)
	repo, _ := signedRepo(t, true)
	// A different trust root that does NOT contain the signing key.
	otherSigners := filepath.Join(t.TempDir(), "allowed_signers")
	os.WriteFile(otherSigners, []byte("someone@else ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBogus0000000000000000000000000000000000000000000\n"), 0o644)
	err := VerifyRef(context.Background(), repo, "v1.0.0", otherSigners)
	if !errors.Is(err, esc.ErrSignature) {
		t.Fatalf("signature from untrusted key accepted: %v", err)
	}
}

func TestVerifyNoSignersConfigured(t *testing.T) {
	err := VerifyRef(context.Background(), t.TempDir(), "v1.0.0", "")
	if !errors.Is(err, esc.ErrSignature) {
		t.Fatalf("missing signers file must fail closed: %v", err)
	}
}
