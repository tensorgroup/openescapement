---
targets: [agents]
---
# GPT-5.6 Terra governance (gpt-5.6-terra)

- Route day-to-day coding to `gpt-5.6-terra`: competitive with the prior GPT-5.5 flagship at well under half the cost (1.05M-token context).
- Also use it for review on lower-stakes diffs where a full `gpt-6-astra` pass is not warranted.
- Keep planning and plan-checking on `gpt-6-astra`, and run bulk work on `gpt-5.6-luna`.
- Migrate any configuration still pointing at the legacy `gpt-5.2` to Terra or Luna.

See the OpenAI model guidance page for sources and full governance notes.
