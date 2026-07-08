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

err() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }

# The entire body lives in main so a truncated `curl | bash` stream is inert:
# bash executes main only after the closing brace and the call below have parsed.
main() {
  REPO="tensorgroup/openescapement"
  CERT_IDENTITY_REGEXP="^https://github\\.com/${REPO}/\\.github/workflows/release\\.yml@refs/tags/v"
  OIDC_ISSUER="https://token.actions.githubusercontent.com"

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
}

main "$@"
