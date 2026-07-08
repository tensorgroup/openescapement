# Release Automation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `git tag v0.1.0 && git push --tags` produces signed, reproducible binaries with SLSA provenance, SBOMs, and a GitHub Release — plus CI, an install script, and verification docs.

**Architecture:** goreleaser builds/archives/checksums the five target platforms; cosign keyless (OIDC via GitHub Actions) signs the checksums file and every archive; the slsa-github-generator reusable workflow attests provenance at SLSA build level 3; a final CI job re-verifies the published signature as a smoke test. `esc version` gains Commit/Date stamped via ldflags.

**Tech Stack:** goreleaser v2, cosign v3 (sigstore bundle format), syft (CycloneDX SBOM), slsa-github-generator v2.1.0, GitHub Actions.

**Spec:** `docs/roadmap/release-automation.md`

## Global Constraints

- **No new Go module dependencies.** `gopkg.in/yaml.v3` remains the only entry in `go.mod`. goreleaser/cosign/syft are CI tools invoked externally, never imported.
- Go 1.24 (`go-version-file: go.mod` in all workflows). Module `github.com/tensorgroup/openescapement`, binary `esc`, main package `./cmd/esc`.
- Builds must be reproducible: `CGO_ENABLED=0`, `-trimpath`, `mod_timestamp` = commit timestamp, and stamp `{{.CommitDate}}` (not build time) as the date.
- Release targets exactly: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`.
- The release job MUST be gated on the full test suite passing (`needs: test`).
- The slsa-github-generator reusable workflow MUST be referenced by tag (`@v2.1.0`) — the SLSA tooling rejects non-tag refs.
- **Deviations from the spec doc (already decided, do not "fix"):** cosign v3 emits a single sigstore bundle per artifact (`<artifact>.sigstore.json`) instead of the spec's `.sig` + `.pem` pair — same trust guarantees, current tooling. goreleaser deprecated `brews:`; we use `homebrew_casks:`.
- Working branch: `worktree-release-automation` (worktree at `.claude/worktrees/release-automation`). Commit after every task.
- Local validation runs goreleaser via `go run github.com/goreleaser/goreleaser/v2@latest` (not installed locally; this does NOT touch go.mod) and actionlint via `go run github.com/rhysd/actionlint/cmd/actionlint@latest`.

---

### Task 1: Stamp Commit and Date into `esc version`

**Files:**
- Modify: `internal/cli/cli.go:20-21` (vars) and `internal/cli/cli.go:62-64` (version case)
- Test: `internal/cli/cli_test.go` (append; package is `cli`, internal)

**Interfaces:**
- Produces: package vars `cli.Version`, `cli.Commit`, `cli.Date` (all `string`) — Task 2's ldflags set all three via `-X github.com/tensorgroup/openescapement/internal/cli.<Name>=`.
- Output format (Task 5's docs quote it): `esc <version> (commit <commit>, built <date>)\n`

- [ ] **Step 1: Write the failing test**

Append to `internal/cli/cli_test.go`:

```go
func TestVersionOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(t.TempDir(), []string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	want := fmt.Sprintf("esc %s (commit %s, built %s)\n", Version, Commit, Date)
	if got := stdout.String(); got != want {
		t.Errorf("version output = %q, want %q", got, want)
	}
}
```

Add `"fmt"` to the test file's imports if not already present.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestVersionOutput -v`
Expected: compile FAIL — `undefined: Commit` and `undefined: Date`.

- [ ] **Step 3: Write minimal implementation**

In `internal/cli/cli.go`, replace lines 20–21:

```go
// Version is stamped at release time via -ldflags.
var Version = "0.1.0-dev"
```

with:

```go
// Version, Commit, and Date are stamped at release time via -ldflags
// (see .goreleaser.yaml). Defaults identify from-source dev builds.
var (
	Version = "0.1.0-dev"
	Commit  = "unknown"
	Date    = "unknown"
)
```

and replace the version case body (`fmt.Fprintf(stdout, "esc %s\n", Version)`) with:

