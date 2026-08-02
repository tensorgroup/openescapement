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

## Sources

- https://platform.kimi.ai/docs/models
- https://platform.kimi.ai/docs/pricing/chat-k3
- https://github.com/MoonshotAI/kimi-code/blob/main/docs/en/customization/agents.md
