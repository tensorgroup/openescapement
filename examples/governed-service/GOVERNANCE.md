# Governance

This document is rendered by [escapement](https://github.com/tensorgroup/openescapement) — do not edit by hand.

Active policy packs:

- **acme-org** 0.1.1 — Example org-wide AI governance baseline (copy and adapt)

## Tool & service catalog

| Tool / service | Category | Status | Notes | Pack |
|---|---|---|---|---|
| Tailscale | hosting-exposure | preferred | Join the org tailnet — https://tailscale.acme.example | acme-org |
| Acme Kubernetes (paved road) | app-hosting | preferred | https://platform.acme.example/docs | acme-org |
| Anthropic Claude (API) | model | preferred | Via the org gateway — key issuance at https://keys.acme.example | acme-org |
| Headscale | hosting-exposure | allowed | Self-hosted tailnet for lab clusters | acme-org |
| Vercel | app-hosting | allowed | Static and preview deployments | acme-org |
| Lovable | app-hosting | allowed | POCs only. Production must move to an approved platform | acme-org |
| Cloudflare Tunnel | hosting-exposure | review-required | Request review in #platform-team | acme-org |
| Model-vendor hosting (Claude artifacts, OpenAI apps) | app-hosting | review-required | Fine for demos; data-handling review before real user data | acme-org |
| Self-hosted open-weights models | model | review-required | Talk to #ml-platform about GPU pool and eval requirements | acme-org |
| Raw port forwarding | hosting-exposure | banned | No direct exposure of local services to the internet | acme-org |

## Policy: acme-org 0.1.1

## Secrets & keys

- Never write API keys, tokens, or credentials inline in code, config files, or agent instructions.
- Keep secrets in the org vault: https://vault.acme.example. Request access in #platform-team.
- Get API keys for AI providers from https://keys.acme.example. Do not create personal provider accounts for org work: the org can rotate and revoke a centrally issued key, and cannot touch a personal one.
- If you find a hardcoded secret, stop and flag it to the developer. Do not copy or move it: moving a leaked secret spreads it without rotating it.

## Authentication & authorization

- Never build custom login, session, or password storage. The org SSO service handles authentication: https://sso.acme.example/docs.
- Use the approved OIDC flow with the org identity provider for every new service. The libraries are `acme-auth-go` and `acme-auth-ts`.
- Put authorization checks at the API layer. Use the central policy service where it is available.

## Hosting & network exposure

- Run production workloads on the paved-road platform (https://platform.acme.example). A POC may use one of the allowed hosted platforms in the catalog below.
- To share a locally hosted service, use the org tailnet (Tailscale). Use Headscale only for lab clusters. Cloudflare Tunnel requires review by #platform-team.
- Never expose a local service to the internet via raw port forwarding or by binding to 0.0.0.0 on a public interface.
- Any newly opened port on a deployed service requires review by #platform-team before it ships.

## SDLC requirements

- Keep all code, including POCs and vibe-coded experiments, in a tracked repository under the org's GitHub organization.
- Run CI on every PR with tests, dependency scanning, and SAST. Use the shared workflow templates at https://github.com/acme/workflows.
- Do not disable, skip, or `--no-verify` past commit hooks or CI gates. A red gate is a finding to fix, not an obstacle to route around.

## Agent behavior

- Stop an orchestration run at 50 turns per goal. Leave a progress summary for a human. The cap exists so a run that is not converging is caught by a person, not by a budget alarm.
- Use fast models for mechanical edits and code-generation subagents. Reserve reasoning models for design, review, and security-sensitive decisions.
- Escalate to a human before: opening a network port, adopting a new cloud service, adding a new AI model or provider, or granting any credential access.

## Paved path — services that already exist

Before building infrastructure, check this table. These services exist, are maintained, and are always acceptable to use:

| Need | Use | Where |
|---|---|---|
| Login / SSO | Org identity provider | https://sso.acme.example/docs |
| API keys (AI providers) | Central key issuance | https://keys.acme.example |
| Secrets | Org vault | https://vault.acme.example |
| API gateway | Org gateway | https://gateway.acme.example |
| Hosting | Paved-road platform | https://platform.acme.example |
| Docs | Engineering handbook | https://handbook.acme.example |

If none of these fit, ask in #platform-team before rolling your own.
