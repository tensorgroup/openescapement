---
targets: [agents]
---
# GPT-6 Sol governance (gpt-6-sol)

- `gpt-6.1-sol` is the coding model now; use `gpt-6-sol` for day-to-day coding only where the GPT-6.1 Sol rollout has not reached the account (1.05M-token context).
- Where it is the fallback, it also covers review on lower-stakes diffs where a full `gpt-6-astra` pass is not warranted.
- Keep planning and plan-checking on `gpt-6-astra`, and run bulk work on `gpt-6-luna`.
- If Codex under a ChatGPT sign-in rejects it as not supported, the GPT-6 rollout has not reached the account either; use `gpt-5.6-terra` until it does.
- Migrate any configuration still pointing at `gpt-5.5` (left Codex on 2026-10-14) or the legacy `gpt-5.2` to `gpt-6.1-sol`, or to this model where GPT-6.1 Sol is not yet available.

See the OpenAI model guidance page for sources and full governance notes.
