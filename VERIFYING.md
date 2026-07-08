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
sha256sum --check --ignore-missing checksums.txt
```

On macOS, `shasum -a 256 -c` has no `--ignore-missing` equivalent and will
error on any listed artifact you haven't downloaded. To check just the file(s)
you have, filter the checksums file first:

```sh
grep <artifact> checksums.txt | shasum -a 256 -c
```

## 3. Verify SLSA provenance

```sh
TAG=v0.1.0
BASE=https://github.com/tensorgroup/openescapement/releases/download/$TAG
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
