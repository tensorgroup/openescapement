## Choosing an OpenAI model

Pick the model by the task, not by habit. Astra is the frontier model; the GPT-5.6 tiers cover everything that does not need it.

| Model | ID | $/1M tokens (in/out) | Use for |
|---|---|---|---|
| GPT-6 Astra | `gpt-6-astra` | $10 / $50 | Planning, plan-check, design, and review. 1.05M context, 128K output. Batch and flex run at half price; fast mode at double; requests over 272K input tokens at $20 / $75. |
| GPT-5.6 Terra | `gpt-5.6-terra` | $2 / $12 | Day-to-day coding and lower-stakes review. |
| GPT-5.6 Luna | `gpt-5.6-luna` | $0.20 / $1.20 | Bulk and mechanical work: classification, extraction, simple agentic subtasks at scale. |
| GPT-5.6 Sol | `gpt-5.6-sol` | $4 / $20 (promotional through 2026-11-21; list $5 / $30) | A capable review model, superseded by Astra for planning and design. |
| GPT-5.2 | `gpt-5.2` | banned | Retired for new work; Codex under a ChatGPT sign-in rejects it, a direct API key still serves it. Migrate to Terra or Luna. |

Rules of thumb:

- Reach for Astra when the work is a plan, a design, a decision, or a review where a missed finding is expensive. It matches the top Anthropic tier on price, so treat it the same way: the first model for judgment, not for volume.
- Code day-to-day on Terra. It is the default implementation model and covers review on lower-stakes diffs.
- Run bulk and mechanical edits on Luna; it is an order of magnitude cheaper than Terra on output tokens.
- Sol is not retired. Keep it where it already reviews well, and move planning and design work to Astra as sessions roll over.
- Codex CLI under a ChatGPT sign-in bills to the plan, not per token; the same models through the API bill per token. Know which one a given session is on before running something long.
- Use the exact model IDs in the table. Never guess or construct IDs.
- Model lineup and pricing change. This guidance is versioned and updated centrally; changes arrive through the pack, not by editing this block.
