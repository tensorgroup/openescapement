---
targets: [gemini, agents]
---
# Gemini 4 Carbon governance (gemini-4-carbon)

- Route only the hardest planning, plan-checking, and review to `gemini-4-carbon`: system architecture, multi-file debugging, security audits, formal proofs, or a task where `gemini-4-neon` gets stuck. Google calls it its strongest reasoning model.
- Keep everyday planning and review on `gemini-4-neon`. Carbon spends more thinking tokens per turn, so it is slower and costs more per task.
- Set the thinking level explicitly. Carbon accepts `low`, `medium` (the default), and `high`.
- In the Antigravity CLI (`agy`), select it by an effort-suffixed id, such as `gemini-4-carbon-high`. agy offers it on Google AI Ultra and Enterprise only.
- Require a human review gate before merging any change it produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the Google model guidance page for sources and full governance notes.
