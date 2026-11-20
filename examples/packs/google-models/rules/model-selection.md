## Choosing a Google Gemini model

Gemini 4 Neon is the model for judgment: plans, plan checks, reviews, and complex agentic coding. Gemini 4 Carbon takes the hardest problems. Gemini 4 Hydrogen and the Flash tiers carry the volume.

| Model | ID | $/1M tokens (in/out), 2026-11-20 | Use for |
|---|---|---|---|
| Gemini 4 Neon | `gemini-4-neon` | $3 / $15; Batch and Flex half | Planning, plan-check, review, complex agentic coding. The default frontier pick. |
| Gemini 4 Carbon | `gemini-4-carbon` | $2 / $10 through 2026-11-30, then $4 / $20; Batch and Flex half | The hardest problems: architecture, multi-file debugging, security audits, or a task where Neon gets stuck. |
| Gemini 4 Hydrogen | `gemini-4-hydrogen` | $1.50 / $7.50; Batch and Flex half | Day-to-day coding, parallel worker agents, lower-stakes review. |
| Gemini 3.8 Flash | `gemini-3.8-flash` | $0.75 / $3.75 through 2026-12-31, then $1.50 / $7.50 | Coding where cost binds, until its introductory price ends. |
| Gemini 3.5 Flash-Lite | `gemini-3.5-flash-lite` | $0.30 / $2.50 | Bulk and mechanical work at scale. |
| Gemini 4 Argon | `gemini-4-argon` | $2 / $10 through 2026-11-30, then $4 / $20 | Only workloads already tuned to it. Move new work to Neon. |
| Gemini 3.1 Pro Preview | `gemini-3.1-pro-preview` | $2 / $12 for prompts up to 200K tokens, $4 / $18 above | Only workloads that need explicit context caching. |

Rules of thumb:

- Run daily agent work through the Antigravity CLI (`agy`) on a Google AI Pro or Ultra plan. A plan is cheaper than per-token billing once a person codes with an agent most days, and agy also serves Claude Opus 5.5 and Sonnet 5.5 on the same plan.
- Select a model in agy by its effort-suffixed id, such as `gemini-4-neon-high` for plans and reviews. agy encodes the effort level in the model id.
- Set the thinking level explicitly in API calls. Neon and Hydrogen default to `high` and add `xhigh`; Carbon and Argon default to `medium` and stop at `high`. The level changes both quality and spend.
- Use `xhigh` only when `high` is not enough. It spends the most tokens.
- Carbon is on Google AI Ultra and Enterprise only in agy. On Google AI Pro, Neon is the strongest Gemini model available.
- Budget for Carbon's and Argon's price doubling on 2026-12-01. Send reviews that can wait to the Batch tier at half price.
- No Gemini 4 model supports explicit context caching. Remove explicit caching calls when moving a workload to Gemini 4.
- Model lineup and pricing change. This guidance is versioned and updated centrally; changes arrive through the pack, not by editing this block.
