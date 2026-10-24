# Kimi

Moonshot AI's Kimi ships one flagship reasoning model (K3) alongside a dedicated coding line (K2.7 Code) and a general multimodal model (K2.6), all OpenAI- and Anthropic-API compatible. Kimi Code, Moonshot's terminal coding agent, runs on this lineup and layers in sub-agent isolation: a read-only explore agent, a tool-free plan agent, and a full-access coder agent.

## Models

### Kimi K3 (kimi-k3) - frontier

Moonshot's flagship: 2.8 trillion parameters, a 1M-token context window, tuned for long-horizon software engineering and deep reasoning, with a configurable reasoning_effort (low, high, or max). Recommended roles: planning, plan-check, and review.

### Kimi K2.7 Code (kimi-k2.7-code) - mid

The dedicated coding model, 256K context, tuned to follow instructions reliably in long contexts and land more coding tasks successfully than the general-purpose line. Recommended role: coding.

### Kimi K2.7 Code High-Speed (kimi-k2.7-code-highspeed) - fast

A high-speed variant of K2.7 Code, up to roughly 260 tokens/s in short-context runs, for rapid iterative coding and high-volume edits. Recommended roles: coding and bulk.

### Kimi K2.6 (kimi-k2.6) - mid

A general multimodal model (vision and text, thinking and non-thinking modes) for dialogue and broader agent tasks outside the dedicated coding line. Recommended roles: coding and review.

### Kimi K2.5 (kimi-k2.5) - legacy

Superseded by K3. Moonshot has closed it to new accounts and is sunsetting it platform-wide on 2026-08-31; migrate any remaining routing off it before that date.

## Governance

- Require human review before merging changes the `coder` sub-agent in Kimi Code produced with shell access and file-write permissions; the `explore` and `plan` sub-agents are read-only or tool-free by design and carry less risk.
- Route planning, plan-check, and review to K3; day-to-day coding to K2.7 Code; high-volume or rapid-iteration edits to K2.7 Code High-Speed.
- Move any workflow still on K2.5 or the moonshot-v1 series off before the 2026-08-31 platform sunset; those ids stop working after that date.

## Access routes

Prices as of 2026-10-24.

| Route | What you pay | Tool | Limits, per the vendor |
|---|---|---|---|
| Plan + vendor CLI | Kimi Code plan, subscription-based; tiers and prices are on the Kimi Code site, which this note could not read on 2026-10-24 | Kimi Code CLI (`kimi`) | One subscription covers desktop, CLI, and IDE. The Kimi Code API is separate from the Kimi API Platform, and keys are not interchangeable. |
| Direct API | K3.1 Pro $2 / $10; K3.1 Pro High-Speed $4 / $20; K3 $3 / $15 per million input/output tokens (cache hits $0.20 on K3.1) | Kimi Code with an API Platform key, or any harness | Metered per token. Context caching is billed automatically. |
| OpenRouter | `moonshotai/kimi-k3.1-pro` $2 / $10 | Any OpenAI-compatible harness | Provider prices can change without notice. |

- When a plan pays for itself: a Kimi Code plan at $P a month matches P / 10 million K3.1 Pro output tokens at the API's $10 per M, or P / 20 million on High-Speed. Read the current tier prices on the Kimi Code site before deciding.
- Run repo-aware agent work through Kimi Code on a plan for daily use, under an OS sandbox (see Governance).
- Use a metered key for fast, small, occasional calls.

## Sources

- https://platform.kimi.ai/docs/models
- https://platform.kimi.ai/docs/pricing/chat-k3
- https://github.com/MoonshotAI/kimi-code/blob/main/docs/en/customization/agents.md
- https://platform.kimi.ai/docs/guide/product-plans (Kimi Code is subscription-based and separate from the API Platform; retrieved 2026-10-24)
- https://www.kimi.ai/code (Kimi Code plan tiers)
- https://openrouter.ai/api/v1/models (OpenRouter prices; retrieved 2026-10-24)
