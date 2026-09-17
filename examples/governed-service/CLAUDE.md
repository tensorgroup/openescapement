# CLAUDE.md — example governed service

This file is owned by the team. Everything above the managed block is ours;
`esc sync` only touches the block below.

## Build & test

- Install: `pnpm install`
- Test: `pnpm test`
- This service is a demo — it has no real code, it exists to show escapement managing a repo.

<!-- escapement:begin packs=acme-org@0.1.1 hash=sha256:4127f9f00a25a0e7c70e411b08ee1cc7662f1c193470a485e4994713e63f1b80 -->
> Managed by escapement. Do not edit. Run `esc diff` to see source. Team content goes outside this block.

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

## Tool & service policy
- **Preferred:** Tailscale (hosting-exposure) - Join the org tailnet — https://tailscale.acme.example
- **Preferred:** Acme Kubernetes (paved road) (app-hosting) - https://platform.acme.example/docs
- **Preferred:** Anthropic Claude (API) (model) - Via the org gateway — key issuance at https://keys.acme.example
- **Allowed:** Headscale (hosting-exposure) - Self-hosted tailnet for lab clusters
- **Allowed:** Vercel (app-hosting) - Static and preview deployments
- **Allowed:** Lovable (app-hosting) - POCs only. Production must move to an approved platform
- **Review required:** Cloudflare Tunnel (hosting-exposure) - Request review in #platform-team
- **Review required:** Model-vendor hosting (Claude artifacts, OpenAI apps) (app-hosting) - Fine for demos; data-handling review before real user data
- **Review required:** Self-hosted open-weights models (model) - Talk to #ml-platform about GPU pool and eval requirements
- **Banned:** Raw port forwarding (hosting-exposure) - No direct exposure of local services to the internet
<!-- escapement:end -->
