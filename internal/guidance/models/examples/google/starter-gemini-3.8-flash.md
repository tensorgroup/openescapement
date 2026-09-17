---
targets: [gemini, agents]
---
# Gemini 3.8 Flash governance (gemini-3.8-flash)

- Route day-to-day coding to `gemini-3.8-flash`: Google's most intelligent Flash model, built for long-horizon software engineering and agents at Flash speed and cost (1,048,576-token input context).
- Also use it for review on lower-stakes diffs where a full `gemini-3.1-pro-preview` pass is not warranted.
- Keep planning and plan-checking on `gemini-3.1-pro-preview`, and run bulk work on `gemini-3.5-flash-lite`.
- Its introductory price runs through 2026-12-31 and then doubles; `gemini-3.5-flash-lite` stays the cheaper tier throughout, so keep bulk work there.
- It supersedes `gemini-3.7-flash` and `gemini-3.6-flash`; migrate saved configurations to it.

See the Google model guidance page for sources and full governance notes.
