---
targets: [agents]
---
# GPT-5.6 Sol governance (gpt-5.6-sol)

- Use `gpt-5.6-sol` for planning, plan-checking, and reviewing multi-file changes (1.05M-token context, $5 / $30 per million input/output tokens).
- Reserve it for problems that need its full reasoning depth rather than routine coding; send day-to-day coding to `gpt-5.6-terra`.
- Run bulk and mechanical work on `gpt-5.6-luna`.
- Require a human review gate before merging any change Sol produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the OpenAI model guidance page for sources and full governance notes.
