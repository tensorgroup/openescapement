# Vendored htmx

- File: `static/htmx.min.js` (embedded via `//go:embed static/*`, served at `/static/htmx.min.js`)
- Version: 2.0.9
- Source: https://github.com/bigskysoftware/htmx/releases/tag/v2.0.9
- SHA-256: 57d9191515339922bd1356d7b2d80b1ee3b29f1b3a2c65a078bb8b2e8fd9ae5f

This is a vendored JavaScript asset, not a Go dependency. It is never fetched
at runtime. Hardening lives in `static/htmx-config.js`.

## Upgrading

1. Download the new `htmx.min.js` from the release tag above.
2. Verify the download's SHA-256 against the checksum published with the release before trusting it.
3. Diff-review the source against the current file.
4. Update Version and SHA-256 here.
5. Re-run `go test ./internal/portal/...`.
