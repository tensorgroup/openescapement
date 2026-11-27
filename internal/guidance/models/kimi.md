# Kimi

Moonshot AI's Kimi ships one flagship line (K3.1 Pro and its High-Speed variant) alongside a dedicated coding model (K2.7 Code) and a general multimodal model (K2.6), all OpenAI- and Anthropic-API compatible. Kimi Code, Moonshot's terminal coding agent, runs on this lineup and layers in sub-agent isolation: a read-only explore agent, a tool-free plan agent, and a full-access coder agent. Refreshed 2026-10-24: K3.1 Pro and K3.1 Pro High-Speed added; K3 moved to legacy, which Moonshot now calls its previous-generation flagship; the K2 series' end of life noted.

## Models

### Kimi K3.1 Pro (kimi-k3.1-pro) - frontier

The first model of the K3.1 series and Moonshot's most capable flagship: a post-trained version of K3 with substantially more post-training compute, 3T-class, with native visual understanding and a 1,048,576-token context window. Moonshot reports gains over K3 in coding, agentic work, and knowledge work; open weights are due by 2026-11-04. $2 / $10 per million input/output tokens (cache hits $0.20), cheaper than K3 on both sides. Recommended roles: planning, plan-check, review, and day-to-day coding. In Kimi Code it appears as `kimi-code/k3.1-pro` (observed in a Kimi Code 2.3.1 configuration on 2026-10-24; the CLI docs still show `kimi-code/k3`).

### Kimi K3.1 Pro High-Speed (kimi-k3.1-pro-highspeed) - fast

The same model served at about 2-3x the speed of K3.1 Pro, at double the price: $4 / $20 per million tokens (cache hits $0.20). Recommended roles: coding and bulk, for fast reads and rapid iteration where latency matters more than cost. In Kimi Code: `kimi-code/k3.1-pro-highspeed` (observed 2026-10-24).

### Kimi K3 (kimi-k3) - legacy

The previous-generation flagship: 2.8 trillion parameters, a 1M-token context window, configurable reasoning_effort (low, high, or max). $3 / $15 per million tokens, more than K3.1 Pro on both sides. Migrate pins to `kimi-k3.1-pro`.

### Kimi K2.7 Code (kimi-k2.7-code) - mid

The dedicated coding model, 256K context, tuned to follow instructions reliably in long contexts and land more coding tasks successfully than the general-purpose line. $0.95 / $4.00 per million input/output tokens (cache hits $0.19). Recommended role: coding. Part of the K2 series, which closes to newly registered users on 2026-10-31 and retires by the end of December 2026.

### Kimi K2.7 Code High-Speed (kimi-k2.7-code-highspeed) - fast

A high-speed variant of K2.7 Code, about 180 tokens/s and up to roughly 260 tokens/s in short-context runs, for rapid iterative coding and high-volume edits. $1.90 / $8.00 per million tokens (cache hits $0.38). Recommended roles: coding and bulk. Same K2-series end of life as K2.7 Code.

### Kimi K2.6 (kimi-k2.6) - mid

A general multimodal model (vision and text, thinking and non-thinking modes) for dialogue and broader agent tasks outside the dedicated coding line. $0.95 / $4.00 per million tokens (cache hits $0.16). Recommended roles: coding and review. Same K2-series end of life as K2.7 Code.

### Kimi K2.5 (kimi-k2.5) - legacy

Superseded by K3, closed to new accounts, and sunset platform-wide on 2026-08-31; the id no longer works.

## Governance

