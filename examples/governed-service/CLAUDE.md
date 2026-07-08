# CLAUDE.md — example governed service

This file is owned by the team. Everything above the managed block is ours;
`esc sync` only touches the block below.

## Build & test

- Install: `pnpm install`
- Test: `pnpm test`
- This service is a demo — it has no real code, it exists to show escapement managing a repo.

<!-- escapement:begin packs=acme-org@0.1.0 hash=sha256:e5abb502c886ee2c2f46029101848bbdfbf8383d58afc229d91049a1c1020c97 -->
> Managed by escapement — do not edit. Run `esc diff` to see source. Team content goes outside this block.

## Secrets & keys

- Never write API keys, tokens, or credentials inline in code, config files, or agent instructions.
- Secrets live in the org vault: https://vault.acme.example — request access in #platform-team.
- API keys for AI providers are issued through https://keys.acme.example (do not create personal provider accounts for org work).
- If you (an agent) encounter a hardcoded secret, stop and flag it for the developer instead of copying or moving it.

## Authentication & authorization

- Never build custom login, session, or password storage. The org SSO service handles authentication: https://sso.acme.example/docs.
- New services must use the approved OIDC flow with the org identity provider; libraries: `acme-auth-go`, `acme-auth-ts`.
- Authorization checks belong at the API layer; use the central policy service where available.

## Hosting & network exposure

- Production workloads run on the paved-road platform (https://platform.acme.example). POCs may use the allowed hosted platforms in the catalog below.
- Sharing a locally-hosted service: use the org tailnet (Tailscale — preferred) or Headscale for lab clusters. Cloudflare Tunnel requires review by #platform-team.
- Never expose a local service to the internet via raw port forwarding or by binding to 0.0.0.0 on a public interface.
- Any newly opened port on a deployed service requires review by the platform team before it ships.

## SDLC requirements

- All code — including POCs and vibe-coded experiments — lives in a tracked repository under the org's GitHub organization.
- CI runs on every PR and must include: tests, dependency scanning, and SAST. Use the shared workflow templates at https://github.com/acme/workflows.
- Do not disable, skip, or `--no-verify` past commit hooks or CI gates.

## Agent behavior

- Orchestration runs are limited to 50 turns per goal; if a task exceeds this, stop and summarize progress for a human instead of continuing.
- Model routing: use fast models for mechanical edits and code generation subagents; reserve reasoning models for design, review, and security-sensitive decisions.
- Escalate to a human before: opening a network port, adopting a new cloud service, adding a new AI model or provider, or granting any credential access.

## Paved path — services that already exist

Before building infrastructure, check these. They exist, they're maintained, and using them is always acceptable:

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
- **Preferred:** Tailscale (hosting-exposure) — Join the org tailnet — https://tailscale.acme.example
- **Preferred:** Acme Kubernetes (paved road) (app-hosting) — https://platform.acme.example/docs
- **Preferred:** Anthropic Claude (API) (model) — Via the org gateway — key issuance at https://keys.acme.example
- **Allowed:** Headscale (hosting-exposure) — Self-hosted tailnet for lab clusters
- **Allowed:** Vercel (app-hosting) — Static and preview deployments
- **Allowed:** Lovable (app-hosting) — POCs only — production must move to an approved platform
- **Review required:** Cloudflare Tunnel (hosting-exposure) — Request review in #platform-team
- **Review required:** Model-vendor hosting (Claude artifacts, OpenAI apps) (app-hosting) — Fine for demos; data-handling review before real user data
- **Review required:** Self-hosted open-weights models (model) — Talk to #ml-platform about GPU pool and eval requirements
- **Banned:** Raw port forwarding (hosting-exposure) — No direct exposure of local services to the internet
<!-- escapement:end -->
