# Release Automation — goreleaser + Signing

**Status:** TODO / design. Blocks the first tagged release (`v0.1.0`).
**Why now:** the product's whole pitch is a trustworthy distribution path. Shipping unsigned binaries from a security-governance tool is a contradiction reviewers will notice. This is table stakes, not polish.

## Goal

`git tag v0.1.0 && git push --tags` produces, with no manual steps:
1. Reproducible static binaries for the platforms our ICP uses.
2. Cryptographic signatures + provenance that let anyone verify a binary came from our CI and wasn't tampered with.
3. A GitHub Release with checksums, signatures, and generated notes.
4. Correct `esc version` output (version + commit + date stamped via ldflags).

## Deliverables

### 1. `.goreleaser.yaml`
- **Targets:** `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`. (CI is overwhelmingly linux/amd64; dev machines are darwin/arm64.)
- **ldflags:** stamp `internal/cli.Version`, plus new `Commit` and `Date` vars, from tag/commit. Update `esc version` to print all three.
- **`mod_timestamp`** set to the commit time for reproducible archives.
- **`CGO_ENABLED=0`**, `-trimpath` — static, path-independent builds.
- **Checksums:** `checksums.txt` (sha256) as a release artifact.

### 2. Signing & provenance
- **cosign keyless signing** of checksums and archives (OIDC via GitHub Actions — no long-lived keys to leak). Publish `.sig` + `.pem` per artifact.
- **SLSA provenance:** adopt `slsa-framework/slsa-github-generator` (or goreleaser's built-in SLSA) to emit a provenance attestation. Target SLSA build level 3.
- **SBOM:** goreleaser `sboms` (syft) → CycloneDX SBOM per release. Cheap, and a frequent enterprise checklist item.

### 3. Release workflow (`.github/workflows/release.yml`)
- Trigger: push of a `v*` tag.
- Permissions: `contents: write`, `id-token: write` (keyless), `packages` if we later push containers.
- Steps: checkout (full history for notes) → setup-go → `go test ./...` gate → goreleaser release → verify the just-published signature as a smoke test.
- **Gate on green tests** — never publish a release whose tests didn't pass in CI.

### 4. CI workflow (`.github/workflows/ci.yml`) — prerequisite
- Trigger: PR + push to main. `go test ./...` (incl. `-race` on the engine/cli packages), `go vet`, `gofmt -l` (fail if non-empty), `goreleaser build --snapshot` (proves the release config compiles).
- Matrix at least linux + macos so path/symlink behavior is exercised on both.

### 5. Install paths
- **`go install github.com/tensorgroup/openescapement/cmd/esc@latest`** — already works; document as the baseline.
- **Homebrew tap** (`tensorgroup/homebrew-tap`) via goreleaser `brews:` — the darwin-heavy ICP expects `brew install esc`.
- **`install.sh`** convenience script that downloads the right archive, verifies the cosign signature, and drops `esc` on PATH. Ship the verify step *in the installer* — dogfood our own trust story.
- Publish a **`VERIFYING.md`** with copy-paste `cosign verify-blob` / `slsa-verifier` commands.

### 6. Version-check nicety (optional, later)
- `esc version --check` compares against the latest GitHub release tag and warns if stale. Low priority; keep it opt-in and offline-safe.

## Acceptance
- [ ] Tagging a release produces signed, reproducible binaries for all five targets
- [ ] `cosign verify-blob` succeeds against published artifacts using the documented command
- [ ] `slsa-verifier verify-artifact` passes
- [ ] `esc version` prints version + commit + date
- [ ] `brew install` and `install.sh` both land a working, signature-verified binary
- [ ] README install section updated; `SECURITY.md` "binary signing lands with first release" caveat removed

## Notes
- Keyless cosign avoids key custody entirely — the right default for a small team. Revisit a KMS-backed key only if an offline/air-gapped signing requirement appears.
- Windows support is build-target-only for now; we don't test Windows path behavior in v0.1. Flag if a Windows user appears.