```go
fmt.Fprintf(stdout, "esc %s (commit %s, built %s)\n", Version, Commit, Date)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ && go vet ./... && test -z "$(gofmt -l .)"`
Expected: PASS, no vet or fmt findings.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/cli.go internal/cli/cli_test.go
git commit -m "feat(cli): stamp commit and date into esc version output"
```

---

### Task 2: `.goreleaser.yaml`

**Files:**
- Create: `.goreleaser.yaml` (repo root)

**Interfaces:**
- Consumes: `cli.Version` / `cli.Commit` / `cli.Date` from Task 1.
- Produces: `dist/` artifacts CI relies on — archives named `esc_<version>_<os>_<arch>.tar.gz` (`.zip` on windows), `checksums.txt`, per-artifact `*.sigstore.json`, per-archive `*.cyclonedx.sbom.json`. Task 4 signs via the `signs:` entries (cosign must be on PATH there); Task 5's install.sh depends on these exact artifact names.

- [ ] **Step 1: Write `.goreleaser.yaml`**

```yaml
# Release configuration — see docs/roadmap/release-automation.md.
# Validate with: go run github.com/goreleaser/goreleaser/v2@latest check
version: 2

project_name: esc

builds:
  - id: esc
    main: ./cmd/esc
    binary: esc
    env:
      - CGO_ENABLED=0
    flags:
      - -trimpath
    ldflags:
      - -s -w
      - -X github.com/tensorgroup/openescapement/internal/cli.Version={{ .Version }}
      - -X github.com/tensorgroup/openescapement/internal/cli.Commit={{ .FullCommit }}
      - -X github.com/tensorgroup/openescapement/internal/cli.Date={{ .CommitDate }}
    # Commit time, not build time — keeps archives byte-reproducible.
    mod_timestamp: "{{ .CommitTimestamp }}"
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    ignore:
      - goos: windows
        goarch: arm64

archives:
  - id: esc
    formats: [tar.gz]
    format_overrides:
      - goos: windows
        formats: [zip]
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: checksums.txt
  algorithm: sha256

sboms:
  - id: archives
    artifacts: archive
    documents:
      - "${artifact}.cyclonedx.sbom.json"
    args: ["$artifact", "--output", "cyclonedx-json=$document"]

# Keyless signing (OIDC from the Actions job); cosign v3 bundle format.
signs:
  - id: checksums
    cmd: cosign
    signature: "${artifact}.sigstore.json"
    artifacts: checksum
    args: ["sign-blob", "--bundle=${signature}", "${artifact}", "--yes"]
  - id: archives
    cmd: cosign
    signature: "${artifact}.sigstore.json"
    artifacts: archive
    args: ["sign-blob", "--bundle=${signature}", "${artifact}", "--yes"]

homebrew_casks:
  - name: esc
    repository:
      owner: tensorgroup
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
    # Skip (instead of fail) when the tap token secret isn't configured yet.
    skip_upload: "{{ if .Env.HOMEBREW_TAP_GITHUB_TOKEN }}false{{ else }}true{{ end }}"
    homepage: https://github.com/tensorgroup/openescapement
    description: Deterministic governance for AI usage
    license: Apache-2.0
    url:
      verified: github.com/tensorgroup/openescapement
    hooks:
      post:
        # Binaries are cosign-signed but not Apple-notarized; clear quarantine.
        install: |
          if OS.mac?
            system_command "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "#{staged_path}/esc"]
          end

changelog:
  use: github-native

release:
  prerelease: auto
