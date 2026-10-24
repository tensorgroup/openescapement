---
targets: [gemini, agents]
---
# Gemini 3.1 Pro Preview governance (gemini-3.1-pro-preview)

- Route planning, plan-checking, and review to `gemini-4-argon`; it is Google's flagship reasoning model and generally available, while `gemini-3.1-pro-preview` still carries a preview label.
- Keep `gemini-3.1-pro-preview` only where a workload depends on explicit context caching, which `gemini-4-argon` does not support.
- Send day-to-day coding to `gemini-3.8-flash` and bulk work to `gemini-3.5-flash-lite`.
- Require a human review gate before merging any change `gemini-3.1-pro-preview` produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the Google model guidance page for sources and full governance notes.
