---
targets: [agents]
---
# GPT-6 Luna governance (gpt-6-luna)

- Route bulk, mechanical, and high-volume work to `gpt-6-luna`: classification, extraction, transformation, and structured summaries run at scale (1.05M-token context).
- It is the cheapest tier in the family, 20x cheaper than `gpt-6-sol` and 100x cheaper than `gpt-6-astra` on output tokens. Start at `high` reasoning effort; it supports up to `max`, not `ultra`.
- Send day-to-day coding to `gpt-6-sol` and planning or review to `gpt-6-astra`.
- If Codex under a ChatGPT sign-in rejects it as not supported, the rollout has not reached the account yet (Enterprise and Edu also need an administrator to enable it); use `gpt-5.6-luna` until it does.

See the OpenAI model guidance page for sources and full governance notes.
