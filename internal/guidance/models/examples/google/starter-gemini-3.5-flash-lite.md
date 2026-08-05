---
targets: [gemini, agents]
---
# Gemini 3.5 Flash-Lite governance (gemini-3.5-flash-lite)

- Route bulk, mechanical, and high-volume work to `gemini-3.5-flash-lite`: sub-agent tasks, document parsing, and simple extraction where latency and API cost bind (1,048,576-token input context).
- On output tokens it is roughly 3x cheaper than `gemini-3.6-flash` and 5x cheaper than `gemini-3.1-pro-preview`.
- Send day-to-day coding to `gemini-3.6-flash` and planning or review to `gemini-3.1-pro-preview`.
- Use it for classification, extraction, and high-volume agentic subtasks run at scale.

See the Google model guidance page for sources and full governance notes.