```

- [ ] **Step 2: Validate the config**

Run: `go run github.com/goreleaser/goreleaser/v2@latest check`
Expected: `1 configuration file(s) validated` / no deprecation warnings. (First run downloads goreleaser; go.mod must remain unchanged — verify with `git diff go.mod go.sum` → empty.)

- [ ] **Step 3: Snapshot build and verify stamping**

Run:
```bash
go run github.com/goreleaser/goreleaser/v2@latest build --snapshot --clean
./dist/esc_darwin_arm64_v8.0/esc version
```
(Adjust the dist subdir to what goreleaser prints if the arm64 suffix differs.)
Expected: build succeeds for all 5 targets; output matches `esc <snapshot-version> (commit <full-sha>, built <RFC3339 date>)` with no `unknown` fields.

- [ ] **Step 4: Commit**

```bash
git add .goreleaser.yaml .gitignore
git commit -m "feat(release): goreleaser config — reproducible builds, cosign keyless, sbom, homebrew cask"
```
(Only add `.gitignore` if you added `dist/` to it — check first; add `dist/` if absent.)

---

### Task 3: CI Workflow

**Files:**
- Create: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: `.goreleaser.yaml` from Task 2 (the snapshot job proves it compiles).
- Produces: the `ci` workflow; Task 6's docs may reference it. Test/lint gate mirrors what Task 4's release gate runs.

- [ ] **Step 1: Write `.github/workflows/ci.yml`**

```yaml
name: ci

on:
  push:
    branches: [main]
  pull_request:

permissions:
  contents: read

