## Choosing a Google Gemini model

Gemini 4 Argon is the model for judgment: plans, plan checks, and reviews. The Flash tiers carry the volume.

| Model | ID | $/1M tokens (in/out), 2026-10-24 | Use for |
|---|---|---|---|
| Gemini 4 Argon | `gemini-4-argon` | $2 / $10 through 2026-11-30, then $4 / $20; Batch and Flex half | Planning, plan-check, review. Set the thinking level explicitly. |
| Gemini 3.8 Flash | `gemini-3.8-flash` | $0.75 / $3.75 through 2026-12-31, then $1.50 / $7.50 | Day-to-day coding, lower-stakes review. |
| Gemini 3.5 Flash-Lite | `gemini-3.5-flash-lite` | $0.30 / $2.50 | Bulk and mechanical work at scale. |
| Gemini 3.1 Pro Preview | `gemini-3.1-pro-preview` | $2 / $12 for prompts up to 200K tokens, $4 / $18 above | Only workloads that need explicit context caching. |

Rules of thumb:

- Run daily agent work through the Antigravity CLI (`agy`) on a Google AI Pro or Ultra plan. A plan is cheaper than per-token billing once a person codes with an agent most days, and agy also serves Claude Opus 5.5 and Sonnet 5.5 on the same plan.
- Select Argon in agy by its effort-suffixed id (`gemini-4-argon-high` for plans and reviews). agy encodes the effort level in the model id.
- Set Argon's thinking level explicitly in API calls. The default is `medium`, and the level changes both quality and spend.
- Budget for Argon's price doubling on 2026-12-01. Send reviews that can wait to the Batch tier at half price.
- Remove explicit context caching and sampling parameters when moving a workload to Argon. Argon does not support explicit caching and ignores `temperature`, `top_p`, and `top_k`.
- Model lineup and pricing change. This guidance is versioned and updated centrally; changes arrive through the pack, not by editing this block.
