---
targets: [agents]
---
# GPT-6 Astra governance (gpt-6-astra)

- Use `gpt-6-astra` for planning, plan-checking, and reviewing multi-file changes, and for decisions where a missed finding is expensive (1.05M-token context).
- It is priced like the top Anthropic tier, so reserve it for judgment rather than volume; send day-to-day coding to `gpt-5.6-terra` and bulk work to `gpt-5.6-luna`.
- Know which route a session is on: Codex CLI under a ChatGPT sign-in bills to the plan, the API bills per token.
- Require a human review gate before merging any change Astra produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the OpenAI model guidance page for sources and full governance notes.