jobs:
  test:
    strategy:
      fail-fast: false
      matrix:
        # Path/symlink behavior must be exercised on both linux and darwin.
        os: [ubuntu-latest, macos-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: gofmt
        run: |
          out=$(gofmt -l .)
          if [ -n "$out" ]; then echo "gofmt needed:"; echo "$out"; exit 1; fi
      - run: go vet ./...
      - run: go test ./...
      - name: race detector (engine + cli)
        run: go test -race ./internal/engine/... ./internal/cli/...

  snapshot:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: goreleaser config compiles
        uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: "~> v2"
          args: build --snapshot --clean
```

- [ ] **Step 2: Lint the workflow**

Run: `go run github.com/rhysd/actionlint/cmd/actionlint@latest .github/workflows/ci.yml`
Expected: no output (clean). Verify `git diff go.mod go.sum` is still empty.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: test matrix (linux+macos), race, gofmt/vet gates, goreleaser snapshot check"
```

---

### Task 4: Release Workflow

**Files:**
- Create: `.github/workflows/release.yml`

**Interfaces:**
- Consumes: `.goreleaser.yaml` (Task 2). Signing identity produced here: certificate identity `https://github.com/tensorgroup/openescapement/.github/workflows/release.yml@refs/tags/v*`, OIDC issuer `https://token.actions.githubusercontent.com` — Task 5's install.sh and VERIFYING.md must use these exact values.
- Produces: on `v*` tag push — GitHub Release with archives, `checksums.txt`, `*.sigstore.json`, SBOMs, and `multiple.intoto.jsonl` (SLSA provenance from the generator).

- [ ] **Step 1: Write `.github/workflows/release.yml`**

```yaml
name: release

on:
  push:
    tags: ["v*"]

permissions: {}

jobs:
  # Never publish a release whose tests didn't pass in CI.
  test:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go vet ./...
      - run: go test ./...
      - run: go test -race ./internal/engine/... ./internal/cli/...

  goreleaser:
    needs: test
    runs-on: ubuntu-latest
    permissions:
      contents: write   # create the release and upload assets
      id-token: write   # cosign keyless OIDC
    outputs:
      hashes: ${{ steps.hashes.outputs.hashes }}
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0   # full history for release notes
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - uses: sigstore/cosign-installer@v3
      - uses: anchore/sbom-action/download-syft@v0
      - uses: goreleaser/goreleaser-action@v6
        id: goreleaser
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          # Empty until the tap repo + secret exist; goreleaser then skips the cask upload.
          HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
      - name: Collect subject hashes for SLSA provenance
        id: hashes
        env:
          ARTIFACTS: ${{ steps.goreleaser.outputs.artifacts }}
        run: |
          set -euo pipefail
          checksum_file=$(echo "$ARTIFACTS" | jq -r '.[] | select (.type=="Checksum") | .path')
          echo "hashes=$(base64 -w0 < "$checksum_file")" >> "$GITHUB_OUTPUT"

  provenance:
    needs: goreleaser
    permissions:
      actions: read     # read the workflow path
      id-token: write   # sign the provenance
      contents: write   # upload the attestation to the release
    uses: slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml@v2.1.0
    with:
      base64-subjects: ${{ needs.goreleaser.outputs.hashes }}
      upload-assets: true

  # Smoke test: verify the just-published signature exactly as VERIFYING.md tells users to.
  verify:
    needs: [goreleaser, provenance]
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - uses: sigstore/cosign-installer@v3
      - name: Download published checksums and signature
        env:
          GH_TOKEN: ${{ github.token }}
        run: |
          gh release download "${GITHUB_REF_NAME}" \
            --repo "${GITHUB_REPOSITORY}" \
            --pattern 'checksums.txt*'
      - name: cosign verify-blob
        run: |
          cosign verify-blob \
            --bundle checksums.txt.sigstore.json \
            --certificate-identity-regexp '^https://github\.com/tensorgroup/openescapement/\.github/workflows/release\.yml@refs/tags/v' \
            --certificate-oidc-issuer https://token.actions.githubusercontent.com \
            checksums.txt
```

- [ ] **Step 2: Lint the workflow**

Run: `go run github.com/rhysd/actionlint/cmd/actionlint@latest .github/workflows/release.yml`
Expected: no output. (actionlint cannot validate the reusable-workflow inputs; that's exercised on the first real tag.)

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci: tag-triggered release — test gate, goreleaser, SLSA provenance, signature smoke test"
```

---

### Task 5: `install.sh` and `VERIFYING.md`

**Files:**
- Create: `install.sh` (repo root — fetched raw from main by users)
- Create: `VERIFYING.md` (repo root)

**Interfaces:**
- Consumes: artifact names from Task 2 (`esc_<version>_<os>_<arch>.tar.gz`, `checksums.txt`, `checksums.txt.sigstore.json` — note goreleaser strips the tag's `v` in `{{ .Version }}`), signing identity from Task 4.
- Produces: `sh install.sh` (or `curl … | bash`) installs a verified `esc`; VERIFYING.md documents manual verification.

- [ ] **Step 1: Write `install.sh`**

```bash
#!/usr/bin/env bash
# Install esc: download the release archive, verify its cosign signature,
# check its sha256, and place the binary on PATH.
#
#   curl -sSfL https://raw.githubusercontent.com/tensorgroup/openescapement/main/install.sh | bash
#
# Options (env vars):
#   ESC_VERSION      release tag to install, e.g. v0.1.0 (default: latest)
#   ESC_INSTALL_DIR  destination directory (default: ~/.local/bin)
#   ESC_SKIP_VERIFY  set to 1 to proceed without cosign (NOT recommended)
set -euo pipefail

REPO="tensorgroup/openescapement"
CERT_IDENTITY_REGEXP="^https://github\\.com/${REPO}/\\.github/workflows/release\\.yml@refs/tags/v"
OIDC_ISSUER="https://token.actions.githubusercontent.com"

err() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux|darwin) ;;
  *) err "unsupported OS: $os (windows: download the zip from the releases page)" ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) err "unsupported architecture: $arch" ;;
esac

version="${ESC_VERSION:-}"
if [ -z "$version" ]; then
  # Resolve the latest tag from the release redirect — no API token or jq needed.
  version=$(curl -sSfI "https://github.com/${REPO}/releases/latest" |
    tr -d '\r' | awk -F'/tag/' 'tolower($1) ~ /^location:/ {print $2}')
  [ -n "$version" ] || err "could not resolve the latest release tag"
fi

archive="esc_${version#v}_${os}_${arch}.tar.gz"
base="https://github.com/${REPO}/releases/download/${version}"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading esc ${version} (${os}/${arch})..."
curl -sSfL -o "$tmp/$archive" "$base/$archive"
curl -sSfL -o "$tmp/checksums.txt" "$base/checksums.txt"
curl -sSfL -o "$tmp/checksums.txt.sigstore.json" "$base/checksums.txt.sigstore.json"

