---
targets: [agents]
---
# GPT-6.1 Sol governance (gpt-6.1-sol)

- Route day-to-day coding to `gpt-6.1-sol`: OpenAI positions it as near-Astra performance for complex coding at a lower cost, and Codex recommends it where it is available (1.05M-token context).
- Also use it for review on lower-stakes diffs where a full `gpt-6-astra` pass is not warranted.
- Keep planning and plan-checking on `gpt-6-astra`, and run bulk work on `gpt-6-luna`.
- Set reasoning effort from `low` up; it does not accept `none` or `minimal`, so a configuration copied from `gpt-6-sol` that sets either fails.
- If Codex under a ChatGPT sign-in does not offer it, the rollout has not reached the account yet (Enterprise and Edu also need an administrator to enable it); use `gpt-6-sol` until it does.
- Migrate any configuration still pointing at `gpt-5.5` (left Codex on 2026-10-14) or the legacy `gpt-5.2` to this model.

See the OpenAI model guidance page for sources and full governance notes.
