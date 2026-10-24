---
targets: [gemini, agents]
---
# Gemini 3.7 Flash governance (gemini-3.7-flash)

- Migrate `gemini-3.7-flash` pins to `gemini-3.8-flash`. Google deprecated 3.7 Flash on 2026-10-08, and API requests to it are already served by `gemini-3.8-flash`.
- Do not start new work on `gemini-3.7-flash`; a pin that silently routes to another model hides which model did the work.
- Keep planning and plan-checking on `gemini-4-argon`, and run bulk work on `gemini-3.5-flash-lite`.

See the Google model guidance page for sources and full governance notes.
