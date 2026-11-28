## Choosing an OpenAI model

Pick the model by the task, not by habit. The GPT-6.2 generation covers the main roles: Astra for judgment, Sol for everyday coding. GPT-6 Luna covers volume, and GPT-6.1 Terra covers visual understanding and computer use. GPT-6 Astra and GPT-6.1 Sol are the previous generation; GPT-6 Sol and the GPT-5.6 tiers are legacy fallbacks.

| Model | ID | $/1M tokens (in/out) | Use for |
|---|---|---|---|
| GPT-6.2 Astra | `gpt-6.2-astra` | see OpenAI's pricing page | Planning, plan-check, design, and review. OpenAI's most capable model for complex coding and professional work. |
| GPT-6.2 Sol | `gpt-6.2-sol` | see OpenAI's pricing page | Day-to-day coding and lower-stakes review. OpenAI's fast, go-to model for coding and everyday work. |
| GPT-6.1 Terra | `gpt-6.1-terra` | see OpenAI's pricing page | Visual understanding and computer use: reading screenshots, driving a user interface. |
| GPT-6 Luna | `gpt-6-luna` | $0.10 / $0.50 | Bulk and mechanical work: classification, extraction, transformation, structured summaries at scale. |
| GPT-6 Astra | `gpt-6-astra` | $10 / $50 | Previous generation. Fallback for judgment work where GPT-6.2 Astra is not yet available. |
| GPT-6.1 Sol | `gpt-6.1-sol` | $2 / $10 | Previous generation. Fallback for coding where GPT-6.2 Sol is not yet available. |
| GPT-6 Sol | `gpt-6-sol` | $2 / $10 | Legacy. Fallback for coding where no newer Sol model is available. |
| GPT-5.6 Sol | `gpt-5.6-sol` | $5 / $30 | Legacy. Keep only where it already reviews well. |
| GPT-5.6 Terra | `gpt-5.6-terra` | $2 / $12 | Legacy. Fallback for coding where no GPT-6 Sol model is available yet. |
| GPT-5.6 Luna | `gpt-5.6-luna` | $0.20 / $1.20 | Legacy. Fallback for bulk work where GPT-6 Luna is not yet available. |
| GPT-5.5 | `gpt-5.5` | review-required | Left ChatGPT and Codex on 2026-10-14 (the API keeps it). Migrate to GPT-6.2 Sol or GPT-6 Luna. |
| GPT-5.2 | `gpt-5.2` | banned | Retired for new work; Codex under a ChatGPT sign-in rejects it, a direct API key still serves it. Migrate to GPT-6.2 Sol or GPT-6 Luna. |

Rules of thumb:

- Reach for GPT-6.2 Astra when the work is a plan, a design, a decision, or a review where a missed finding is expensive. Use it for judgment, not for volume. Astra has been OpenAI's highest-priced tier, so check its price before running it at scale.
- Code day-to-day on GPT-6.2 Sol. It is the default implementation model and covers review on lower-stakes diffs.
- Run bulk and mechanical edits on GPT-6 Luna, the cheapest current tier.
- Use GPT-6.1 Terra when the task depends on reading images or operating a user interface.
- If Codex under a ChatGPT sign-in does not offer a GPT-6.2 model, use its previous-generation counterpart (GPT-6 Astra or GPT-6.1 Sol) until the new one reaches the account. Do not treat a missing model as a retirement.
- Move new work off the legacy models (GPT-6 Sol and the GPT-5.6 tiers). Keep a legacy model only where it already works well and no newer model is available.
- Codex CLI under a ChatGPT sign-in bills to the plan, not per token; the same models through the API bill per token. Know which one a given session is on before running something long.
- Use the exact model IDs in the table. Never guess or construct IDs.
- Model lineup and pricing change. This guidance is versioned and updated centrally; changes arrive through the pack, not by editing this block.
