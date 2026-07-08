# Governance

This document is rendered by [escapement](https://github.com/tensorgroup/openescapement) — do not edit by hand.

Active policy packs:

- **acme-org** 1.4.0
- **eng-dept** 2.1.0

## Tool & service catalog

| Tool / service | Category | Status | Notes | Pack |
|---|---|---|---|---|
| Tailscale | hosting-exposure | preferred | Org tailnet | acme-org |
| Lovable | app-hosting | allowed | POCs only | acme-org |
| Cloudflare Tunnel | hosting-exposure | review-required | Ask #platform | acme-org |
| Raw port forwarding | hosting-exposure | banned |  | acme-org |

## Policy: acme-org 1.4.0

## Secrets
Use Vault.

## For humans
Read the wiki.

## Policy: eng-dept 2.1.0

## CI
All repos use Actions.
