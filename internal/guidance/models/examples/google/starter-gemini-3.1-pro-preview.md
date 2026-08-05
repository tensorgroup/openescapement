---
targets: [gemini, agents]
---
# Gemini 3.1 Pro Preview governance (gemini-3.1-pro-preview)

- Use `gemini-3.1-pro-preview` for planning, plan-checking, and reviewing multi-file changes (1,048,576-token input context).
- Treat its preview label as a production caveat, not a reason to avoid it; it is Google's current frontier model.
- Send day-to-day coding to `gemini-3.6-flash` and bulk work to `gemini-3.5-flash-lite`.
- Require a human review gate before merging any change it produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the Google model guidance page for sources and full governance notes.