if command -v cosign >/dev/null 2>&1; then
  echo "Verifying signature (cosign keyless)..."
  cosign verify-blob \
    --bundle "$tmp/checksums.txt.sigstore.json" \
    --certificate-identity-regexp "$CERT_IDENTITY_REGEXP" \
    --certificate-oidc-issuer "$OIDC_ISSUER" \
    "$tmp/checksums.txt" >/dev/null
elif [ "${ESC_SKIP_VERIFY:-0}" = "1" ]; then
  echo "WARNING: cosign not found and ESC_SKIP_VERIFY=1 — skipping signature verification." >&2
else
  err "cosign not found. Install cosign (https://docs.sigstore.dev/cosign/system_config/installation/) to verify this download, or re-run with ESC_SKIP_VERIFY=1 to skip verification (not recommended). See VERIFYING.md."
fi

echo "Checking sha256..."
expected=$(awk -v f="$archive" '$2 == f {print $1}' "$tmp/checksums.txt")
[ -n "$expected" ] || err "no checksum entry for $archive"
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp/$archive" | awk '{print $1}')
else
  actual=$(shasum -a 256 "$tmp/$archive" | awk '{print $1}')
fi
[ "$expected" = "$actual" ] || err "sha256 mismatch for $archive"

install_dir="${ESC_INSTALL_DIR:-$HOME/.local/bin}"
mkdir -p "$install_dir"
tar -xzf "$tmp/$archive" -C "$tmp" esc
install -m 0755 "$tmp/esc" "$install_dir/esc"

echo "Installed $("$install_dir/esc" version) to $install_dir/esc"
case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) echo "NOTE: $install_dir is not on your PATH." ;;
esac
```

- [ ] **Step 2: Syntax-check the script**

Run: `bash -n install.sh && chmod +x install.sh`
Expected: no output. (Full end-to-end runs only after the first published release; the release `verify` job covers the signature path.)

- [ ] **Step 3: Write `VERIFYING.md`**

```markdown
# Verifying esc release artifacts