- Require human review before merging changes the `coder` sub-agent in Kimi Code produced with shell access and file-write permissions; the `explore` and `plan` sub-agents are read-only or tool-free by design and carry less risk.
- Do not rely on Kimi Code's permission rules to stop writes in non-interactive (`-p`) runs. Non-interactive runs use the `auto` permission mode, deny rules did not stop writes in a 2026-10-24 test of Kimi Code 2.3.1, and hooks fail open by Kimi Code's own documentation. Run an unattended Kimi Code session under an OS sandbox that denies writes outside its working directory.
- Route planning, plan-check, and review to K3.1 Pro; fast reads and high-volume edits to K3.1 Pro High-Speed; day-to-day coding to K3.1 Pro, or to K2.7 Code where a configuration already pins it.
- Plan the move off the K2 series (K2.7 Code, K2.7 Code High-Speed, K2.6) before the end of December 2026; new accounts lose access on 2026-10-31.
- Migrate K3 pins to K3.1 Pro, which costs less per token.
- The K2.5 and moonshot-v1 ids were sunset on 2026-08-31 and no longer work; remove any configuration that still names them.

## Access routes

Prices as of 2026-10-24.

| Route | What you pay | Tool | Limits, per the vendor |
|---|---|---|---|
| Plan + vendor CLI | Kimi Code plan, subscription-based; tiers and prices are on the Kimi Code site, which this note could not read on 2026-10-24 | Kimi Code CLI (`kimi`) | One subscription covers desktop, CLI, and IDE. The Kimi Code API is separate from the Kimi API Platform, and keys are not interchangeable. |
| Direct API | K3.1 Pro $2 / $10; K3.1 Pro High-Speed $4 / $20; K3 $3 / $15; K2.7 Code $0.95 / $4.00; K2.7 Code High-Speed $1.90 / $8.00; K2.6 $0.95 / $4.00 per million input/output tokens (cache hits $0.20 on K3.1) | Kimi Code with an API Platform key, or any harness | Metered per token. Context caching is billed automatically. |
| OpenRouter | `moonshotai/kimi-k3.1-pro` $2 / $10 | Any OpenAI-compatible harness | Provider prices can change without notice. |

- When a plan pays for itself: a Kimi Code plan at $P a month matches P / 10 million K3.1 Pro output tokens at the API's $10 per M, or P / 20 million on High-Speed. Read the current tier prices on the Kimi Code site before deciding.
- Run repo-aware agent work through Kimi Code on a plan for daily use, under an OS sandbox (see Governance).
- Use a metered key for fast, small, occasional calls.

## Sources

- https://platform.kimi.ai/docs/models
- https://platform.kimi.ai/docs/pricing/chat-k3
- https://platform.kimi.ai/docs/pricing/chat (K2-series prices: K2.7 Code, K2.7 Code High-Speed, K2.6; retrieved 2026-10-24)
- https://github.com/MoonshotAI/kimi-code/blob/main/docs/en/customization/agents.md
- https://platform.kimi.ai/docs/guide/product-plans (Kimi Code is subscription-based and separate from the API Platform; retrieved 2026-10-24)
- https://www.kimi.ai/code (Kimi Code plan tiers; not retrievable on 2026-10-24)
- https://openrouter.ai/api/v1/models (OpenRouter prices; retrieved 2026-10-24)
- https://platform.kimi.ai/docs/guide/kimi-k3-1-pro-quickstart (retrieved 2026-10-24)
- https://github.com/MoonshotAI/kimi-code/blob/main/docs/en/reference/kimi-command.md (`-p` runs use the `auto` permission mode; retrieved 2026-10-24)
- https://github.com/MoonshotAI/kimi-code/blob/main/docs/en/configuration/config-files.md (`[[permission.rules]]` matching: in order, first match wins; retrieved 2026-10-24)
- https://github.com/MoonshotAI/kimi-code/blob/main/docs/en/customization/hooks.md (hooks fail open on error, timeout, or crash; retrieved 2026-10-24)
- First-party observation, 2026-10-24: Kimi Code 2.3.1 in `-p` mode ignored `[[permission.rules]]` deny rules for `Write`, `Edit`, `Bash`, and `Agent`; every attempted write succeeded unsandboxed, and an OS sandbox denied the same writes. The `-p` docs say static deny rules remain in effect; this test contradicts them.
