// Package esc holds cross-cutting primitives: sentinel errors and hashing.
package esc

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

var (
	ErrManifest     = errors.New("invalid pack manifest")
	ErrFetch        = errors.New("pack fetch failed")
	ErrSignature    = errors.New("signature verification failed")
	ErrLockMismatch = errors.New("lockfile integrity mismatch")
	ErrConstraint   = errors.New("constraint violation")
	ErrConfig       = errors.New("invalid repo config")
	// ErrUsage marks a flag/arity mistake (maps to exit 2, alongside
	// ErrConfig). Used by the `esc pack` command family for a bad flag or
	// wrong argument count.
	ErrUsage = errors.New("usage")
)

// HashBytes returns the canonical hash string for b: "sha256:<hex>".
func HashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}
