---
targets: [agents]
---
# GPT-6.2 Sol governance (gpt-6.2-sol)

- Route day-to-day coding to `gpt-6.2-sol`. OpenAI calls it its fast, go-to model for coding and everyday work, and lists it first in Codex.
- Also use it for review on lower-stakes diffs where a full `gpt-6.2-astra` pass is not warranted.
- Keep planning and plan-checking on `gpt-6.2-astra`, and run bulk work on `gpt-6-luna`.
- If Codex under a ChatGPT sign-in does not offer it, use `gpt-6.1-sol` until it reaches the account.
- Migrate any configuration still pointing at `gpt-6-sol` or the GPT-5.6 models (legacy), `gpt-5.5` (left Codex on 2026-10-14), or `gpt-5.2` to this model.

See the OpenAI model guidance page for sources and full governance notes.
