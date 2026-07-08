package source

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/tensorgroup/openescapement/internal/esc"
)

// VerifyRef verifies the signature on ref (tag preferred, then commit) in the
// checkout at repoDir against an SSH allowed_signers file. GPG signatures
// verify through git's normal keyring when the ssh verification fails.
func VerifyRef(ctx context.Context, repoDir, ref, allowedSignersFile string) error {
	if allowedSignersFile == "" {
		return fmt.Errorf("%w: source is signed-mode but no allowed_signers_file configured", esc.ErrSignature)
	}
	if !validRef.MatchString(ref) {
		return fmt.Errorf("%w: ref %q contains disallowed characters", esc.ErrSignature, ref)
	}
	abs, err := filepath.Abs(allowedSignersFile)
	if err != nil {
		return fmt.Errorf("%w: %v", esc.ErrSignature, err)
	}
	cfg := "gpg.ssh.allowedSignersFile=" + abs
	if _, tagErr := git(ctx, repoDir, "-c", cfg, "verify-tag", ref); tagErr == nil {
		return nil
	}
	if _, commitErr := git(ctx, repoDir, "-c", cfg, "verify-commit", ref); commitErr == nil {
		return nil
	}
	return fmt.Errorf("%w: ref %q has no valid signature from allowed signers (%s)", esc.ErrSignature, ref, abs)
}
