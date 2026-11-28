---
targets: [agents]
---
# GPT-6.1 Sol governance (gpt-6.1-sol)

- `gpt-6.2-sol` is the coding model now; use `gpt-6.1-sol` (previous generation) for day-to-day coding only where GPT-6.2 Sol has not reached the account (1.05M-token context).
- Where it is the fallback, it also covers review on lower-stakes diffs where a full Astra pass is not warranted.
- Keep planning and plan-checking on `gpt-6.2-astra` (or `gpt-6-astra`), and run bulk work on `gpt-6-luna`.
- Set reasoning effort from `low` up; it does not accept `none` or `minimal`, so a configuration copied from `gpt-6-sol` that sets either fails.

See the OpenAI model guidance page for sources and full governance notes.
