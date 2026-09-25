---
targets: [agents]
---
# GPT-5.6 Terra governance (gpt-5.6-terra)

- `gpt-6-sol` is the default coding model now; use `gpt-5.6-terra` for day-to-day coding only where the GPT-6 rollout has not reached the account (1.05M-token context).
- Where it is the fallback, it also covers review on lower-stakes diffs where a full `gpt-6-astra` pass is not warranted.
- Keep planning and plan-checking on `gpt-6-astra`, and run bulk work on `gpt-6-luna` (or `gpt-5.6-luna` as its fallback).
- Migrate any configuration still pointing at the legacy `gpt-5.2` to `gpt-6-sol`, or to Terra where Sol is not yet available.

See the OpenAI model guidance page for sources and full governance notes.
