## Choosing an OpenAI model

Pick the model by the task, not by habit. The GPT-6 family covers every role: Astra for judgment, Sol for everyday coding, Luna for volume. The GPT-5.6 tiers are the fallback while GPT-6 Sol and Luna roll out.

| Model | ID | $/1M tokens (in/out) | Use for |
|---|---|---|---|
| GPT-6 Astra | `gpt-6-astra` | $10 / $50 | Planning, plan-check, design, and review. 1.05M context, 128K output. Batch and flex run at half price; fast mode at double; requests over 272K input tokens at $20 / $75. |
| GPT-6 Sol | `gpt-6-sol` | $2 / $10 | Day-to-day coding and lower-stakes review. Codex's recommended default. |
| GPT-6 Luna | `gpt-6-luna` | $0.10 / $0.50 | Bulk and mechanical work: classification, extraction, transformation, structured summaries at scale. |
| GPT-5.6 Sol | `gpt-5.6-sol` | $4 / $20 (promotional through 2026-11-21; list $5 / $30) | A capable review model, superseded by Astra for planning and design. |
| GPT-5.6 Terra | `gpt-5.6-terra` | $2 / $12 | Fallback for coding where GPT-6 Sol is not yet available. |
| GPT-5.6 Luna | `gpt-5.6-luna` | $0.20 / $1.20 | Fallback for bulk work where GPT-6 Luna is not yet available. |
| GPT-5.5 | `gpt-5.5` | review-required | Leaves ChatGPT and Codex on 2026-10-14 (the API keeps it). Migrate to GPT-6 Sol or Luna. |
| GPT-5.2 | `gpt-5.2` | banned | Retired for new work; Codex under a ChatGPT sign-in rejects it, a direct API key still serves it. Migrate to GPT-6 Sol or Luna. |

Rules of thumb:

- Reach for Astra when the work is a plan, a design, a decision, or a review where a missed finding is expensive. It matches the top Anthropic tier on price, so treat it the same way: the first model for judgment, not for volume.
- Code day-to-day on GPT-6 Sol. It is the default implementation model and covers review on lower-stakes diffs.
- Run bulk and mechanical edits on GPT-6 Luna; it is 20x cheaper than GPT-6 Sol on output tokens.
- If Codex under a ChatGPT sign-in answers a GPT-6 Sol or Luna request with "not supported when using Codex with a ChatGPT account", the rollout has not reached that account. Use GPT-5.6 Terra or Luna until it does; do not treat the error as a retirement.
- GPT-5.6 Sol is not retired. Keep it where it already reviews well, and move new work to the GPT-6 models as they become available.
- Codex CLI under a ChatGPT sign-in bills to the plan, not per token; the same models through the API bill per token. Know which one a given session is on before running something long.
- Use the exact model IDs in the table. Never guess or construct IDs.
- Model lineup and pricing change. This guidance is versioned and updated centrally; changes arrive through the pack, not by editing this block.
