---
targets: [gemini, agents]
---
# Gemini 3.1 Pro Preview governance (gemini-3.1-pro-preview)

- Route planning, plan-checking, and review to `gemini-4-neon`. Google names Neon as the upgrade for `gemini-3.1-pro-preview` (or `gemini-4-carbon` for the hardest problems).
- Keep `gemini-3.1-pro-preview` only where a workload depends on explicit context caching. No Gemini 4 model supports explicit caching.
- Send day-to-day coding to `gemini-4-hydrogen` and bulk work to `gemini-3.5-flash-lite`.
- Require a human review gate before merging any change `gemini-3.1-pro-preview` produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the Google model guidance page for sources and full governance notes.
