---
targets: [agents]
---
# GPT-5.6 Luna governance (gpt-5.6-luna)

- `gpt-6-luna` is the bulk model now, at under half the price; use `gpt-5.6-luna` for bulk, mechanical, and high-volume work only where the GPT-6 rollout has not reached the account (1.05M-token context).
- Send day-to-day coding to `gpt-6-sol` (or `gpt-5.6-terra` as its fallback) and planning or review to `gpt-6-astra`.
- Migrate any configuration still pointing at the legacy `gpt-5.2` to `gpt-6-luna`, or to this model where GPT-6 Luna is not yet available.

See the OpenAI model guidance page for sources and full governance notes.
