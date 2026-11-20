---
targets: [gemini, agents]
---
# Gemini 4 Neon governance (gemini-4-neon)

- Route planning, plan-checking, review, and complex agentic coding to `gemini-4-neon`. Google places it between Hydrogen and Carbon: it reasons more deeply than Hydrogen and finishes faster than Carbon.
- Set the thinking level explicitly. Neon accepts `low`, `medium`, `high` (the default), and `xhigh`. Use `xhigh` only when `high` is not enough, because it spends the most tokens.
- In the Antigravity CLI (`agy`), select it by an effort-suffixed id, such as `gemini-4-neon-high` or `gemini-4-neon-xhigh`.
- Send the hardest problems (system architecture, multi-file debugging, security audits, or a task where Neon gets stuck) to `gemini-4-carbon`. Send everyday coding to `gemini-4-hydrogen`.
- It supersedes `gemini-3.1-pro-preview` and is the recommended pick over `gemini-4-argon`. Migrate saved configurations to it.
- Require a human review gate before merging any change it produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the Google model guidance page for sources and full governance notes.
