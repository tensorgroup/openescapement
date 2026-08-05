---
targets: [claude, agents]
---
# Claude Haiku 4.5 governance (claude-haiku-4-5)

- Route bulk, mechanical, and high-volume work to `claude-haiku-4-5`: classification, extraction, mechanical edits, and simple agentic subtasks run at scale (200K-token context).
- Do not route multi-step reasoning to it; it is the only current Claude model without adaptive thinking.
- Send day-to-day coding to `claude-sonnet-5` and planning or review to `claude-opus-5`.
- It is roughly 5x cheaper than Sonnet 5 on output tokens, so prefer it for repetitive work to hold spend down.

See the Anthropic model guidance page for sources and full governance notes.