Every release is built by GitHub Actions from a tagged commit, signed with
[cosign](https://docs.sigstore.dev/) keyless signing, and attested with
[SLSA](https://slsa.dev/) build-level-3 provenance. No long-lived signing keys
exist; trust is anchored in the repository's release workflow identity.

Replace `v0.1.0` with the tag you are verifying.

## 1. Verify the checksums signature (cosign)

```sh
TAG=v0.1.0
BASE=https://github.com/tensorgroup/openescapement/releases/download/$TAG
curl -sSfLO "$BASE/checksums.txt"
curl -sSfLO "$BASE/checksums.txt.sigstore.json"

cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github\.com/tensorgroup/openescapement/\.github/workflows/release\.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
```

Expected output: `Verified OK`.

Each archive also ships its own `<archive>.sigstore.json` bundle and can be
verified the same way.

## 2. Verify your download against the checksums

```sh
sha256sum --check --ignore-missing checksums.txt   # macOS: shasum -a 256 -c
```

## 3. Verify SLSA provenance

```sh
curl -sSfLO "$BASE/multiple.intoto.jsonl"
curl -sSfLO "$BASE/esc_0.1.0_linux_amd64.tar.gz"

slsa-verifier verify-artifact esc_0.1.0_linux_amd64.tar.gz \
  --provenance-path multiple.intoto.jsonl \
  --source-uri github.com/tensorgroup/openescapement \
  --source-tag $TAG
```

Expected output ends with `PASSED: SLSA verification passed`.
([slsa-verifier install instructions](https://github.com/slsa-framework/slsa-verifier#installation))

## 4. SBOMs

Each archive has a CycloneDX SBOM (`<archive>.cyclonedx.sbom.json`) generated
by syft at build time, listed in `checksums.txt` like every other artifact.
```

- [ ] **Step 4: Commit**

```bash
git add install.sh VERIFYING.md
git commit -m "feat(release): verified install script and VERIFYING.md"
```

---

### Task 6: Documentation Updates

**Files:**
- Modify: `README.md:31-44` (Quickstart install block)
- Modify: `SECURITY.md:27` (remove the "lands with first release" caveat)
- Modify: `CHANGELOG.md` (under `## [Unreleased]`)
- Modify: `docs/roadmap/release-automation.md` (status + deviations + manual prerequisites)

**Interfaces:**
- Consumes: install paths from Task 5, verification story from Tasks 2/4.

- [ ] **Step 1: Update README Quickstart**

Replace the single `go install` line inside the Quickstart code block (`README.md:34`) with an install section. Keep the rest of the Quickstart (esc init/sync/status) unchanged:

```markdown
## Install

```sh
# Verified install (downloads, cosign-verifies, and installs the latest release):
curl -sSfL https://raw.githubusercontent.com/tensorgroup/openescapement/main/install.sh | bash

# Homebrew:
brew install tensorgroup/tap/esc

# Or build from source:
go install github.com/tensorgroup/openescapement/cmd/esc@latest
```

Release binaries are cosign-signed with SLSA build-level-3 provenance — see
[VERIFYING.md](VERIFYING.md).
```

(Adjust placement to the file's actual structure: add the `## Install` section immediately before `## Quickstart` and remove the `go install` line from the Quickstart block.)

- [ ] **Step 2: Update SECURITY.md**

Replace line 27:

```markdown
- **Binary release signing** (cosign/SLSA provenance) lands with the first tagged release; until then, build from source.
```

with:

```markdown
- **Binary releases are signed**: cosign keyless signatures (sigstore bundles) plus SLSA build-level-3 provenance on every artifact. See [VERIFYING.md](VERIFYING.md) for verification commands.
```

- [ ] **Step 3: Update CHANGELOG.md**

Under `## [Unreleased]`, add (create an `### Added` subsection if the file uses Keep-a-Changelog subsections — match the existing entries' style):

```markdown
- Release automation: goreleaser builds for linux/darwin/windows, cosign keyless
  signing, SLSA v1 build-level-3 provenance, CycloneDX SBOMs, Homebrew cask,
  verified `install.sh`, and `VERIFYING.md`.
- `esc version` now prints commit and build date alongside the version.
- CI workflow: test matrix (linux + macos), race detector on engine/cli,
  gofmt/vet gates, goreleaser snapshot check.
```

- [ ] **Step 4: Update the roadmap doc**

In `docs/roadmap/release-automation.md`:
- Change the status line to: `**Status:** Implemented (pending first tagged release). Plan: docs/superpowers/plans/2026-07-08-release-automation.md`
- Under **Notes**, append:

```markdown
- Implementation deviations: cosign v3 emits a single sigstore bundle
  (`<artifact>.sigstore.json`) per artifact instead of separate `.sig` + `.pem`;
  goreleaser's `brews:` is deprecated, so the tap ships a Cask
  (`homebrew_casks:`) with a quarantine-clearing post-install hook.
- Manual prerequisites before tagging v0.1.0:
  1. Create the `tensorgroup/homebrew-tap` repo (empty is fine).
  2. Add a `HOMEBREW_TAP_GITHUB_TOKEN` repo secret (fine-grained PAT,
     contents: write on the tap). Until it exists, releases succeed but skip
     the cask upload.
  3. `git tag v0.1.0 && git push origin v0.1.0`.
- Acceptance items that can only be checked against a published release
  (cosign verify-blob, slsa-verifier, brew/install.sh end-to-end) are exercised
  by the release workflow's `verify` job and must be confirmed on v0.1.0.
```

- Check the acceptance checklist boxes that are now satisfied by code (`esc version` prints version+commit+date; README/SECURITY updates), leaving release-dependent boxes unchecked.

- [ ] **Step 5: Verify and commit**

Run: `go test ./... && test -z "$(gofmt -l .)"`
Expected: all PASS.

```bash
git add README.md SECURITY.md CHANGELOG.md docs/roadmap/release-automation.md
git commit -m "docs: install paths, signing/verification story, release-automation status"
```
