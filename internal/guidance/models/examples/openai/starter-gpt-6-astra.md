---
targets: [agents]
---
# GPT-6 Astra governance (gpt-6-astra)

- `gpt-6.2-astra` is the judgment model now; use `gpt-6-astra` (previous generation) for planning, plan-checking, and review only where GPT-6.2 Astra has not reached the account (1.05M-token context).
- Reserve it for judgment rather than volume; send day-to-day coding to `gpt-6.2-sol` (or `gpt-6.1-sol`) and bulk work to `gpt-6-luna`.
- Know which route a session is on: Codex CLI under a ChatGPT sign-in bills to the plan, the API bills per token.
- Require a human review gate before merging any change Astra produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the OpenAI model guidance page for sources and full governance notes.
