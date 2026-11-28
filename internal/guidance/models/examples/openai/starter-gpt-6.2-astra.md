---
targets: [agents]
---
# GPT-6.2 Astra governance (gpt-6.2-astra)

- Use `gpt-6.2-astra` for planning, plan-checking, and reviewing multi-file changes, and for decisions where a missed finding is expensive. OpenAI calls it its most capable model for complex coding and professional work.
- Reserve it for judgment rather than volume; send day-to-day coding to `gpt-6.2-sol` and bulk work to `gpt-6-luna`. Check its price on OpenAI's pricing page before running it at scale.
- If Codex under a ChatGPT sign-in does not offer it, use `gpt-6-astra` until it reaches the account.
- Know which route a session is on: Codex CLI under a ChatGPT sign-in bills to the plan, the API bills per token.
- Require a human review gate before merging any change Astra produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the OpenAI model guidance page for sources and full governance notes.
