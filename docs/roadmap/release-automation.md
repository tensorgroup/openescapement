# Release Automation — goreleaser + Signing

**Status:** Implemented (pending first tagged release). Plan: docs/superpowers/plans/2026-07-08-release-automation.md
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
- **cosign keyless signing** of checksums and archives (OIDC via GitHub Actions — no long-lived keys to leak). Publish a sigstore bundle (`<artifact>.sigstore.json`) per artifact.
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
- **Homebrew tap** (`tensorgroup/homebrew-tap`) via goreleaser `homebrew_casks:` — the darwin-heavy ICP expects `brew install esc`.
- **`install.sh`** convenience script that downloads the right archive, verifies the cosign signature, and drops `esc` on PATH. Ship the verify step *in the installer* — dogfood our own trust story.
- Publish a **`VERIFYING.md`** with copy-paste `cosign verify-blob` / `slsa-verifier` commands.

### 6. Version-check nicety (optional, later)
- `esc version --check` compares against the latest GitHub release tag and warns if stale. Low priority; keep it opt-in and offline-safe.

## Acceptance
- [ ] Tagging a release produces signed, reproducible binaries for all five targets
- [ ] `cosign verify-blob` succeeds against published artifacts using the documented command
- [ ] `slsa-verifier verify-artifact` passes
- [x] `esc version` prints version + commit + date
- [ ] `brew install` and `install.sh` both land a working, signature-verified binary
- [x] README install section updated; `SECURITY.md` "binary signing lands with first release" caveat removed

## Notes
- Keyless cosign avoids key custody entirely — the right default for a small team. Revisit a KMS-backed key only if an offline/air-gapped signing requirement appears.
- Windows support is build-target-only for now; we don't test Windows path behavior in v0.1. Flag if a Windows user appears.
- Implementation deviations: cosign v3 emits a single sigstore bundle
  (`<artifact>.sigstore.json`) per artifact instead of separate `.sig` + `.pem`;
  goreleaser's `brews:` is deprecated, so the tap ships a Cask
  (`homebrew_casks:`) with a quarantine-clearing post-install hook.
- Manual prerequisites before tagging v0.1.0:
  1. ~~Create the `tensorgroup/homebrew-tap` repo.~~ Done 2026-09-16 (public,
     README only; goreleaser writes `Casks/esc.rb`).
  2. ~~Add a tap token secret.~~ Done 2026-09-16, as a deploy key rather than a
     PAT: an ed25519 key registered on the tap with write access, its private
     half stored as the `HOMEBREW_TAP_DEPLOY_KEY` secret on this repo, and the
     cask pushed over SSH (`repository.git` in `.goreleaser.yaml`). Scoped to
     one repo, tied to no person. Rotate by adding a new deploy key, updating
     the secret, then deleting the old key. Until the secret exists, releases
     succeed but skip the cask upload.
  3. `git tag v0.1.0 && git push origin v0.1.0`.
- Known at 2026-09-16: goreleaser <= 2.18 renders the cask's post-install hook as
  Homebrew's deprecated `postflight` stanza, so `brew` warns on every command that
  touches the cask; the quarantine clear still runs. goreleaser/goreleaser#6870
  (milestone v2.19) replaces it with `postflight_steps`. Re-check the hook syntax
  when `~> v2` resolves to 2.19.
- Acceptance items that can only be checked against a published release
  (cosign verify-blob, slsa-verifier, brew/install.sh end-to-end) are exercised
  by the release workflow's `verify` job and must be confirmed on v0.1.0.
