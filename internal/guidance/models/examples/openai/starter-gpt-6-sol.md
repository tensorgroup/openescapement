---
targets: [agents]
---
# GPT-6 Sol governance (gpt-6-sol)

- Route day-to-day coding to `gpt-6-sol`: built for complex coding and agentic workflows, and cheaper than `gpt-5.6-terra` on output tokens (1.05M-token context).
- Also use it for review on lower-stakes diffs where a full `gpt-6-astra` pass is not warranted.
- Keep planning and plan-checking on `gpt-6-astra`, and run bulk work on `gpt-6-luna`.
- If Codex under a ChatGPT sign-in rejects it as not supported, the rollout has not reached the account yet; use `gpt-5.6-terra` until it does.
- Migrate any configuration still pointing at `gpt-5.5` (leaves Codex on 2026-10-14) or the legacy `gpt-5.2` to Sol.

See the OpenAI model guidance page for sources and full governance notes.
